// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stolostron/search-indexer/pkg/metrics"
	"github.com/stolostron/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

// validateUIDPrefix rejects resource UIDs that don't begin with "<clusterName>/".
// Collector-assigned UIDs always follow this convention. Enforcing it here prevents
// a spoke from crafting a payload that targets another cluster's rows by uid.
func validateUIDPrefix(uid, clusterName string) error {
	prefix := clusterName + "/"
	if !strings.HasPrefix(uid, prefix) && uid != "cluster__"+clusterName {
		return fmt.Errorf("uid %q does not start with expected prefix %q", uid, prefix)
	}
	return nil
}

func (dao *DAO) SyncDataShared(ctx context.Context, event model.SyncEvent,
	clusterName string, syncResponse *model.SyncResponse) error {

	allItems := make([]sharedBatchItem, 0)

	// ADD RESOURCES
	// In case of conflict update only if data has changed AND the row is owned by this cluster.
	// The cluster guard on the conflict target prevents a spoke from overwriting another spoke's
	// row by submitting a resource with a colliding UID.
	for _, resource := range event.AddResources {
		if err := validateUIDPrefix(resource.UID, clusterName); err != nil {
			klog.Warningf("Rejecting addResource from cluster [%s]: %v", clusterName, err)
			syncResponse.AddErrors = append(syncResponse.AddErrors, model.SyncError{ResourceUID: resource.UID, Message: err.Error()})
			continue
		}
		data, _ := json.Marshal(resource.Properties)
		allItems = append(allItems, sharedBatchItem{
			batchItem: batchItem{
				action: "addResource",
				query: `INSERT into search.resources as r values($1,$2,$3) ON CONFLICT (uid)
				DO UPDATE SET data=$3 WHERE r.uid=$1 AND r.cluster=$2 AND r.data IS DISTINCT FROM $3`,
				uid:  resource.UID,
				args: []interface{}{resource.UID, clusterName, string(data)},
			},
			onError: func(msg string) {
				syncResponse.AddErrors = append(syncResponse.AddErrors,
					model.SyncError{ResourceUID: resource.UID, Message: msg})
			},
		})
		kind := resource.Properties["kind"].(string)
		metrics.IncrementDBResourceEventSent("insert", kind, clusterName)
	}

	// UPDATE RESOURCES
	// The collector enforces that a resource isn't added and updated in the same sync event.
	// The uid and cluster fields will never get updated for a resource.
	// AND cluster=$3 ensures a spoke can only update rows it owns.
	for _, resource := range event.UpdateResources {
		if err := validateUIDPrefix(resource.UID, clusterName); err != nil {
			klog.Warningf("Rejecting updateResource from cluster [%s]: %v", clusterName, err)
			syncResponse.UpdateErrors = append(syncResponse.UpdateErrors, model.SyncError{ResourceUID: resource.UID, Message: err.Error()})
			continue
		}
		data, _ := json.Marshal(resource.Properties)
		allItems = append(allItems, sharedBatchItem{
			batchItem: batchItem{
				action: "updateResource",
				query:  "UPDATE search.resources SET data=$2 WHERE uid=$1 AND cluster=$3",
				uid:    resource.UID,
				args:   []interface{}{resource.UID, string(data), clusterName},
			},
			onError: func(msg string) {
				syncResponse.UpdateErrors = append(syncResponse.UpdateErrors,
					model.SyncError{ResourceUID: resource.UID, Message: msg})
			},
		})
		kind := resource.Properties["kind"].(string)
		metrics.IncrementDBResourceEventSent("update", kind, clusterName)
	}

	// DELETE RESOURCES and all edges pointing to the resource.
	// AND cluster=$N ensures a spoke can only delete its own rows.
	if len(event.DeleteResources) > 0 {
		params := make([]string, len(event.DeleteResources))
		uids := make([]interface{}, len(event.DeleteResources))
		for i, resource := range event.DeleteResources {
			params[i] = fmt.Sprintf("$%d", i+2)
			uids[i] = resource.UID
			metrics.IncrementDBResourceEventSent("delete", "", clusterName)
		}
		paramStr := strings.Join(params, ",")
		args := append([]interface{}{clusterName}, uids...)

		deleteUID := fmt.Sprintf("%s", uids)
		allItems = append(allItems, sharedBatchItem{
			batchItem: batchItem{
				action: "deleteResource",
				query:  fmt.Sprintf("DELETE from search.resources WHERE cluster=$1 AND uid IN (%s)", paramStr),
				uid:    deleteUID,
				args:   args,
			},
			onError: func(msg string) {
				syncResponse.DeleteErrors = append(syncResponse.DeleteErrors,
					model.SyncError{ResourceUID: deleteUID, Message: msg})
			},
		})

		// Delete edges for the removed resources.
		allItems = append(allItems, sharedBatchItem{
			batchItem: batchItem{
				action: "deleteEdge",
				query:  fmt.Sprintf("DELETE from search.edges WHERE cluster=$1 AND (sourceid IN (%s) OR destid IN (%s))", paramStr, paramStr),
				uid:    deleteUID,
				args:   args,
			},
			onError: func(msg string) {
				syncResponse.DeleteEdgeErrors = append(syncResponse.DeleteEdgeErrors,
					model.SyncError{ResourceUID: deleteUID, Message: msg})
			},
		})
	}

	// ADD EDGES
	// Nothing to update in case of conflict as resource kind cannot change
	for _, edge := range event.AddEdges {
		allItems = append(allItems, sharedBatchItem{
			batchItem: batchItem{
				action: "addEdge",
				query:  "INSERT into search.edges values($1,$2,$3,$4,$5,$6) ON CONFLICT (sourceid, destid, edgetype) DO NOTHING",
				uid:    edge.SourceUID,
				args:   []interface{}{edge.SourceUID, edge.SourceKind, edge.DestUID, edge.DestKind, edge.EdgeType, clusterName},
			},
			onError: func(msg string) {
				syncResponse.AddEdgeErrors = append(syncResponse.AddEdgeErrors,
					model.SyncError{ResourceUID: edge.SourceUID, Message: msg})
			},
		})
	}

	// UPDATE EDGES
	// Edges are never updated. The collector only sends ADD and DELETE events for edges.

	// DELETE EDGES
	// AND cluster=$4 ensures a spoke can only delete edges it owns.
	for _, edge := range event.DeleteEdges {
		allItems = append(allItems, sharedBatchItem{
			batchItem: batchItem{
				action: "deleteEdge",
				query:  "DELETE from search.edges WHERE sourceid=$1 AND destid=$2 AND edgetype=$3 AND cluster=$4",
				uid:    edge.SourceUID,
				args:   []interface{}{edge.SourceUID, edge.DestUID, edge.EdgeType, clusterName},
			},
			onError: func(msg string) {
				syncResponse.DeleteEdgeErrors = append(syncResponse.DeleteEdgeErrors,
					model.SyncError{ResourceUID: edge.SourceUID, Message: msg})
			},
		})
	}

	err := dao.Accumulator.SubmitAndWait(ctx, clusterName, allItems)
	if err != nil {
		klog.V(1).Infof("Completed sync of cluster %12s with errors.", clusterName)
		return err
	}

	syncResponse.TotalAdded = len(event.AddResources) - len(syncResponse.AddErrors)
	syncResponse.TotalUpdated = len(event.UpdateResources) - len(syncResponse.UpdateErrors)
	syncResponse.TotalDeleted = len(event.DeleteResources) - len(syncResponse.DeleteErrors)
	syncResponse.TotalEdgesAdded = len(event.AddEdges) - len(syncResponse.AddEdgeErrors)
	syncResponse.TotalEdgesDeleted = len(event.DeleteEdges) - len(syncResponse.DeleteEdgeErrors)

	klog.V(1).Infof("Completed sync of cluster %12s", clusterName)
	return nil
}
