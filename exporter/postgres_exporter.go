// Copyright 2021 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package exporter

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blang/semver/v4"
	"github.com/percona/postgres_exporter/internal/versioncache"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/semaphore"
)

// Metric name parts.
const (
	// Namespace for all metrics.
	namespace = "pg"
	// Subsystems.
	exporter = "exporter"
	// Metric label used for static string data thats handy to send to Prometheus
	// e.g. version
	staticLabelName = "static"
	// Metric label used for server identification.
	serverLabelName = "server"
)

// ColumnUsage should be one of several enum values which describe how a
// queried row is to be converted to a Prometheus metric.
type ColumnUsage int

const (
	// DISCARD ignores a column
	DISCARD ColumnUsage = iota
	// LABEL identifies a column as a label
	LABEL ColumnUsage = iota
	// COUNTER identifies a column as a counter
	COUNTER ColumnUsage = iota
	// GAUGE identifies a column as a gauge
	GAUGE ColumnUsage = iota
	// MAPPEDMETRIC identifies a column as a mapping of text values
	MAPPEDMETRIC ColumnUsage = iota
	// DURATION identifies a column as a text duration (and converted to milliseconds)
	DURATION ColumnUsage = iota
	// HISTOGRAM identifies a column as a histogram
	HISTOGRAM ColumnUsage = iota
)

// UnmarshalYAML implements the yaml.Unmarshaller interface.
func (cu *ColumnUsage) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var value string
	if err := unmarshal(&value); err != nil {
		return err
	}

	columnUsage, err := stringToColumnUsage(value)
	if err != nil {
		return err
	}

	*cu = columnUsage
	return nil
}

// MappingOptions is a copy of ColumnMapping used only for parsing
type MappingOptions struct {
	Usage             string             `yaml:"usage"`
	Description       string             `yaml:"description"`
	Mapping           map[string]float64 `yaml:"metric_mapping"` // Optional column mapping for MAPPEDMETRIC
	SupportedVersions semver.Range       `yaml:"pg_version"`     // Semantic version ranges which are supported. Unsupported columns are not queried (internally converted to DISCARD).
}

// Mapping represents a set of MappingOptions
type Mapping map[string]MappingOptions

// Regex used to get the "short-version" from the postgres version field.
var versionRegex = regexp.MustCompile(`^\w+ ((\d+)(\.\d+)?(\.\d+)?)`)
var lowestSupportedVersion = semver.MustParse("10.0.0")

// Parses the version of postgres into the short version string we can use to
// match behaviors.
func parseVersion(versionString string) (semver.Version, error) {
	submatches := versionRegex.FindStringSubmatch(versionString)
	if len(submatches) > 1 {
		return semver.ParseTolerant(submatches[1])
	}
	return semver.Version{},
		errors.New(fmt.Sprintln("Could not find a postgres version in string:", versionString))
}

// ColumnMapping is the user-friendly representation of a prometheus descriptor map
type ColumnMapping struct {
	usage             ColumnUsage        `yaml:"usage"`
	description       string             `yaml:"description"`
	mapping           map[string]float64 `yaml:"metric_mapping"` // Optional column mapping for MAPPEDMETRIC
	supportedVersions semver.Range       `yaml:"pg_version"`     // Semantic version ranges which are supported. Unsupported columns are not queried (internally converted to DISCARD).
}

// UnmarshalYAML implements yaml.Unmarshaller
func (cm *ColumnMapping) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain ColumnMapping
	return unmarshal((*plain)(cm))
}

// intermediateMetricMap holds the partially loaded metric map parsing.
// This is mainly so we can parse cacheSeconds around.
type intermediateMetricMap struct {
	columnMappings map[string]ColumnMapping
	master         bool
	cacheSeconds   uint64
}

// MetricMapNamespace groups metric maps under a shared set of labels.
type MetricMapNamespace struct {
	labels         []string             // Label names for this namespace
	columnMappings map[string]MetricMap // Column mappings in this namespace
	master         bool                 // Call query only for master database
	cacheSeconds   uint64               // Number of seconds this metric namespace can be cached. 0 disables.
}

// MetricMap stores the prometheus metric description which a given column will
// be mapped to by the collector
type MetricMap struct {
	discard    bool                              // Should metric be discarded during mapping?
	histogram  bool                              // Should metric be treated as a histogram?
	vtype      prometheus.ValueType              // Prometheus valuetype
	desc       *prometheus.Desc                  // Prometheus descriptor
	conversion func(interface{}) (float64, bool) // Conversion function to turn PG result into float64
}

// ErrorConnectToServer is a connection to PgSQL server error
type ErrorConnectToServer struct {
	Msg string
}

