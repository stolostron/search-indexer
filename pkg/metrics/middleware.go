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

		path := "/sync"
		if isValidationRequest(r.URL.Path) {
			path = "/validation"
		}

		// Keep path labels bounded; the full URL contains the cluster name.
		requestLabels := prometheus.Labels{"managed_cluster_name": clusterName, "path": path}
		curriedRequestCount, _ := RequestCount.CurryWith(requestLabels)
		curriedRequestDuration, _ := RequestDuration.CurryWith(prometheus.Labels{"path": path})

		// Instrument and serve.
		promhttp.InstrumentHandlerInFlight(RequestsInFlight.WithLabelValues(path),
			promhttp.InstrumentHandlerDuration(curriedRequestDuration,
				promhttp.InstrumentHandlerCounter(curriedRequestCount, next))).ServeHTTP(w, r)
	})
}

func isValidationRequest(path string) bool {
	path = strings.TrimSuffix(path, "/")
	return strings.HasSuffix(path, "/validation")
}
