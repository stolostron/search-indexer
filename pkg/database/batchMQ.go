// Copyright Contributors to the Open Cluster Management project
package database

import "github.com/stolostron/search-indexer/pkg/model"

// Same as queue() but public.
func (b *batchWithRetry) QueueMQ(clusterName string, mqMessage model.MqMessage) error {
	if b.connError != nil { // Can't queue more items after DB connection error.
		return b.connError
	}
	mqItem := batchItem{
		query:  "Insert into search.resources(uid, cluster, data) values($1,$2,$3) ON CONFLICT (uid) DO UPDATE SET data=$3 WHERE search.resources.uid=$1 and search.resources.data IS DISTINCT FROM $3",
		args:   []interface{}{mqMessage.UID, clusterName, mqMessage.Properties},
		action: "addResource",
		uid:    mqMessage.UID,
	}

	b.items = append(b.items, mqItem)

	if len(b.items) >= b.dao.batchSize {
		items := b.items               // Create a snapshot of the items to process.
		b.items = make([]batchItem, 0) // Reset the queue.
		b.wg.Add(1)
		go b.sendBatch(items) // nolint: errcheck
	}
	return nil
}

// Same as flush() but public.
func (b *batchWithRetry) FlushMQ() {
	if len(b.items) > 0 {
		items := b.items               // Create a snapshot of the items to process.
		b.items = make([]batchItem, 0) // Reset the queue.
		b.wg.Add(1)
		go b.sendBatch(items) // nolint: errcheck
	}
}
