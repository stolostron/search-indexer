// Copyright Contributors to the Open Cluster Management project

package clustersync

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	klog "k8s.io/klog/v2"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

var dynamicClient dynamic.Interface
var dao database.DAO
var client *kubernetes.Clientset
var mux sync.Mutex

const managedClusterGVR = "managedclusters.v1.cluster.open-cluster-management.io"
const managedClusterInfoGVR = "managedclusterinfos.v1beta1.internal.open-cluster-management.io"
const managedClusterAddonGVR = "managedclusteraddons.v1alpha1.addon.open-cluster-management.io"
const lockName = "search-indexer.open-cluster-management.io"
const managedClusterInfoApiGrp = "internal.open-cluster-management.io"

var allAddons = [9]string{
	"application-manager",
	"cert-policy-controller",
	"cluster-proxy",
	"config-policy-controller",
	"governance-policy-framework",
	"iam-policy-controller",
	"observability-controller",
	"search-collector",
	"work-manager",
}

func ElectLeaderAndStart(ctx context.Context) {
	client = config.Cfg.KubeClient
	podName := config.Cfg.PodName
	podNamespace := config.Cfg.PodNamespace
	dynamicClient = config.GetDynamicClient()
	if (database.DAO{} == dao) {
		dao = database.NewDAO(nil)
	}
	lock := getNewLock(client, lockName, podName, podNamespace)
	runLeaderElection(ctx, lock, syncClusters)
}

// Watches ManagedCluster objects and updates the database with a Cluster node.
func syncClusters(ctx context.Context) {
	klog.Info("Attempting to sync clusters. Begin ClusterWatch routine")

	dynamicFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient,
		time.Duration(config.Cfg.RediscoverRateMS)*time.Millisecond)

	// Filter and Process only search-addon events
	filter := metav1.ListOptions{FieldSelector: "metadata.name=search-collector"}
	filterFunc := dynamicinformer.TweakListOptionsFunc(func(options *metav1.ListOptions) { *options = filter })

	filteredDynamicFactory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dynamicClient,
		time.Duration(config.Cfg.RediscoverRateMS)*time.Millisecond, metav1.NamespaceAll, filterFunc)

	// Create GVR for ManagedCluster and ManagedClusterInfo
	managedClusterGvr, _ := schema.ParseResourceArg(managedClusterGVR)
	managedClusterInfoGvr, _ := schema.ParseResourceArg(managedClusterInfoGVR)
	managedClusterAddonGvr, _ := schema.ParseResourceArg(managedClusterAddonGVR)

	//Create Informers for ManagedCluster and ManagedClusterInfo
	managedClusterInformer := dynamicFactory.ForResource(*managedClusterGvr).Informer()
	managedClusterInfoInformer := dynamicFactory.ForResource(*managedClusterInfoGvr).Informer()
	managedClusterAddonInformer := filteredDynamicFactory.ForResource(*managedClusterAddonGvr).Informer()

	resyncPeriod := time.Duration(config.Cfg.ResyncPeriod) * time.Minute
	// Confirm delete event not missed if indexer OR db goes offline:
	err := deleteStaleClusterResources(ctx, dynamicClient, *managedClusterGvr)
	if err != nil {
		klog.Warning("Error deleting stale clusters resources", err.Error())
	}

	// Create handlers for events
	handlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			klog.V(4).Info("AddFunc for ", obj.(*unstructured.Unstructured).GetKind())
			processClusterUpsert(ctx, obj)
		},
		UpdateFunc: func(prev interface{}, next interface{}) {
			klog.V(4).Info("UpdateFunc for ", next.(*unstructured.Unstructured).GetKind())
			processClusterUpsert(ctx, next)
		},
		DeleteFunc: func(obj interface{}) {
			klog.V(4).Info("DeleteFunc for ", obj.(*unstructured.Unstructured).GetKind())
			processClusterDelete(ctx, obj)
		},
	}

	// Add Handlers to both Informers
	_, managedClusterErr := managedClusterInformer.AddEventHandlerWithResyncPeriod(handlers, resyncPeriod)
	checkError(managedClusterErr, "Error adding eventHandler for managedCluster")
	_, managedClusterInfoErr := managedClusterInfoInformer.AddEventHandlerWithResyncPeriod(handlers, resyncPeriod)
	checkError(managedClusterInfoErr, "Error adding eventHandler for managedClusterInfo")
	_, managedClusterAddonErr := managedClusterAddonInformer.AddEventHandlerWithResyncPeriod(handlers, resyncPeriod)
	checkError(managedClusterAddonErr, "Error adding eventHandler for managedClusterAddon")

	// Periodically check if the ManagedCluster/ManagedClusterInfo resource exists
	go stopAndStartInformer(ctx, "cluster.open-cluster-management.io/v1", managedClusterInformer)
	go stopAndStartInformer(ctx, "internal.open-cluster-management.io/v1beta1", managedClusterInfoInformer)
	go stopAndStartInformer(ctx, "addon.open-cluster-management.io/v1alpha1", managedClusterAddonInformer)

}