// Error returns error
func (e *ErrorConnectToServer) Error() string {
	return e.Msg
}

// TODO: revisit this with the semver system
func DumpMaps() {
	// TODO: make this function part of the exporter
	for _, cmap := range builtinMetricMaps {

		for column, details := range cmap.columnMappings {
			fmt.Printf("  %-40s %v\n", column, details)
		}
		fmt.Println()
	}
}

var builtinMetricMaps = map[string]intermediateMetricMap{
	"pg_stat_database_conflicts": {
		map[string]ColumnMapping{
			"datid":            {LABEL, "OID of a database", nil, nil},
			"datname":          {LABEL, "Name of this database", nil, nil},
			"confl_tablespace": {COUNTER, "Number of queries in this database that have been canceled due to dropped tablespaces", nil, nil},
			"confl_lock":       {COUNTER, "Number of queries in this database that have been canceled due to lock timeouts", nil, nil},
			"confl_snapshot":   {COUNTER, "Number of queries in this database that have been canceled due to old snapshots", nil, nil},
			"confl_bufferpin":  {COUNTER, "Number of queries in this database that have been canceled due to pinned buffers", nil, nil},
			"confl_deadlock":   {COUNTER, "Number of queries in this database that have been canceled due to deadlocks", nil, nil},
		},
		true,
		0,
	},
	"pg_lock_conflicts": {
		map[string]ColumnMapping{
			"blocking_pid": {LABEL, "PID of blocking session", nil, nil},
			"count":        {GAUGE, "Number of blocked sessions", nil, nil},
		},
		true,
		0,
	},
}

// Turn the MetricMap column mapping into a prometheus descriptor mapping.
func makeDescMap(pgVersion semver.Version, serverLabels prometheus.Labels, metricMaps map[string]intermediateMetricMap, logger *slog.Logger, metricPrefix string) map[string]MetricMapNamespace {
	var metricMap = make(map[string]MetricMapNamespace)

	for namespace, intermediateMappings := range metricMaps {
		thisMap := make(map[string]MetricMap)

		namespace = strings.Replace(namespace, "pg", metricPrefix, 1)

		// Get the constant labels
		var variableLabels []string
		for columnName, columnMapping := range intermediateMappings.columnMappings {
			if columnMapping.usage == LABEL {
				variableLabels = append(variableLabels, columnName)
			}
		}

		for columnName, columnMapping := range intermediateMappings.columnMappings {
			// Check column version compatibility for the current map
			// Force to discard if not compatible.
			if columnMapping.supportedVersions != nil {
				if !columnMapping.supportedVersions(pgVersion) {
					// It's very useful to be able to see what columns are being
					// rejected.
					logger.Debug("Column is being forced to discard due to version incompatibility", "column", columnName)
					thisMap[columnName] = MetricMap{
						discard: true,
						conversion: func(_ interface{}) (float64, bool) {
							return math.NaN(), true
						},
					}
					continue
				}
			}

			// Determine how to convert the column based on its usage.
			// nolint: dupl
			switch columnMapping.usage {
			case DISCARD, LABEL:
				thisMap[columnName] = MetricMap{
					discard: true,
					conversion: func(_ interface{}) (float64, bool) {
						return math.NaN(), true
					},
				}
			case COUNTER:
				thisMap[columnName] = MetricMap{
					vtype: prometheus.CounterValue,
					desc:  prometheus.NewDesc(fmt.Sprintf("%s_%s", namespace, columnName), columnMapping.description, variableLabels, serverLabels),
					conversion: func(in interface{}) (float64, bool) {
						return dbToFloat64(in, logger)
					},
				}
			case GAUGE:
				thisMap[columnName] = MetricMap{
					vtype: prometheus.GaugeValue,
					desc:  prometheus.NewDesc(fmt.Sprintf("%s_%s", namespace, columnName), columnMapping.description, variableLabels, serverLabels),
					conversion: func(in interface{}) (float64, bool) {
						return dbToFloat64(in, logger)
					},
				}
			case HISTOGRAM:
				thisMap[columnName] = MetricMap{
					histogram: true,
					vtype:     prometheus.UntypedValue,
					desc:      prometheus.NewDesc(fmt.Sprintf("%s_%s", namespace, columnName), columnMapping.description, variableLabels, serverLabels),
					conversion: func(in interface{}) (float64, bool) {
						return dbToFloat64(in, logger)
					},
				}
				thisMap[columnName+"_bucket"] = MetricMap{
					histogram: true,
					discard:   true,
				}
				thisMap[columnName+"_sum"] = MetricMap{
					histogram: true,
					discard:   true,
				}
				thisMap[columnName+"_count"] = MetricMap{
					histogram: true,
					discard:   true,
				}
			case MAPPEDMETRIC:
				thisMap[columnName] = MetricMap{
					vtype: prometheus.GaugeValue,
					desc:  prometheus.NewDesc(fmt.Sprintf("%s_%s", namespace, columnName), columnMapping.description, variableLabels, serverLabels),
					conversion: func(in interface{}) (float64, bool) {
						text, ok := in.(string)
						if !ok {
							return math.NaN(), false
						}

						val, ok := columnMapping.mapping[text]
						if !ok {
							return math.NaN(), false
						}
						return val, true
					},
				}
			case DURATION:
				thisMap[columnName] = MetricMap{
					vtype: prometheus.GaugeValue,
					desc:  prometheus.NewDesc(fmt.Sprintf("%s_%s_milliseconds", namespace, columnName), columnMapping.description, variableLabels, serverLabels),
					conversion: func(in interface{}) (float64, bool) {
						var durationString string
						switch t := in.(type) {
						case []byte:
							durationString = string(t)
						case string:
							durationString = t
						default:
							logger.Error("Duration conversion metric was not a string")
							return math.NaN(), false
						}

						if durationString == "-1" {
							return math.NaN(), false
						}

						d, err := time.ParseDuration(durationString)
						if err != nil {
							logger.Error("Failed converting result to metric", "column", columnName, "in", in, "err", err)
							return math.NaN(), false
						}
						return float64(d / time.Millisecond), true
					},
				}
			}
		}

		metricMap[namespace] = MetricMapNamespace{variableLabels, thisMap, intermediateMappings.master, intermediateMappings.cacheSeconds}
	}

	return metricMap
}

