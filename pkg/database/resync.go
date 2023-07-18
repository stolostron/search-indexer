// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/stolostron/search-indexer/pkg/metrics"
	"github.com/stolostron/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

func logStepDuration(timer *time.Time, cluster, message string) {
	klog.V(2).Infof("\t> %6s\t - [%10s] %s", time.Since(*timer).Round(time.Millisecond), cluster, message)
	*timer = time.Now()
}

// Reset data for the to the incoming state.
func (dao *DAO) ResyncData(ctx context.Context, event model.SyncEvent,
	clusterName string, syncResponse *model.SyncResponse) error {

	defer metrics.SlowLog(fmt.Sprintf("Slow resync from cluster %s", clusterName), 0)()
	klog.Infof(
		"Starting resync of [%10s]. This is normal, but it could be a problem if it happens often.", clusterName)
	tableName := "search.resources_" + strings.ReplaceAll(clusterName, "-", "_")
	// createSql := fmt.Sprintf("DROP TABLE IF EXISTS %s; CREATE TABLE IF NOT EXISTS %s (uid TEXT, cluster TEXT, data JSONB, PRIMARY KEY(uid, cluster))", tableName, tableName)
	createSql := fmt.Sprintf("DROP TABLE IF EXISTS %s; CREATE TABLE IF NOT EXISTS %s PARTITION OF search.resources FOR VALUES IN ('%s')", tableName, tableName, clusterName)
	_, createerr := dao.pool.Exec(ctx, createSql)
	checkError(createerr, fmt.Sprintf("Error creating partition table tableName %s. Query: %s", tableName, createSql))

	wg := &sync.WaitGroup{}
	wg.Add(2)
	// Reset resources
	go dao.resyncResources(ctx, wg, event.AddResources, clusterName, syncResponse)
	// Reset edges
	go dao.resyncEdges(ctx, wg, event.AddEdges, clusterName, syncResponse)
	wg.Wait()

	// TODO: Need to capture errors from the goroutines above.

	klog.V(1).Infof("Completed resync of cluster %s", clusterName)
	return nil // TODO return queueErr
}