func deleteStaleClusterResources(ctx context.Context, dynamicClient dynamic.Interface,
	managedClusterGvr schema.GroupVersionResource) error {
	clusterRemaining, err := findStaleClusterResources(ctx, dynamicClient, managedClusterGvr)
	if err != nil {
		klog.Warning("Error finding stale cluster resources", err.Error())
		return err
	} else if len(clusterRemaining) > 0 {
		for _, cluster := range clusterRemaining {
			dao.DeleteClusterAndResources(ctx, cluster, false)
		}
	}
	return err
}

// Stop and Start informer according to Rediscover Rate
func stopAndStartInformer(ctx context.Context, groupVersion string, informer cache.SharedIndexInformer) {
	var stopper chan struct{}
	informerRunning := false
	wait := time.Duration(1 * time.Millisecond)

	for {
		select {
		case <-ctx.Done():
			klog.Info("Exit informers for clusterwatch.")
			stopper <- struct{}{}
			return
		case <-time.After(wait):
			_, err := config.Cfg.KubeClient.ServerResourcesForGroupVersion(groupVersion)
			// we fail to fetch for some reason other than not found
			if err != nil && !isClusterCrdMissing(err) {
				klog.Errorf("Cannot fetch resource list for %s, error message: %s ", groupVersion, err)
			} else {
				if informerRunning && isClusterCrdMissing(err) {
					klog.Infof("Stopping cluster informer routine because %s resource not found.", groupVersion)
					stopper <- struct{}{}
					informerRunning = false
				} else if !informerRunning && !isClusterCrdMissing(err) {
					klog.Infof("Starting cluster informer routine for cluster watch for %s resource", groupVersion)
					stopper = make(chan struct{})
					informerRunning = true
					go informer.Run(stopper)
				}
			}
			wait = time.Duration(config.Cfg.RediscoverRateMS) * time.Millisecond
		}
	}
}

func processClusterUpsert(ctx context.Context, obj interface{}) {
	// Lock so only one goroutine at a time can access add a cluster.
	// Helps to eliminate duplicate entries.
	mux.Lock()
	defer mux.Unlock()
	j, err := json.Marshal(obj.(*unstructured.Unstructured))
	if err != nil {
		klog.Warning("Error unmarshalling object from Informer in processClusterUpsert.")
	}

	// We update by name, and the name *should be* the same for a given cluster in either object
	// Objects from a given cluster collide and update rather than duplicate insert
	// Unmarshall either ManagedCluster or ManagedClusterInfo
	// check which object we are using

	var resource model.Resource
	switch obj.(*unstructured.Unstructured).GetKind() {
	case "ManagedCluster":
		managedCluster := clusterv1.ManagedCluster{}
		err = json.Unmarshal(j, &managedCluster)
		if err != nil {
			klog.Warning("Failed to Unmarshal ManagedCluster", err)
		}
		resource = transformManagedCluster(&managedCluster)
	case "ManagedClusterInfo":
		managedClusterInfo := clusterv1beta1.ManagedClusterInfo{}
		err = json.Unmarshal(j, &managedClusterInfo)
		if err != nil {
			klog.Warning("Failed to Unmarshal ManagedclusterInfo", err)
		}
		resource = transformManagedClusterInfo(&managedClusterInfo)
	case "ManagedClusterAddOn":
		klog.V(4).Infof("No upsert cluster actions for kind: %s", obj.(*unstructured.Unstructured).GetKind())
		return
	default:
		klog.Warning("ClusterWatch received unknown kind.", obj.(*unstructured.Unstructured).GetKind())
		return
	}

	// Upsert (attempt insert, update on failure)
	dao.UpsertCluster(ctx, resource)

	// A cluster can be offline due to resource shortage, network outage or other reasons. We are not deleting
	// the cluster or resources if a cluster is offline to avoid unnecessary deletes and re-inserts in the database.
	// We need to add a Message in the UI to show a list of clusters that are offline and warn users
	// that the data might be stale
	/*if resource.Properties["status"] == "offline" {
		klog.Infof("Cluster %s is offline, removing cluster resources from datastore.", cluster.GetName())
		delClusterResources(cluster)
	}*/

}

