// Copyright Contributors to the Open Cluster Management project
package clustersync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"testing"
	"time"

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
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// Create a GroupVersionResource
const managedclusterinfogroupAPIVersion = "internal.open-cluster-management.io/v1beta1"
const managedclustergroupAPIVersion = "cluster.open-cluster-management.io/v1"
const managedclusteraddongroupAPIVersion = "addon.open-cluster-management.io/v1alpha1"
const managedclusters = "managedclusters.open-cluster-management/v1"

var managedClusterGvr *schema.GroupVersionResource
var managedClusterInfoGvr *schema.GroupVersionResource
var managedClusterAddonGvr *schema.GroupVersionResource
var existingCluster map[string]interface{}

func fakeDynamicClient() *fake.FakeDynamicClient {
	managedClusterGvr, _ = schema.ParseResourceArg(managedClusterGVR)
	managedClusterInfoGvr, _ = schema.ParseResourceArg(managedClusterInfoGVR)
	managedClusterAddonGvr, _ = schema.ParseResourceArg(managedClusterAddonGVR)

	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(managedClusterGvr.GroupVersion())
	scheme.AddKnownTypes(managedClusterInfoGvr.GroupVersion())
	scheme.AddKnownTypes(managedClusterAddonGvr.GroupVersion())

	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "cluster.open-cluster-management.io", Version: "v1", Kind: "ManagedCluster"},
		&unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "cluster.open-cluster-management.io", Version: "v1", Kind: "ManagedClusterList"},
		&unstructured.UnstructuredList{})

	scheme.AddKnownTypes(schema.GroupVersionResource{Group: "clusters-open-cluster-management.io", Version: "v1", Resource: "managedclusters"}.GroupVersion(),
		&unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "internal.open-cluster-management.io", Version: "v1beta1", Kind: "ManagedClusterInfoList"},
		&unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "addon.open-cluster-management.io", Version: "v1alpha1", Kind: "ManagedClusterAddOnList"},
		&unstructured.UnstructuredList{})

	dyn := fake.NewSimpleDynamicClient(scheme,
		newTestUnstructured(managedclusters, "ManagedCluster", "", "name-foo", ""),
		newTestUnstructured(managedclusterinfogroupAPIVersion, "ManagedClusterInfo", "name-foo", "name-foo", ""),
		newTestUnstructured(managedclustergroupAPIVersion, "ManagedCluster", "", "name-foo", ""),
		newTestUnstructured(managedclustergroupAPIVersion, "ManagedCluster", "", "name-foo-error", ""))
	_, err := dyn.Resource(*managedClusterGvr).Get(context.Background(), "name-foo", v1.GetOptions{})
	if err != nil {
		klog.Warning("Error creating fake NewSimpleDynamicClient: ", err.Error())
	}
	return dyn
}

func newTestUnstructured(apiVersion, kind, namespace, name, uid string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
				"uid":       uid,
				"labels": map[string]interface{}{
					"env": "dev",
					"feature.open-cluster-management.io/addon-search-collector": "available",
				},
			},
		},
	}
}

func initializeVars() {
	clusterProps := map[string]interface{}{
		"label": map[string]string{
			"env": "dev",
			"feature.open-cluster-management.io/addon-search-collector": "available",
		},
		"addon": map[string]string{
			"application-manager":         "false",
			"cert-policy-controller":      "false",
			"cluster-proxy":               "false",
			"config-policy-controller":    "false",
			"governance-policy-framework": "false",
			"iam-policy-controller":       "false",
			"observability-controller":    "false",
			"search-collector":            "true",
			"work-manager":                "false",
		},
		"apigroup":            managedClusterInfoApiGrp,
		"kind_plural":         "managedclusterinfos",
		"cpu":                 0,
		"created":             "0001-01-01T00:00:00Z",
		"kind":                "Cluster",
		"kubernetesVersion":   "",
		"memory":              "0",
		"name":                "name-foo",
		"_hubClusterResource": true,
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
		gomock.Eq(`SELECT "uid", "data" FROM "search"."resources" WHERE ("uid" = 'cluster__name-foo')`),
		gomock.Eq([]interface{}{}),
	).Return(nil, nil)

	sql := fmt.Sprintf(`INSERT INTO "search"."resources" AS "r" ("cluster", "data", "uid") VALUES ('name-foo', '%[1]s', '%[2]s') ON CONFLICT (uid) DO UPDATE SET "data"='%[1]s' WHERE ("r".uid = '%[2]s')`, string(expectedProps), "cluster__name-foo")
	mockPool.EXPECT().Exec(gomock.Any(),
		gomock.Eq(sql),
		gomock.Eq([]interface{}{}),
	).Return(nil, nil)

	processClusterUpsert(context.Background(), obj)
	// Once processClusterUpsert is done, existingClustersCache should have an entry for cluster foo
	_, ok := database.ReadClustersCache("cluster__name-foo")
	AssertEqual(t, ok, true, "existingClustersCache should have an entry for cluster foo")

}

