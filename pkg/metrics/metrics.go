// Copyright Contributors to the Open Cluster Management project

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	PromRegistry = prometheus.NewRegistry()

	SyncRequestCount = promauto.With(PromRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "search_indexer_request_count",
		Help: "Total incoming sync requests to the search indexer from managed clusters.",
	}, []string{"managed_cluster_name"})

	SyncRequestDuration = promauto.With(PromRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "search_indexer_request_duration",
		Help:    "Time (seconds) the search indexer takes to process a sync request from managed clusters.",
		Buckets: []float64{.25, .5, 1, 1.5, 2, 3, 5, 10},
	}, []string{"code"})

	RequestsInFlight = promauto.With(PromRegistry).NewGauge(prometheus.GaugeOpts{
		Name: "search_indexer_requests_in_flight",
		Help: "Total sync requests being processed.",
	})

	// Experimenting.

	SyncRequestSize = promauto.With(PromRegistry).NewHistogram(prometheus.HistogramOpts{
		Name:    "search_indexer_request_size",
		Help:    "Number of changes processed (add, update, delete) in a sync request from managed cluster.",
		Buckets: []float64{100, 200, 5000, 10000, 25000, 50000, 100000, 200000},
	})

	RequestSummary = promauto.With(PromRegistry).NewSummaryVec(prometheus.SummaryOpts{
		Name: "search_indexer_requests_summary",
		Help: "TODO...",
	}, []string{"code", "managed_cluster_name"})
)
