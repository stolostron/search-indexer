// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/stolostron/search-indexer/pkg/config"
	"github.com/stolostron/search-indexer/pkg/metrics"
	"github.com/stolostron/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

// Reset data for the cluster to the incoming state.
// body is consumed as a streaming JSON reader to avoid buffering the full (potentially large) payload in memory.
// The top-level JSON object is scanned in a single pass so that addResources and addEdges are handled
// correctly regardless of which key appears first in the payload.
func (dao *DAO) ResyncData(ctx context.Context, clusterName string, syncResponse *model.SyncResponse, body io.Reader) error {

	defer metrics.SlowLog(fmt.Sprintf("Slow resync from %12s.", clusterName), 0)()
	klog.Infof(
		"Starting resync from %12s. This is normal, but it could be a problem if it happens often.", clusterName)

	dec := json.NewDecoder(body)

	// Read the opening '{' of the top-level object.
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("error reading resync payload opening token: %v", err)
	}

	// Prepare the resource batch upfront so both sections can populate it.
	resourceBatch := NewBatchWithRetry(ctx, dao, syncResponse)
	var incomingUIDs []interface{}
	var lastUpsertResource model.Resource
	var resourceErr error

	// Pre-fetch existing edges before scanning the payload so we can compare
	// inline as edges are encountered regardless of their position in the object.
	existingEdgesMap, edgeFetchErr := dao.fetchExistingEdges(ctx, clusterName)
	if edgeFetchErr != nil {
		klog.Warningf("Error fetching existing edges during resync of cluster %12s. Error: %+v", clusterName, edgeFetchErr)
		// Continue — addEdges will insert all incoming edges and none will be deleted.
	}
	edgeBatch := NewBatchWithRetry(ctx, dao, syncResponse)

	// Single pass over the top-level JSON keys — order-independent dispatch.
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return fmt.Errorf("error reading resync payload key: %v", err)
		}
		switch key {
		case "addResources":
			incomingUIDs, lastUpsertResource, resourceErr = dao.upsertResources(ctx, dec, clusterName, syncResponse, &resourceBatch)
		case "addEdges":
			if err := addEdges(dec, &existingEdgesMap, clusterName, syncResponse, &edgeBatch); err != nil {
				klog.Warningf("Error processing edges for cluster %12s. Error: %+v", clusterName, err)
			}
		default:
			// Skip unknown or irrelevant top-level fields (e.g. updateResources, deleteResources
			// are handled by SyncData for delta syncs, not here).
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil && err != io.EOF {
				klog.V(4).Infof("Skipping unknown resync field [%v] for cluster %12s", key, clusterName)
			}
		}
	}

	// Flush resource upserts and deletions.
	if err := dao.resetResources(ctx, clusterName, syncResponse, &resourceBatch, incomingUIDs); err != nil {
		klog.Warningf("Error resyncing resources for cluster %12s. Error: %+v", clusterName, err)
		return err
	}
	if resourceErr != nil {
		return resourceErr
	}

	// Flush edge inserts and delete stale edges.
	if err := dao.resetEdges(ctx, clusterName, syncResponse, &edgeBatch, existingEdgesMap); err != nil {
		klog.Warningf("Error resyncing edges for cluster %12s. Error: %+v", clusterName, err)
		return err
	}

	if _, ok := lastUpsertResource.Properties["_hubClusterResource"]; ok {
		go dao.hubClusterCleanUpWithRetry(context.Background(), clusterName) // #nosec G118 -- Background cleanup goroutine intentionally uses independent context
	}

	klog.V(1).Infof("Completed resync of cluster %12s.", clusterName)
	return nil
}

