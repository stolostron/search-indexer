package metrics

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Instrument with prom middleware to capture request metrics.
func PrometheusMiddleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		clusterName := params["id"]

		// Add the managed_cluster_name label to metrics.
		// FUTURE: use managed_cluster_id instead of name.
		clusterNameLabel := prometheus.Labels{"managed_cluster_name": clusterName}
		curriedSyncCount, _ := SyncRequestCount.CurryWith(clusterNameLabel)

		// Instrument and serve.
		promhttp.InstrumentHandlerDuration(SyncRequestDuration,
			promhttp.InstrumentHandlerRequestSize(SyncRequestSize,
				promhttp.InstrumentHandlerCounter(curriedSyncCount, next))).ServeHTTP(w, r)
	})
}
