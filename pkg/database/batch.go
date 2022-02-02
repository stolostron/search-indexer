// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"sync"

	pgx "github.com/jackc/pgx/v4"
	"github.com/stolostron/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

// This is a wrapper for pgx.Batch
// We need this because pgx.Batch doesn't provide a way to retry batches with errors.
// Also adds reporting of errors.

type batchItem struct {
	query  string
	args   []interface{}
	action string // Used to report errors.
	uid    string // Used to report errors.
}

type batchWithRetry struct {
	items        []batchItem
	dao          *DAO
	wg           *sync.WaitGroup
	syncResponse *model.SyncResponse
}

func NewBatchWithRetry(dao *DAO, syncResponse *model.SyncResponse) batchWithRetry {
	batch := batchWithRetry{
		items:        make([]batchItem, 0),
		wg:           &sync.WaitGroup{},
		dao:          dao,
		syncResponse: syncResponse,
	}
	return batch
}

func (b *batchWithRetry) Queue(item batchItem) {
	b.items = append(b.items, item)

	if len(b.items) >= b.dao.batchSize {
		items := b.items               // Create a snapshot of the items to process.
		b.items = make([]batchItem, 0) // Reset the queue.
		b.wg.Add(1)
		go b.sendBatch(items) // nolint: errcheck
	}
}

func (b *batchWithRetry) sendBatch(items []batchItem) error {
	defer b.wg.Done()

	batch := &pgx.Batch{}
	for _, item := range items {
		batch.Queue(item.query, item.args...)
	}
	br := b.dao.pool.SendBatch(context.Background(), batch)
	_, execErr := br.Exec()

	closeErr := br.Close()
	if closeErr != nil {
		klog.Error("Error closing batch result.", closeErr)
	}

	// Process errors.
	// pgx.Batch is processed as a transaction, so in case of an error, the entire batch will fail.
	if execErr != nil && len(items) == 1 {

		errorItem := items[0]
		klog.Errorf("ERROR processing batchItem.  %+v", errorItem)

		var errorArray *[]model.SyncError
		switch errorItem.action {
		case "addResource":
			errorArray = &b.syncResponse.AddErrors
		case "updateResource":
			errorArray = &b.syncResponse.UpdateErrors
		case "deleteResource":
			errorArray = &b.syncResponse.DeleteErrors
		case "addEdge":
			errorArray = &b.syncResponse.AddEdgeErrors
		case "deleteEdge":
			errorArray = &b.syncResponse.DeleteEdgeErrors
		default:
			klog.Error("Unable to process sync error with type: ", errorItem.action)
		}
		*errorArray = append(*errorArray,
			model.SyncError{ResourceUID: errorItem.uid, Message: "Resource generated an error while updating the database."})

		return nil // We have processed the error, so don't return an error here to stop the recursion.

	} else if execErr != nil {
		// Error in sent batch, resend queries using smaller batches.
		// Use a binary search recursively until we find the error.

		b.wg.Add(2)
		err1 := b.sendBatch(items[:len(items)/2])
		err2 := b.sendBatch(items[len(items)/2:])

		// Returns error only if we fail processing either retry.
		if err1 != nil && err2 != nil {
			return nil
		}
	}
	return execErr
}

func (b *batchWithRetry) flush() {
	if len(b.items) > 0 {
		items := b.items               // Create a snapshot of the items to process.
		b.items = make([]batchItem, 0) // Reset the queue.
		b.wg.Add(1)
		go b.sendBatch(items) // nolint: errcheck
	}
}
