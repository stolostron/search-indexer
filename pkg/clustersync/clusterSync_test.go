// Copyright Contributors to the Open Cluster Management project
package clustersync

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/driftprogramming/pgxpoolmock"
	"github.com/golang/mock/gomock"
	"github.com/jackc/pgx/v4"
	"github.com/pashagolub/pgxmock"
	"github.com/stolostron/search-indexer/pkg/database"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/klog/v2"
)

// Create a GroupVersionResource
const managedclusterinfogroupAPIVersion = "internal.open-cluster-management.io/v1beta1"
const managedclustergroupAPIVersion = "cluster.open-cluster-management.io/v1"
const managedclusteraddongroupAPIVersion = "addon.open-cluster-management.io/v1alpha1"

var managedClusterGvr *schema.GroupVersionResource
var managedClusterInfoGvr *schema.GroupVersionResource
var existingCluster map[string]interface{}

func fakeDynamicClient() *fake.FakeDynamicClient {
	managedClusterGvr, _ = schema.ParseResourceArg(managedClusterGVR)
	managedClusterInfoGvr, _ = schema.ParseResourceArg(managedClusterInfoGVR)
	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(managedClusterGvr.GroupVersion())
	scheme.AddKnownTypes(managedClusterInfoGvr.GroupVersion())

	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "cluster.open-cluster-management.io", Version: "v1", Kind: "ManagedCluster"},
		&unstructured.UnstructuredList{})

	dyn := fake.NewSimpleDynamicClient(scheme, newTestUnstructured(managedclusterinfogroupAPIVersion, "ManagedClusterInfo", "name-foo", "name-foo", ""),
		newTestUnstructured(managedclustergroupAPIVersion, "ManagedCluster", "", "name-foo", ""),
		newTestUnstructured(managedclustergroupAPIVersion, "ManagedCluster", "", "name-foo-error", ""))
	_, err := dyn.Resource(*managedClusterGvr).Get(context.TODO(), "name-foo", v1.GetOptions{})
	if err != nil {
		klog.Warning("Error creating fake NewSimpleDynamicClient: ", err.Error())
	}
	return dyn
}

func newTestUnstructured(apiVersion, kind, namespace, name, uid string) *unstructured.Unstructured {
	labels := make(map[string]interface{})
	labels["env"] = "dev"
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
				"uid":       uid,
				"labels":    labels,
			},
		},
	}
}
func initializeVars() {
	labelMap := map[string]string{"env": "dev"}
	clusterProps := map[string]interface{}{
		"label":             labelMap,
		"apigroup":          "internal.open-cluster-management.io",
		"cpu":               0,
		"created":           "0001-01-01T00:00:00Z",
		"kind":              "Cluster",
		"kubernetesVersion": "",
		"memory":            "0",
		"name":              "name-foo",
	}
	existingCluster = map[string]interface{}{"UID": "cluster__name-foo",
		"Kind":       "Cluster",
		"Properties": clusterProps}
}

// Verify that ProcessClusterUpsert works.
func Test_ProcessClusterUpsert_ManagedCluster(t *testing.T) {
	initializeVars()
	obj := newTestUnstructured(managedclustergroupAPIVersion, "ManagedCluster", "", "name-foo", "test-mc-uid")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPool := pgxpoolmock.NewMockPgxPool(ctrl)
	// Prepare a mock DAO instance
	dao = database.NewDAO(mockPool)
	dynamicClient = fakeDynamicClient()
	expectedProps, _ := json.Marshal(existingCluster["Properties"])

	mockPool.EXPECT().Query(gomock.Any(),
		gomock.Eq(`SELECT uid, data from search.resources where uid=$1`),
		gomock.Eq([]interface{}{"cluster__name-foo"}),
	).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(),
		gomock.Eq(`INSERT INTO search.resources as r (uid, cluster, data) values($1,'',$2) ON CONFLICT (uid) DO UPDATE SET data=$2 WHERE r.uid=$1`),
		gomock.Eq([]interface{}{"cluster__name-foo", string(expectedProps)}),
	).Return(nil, nil)

	processClusterUpsert(context.TODO(), obj)
	//Once processClusterUpsert is done, existingClustersCache should have an entry for cluster foo
	_, ok := database.ReadClustersCache("cluster__name-foo")
	AssertEqual(t, ok, true, "existingClustersCache should have an entry for cluster foo")

}

