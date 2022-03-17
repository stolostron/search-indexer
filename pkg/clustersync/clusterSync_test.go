// // // Copyright Contributors to the Open Cluster Management project
package clustersync

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/driftprogramming/pgxpoolmock"
	"github.com/golang/mock/gomock"
	"github.com/stolostron/search-indexer/pkg/database"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/klog/v2"
)

// // Create a GroupVersionResource
var managedclusterinfogroupAPIVersion = "internal.open-cluster-management.io/v1beta1"
var managedclustergroupAPIVersion = "cluster.open-cluster-management.io/v1"
var managedClusterGvr *schema.GroupVersionResource
var managedClusterInfoGvr *schema.GroupVersionResource

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

// Verify that ProcessClusterUpsert works.
func Test_ProcessClusterUpsert_ManagedCluster(t *testing.T) {
	database.ExistingClustersMap = make(map[string]interface{})
	obj := newTestUnstructured(managedclustergroupAPIVersion, "ManagedCluster", "", "name-foo", "test-mc-uid")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPool := pgxpoolmock.NewMockPgxPool(ctrl)
	// Prepare a mock DAO instance
	dao = database.NewDAO(mockPool)
	dynamicClient = fakeDynamicClient()
	clusterResource := `{"UID":"cluster__name-foo", "Kind":"Cluster", "Properties":{"_clusterNamespace":"name-foo" ,"label":{"env":"dev"},"apigroup":"internal.open-cluster-management.io", "cpu":0,"memory":"0", "created":"0001-01-01T00:00:00Z", "kind":"Cluster", "kubernetesVersion":"", "name":"name-foo" }}`

	var existingCluster map[string]interface{}
	_ = json.Unmarshal([]byte(clusterResource), &existingCluster)

	expectedProps, _ := json.Marshal(existingCluster["Properties"])
	mockPool.EXPECT().Query(gomock.Any(),
		gomock.Eq(`SELECT uid, data from search.resources where uid=$1`),
		gomock.Eq([]interface{}{"cluster__name-foo"}),
	).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(),
		gomock.Eq(`INSERT INTO search.resources as r (uid, cluster, data) values($1,'',$2) ON CONFLICT (uid) DO UPDATE SET data=$2 WHERE r.uid=$1`),
		gomock.Eq([]interface{}{"cluster__name-foo", string(expectedProps)}),
	).Return(nil, nil)

	processClusterUpsert(obj)
	//Once processClusterUpsert is done, ExistingClustersMap should have an entry for cluster foo
	AssertEqual(t, len(database.ExistingClustersMap), 1, "ExistingClustersMap should have length of 1")
	_, ok := database.ExistingClustersMap["cluster__name-foo"]
	AssertEqual(t, ok, true, "ExistingClustersMap should have an entry for cluster foo")

}

func Test_ProcessClusterUpsert_ManagedClusterInfo(t *testing.T) {
	database.ExistingClustersMap = make(map[string]interface{})
	obj := newTestUnstructured(managedclusterinfogroupAPIVersion, "ManagedClusterInfo", "name-foo", "name-foo", "test-mc-uid")

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockPool := pgxpoolmock.NewMockPgxPool(ctrl)
	// Prepare a mock DAO instance
	dao = database.NewDAO(mockPool)
	dynamicClient = fakeDynamicClient()
	clusterResource := `{"UID":"cluster__name-foo", "Kind":"Cluster", "Properties":{"_clusterNamespace":"name-foo", "nodes":0,"apigroup":"internal.open-cluster-management.io", "consoleURL":"", "kind":"Cluster", "name":"name-foo" }}`

	var existingCluster map[string]interface{}
	_ = json.Unmarshal([]byte(clusterResource), &existingCluster)

	expectedProps, _ := json.Marshal(existingCluster["Properties"])
	mockPool.EXPECT().Query(gomock.Any(),
		gomock.Eq(`SELECT uid, data from search.resources where uid=$1`),
		gomock.Eq([]interface{}{"cluster__name-foo"}),
	).Return(nil, nil)
	mockPool.EXPECT().Exec(gomock.Any(),
		gomock.Eq(`INSERT INTO search.resources as r (uid, cluster, data) values($1,'',$2) ON CONFLICT (uid) DO UPDATE SET data=$2 WHERE r.uid=$1`),
		gomock.Eq([]interface{}{"cluster__name-foo", string(expectedProps)}),
	).Return(nil, nil)

	processClusterUpsert(obj)
	//Once processClusterUpsert is done, ExistingClustersMap should have an entry for cluster foo
	AssertEqual(t, len(database.ExistingClustersMap), 1, "ExistingClustersMap should have length of 1")
	_, ok := database.ExistingClustersMap["cluster__name-foo"]
	AssertEqual(t, ok, true, "ExistingClustersMap should have an entry for cluster foo")

}

// AssertEqual checks if values are equal
func AssertEqual(t *testing.T, a interface{}, b interface{}, message string) {
	if a == b {
		return
	}
	t.Errorf("%s Received %v (type %v), expected %v (type %v)", message, a, reflect.TypeOf(a), b, reflect.TypeOf(b))
}