func isClusterCrdMissing(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "could not find the requested resource")
}
func addAdditionalProperties(props map[string]interface{}) map[string]interface{} {
	clusterUid := string("cluster__" + props["name"].(string))
	data, ok := database.ReadClustersCache(clusterUid)
	if ok {
		existingProps, _ := data.(map[string]interface{})
		for key, val := range existingProps {
			_, present := props[key]
			if !present {
				props[key] = val
			}
		}
	}

	// Map apigroup and kind to sync with ManagedClusterInfo for RBAC
	props["apigroup"] = managedClusterInfoApiGrp
	props["kind_plural"] = "managedclusterinfos"
	props["_hubClusterResource"] = true
	return props
}

// Transform ManagedClusterInfo object into Resource suitable for insert into database
func transformManagedClusterInfo(managedClusterInfo *clusterv1beta1.ManagedClusterInfo) model.Resource {
	// https://github.com/stolostron/multicloud-operators-foundation/
	//    blob/main/pkg/apis/internal.open-cluster-management.io/v1beta1/clusterinfo_types.go

	props := make(map[string]interface{})

	// Get properties from ManagedClusterInfo
	props["consoleURL"] = managedClusterInfo.Status.ConsoleURL
	props["nodes"] = int64(len(managedClusterInfo.Status.NodeList))
	props["kind"] = "Cluster"
	props["name"] = managedClusterInfo.GetName()
	props["apigroup"] = managedClusterInfoApiGrp // Maps rbac to ManagedClusterInfo
	props = addAdditionalProperties(props)
	// Create the resource
	resource := model.Resource{
		Kind:           "Cluster",
		UID:            string("cluster__" + managedClusterInfo.GetName()),
		Properties:     props,
		ResourceString: "managedclusterinfos", // Maps rbac to ManagedClusterInfo.
	}
	return resource
}

// Transform ManagedCluster object into Resource suitable for insert into database
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

			// Extract the enabled addons from labels
			props["addon"] = getEnabledAddons(labelMap) // maps to the enabled addons on the cluster

		}
	}

	props["kind"] = "Cluster"
	props["name"] = managedCluster.GetName()     // must match ManagedClusterInfo
	props["apigroup"] = managedClusterInfoApiGrp // maps rbac to ManagedClusterInfo
	props["created"] = managedCluster.GetCreationTimestamp().UTC().Format(time.RFC3339)

	cpuCapacity := managedCluster.Status.Capacity["cpu"]
	props["cpu"], _ = cpuCapacity.AsInt64()
	memCapacity := managedCluster.Status.Capacity["memory"]
	props["memory"] = memCapacity.String()
	props["kubernetesVersion"] = managedCluster.Status.Version.Kubernetes

	for _, condition := range managedCluster.Status.Conditions {
		props[condition.Type] = string(condition.Status)
	}
	props = addAdditionalProperties(props)
	resource := model.Resource{
		Kind:           "Cluster",
		UID:            string("cluster__" + managedCluster.GetName()),
		Properties:     props,
		ResourceString: "managedclusterinfos", // Maps rbac to ManagedClusterInfo
	}
	return resource
}