func Test_ProcessClusterUpsert_ManagedClusterInfo(t *testing.T) {
	initializeVars()
	// Ensure there is an entry for cluster_foo in the cluster cache
	database.UpdateClustersCache("cluster__name-foo", existingCluster["Properties"])
	obj := newTestUnstructured(managedclusterinfogroupAPIVersion, "ManagedClusterInfo", "name-foo", "name-foo", "test-mc-uid")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPool := pgxpoolmock.NewMockPgxPool(ctrl)
	// Prepare a mock DAO instance
	dao = database.NewDAO(mockPool)
	dynamicClient = fakeDynamicClient()
	// Add props specific to ManagedClusterInfo
	props := existingCluster["Properties"].(map[string]interface{})
	props["apiEndpoint"] = ""
	props["consoleURL"] = ""
	props["nodes"] = 0
	existingCluster["Properties"] = props
	expectedProps, _ := json.Marshal(existingCluster["Properties"])

	sql := fmt.Sprintf(`INSERT INTO "search"."resources" AS "r" ("cluster", "data", "uid") VALUES ('name-foo', '%[1]s', '%[2]s') ON CONFLICT (uid) DO UPDATE SET "data"='%[1]s' WHERE ("r".uid = '%[2]s')`, string(expectedProps), "cluster__name-foo")
	mockPool.EXPECT().Exec(gomock.Any(),
		gomock.Eq(sql),
		gomock.Eq([]interface{}{}),
	).Return(nil, nil)

	processClusterUpsert(context.Background(), obj)
	// Once processClusterUpsert is done, existingClustersCache should have an entry for cluster foo
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
	processClusterDelete(context.Background(), obj)

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

	mockConn, err := pgxmock.NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer func(mockConn pgxmock.PgxConnIface, ctx context.Context) {
		_ = mockConn.Close(ctx)
	}(mockConn, context.Background())
	mockPool.EXPECT().BeginTx(context.Background(), pgx.TxOptions{}).Return(mockConn, nil)
	mockConn.ExpectExec(regexp.QuoteMeta(`DELETE FROM "search"."resources" WHERE (("cluster" = 'name-foo') AND ("uid" != 'cluster__name-foo'))`)).WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mockConn.ExpectExec(regexp.QuoteMeta(`DELETE FROM "search"."edges" WHERE ("cluster" = 'name-foo')`)).WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mockConn.ExpectCommit()

	mockPool.EXPECT().Exec(gomock.Any(),
		gomock.Eq(`DELETE FROM "search"."resources" WHERE ("uid" = 'cluster__name-foo')`),
		gomock.Eq([]interface{}{}),
	).Return(nil, nil)

	processClusterDelete(context.Background(), obj)

	//Once processClusterDelete is done, existingClustersCache should not have an entry for cluster foo
	_, ok := database.ReadClustersCache("cluster__name-foo")
	AssertEqual(t, ok, false, "existingClustersCache should not have an entry for cluster foo")

}

