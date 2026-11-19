package exporter

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blang/semver/v4"
)

// MetricResolution selects which custom query directory an exporter loads (Percona).
type MetricResolution string

const (
	LR MetricResolution = "lr"
	MR MetricResolution = "mr"
	HR MetricResolution = "hr"
)

func (e *Exporter) loadCustomQueries(res MetricResolution, version semver.Version, server *Server) {
	if e.userQueriesPath[res] != "" {
		fi, err := os.ReadDir(e.userQueriesPath[res])
		if err != nil {
			e.logger.Error(fmt.Sprintf("failed read dir %q for custom query", e.userQueriesPath[res]),
				"err", err)
			return
		}
		e.logger.Debug(fmt.Sprintf("reading dir %q for custom query", e.userQueriesPath[res]))

		for _, v := range fi {
			if v.IsDir() {
				continue
			}

			if filepath.Ext(v.Name()) == ".yml" || filepath.Ext(v.Name()) == ".yaml" {
				path := filepath.Join(e.userQueriesPath[res], v.Name())
				e.addCustomQueriesFromFile(path, version, server)
			}
		}
	}
}

func (e *Exporter) addCustomQueriesFromFile(path string, version semver.Version, server *Server) {
	// Calculate the hashsum of the useQueries
	userQueriesData, err := os.ReadFile(path)
	if err != nil {
		e.logger.Error("Failed to reload user queries:"+path, "err", err)
		e.userQueriesError.WithLabelValues(path, "").Set(1)
		return
	}

	hashsumStr := fmt.Sprintf("%x", sha256.Sum256(userQueriesData))

	if err := addQueries(userQueriesData, version, server, e.metricPrefix); err != nil {
		e.logger.Error("Failed to reload user queries:"+path, "err", err)
		e.userQueriesError.WithLabelValues(path, hashsumStr).Set(1)
		return
	}

	// Mark user queries as successfully loaded
	e.userQueriesError.WithLabelValues(path, hashsumStr).Set(0)
}