// resetResources deletes resources absent from the incoming set and flushes the resource batch.
// incomingUIDs is the list of UIDs received in addResources; it is used to scope the DELETE.
func (dao *DAO) resetResources(ctx context.Context, clusterName string,
	syncResponse *model.SyncResponse, batch *batchWithRetry, incomingUIDs []interface{}) error {

	// Add the uid of the Cluster pseudo node that is created by the indexer to exclude from deletion
	incomingUIDs = append(incomingUIDs, fmt.Sprintf("cluster__%s", clusterName))

	// DELETE resources that no longer exist.
	query, params, err := useGoqu(
		"DELETE from search.resources WHERE cluster=$1 AND uid NOT IN ($2)",
		[]interface{}{clusterName, incomingUIDs})
	if err == nil {
		queueErr := batch.Queue(batchItem{
			action: "deleteResource",
			query:  query,
			uid:    fmt.Sprintf("%s", incomingUIDs),
			args:   params,
		})
		if queueErr != nil {
			klog.Warningf("Error queuing resources for deletion. Error: %+v", queueErr)
		}
	}

	// DELETE edges pointing to resources that no longer exist.
	query, _, err = useGoqu(
		"DELETE from search.edges WHERE cluster=$1 AND sourceid NOT IN ($2) OR destid NOT IN ($2)",
		[]interface{}{clusterName, incomingUIDs})
	if err == nil {
		queueErr := batch.Queue(batchItem{
			action: "deleteEdge",
			query:  query,
			uid:    fmt.Sprintf("%s", incomingUIDs),
			args:   params,
		})
		if queueErr != nil {
			klog.Warningf("Error queuing edges for deletion. Error: %+v", queueErr)
		}
	}
	batch.flush()
	batch.wg.Wait()

	return batch.connError
}

// fetchExistingEdges returns all non-interCluster edges for the cluster, keyed by SourceUID+EdgeType+DestUID.
// Called before scanning the payload so edge comparison works regardless of JSON field order.
func (dao *DAO) fetchExistingEdges(ctx context.Context, clusterName string) (map[string]model.Edge, error) {
	existingEdgesMap := make(map[string]model.Edge)
	query, params, err := useGoqu(
		"SELECT sourceid, edgetype, destid FROM search.edges WHERE edgetype!='interCluster' AND cluster=$1",
		[]interface{}{clusterName})
	if err != nil {
		return existingEdgesMap, err
	}
	edgeRow, err := dao.pool.Query(ctx, query, params...)
	if err != nil {
		return existingEdgesMap, err
	}
	defer edgeRow.Close()
	for edgeRow.Next() {
		edge := model.Edge{}
		if err := edgeRow.Scan(&edge.SourceUID, &edge.EdgeType, &edge.DestUID); err != nil {
			klog.Warningf("Error scanning edge row. Error: %+v", err)
			continue
		}
		existingEdgesMap[edge.SourceUID+edge.EdgeType+edge.DestUID] = edge
	}
	return existingEdgesMap, nil
}

// resetEdges deletes stale edges (those not present in existingEdgesMap after addEdges removed matched ones)
// and flushes the edge batch.
func (dao *DAO) resetEdges(ctx context.Context, clusterName string,
	syncResponse *model.SyncResponse, batch *batchWithRetry, existingEdgesMap map[string]model.Edge) error {
	timer := time.Now()

	// Delete existing edges that were not present in the resync payload.
	// AND cluster=$4 scopes the delete to this cluster's rows only.
	for _, edge := range existingEdgesMap {
		query, params, err := useGoqu(
			"DELETE from search.edges WHERE sourceid=$1 AND destid=$2 AND edgetype=$3 AND cluster=$4",
			[]interface{}{edge.SourceUID, edge.DestUID, edge.EdgeType, clusterName})
		if err == nil {
			queueErr := batch.Queue(batchItem{
				action: "deleteEdge",
				query:  query,
				uid:    edge.SourceUID,
				args:   params,
			})
			if queueErr != nil {
				klog.Warningf("Error queuing edges. Error: %+v", queueErr)
				return queueErr
			}
			syncResponse.TotalEdgesDeleted++
		}
	}

	batch.flush()
	batch.wg.Wait()
	metrics.LogStepDuration(&timer, clusterName, fmt.Sprintf("Reset edges stats: INSERT [%d] DELETE [%d]",
		syncResponse.TotalEdgesAdded, syncResponse.TotalEdgesDeleted))

	return batch.connError
}

