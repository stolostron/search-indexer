// Copyright Contributors to the Open Cluster Management project
package database

import (
	// "bytes"
	"encoding/json"
	// "io/ioutil"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stolostron/search-indexer/pkg/model"
	// "k8s.io/klog/v2"
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
	response := &model.SyncResponse{}
	dao.SyncData(syncEvent, "test-cluster", response)

	// Assert
	AssertEqual(t, response.TotalAdded, 2, "Incorrect number of resources added.")
	AssertEqual(t, response.TotalUpdated, 1, "Incorrect number of resources updated.")
	AssertEqual(t, response.TotalDeleted, 1, "Incorrect number of resources deleted.")
	AssertEqual(t, response.TotalEdgesAdded, 1, "Incorrect number of edges added.")
	AssertEqual(t, response.TotalEdgesDeleted, 1, "Incorrect number of edges deleted.")
}

// Test for the error path.
func Test_Sync_With_Errors(t *testing.T) {
	// Supress console output to prevent log messages from polluting test output.
	// var buf bytes.Buffer
	// klog.LogToStderr(false)
	// klog.SetOutput(&buf)
	// defer func() {
	// 	klog.SetOutput(os.Stderr)
	// }()

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

	// Assert
	AssertEqual(t, len(response.AddErrors), 2, "Incorrect number of AddErrors.")
	AssertEqual(t, len(response.UpdateErrors), 1, "Incorrect number of UpdateErrors.")
	AssertEqual(t, len(response.DeleteErrors), 2, "Incorrect number of DeleteErrors.")
	AssertEqual(t, len(response.AddEdgeErrors), 1, "Incorrect number of AddEdgeErrors.")
	AssertEqual(t, len(response.DeleteEdgeErrors), 1, "Incorrect number of DeleteEdgeErrors.")
}
