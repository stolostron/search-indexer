// Copyright Contributors to the Open Cluster Management project

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	PromRegistry = prometheus.NewRegistry()

	RequestCount = promauto.With(PromRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "search_indexer_request_count",
		Help: "Total requests received by the search indexer (from managed clusters).",
	}, []string{"managed_cluster_name"})

	RequestDuration = promauto.With(PromRegistry).NewHistogramVec(prometheus.HistogramOpts{
		Name:    "search_indexer_request_duration",
		Help:    "Time (seconds) the search indexer takes to process a request (from managed cluster).",
		Buckets: []float64{.25, .5, 1, 1.5, 2, 3, 5, 10},
	}, []string{"code"})

	RequestsInFlight = promauto.With(PromRegistry).NewGauge(prometheus.GaugeOpts{
		Name: "search_indexer_requests_in_flight",
		Help: "Total requests the search indexer is processing at a given time.",
	})

	RequestSize = promauto.With(PromRegistry).NewHistogram(prometheus.HistogramOpts{
		Name:    "search_indexer_request_size",
		Help:    "Total changes (add, update, delete) in the search indexer request (from managed cluster).",
		Buckets: []float64{50, 100, 200, 500, 5000, 10000, 25000, 50000, 100000, 200000},
	})

	// DBResourceEventSent counts the number of resource operations sent to the database,
	// broken down by operation type, resource kind, and managed cluster name.
	// Use rate() or irate() in PromQL to obtain operations per minute/second.
	// Example: rate(search_indexer_db_resource_event_count[1m]) * 60
	// Labels:
	//   operation  - "insert", "update", or "delete" (matches the DB operation name)
	//   kind       - Kubernetes resource kind (e.g. "Pod", "Deployment"); empty for bulk resync deletes
	//   cluster    - name of the cluster that originated the sync event
	DBResourceEventSent = promauto.With(PromRegistry).NewCounterVec(prometheus.CounterOpts{
		Name: "search_indexer_db_resource_event_count",
		Help: "Number of resource DB events (insert, update, delete) sent to the database, by operation, kind, and cluster.",
	}, []string{"operation", "kind", "cluster"})
)