// upsertResources decodes the addResources array from a decoder that is already positioned
// immediately after the "addResources" key (i.e. next token is the opening '[').
// It reads the closing ']' so the outer dispatch loop can continue with the next key.
func (dao *DAO) upsertResources(ctx context.Context, dec *json.Decoder, clusterName string, syncResponse *model.SyncResponse, batch *batchWithRetry) ([]interface{}, model.Resource, error) {
	incomingUIDs := make([]interface{}, 0)
	var resource model.Resource

	// Read opening '['.
	if _, err := dec.Token(); err != nil {
		return incomingUIDs, resource, fmt.Errorf("error reading addResources opening token: %v", err)
	}
	for dec.More() {
		resource = model.Resource{}
		if err := dec.Decode(&resource); err != nil {
			return incomingUIDs, resource, fmt.Errorf("error decoding resource from request: %v", err)
		}
		uid := resource.UID
		// Reject UIDs that don't belong to this cluster before they reach the DB.
		if err := validateUIDPrefix(uid, clusterName); err != nil {
			klog.Warningf("Rejecting resync resource from cluster [%s]: %v", clusterName, err)
			syncResponse.AddErrors = append(syncResponse.AddErrors, model.SyncError{ResourceUID: uid, Message: err.Error()})
			continue
		}
		data, _ := json.Marshal(resource.Properties)
		query, params, err := useGoqu(
			"INSERT into search.resources values($1,$2,$3) ON CONFLICT (uid) DO UPDATE SET data=$3 WHERE r.cluster=$2 AND data!=$3",
			[]interface{}{uid, clusterName, string(data)})
		if err == nil {
			queueErr := batch.Queue(batchItem{
				action: "addResource",
				query:  query,
				uid:    uid,
				args:   params,
			})
			if queueErr != nil {
				klog.Warningf("Error queuing resources to add. Error: %+v", queueErr)
				return incomingUIDs, resource, queueErr
			}
			syncResponse.TotalAdded++
		}
		incomingUIDs = append(incomingUIDs, uid)
	}
	// Consume the closing ']' so the outer dispatch loop sees the next top-level key.
	if _, err := dec.Token(); err != nil && err != io.EOF {
		return incomingUIDs, resource, fmt.Errorf("error reading addResources closing token: %v", err)
	}
	return incomingUIDs, resource, nil
}

// hubClusterCleanUpWithRetry takes the known hub cluster name from the latest resync request and deletes all other
// old hub cluster resources and edges, if any. Retries until success to ensure outdated resources are deleted.
func (dao *DAO) hubClusterCleanUpWithRetry(ctx context.Context, requestCluster string) {
	cfg := config.Cfg
	retry := 0

	for {
		if err := dao.checkHubClusterRename(ctx, requestCluster); err != nil {
			waitMS := int(math.Min(float64(retry*500), float64(cfg.MaxBackoffMS)))
			timeToSleep := time.Duration(waitMS) * time.Millisecond
			retry++
			klog.Errorf("Error handling old hub cluster check and cleanup: %s. Will retry in %s\n", err.Error(), timeToSleep)
			time.Sleep(timeToSleep)
		} else {
			klog.V(1).Info("Successfully completed check and handling of old hub clusters.")
			break
		}
	}
}

