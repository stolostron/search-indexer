// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	pgx "github.com/jackc/pgx/v4"
	"github.com/open-cluster-management/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

const BATCH_SIZE = 500

func (dao *DAO) SyncData(event model.SyncEvent, clusterName string) {
	wg := &sync.WaitGroup{}
	batch := &pgx.Batch{}
	count := 0

	// ADD
	for _, resource := range event.AddResources {
		json, _ := json.Marshal(resource.Properties)
		batch.Queue("INSERT into resources values($1,$2,$3)", resource.UID, clusterName, string(json))
		count++
		if count == BATCH_SIZE {
			wg.Add(1)
			go dao.sendBatch(*batch, wg)
			count = 0
			batch = &pgx.Batch{}
		}
	}
	// UPDATE
	for _, resource := range event.UpdateResources {
		json, _ := json.Marshal(resource.Properties)
		batch.Queue("UPDATE resources SET data=$2 WHERE uid=$1", resource.UID, string(json))
		count++
		if count == BATCH_SIZE {
			wg.Add(1)
			go dao.sendBatch(*batch, wg)
			count = 0
			batch = &pgx.Batch{}
		}
	}

	// ADD EDGES
	for _, edge := range event.AddEdges {
		batch.Queue("INSERT into edges values($1,$2)", edge.SourceUID, edge.DestUID)
		count++
		if count == BATCH_SIZE {
			wg.Add(1)
			go dao.sendBatch(*batch, wg)
			count = 0
			batch = &pgx.Batch{}
		}
	}

	// UPDATE EDGES
	// We don't need update. The collector only sends add and delete for edges.

	// DELETE - NODE AND EDGES
	if len(event.DeleteResources) > 0 {
		uids := make([]string, len(event.DeleteResources))
		for i, resource := range event.DeleteResources {
			uids[i] = resource.UID
		}
		batch.Queue("DELETE from resources WHERE uid IN ($1)", strings.Join(uids, ", "))
		batch.Queue("DELETE from edges WHERE sourceId IN ($1)", strings.Join(uids, ", "))
		batch.Queue("DELETE from edges WHERE destId IN ($1)", strings.Join(uids, ", "))
		count += 3
	}
	if count > 0 {
		wg.Add(1)
		go dao.sendBatch(*batch, wg)
	}

	wg.Wait()

}

func (dao *DAO) sendBatch(batch pgx.Batch, wg *sync.WaitGroup) {
	defer wg.Done()
	// klog.Info("Sending batch")
	br := dao.pool.SendBatch(context.Background(), &batch)
	res, err := br.Exec()
	if err != nil {
		klog.Error("Error sending batch. res: ", res, "  err: ", err, batch.Len())

		// TODO: Need to report the errors back.
	}
	// klog.Info("Batch response: ", res)
	br.Close()
}
