// Copyright Contributors to the Open Cluster Management project

package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/stolostron/search-indexer/pkg/model"
	"github.com/stolostron/search-indexer/pkg/testutils"
	"github.com/stretchr/testify/assert"
)

func TestValidationRequest(t *testing.T) {
	server, mockPool := buildMockServer(t)
	batchResults := &testutils.MockBatchResults{
		MockRows: testutils.MockRows{
			MockData: []map[string]interface{}{{"count": 5}, {"count": 3}},
		},
	}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(batchResults)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/aggregator/clusters/local-cluster/validation", nil)
	router := mux.NewRouter()
	router.HandleFunc("/aggregator/clusters/{id}/validation", server.ValidateCluster)
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	response := model.ValidationResponse{}
	assert.NoError(t, json.NewDecoder(recorder.Body).Decode(&response))
	assert.Equal(t, model.ValidationResponse{TotalResources: 5, TotalEdges: 3}, response)
}

func TestValidationRequestDatabaseError(t *testing.T) {
	server, mockPool := buildMockServer(t)
	batchResults := &testutils.MockBatchResults{
		MockRows: testutils.MockRows{
			MockData:        []map[string]interface{}{{"count": 5}, {"count": 3}},
			MockErrorOnScan: errors.New("unexpected EOF"),
		},
	}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(batchResults)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/aggregator/clusters/local-cluster/validation", nil)
	router := mux.NewRouter()
	router.HandleFunc("/aggregator/clusters/{id}/validation", server.ValidateCluster)
	router.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
}
