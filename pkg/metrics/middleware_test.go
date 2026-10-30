package metrics

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gorilla/mux"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_PrometheusInstrumentation(t *testing.T) {
	RequestCount.Reset()
	RequestDuration.Reset()
	RequestsInFlight.Reset()

	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, path := range []string{"/sync", "/validation"} {
			metric := &dto.Metric{}
			require.NoError(t, RequestsInFlight.WithLabelValues(path).Write(metric))
			expected := 0.0
			if r.URL.Path == "/aggregator/clusters/clusterA"+path {
				expected = 1.0
			}
			assert.Equal(t, expected, metric.GetGauge().GetValue(), "in-flight requests for %s", path)
		}
	})
	promMiddle := PrometheusMiddleware(httpHandler)
	for _, request := range []struct {
		method string
		path   string
	}{{http.MethodPost, "sync"}, {http.MethodGet, "validation"}} {
		req := httptest.NewRequest(request.method, "https://localhost:3010/aggregator/clusters/clusterA/"+request.path, nil)
		req = mux.SetURLVars(req, map[string]string{"id": "clusterA"})
		res := httptest.NewRecorder()
		promMiddle.ServeHTTP(res, req)
	}

	// Validate the collected metrics.

	collectedMetrics, err := PromRegistry.Gather() // use the prometheus registry to confirm metrics have been scraped.
	require.NoError(t, err)
	require.Len(t, collectedMetrics, 4) // Validation uses the same metric families as sync.

	// METRIC 1:  search_indexer_request_count
	assert.Equal(t, "search_indexer_request_count", collectedMetrics[0].GetName())
	require.Len(t, collectedMetrics[0].Metric, 2)
	for i, path := range []string{"/sync", "/validation"} {
		metric := collectedMetrics[0].Metric[i]
		require.Len(t, metric.GetLabel(), 2)
		assert.Equal(t, "managed_cluster_name", metric.GetLabel()[0].GetName())
		assert.Equal(t, "clusterA", metric.GetLabel()[0].GetValue())
		assert.Equal(t, "path", metric.GetLabel()[1].GetName())
		assert.Equal(t, path, metric.GetLabel()[1].GetValue())
		assert.Equal(t, 1.0, metric.GetCounter().GetValue())
	}

	// METRIC 2: search_indexer_request_duration
	assert.Equal(t, "search_indexer_request_duration", collectedMetrics[1].GetName())
	require.Len(t, collectedMetrics[1].Metric, 2)
	for i, path := range []string{"/sync", "/validation"} {
		metric := collectedMetrics[1].Metric[i]
		require.Len(t, metric.GetLabel(), 2)
		assert.Equal(t, "code", metric.GetLabel()[0].GetName())
		assert.Equal(t, "200", metric.GetLabel()[0].GetValue())
		assert.Equal(t, "path", metric.GetLabel()[1].GetName())
		assert.Equal(t, path, metric.GetLabel()[1].GetValue())
		assert.Equal(t, uint64(1), metric.GetHistogram().GetSampleCount())
	}

	// METRIC 3: search_indexer_request_size
	assert.Equal(t, "search_indexer_request_size", collectedMetrics[2].GetName())
	assert.Equal(t, 0.0, collectedMetrics[2].GetMetric()[0].GetCounter().GetValue())

	// METRIC 4: search_indexer_requests_in_flight
	assert.Equal(t, "search_indexer_requests_in_flight", collectedMetrics[3].GetName())
	require.Len(t, collectedMetrics[3].Metric, 2)
	for i, path := range []string{"/sync", "/validation"} {
		metric := collectedMetrics[3].Metric[i]
		require.Len(t, metric.GetLabel(), 1)
		assert.Equal(t, "path", metric.GetLabel()[0].GetName())
		assert.Equal(t, path, metric.GetLabel()[0].GetValue())
		assert.Equal(t, 0.0, metric.GetGauge().GetValue())
	}
}

func TestValidationInstrumentationStatus(t *testing.T) {
	for _, code := range []int{http.StatusTooManyRequests, http.StatusInternalServerError} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			RequestCount.Reset()
			RequestDuration.Reset()
			RequestsInFlight.Reset()
			req := httptest.NewRequest(http.MethodGet, "/aggregator/clusters/clusterB/validation", nil)
			req = mux.SetURLVars(req, map[string]string{"id": "clusterB"})
			handler := PrometheusMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
			}))
			handler.ServeHTTP(httptest.NewRecorder(), req)

			metrics, err := PromRegistry.Gather()
			require.NoError(t, err)
			require.Len(t, metrics, 4)
			require.Len(t, metrics[0].Metric, 1)
			assert.Equal(t, "clusterB", metrics[0].Metric[0].GetLabel()[0].GetValue())
			assert.Equal(t, "/validation", metrics[0].Metric[0].GetLabel()[1].GetValue())
			assert.Equal(t, 1.0, metrics[0].Metric[0].GetCounter().GetValue())
			require.Len(t, metrics[1].Metric, 1)
			assert.Equal(t, strconv.Itoa(code), metrics[1].Metric[0].GetLabel()[0].GetValue())
			assert.Equal(t, "/validation", metrics[1].Metric[0].GetLabel()[1].GetValue())
			assert.Equal(t, uint64(1), metrics[1].Metric[0].GetHistogram().GetSampleCount())
			assert.Equal(t, 0.0, metrics[3].Metric[0].GetGauge().GetValue())
		})
	}
}

func TestIsValidationRequest(t *testing.T) {
	assert.True(t, isValidationRequest("/aggregator/clusters/clusterA/validation/"))
	assert.False(t, isValidationRequest("/aggregator/clusters/clusterA/sync"))
}
