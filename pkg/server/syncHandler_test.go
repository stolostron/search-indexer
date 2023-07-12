// Copyright Contributors to the Open Cluster Management project
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/driftprogramming/pgxpoolmock"
	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/stolostron/search-indexer/pkg/config"
	"github.com/stolostron/search-indexer/pkg/model"

	"github.com/stretchr/testify/assert"
)

func Test_syncRequest(t *testing.T) {
	// Read mock request body.
	body, readErr := os.Open("./mocks/simple.json")
	if readErr != nil {
		t.Fatal(readErr)
	}
	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest(http.MethodPost, "/aggregator/clusters/test-cluster/sync", body)
	router := mux.NewRouter()

	// Create server with mock database.
	server, mockPool := buildMockServer(t)

	br := &batchResults{rows: []int{5, 3}}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(2)

	router.HandleFunc("/aggregator/clusters/{id}/sync", server.SyncResources)
	router.ServeHTTP(responseRecorder, request)

	// Validation
	expected := model.SyncResponse{Version: config.COMPONENT_VERSION, TotalAdded: 2, TotalResources: 5, TotalEdges: 3}

	if responseRecorder.Code != http.StatusOK {
		t.Errorf("Want status '%d', got '%d'", http.StatusOK, responseRecorder.Code)
	}

	var decodedResp model.SyncResponse
	err := json.NewDecoder(responseRecorder.Body).Decode(&decodedResp)
	if err != nil {
		t.Error("Unable to decode respoonse body.")
	}

	if fmt.Sprintf("%+v", decodedResp) != fmt.Sprintf("%+v", expected) {
		t.Errorf("Incorrect response body.\n expected '%+v'\n received '%+v'", expected, decodedResp)
	}
}

func Test_syncRequest_withError(t *testing.T) {
	// Read mock request body.
	body, readErr := os.Open("./mocks/simple.json")
	if readErr != nil {
		t.Fatal(readErr)
	}
	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/aggregator/clusters/test-cluster/sync", body)
	router := mux.NewRouter()

	// Create server with mock database.
	server, mockPool := buildMockServer(t)
	br := &batchResults{rows: []int{5, 3}, mockErrorOnClose: errors.New("unexpected EOF")}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(2)

	router.HandleFunc("/aggregator/clusters/{id}/sync", server.SyncResources)
	router.ServeHTTP(responseRecorder, request)

	// Validate
	assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	bodyString, _ := responseRecorder.Body.ReadString(byte(0))
	assert.Equal(t, "Server error while processing the request.\n", bodyString)
}

func Test_syncRequest_withErrorQueryingTotalResources(t *testing.T) {
	// Read mock request body.
	body, readErr := os.Open("./mocks/simple.json")
	if readErr != nil {
		t.Fatal(readErr)
	}
	responseRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/aggregator/clusters/test-cluster/sync", body)
	router := mux.NewRouter()

	// Create server with mock database.
	server, mockPool := buildMockServer(t)
	br := &batchResults{rows: []int{10, 4}, mockErrorOnQuery: errors.New("unexpected EOF")}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(2)

	router.HandleFunc("/aggregator/clusters/{id}/sync", server.SyncResources)
	router.ServeHTTP(responseRecorder, request)

	// Validate
	assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	bodyString, _ := responseRecorder.Body.ReadString(byte(0))
	assert.Equal(t, "Server error while processing the request.\n", bodyString)
}

func Test_resyncRequest(t *testing.T) {
	// Read mock request body.
	body, readErr := os.Open("./mocks/clearAll.json")
	if readErr != nil {
		t.Fatal(readErr)
	}
	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest(http.MethodPost, "/aggregator/clusters/test-cluster/sync", body)
	router := mux.NewRouter()

	// Create server with mock database.
	server, mockPool := buildMockServer(t)

	br := &batchResults{rows: []int{10, 4}}
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any()).Times(2)
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(2)
	columns := []string{"sourceId", "edgeType", "destId"}
	pgxRows := pgxpoolmock.NewRows(columns).AddRow("sourceId1", "edgeType1", "destId1").ToPgxRows()
	mockPool.EXPECT().Query(gomock.Any(), gomock.Eq(`SELECT sourceId,edgeType,destId FROM search.edges WHERE edgetype!='interCluster' AND cluster=$1`), []interface{}{"test-cluster"}).Return(pgxRows, nil)
	mockPool.EXPECT().Query(gomock.Any(), gomock.Eq(`SELECT uid FROM search.resources WHERE cluster=$1`), []interface{}{"test-cluster"}).Return(pgxRows, nil)
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(3)

	router.HandleFunc("/aggregator/clusters/{id}/sync", server.SyncResources)
	router.ServeHTTP(responseRecorder, request)

	expected := model.SyncResponse{Version: config.COMPONENT_VERSION, TotalAdded: 2, TotalResources: 10, TotalEdges: 4}

	if responseRecorder.Code != http.StatusOK {
		t.Errorf("Want status '%d', got '%d'", http.StatusOK, responseRecorder.Code)
	}

	var decodedResp model.SyncResponse
	err := json.NewDecoder(responseRecorder.Body).Decode(&decodedResp)
	if err != nil {
		t.Error("Unable to decode response body.")
	}

	if fmt.Sprintf("%+v", decodedResp) != fmt.Sprintf("%+v", expected) {
		t.Errorf("Incorrect response body.\n expected '%+v'\n received '%+v'", expected, decodedResp)
	}
}

