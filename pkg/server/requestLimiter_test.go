// Copyright Contributors to the Open Cluster Management project
package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Verify that request is accepted with 3 pending requests.
func Test_requestLimiterMiddleware(t *testing.T) {
	// Mock 3 requests.
	pendingRequests = map[string]time.Time {"A": time.Now(), "B": time.Now(), "C": time.Now()}

	requestLimiterHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	req := httptest.NewRequest("POST", "https://localhost:3010/aggregator/clusters/cluster1/sync", nil)
	res := httptest.NewRecorder()

	requestLimiterHandler(res, req)
	middleware := requestLimiterMiddleware(requestLimiterHandler)

	middleware.ServeHTTP(res, req)

	// Validate response code and messsage.
	assert.Equal(t, http.StatusOK, res.Code)
}

// Verify that request is rejected when there's a pending request form the same cluster.
func Test_requestLimiterMiddleware_existingRequest(t *testing.T) {
	// Mock a pending request from cluster.
	// Note: Omitting the cluster name to keep the test simple, otherwise we need to mock the mux
	// router so the handler can read the cluster {id} from the route.
	pendingRequests = map[string]time.Time {"": time.Now()}

	// Mock Request and Response.
	requestLimiterHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	req := httptest.NewRequest("POST", "https://localhost:3010/aggregator/clusters/cluster1/sync", nil)
	res := httptest.NewRecorder()
	requestLimiterHandler(res, req)
	middleware := requestLimiterMiddleware(requestLimiterHandler)

	// Execute middleware.
	middleware.ServeHTTP(res, req)

	// Validate response code.
	assert.Equal(t, http.StatusTooManyRequests, res.Code)

	bodyBytes, _ := io.ReadAll(res.Body)
	assert.Equal(t, "A previous request from this cluster is processing, retry later.\n", string(bodyBytes))

}

// Verify that request is rejected when there's 50 or more pending requests.
func Test_requestLimiterMiddleware_with50PendingRequests(t *testing.T) {
	// Mock 50 pending requests.
	pendingRequests = map[string]time.Time {}
	for i:=0; i<50; i++{
		pendingRequests["cluster" + strconv.Itoa(i)] = time.Now()
	}

	requestLimiterHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	req := httptest.NewRequest("POST", "https://localhost:3010/aggregator/clusters/cluster99/sync", nil)
	res := httptest.NewRecorder()

	requestLimiterHandler(res, req)
	middleware := requestLimiterMiddleware(requestLimiterHandler)

	middleware.ServeHTTP(res, req)

	// Validate response code and messsage.
	assert.Equal(t, http.StatusTooManyRequests, res.Code)

	bodyBytes, _ := io.ReadAll(res.Body)
	assert.Equal(t, "Indexer has too many pending requests, retry later.\n", string(bodyBytes))

}
