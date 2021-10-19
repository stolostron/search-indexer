package server

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var HttpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name: "acm_searchaggr_http_duration_seconds",
	Help: "Duration the search aggregator takes to process HTTP requests.",
}, []string{"cluster", "endpoint"})

var (
	OpsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "acm_searchaggr_hits_total",
		Help: "The total number of incoming requests to the search aggregator.",
	}, []string{"cluster", "endpoint"})
)

// var HttpSummary = promauto.NewHistogramVec(prometheus.HistogramOpts{
// 	Name: "acm_searchaggr_http_summary",
// 	Help: "Duration the search aggregator takes to process HTTP requests.",
// }, []string{"cluster", "endpoint"})
