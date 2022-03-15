// // // Copyright Contributors to the Open Cluster Management project
package clustermgmt

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

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
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "open-cluster-management.io", Version: "v1", Kind: "TheKind"},
		&unstructured.UnstructuredList{})

	dyn := fake.NewSimpleDynamicClient(scheme, newTestUnstructured(managedclusterinfogroupAPIVersion, "ManagedClusterInfo", "name-foo", "name-foo", ""),
		newTestUnstructured(managedclustergroupAPIVersion, "ManagedCluster", "", "name-foo", ""),
		newTestUnstructured(managedclustergroupAPIVersion, "ManagedCluster", "", "name-foo-error", ""))
	_, err := dyn.Resource(*managedClusterGvr).Get(context.TODO(), "name-foo", v1.GetOptions{})
	klog.Error("Error: ", err)
	return dyn
}

// func generateSimpleEvent(informer GenericInformer, t *testing.T) {
// 	// Add resource. Generates ADDED event.
// 	newResource := newTestUnstructured("open-cluster-management.io/v1", "TheKind", "ns-foo", "name-new", "id-999")
// 	_, err1 := informer.client.Resource(gvr).Namespace("ns-foo").Create(contextVar, newResource, v1.CreateOptions{})

// 	// Update resource. Generates MODIFIED event.
// 	_, err2 := informer.client.Resource(gvr).Namespace("ns-foo").Update(contextVar, newResource, v1.UpdateOptions{})

// 	// Delete resource. Generated DELETED event.
// 	err3 := informer.client.Resource(gvr).Namespace("ns-foo").Delete(contextVar, "name-bar2", v1.DeleteOptions{})

// 	if err1 != nil || err2 != nil || err3 != nil {
// 		t.Error("Error generating mocked events.")
// 	}
// }

func newTestUnstructured(apiVersion, kind, namespace, name, uid string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
				"uid":       uid,
			},
		},
	}
}

// // Verify that a generic informer can be created.
func Test_ProcessClusterUpsert(t *testing.T) {
	database.ExistingClustersMap = make(map[string]interface{})
	clusterResource := `{"_clusterNamespace":"name-foo", "apigroup":"internal.open-cluster-management.io", "consoleURL":"", "cpu":0, "created":"0001-01-01T00:00:00Z", "kind":"Cluster", "kubernetesVersion":"", "memory":0, "name":"name-foo", "nodes":0}`
	var clusterRes map[string]interface{}
	json.Unmarshal([]byte(clusterResource), &clusterRes)
	clusterRes["cpu"] = int64(0)
	clusterRes["memory"] = "0"
	clusterRes["nodes"] = int64(0)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Prepare a mock DAO instance
	database.ExistingClustersMap["cluster__name-foo"] = clusterRes
	dynamicClient = fakeDynamicClient()
	processClusterUpsert("name-foo")
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
