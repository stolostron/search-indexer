// Copyright Contributors to the Open Cluster Management project

package database

import (
	"encoding/json"
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
	ExistingClustersCache = make(map[string]interface{})

	// clusterResource := `{"UID":"cluster__name-foo", "Kind":"Cluster", "Properties":{"_clusterNamespace":"name-foo", "apigroup":"internal.open-cluster-management.io", "consoleURL":"", "cpu":0, "created":"0001-01-01T00:00:00Z", "kind":"Cluster", "kubernetesVersion":"", "memory":0, "name":"name-foo", "nodes":0}}`

	// _ = json.Unmarshal([]byte(clusterResource), &existingCluster)
	UpdateClustersCache("cluster__name-foo", existingCluster["Properties"])

	currCluster := model.Resource{Kind: existingCluster["Kind"].(string), UID: existingCluster["UID"].(string),
		Properties: existingCluster["Properties"].(map[string]interface{})}

	// Prepare a mock DAO instance
	dao, _ := buildMockDAO(t)

	dao.UpsertCluster(currCluster)
	AssertEqual(t, len(ExistingClustersCache), 1, "ExistingClustersMap should have length of 1")
	_, ok := ExistingClustersCache["cluster__name-foo"]
	AssertEqual(t, ok, true, "ExistingClustersMap should have an entry for cluster foo")
}

func Test_UpsertCluster_Update1(t *testing.T) {
	initializeVars()
	tmpClusterProps, _ := existingCluster["Properties"].(map[string]interface{})
	tmpClusterProps["cpu"] = 9
	delete(tmpClusterProps, "nodes")
	existingCluster["Properties"] = tmpClusterProps
	ExistingClustersCache = make(map[string]interface{})
	// clusterResource := `{"UID":"cluster__name-foo", "Kind":"Cluster", "Properties":{"_clusterNamespace":"name-foo", "apigroup":"internal.open-cluster-management.io", "consoleURL":"", "created":"0001-01-01T00:00:00Z", "kind":"Cluster", "kubernetesVersion":"", "memory":0, "cpu":9, "name":"name-foo"}}`

	// var existingCluster map[string]interface{}
	// _ = json.Unmarshal([]byte(clusterResource), &existingCluster)
	ExistingClustersCache["cluster__name-foo"] = existingCluster["Properties"]

	props := make(map[string]interface{})
	for key, val := range existingCluster["Properties"].(map[string]interface{}) {
		props[key] = val
	}
	tmpProps := props
	tmpProps["cpu"] = int64(10)

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
	AssertEqual(t, len(ExistingClustersCache), 1, "ExistingClustersMap should have length of 1")
	_, clusterPresent := ReadClustersCache("cluster__name-foo")
	AssertEqual(t, clusterPresent, true, "ExistingClustersMap should have an entry for cluster foo")
}

func Test_UpsertCluster_Update2(t *testing.T) {
	ExistingClustersCache = make(map[string]interface{})
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
	currProps := props
	currProps["cpu"] = int64(10)

	currCluster := model.Resource{Kind: existingCluster["Kind"].(string), UID: existingCluster["UID"].(string), Properties: currProps}

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
	AssertEqual(t, len(ExistingClustersCache), 1, "ExistingClustersMap should have length of 1")
	_, ok := ExistingClustersCache["cluster__name-foo"]
	AssertEqual(t, ok, true, "ExistingClustersMap should have an entry for cluster foo")
}

//Should insert cluster
func Test_UpsertCluster_Insert(t *testing.T) {
	ExistingClustersCache = make(map[string]interface{})
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

	currCluster := model.Resource{Kind: existingCluster["Kind"].(string), UID: existingCluster["UID"].(string), Properties: tmpProps}

	// Prepare a mock DAO instance
	dao, mockPool := buildMockDAO(t)

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
	AssertEqual(t, len(ExistingClustersCache), 1, "ExistingClustersMap should have length of 1")
	_, ok := ExistingClustersCache["cluster__name-foo"]
	AssertEqual(t, ok, true, "ExistingClustersMap should have an entry for cluster foo")
}
