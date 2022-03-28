// Copyright Contributors to the Open Cluster Management project

package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stolostron/search-indexer/pkg/model"
)

var clusterProps map[string]interface{}
var existingCluster map[string]interface{}

func initializeVars() {
	clusterProps = map[string]interface{}{
		"apigroup":          "internal.open-cluster-management.io",
		"consoleURL":        "",
		"cpu":               0,
		"created":           "0001-01-01T00:00:00Z",
		"kind":              "Cluster",
		"kubernetesVersion": "",
		"memory":            0,
		"name":              "name-foo",
		"nodes":             0}
	existingCluster = map[string]interface{}{"UID": "cluster__name-foo",
		"Kind":       "Cluster",
		"Properties": clusterProps}
}
func Test_UpsertCluster_NoUpdate(t *testing.T) {
	initializeVars()
	UpdateClustersCache("cluster__name-foo", existingCluster["Properties"])

	currCluster := model.Resource{Kind: existingCluster["Kind"].(string), UID: existingCluster["UID"].(string),
		Properties: existingCluster["Properties"].(map[string]interface{})}

	// Prepare a mock DAO instance
	dao, _ := buildMockDAO(t)

	dao.UpsertCluster(currCluster)
	AssertEqual(t, len(existingClustersCache), 1, "existingClustersCache should have length of 1")
	_, ok := ReadClustersCache("cluster__name-foo")
	AssertEqual(t, ok, true, "existingClustersCache should have an entry for cluster foo")
}

//Test when number of properties match but values are updated
// Values in clusters cache should get updated
func Test_UpsertCluster_Update1(t *testing.T) {
	initializeVars()
	tmpClusterProps, _ := existingCluster["Properties"].(map[string]interface{})
	tmpClusterProps["cpu"] = 9
	delete(tmpClusterProps, "nodes")
	existingCluster["Properties"] = tmpClusterProps

	existingClustersCache = make(map[string]interface{})
	props := make(map[string]interface{})
	for key, val := range existingCluster["Properties"].(map[string]interface{}) {
		props[key] = val
	}
	tmpProps := props
	tmpProps["cpu"] = int(10)

	currCluster := model.Resource{Kind: existingCluster["Kind"].(string), UID: existingCluster["UID"].(string), Properties: tmpProps}
	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)
	mrows := newMockRows()
	mockPool.EXPECT().Query(gomock.Any(),
		gomock.Eq(`SELECT uid, data from search.resources where uid=$1`),
		gomock.Eq([]interface{}{"cluster__name-foo"}),
	).Return(mrows, nil)
	expectedProps, _ := json.Marshal(currCluster.Properties)
	mockPool.EXPECT().Exec(gomock.Any(),
		gomock.Eq(`INSERT INTO search.resources as r (uid, cluster, data) values($1,'',$2) ON CONFLICT (uid) DO UPDATE SET data=$2 WHERE r.uid=$1`),
		gomock.Eq([]interface{}{"cluster__name-foo", string(expectedProps)}),
	).Return(nil, nil)

	dao.UpsertCluster(currCluster)
	AssertEqual(t, len(existingClustersCache), 1, "existingClustersCache should have length of 1")
	currProps, clusterPresent := ReadClustersCache("cluster__name-foo")
	AssertEqual(t, clusterPresent, true, "existingClustersCache should have an entry for cluster foo")
	currPropsMap, _ := currProps.(map[string]interface{})
	AssertEqual(t, currPropsMap["cpu"], 10, fmt.Sprintf("existingClustersCache should have updated the entry for cluster foo cpu. Expected: %d. Got:%d", 10, currPropsMap["cpu"]))

}

// Test when number of existing and current properties won't match.
// So, properties will need to be updated in cache and db.
func Test_UpsertCluster_Update2(t *testing.T) {
	initializeVars()
	tmpClusterProps, _ := existingCluster["Properties"].(map[string]interface{})
	delete(tmpClusterProps, "cpu")
	delete(tmpClusterProps, "memory")
	delete(tmpClusterProps, "nodes")
	existingCluster["Properties"] = tmpClusterProps

	props := make(map[string]interface{})
	for key, val := range existingCluster["Properties"].(map[string]interface{}) {
		props[key] = val
	}
	tmpCurrProps := props
	tmpCurrProps["cpu"] = int(10)

	currCluster := model.Resource{Kind: existingCluster["Kind"].(string),
		UID:        existingCluster["UID"].(string),
		Properties: tmpCurrProps}

	//Clear cluster cache
	existingClustersCache = make(map[string]interface{})
	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)
	mrows := newMockRows()
	mockPool.EXPECT().Query(gomock.Any(),
		gomock.Eq(`SELECT uid, data from search.resources where uid=$1`),
		gomock.Eq([]interface{}{"cluster__name-foo"}),
	).Return(mrows, nil)
	expectedProps, _ := json.Marshal(currCluster.Properties)

	mockPool.EXPECT().Exec(gomock.Any(),
		gomock.Eq(`INSERT INTO search.resources as r (uid, cluster, data) values($1,'',$2) ON CONFLICT (uid) DO UPDATE SET data=$2 WHERE r.uid=$1`),
		gomock.Eq([]interface{}{"cluster__name-foo", string(expectedProps)}),
	).Return(nil, nil)
	dao.UpsertCluster(currCluster)
	AssertEqual(t, len(existingClustersCache), 1, "existingClustersCache should have length of 1")
	currProps, clusterPresent := ReadClustersCache("cluster__name-foo")
	AssertEqual(t, clusterPresent, true, "existingClustersCache should have an entry for cluster foo")
	currPropsMap, _ := currProps.(map[string]interface{})
	AssertEqual(t, currPropsMap["cpu"], 10, fmt.Sprintf("existingClustersCache should have updated entry for cluster foo cpu. Expected: %d. Got:%d", 10, currPropsMap["cpu"]))
	AssertEqual(t, currPropsMap["nodes"], nil, fmt.Sprintf("existingClustersCache should not have an entry for cluster foo nodes. Expected: nil. Got:%d", currPropsMap["nodes"]))

}

