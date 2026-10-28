package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func Test_PrometheusInstrumentation(t *testing.T) {
	RequestCount.Reset()
	RequestDuration.Reset()
	RequestsInFlight.Set(0)
	ValidationRequestCount.Reset()
	ValidationRequestDuration.Reset()

	inFlight := map[string]float64{}
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metric := &dto.Metric{}
		_ = RequestsInFlight.Write(metric)
		inFlight[r.URL.Path] = metric.GetGauge().GetValue()
	})
	promMiddle := PrometheusMiddleware(httpHandler)
	for _, request := range []struct {
		method string
		path   string
	}{{http.MethodPost, "sync"}, {http.MethodGet, "validation"}} {
		req := httptest.NewRequest(request.method, "https://localhost:3010/aggregator/clusters/clusterA/"+request.path, nil)
		res := httptest.NewRecorder()
		promMiddle.ServeHTTP(res, req)
	}

	// Validate the collected metrics.

	collectedMetrics, _ := PromRegistry.Gather() // use the prometheus registry to confirm metrics have been scraped.
	assert.Equal(t, 6, len(collectedMetrics))    // Validate total metrics collected.

	// METRIC 1:  search_indexer_request_count
	assert.Equal(t, "search_indexer_request_count", collectedMetrics[0].GetName())
	assert.Len(t, collectedMetrics[0].Metric, 1)
	assert.Equal(t, 1, len(collectedMetrics[0].Metric[0].GetLabel()))
	assert.Equal(t, "managed_cluster_name", *collectedMetrics[0].Metric[0].GetLabel()[0].Name)
	assert.Equal(t, 1.0, collectedMetrics[0].GetMetric()[0].GetCounter().GetValue())
	// Note: to validate cluster name we need to mock the mux router, which adds too much complexity to this test.
	// assert.Equal(t, "clusterA", *collectedMetrics[0].Metric[0].GetLabel()[0].Value)

	// METRIC 2: search_indexer_request_duration
	assert.Equal(t, "search_indexer_request_duration", collectedMetrics[1].GetName())
	assert.Equal(t, 1, len(collectedMetrics[1].Metric[0].GetLabel()))
	assert.Equal(t, "code", *collectedMetrics[1].Metric[0].GetLabel()[0].Name)
	assert.Equal(t, "200", *collectedMetrics[1].Metric[0].GetLabel()[0].Value)
	assert.Equal(t, 1, len(collectedMetrics[1].GetMetric()))
	assert.Equal(t, uint64(1), collectedMetrics[1].GetMetric()[0].GetHistogram().GetSampleCount())

	// METRIC 3: search_indexer_request_size
	assert.Equal(t, "search_indexer_request_size", collectedMetrics[2].GetName())
	assert.Equal(t, 0.0, collectedMetrics[2].GetMetric()[0].GetCounter().GetValue())

	// METRIC 4: search_indexer_requests_in_flight
	assert.Equal(t, "search_indexer_requests_in_flight", collectedMetrics[3].GetName())
	assert.Equal(t, 0.0, collectedMetrics[3].GetMetric()[0].GetGauge().GetValue())

	// METRIC 5: search_indexer_validation_request_count
	assert.Equal(t, "search_indexer_validation_request_count", collectedMetrics[4].GetName())
	assert.Equal(t, 1.0, collectedMetrics[4].GetMetric()[0].GetCounter().GetValue())

	// METRIC 6: search_indexer_validation_request_duration
	assert.Equal(t, "search_indexer_validation_request_duration", collectedMetrics[5].GetName())
	assert.Equal(t, uint64(1), collectedMetrics[5].GetMetric()[0].GetHistogram().GetSampleCount())

	assert.Equal(t, 1.0, inFlight["/aggregator/clusters/clusterA/sync"])
	assert.Equal(t, 0.0, inFlight["/aggregator/clusters/clusterA/validation"])
}

func TestIsValidationRequest(t *testing.T) {
	assert.True(t, isValidationRequest("/aggregator/clusters/clusterA/validation/"))
	assert.False(t, isValidationRequest("/aggregator/clusters/clusterA/sync"))
}
