// Copyright Contributors to the Open Cluster Management project
package database

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pashagolub/pgxmock"
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

	// Mock PosgreSQL calls.
	// SendBatch count: 2 (add) + 1 (update) + 1 (edge-delete for resource) + 1 (addEdge) + 1 (deleteEdge) = 6.
	// Resource DELETE is now a direct pool.Exec call returning RowsAffected().
	br := &testutils.MockBatchResults{}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(6)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(pgxmock.NewResult("DELETE", 1), nil)

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

	// Mock PosgreSQL calls.
	// Batch exec errors cover: 2 add, 1 update, 1 edge-delete (for resource), 1 addEdge, 1 deleteEdge = 6 batches.
	// Resource DELETE is now a direct pool.Exec call; mock it as an error → appended to DeleteErrors.
	br := &testutils.MockBatchResults{
		MockErrorOnExec: errors.New("mocking error on exec"),
	}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(6)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("mocking exec error"))

	// Supress console output to prevent log messages from polluting test output.
	defer testutils.SupressConsoleOutput()()

	response := &model.SyncResponse{}
	err := dao.SyncData(context.Background(), loadSimpleSyncEvent(t), "local-cluster", response)

	assert.Nil(t, err)
	AssertEqual(t, len(response.AddErrors), 2, "Incorrect number of AddErrors.")
	AssertEqual(t, len(response.UpdateErrors), 1, "Incorrect number of UpdateErrors.")
	// Resource DELETE is now a direct Exec → 1 DeleteError on Exec failure.
	AssertEqual(t, len(response.DeleteErrors), 1, "Incorrect number of DeleteErrors.")
	AssertEqual(t, len(response.AddEdgeErrors), 1, "Incorrect number of AddEdgeErrors.")
	// Edge DELETE for the removed resource + explicit deleteEdge = 2 DeleteEdgeErrors.
	AssertEqual(t, len(response.DeleteEdgeErrors), 2, "Incorrect number of DeleteEdgeErrors.")
}

func Test_Sync_With_OnClose_Errors(t *testing.T) {
	// Prepare a mock DAO instance.
	// Use a batch size large enough to hold all three resource operations (2 adds + 1 update)
	// so they are queued before any batch is sent. The single flush() call then sends them
	// together in one SendBatch, which triggers connError via the "unexpected EOF" close error.
	// Subsequent Queue() calls see connError and return immediately — no further SendBatch calls.
	// The resource DELETE runs as a direct pool.Exec (outside the batch) and is unaffected.
	dao, mockPool := buildMockDAO(t)
	dao.batchSize = 4

	br := &testutils.MockBatchResults{
		MockErrorOnClose: errors.New("unexpected EOF"),
	}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(1)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(pgxmock.NewResult("DELETE", 0), nil)

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
	// The delete, addEdge, and deleteEdge have no UID prefix check and still execute:
	// - Resource DELETE runs via pool.Exec (1 Exec call).
	// - Edge-delete-for-resource + addEdge + deleteEdge flush together into 1 SendBatch.
	br := &testutils.MockBatchResults{}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(1)
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(pgxmock.NewResult("DELETE", 0), nil)

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