func Test_ProcessClusterUpsert_ManagedClusterInfo(t *testing.T) {
	initializeVars()
	//Ensure there is an entry for cluster_foo in the cluster cache
	database.UpdateClustersCache("cluster__name-foo", existingCluster["Properties"])
	obj := newTestUnstructured(managedclusterinfogroupAPIVersion, "ManagedClusterInfo", "name-foo", "name-foo", "test-mc-uid")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPool := pgxpoolmock.NewMockPgxPool(ctrl)
	// Prepare a mock DAO instance
	dao = database.NewDAO(mockPool)
	dynamicClient = fakeDynamicClient()
	//Add props specific to ManagedClusterInfo
	props := existingCluster["Properties"].(map[string]interface{})
	props["consoleURL"] = ""
	props["nodes"] = 0
	existingCluster["Properties"] = props
	expectedProps, _ := json.Marshal(existingCluster["Properties"])

	mockPool.EXPECT().Exec(gomock.Any(),
		gomock.Eq(`INSERT INTO search.resources as r (uid, cluster, data) values($1,'',$2) ON CONFLICT (uid) DO UPDATE SET data=$2 WHERE r.uid=$1`),
		gomock.Eq([]interface{}{"cluster__name-foo", string(expectedProps)}),
	).Return(nil, nil)

	processClusterUpsert(context.TODO(), obj)
	//Once processClusterUpsert is done, existingClustersCache should have an entry for cluster foo
	_, ok := database.ReadClustersCache("cluster__name-foo")
	AssertEqual(t, ok, true, "existingClustersCache should have an entry for cluster foo")

}

// AssertEqual checks if values are equal
func AssertEqual(t *testing.T, a interface{}, b interface{}, message string) {
	if a == b {
		return
	}
	t.Errorf("%s Received %v (type %v), expected %v (type %v)", message, a, reflect.TypeOf(a), b, reflect.TypeOf(b))
}

func Test_isClusterCrdMissingNoError(t *testing.T) {
	ok := isClusterCrdMissing(nil)
	AssertEqual(t, ok, false, "No error found in clusterCRDMissing")
}

