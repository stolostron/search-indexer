// Copyright Contributors to the Open Cluster Management project

package model

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_AppendError_RoutesByAction(t *testing.T) {
	tests := []struct {
		action string
		get    func(*SyncResponse) []SyncError
	}{
		{"addResource", func(r *SyncResponse) []SyncError { return r.AddErrors }},
		{"updateResource", func(r *SyncResponse) []SyncError { return r.UpdateErrors }},
		{"deleteResource", func(r *SyncResponse) []SyncError { return r.DeleteErrors }},
		{"addEdge", func(r *SyncResponse) []SyncError { return r.AddEdgeErrors }},
		{"deleteEdge", func(r *SyncResponse) []SyncError { return r.DeleteEdgeErrors }},
	}

	for _, tc := range tests {
		t.Run(tc.action, func(t *testing.T) {
			r := &SyncResponse{}
			e := SyncError{ResourceUID: "cluster1/abc", Message: "boom"}

			ok := r.AppendError(tc.action, e)

			assert.True(t, ok, "recognized action should return true")
			assert.Equal(t, []SyncError{e}, tc.get(r), "error should land in the matching slice")
		})
	}
}

func Test_AppendError_UnknownAction(t *testing.T) {
	r := &SyncResponse{}

	ok := r.AppendError("bogusAction", SyncError{ResourceUID: "cluster1/abc", Message: "boom"})

	assert.False(t, ok, "unknown action should return false")
	assert.Empty(t, r.AddErrors)
	assert.Empty(t, r.UpdateErrors)
	assert.Empty(t, r.DeleteErrors)
	assert.Empty(t, r.AddEdgeErrors)
	assert.Empty(t, r.DeleteEdgeErrors)
}

// Test_AppendError_Concurrent exercises the lock: many goroutines appending to the
// same slice must not race or drop entries. Run with -race to catch regressions.
func Test_AppendError_Concurrent(t *testing.T) {
	r := &SyncResponse{}
	const n = 100

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			r.AppendError("addResource", SyncError{ResourceUID: "cluster1/abc", Message: "boom"})
		}()
	}
	wg.Wait()

	assert.Len(t, r.AddErrors, n, "every concurrent append should be recorded")
}
