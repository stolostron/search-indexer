// Copyright Contributors to the Open Cluster Management project

package clustermgmt

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	clusterv1beta1 "github.com/stolostron/multicloud-operators-foundation/pkg/apis/internal.open-cluster-management.io/v1beta1"
	"github.com/stolostron/search-indexer/pkg/config"
	"github.com/stolostron/search-indexer/pkg/database"
	"github.com/stolostron/search-indexer/pkg/model"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

var dynamicClient dynamic.Interface

// Watches ManagedCluster objects and updates the database with a Cluster node.
func WatchClusters() {
	klog.Info("Begin ClusterWatch routine")

	dynamicClient = config.GetDynamicClient()
	dynamicFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 60*time.Second)

	// Create GVR for ManagedCluster
	managedClusterGvr, _ := schema.ParseResourceArg("managedclusters.v1.cluster.open-cluster-management.io")

	//Create Informers for ManagedCluster
	managedClusterInformer := dynamicFactory.ForResource(*managedClusterGvr).Informer()

	// Create handlers for events
	handlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			klog.Info("clusterWatch: AddFunc")
			processClusterUpsert(obj)
		},
		UpdateFunc: func(prev interface{}, next interface{}) {
			klog.Info("clusterWatch: UpdateFunc")
			processClusterUpsert(next)
		},
		// DeleteFunc: func(obj interface{}) {
		// 	klog.Info("clusterWatch: DeleteFunc")
		// 	processClusterDelete(obj)
		// },
	}

	// Add Handlers to both Informers
	managedClusterInformer.AddEventHandler(handlers)

	// Periodically check if the ManagedCluster resource exists
	go stopAndStartInformer("cluster.open-cluster-management.io/v1", managedClusterInformer)
}

// Stop and Start informer according to Rediscover Rate
func stopAndStartInformer(groupVersion string, informer cache.SharedIndexInformer) {
	var stopper chan struct{}
	informerRunning := false

	for {
		_, err := config.GetKubeClient().ServerResourcesForGroupVersion(groupVersion)
		// we fail to fetch for some reason other than not found
		if err != nil && !isClusterMissing(err) {
			klog.Errorf("Cannot fetch resource list for %s, error message: %s ", groupVersion, err)
		} else {
			if informerRunning && isClusterMissing(err) {
				klog.Infof("Stopping cluster informer routine because %s resource not found.", groupVersion)
				stopper <- struct{}{}
				informerRunning = false
			} else if !informerRunning && !isClusterMissing(err) {
				klog.Infof("Starting cluster informer routine for cluster watch for %s resource", groupVersion)
				stopper = make(chan struct{})
				informerRunning = true
				go informer.Run(stopper)
			}
		}
		time.Sleep(time.Duration(config.Cfg.RediscoverRateMS) * time.Millisecond)
	}
}

var mux sync.Mutex

func processClusterUpsert(obj interface{}) {
	// Lock so only one goroutine at a time can access add a cluster.
	// Helps to eliminate duplicate entries.
	mux.Lock()
	defer mux.Unlock()
	j, err := json.Marshal(obj.(*unstructured.Unstructured))
	if err != nil {
		klog.Warning("Error unmarshalling object from Informer in processClusterUpsert.")
	}

	// We update by name, and the name *should be* the same for a given cluster
	// Objects from a given cluster collide and update rather than duplicate insert
	// Unmarshall ManagedCluster

	var resource model.Resource
	managedCluster := clusterv1.ManagedCluster{}
	err = json.Unmarshal(j, &managedCluster)
	if err != nil {
		klog.Warning("Failed to Unmarshal MangedCluster", err)
	}
	klog.Info("Received ManagedCluster: ", managedCluster.Name)

	resource = transformManagedCluster(&managedCluster)

	dao := database.NewDAO(nil)

	// Upsert (attempt update, attempt insert on failure)
	dao.UpsertCluster(resource)

	// If a cluster is offline we remove the resources from that cluster, but leave the cluster resource object.
	/*if resource.Properties["status"] == "offline" {
		klog.Infof("Cluster %s is offline, removing cluster resources from datastore.", cluster.GetName())
		delClusterResources(cluster)
	}*/

}

