package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/percona/postgres_exporter/collector"
	"github.com/percona/postgres_exporter/exporter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/semaphore"
	"log/slog"
)

var (
	collectCustomQueryLr          = kingpin.Flag("collect.custom_query.lr", "Enable custom queries with low resolution directory.").Default("false").Envar("PG_EXPORTER_EXTEND_QUERY_LR").Bool()
	collectCustomQueryMr          = kingpin.Flag("collect.custom_query.mr", "Enable custom queries with medium resolution directory.").Default("false").Envar("PG_EXPORTER_EXTEND_QUERY_MR").Bool()
	collectCustomQueryHr          = kingpin.Flag("collect.custom_query.hr", "Enable custom queries with high resolution directory.").Default("false").Envar("PG_EXPORTER_EXTEND_QUERY_HR").Bool()
	collectCustomQueryLrDirectory = kingpin.Flag("collect.custom_query.lr.directory", "Path to custom queries with low resolution directory.").Envar("PG_EXPORTER_EXTEND_QUERY_LR_PATH").String()
	collectCustomQueryMrDirectory = kingpin.Flag("collect.custom_query.mr.directory", "Path to custom queries with medium resolution directory.").Envar("PG_EXPORTER_EXTEND_QUERY_MR_PATH").String()
	collectCustomQueryHrDirectory = kingpin.Flag("collect.custom_query.hr.directory", "Path to custom queries with high resolution directory.").Envar("PG_EXPORTER_EXTEND_QUERY_HR_PATH").String()

	maxConnections = kingpin.Flag("max-connections", "Maximum number of connections to use").Default("5").Envar("PG_EXPORTER_MAX_CONNECTIONS").Int64()
)

// Handler returns a http.Handler that serves metrics. Can be used instead of
// run for hooking up custom HTTP servers.
func Handler(logger *slog.Logger, dsns []string, connSema *semaphore.Weighted, globalCollectors map[string]prometheus.Collector) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seconds, err := strconv.Atoi(r.Header.Get("X-Prometheus-Scrape-Timeout-Seconds"))
		// To support also older ones vmagents.
		if err != nil {
			seconds = 10
		}

		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(seconds)*time.Second)
		defer cancel()

		filters := r.URL.Query()["collect[]"]
		logger.Debug("Collect query", "filters", filters)

		var f Filters
		if len(filters) == 0 {
			f.EnableAllCollectors = true
		} else {
			for _, filter := range filters {
				switch filter {
				case "standard.process":
					f.EnableProcessCollector = true
				case "standard.go":
					f.EnableGoCollector = true
				case "version":
					f.EnableVersionCollector = true
				case "exporter":
					f.EnableDefaultCollector = true
				case "custom_query.hr":
					f.EnableHRCollector = true
				case "custom_query.mr":
					f.EnableMRCollector = true
				case "custom_query.lr":
					f.EnableLRCollector = true
				case "postgres":
					f.EnablePostgresCollector = true
				}
			}
		}

		registry := makeRegistry(ctx, dsns, connSema, globalCollectors, f)

		// Delegate http serving to Prometheus client library, which will call collector.Collect.
		h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{
			ErrorHandling: promhttp.ContinueOnError,
			ErrorLog:      slog.NewLogLogger(logger.Handler(), slog.LevelError),
		})

		h.ServeHTTP(w, r)
	})
}

// Filters is a struct to enable or disable collectors.
type Filters struct {
	EnableAllCollectors     bool
	EnableLRCollector       bool
	EnableMRCollector       bool
	EnableHRCollector       bool
	EnableDefaultCollector  bool
	EnableGoCollector       bool
	EnableVersionCollector  bool
	EnableProcessCollector  bool
	EnablePostgresCollector bool
}