// Delete only if addon name is search-collector
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
	mockConn, err := pgxmock.NewConn()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer func(mockConn pgxmock.PgxConnIface, ctx context.Context) {
		_ = mockConn.Close(ctx)
	}(mockConn, context.Background())
	mockPool.EXPECT().BeginTx(context.Background(), pgx.TxOptions{}).Return(mockConn, nil)
	mockConn.ExpectExec(regexp.QuoteMeta(`DELETE FROM "search"."resources" WHERE (("cluster" = 'name-foo') AND ("uid" != 'cluster__name-foo'))`)).WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mockConn.ExpectExec(regexp.QuoteMeta(`DELETE FROM "search"."edges" WHERE ("cluster" = 'name-foo')`)).WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mockConn.ExpectCommit()

	processClusterDelete(context.Background(), obj)

	// Once processClusterDelete is done, existingClustersCache should still have an entry for cluster foo
	// as we are not deleting it until MC is deleted.
	_, ok := database.ReadClustersCache("cluster__name-foo")
	AssertEqual(t, ok, true, "existingClustersCache should still have an entry for cluster foo")

}

func Test_AddAdditionalProps(t *testing.T) {
	props := map[string]interface{}{}
	props["kind"] = "Cluster"
	props["name"] = "cluster1"

	//execute function
	updatedProps := addAdditionalProperties(props)
	apigroup, apigroupPresent := updatedProps["apigroup"]
	AssertEqual(t, apigroup, managedClusterInfoApiGrp, "Expected apigroup not found.")
	AssertEqual(t, apigroupPresent, true, "Expected apigroup to be set")
	kindPlural, kindPluralPresent := updatedProps["kind_plural"]
	AssertEqual(t, kindPlural, "managedclusterinfos", "Expected kindPlural not found.")
	AssertEqual(t, kindPluralPresent, true, "Expected kindPlural to be set")
}

// Find stale cluster resources, if found, delete them
func Test_DeleteStaleClustersResources(t *testing.T) {
	//ensure cluster in cache exists
	initializeVars()

	//add two clusters to cache one that will exist in kube and one that will not exist in kube
	database.UpdateClustersCache("cluster__name-foo", existingCluster["Properties"])
	database.UpdateClustersCache("cluster__remaining-managed-foo", existingCluster["Properties"])

	//managed cluster objs to create in with kube client:
	obj := newTestUnstructured(managedclusterinfogroupAPIVersion, "ManagedCluster", "name-foo", "name-foo", "test-mc-uid")
	//search-addon for managed cluster name-foo:
	obj3 := newTestUnstructured(managedclusteraddongroupAPIVersion, "ManagedClusterAddOn", "name-foo", "search-collector", "test-mc-uid")
	//add label to identify addon:
	label := make(map[string]string)
	label["feature.open-cluster-management.io/addon-search-collector"] = "available"
	obj.SetLabels(label)
	//create obj in with client:
	dynamicClient := fakeDynamicClient()
	_, clientErr := dynamicClient.Resource(*managedClusterGvr).Namespace("name-foo").Create(context.Background(), obj, v1.CreateOptions{})
	if clientErr != nil {
		t.Errorf("an error '%s' has occured while trying to create resources", clientErr)
	}
	//create the addon in namespace name-foo:
	_, clientErr = dynamicClient.Resource(*managedClusterAddonGvr).Namespace("name-foo").Create(context.Background(), obj3, v1.CreateOptions{})

	if clientErr != nil {
		t.Errorf("an error '%s' has occured while trying to create resources", clientErr)
	}

	// Prepare a mock DAO instance
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPool := pgxpoolmock.NewMockPgxPool(ctrl)
	dao = database.NewDAO(mockPool)
	mockConn, err := pgxmock.NewConn()
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}

	defer func(mockConn pgxmock.PgxConnIface, ctx context.Context) {
		_ = mockConn.Close(ctx)
	}(mockConn, context.Background())
	mockPool.EXPECT().BeginTx(context.Background(), pgx.TxOptions{}).Return(mockConn, nil)
	mockConn.ExpectExec(regexp.QuoteMeta(`DELETE FROM "search"."resources" WHERE (("cluster" = 'name-foo') AND ("uid" != 'cluster__name-foo'))`)).WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mockConn.ExpectExec(regexp.QuoteMeta(`DELETE FROM "search"."edges" WHERE ("cluster" = 'name-foo')`)).WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mockConn.ExpectCommit()

	mockPool.EXPECT().Exec(gomock.Any(),
		gomock.Eq(`DELETE FROM "search"."resources" WHERE ("uid" = 'cluster__name-foo')`),
		gomock.Eq([]interface{}{}),
	).Return(nil, nil)
	//delete managed cluster:
	processClusterDelete(context.Background(), obj)

	columns := []string{"cluster"}
	pgxRows := pgxpoolmock.NewRows(columns).AddRow("name-foo").AddRow("remaining-managed-foo").ToPgxRows()

	mockPool.EXPECT().Query(gomock.Any(),
		gomock.Eq(`SELECT DISTINCT "cluster" FROM "search"."resources" WHERE ((data ? '_hubClusterResource') IS FALSE)`),
		gomock.Eq([]interface{}{}),
	).Return(pgxRows, nil).Times(2)

	// Execute function test - the clusters in mc are to be deleted
	mc, _ := findStaleClusterResources(context.Background(), dynamicClient, *managedClusterGvr)

	err = deleteStaleClusterResources(context.Background(), dynamicClient, *managedClusterGvr)
	if err != nil {
		t.Errorf("Error processing delete for remaining cluster: %s", err)
	}

	//ensure that the remaining clusters are deleted from db
	for _, c := range mc {
		if c != "remaining-managed-foo" {
			t.Errorf("Remaining cluster does not match. Expected: remaining-managed-foo Got: %s", c)
		}

		_, ok := database.ReadClustersCache(c)
		AssertEqual(t, ok, false, "existingClustersCache should not have an entry for cluster foo")
	}

}

