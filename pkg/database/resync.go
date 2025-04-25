// Copyright Contributors to the Open Cluster Management project

package database

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/stolostron/search-indexer/pkg/metrics"
	"github.com/stolostron/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

// Reset data for the cluster to the incoming state.
func (dao *DAO) ResyncData(ctx context.Context, clusterName string, syncResponse *model.SyncResponse, requestBody []byte) error {

	defer metrics.SlowLog(fmt.Sprintf("Slow resync from %12s.", clusterName), 0)()
	klog.Infof(
		"Starting resync from %12s. This is normal, but it could be a problem if it happens often.", clusterName)

	// Reset resources
	err := dao.resetResources(ctx, clusterName, syncResponse, requestBody)
	if err != nil {
		klog.Warningf("Error resyncing resources for cluster %12s. Error: %+v", clusterName, err)
		return err
	}

	// Reset edges
	err = dao.resetEdges(ctx, clusterName, syncResponse, requestBody)
	if err != nil {
		klog.Warningf("Error resyncing edges for cluster %12s. Error: %+v", clusterName, err)
		return err
	}

	klog.V(1).Infof("Completed resync of cluster %12s.", clusterName)
	return nil
}

// Reset Resources.
// 1. Upsert each incoming resource. Keep the UID.
// 2. Delete existing UIDs that don't match the incoming UIDs.
func (dao *DAO) resetResources(ctx context.Context, clusterName string,
	syncResponse *model.SyncResponse, resyncBody []byte) error {

	batch := NewBatchWithRetry(ctx, dao, syncResponse)

	// UPSERT resources in the database.
	incomingUIDs, err := addResources(resyncBody, clusterName, syncResponse, batch)
	if err != nil {
		return err
	}
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

// Reset Edges
//  1. Get existing edges for the cluster. Excluding intercluster edges.
//  2. For each incoming edge, INSERT if it doesn't exist.
//  3. Delete any existing edges that aren't in the incoming sync event.
func (dao *DAO) resetEdges(ctx context.Context, clusterName string,
	syncResponse *model.SyncResponse, resyncRequest []byte) error {
	timer := time.Now()

	batch := NewBatchWithRetry(ctx, dao, syncResponse)

	var queueErr error
	existingEdgesMap := make(map[string]model.Edge)

	// Get all existing edges for the cluster.
	query, params, err := useGoqu(
		"SELECT sourceid, edgetype, destid FROM search.edges WHERE edgetype!='interCluster' AND cluster=$1",
		[]interface{}{clusterName})
	if err == nil {
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

	// Now insert edges from the reqeust that don't already exist
	if err = addEdges(resyncRequest, &existingEdgesMap, clusterName, syncResponse, batch); err != nil {
		return err
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
			syncResponse.TotalEdgesDeleted++
		}
	}

	batch.flush()
	batch.wg.Wait()
	metrics.LogStepDuration(&timer, clusterName, fmt.Sprintf("Reset edges stats: INSERT [%d] DELETE [%d]",
		syncResponse.TotalEdgesAdded, syncResponse.TotalEdgesDeleted))
	return batch.connError
}

func addResources(resyncBody []byte, clusterName string, syncResponse *model.SyncResponse, batch batchWithRetry) ([]interface{}, error) {
	dec := json.NewDecoder(bytes.NewReader(resyncBody))
	incomingUIDs := make([]interface{}, 0)
	for {
		// read tokens until we get to addResources
		field, err := dec.Token()
		if err == io.EOF {
			break
		}
		if field == "addResources" {
			// read opening [
			if _, err = dec.Token(); err != nil {
				return incomingUIDs, fmt.Errorf("error reading addResources opening token: %v", err)
			}
			for dec.More() {
				var resource model.Resource
				if err = dec.Decode(&resource); err != nil {
					return incomingUIDs, fmt.Errorf("error decoding resource from request: %v", err)
				}
				uid := resource.UID
				data, _ := json.Marshal(resource.Properties)
				query, params, err := useGoqu(
					"INSERT into search.resources values($1,$2,$3) ON CONFLICT (uid) DO UPDATE SET data=$3 WHERE data!=$3",
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
						return incomingUIDs, queueErr
					}
					syncResponse.TotalAdded++
				}
				incomingUIDs = append(incomingUIDs, uid)
			}
			return incomingUIDs, nil
		}
	}
	return incomingUIDs, nil
}

func addEdges(requestBody []byte, existingEdgesMap *map[string]model.Edge, clusterName string, syncResponse *model.SyncResponse, batch batchWithRetry) error {
	dec := json.NewDecoder(bytes.NewReader(requestBody))

	for {
		// read tokens until we get to addEdges
		field, err := dec.Token()
		if err == io.EOF {
			break
		}
		if field == "addEdges" {
			// read opening [
			if _, err := dec.Token(); err != nil {
				return fmt.Errorf("error reading addEdges opening token: %v", err)
			}
			for dec.More() {
				var edge model.Edge
				if err = dec.Decode(&edge); err != nil {
					return fmt.Errorf("error decoding edge from request: %v", err)
				}
				// If the edge already exists, do nothing.
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
		}
	}

	return nil
}
