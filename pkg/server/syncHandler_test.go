package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/driftprogramming/pgxpoolmock"
	"github.com/gorilla/mux"
	"github.com/open-cluster-management/search-indexer/pkg/config"
	"github.com/open-cluster-management/search-indexer/pkg/database"
	"github.com/open-cluster-management/search-indexer/pkg/model"
	// "github.com/driftprogramming/pgxpoolmock/testdata"
	"github.com/golang/mock/gomock"
	// "github.com/stretchr/testify/assert"
	"github.com/jackc/pgconn"
	pgx "github.com/jackc/pgx/v4"
)

type BatchResults struct{}

// Exec() (pgconn.CommandTag, error)
// Query() (Rows, error)
// QueryRow() Row
// QueryFunc(scans []interface{}, f func(QueryFuncRow) error) (pgconn.CommandTag, error)
// Close() error
func (s BatchResults) Exec() (pgconn.CommandTag, error) {
	fmt.Println("MOCKING Exec()!!!")
	return nil, nil
}
func (s BatchResults) Query() (pgx.Rows, error) {
	return nil, nil
}
func (s BatchResults) QueryRow() pgx.Row {
	return nil
}
func (s BatchResults) QueryFunc(scans []interface{}, f func(pgx.QueryFuncRow) error) (pgconn.CommandTag, error) {
	return nil, nil
}
func (s BatchResults) Close() error {
	return nil
}

func Test_syncRequest(t *testing.T) {
	// Read mock request body.
	body, readErr := os.Open("./mocks/simple.json")
	if readErr != nil {
		t.Fatal(readErr)
	}

	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest(http.MethodPost, "/aggregator/clusters/test-cluster/sync", body)
	router := mux.NewRouter()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// given
	mockPool := pgxpoolmock.NewMockPgxPool(ctrl)

	br := BatchResults{}

	// br := BR{}

	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br)
	dao := database.NewDAO(mockPool)
	server := ServerConfig{
		Dao: &dao,
	}

	router.HandleFunc("/aggregator/clusters/{id}/sync", server.SyncResources)
	router.ServeHTTP(responseRecorder, request)

	expected := model.SyncResponse{Version: config.COMPONENT_VERSION}

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