func (dao *DAO) resyncResources(ctx context.Context, wg *sync.WaitGroup, resources []model.Resource, clusterName string, syncResponse *model.SyncResponse) {
	defer wg.Done()
	timer := time.Now()
	//write csv file
	fileResources := [][]interface{}{} // []model.FileResource{}
	klog.Info("Resources to add", len(resources))
	for _, resource := range resources {
		fileResources = append(fileResources, []interface{}{resource.UID, clusterName, resource.Properties}) //[]interface{}{resource.UID, clusterName, resource.Properties})
	}
	// // file, _ := ndjson.Marshal(fileResources)

	// fileName := clusterName + ".csv"
	// // file, _ := json.MarshalIndent(resources, "", " ")

	// // Create a csv file
	// f, err := os.Create(fileName)
	// if err != nil {
	// 	fmt.Println(err)
	// }
	// defer f.Close()
	// // Write Unmarshaled json data to CSV file
	// w := csv.NewWriter(f)
	// for _, obj := range fileResources {
	// 	var record []string
	// 	record = append(record, obj.Uid)
	// 	record = append(record, obj.Cluster)
	// 	b, _ := json.Marshal(obj.Data)
	// 	record = append(record, string(b))
	// 	w.Write(record)
	// }
	// w.Flush()

	// testfileName := "test1.json"

	// _ = ioutil.WriteFile("test.json", file, 0644)

	// klog.Info("*** Wrote file ")

	// fileContent, err := ioutil.ReadFile(fileName)
	// if err != nil {
	// 	klog.Error("Error reading file", err)
	// }

	// // Convert []byte to string
	// text := string(fileContent)
	// textNew := strings.ReplaceAll(text, `""`, `'"`)
	// writeerr := os.WriteFile(fileName, []byte(textNew), 0644)
	// if err != nil {
	// 	klog.Error("Error writing modified file", writeerr)
	// }                                                                                                              xxresources_" + strings.ReplaceAll(clusterName, "-", "_")
	tableName := "resources_" + strings.ReplaceAll(clusterName, "-", "_")
	// rows, copyErr := dao.pool.Query(ctx, fmt.Sprintf(`COPY %s (uid,cluster,data) FROM '%s';`, tableName, fileName))
	// for rows.Next() {
	// 	res, err := rows.Values()
	// 	klog.Info("vals: ", res, err)

	// }
	// klog.Info("copyerr: ", copyErr)
	columnNames := []string{"uid", "cluster", "data"}
	tx, txErr := dao.pool.BeginTx(ctx, pgx.TxOptions{})
	if txErr != nil {
		klog.Error("Error while beginning transaction block for deleting cluster ", clusterName)
	}
	// _, tx1Err := tx.Exec(ctx, fmt.Sprintf("alter table search.%s set unlogged;", tableName))
	// klog.Info("tx1Err: ", tx1Err)

	//This inserting from file did not work or took too much time
	// sql := fmt.Sprintf("INSERT INTO search.%s (uid, cluster,data) values($1,$2,$3)", tableName)
	// stmt, err := tx.Prepare(ctx, "bulkinsert", sql)
	// if err != nil {
	// 	klog.Error("error preparing stmt", err)
	// }

	// //loop through an array of struct filled with data, or read from a file
	// for i, res := range fileResources {
	// 	_, err := tx.Exec(ctx, stmt.SQL, res[0], res[1], res[2])
	// 	if err != nil {
	// 		klog.Error("error insertng record ", i, err)
	// 	}
	// }

	//copy takes >1 min for 15000 resources into a separate table/partition table
	table := pgx.Identifier{"search", tableName}
	klog.Info("Number of rows to insert:", len(fileResources))
	// // logStepDuration(&timer, clusterName, "end copy")

	copyCount, copyerr := tx.CopyFrom(ctx, table, columnNames, pgx.CopyFromRows(fileResources))
	klog.Info("copyerr: ", copyerr)
	// logStepDuration(&timer, clusterName, "copy over")

	// _, tx2Err := tx.Exec(ctx, fmt.Sprintf("alter table search.%s set logged;", tableName))
	// klog.Info("tx1Err: ", tx2Err)

	commiterr := tx.Commit(ctx)
	klog.Info("commiterr: ", commiterr)
	klog.Info("Number of rows copied:", copyCount)

	// logStepDuration(&timer, clusterName, "commit over")

	// batch := NewBatchWithRetry(ctx, dao, syncResponse)

	// Get existing resources for the cluster.
	// existingResourcesMap := make(map[string]struct{})
	// existingRows, err := dao.pool.Query(ctx, fmt.Sprintf("SELECT uid FROM search.%s WHERE cluster=$1", tableName), clusterName)
	// if err != nil {
	// 	klog.Warningf("Error getting existing resources uids of cluster %s. Error: %+v", clusterName, err)
	// }
	// defer existingRows.Close()
	// for existingRows.Next() {
	// 	id := ""
	// 	err := existingRows.Scan(&id)
	// 	if err != nil {
	// 		klog.Warningf("Error scanning existing resource row. Error: %+v", err)
	// 		continue
	// 	}
	// 	existingResourcesMap[id] = struct{}{}
	// }
	// logStepDuration(&timer, clusterName, "QUERY existing resources")

	// INSERT or UPDATE resources.
	// In case of conflict update only if data has changed.
	// for _, resource := range resources {
	// 	delete(existingResourcesMap, resource.UID)
	// 	data, _ := json.Marshal(resource.Properties)
	// 	// TODO: Use goqu to build the query.
	// 	// TODO: Combine multiple inserts into a single query.
	// 	queueErr := batch.Queue(batchItem{
	// 		action: "addResource",
	// 		query: fmt.Sprintf(`INSERT into %s as r values($1,$2,$3)
	// 		ON CONFLICT (uid,cluster)
	// 		DO UPDATE SET data=$3 WHERE r.uid=$1 and r.data IS DISTINCT FROM $3`, tableName),
	// 		uid:  resource.UID,
	// 		args: []interface{}{resource.UID, clusterName, string(data)},
	// 	})
	// 	if queueErr != nil {
	// 		klog.Warningf("Error queuing resources. Error: %+v", queueErr)
	// 		return // TODO: return queueErr
	// 	}
	// }
	// batch.flush()

	// // DELETE any previous resources for the cluster that isn't included in the incoming resync event.

	// if len(existingResourcesMap) > 0 {
	// 	// TODO: Use goqu to build the query.
	// 	resourcesToDelete := make([]string, 0)
	// 	for resourceUID := range existingResourcesMap {
	// 		resourcesToDelete = append(resourcesToDelete, "'"+resourceUID+"'")
	// 	}

	// 	queryStr := fmt.Sprintf("DELETE from %s WHERE uid IN (%s)", tableName, strings.Join(resourcesToDelete, ","))

	// 	deletedRows, err := dao.pool.Exec(ctx, queryStr) // TODO: Use batch.Queue() instead of Exec()
	// 	if err != nil {
	// 		klog.Warningf("Error deleting resources during resync of cluster %s. Error: %+v", clusterName, err)
	// 	}
	// 	klog.Infof("Deleted %d resources during resync of cluster %s", deletedRows.RowsAffected(), clusterName)
	// }
	// batch.wg.Wait()
	logStepDuration(&timer, clusterName,
		fmt.Sprintf("Resync INSERT/UPDATE [%d] DELETE [%d] resources", len(resources), 0))
}

// Reset Edges
func (dao *DAO) resyncEdges(ctx context.Context, wg *sync.WaitGroup,
	edges []model.Edge, clusterName string, syncResponse *model.SyncResponse) {
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
		// TODO: Use goqu to build the query.
		// TODO: Combine multiple inserts into a single query.
		queueErr = batch.Queue(batchItem{
			action: "addEdge",
			query:  "INSERT into search.edges values($1,$2,$3,$4,$5,$6) ON CONFLICT (sourceid, destid, edgetype) DO NOTHING",
			uid:    edge.SourceUID,
			args:   []interface{}{edge.SourceUID, edge.SourceKind, edge.DestUID, edge.DestKind, edge.EdgeType, clusterName}})

		if queueErr != nil {
			klog.Warningf("Error queuing edges. Error: %+v", queueErr)
			return // TODO: return queueErr
		}
	}

	// Delete existing edges that are not in the new sync event.
	for _, edge := range existingEdgesMap {
		// If the edge already exists, do nothing.
		// TODO: Use goqu to build the query.
		// TODO: Combine multiple deletes into a single query.
		queueErr = batch.Queue(batchItem{
			action: "deleteEdge",
			query:  "DELETE from search.edges WHERE sourceid=$1 AND destid=$2 AND edgetype=$3",
			uid:    edge.SourceUID,
			args:   []interface{}{edge.SourceUID, edge.DestUID, edge.EdgeType},
		})
		if queueErr != nil {
			klog.Warningf("Error queuing edges. Error: %+v", queueErr)
			return // TODO: return queueErr
		}
	}

	batch.flush()
	batch.wg.Wait()
	logStepDuration(&timer, clusterName, fmt.Sprintf("Resync INSERT [%d] DELETE [%d] edges", len(edges), len(existingEdgesMap)))
}
