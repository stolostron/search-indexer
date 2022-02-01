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

type batchItem struct {
	query    string
	args     []interface{}
	uid      string // Used to report errors.
	syncType string // Used to report errors.
}

func (dao *DAO) SyncData(event model.SyncEvent, clusterName string, syncResponse *model.SyncResponse) {
	wg := &sync.WaitGroup{}
	batchItems := make([]batchItem, 0)

	addToBatchQueue := func(syncType string, query string, args ...interface{}) {
		batchItems = append(batchItems, batchItem{query: query, args: args, uid: args[0].(string), syncType: syncType})

		if len(batchItems) >= dao.batchSize {
			wg.Add(1)
			go dao.sendBatch(batchItems, wg, syncResponse) // ??? How do I check for error here?
			batchItems = make([]batchItem, 0)
		}
	}

	// ADD RESOURCES
	for _, resource := range event.AddResources {
		////// Inserting error
		// if i == 50 {
		// 	addToBatchQueue("addResource", "INSERT into search.r values($1,$2,$3)", "id111", 2, 3)
		// 	addToBatchQueue("updateResource", "INSERT into search.r values($1,$2,$3)", "id222", 2, 3)
		// 	//  addToBatchQueue("INSERT into search.resources values('a','b','{}')")
		// 	//  addToBatchQueue("INSERT into search.resources values(1,2,3)")
		// }
		data, _ := json.Marshal(resource.Properties)
		addToBatchQueue("addResource", "INSERT into search.resources values($1,$2,$3)", resource.UID, clusterName, string(data))
	}
	// UPDATE RESOURCES
	// The collector enforces that a resource isn't added and updated in the same sync event.
	// The uid and cluster fields will never get updated for a resource.
	for _, resource := range event.UpdateResources {
		json, _ := json.Marshal(resource.Properties)
		addToBatchQueue("updateResource", "UPDATE search.resources SET data=$2 WHERE uid=$1", resource.UID, string(json))
	}

	// DELETE RESOURCES and all edges pointing to the resource.
	if len(event.DeleteResources) > 0 {
		uids := make([]string, len(event.DeleteResources))
		for i, resource := range event.DeleteResources {
			uids[i] = resource.UID
		}

		// TODO: Need better safety for delete errors.
		// The current retry logic won't work well if there's an error here.
		addToBatchQueue("deleteResource", "DELETE from search.resources WHERE uid IN ($1)", strings.Join(uids, ", "))
		addToBatchQueue("deleteResource", "DELETE from search.edges WHERE sourceId IN ($1)", strings.Join(uids, ", "))
		addToBatchQueue("deleteResource", "DELETE from search.edges WHERE destId IN ($1)", strings.Join(uids, ", "))
	}

	// ADD EDGES
	for _, edge := range event.AddEdges {
		addToBatchQueue("addEdge", "INSERT into search.edges values($1,$2,$3,$4,$5,$6)",
			edge.SourceUID, edge.SourceKind, edge.DestUID, edge.DestKind, edge.EdgeType, clusterName)
	}

	// UPDATE EDGES
	// Edges are never updated. The collector only sends ADD and DELETE eveents for edges.

	// DELETE EDGES
	for _, edge := range event.DeleteEdges {
		addToBatchQueue("deleteEdge", "DELETE from search.edges WHERE sourceId=$1 AND destId=$2 AND edgeType=$3",
			edge.SourceUID, edge.DestUID, edge.EdgeType)
	}

	// Flush remaining items in the batch.
	if len(batchItems) > 0 {
		wg.Add(1)
		go dao.sendBatch(batchItems, wg, syncResponse)
	}

	// Wait for all batches to complete.
	wg.Wait()
}

func (dao *DAO) sendBatch(batchItems []batchItem, wg *sync.WaitGroup, syncResponse *model.SyncResponse) error {
	defer wg.Done()

	batch := &pgx.Batch{}
	for _, item := range batchItems {
		batch.Queue(item.query, item.args...)
	}
	br := dao.pool.SendBatch(context.Background(), batch)
	_, err := br.Exec()
	br.Close()

	// Process errors.
	// pgx.Batch is processed as a transaction, so in case of an error, the entire batch will fail.
	if err != nil {
		if len(batchItems) == 1 {
			item := batchItems[0]
			klog.Errorf("ERROR processing batchItem.  %+v", batchItems[0])

			var errorArray *[]model.SyncError
			switch item.syncType {
			case "addResource":
				errorArray = &syncResponse.AddErrors
			case "updateResource":
				errorArray = &syncResponse.UpdateErrors
			case "deleteResource":
				errorArray = &syncResponse.DeleteErrors
			case "addEdge":
				errorArray = &syncResponse.AddEdgeErrors
			case "deleteEdge":
				errorArray = &syncResponse.DeleteEdgeErrors
			default:
				klog.Error("Unable to process sync error with type: ", item.syncType)
			}
			*errorArray = append(*errorArray, model.SyncError{ResourceUID: batchItems[0].uid, Message: "Error details"})

			return nil // Don't return error here to stop the recursion.
		}

		klog.Error("Error sending batch, resending smaller batch.")

		// Retry first half.
		firstHalf := batchItems[:len(batchItems)/2]
		wg.Add(1)
		err1 := dao.sendBatch(firstHalf, wg, syncResponse)

		// Retry second half
		secondHalf := batchItems[len(batchItems)/2:]
		wg.Add(1)
		err2 := dao.sendBatch(secondHalf, wg, syncResponse)

		// Returns error only if we fail processing either retry.
		if err1 != nil && err2 != nil {
			return nil
		}
	}
	return err
}

// TODO: move this to different file.
func (dao *DAO) ClusterTotals(clusterName string) (resources int, edges int) {
	batch := &pgx.Batch{}
	batch.Queue("SELECT count(*) FROM search.resources WHERE cluster=$1", clusterName)
	batch.Queue("SELECT count(*) FROM search.edges WHERE cluster=$1", clusterName)

	br := dao.pool.SendBatch(context.Background(), batch)
	defer br.Close()

	resourcesRow := br.QueryRow()
	resourcesErr := resourcesRow.Scan(&resources)
	if resourcesErr != nil {
		klog.Error("Error reading total resources for cluster ", clusterName)
	}
	edgesRow := br.QueryRow()
	edgesErr := edgesRow.Scan(&edges)
	if edgesErr != nil {
		klog.Error("Error reading total edges for cluster ", clusterName)
	}

	return resources, edges
}