func Test_clusterCrdMissingWithMissingError(t *testing.T) {
	err := errors.New("could not find the requested resource: ClusterCRD")
	ok := isClusterCrdMissing(err)
	AssertEqual(t, ok, true, "Error found: clusterCRD is missing")
}
func Test_clusterCrdMissingWithNotMissingError(t *testing.T) {
	err := errors.New("some other error")
	ok := isClusterCrdMissing(err)
	AssertEqual(t, ok, false, "Error found: clusterCRD is missing")
}
func Test_ProcessClusterNoDeleteOnMCInfo(t *testing.T) {
	initializeVars()
	obj := newTestUnstructured(managedclusterinfogroupAPIVersion, "ManagedClusterInfo", "", "name-foo", "test-mc-uid")
	//Ensure there is an entry for cluster_foo in the cluster cache
	database.UpdateClustersCache("cluster__name-foo", nil)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	processClusterDelete(context.TODO(), obj)

	//Once processClusterDelete is done, existingClustersCache should still have an entry for cluster foo as resources
	// are not deleted on deletion of ManadClusterInfo
	_, ok := database.ReadClustersCache("cluster__name-foo")
	AssertEqual(t, ok, true, "existingClustersCache should not have an entry for cluster foo")

}
func Test_ProcessClusterDeleteOnMC(t *testing.T) {
	initializeVars()
	obj := newTestUnstructured(managedclusterinfogroupAPIVersion, "ManagedCluster", "", "name-foo", "test-mc-uid")
	//Ensure there is an entry for cluster_foo in the cluster cache
	database.UpdateClustersCache("cluster__name-foo", nil)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPool := pgxpoolmock.NewMockPgxPool(ctrl)
	// Prepare a mock DAO instance
	dao = database.NewDAO(mockPool)
	clusterName := "name-foo"
	clusterUID := "cluster__name-foo"
	mockConn, err := pgxmock.NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockConn.Close(context.Background())
	mockPool.EXPECT().BeginTx(context.TODO(), pgx.TxOptions{}).Return(mockConn, nil)
	mockConn.ExpectExec(`DELETE FROM search.resources`).WithArgs(clusterName).WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mockConn.ExpectExec(`DELETE FROM search.edges`).WithArgs(clusterName).WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mockConn.ExpectCommit()

	mockPool.EXPECT().Exec(gomock.Any(),
		gomock.Eq(`DELETE FROM search.resources WHERE uid=$1`),
		gomock.Eq([]interface{}{clusterUID}),
	).Return(nil, nil)

	processClusterDelete(context.TODO(), obj)

	//Once processClusterDelete is done, existingClustersCache should not have an entry for cluster foo
	_, ok := database.ReadClustersCache("cluster__name-foo")
	AssertEqual(t, ok, false, "existingClustersCache should not have an entry for cluster foo")

}

//Do not delete if addon name is not search-collector
func Test_ProcessClusterNoDeleteOnMCANotSearch(t *testing.T) {
	initializeVars()
	obj := newTestUnstructured(managedclusteraddongroupAPIVersion, "ManagedClusterAddOn", "name-foo", "work-manager", "test-mc-uid")

	//Ensure there is an entry for cluster_foo in the cluster cache
	database.UpdateClustersCache("cluster__name-foo", nil)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPool := pgxpoolmock.NewMockPgxPool(ctrl)
	// Prepare a mock DAO instance
	dao = database.NewDAO(mockPool)

	// Execute the function - delete should not happen as addon is not for search collector
	processClusterDelete(context.TODO(), obj)

	// Once processClusterDelete is done, existingClustersCache should still have an entry for cluster foo
	// as we are not deleting anything.
	_, ok := database.ReadClustersCache("cluster__name-foo")
	AssertEqual(t, ok, true, "existingClustersCache should still have an entry for cluster foo")

}

//Delete only if addon name is search-collector
func Test_ProcessClusterDeleteOnMCASearch(t *testing.T) {
	initializeVars()
	obj := newTestUnstructured(managedclusteraddongroupAPIVersion, "ManagedClusterAddOn", "name-foo", "search-collector", "test-mc-uid")

	//Ensure there is an entry for cluster_foo in the cluster cache
	database.UpdateClustersCache("cluster__name-foo", nil)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPool := pgxpoolmock.NewMockPgxPool(ctrl)
	// Prepare a mock DAO instance
	dao = database.NewDAO(mockPool)
	clusterName := "name-foo"
	mockConn, err := pgxmock.NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mockConn.Close(context.Background())
	mockPool.EXPECT().BeginTx(context.TODO(), pgx.TxOptions{}).Return(mockConn, nil)
	mockConn.ExpectExec(`DELETE FROM search.resources`).WithArgs(clusterName).WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mockConn.ExpectExec(`DELETE FROM search.edges`).WithArgs(clusterName).WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mockConn.ExpectCommit()

	processClusterDelete(context.TODO(), obj)

	// Once processClusterDelete is done, existingClustersCache should still have an entry for cluster foo
	// as we are not deleting it until MC is deleted.
	_, ok := database.ReadClustersCache("cluster__name-foo")
	AssertEqual(t, ok, true, "existingClustersCache should still have an entry for cluster foo")

}