func (dao *DAO) checkHubClusterRename(ctx context.Context, requestCluster string) error {
	// check if hub cluster name(s) has changed by comparing existing cluster name
	//   where _hubClusterResource is true against new resource cluster where _hubClusterResource is true
	// we check for multiple distinct old hub cluster names in case of the off chance multiple renames have happened
	//   prior to this old name cleaning
	// SELECT distinct cluster FROM search.resources WHERE data ? '_hubClusterResource'
	sql, args, err := goqu.From(goqu.S("search").Table("resources")).
		Select("cluster").
		Distinct().
		Where(goqu.And(
			goqu.L("???", goqu.C("data"), goqu.Literal("?"), "_hubClusterResource"),
			goqu.L("?->>? <> ?", goqu.C("data"), "kind", "Cluster"))).ToSQL()
	if err != nil {
		klog.Errorf("Error creating query to check existing hub cluster name")
		return err
	}
	rows, err := dao.pool.Query(ctx, sql, args...)
	if err != nil {
		klog.Errorf("Error while fetching hub cluster name from database: %s", err.Error())
		return err
	}

	clustersToDelete := make([]string, 0)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var cluster string
			err = rows.Scan(&cluster)
			if err != nil {
				klog.Errorf("Error %s retrieving cluster with query: %s", err.Error(), sql)
				return err
			}
			if cluster != requestCluster && cluster != "" {
				clustersToDelete = append(clustersToDelete, cluster)
			}
		}
	}

	// if old hub cluster name(s) != new hub cluster name, delete all old hub cluster(s) resources and edges
	for _, c := range clustersToDelete {
		if err = dao.deleteOldHubClusterFromDBTable(ctx, c, "resources", requestCluster); err != nil {
			return err
		}
		if err = dao.deleteOldHubClusterFromDBTable(ctx, c, "edges", requestCluster); err != nil {
			return err
		}
	}
	return nil
}

func (dao *DAO) deleteOldHubClusterFromDBTable(ctx context.Context, oldHubCluster string, table string, newHubCluster string) error {
	// DELETE FROM search.? WHERE cluster = ?
	sql, args, err := goqu.From(goqu.S("search").Table(table)).
		Delete().Where(goqu.C("cluster").Eq(oldHubCluster)).ToSQL()
	if err != nil {
		klog.Errorf("Error creating query to delete old hub cluster %s: %s", table, err.Error())
		return err
	}
	res, err := dao.pool.Exec(ctx, sql, args...)
	if err != nil {
		klog.Errorf("Error deleting old hub cluster %s: %s", table, err.Error())
		return err
	}
	klog.Infof("Deleted %d old hub cluster %s from cluster: %s. New hub cluster is %s", res.RowsAffected(), table, oldHubCluster, newHubCluster)

	return nil
}

// addEdges decodes the addEdges array from a decoder that is already positioned
// immediately after the "addEdges" key (i.e. next token is the opening '[').
// Edges already present in existingEdgesMap are removed from the map (so they won't be deleted later);
// new edges are queued for INSERT.
// It reads the closing ']' so the outer dispatch loop can continue with the next key.
func addEdges(dec *json.Decoder, existingEdgesMap *map[string]model.Edge, clusterName string, syncResponse *model.SyncResponse, batch *batchWithRetry) error {
	// Read opening '['.
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("error reading addEdges opening token: %v", err)
	}
	for dec.More() {
		var edge model.Edge
		if err := dec.Decode(&edge); err != nil {
			return fmt.Errorf("error decoding edge from request: %v", err)
		}
		// If the edge already exists, mark it as seen so it isn't deleted later.
		if _, ok := (*existingEdgesMap)[edge.SourceUID+edge.EdgeType+edge.DestUID]; ok {
			delete(*existingEdgesMap, edge.SourceUID+edge.EdgeType+edge.DestUID)
			continue
		}
		// If the edge doesn't exist, add it.
		query, params, err := useGoqu(
			"INSERT into search.edges values($1,$2,$3,$4,$5,$6) ON CONFLICT (sourceid, destid, edgetype) DO NOTHING",
			[]interface{}{edge.SourceUID, edge.SourceKind, edge.DestUID, edge.DestKind, edge.EdgeType, clusterName})
		if err == nil {
			queueErr := batch.Queue(batchItem{
				action: "addEdge",
				query:  query,
				uid:    edge.SourceUID,
				args:   params,
			})
			if queueErr != nil {
				klog.Warningf("Error queuing edges. Error: %+v", queueErr)
				return queueErr
			}
			syncResponse.TotalEdgesAdded++
		}
	}
	// Consume the closing ']' so the outer dispatch loop sees the next top-level key.
	if _, err := dec.Token(); err != nil && err != io.EOF {
		return fmt.Errorf("error reading addEdges closing token: %v", err)
	}
	return nil
}
