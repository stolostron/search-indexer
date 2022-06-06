// Copyright Contributors to the Open Cluster Management project

package database

import (
	"encoding/json"
	"strings"

	"github.com/stolostron/search-indexer/pkg/model"
)

func (dao *DAO) SyncData(event model.SyncEvent, clusterName string, syncResponse *model.SyncResponse) {

	batch := NewBatchWithRetry(dao, syncResponse)

	// ADD RESOURCES
	for _, resource := range event.AddResources {
		data, _ := json.Marshal(resource.Properties)
		batch.Queue(batchItem{
			action: "addResource",
			query:  "INSERT into search.resources values($1,$2,$3)",
			uid:    resource.UID,
			args:   []interface{}{resource.UID, clusterName, strings.ToLower(string(data))},
		})
	}

	// UPDATE RESOURCES
	// The collector enforces that a resource isn't added and updated in the same sync event.
	// The uid and cluster fields will never get updated for a resource.
	for _, resource := range event.UpdateResources {
		data, _ := json.Marshal(resource.Properties)
		batch.Queue(batchItem{
			action: "updateResource",
			query:  "UPDATE search.resources SET data=$2 WHERE uid=$1",
			uid:    resource.UID,
			args:   []interface{}{resource.UID, strings.ToLower(string(data))},
		})
	}

	// DELETE RESOURCES and all edges pointing to the resource.
	if len(event.DeleteResources) > 0 {
		uids := make([]string, len(event.DeleteResources))
		for i, resource := range event.DeleteResources {
			uids[i] = resource.UID
		}

		// TODO: Need better safety for delete errors.
		// The current retry logic won't work well if there's an error here.
		batch.Queue(batchItem{
			action: "deleteResource",
			query:  "DELETE from search.resources WHERE uid IN ($1)",
			uid:    strings.Join(uids, ", "),
			args:   []interface{}{strings.Join(uids, ", ")},
		})
		batch.Queue(batchItem{
			action: "deleteResource",
			query:  "DELETE from search.edges WHERE sourceId IN ($1) OR destId IN ($1)",
			uid:    strings.Join(uids, ", "),
			args:   []interface{}{strings.Join(uids, ", ")},
		})
	}

	// ADD EDGES
	for _, edge := range event.AddEdges {
		batch.Queue(batchItem{
			action: "addEdge",
			query:  "INSERT into search.edges values($1,$2,$3,$4,$5,$6)",
			uid:    edge.SourceUID,
			args:   []interface{}{edge.SourceUID, edge.SourceKind, edge.DestUID, edge.DestKind, edge.EdgeType, clusterName}})
	}

	// UPDATE EDGES
	// Edges are never updated. The collector only sends ADD and DELETE eveents for edges.

	// DELETE EDGES
	for _, edge := range event.DeleteEdges {
		batch.Queue(batchItem{
			action: "deleteEdge",
			query:  "DELETE from search.edges WHERE sourceId=$1 AND destId=$2 AND edgeType=$3",
			uid:    edge.SourceUID,
			args:   []interface{}{edge.SourceUID, edge.DestUID, edge.EdgeType}})
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
