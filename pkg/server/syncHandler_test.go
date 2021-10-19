package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
	"github.com/jlpadilla/search-indexer/pkg/config"
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
	router.HandleFunc("/aggregator/clusters/{id}/sync", SyncResources)
	router.ServeHTTP(responseRecorder, request)

	expected := SyncResponse{Version: config.AGGREGATOR_API_VERSION}

	if responseRecorder.Code != http.StatusOK {
		t.Errorf("Want status '%d', got '%d'", http.StatusOK, responseRecorder.Code)
	}

	var decodedResp SyncResponse
	err := json.NewDecoder(responseRecorder.Body).Decode(&decodedResp)
	if err != nil {
		t.Error("Unable to decode respoonse body.")
	}

	// fmt.Printf("Decoded response: %+v", decodedResp)
	if fmt.Sprintf("%+v", decodedResp) != fmt.Sprintf("%+v", expected) {
		// if decodedResp != expected {
		t.Errorf("Incorrect response body.\n expected '%+v'\n received '%+v'", expected, decodedResp)
	}
}
