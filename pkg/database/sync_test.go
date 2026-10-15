// Copyright Contributors to the Open Cluster Management project
package database

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

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

func Test_SyncDataShared(t *testing.T) {
	dao, mockPool := buildMockDAO(t)
	dao.Dispatcher = NewSharedBatch(&dao, 2500, 1*time.Millisecond)

	// SyncDataShared sends all items in a single SharedBatch → 1 SendBatch call.
	br := &testutils.MockBatchResults{}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(1)

	response := &model.SyncResponse{}
	err := dao.SyncData(context.Background(), loadSimpleSyncEvent(t), "local-cluster", response)

	assert.Nil(t, err)
	AssertEqual(t, response.TotalAdded, 2, "Incorrect number of resources added.")
	AssertEqual(t, response.TotalUpdated, 1, "Incorrect number of resources updated.")
	AssertEqual(t, response.TotalEdgesAdded, 1, "Incorrect number of edges added.")
	AssertEqual(t, response.TotalEdgesDeleted, 1, "Incorrect number of edges deleted.")
}

func Test_SyncDataShared_With_Exec_Errors(t *testing.T) {
	dao, mockPool := buildMockDAO(t)
	dao.Dispatcher = NewSharedBatch(&dao, 2500, 1*time.Millisecond)

	// Exec error on every SendBatch triggers binary-search retry.
	// Each item is isolated and its onError callback fires.
	br := &testutils.MockBatchResults{
		MockErrorOnExec: errors.New("mocking error on exec"),
	}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).AnyTimes()

	defer testutils.SupressConsoleOutput()()

	response := &model.SyncResponse{}
	err := dao.SyncData(context.Background(), loadSimpleSyncEvent(t), "local-cluster", response)

	assert.Nil(t, err, "per-item errors are handled via onError, not propagated as batch-level error")
	assert.NotEmpty(t, response.AddErrors, "add errors should be populated by onError callbacks")
	assert.NotEmpty(t, response.UpdateErrors, "update errors should be populated by onError callbacks")
	assert.NotEmpty(t, response.DeleteErrors, "delete errors should be populated by onError callbacks")
}

func Test_SyncDataShared_With_OnClose_Errors(t *testing.T) {
	dao, mockPool := buildMockDAO(t)
	dao.Dispatcher = NewSharedBatch(&dao, 2500, 1*time.Millisecond)

	br := &testutils.MockBatchResults{
		MockErrorOnClose: errors.New("unexpected EOF"),
	}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(1)

	defer testutils.SupressConsoleOutput()()

	response := &model.SyncResponse{}
	err := dao.SyncData(context.Background(), loadSimpleSyncEvent(t), "local-cluster", response)

	assert.NotNil(t, err)
}

// --- Security: UID prefix validation ---

func Test_SyncDataShared_RejectsWrongClusterUID(t *testing.T) {
	defer testutils.SupressConsoleOutput()()

	dao, mockPool := buildMockDAO(t)
	dao.Dispatcher = NewSharedBatch(&dao, 2500, 1*time.Millisecond)

	// All add/update UIDs are "local-cluster/..." but we send as "evil-cluster".
	// The adds and updates are rejected before reaching the batch.
	// The delete, edge-delete, addEdge, and deleteEdge still go into the batch → 1 SendBatch.
	br := &testutils.MockBatchResults{}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(1)

	response := &model.SyncResponse{}
	err := dao.SyncData(context.Background(), loadSimpleSyncEvent(t), "evil-cluster", response)

	assert.Nil(t, err)
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