type cachedMetrics struct {
	metrics    []prometheus.Metric
	lastScrape time.Time
}

// Exporter collects Postgres metrics. It implements prometheus.Collector.
type Exporter struct {
	collectorName     string
	userQueriesPath   map[MetricResolution]string
	resolutionEnabled MetricResolution
	enabled           bool

	// Holds a reference to the build in column mappings. Currently this is for testing purposes
	// only, since it just points to the global.
	builtinMetricMaps map[string]intermediateMetricMap

	disableDefaultMetrics, autoDiscoverDatabases bool

	excludeDatabases []string
	includeDatabases []string
	dsn              []string
	constantLabels   prometheus.Labels
	duration         prometheus.Gauge
	error            prometheus.Gauge
	psqlUp           prometheus.Gauge
	userQueriesError *prometheus.GaugeVec
	totalScrapes     prometheus.Counter

	// servers are used to allow re-using the DB connection between scrapes.
	// servers contains metrics map and query overrides.
	// servers *Servers

	connSema  *semaphore.Weighted
	ctx       context.Context
	masterDSN string

	logger       *slog.Logger
	metricPrefix string
}

// ExporterOpt configures Exporter.
type ExporterOpt func(*Exporter)

// DisableDefaultMetrics configures default metrics export.
func DisableDefaultMetrics(b bool) ExporterOpt {
	return func(e *Exporter) {
		e.disableDefaultMetrics = b
	}
}

// CollectorName configures collector name.
func CollectorName(name string) ExporterOpt {
	return func(e *Exporter) {
		e.collectorName = name
	}
}

// WithUserQueriesEnabled enables user's queries.
func WithUserQueriesEnabled(p MetricResolution) ExporterOpt {
	return func(e *Exporter) {
		e.resolutionEnabled = p
	}
}

// WithUserQueriesEnabled enables user's queries.
func WithEnabled(p bool) ExporterOpt {
	return func(e *Exporter) {
		e.enabled = p
	}
}

// WithContext sets context for the exporter.
func WithContext(ctx context.Context) ExporterOpt {
	return func(e *Exporter) {
		e.ctx = ctx
	}
}

// WithConnectionsSemaphore sets the semaphore for limiting the number of connections to the database instance.
func WithConnectionsSemaphore(sem *semaphore.Weighted) ExporterOpt {
	return func(e *Exporter) {
		e.connSema = sem
	}
}

// AutoDiscoverDatabases allows scraping all databases on a database server.
func AutoDiscoverDatabases(b bool) ExporterOpt {
	return func(e *Exporter) {
		e.autoDiscoverDatabases = b
	}
}

// ExcludeDatabases allows to filter out result from AutoDiscoverDatabases
func ExcludeDatabases(s []string) ExporterOpt {
	return func(e *Exporter) {
		e.excludeDatabases = s
	}
}

