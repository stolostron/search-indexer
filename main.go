// Copyright Contributors to the Open Cluster Management project

package main

import (
	"flag"

	"github.com/open-cluster-management/search-indexer/pkg/config"
	"github.com/open-cluster-management/search-indexer/pkg/server"
	"k8s.io/klog/v2"
)

func main() {
	// Initialize the logger.
	klog.InitFlags(nil)
	flag.Parse()
	defer klog.Flush()
	klog.Info("Starting search-indexer.")

	// Read the config.
	config := config.New()
	config.PrintConfig()

	// Start the server.
	server.StartAndListen()
}
