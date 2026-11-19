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
	"errors"
	"fmt"
	"log/slog"

	"github.com/blang/semver/v4"
	"gopkg.in/yaml.v2"

	"github.com/percona/postgres_exporter/distribution"
)

const (
	// '!' is a reserved character indicating that the query is not supported on Aurora and should be skipped for Aurora instances.
	notSupportedByAurora = "!"
)

// UserQuery represents a user defined query, including support for Aurora, if needed
type UserQuery struct {
	Query        string    `yaml:"query"`         // Standard query
	QueryAurora  string    `yaml:"query_aurora"`  // Aurora specific query
	Metrics      []Mapping `yaml:"metrics"`       // Metrics to be collected
	Master       bool      `yaml:"master"`        // Querying only for master database
	CacheSeconds uint64    `yaml:"cache_seconds"` // Number of seconds to cache the namespace result metrics for.
	RunOnServer  string    `yaml:"runonserver"`   // Querying to run on which server version
}

// UserQueries represents a set of UserQuery objects
type UserQueries map[string]UserQuery

// OverrideQuery 's are run in-place of simple namespace look ups, and provide
// advanced functionality. But they have a tendency to postgres version specific.
// There aren't too many versions, so we simply store customized versions using
// the semver matching we do for columns.
type OverrideQuery struct {
	versionRange semver.Range
	query        string
}

// Overriding queries for namespaces above.
// TODO: validate this is a closed set in tests, and there are no overlaps
var queryOverrides = map[string][]OverrideQuery{
	"pg_lock_conflicts": {
		{
			semver.MustParseRange(">0.0.0"),
			`SELECT blockinga.pid AS blocking_pid, count(*) as count
			FROM pg_catalog.pg_locks blockedl
			JOIN pg_stat_activity blockeda ON blockedl.pid = blockeda.pid
			JOIN pg_catalog.pg_locks blockingl ON(blockingl.transactionid=blockedl.transactionid
			  AND blockedl.pid != blockingl.pid)
			JOIN pg_stat_activity blockinga ON blockingl.pid = blockinga.pid
			WHERE NOT blockedl.granted
			group by blocking_pid`,
		},
	},
}

// Convert the query override file to the version-specific query override file
// for the exporter.
func makeQueryOverrideMap(pgVersion semver.Version, queryOverrides map[string][]OverrideQuery, logger *slog.Logger) map[string]string {
	resultMap := make(map[string]string)
	for name, overrideDef := range queryOverrides {
		// Find a matching semver. We make it an error to have overlapping
		// ranges at test-time, so only 1 should ever match.
		matched := false
		for _, queryDef := range overrideDef {
			if queryDef.versionRange(pgVersion) {
				resultMap[name] = queryDef.query
				matched = true
				break
			}
		}
		if !matched {
			logger.Warn("No query matched override, disabling metric space", "name", name)
			resultMap[name] = ""
		}
	}

	return resultMap
}

func parseUserQueries(content []byte, dist string, logger *slog.Logger) (map[string]intermediateMetricMap, map[string]string, error) {
	var userQueries UserQueries

	err := yaml.Unmarshal(content, &userQueries)
	if err != nil {
		return nil, nil, err
	}

	// Stores the loaded map representation
	metricMaps := make(map[string]intermediateMetricMap)
	newQueryOverrides := make(map[string]string)

	for metric, specs := range userQueries {
		logger.Debug("New user metric namespace from YAML metric", "metric", metric, "cache_seconds", specs.CacheSeconds)

		// Query selection logic:
		// For Aurora: use query_aurora if defined and not empty, otherwise use query if defined and not empty.
		// If query_aurora is set to '!', skip this query for Aurora (not supported).
		// For standard (non-Aurora): always use query.
		switch dist {
		case distribution.Aurora:
			if specs.QueryAurora != "" {
				if specs.QueryAurora == notSupportedByAurora {
					continue
				}
				newQueryOverrides[metric] = specs.QueryAurora
			} else {
				newQueryOverrides[metric] = specs.Query
			}
		default:
			newQueryOverrides[metric] = specs.Query
		}
		metricMap, ok := metricMaps[metric]
		if !ok {
			// Namespace for metric not found - add it.
			newMetricMap := make(map[string]ColumnMapping)
			metricMap = intermediateMetricMap{
				columnMappings: newMetricMap,
				master:         specs.Master,
				cacheSeconds:   specs.CacheSeconds,
			}
			metricMaps[metric] = metricMap
		}
		for _, metric := range specs.Metrics {
			for name, mappingOption := range metric {
				var columnMapping ColumnMapping
				tmpUsage, _ := stringToColumnUsage(mappingOption.Usage)
				columnMapping.usage = tmpUsage
				columnMapping.description = mappingOption.Description

				// TODO: we should support cu
				columnMapping.mapping = nil
				// Should we support this for users?
				columnMapping.supportedVersions = nil

				metricMap.columnMappings[name] = columnMapping
			}
		}
	}
	return metricMaps, newQueryOverrides, nil
}

// Add queries to the builtinMetricMaps and queryOverrides maps. Added queries do not
// respect version requirements, because it is assumed that the user knows
// what they are doing with their version of postgres.
//
// This function modifies metricMap and queryOverrideMap to contain the new
// queries.
// TODO: test code for all cu.
// TODO: the YAML this supports is "non-standard" - we should move away from it.
func addQueries(content []byte, pgVersion semver.Version, server *Server, metricPrefix string) error {
	metricMaps, newQueryOverrides, err := parseUserQueries(content, server.distribution, server.logger)
	if err != nil {
		return err
	}
	// Convert the loaded metric map into exporter representation
	partialExporterMap := makeDescMap(pgVersion, server.labels, metricMaps, server.logger, metricPrefix)

	// Merge the two maps (which are now quite flatteend)
	for k, v := range partialExporterMap {
		_, found := server.metricMap[k]
		if found {
			server.logger.Debug("Overriding metric from user YAML file", "metric", k)
		} else {
			server.logger.Debug("Adding new metric from user YAML file", "metric", k)
		}
		server.metricMap[k] = v
	}

	// Merge the query override map
	for k, v := range newQueryOverrides {
		_, found := server.queryOverrides[k]
		if found {
			server.logger.Debug("Overriding query override from user YAML file", "query_override", k)
		} else {
			server.logger.Debug("Adding new query override from user YAML file", "query_override", k)
		}
		server.queryOverrides[k] = v
	}
	return nil
}

func queryDatabases(ctx context.Context, server *Server) ([]string, error) {
	rows, err := server.db.QueryContext(ctx, "SELECT datname FROM pg_database WHERE datallowconn = true AND datistemplate = false AND datname != current_database() AND has_database_privilege(current_user, datname, 'connect')")
	if err != nil {
		return nil, fmt.Errorf("Error retrieving databases: %v", err)
	}
	defer rows.Close() // nolint: errcheck

	var databaseName string
	result := make([]string, 0)
	for rows.Next() {
		err = rows.Scan(&databaseName)
		if err != nil {
			return nil, errors.New(fmt.Sprintln("Error retrieving rows:", err))
		}
		result = append(result, databaseName)
	}

	return result, nil
}
