// Copyright Contributors to the Open Cluster Management project
package database

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/jackc/pgconn"
	"github.com/stolostron/search-indexer/pkg/testutils"
	"github.com/stretchr/testify/assert"
)

func Test_SharedBatch_SingleCaller_TimerFire(t *testing.T) {
	dao, mockPool := buildMockDAO(t)

	br := &testutils.MockBatchResults{}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(1)

	sb := NewSharedBatch(&dao, 100, 1*time.Millisecond)

	items := []sharedBatchItem{
		{batchItem: batchItem{query: "INSERT INTO search.resources VALUES($1,$2,$3)", args: []interface{}{"uid1", "cluster1", "{}"}, action: "addResource", uid: "uid1"}},
		{batchItem: batchItem{query: "INSERT INTO search.resources VALUES($1,$2,$3)", args: []interface{}{"uid2", "cluster1", "{}"}, action: "addResource", uid: "uid2"}},
	}

	err := sb.SubmitAndWait(context.Background(), items)
	assert.Nil(t, err)
}

func Test_SharedBatch_FireOnBatchSize(t *testing.T) {
	dao, mockPool := buildMockDAO(t)

	br := &testutils.MockBatchResults{}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(1)

	sb := NewSharedBatch(&dao, 2, 5*time.Second)

	items := []sharedBatchItem{
		{batchItem: batchItem{query: "INSERT INTO search.resources VALUES($1)", args: []interface{}{"uid1"}, action: "addResource", uid: "uid1"}},
		{batchItem: batchItem{query: "INSERT INTO search.resources VALUES($1)", args: []interface{}{"uid2"}, action: "addResource", uid: "uid2"}},
	}

	err := sb.SubmitAndWait(context.Background(), items)
	assert.Nil(t, err)
}

func Test_SharedBatch_MultipleConcurrentCallers(t *testing.T) {
	dao, mockPool := buildMockDAO(t)

	br := &testutils.MockBatchResults{}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).MinTimes(1)

	sb := NewSharedBatch(&dao, 100, 1*time.Millisecond)

	var wg sync.WaitGroup
	errs := make([]error, 3)

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			items := []sharedBatchItem{
				{batchItem: batchItem{query: "INSERT INTO search.resources VALUES($1)", args: []interface{}{idx}, action: "addResource", uid: "uid"}},
			}
			errs[idx] = sb.SubmitAndWait(context.Background(), items)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		assert.Nil(t, err, "caller %d should not get an error", i)
	}
}

func Test_SharedBatch_OnResultCallback(t *testing.T) {
	dao, mockPool := buildMockDAO(t)

	br := &testutils.MockBatchResults{}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(1)

	sb := NewSharedBatch(&dao, 100, 1*time.Millisecond)

	var callbackCalled bool
	items := []sharedBatchItem{
		{
			batchItem: batchItem{query: "DELETE FROM search.resources WHERE uid=$1", args: []interface{}{"uid1"}, action: "deleteResource", uid: "uid1"},
			onResult: func(tag pgconn.CommandTag) {
				callbackCalled = true
			},
		},
	}

	err := sb.SubmitAndWait(context.Background(), items)
	assert.Nil(t, err)
	assert.True(t, callbackCalled, "onResult callback should have been called")
}

func Test_SharedBatch_ErrorPropagates(t *testing.T) {
	dao, mockPool := buildMockDAO(t)

	br := &testutils.MockBatchResults{
		MockErrorOnExec: errors.New("batch exec error"),
	}
	mockPool.EXPECT().SendBatch(gomock.Any(), gomock.Any()).Return(br).Times(1)

	defer testutils.SupressConsoleOutput()()

	sb := NewSharedBatch(&dao, 100, 1*time.Millisecond)

	items := []sharedBatchItem{
		{batchItem: batchItem{query: "INSERT INTO search.resources VALUES($1)", args: []interface{}{"uid1"}, action: "addResource", uid: "uid1"}},
	}

	err := sb.SubmitAndWait(context.Background(), items)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "batch exec error")
}

func Test_SharedBatch_ContextCancellation(t *testing.T) {
	dao, _ := buildMockDAO(t)

	sb := NewSharedBatch(&dao, 100, 10*time.Second)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	items := []sharedBatchItem{
		{batchItem: batchItem{query: "INSERT INTO search.resources VALUES($1)", args: []interface{}{"uid1"}, action: "addResource", uid: "uid1"}},
	}

	err := sb.SubmitAndWait(ctx, items)
	assert.Equal(t, context.Canceled, err)
}

func Test_SharedBatch_EmptyItems(t *testing.T) {
	dao, _ := buildMockDAO(t)

	sb := NewSharedBatch(&dao, 100, 1*time.Millisecond)

	err := sb.SubmitAndWait(context.Background(), []sharedBatchItem{})
	assert.Nil(t, err)
}