// [AI] Test that stopAndStartInformer exits promptly when context is canceled
func Test_stopAndStartInformer_ContextCanceled(t *testing.T) {
	// Create a context that we'll cancel immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Create a nil informer (it won't be used since context is already canceled)
	var informer cache.SharedIndexInformer

	// Start the function in a goroutine
	done := make(chan bool)
	go func() {
		stopAndStartInformer(ctx, "test.group/v1", informer)
		done <- true
	}()

	// Wait for the function to exit (with timeout)
	// Since context is already canceled, it should exit immediately
	select {
	case <-done:
		// Success - function exited when it detected the canceled context
	case <-time.After(10 * time.Millisecond):
		t.Error("stopAndStartInformer did not exit within timeout after context cancellation")
	}
}

// Mock database outage:
func Test_DeleteStaleClustersResources_DB_Outage(t *testing.T) {
	//ensure cluster in cache exists
	initializeVars()

	//add two clusters to cache one that will exist in kube and one that will not exist in kube
	database.UpdateClustersCache("cluster__name-foo", existingCluster["Properties"])
	database.UpdateClustersCache("cluster__remaining-managed-foo", existingCluster["Properties"])

	//managed cluster obj to create in with kube client:
	obj := newTestUnstructured(managedclusterinfogroupAPIVersion, "ManagedCluster", "name-foo", "name-foo", "test-mc-uid")

	//add label to identify addon:
	label := make(map[string]string)
	label["feature.open-cluster-management.io/addon-search-collector"] = "available"
	obj.SetLabels(label)

	//create obj in with client:
	dynamicClient := fakeDynamicClient()
	_, clientErr := dynamicClient.Resource(*managedClusterGvr).Namespace("name-foo").Create(context.Background(), obj, v1.CreateOptions{})
	if clientErr != nil {
		t.Errorf("an error '%s' has occured while trying to create resources", clientErr)
	}

	// Prepare a mock DAO instance
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPool := pgxpoolmock.NewMockPgxPool(ctrl)
	dao = database.NewDAO(mockPool)
	mockConn, err := pgxmock.NewConn()
	//mock db error
	fakeErr := errors.New("Mock DB Error")
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}

	defer func(mockConn pgxmock.PgxConnIface, ctx context.Context) {
		_ = mockConn.Close(ctx)
	}(mockConn, context.Background())
	mockPool.EXPECT().BeginTx(context.Background(), pgx.TxOptions{}).Return(mockConn, fakeErr).Times(1).Return(mockConn, nil).Times(1) // return mock error
	mockConn.ExpectExec(regexp.QuoteMeta(`DELETE FROM "search"."resources" WHERE (("cluster" = 'name-foo') AND ("uid" != 'cluster__name-foo'))`)).WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mockConn.ExpectExec(regexp.QuoteMeta(`DELETE FROM "search"."edges" WHERE ("cluster" = 'name-foo')`)).WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mockConn.ExpectCommit()

	mockPool.EXPECT().Exec(gomock.Any(),
		gomock.Eq(`DELETE FROM "search"."resources" WHERE ("uid" = 'cluster__name-foo')`),
		gomock.Eq([]interface{}{}),
	).Return(nil, nil)
	//delete managed cluster:

	processClusterDelete(context.Background(), obj)

	columns := []string{"cluster"}
	pgxRows := pgxpoolmock.NewRows(columns).AddRow("name-foo").AddRow("remaining-managed-foo").ToPgxRows()

	mockPool.EXPECT().Query(gomock.Any(),
		gomock.Eq(`SELECT DISTINCT "cluster" FROM "search"."resources" WHERE ((data ? '_hubClusterResource') IS FALSE)`),
		gomock.Eq([]interface{}{}),
	).Return(pgxRows, nil)

	// Execute function test
	mc, _ := findStaleClusterResources(context.Background(), dynamicClient, *managedClusterGvr)

	//Once findStaleClusterResources is done, existingClustersCache should not have an entry for remaining-managed-foo
	for _, c := range mc {
		_, ok := database.ReadClustersCache(c)
		AssertEqual(t, ok, false, "existingClustersCache should not have an entry for cluster foo")
	}

}

