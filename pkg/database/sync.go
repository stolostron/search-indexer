// Copyright Contributors to the Open Cluster Management project

package database

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/stolostron/search-indexer/pkg/model"
)

func (dao *DAO) SyncData(event model.SyncEvent, clusterName string, syncResponse *model.SyncResponse) {

	batch := NewBatchWithRetry(dao, syncResponse)
	var deleted bool = true
	// ADD RESOURCES
	for _, resource := range event.AddResources {
		data, _ := json.Marshal(resource.Properties)
		updatedTime := time.Unix(resource.Time, 0)

		batch.Queue(batchItem{
			action: "addResource",
			query:  "INSERT into search.resources values($1,$2,$3)",
			uid:    resource.UID,
			args:   []interface{}{resource.UID, clusterName, string(data)},
		})

		batch.Queue(batchItem{
			action: "addResource",
			query:  "INSERT into search.resources_hist as r (uid, cluster, updated) values($1,$2,$3) ON CONFLICT (uid, updated) DO NOTHING",
			uid:    resource.UID,
			args:   []interface{}{resource.UID, clusterName, updatedTime},
		})
	}

	// UPDATE RESOURCES
	// The collector enforces that a resource isn't added and updated in the same sync event.
	// The uid and cluster fields will never get updated for a resource.
	for _, resource := range event.UpdateResources {
		data, _ := json.Marshal(resource.Properties)
		updatedTime := time.Unix(resource.Time, 0)

		batch.Queue(batchItem{
			action: "updateResource",
			query:  "UPDATE search.resources SET data=$2 WHERE uid=$1",
			uid:    resource.UID,
			args:   []interface{}{resource.UID, string(data)},
		})

		batch.Queue(batchItem{
			action: "updateResource",
			query:  "INSERT into search.resources_hist values($1,$2,$3)",
			uid:    resource.UID,
			args:   []interface{}{resource.UID, clusterName, updatedTime},
		})
	}

	// DELETE RESOURCES and all edges pointing to the resource.
	if len(event.DeleteResources) > 0 {
		params := make([]string, len(event.DeleteResources))
		uids := make([]interface{}, len(event.DeleteResources))
		for i, resource := range event.DeleteResources {
			params[i] = fmt.Sprintf("$%d", i+1)
			uids[i] = resource.UID
			deletedTime := resource.Time
			batch.Queue(batchItem{
				action: "deleteResource",
				query:  "INSERT into search.resources_hist values($1,$2,$3,$4)",
				uid:    resource.UID,
				args:   []interface{}{resource.UID, clusterName, deletedTime, deleted},
			})
		}
		paramStr := strings.Join(params, ",")

		// TODO: Need better safety for delete errors.
		// The current retry logic won't work well if there's an error here.
		batch.Queue(batchItem{
			action: "deleteResource",
			query:  fmt.Sprintf("DELETE from search.resources WHERE uid IN (%s)", paramStr),
			uid:    fmt.Sprintf("%s", uids),
			args:   uids,
		})
		batch.Queue(batchItem{
			action: "deleteResource",
			query:  fmt.Sprintf("DELETE from search.edges WHERE sourceId IN (%s) OR destId IN (%s)", paramStr, paramStr),
			uid:    fmt.Sprintf("%s", uids),
			args:   uids,
		})
	}

	// ADD EDGES
	for _, edge := range event.AddEdges {
		updatedTime := time.Now()
		batch.Queue(batchItem{
			action: "addEdge",
			query:  "INSERT into search.edges values($1,$2,$3,$4,$5,$6)",
			uid:    edge.SourceUID,
			args:   []interface{}{edge.SourceUID, edge.SourceKind, edge.DestUID, edge.DestKind, edge.EdgeType, clusterName}})

		batch.Queue(batchItem{
			action: "addEdge",
			query:  "INSERT into search.edges_hist as r (sourceId, sourceKind, destId, destKind, edgeType, cluster, updated, deleted) values($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (sourceId, destId, edgeType, deleted) DO NOTHING",
			uid:    edge.SourceUID,
			args:   []interface{}{edge.SourceUID, edge.SourceKind, edge.DestUID, edge.DestKind, edge.EdgeType, clusterName, updatedTime, !deleted}})
	}

	// UPDATE EDGES
	// Edges are never updated. The collector only sends ADD and DELETE eveents for edges.

	// DELETE EDGES
	for _, edge := range event.DeleteEdges {
		updatedTime := time.Now()
		batch.Queue(batchItem{
			action: "deleteEdge",
			query:  "DELETE from search.edges WHERE sourceId=$1 AND destId=$2 AND edgeType=$3",
			uid:    edge.SourceUID,
			args:   []interface{}{edge.SourceUID, edge.DestUID, edge.EdgeType}})

		batch.Queue(batchItem{
			action: "deleteEdge",
			query:  "INSERT into search.edges_hist values($1,$2,$3,$4,$5,$6,$7,$8)",
			uid:    edge.SourceUID,
			args:   []interface{}{edge.SourceUID, edge.SourceKind, edge.DestUID, edge.DestKind, edge.EdgeType, clusterName, updatedTime, deleted}})

	}

	// Flush remaining items in the batch.
	batch.flush()

	// Wait for all batches to complete.
	batch.wg.Wait()

	// The response fields below are redundant, these are more interesting for resync.
	syncResponse.TotalAdded = len(event.AddResources) - len(syncResponse.AddErrors)
	syncResponse.TotalUpdated = len(event.UpdateResources) - len(syncResponse.UpdateErrors)
	syncResponse.TotalDeleted = len(event.DeleteResources) - len(syncResponse.DeleteErrors)
	syncResponse.TotalEdgesAdded = len(event.AddEdges) - len(syncResponse.AddEdgeErrors)
	syncResponse.TotalEdgesDeleted = len(event.DeleteEdges) - len(syncResponse.DeleteEdgeErrors)
}
