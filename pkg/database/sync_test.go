// Copyright Contributors to the Open Cluster Management project
package database

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stolostron/search-indexer/pkg/model"
)

func Test_SyncData(t *testing.T) {
	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)
	dao.batchSize = 1

	// Mock PosgreSQL calls
	br := BatchResults{}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(7)

	// Prepare Request data
	data, _ := os.Open("./mocks/simple.json")
	var syncEvent model.SyncEvent
	json.NewDecoder(data).Decode(&syncEvent) //nolint: errcheck

	// Execute test
	dao.SyncData(syncEvent, "test-cluster", &model.SyncResponse{})
}

// Test for the error path.
func Test_Sync_With_Errors(t *testing.T) {
	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)
	dao.batchSize = 1

	// Mock PosgreSQL calls
	br := BatchResults{
		mockErrorOnClose: true,
		mockErrorOnExec:  true,
	}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(7)

	// Prepare Request data
	data, _ := os.Open("./mocks/simple.json")
	var syncEvent model.SyncEvent
	json.NewDecoder(data).Decode(&syncEvent) //nolint: errcheck

	// Execute test
	response := &model.SyncResponse{}
	dao.SyncData(syncEvent, "test-cluster", response)

	if len(response.AddErrors) != 2 {
		t.Errorf("Incorrect number of AddErrors. Expected: %d  Got: %d", 2, len(response.AddErrors))
	}
	if len(response.UpdateErrors) != 1 {
		t.Errorf("Incorrect number of UpdateErrors. Expected: %d  Got: %d", 1, len(response.UpdateErrors))
	}
	if len(response.DeleteErrors) != 2 {
		t.Errorf("Incorrect number of DeleteErrors. Expected: %d  Got: %d", 2, len(response.DeleteErrors))
	}
	if len(response.AddEdgeErrors) != 1 {
		t.Errorf("Incorrect number of AddEdgeErrors. Expected: %d  Got: %d", 1, len(response.AddEdgeErrors))
	}
	if len(response.DeleteEdgeErrors) != 1 {
		t.Errorf("Incorrect number of DeleteEdgeErrors. Expected: %d  Got: %d", 1, len(response.DeleteEdgeErrors))
	}
}
