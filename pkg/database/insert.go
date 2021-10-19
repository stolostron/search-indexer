package database

import (
	"k8s.io/klog/v2"
)

func Insert(resources []Resource, clusterName string) (map[string]error, error) {

	klog.Infof("TODO: Cluster: %s.\t Insert %d resources.\n", clusterName, len(resources))

	return nil, nil
}
