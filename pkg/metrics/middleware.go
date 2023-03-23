package metrics

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Instrument with prometheus middleware to capture request metrics.
func PrometheusMiddleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		clusterName := params["id"]

		// Add the managed_cluster_name label to metrics.
		clusterNameLabel := prometheus.Labels{"managed_cluster_name": clusterName}
		curriedSyncCount, _ := RequestCount.CurryWith(clusterNameLabel)
		// curriedRequestSummary, _ := RequestSummary.CurryWith(clusterNameLabel)

		// Instrument and serve.
		promhttp.InstrumentHandlerInFlight(RequestsInFlight,
			promhttp.InstrumentHandlerDuration(RequestDuration,
				// promhttp.InstrumentHandlerDuration(curriedRequestSummary,
				promhttp.InstrumentHandlerCounter(curriedSyncCount, next))).ServeHTTP(w, r)
	})
}
