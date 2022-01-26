// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	pgx "github.com/jackc/pgx/v4"
	"github.com/stolostron/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

func (dao *DAO) SyncData(event model.SyncEvent, clusterName string) {
	wg := &sync.WaitGroup{}
	batch := &pgx.Batch{}
	count := 0

	// ADD
	for _, resource := range event.AddResources {
		data, _ := json.Marshal(resource.Properties)

		propData, _ := json.Marshal(resource.Properties)
		primaryProp := resource.PrimaryProperties
		err := json.Unmarshal([]byte(propData), &primaryProp)
		if err != nil {
			klog.Warning("Unmarshaling error", err)
		}
		batch.Queue("INSERT into search.resources values($1,$2,$3,$4,$5,$6)", resource.UID, clusterName, string(data), primaryProp.Kind, primaryProp.Name, primaryProp.NameSpace)
		count++
		if count == dao.batchSize {
			wg.Add(1)
			go dao.sendBatch(*batch, wg)
			count = 0
			batch = &pgx.Batch{}
		}
	}
	// UPDATE
	for _, resource := range event.UpdateResources {
		json, _ := json.Marshal(resource.Properties)
		batch.Queue("UPDATE search.resources SET data=$2 WHERE uid=$1", resource.UID, string(json))
		count++
		if count == dao.batchSize {
			wg.Add(1)
			go dao.sendBatch(*batch, wg)
			count = 0
			batch = &pgx.Batch{}
		}
	}

	// ADD EDGES
	for _, edge := range event.AddEdges {
		batch.Queue("INSERT into search.edges values($1,$2,$3,$4,$5)", edge.SourceUID, edge.SourceKind, edge.DestUID, edge.DestKind, edge.EdgeType)
		count++
		if count == dao.batchSize {
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
		batch.Queue("DELETE from search.resources WHERE uid IN ($1)", strings.Join(uids, ", "))
		batch.Queue("DELETE from search.edges WHERE sourceId IN ($1)", strings.Join(uids, ", "))
		batch.Queue("DELETE from search.edges WHERE destId IN ($1)", strings.Join(uids, ", "))
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
	br := dao.pool.SendBatch(context.Background(), &batch)
	defer br.Close()
	_, err := br.Exec()
	if err != nil {
		klog.Error("Error sending batch.", err)

		// TODO: Need to report the errors back in response body.
	}
}
