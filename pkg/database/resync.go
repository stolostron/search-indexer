// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/stolostron/search-indexer/pkg/metrics"
	"github.com/stolostron/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

func logStepDuration(timer *time.Time, cluster, message string) {
	klog.Infof("\t> %6s\t - [%10s] %s", time.Since(*timer).Round(time.Millisecond), cluster, message)
	*timer = time.Now()
}

// Resets cluster data to the incoming state.
func (dao *DAO) ResyncData(ctx context.Context, event model.SyncEvent,
	clusterName string, syncResponse *model.SyncResponse) error {

	defer metrics.SlowLog(fmt.Sprintf("Slow Resync from cluster %s", clusterName), 0)()
	klog.Infof(
		"Starting Resync of cluster %s. This is normal, but it could be a problem if it happens often.",
		clusterName)

	wg := &sync.WaitGroup{}
	wg.Add(2)
	// Reset resources
	dao.resyncResources(ctx, wg, event.AddResources, clusterName, syncResponse)

	// Reset edges
	dao.resyncEdges(ctx, wg, event.AddEdges, clusterName, syncResponse)
	wg.Wait()

	klog.V(1).Infof("Completed resync of cluster %s", clusterName)
	//return queueErr // TODO
	return nil
}

func (dao *DAO) resyncResources(ctx context.Context, wg *sync.WaitGroup, resources []model.Resource, clusterName string, syncResponse *model.SyncResponse) {
	defer wg.Done()
	timer := time.Now()

	batch := NewBatchWithRetry(ctx, dao, syncResponse)
	newUIDs := make([]string, len(resources))

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
	logStepDuration(&timer, clusterName, "QUERY existing resources")

	// INSERT or UPDATE resources.
	// In case of conflict update only if data has changed.
	for i, resource := range resources {
		delete(existingResourcesMap, resource.UID)
		data, _ := json.Marshal(resource.Properties)
		newUIDs[i] = "'" + resource.UID + "'" // TODO: Is there a better alternative to add the quotes?
		queueErr := batch.Queue(batchItem{
			action: "addResource",
			query: `INSERT into search.resources as r values($1,$2,$3) ON CONFLICT (uid)
			DO UPDATE SET data=$3 WHERE r.uid=$1 and r.data IS DISTINCT FROM $3`,
			uid:  resource.UID,
			args: []interface{}{resource.UID, clusterName, string(data)},
		})
		if queueErr != nil {
			klog.Warningf("Error queuing resources. Error: %+v", queueErr)
			// return queueErr // TODO
		}
	}
	batch.flush()
	// batch.wg.Wait()
	// logStepDuration(&timer, clusterName, fmt.Sprintf("Resync INSERT/UPDATE [%d] resources", len(resources)))

	// DELETE any previous resources for the cluster that isn't included in the incoming resync event.

	if len(existingResourcesMap) > 0 {
		resourcesToDelete := make([]string, 0)
		for resourceUID := range existingResourcesMap {
			resourcesToDelete = append(resourcesToDelete, "'"+resourceUID+"'") // TODO: Is there a better alternative to add the quotes?
		}

		queryStr := fmt.Sprintf("DELETE from search.resources WHERE uid IN (%s)", strings.Join(resourcesToDelete, ","))

		deletedRows, err := dao.pool.Exec(ctx, queryStr)
		if err != nil {
			klog.Warningf("Error deleting resources during resync of cluster %s. Error: %+v", clusterName, err)
		}
		klog.Infof("Deleted %d resources during resync of cluster %s", deletedRows.RowsAffected(), clusterName)
	}
	batch.wg.Wait()
	logStepDuration(&timer, clusterName, fmt.Sprintf("Resync INSERT/UPDATE [%d] DELETE [%d] resources", len(resources), len(existingResourcesMap)))
}

// Reset Edges
func (dao *DAO) resyncEdges(ctx context.Context, wg *sync.WaitGroup, edges []model.Edge, clusterName string, syncResponse *model.SyncResponse) {
	defer wg.Done()
	timer := time.Now()

	batch := NewBatchWithRetry(ctx, dao, syncResponse)
	var queueErr error

	// Get all existing edges for the cluster.
	edgeRow, err := dao.pool.Query(ctx, "SELECT sourceId,edgeType,destId FROM search.edges WHERE cluster=$1", clusterName)
	if err != nil {
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

	logStepDuration(&timer, clusterName, "Resync QUERY existing edges")

	// Now compare existing edges with the new edges.
	for _, edge := range edges {
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

		if queueErr != nil {
			klog.Warningf("Error queuing edges. Error: %+v", queueErr)
			// return queueErr // TODO
		}
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
		if queueErr != nil {
			klog.Warningf("Error queuing edges. Error: %+v", queueErr)
			// return queueErr // TODO
		}
	}

	batch.flush()
	batch.wg.Wait()
	logStepDuration(&timer, clusterName, fmt.Sprintf("Resync INSERT [%d] DELETE [%d] edges", len(edges), len(existingEdgesMap)))
}