// [AI] Test that syncClusters properly exits when context is canceled
func Test_syncClusters_ContextCanceled(t *testing.T) {
	// Suppress log output for cleaner test output
	restore := supressConsoleOutput()
	defer restore()

	initializeVars()

	// Set up the mock infrastructure
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPool := pgxpoolmock.NewMockPgxPool(ctrl)
	dao = database.NewDAO(mockPool)
	dynamicClient = fakeDynamicClient()

	// Mock the deleteStaleClusterResources call
	columns := []string{"cluster"}
	pgxRows := pgxpoolmock.NewRows(columns).ToPgxRows()
	mockPool.EXPECT().Query(gomock.Any(),
		gomock.Eq(`SELECT DISTINCT "cluster" FROM "search"."resources" WHERE ((data ? '_hubClusterResource') IS FALSE)`),
		gomock.Eq([]interface{}{}),
	).Return(pgxRows, nil).AnyTimes()

	// Mock the upsert operations that will be triggered by informers processing existing resources
	// Allow any Query calls for checking existing resources
	mockPool.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	// Allow any Exec calls for upserting clusters
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	// Create a context with a short timeout to ensure syncClusters exits
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start syncClusters in a goroutine
	done := make(chan bool)
	go func() {
		syncClusters(ctx)
		done <- true
	}()

	// Wait for the function to exit (with a reasonable timeout)
	select {
	case <-done:
		// Success - function exited when context was canceled
	case <-time.After(500 * time.Millisecond):
		t.Error("syncClusters did not exit within timeout after context cancellation")
	}
}

