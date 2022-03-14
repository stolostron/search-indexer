// Copyright Contributors to the Open Cluster Management project

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/stolostron/search-indexer/pkg/clustersync"
	"github.com/stolostron/search-indexer/pkg/config"
	"github.com/stolostron/search-indexer/pkg/database"
	"github.com/stolostron/search-indexer/pkg/server"
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

	ctx, cancel := context.WithCancel(context.Background())

	// Initialize the database
	dao := database.NewDAO(nil)
	dao.InitializeTables()

	// Start cluster sync.
	go clustersync.ElectLeaderAndStart(ctx)

	// Start the server.
	srv := &server.ServerConfig{
		Dao: &dao,
	}
	go srv.StartAndListen(ctx)

	// Listen and wait for termination signal.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigs // Wait for termination signal.
	klog.Warning("Received termination signal: ", sig)
	cancel()

	time.Sleep(5 * time.Second) // TODO: How can I wait synchronously?
	klog.Info("Exiting search-indexer.")
}