//Should insert cluster
func Test_UpsertCluster_Insert(t *testing.T) {
	initializeVars()
	tmpClusterProps, _ := existingCluster["Properties"].(map[string]interface{})
	delete(tmpClusterProps, "memory")
	delete(tmpClusterProps, "nodes")
	tmpClusterProps["cpu"] = int64(10)

	existingCluster["Properties"] = tmpClusterProps

	props := make(map[string]interface{})
	for key, val := range existingCluster["Properties"].(map[string]interface{}) {
		props[key] = val
	}
	tmpProps := props
	tmpProps["cpu"] = int64(10)

	currCluster := model.Resource{Kind: existingCluster["Kind"].(string),
		UID: existingCluster["UID"].(string), Properties: tmpProps}

	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)
	//Clear cluster cache
	existingClustersCache = make(map[string]interface{})
	mockPool.EXPECT().Query(gomock.Any(),
		gomock.Eq(`SELECT uid, data from search.resources where uid=$1`),
		gomock.Eq([]interface{}{"cluster__name-foo"}),
	).Return(nil, nil)
	expectedProps, _ := json.Marshal(currCluster.Properties)
	mockPool.EXPECT().Exec(gomock.Any(),
		gomock.Eq(`INSERT INTO search.resources as r (uid, cluster, data) values($1,'',$2) ON CONFLICT (uid) DO UPDATE SET data=$2 WHERE r.uid=$1`),
		gomock.Eq([]interface{}{"cluster__name-foo", string(expectedProps)}),
	).Return(nil, nil)

	dao.UpsertCluster(currCluster)
	AssertEqual(t, len(existingClustersCache), 1, "existingClustersCache should have length of 1")
	_, ok := ReadClustersCache("cluster__name-foo")
	AssertEqual(t, ok, true, "existingClustersCache should have an entry for cluster foo")
}

// foo1 cluster should not be in cache
func Test_clusterPropsUpToDate_notInCache(t *testing.T) {
	// Prepare a mock DAO instance
	dao, _ := buildMockDAO(t)
	ok := dao.clusterPropsUpToDate("cluster__name-foo1", model.Resource{})
	AssertEqual(t, ok, false, "existingClustersCache should not have an entry for cluster foo1")

}

//select query error condition
func Test_clusterInDB_QueryErr(t *testing.T) {

	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)
	mockPool.EXPECT().Query(gomock.Any(),
		gomock.Eq(`SELECT uid, data from search.resources where uid=$1`),
		gomock.Eq([]interface{}{"cluster__name-foo1"}),
	).Return(nil, errors.New("Error fetching data"))
	ok := dao.clusterInDB("cluster__name-foo1")
	AssertEqual(t, ok, false, "existingClustersCache should not have an entry for cluster foo1")

}

//The previous test will create an entry for cluster foo in existingClustersCache.
// This should get removed after the delete function comepletes
func Test_DelCluster(t *testing.T) {
	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)
	_, ok := ReadClustersCache("cluster__name-foo")
	AssertEqual(t, ok, true, "existingClustersCache should have an entry for cluster foo")
	mockPool.EXPECT().Exec(gomock.Any(),
		gomock.Eq(`DELETE FROM search.resources WHERE cluster=$1`),
		gomock.Eq([]interface{}{"name-foo"}),
	).Return(nil, nil)

	mockPool.EXPECT().Exec(gomock.Any(),
		gomock.Eq(`DELETE FROM search.edges WHERE cluster=$1`),
		gomock.Eq([]interface{}{"name-foo"}),
	).Return(nil, nil)

	mockPool.EXPECT().Exec(gomock.Any(),
		gomock.Eq(`DELETE FROM search.resources WHERE uid=$1`),
		gomock.Eq([]interface{}{"cluster__name-foo"}),
	).Return(nil, nil)

	dao.DeleteCluster("name-foo")
	_, ok = ReadClustersCache("cluster__name-foo")
	AssertEqual(t, ok, false, "existingClustersCache should not have an entry for cluster foo")
}