// [AI]Test that syncClusters handles errors during deleteStaleClusterResources
func Test_syncClusters_DeleteStaleError(t *testing.T) {
	// Suppress log output for cleaner test output
	restore := supressConsoleOutput()
	defer restore()

	initializeVars()

	// Set up the mock infrastructure
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPool := pgxpoolmock.NewMockPgxPool(ctrl)
	dao = database.NewDAO(mockPool)
	dynamicClient = fakeDynamicClient()

	// Mock the deleteStaleClusterResources call to return an error
	mockPool.EXPECT().Query(gomock.Any(),
		gomock.Eq(`SELECT DISTINCT "cluster" FROM "search"."resources" WHERE ((data ? '_hubClusterResource') IS FALSE)`),
		gomock.Eq([]interface{}{}),
	).Return(nil, errors.New("database connection error")).AnyTimes()

	// Mock the upsert operations that will be triggered by informers processing existing resources
	// Allow any Query calls for checking existing resources
	mockPool.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	// Allow any Exec calls for upserting clusters
	mockPool.EXPECT().Exec(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	// Create a context with a short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start syncClusters in a goroutine - it should handle the error gracefully
	done := make(chan bool)
	go func() {
		syncClusters(ctx)
		done <- true
	}()

	// Wait for the function to exit (with a reasonable timeout)
	select {
	case <-done:
		// Success - function handled the error and exited when context was canceled
	case <-time.After(500 * time.Millisecond):
		t.Error("syncClusters did not exit within timeout after error in deleteStaleClusterResources")
	}
}

func Test_storeDesiredFields_NonUnstructured(t *testing.T) {
	input := "not an unstructured object"
	result, err := storeDesiredFields(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != input {
		t.Errorf("expected passthrough, got %v", result)
	}
}

func Test_storeDesiredFields_ManagedCluster(t *testing.T) {
	input := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cluster.open-cluster-management.io/v1",
		"kind":       "ManagedCluster",
		"metadata": map[string]interface{}{
			"name":              "cluster1",
			"uid":               "should-be-stripped",
			"resourceVersion":   "12345",
			"creationTimestamp":  "2024-01-01T00:00:00Z",
			"labels":            map[string]interface{}{"env": "prod"},
			"managedFields":     []interface{}{"big", "blob"},
			"annotations":       map[string]interface{}{"note": "strip-me"},
		},
		"spec": map[string]interface{}{
			"hubAcceptsClient": true,
		},
		"status": map[string]interface{}{
			"capacity":   map[string]interface{}{"cpu": "8"},
			"version":    map[string]interface{}{"kubernetes": "v1.28.0"},
			"conditions": []interface{}{map[string]interface{}{"type": "Available"}},
			"allocatable": map[string]interface{}{"cpu": "6"},
		},
	}}

	result, err := storeDesiredFields(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	u := result.(*unstructured.Unstructured)

	if u.GetAPIVersion() != "cluster.open-cluster-management.io/v1" {
		t.Errorf("apiVersion mismatch: %s", u.GetAPIVersion())
	}
	if u.GetKind() != "ManagedCluster" {
		t.Errorf("kind mismatch: %s", u.GetKind())
	}
	if u.GetName() != "cluster1" {
		t.Errorf("name mismatch: %s", u.GetName())
	}
	if u.GetLabels()["env"] != "prod" {
		t.Errorf("labels mismatch: %v", u.GetLabels())
	}

	status, _ := u.Object["status"].(map[string]interface{})
	if status == nil {
		t.Fatal("status should be present")
	}
	if _, ok := status["capacity"]; !ok {
		t.Error("status.capacity should be kept")
	}
	if _, ok := status["version"]; !ok {
		t.Error("status.version should be kept")
	}
	if _, ok := status["conditions"]; !ok {
		t.Error("status.conditions should be kept")
	}
	if _, ok := status["allocatable"]; ok {
		t.Error("status.allocatable should be stripped")
	}

	// Verify stripped fields are absent.
	if _, ok := u.Object["spec"]; ok {
		t.Error("spec should be stripped from ManagedCluster")
	}
	meta, _ := u.Object["metadata"].(map[string]interface{})
	if _, ok := meta["uid"]; ok {
		t.Error("metadata.uid should be stripped")
	}
	if _, ok := meta["managedFields"]; ok {
		t.Error("metadata.managedFields should be stripped")
	}
	if _, ok := meta["annotations"]; ok {
		t.Error("metadata.annotations should be stripped")
	}
	if _, ok := meta["resourceVersion"]; ok {
		t.Error("metadata.resourceVersion should be stripped")
	}
}

func Test_storeDesiredFields_ManagedClusterInfo(t *testing.T) {
	input := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "internal.open-cluster-management.io/v1beta1",
		"kind":       "ManagedClusterInfo",
		"metadata": map[string]interface{}{
			"name":            "cluster1",
			"namespace":       "cluster1",
			"uid":             "should-be-stripped",
			"managedFields":   []interface{}{"big"},
			"resourceVersion": "999",
		},
		"spec": map[string]interface{}{
			"masterEndpoint": "https://api.cluster1:6443",
			"loggingCA":      "strip-me",
		},
		"status": map[string]interface{}{
			"consoleURL":    "https://console.cluster1",
			"nodeList":      []interface{}{map[string]interface{}{"name": "node1"}},
			"distributionInfo": map[string]interface{}{"type": "OCP"},
			"conditions":    []interface{}{map[string]interface{}{"type": "Available"}},
		},
	}}

	result, err := storeDesiredFields(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	u := result.(*unstructured.Unstructured)

	if u.GetName() != "cluster1" {
		t.Errorf("name mismatch: %s", u.GetName())
	}
	if u.GetKind() != "ManagedClusterInfo" {
		t.Errorf("kind mismatch: %s", u.GetKind())
	}

	spec, _ := u.Object["spec"].(map[string]interface{})
	if spec == nil {
		t.Fatal("spec should be present")
	}
	if spec["masterEndpoint"] != "https://api.cluster1:6443" {
		t.Errorf("spec.masterEndpoint mismatch: %v", spec["masterEndpoint"])
	}
	if _, ok := spec["loggingCA"]; ok {
		t.Error("spec.loggingCA should be stripped")
	}

	status, _ := u.Object["status"].(map[string]interface{})
	if status == nil {
		t.Fatal("status should be present")
	}
	if status["consoleURL"] != "https://console.cluster1" {
		t.Error("status.consoleURL should be kept")
	}
	if _, ok := status["nodeList"]; !ok {
		t.Error("status.nodeList should be kept")
	}
	if _, ok := status["distributionInfo"]; ok {
		t.Error("status.distributionInfo should be stripped")
	}
	if _, ok := status["conditions"]; ok {
		t.Error("status.conditions should be stripped")
	}

	meta, _ := u.Object["metadata"].(map[string]interface{})
	if _, ok := meta["uid"]; ok {
		t.Error("metadata.uid should be stripped")
	}
}