func Test_resyncRequest_withErrorDeletingResources(t *testing.T) {
	// Read mock request body.
	body, readErr := os.Open("./mocks/clearAll.json")
	if readErr != nil {
		t.Fatal(readErr)
	}
	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest(http.MethodPost, "/aggregator/clusters/test-cluster/sync", body)
	router := mux.NewRouter()

	columns := []string{"sourceId", "edgeType", "destId"}
	pgxRows := pgxpoolmock.NewRows(columns).AddRow("sourceId1", "edgeType1", "destId1").ToPgxRows()

	// Create server with mock database.
	server, mockPool := buildMockServer(t)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("unexpected EOF"))
	mockPool.EXPECT().Query(gomock.Any(), gomock.Eq(`SELECT sourceId,edgeType,destId FROM search.edges WHERE edgetype!='interCluster' AND cluster=$1`), []interface{}{"test-cluster"}).Return(pgxRows, nil)
	mockPool.EXPECT().Query(gomock.Any(), gomock.Eq(`SELECT uid FROM search.resources WHERE cluster=$1`), []interface{}{"test-cluster"}).Return(pgxRows, nil)

	br := &batchResults{rows: []int{10}, mockErrorOnQuery: errors.New("unexpected EOF")} //mockErrorOnClose: errors.New("Server error while processing the request.")}

	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(3)

	router.HandleFunc("/aggregator/clusters/{id}/sync", server.SyncResources)
	router.ServeHTTP(responseRecorder, request)

	// Validate
	assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	bodyString, _ := responseRecorder.Body.ReadString(byte(0))
	assert.Equal(t, "Server error while processing the request.\n", bodyString)
}

func Test_resyncRequest_withErrorDeletingEdges(t *testing.T) {
	// Read mock request body.
	body, readErr := os.Open("./mocks/clearAll.json")
	if readErr != nil {
		t.Fatal(readErr)
	}
	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest(http.MethodPost, "/aggregator/clusters/test-cluster/sync", body)
	router := mux.NewRouter()

	columns := []string{"sourceId", "edgeType", "destId"}
	pgxRows := pgxpoolmock.NewRows(columns).AddRow("sourceId1", "edgeType1", "destId1").ToPgxRows()

	// Create server with mock database.
	server, mockPool := buildMockServer(t)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any())
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("unexpected EOF"))
	mockPool.EXPECT().Query(gomock.Any(), gomock.Eq(`SELECT sourceId,edgeType,destId FROM search.edges WHERE edgetype!='interCluster' AND cluster=$1`), []interface{}{"test-cluster"}).Return(pgxRows, nil)
	mockPool.EXPECT().Query(gomock.Any(), gomock.Eq(`SELECT uid FROM search.resources WHERE cluster=$1`), []interface{}{"test-cluster"}).Return(pgxRows, nil)

	br := &batchResults{rows: []int{10}, mockErrorOnQuery: errors.New("unexpected EOF")} //mockErrorOnClose: errors.New("Server error while processing the request.")}

	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(3)
	router.HandleFunc("/aggregator/clusters/{id}/sync", server.SyncResources)
	router.ServeHTTP(responseRecorder, request)

	// Validate
	assert.Equal(t, http.StatusInternalServerError, responseRecorder.Code)
	bodyString, _ := responseRecorder.Body.ReadString(byte(0))
	assert.Equal(t, "Server error while processing the request.\n", bodyString)
}

func Test_incorrectRequestBody(t *testing.T) {
	body := strings.NewReader("This is an incorrect request body.")

	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest(http.MethodPost, "/aggregator/clusters/test-cluster/sync", body)
	router := mux.NewRouter()

	server, _ := buildMockServer(t)

	router.HandleFunc("/aggregator/clusters/{id}/sync", server.SyncResources)
	router.ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusBadRequest {
		t.Errorf("Want status '%d', got '%d'", http.StatusBadRequest, responseRecorder.Code)
	}
}
