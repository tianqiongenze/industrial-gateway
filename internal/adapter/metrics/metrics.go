// Package metrics exposes a Prometheus registry + counters for ingest events.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// IngestCounter counts ingested readings by source + status.
	IngestCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_ingest_total", Help: "Number of ingested readings",
	}, []string{"source", "status"})

	// CollectorErrors counts collector failures by collector name.
	CollectorErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gateway_collector_errors_total", Help: "Collector failures",
	}, []string{"collector"})
)
