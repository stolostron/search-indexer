// Copyright Contributors to the Open Cluster Management project

package server

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

func Test_largeReqeustLimiterMiddleware(t *testing.T) {
	// Given: no large requests being processed and a large request
	largeRequestCountTracker = 0
	largeRequestLimiterHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	largeRequest := make([]byte, 1024*1024*21)
	for i := range largeRequest {
		largeRequest[i] = 0
	}
	reader := bytes.NewReader(largeRequest)
	req := httptest.NewRequest("POST", "https://localhost:3010/aggregator/clusters/cluster1/sync", reader)
	res := httptest.NewRecorder()

	// When: we process the request
	largeRequestLimiterHandler(res, req)
	middleware := largeRequestLimiterMiddleware(largeRequestLimiterHandler)
	middleware.ServeHTTP(res, req)

	// Then: the request is processed without err
	assert.Equal(t, http.StatusOK, res.Code)
}

func Test_largeRequestLimiterMiddleware_tooManyRequests(t *testing.T) {
	// Given: too many large requests being processed and a large request
	largeRequestCountTracker = 10
	largeRequestLimiterHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	largeRequest := make([]byte, 1024*1024*21)
	for i := range largeRequest {
		largeRequest[i] = 0
	}
	reader := bytes.NewReader(largeRequest)
	req := httptest.NewRequest("POST", "https://localhost:3010/aggregator/clusters/cluster1/sync", reader)
	res := httptest.NewRecorder()

	// When: we get process the request
	largeRequestLimiterHandler(res, req)
	middleware := largeRequestLimiterMiddleware(largeRequestLimiterHandler)
	middleware.ServeHTTP(res, req)

	// Then: the request is rejected with err
	assert.Equal(t, http.StatusTooManyRequests, res.Code)
}

func Test_largeRequestLimiterMiddleware_smallRequest(t *testing.T) {
	// Given: too many large requests being processed and a small request
	largeRequestCountTracker = 10
	largeRequestLimiterHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	req := httptest.NewRequest("POST", "https://localhost:3010/aggregator/clusters/cluster1/sync", nil)
	res := httptest.NewRecorder()

	// When: we process the request
	largeRequestLimiterHandler(res, req)
	middleware := largeRequestLimiterMiddleware(largeRequestLimiterHandler)
	middleware.ServeHTTP(res, req)

	// Then: the request is processed without err
	assert.Equal(t, http.StatusOK, res.Code)
}
