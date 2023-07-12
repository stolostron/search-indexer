// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/stolostron/search-indexer/pkg/metrics"
	"github.com/stolostron/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

// Reset data for the cluster to the incoming state.
func (dao *DAO) ResyncData(ctx context.Context, event model.SyncEvent,
	clusterName string, syncResponse *model.SyncResponse) error {

	defer metrics.SlowLog(fmt.Sprintf("Slow resync from cluster %s", clusterName), 0)()
	klog.Infof(
		"Starting resync from %12s. This is normal, but it could be a problem if it happens often.", clusterName)

	// FUTURE: Parallelize the resync of resources and edges.

	// Reset resources
	err := dao.resetResources(ctx, event.AddResources, clusterName, syncResponse)
	if err != nil {
		klog.Warningf("Error resyncing resources for cluster %12s. Error: %+v", clusterName, err)
		return err
	}

	// Reset edges
	err = dao.resetEdges(ctx, event.AddEdges, clusterName, syncResponse)
	if err != nil {
		klog.Warningf("Error resyncing edges for cluster %12s. Error: %+v", clusterName, err)
		return err
	}

	klog.V(1).Infof("Completed resync of cluster %12s.\t RequestId: %d", clusterName, event.RequestId)
	return nil
}

// Reset Resources.
//	1. Create a map of incoming resources.
//	2. Query and iterate existing resources for the cluster.
//  3. For each existing resource:
//		- DELETE if not found in the incoming resource.
//      - UPDATE if doesn't match the incoming resource.
//  4. INSERT incoming resources not found in the existing resources.
func (dao *DAO) resetResources(ctx context.Context, resources []model.Resource, clusterName string,
	syncResponse *model.SyncResponse) error {
	timer := time.Now()

	batch := NewBatchWithRetry(ctx, dao, syncResponse)

	incomingResMap := make(map[string]*model.Resource)
	for i, resource := range resources {
		incomingResMap[resource.UID] = &resources[i]
	}
	resourcesToDelete := make([]interface{}, 0)
	resourcesToUpdate := make([]*model.Resource, 0)

	// Get existing resources (UID and data) for the cluster.
	query, params, err := useGoqu(
		"SELECT uid, data FROM search.resources WHERE cluster=$1",
		[]interface{}{clusterName})
	if err == nil {
		existingRows, err := dao.pool.Query(ctx, query, params...)
		if err != nil {
			klog.Warningf("Error getting existing resource uids for cluster %12s. Error: %+v", clusterName, err)
		}
		for existingRows.Next() {
			var id, data string
			err := existingRows.Scan(&id, &data)
			if err != nil {
				klog.Warningf("Error scanning existing resource row. Error: %+v", err)
				continue
			}

			props := make(map[string]interface{})
			jsonErr := json.Unmarshal([]byte(data), &props)
			if jsonErr != nil {
				klog.Warningf("Error unmarshalling existing resource data. Error: %+v", err)
			}

			incomingResource, exists := incomingResMap[id]
			if !exists {
				// Resource needs to be deleted.
				resourcesToDelete = append(resourcesToDelete, id)
			} else if !reflect.DeepEqual(incomingResource.Properties, props) {
				// Resource needs to be updated.
				resourcesToUpdate = append(resourcesToUpdate, incomingResource)
				delete(incomingResMap, id)
			} else {
				// Resource exists and doesn't need to be updated.
				delete(incomingResMap, id)
			}
		}
		existingRows.Close()
	}
	metrics.LogStepDuration(&timer, clusterName, "QUERY existing resources.")

	// INSERT resources that weren't found in the database.
	for uid, resource := range incomingResMap {
		data, _ := json.Marshal(resource.Properties)
		// FIXME!!! Using goqu to insert is too slow. Why??
		
		// query, _, err := useGoqu(
		// 	"INSERT into search.resources values($1,$2,$3) ON CONFLICT (uid) DO NOTHING",
		// 	[]interface{}{uid, clusterName, string(data)})
		// if err == nil {

		queueErr := batch.Queue(batchItem{
			action: "addResource",
			query:  `INSERT into search.resources values($1,$2,$3) ON CONFLICT (uid) DO NOTHING`,
			uid:    uid,
			args:   []interface{}{uid, clusterName, string(data)},
		})
		if queueErr != nil {
			klog.Warningf("Error queuing resources to add. Error: %+v", queueErr)
			return queueErr
		}
		// }
	}

	// UPDATE resources that have changed.
	for _, resource := range resourcesToUpdate {
		data, _ := json.Marshal(resource.Properties)
		query, params, err := useGoqu(
			"UPDATE search.resources SET data=$2 WHERE uid=$1",
			[]interface{}{resource.UID, string(data)})
		if err == nil {
			queueErr := batch.Queue(batchItem{
				action: "updateResource",
				query:  query,
				uid:    resource.UID,
				args:   params,
			})
			if queueErr != nil {
				klog.Warningf("Error queuing resources to update. Error: %+v", queueErr)
				return queueErr
			}
		}
	}

	// DELETE resources that no longer exist and their edges.
	if len(resourcesToDelete) > 0 {
		query, params, err := useGoqu(
			"DELETE from search.resources WHERE uid IN ($1)",
			resourcesToDelete)
		if err == nil {
			queueErr := batch.Queue(batchItem{
				action: "deleteResources",
				query:  query,
				uid:    fmt.Sprintf("%s", resourcesToDelete),
				args:   params,
			})
			if queueErr != nil {
				klog.Warningf("Error queuing resources for deletion. Error: %+v", queueErr)
			}
		}

		// DELETE edges that point to deleted resources.
		query, params, err = useGoqu(
			"DELETE from search.edges WHERE sourceId IN ($1) OR destId IN ($1)",
			resourcesToDelete)
		if err == nil {
			queueErr := batch.Queue(batchItem{
				action: "deleteEdges",
				query:  query,
				uid:    fmt.Sprintf("%s", resourcesToDelete),
				args:   params,
			})
			if queueErr != nil {
				klog.Warningf("Error queing edges for deletion. Error: %+v", queueErr)
			}
		}
	}
	batch.flush()
	batch.wg.Wait()
	metrics.LogStepDuration(&timer, clusterName,
		fmt.Sprintf("Reset resources stats: UNCHANGED [%d] INSERT [%d] UPDATE [%d] DELETE [%d]",
			len(resources)-len(incomingResMap)-len(resourcesToUpdate),
			len(incomingResMap), len(resourcesToUpdate), len(resourcesToDelete)))

	return nil
}

