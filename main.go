// Copyright Contributors to the Open Cluster Management project

package main

import (
	"flag"

	"github.com/open-cluster-management/search-indexer/pkg/config"
	"github.com/open-cluster-management/search-indexer/pkg/database"
	"github.com/open-cluster-management/search-indexer/pkg/server"
	"k8s.io/klog/v2"
)

func main() {
	// Initialize the logger.
	klog.InitFlags(nil)
	flag.Parse()
	defer klog.Flush()
	klog.Info("Starting search-indexer.")

	// Read the config from the environment.
	config.Cfg.PrintConfig()

	// Validate required configuration to proceed.
	configError := config.Cfg.Validate()
	if configError != nil {
		klog.Fatal(configError)
	}

	// Initialize the database
	dao := database.NewDAO(nil)
	dao.InitializeTables()

	// Start the server.
	srv := &server.ServerConfig{
		Dao: &dao,
	}
	srv.StartAndListen()
}
