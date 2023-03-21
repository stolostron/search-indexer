package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Instrument with prom middleware to capture request metrics.
func PrometheusMiddleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		clusterName := params["id"]
		curriedSyncDuration, _ := SyncRequestDuration.CurryWith(prometheus.Labels{"cluster": clusterName})
		curriedSyncSize, _ := SyncRequestSize.CurryWith(prometheus.Labels{"cluster": clusterName})
		curriedSyncCount, _ := SyncRequestCount.CurryWith(prometheus.Labels{"cluster": clusterName})

		// Instrument and serve
		promhttp.InstrumentHandlerDuration(curriedSyncDuration,
			promhttp.InstrumentHandlerRequestSize(curriedSyncSize,
				promhttp.InstrumentHandlerCounter(curriedSyncCount, next))).ServeHTTP(w, r)
	})
}
