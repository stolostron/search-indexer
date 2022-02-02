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

type batchItem struct {
	action string // Used to report errors.
	query  string
	args   []interface{}
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
		go b.sendBatch(items)
	}
}

func (b *batchWithRetry) sendBatch(items []batchItem) error {
	defer b.wg.Done()

	batch := &pgx.Batch{}
	for _, item := range items {
		batch.Queue(item.query, item.args...)
	}
	br := b.dao.pool.SendBatch(context.Background(), batch)
	_, err := br.Exec()
	br.Close()

	// Process errors.
	// pgx.Batch is processed as a transaction, so in case of an error, the entire batch will fail.
	if err != nil && len(items) == 1 {

		item := items[0]
		klog.Errorf("ERROR processing batchItem.  %+v", items[0])

		var errorArray *[]model.SyncError
		switch item.action {
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
			klog.Error("Unable to process sync error with type: ", item.action)
		}
		*errorArray = append(*errorArray, model.SyncError{ResourceUID: items[0].uid, Message: "Error details"})

		return nil // Don't return error here to stop the recursion.
	} else if err != nil {
		// Error sending batch, resending in smaller batches.

		// Retry first half.
		firstHalf := items[:len(items)/2]
		b.wg.Add(1)
		err1 := b.sendBatch(firstHalf)

		// Retry second half
		secondHalf := items[len(items)/2:]
		b.wg.Add(1)
		err2 := b.sendBatch(secondHalf)

		// Returns error only if we fail processing either retry.
		if err1 != nil && err2 != nil {
			return nil
		}
	}
	return err
}

func (b *batchWithRetry) flush() {
	if len(b.items) > 0 {
		items := b.items               // Create a snapshot of the items to process.
		b.items = make([]batchItem, 0) // Reset the queue.
		b.wg.Add(1)
		go b.sendBatch(items)
	}
}
