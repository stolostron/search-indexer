// Copyright Contributors to the Open Cluster Management project

package server

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var HttpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name: "acm_search_indexer_http_duration_seconds",
	Help: "Time the search indexer takes to process HTTP requests.",
}, []string{"cluster", "endpoint"})

var (
	OpsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "acm_search_indexer_hits_total",
		Help: "The total number of incoming requests to the search indexer.",
	}, []string{"cluster", "endpoint"})
)