// IncludeDatabases allows to filter result from AutoDiscoverDatabases
func IncludeDatabases(s string) ExporterOpt {
	return func(e *Exporter) {
		if len(s) > 0 {
			e.includeDatabases = strings.Split(s, ",")
		}
	}
}

// WithUserQueriesPath configures user's queries path.
func WithUserQueriesPath(p map[MetricResolution]string) ExporterOpt {
	return func(e *Exporter) {
		e.userQueriesPath = p
	}
}

// WithConstantLabels configures constant labels.
func WithConstantLabels(s string) ExporterOpt {
	return func(e *Exporter) {
		e.constantLabels = parseConstLabels(s, e.logger)
		if e.collectorName != "" {
			e.constantLabels["collector"] = e.collectorName
		} else {
			e.constantLabels["collector"] = "exporter"
		}
	}
}

// WithMetricPrefix configures metric prefix.
func WithMetricPrefix(prefix string) ExporterOpt {
	return func(e *Exporter) {
		e.metricPrefix = prefix
	}
}

func parseConstLabels(s string, logger *slog.Logger) prometheus.Labels {
	labels := make(prometheus.Labels)

	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return labels
	}

	parts := strings.Split(s, ",")
	for _, p := range parts {
		keyValue := strings.Split(strings.TrimSpace(p), "=")
		if len(keyValue) != 2 {
			logger.Error(`Wrong constant labels format, should be "key=value"`, "input", p)
			continue
		}
		key := strings.TrimSpace(keyValue[0])
		value := strings.TrimSpace(keyValue[1])
		if key == "" || value == "" {
			continue
		}
		labels[key] = value
	}

	return labels
}

// NewExporter returns a new PostgreSQL exporter for the provided DSN.
func NewExporter(dsn []string, logger *slog.Logger, opts ...ExporterOpt) *Exporter {
	e := &Exporter{
		dsn:               dsn,
		builtinMetricMaps: builtinMetricMaps,
		enabled:           true,
		ctx:               context.Background(),
		logger:            logger,
	}

	for _, opt := range opts {
		opt(e)
	}

	e.setupInternalMetrics()
	// e.servers = NewServers(ServerWithLabels(e.constantLabels))

	return e
}

// GetServer returns a new Server instance for the provided DSN.
func (e *Exporter) GetServer(dsn string, opts ...ServerOpt) (*Server, error) {
	var err error
	errCount := 0 // start at zero because we increment before doing work
	retries := 1
	var server *Server
	for {
		if errCount++; errCount > retries {
			return nil, err
		}

		server, err = NewServer(dsn, opts...)
		if err != nil {
			time.Sleep(time.Duration(errCount) * time.Second)
			continue
		}

		if err = server.Ping(); err != nil {
			server.Close()
			time.Sleep(time.Duration(errCount) * time.Second)
			continue
		}
		break
	}
	return server, nil
}

func (e *Exporter) setupInternalMetrics() {
	e.duration = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   namespace,
		Subsystem:   exporter,
		Name:        "last_scrape_duration_seconds",
		Help:        "Duration of the last scrape of metrics from PostgreSQL.",
		ConstLabels: e.constantLabels,
	})
	e.totalScrapes = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace:   namespace,
		Subsystem:   exporter,
		Name:        "scrapes_total",
		Help:        "Total number of times PostgreSQL was scraped for metrics.",
		ConstLabels: e.constantLabels,
	})
	e.error = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   namespace,
		Subsystem:   exporter,
		Name:        "last_scrape_error",
		Help:        "Whether the last scrape of metrics from PostgreSQL resulted in an error (1 for error, 0 for success).",
		ConstLabels: e.constantLabels,
	})
	e.psqlUp = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace:   namespace,
		Name:        "up",
		Help:        "Whether the last scrape of metrics from PostgreSQL was able to connect to the server (1 for yes, 0 for no).",
		ConstLabels: e.constantLabels,
	})
	e.userQueriesError = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace:   namespace,
		Subsystem:   exporter,
		Name:        "user_queries_load_error",
		Help:        "Whether the user queries file was loaded and parsed successfully (1 for error, 0 for success).",
		ConstLabels: e.constantLabels,
	}, []string{"filename", "hashsum"})
}

// Describe implements prometheus.Collector.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	if !e.enabled {
		return
	}
	e.scrape(ch)

	ch <- e.duration
	ch <- e.totalScrapes
	ch <- e.error
	ch <- e.psqlUp
	e.userQueriesError.Collect(ch)
}

// CloseServers is a no-op in the Percona build: connections are opened per
// scrape in scrapeDSN and closed there, so there is no server pool to close.
func (e *Exporter) CloseServers() {}

