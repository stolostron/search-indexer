// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/stolostron/search-indexer/pkg/metrics"
	"github.com/stolostron/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

// Sets cluster data to incoming state.
func (dao *DAO) ResyncData(ctx context.Context, event model.SyncEvent,
	clusterName string, syncResponse *model.SyncResponse) error {
	start := time.Now()

	defer metrics.SlowLog(fmt.Sprintf("@@@ Slow Resync from cluster %s", clusterName), 0)()
	klog.Infof(
		"Starting Resync of cluster %s. This is normal, but it could be a problem if it happens often.",
		clusterName)

	batch := NewBatchWithRetry(ctx, dao, syncResponse)
	var queueErr error
	newUIDs := make([]string, len(event.AddResources))

	// Get existing resources for the cluster.
	existingResourcesMap := make(map[string]struct{})
	existingRows, err := dao.pool.Query(ctx, "SELECT uid FROM search.resources WHERE cluster=$1", clusterName)
	if err != nil {
		klog.Warningf("Error getting existing resources uids of cluster %s. Error: %+v", clusterName, err)
	}
	defer existingRows.Close()
	for existingRows.Next() {
		id := ""
		err := existingRows.Scan(&id)
		if err != nil {
			klog.Warningf("Error scanning existing resource row. Error: %+v", err)
			continue
		}
		existingResourcesMap[id] = struct{}{}
	}
	klog.Infof("\t> %6s - [%10s] QUERY existing resources", time.Since(start).Round(time.Millisecond), clusterName)
	start = time.Now()

	// INSERT RESOURCES
	// In case of conflict update only if data has changed.
	for i, resource := range event.AddResources {
		delete(existingResourcesMap, resource.UID)
		data, _ := json.Marshal(resource.Properties)
		newUIDs[i] = "'" + resource.UID + "'" // TODO: Is there a better alternative to quotes?
		queueErr = batch.Queue(batchItem{
			action: "addResource",
			query: `INSERT into search.resources as r values($1,$2,$3) ON CONFLICT (uid) 
			DO UPDATE SET data=$3 WHERE r.uid=$1 and r.data IS DISTINCT FROM $3`,
			uid:  resource.UID,
			args: []interface{}{resource.UID, clusterName, string(data)},
		})
		if queueErr != nil {
			klog.Warningf("Error queuing resources. Error: %+v", queueErr)
			return queueErr
		}
	}
	batch.flush()
	batch.wg.Wait()
	klog.Infof("\t> %6s - [%10s] Resync INSERT resources", time.Since(start).Round(time.Millisecond), clusterName)
	start = time.Now()

	// DELETE any previous resources for the cluster that isn't included in the incoming resync event.

	resourcesToDelete := make([]string, 0)
	for resourceUID := range existingResourcesMap {
		resourcesToDelete = append(resourcesToDelete, "'"+resourceUID+"'") // TODO: alternative to quotes.
	}

	if len(resourcesToDelete) > 0 {
		queryStr := fmt.Sprintf("DELETE from search.resources WHERE uid IN (%s)", strings.Join(resourcesToDelete, ","))

		deletedRows, err := dao.pool.Query(ctx, queryStr)
		if err != nil {
			klog.Warningf("Error deleting resources during resync of cluster %s. Error: %+v", clusterName, err)
		}
		defer deletedRows.Close()
	}
	klog.Infof("\t> %6s - [%10s] Resync DELETE resources", time.Since(start).Round(time.Millisecond), clusterName)
	start = time.Now()

	// Delete edges pointing to deleted resources.
	if len(resourcesToDelete) > 0 {
		deletedUIDsStr := strings.Join(resourcesToDelete, ",")
		queueErr = batch.Queue(batchItem{
			action: "deleteResourceEdges",
			query:  fmt.Sprintf("DELETE FROM search.edges WHERE sourceId IN (%s) OR destId IN (%s)", deletedUIDsStr, deletedUIDsStr),
			uid:    deletedUIDsStr,
			// args:   deletedUIDs,
		})
	}
	klog.Infof("\t> %6s - [%10s] Resync DELETE edges-from-resource", time.Since(start).Round(time.Millisecond), clusterName)
	start = time.Now()

	// UPDATE Edges

	// Get all existing edges for the cluster.
	edgeRow, errEdges := dao.pool.Query(ctx, "SELECT sourceId,edgeType,destId FROM search.edges WHERE cluster=$1", clusterName)
	if errEdges != nil {
		klog.Warningf("Error getting existing edges during resync of cluster %s. Error: %+v", clusterName, err)
	}

	defer edgeRow.Close()

	existingEdgesMap := make(map[string]model.Edge)
	for edgeRow.Next() {
		edge := model.Edge{}
		err := edgeRow.Scan(&edge.SourceUID, &edge.EdgeType, &edge.DestUID)
		if err != nil {
			klog.Warningf("Error scanning edge row. Error: %+v", err)
			continue
		}
		existingEdgesMap[edge.SourceUID+edge.EdgeType+edge.DestUID] = edge
	}
	klog.Infof("\t> %6s - [%10s] Resync QUERY existing edges", time.Since(start).Round(time.Millisecond), clusterName)

	// Now compare existing edges with the new edges.
	for _, edge := range event.AddEdges {
		// If the edge already exists, do nothing.
		if _, ok := existingEdgesMap[edge.SourceUID+edge.EdgeType+edge.DestUID]; ok {
			delete(existingEdgesMap, edge.SourceUID+edge.EdgeType+edge.DestUID)
			continue
		}
		// If the edge doesn't exist, add it.
		queueErr = batch.Queue(batchItem{
			action: "addEdge",
			query:  "INSERT into search.edges values($1,$2,$3,$4,$5,$6) ON CONFLICT (sourceid, destid, edgetype) DO NOTHING",
			uid:    edge.SourceUID,
			args:   []interface{}{edge.SourceUID, edge.SourceKind, edge.DestUID, edge.DestKind, edge.EdgeType, clusterName}})
	}

	// Delete existing edges that are not in the new sync event.
	for _, edge := range existingEdgesMap {
		// If the edge already exists, do nothing.
		queueErr = batch.Queue(batchItem{
			action: "deleteEdge",
			query:  "DELETE from search.edges WHERE sourceid=$1 AND destid=$2 AND edgetype=$3",
			uid:    edge.SourceUID,
			args:   []interface{}{edge.SourceUID, edge.DestUID, edge.EdgeType},
		})
	}

	batch.flush()
	batch.wg.Wait()
	klog.Infof("\t> %6s - [%10s] Resync INSERT/DELETE edges", time.Since(start).Round(time.Millisecond), clusterName)

	klog.V(1).Infof("Completed resync of cluster %s", clusterName)
	return queueErr
}
