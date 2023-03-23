package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_PrometheusInstrumentation(t *testing.T) {
	// Create a mock resquest to pass to handler.
	req := httptest.NewRequest("POST", "https://localhost:3010/aggregator/clusters/clusterA/sync", nil)
	res := httptest.NewRecorder()

	// Execute middleware function.
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	promMiddle := PrometheusMiddleware(httpHandler)
	promMiddle.ServeHTTP(res, req)

	// Validate result.

	collectedMetrics, _ := PromRegistry.Gather() // use the prometheus registry to confirm metrics have been scraped.
	assert.Equal(t, 4, len(collectedMetrics))    // Validate 3 metrics are collected.
	t.Logf("\n\n metrics: \n%+v", collectedMetrics)

	// METRIC 1:  search_indexer_request_count
	assert.Equal(t, "search_indexer_request_count", collectedMetrics[0].GetName())
	assert.Equal(t, 1, len(collectedMetrics[0].Metric[0].GetLabel()))
	assert.Equal(t, "managed_cluster_name", *collectedMetrics[0].Metric[0].GetLabel()[0].Name)
	// To validate cluster name we would need to mock the mux router, which adds too much complexity to this test.
	// assert.Equal(t, "clusterA", *collectedMetrics[0].Metric[0].GetLabel()[0].Value)
	assert.Equal(t, 1.0, collectedMetrics[0].GetMetric()[0].GetCounter().GetValue())

	// METRIC 2: search_indexer_request_duration

	assert.Equal(t, "search_indexer_request_duration", collectedMetrics[1].GetName())

	assert.Equal(t, 1, len(collectedMetrics[1].Metric[0].GetLabel()))
	assert.Equal(t, "code", *collectedMetrics[1].Metric[0].GetLabel()[0].Name)
	assert.Equal(t, "200", *collectedMetrics[1].Metric[0].GetLabel()[0].Value)
	assert.Equal(t, 1, len(collectedMetrics[1].GetMetric()))
	assert.Equal(t, uint64(1), collectedMetrics[1].GetMetric()[0].GetHistogram().GetSampleCount())

	// METRIC 3: search_indexer_request_size

	assert.Equal(t, "search_indexer_request_size", collectedMetrics[2].GetName())
	assert.Equal(t, 1, len(collectedMetrics[2].Metric[0].GetLabel()))
	assert.Equal(t, "code", *collectedMetrics[2].Metric[0].GetLabel()[0].Name)
	assert.Equal(t, "200", *collectedMetrics[2].Metric[0].GetLabel()[0].Value)

	// METRIC 4: search_indexer_requests_in_flight
	assert.Equal(t, "search_indexer_requests_in_flight", collectedMetrics[3].GetName())

}