// Reset Edges
//	1. Get existing edges for the cluster. Excluding intercluster edges.
//  2. For each incoming edge, INSERT if it doesn't exist.
//  3. Delete any existing edges that aren't in the incoming sync event.
func (dao *DAO) resetEdges(ctx context.Context, edges []model.Edge, clusterName string,
	syncResponse *model.SyncResponse) error {
	timer := time.Now()

	batch := NewBatchWithRetry(ctx, dao, syncResponse)

	var queueErr error
	existingEdgesMap := make(map[string]model.Edge)

	// Get all existing edges for the cluster.
	query, params, err := useGoqu(
		"SELECT sourceid, edgetype, destid FROM search.edges WHERE edgetype!='interCluster' AND cluster=$1",
		[]interface{}{clusterName})
	if err == nil {
		// edgeRow, err := dao.pool.Query(ctx, "SELECT sourceId,edgeType,destId FROM search.edges WHERE edgetype!='interCluster' AND cluster=$1", clusterName)
		edgeRow, err := dao.pool.Query(ctx, query, params...)
		if err != nil {
			klog.Warningf("Error getting existing edges during resync of cluster %12s. Error: %+v", clusterName, err)
		}

		for edgeRow.Next() {
			edge := model.Edge{}
			err := edgeRow.Scan(&edge.SourceUID, &edge.EdgeType, &edge.DestUID)
			if err != nil {
				klog.Warningf("Error scanning edge row. Error: %+v", err)
				continue
			}
			existingEdgesMap[edge.SourceUID+edge.EdgeType+edge.DestUID] = edge
		}
		edgeRow.Close()
	}
	metrics.LogStepDuration(&timer, clusterName, "Resync QUERY existing edges")

	// Now compare existing edges with the new edges.
	for _, edge := range edges {
		// If the edge already exists, do nothing.
		if _, ok := existingEdgesMap[edge.SourceUID+edge.EdgeType+edge.DestUID]; ok {
			delete(existingEdgesMap, edge.SourceUID+edge.EdgeType+edge.DestUID)
			continue
		}
		// If the edge doesn't exist, add it.
		// FIXME!!! Using goqu to insert is too slow. Why??
		// query, params, err := useGoqu(
		// 	"INSERT into search.edges values($1,$2,$3,$4,$5,$6) ON CONFLICT (sourceid, destid, edgetype) DO NOTHING",
		// 	[]interface{}{edge.SourceUID, edge.SourceKind, edge.DestUID, edge.DestKind, edge.EdgeType, clusterName})
		// if err == nil {
		queueErr = batch.Queue(batchItem{
			action: "addEdge",
			query:  "INSERT into search.edges values($1,$2,$3,$4,$5,$6) ON CONFLICT (sourceid, destid, edgetype) DO NOTHING",
			uid:    edge.SourceUID,
			args:   []interface{}{edge.SourceUID, edge.SourceKind, edge.DestUID, edge.DestKind, edge.EdgeType, clusterName},
		})
		if queueErr != nil {
			klog.Warningf("Error queuing edges. Error: %+v", queueErr)
			return queueErr
		}
		// }
	}

	// Delete existing edges that are not in the new sync event.
	for _, edge := range existingEdgesMap {
		query, params, err := useGoqu(
			"DELETE from search.edges WHERE sourceid=$1 AND destid=$2 AND edgetype=$3",
			[]interface{}{edge.SourceUID, edge.DestUID, edge.EdgeType})
		if err == nil {
			queueErr = batch.Queue(batchItem{
				action: "deleteEdge",
				query:  query,
				uid:    edge.SourceUID,
				args:   params,
			})
			if queueErr != nil {
				klog.Warningf("Error queuing edges. Error: %+v", queueErr)
				return queueErr
			}
		}
	}

	batch.flush()
	batch.wg.Wait()
	metrics.LogStepDuration(&timer, clusterName, fmt.Sprintf("Reset edges stats: INSERT [%d] DELETE [%d]",
		len(edges), len(existingEdgesMap)))
	return nil
}
