// Copyright Contributors to the Open Cluster Management project
package database

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stolostron/search-indexer/pkg/model"
	"github.com/stolostron/search-indexer/pkg/testutils"
	"github.com/stretchr/testify/assert"
)

// loadSimpleSyncEvent reads the shared mock sync payload from mocks/simple.json.
// All UIDs in that file are prefixed with "local-cluster/".
func loadSimpleSyncEvent(t *testing.T) model.SyncEvent {
	t.Helper()
	data, err := os.Open("./mocks/simple.json")
	if err != nil {
		t.Fatalf("failed to open mock sync payload: %v", err)
	}
	defer data.Close() //nolint: errcheck
	var syncEvent model.SyncEvent
	if err := json.NewDecoder(data).Decode(&syncEvent); err != nil {
		t.Fatalf("failed to decode mock sync payload: %v", err)
	}
	return syncEvent
}

func Test_SyncData(t *testing.T) {
	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)
	dao.batchSize = 1

	// Mock PosgreSQL calls
	br := &testutils.MockBatchResults{}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(7)

	// UIDs in simple.json are "local-cluster/…", so clusterName must match.
	response := &model.SyncResponse{}
	err := dao.SyncData(context.Background(), loadSimpleSyncEvent(t), "local-cluster", response)

	assert.Nil(t, err)
	AssertEqual(t, response.TotalAdded, 2, "Incorrect number of resources added.")
	AssertEqual(t, response.TotalUpdated, 1, "Incorrect number of resources updated.")
	AssertEqual(t, response.TotalDeleted, 1, "Incorrect number of resources deleted.")
	AssertEqual(t, response.TotalEdgesAdded, 1, "Incorrect number of edges added.")
	AssertEqual(t, response.TotalEdgesDeleted, 1, "Incorrect number of edges deleted.")
}

// Test for the error path.
func Test_Sync_With_Exec_Errors(t *testing.T) {
	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)
	dao.batchSize = 1

	// Mock PosgreSQL calls
	br := &testutils.MockBatchResults{
		MockErrorOnExec: errors.New("mocking error on exec"),
	}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(7)

	// Supress console output to prevent log messages from polluting test output.
	defer testutils.SupressConsoleOutput()()

	response := &model.SyncResponse{}
	err := dao.SyncData(context.Background(), loadSimpleSyncEvent(t), "local-cluster", response)

	assert.Nil(t, err)
	AssertEqual(t, len(response.AddErrors), 2, "Incorrect number of AddErrors.")
	AssertEqual(t, len(response.UpdateErrors), 1, "Incorrect number of UpdateErrors.")
	// The resource DELETE (cluster-scoped) is 1 statement → 1 DeleteError.
	// The accompanying edge DELETE (cluster-scoped) is now categorised as deleteEdge
	// → counted in DeleteEdgeErrors together with the explicit edge delete below.
	AssertEqual(t, len(response.DeleteErrors), 1, "Incorrect number of DeleteErrors.")
	AssertEqual(t, len(response.AddEdgeErrors), 1, "Incorrect number of AddEdgeErrors.")
	AssertEqual(t, len(response.DeleteEdgeErrors), 2, "Incorrect number of DeleteEdgeErrors.")
}

func Test_Sync_With_OnClose_Errors(t *testing.T) {
	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)
	dao.batchSize = 1

	// Mock PosgreSQL calls
	br := &testutils.MockBatchResults{
		MockErrorOnClose: errors.New("unexpected EOF"),
	}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(7)

	// Supress console output to prevent log messages from polluting test output.
	defer testutils.SupressConsoleOutput()()

	response := &model.SyncResponse{}
	err := dao.SyncData(context.Background(), loadSimpleSyncEvent(t), "local-cluster", response)

	assert.NotNil(t, err)
}

// --- Security: UID prefix validation ---

// Test_SyncData_RejectsWrongClusterUID verifies that resources whose UIDs belong to
// a different cluster are silently dropped (logged + error recorded) rather than
// written to the database. No DB calls should be issued for the rejected items.
func Test_SyncData_RejectsWrongClusterUID(t *testing.T) {
	defer testutils.SupressConsoleOutput()()

	dao, mockPool := buildMockDAO(t)
	dao.batchSize = 10

	// The payload has 2 adds + 1 update with "local-cluster/" UIDs, but we're
	// sending as "evil-cluster". All three should be rejected before any DB call.
	// The 1 delete and 1 addEdge / 1 deleteEdge have no UID prefix check (they go
	// through the bulk SQL path), so 3 batches are still expected.
	br := &testutils.MockBatchResults{}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(3)

	response := &model.SyncResponse{}
	err := dao.SyncData(context.Background(), loadSimpleSyncEvent(t), "evil-cluster", response)

	assert.Nil(t, err)
	// All adds and the update are rejected; none reach the DB.
	AssertEqual(t, response.TotalAdded, 0, "No resources should be added for wrong-cluster UIDs.")
	AssertEqual(t, response.TotalUpdated, 0, "No resources should be updated for wrong-cluster UIDs.")
	AssertEqual(t, len(response.AddErrors), 2, "Expected 2 AddErrors for rejected UIDs.")
	AssertEqual(t, len(response.UpdateErrors), 1, "Expected 1 UpdateError for rejected UID.")
}

func Test_validateUIDPrefix(t *testing.T) {
	tests := []struct {
		name        string
		uid         string
		clusterName string
		wantErr     bool
	}{
		{"valid prefix", "my-cluster/abc-123", "my-cluster", false},
		{"valid prefix with nested slash", "my-cluster/ns/kind/name", "my-cluster", false},
		{"wrong cluster prefix", "other-cluster/abc-123", "my-cluster", true},
		{"no slash in uid", "abc-123", "my-cluster", true},
		{"empty uid", "", "my-cluster", true},
		{"prefix only no slash", "my-cluster", "my-cluster", true},
		{"empty cluster name", "my-cluster/abc", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateUIDPrefix(tc.uid, tc.clusterName)
			if tc.wantErr {
				assert.Error(t, err, "expected an error for uid=%q cluster=%q", tc.uid, tc.clusterName)
			} else {
				assert.NoError(t, err, "expected no error for uid=%q cluster=%q", tc.uid, tc.clusterName)
			}
		})
	}
}