func checkPostgresVersion(db *sql.DB, server string, logger *slog.Logger) (semver.Version, string, error) {
	logger.Debug("Querying PostgreSQL version", "server", server)
	versionRow := db.QueryRow("SELECT version();")
	var versionString string
	err := versionRow.Scan(&versionString)
	if err != nil {
		return semver.Version{}, "", fmt.Errorf("Error scanning version string on %q: %v", server, err)
	}
	semanticVersion, err := parseVersion(versionString)
	if err != nil {
		return semver.Version{}, "", fmt.Errorf("Error parsing version string on %q: %v", server, err)
	}

	return semanticVersion, versionString, nil
}

// Check and update the exporters query maps if the version has changed.
func (e *Exporter) checkMapVersions(ch chan<- prometheus.Metric, server *Server) error {
	semanticVersion, versionString, err := versioncache.GetVersion(server.db, server.String(), func(db *sql.DB, s string) (semver.Version, string, error) {
		return checkPostgresVersion(db, s, e.logger)
	})
	if err != nil {
		return fmt.Errorf("Error fetching version string on %q: %v", server, err)
	}

	if !e.disableDefaultMetrics && semanticVersion.LT(lowestSupportedVersion) {
		e.logger.Warn("PostgreSQL version is lower than our lowest supported version", "server", server, "version", semanticVersion, "lowest_supported_version", lowestSupportedVersion)
	}

	// Check if semantic version changed and recalculate maps if needed.
	// if semanticVersion.NE(server.lastMapVersion[e.resolutionEnabled]) || server.metricMap == nil {
	//	level.Info(logger).Log("msg", "Semantic version changed", "server", server, "from", server.lastMapVersion[e.resolutionEnabled], "to", semanticVersion)
	server.mappingMtx.Lock()

	// Get Default Metrics only for master database
	if !e.disableDefaultMetrics && server.master {
		server.metricMap = makeDescMap(semanticVersion, server.labels, e.builtinMetricMaps, server.logger, e.metricPrefix)
		server.queryOverrides = makeQueryOverrideMap(semanticVersion, queryOverrides, server.logger)
	} else {
		server.metricMap = make(map[string]MetricMapNamespace)
		server.queryOverrides = make(map[string]string)
	}

	// server.lastMapVersion[e.resolutionEnabled] = semanticVersion

	if e.userQueriesPath[HR] != "" || e.userQueriesPath[MR] != "" || e.userQueriesPath[LR] != "" {
		// Clear the metric while reload
		e.userQueriesError.Reset()
	}

	e.loadCustomQueries(e.resolutionEnabled, semanticVersion, server)

	server.mappingMtx.Unlock()

	// Output the version as a special metric only for master database
	versionDesc := prometheus.NewDesc(fmt.Sprintf("%s_%s", namespace, staticLabelName),
		"Version string as reported by postgres", []string{"version", "short_version"}, server.labels)

	if !e.disableDefaultMetrics && server.master {
		ch <- prometheus.MustNewConstMetric(versionDesc,
			prometheus.UntypedValue, 1, versionString, semanticVersion.String())
	}
	return nil
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric) {
	defer func(begun time.Time) {
		e.duration.Set(time.Since(begun).Seconds())
	}(time.Now())

	e.totalScrapes.Inc()

	dsns := e.dsn
	if e.autoDiscoverDatabases {
		dsns = e.discoverDatabaseDSNs()
	}

	var errorsCount atomic.Int32
	var connectionErrorsCount atomic.Int32
	var wg sync.WaitGroup

	for _, dsn := range dsns {
		dsn := dsn
		wg.Add(1)
		go func() {
			defer wg.Done()

			if e.connSema != nil {
				if err := e.connSema.Acquire(e.ctx, 1); err != nil {
					e.logger.Warn("Failed to acquire semaphore", "err", err)
					return
				}
				defer e.connSema.Release(1)
			}
			if err := e.scrapeDSN(ch, dsn); err != nil {
				errorsCount.Add(1)

				e.logger.Error("error scraping dsn", "err", err, "dsn", loggableDSN(dsn))

				if _, ok := err.(*ErrorConnectToServer); ok {
					connectionErrorsCount.Add(1)
				}
			}
		}()
	}

	wg.Wait()

	switch {
	case int(connectionErrorsCount.Load()) >= len(dsns):
		e.psqlUp.Set(0)
	default:
		e.psqlUp.Set(1) // Didn't fail, can mark connection as up for this scrape.
	}

	switch errorsCount.Load() {
	case 0:
		e.error.Set(0)
	default:
		e.error.Set(1)
	}
}