func isClusterMissing(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "could not find the requested resource")
}

// Transform ManagedCluster object into db.Resource suitable for insert into DB
func transformManagedCluster(managedCluster *clusterv1.ManagedCluster) model.Resource {
	// https://github.com/stolostron/api/blob/main/cluster/v1/types.go#L78
	// We use ManagedCluster as the primary source of information
	// Properties duplicated between this and ManagedClusterInfo are taken from ManagedCluster
	props := make(map[string]interface{})

	if managedCluster.GetLabels() != nil {
		// Unmarshaling labels to map[string]interface{}
		var labelMap map[string]interface{}
		clusterLabels, _ := json.Marshal(managedCluster.GetLabels())
		err := json.Unmarshal(clusterLabels, &labelMap)
		if err == nil {
			props["label"] = labelMap
		}
	}

	props["kind"] = "Cluster"
	props["name"] = managedCluster.GetName()                  // must match ManagedClusterInfo
	props["_clusterNamespace"] = managedCluster.GetName()     // maps to the namespace of ManagedClusterInfo
	props["apigroup"] = "internal.open-cluster-management.io" // maps rbac to ManagedClusterInfo
	props["created"] = managedCluster.GetCreationTimestamp().UTC().Format(time.RFC3339)

	cpuCapacity := managedCluster.Status.Capacity["cpu"]
	props["cpu"], _ = cpuCapacity.AsInt64()
	memCapacity := managedCluster.Status.Capacity["memory"]
	props["memory"] = memCapacity.String()
	props["kubernetesVersion"] = managedCluster.Status.Version.Kubernetes

	for _, condition := range managedCluster.Status.Conditions {
		props[condition.Type] = string(condition.Status)
	}
	// Fetch corresponding ManagedClusterInfo
	managedClusterInfoGvr, _ := schema.ParseResourceArg("managedclusterinfos.v1beta1.internal.open-cluster-management.io")

	minfo, err := dynamicClient.Resource(*managedClusterInfoGvr).Namespace(managedCluster.GetName()).Get(context.TODO(), managedCluster.GetName(), v1.GetOptions{})
	if err != nil {
		klog.Warningf("Error fetching managedClusterInfo object %s from Informer in processClusterUpsert: %s", managedCluster.GetName(), err.Error())
	}
	j, _ := json.Marshal(minfo)

	managedClusterInfo := clusterv1beta1.ManagedClusterInfo{}
	mcInfoUnMarshalErr := json.Unmarshal(j, &managedClusterInfo)
	if mcInfoUnMarshalErr != nil {
		klog.Warning("Error unmarshalling managedClusterInfo object from Informer in processClusterUpsert.")
	}
	// Get properties from ManagedClusterInfo
	props["consoleURL"] = managedClusterInfo.Status.ConsoleURL
	props["nodes"] = int64(len(managedClusterInfo.Status.NodeList))

	resource := model.Resource{
		Kind:           "Cluster",
		UID:            string("cluster__" + managedCluster.GetName()),
		Properties:     props,
		ResourceString: "managedclusterinfos", // Maps rbac to ManagedClusterInfo
	}

	return resource
}

// Deletes a cluster resource and all resources from the cluster.
// func processClusterDelete(obj interface{}) {
// 	klog.Info("Processing Cluster Delete.")

// 	clusterName := obj.(*unstructured.Unstructured).GetName()
// 	clusterUID := string("cluster__" + obj.(*unstructured.Unstructured).GetName())
// 	klog.Infof("Deleting Cluster resource %s and all resources from the cluster. UID %s", clusterName, clusterUID)

// _, err := db.Delete([]string{clusterUID})
// if err != nil {
// 	klog.Error("Error deleting Cluster node with error: ", err)
// }
// delClusterResources(clusterUID, clusterName)
// }

// // Removes all the resources for a cluster, but doesn't remove the Cluster resource object.
// func delClusterResources(clusterUID string, clusterName string) {
// 	_, err := db.DeleteCluster(clusterName)
// 	if err != nil {
// 		klog.Error("Error deleting current resources for cluster: ", err)
// 	} else {
// 		db.DeleteClustersCache(clusterUID)
// 	}
// }