// Deletes a cluster resource and all resources from the cluster.
func processClusterDelete(ctx context.Context, obj interface{}) {
	klog.V(4).Info("Processing Cluster Delete.")
	clusterName := obj.(*unstructured.Unstructured).GetName()
	var deleteClusterNode bool
	kind := obj.(*unstructured.Unstructured).GetKind()
	name := obj.(*unstructured.Unstructured).GetName()
	switch kind {
	case "ManagedCluster":
		// When ManagedCluster (MC) is deleted, delete the resources and edges and cluster node for that cluster from db
		// ManagedClusterInfo (namespace scoped) will be deleted when the MC (cluster scoped) is being deleted.
		// So, we are tracking deletes of MC only to avoid duplication.
		deleteClusterNode = true
		klog.V(3).Infof("Received delete for %s. Deleting Cluster resource %s and all resources from the DB", kind,
			clusterName)

	case "ManagedClusterAddOn":
		clusterName = obj.(*unstructured.Unstructured).GetNamespace() // Namespace reflects the name of the cluster
		// When ManagedClusterAddOn (MCA) is deleted, search is disabled in the cluster. So, we delete the resources
		// and edges for that cluster from db. But the cluster node is kept until MC is deleted.
		deleteClusterNode = false
		klog.V(3).Infof("Received delete for %s %s. Deleting Cluster resources and edges for cluster %s from the DB",
			name, kind, clusterName)

	case "ManagedClusterInfo":
		klog.V(4).Infof("No delete cluster actions for kind: %s", kind)
		return

	default:
		klog.Warningf("No delete cluster actions for kind: %s", kind)
		return
	}
	dao.DeleteClusterAndResources(ctx, clusterName, deleteClusterNode)

}

// finds lingering data in database from deleted/detached clusters or clusters with search-collector-addon disabled:
func findStaleClusterResources(ctx context.Context, dynamicClient dynamic.Interface,
	gvr schema.GroupVersionResource) ([]string, error) {
	var needToDelete []string
	managedClustersFromClient := make(map[string]struct{})

	// get all managed clusters from kube client:
	resourceObj, err := dynamicClient.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Warning("Error resolving ManagedClusters with dynamic client", err.Error())
		return nil, err
	}
	for _, item := range resourceObj.Items {
		// Here we want all managed clusters that have the search-collector addon available
		if item.GetLabels()["local-cluster"] != "true" && //note: need better method instead of using name local-cluster.
			item.GetLabels()["feature.open-cluster-management.io/addon-search-collector"] == "available" {
			managedClustersFromClient[item.GetName()] = struct{}{}
		}
	}
	klog.V(3).Infof("Managed Clusters reported from kube client: %+v", managedClustersFromClient)

	// get all managed clusters from db:
	managedClustersFromDB, err := dao.GetManagedClusters(ctx)
	if err != nil {
		klog.Errorf("Error getting managed clusters names from database. %s", err)
		return nil, err
	}
	klog.V(3).Infof("Managed Clusters reported from database: %+v", managedClustersFromDB)

	for _, dmCluster := range managedClustersFromDB {
		if _, exist := managedClustersFromClient[dmCluster]; !exist {
			// At this point the cluster exists in DB, but not in the list from client.
			needToDelete = append(needToDelete, dmCluster)
			klog.V(1).Infof("Found Managed Cluster data in database that should be deleted! Cluster found: %s", dmCluster)
		}
	}

	return needToDelete, nil

}

// find the enabled addons from the cluster labels
func getEnabledAddons(labelMap map[string]interface{}) map[string]interface{} {

	enabledAddons := make(map[string]interface{}, len(allAddons))
	for _, addon := range allAddons {
		addonLabelName := "feature.open-cluster-management.io/addon-" + addon
		if _, ok := labelMap[addonLabelName]; ok {
			enabledAddons[addon] = "true"
		} else {
			enabledAddons[addon] = "false"
		}
	}
	return enabledAddons
}

func checkError(err error, logMessage string) {
	if err != nil {
		klog.Error(logMessage, " ", err)
	}
}
