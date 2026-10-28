package metrics

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Instrument with prometheus middleware to capture request metrics.
func PrometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		clusterName := params["id"]

		clusterNameLabel := prometheus.Labels{"managed_cluster_name": clusterName}
		if isValidationRequest(r.URL.Path) {
			// Add the managed_cluster_name label to metrics.
			curriedValidationCount, _ := ValidationRequestCount.CurryWith(clusterNameLabel)
			promhttp.InstrumentHandlerDuration(ValidationRequestDuration,
				promhttp.InstrumentHandlerCounter(curriedValidationCount, next)).ServeHTTP(w, r)
			return
		}

		// Add the managed_cluster_name label to metrics.
		curriedRequestCount, _ := RequestCount.CurryWith(clusterNameLabel)

		// Instrument and serve.
		promhttp.InstrumentHandlerInFlight(RequestsInFlight,
			promhttp.InstrumentHandlerDuration(RequestDuration,
				promhttp.InstrumentHandlerCounter(curriedRequestCount, next))).ServeHTTP(w, r)
	})
}

func isValidationRequest(path string) bool {
	path = strings.TrimSuffix(path, "/")
	return strings.HasSuffix(path, "/validation")
}