// makeRegistry creates a new prometheus registry with default and percona exporters.
func makeRegistry(ctx context.Context, dsns []string, connSema *semaphore.Weighted, globalCollectors map[string]prometheus.Collector, filters Filters) *prometheus.Registry {
	registry := prometheus.NewRegistry()

	excludedDatabases := strings.Split(*excludeDatabases, ",")
	logger.Info("Excluded databases", "databases", fmt.Sprintf("%v", excludedDatabases))

	queriesPath := map[exporter.MetricResolution]string{
		exporter.HR: *collectCustomQueryHrDirectory,
		exporter.MR: *collectCustomQueryMrDirectory,
		exporter.LR: *collectCustomQueryLrDirectory,
	}

	opts := []exporter.ExporterOpt{
		exporter.AutoDiscoverDatabases(*autoDiscoverDatabases),
		exporter.ExcludeDatabases(excludedDatabases),
		exporter.WithConnectionsSemaphore(connSema),
		exporter.WithContext(ctx),
		exporter.WithMetricPrefix(*metricPrefix),
	}

	if filters.EnableAllCollectors || filters.EnableDefaultCollector {
		defaultExporter := exporter.NewExporter(dsns, logger, append(
			opts,
			exporter.CollectorName("exporter"),
			exporter.WithConstantLabels(*constantLabelsList), // This option depends on collectors name, so keep it after exporter.CollectorName option
			exporter.DisableDefaultMetrics(*disableDefaultMetrics),
			exporter.IncludeDatabases(*includeDatabases),
		)...)
		registry.MustRegister(defaultExporter)
	}

	if filters.EnableAllCollectors || filters.EnableHRCollector {
		hrExporter := exporter.NewExporter(dsns, logger,
			append(opts,
				exporter.CollectorName("custom_query.hr"),
				exporter.WithConstantLabels(*constantLabelsList), // This option depends on collectors name, so keep it after exporter.CollectorName option
				exporter.WithUserQueriesEnabled(exporter.HR),
				exporter.WithEnabled(*collectCustomQueryHr),
				exporter.DisableDefaultMetrics(true),
				exporter.WithUserQueriesPath(queriesPath),
			)...)
		registry.MustRegister(hrExporter)

	}

	if filters.EnableAllCollectors || filters.EnableMRCollector {
		mrExporter := exporter.NewExporter(dsns, logger,
			append(opts,
				exporter.CollectorName("custom_query.mr"),
				exporter.WithConstantLabels(*constantLabelsList), // This option depends on collectors name, so keep it after exporter.CollectorName option
				exporter.WithUserQueriesEnabled(exporter.MR),
				exporter.WithEnabled(*collectCustomQueryMr),
				exporter.DisableDefaultMetrics(true),
				exporter.WithUserQueriesPath(queriesPath),
			)...)
		registry.MustRegister(mrExporter)
	}

	if filters.EnableAllCollectors || filters.EnableLRCollector {
		lrExporter := exporter.NewExporter(dsns, logger,
			append(opts,
				exporter.CollectorName("custom_query.lr"),
				exporter.WithConstantLabels(*constantLabelsList), // This option depends on collectors name, so keep it after exporter.CollectorName option
				exporter.WithUserQueriesEnabled(exporter.LR),
				exporter.WithEnabled(*collectCustomQueryLr),
				exporter.DisableDefaultMetrics(true),
				exporter.WithUserQueriesPath(queriesPath),
			)...)
		registry.MustRegister(lrExporter)
	}

	if filters.EnableAllCollectors || filters.EnableGoCollector {
		registry.MustRegister(globalCollectors["standard.go"])
	}

	if filters.EnableAllCollectors || filters.EnableProcessCollector {
		registry.MustRegister(globalCollectors["standard.process"])
	}

	if filters.EnableAllCollectors || filters.EnableVersionCollector {
		registry.MustRegister(globalCollectors["version"])
	}

	if filters.EnableAllCollectors || filters.EnablePostgresCollector {
		// This chunk moved here from main.go
		// TODO(@sysadmind): Remove this with multi-target support. We are removing multiple DSN support
		dsn := ""
		if len(dsns) > 0 {
			dsn = dsns[0]
		}

		pe, err := collector.NewPostgresCollector(
			logger,
			excludedDatabases,
			dsn,
			[]string{},
			collector.WithContext(ctx),
			collector.WithConnectionsSemaphore(connSema),
			collector.WithCollectionTimeout(*collectionTimeout),
		)
		if err != nil {
			logger.Error("Failed to create PostgresCollector", "err", err.Error())
		} else {
			registry.MustRegister(pe)
		}
	}

	return registry
}