func Test_storeDesiredFields_ManagedClusterAddOn(t *testing.T) {
	input := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "addon.open-cluster-management.io/v1alpha1",
		"kind":       "ManagedClusterAddOn",
		"metadata": map[string]interface{}{
			"name":            "search-collector",
			"namespace":       "cluster1",
			"uid":             "should-be-stripped",
			"managedFields":   []interface{}{"big"},
			"resourceVersion": "777",
			"annotations":     map[string]interface{}{"note": "strip-me"},
		},
		"spec": map[string]interface{}{
			"installNamespace": "open-cluster-management-agent-addon",
		},
		"status": map[string]interface{}{
			"conditions": []interface{}{map[string]interface{}{"type": "Available"}},
		},
	}}

	result, err := storeDesiredFields(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	u := result.(*unstructured.Unstructured)

	if u.GetName() != "search-collector" {
		t.Errorf("name mismatch: %s", u.GetName())
	}
	if u.GetNamespace() != "cluster1" {
		t.Errorf("namespace mismatch: %s", u.GetNamespace())
	}
	if u.GetKind() != "ManagedClusterAddOn" {
		t.Errorf("kind mismatch: %s", u.GetKind())
	}

	if _, ok := u.Object["spec"]; ok {
		t.Error("spec should be stripped from ManagedClusterAddOn")
	}
	if _, ok := u.Object["status"]; ok {
		t.Error("status should be stripped from ManagedClusterAddOn")
	}
	meta, _ := u.Object["metadata"].(map[string]interface{})
	if _, ok := meta["uid"]; ok {
		t.Error("metadata.uid should be stripped")
	}
	if _, ok := meta["annotations"]; ok {
		t.Error("metadata.annotations should be stripped")
	}
}

func Test_storeDesiredFields_DoesNotMutateOriginal(t *testing.T) {
	input := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "cluster.open-cluster-management.io/v1",
		"kind":       "ManagedCluster",
		"metadata": map[string]interface{}{
			"name":      "cluster1",
			"uid":       "original-uid",
			"labels":    map[string]interface{}{"env": "prod"},
		},
		"spec": map[string]interface{}{
			"hubAcceptsClient": true,
		},
		"status": map[string]interface{}{
			"capacity":    map[string]interface{}{"cpu": "8"},
			"allocatable": map[string]interface{}{"cpu": "6"},
		},
	}}

	_, err := storeDesiredFields(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify original is untouched.
	meta := input.Object["metadata"].(map[string]interface{})
	if meta["uid"] != "original-uid" {
		t.Error("original metadata.uid was mutated")
	}
	if _, ok := input.Object["spec"]; !ok {
		t.Error("original spec was removed")
	}
	status := input.Object["status"].(map[string]interface{})
	if _, ok := status["allocatable"]; !ok {
		t.Error("original status.allocatable was removed")
	}
}
