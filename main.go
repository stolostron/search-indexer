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

	ctx, exitRoutines := context.WithCancel(context.Background())

	// Initialize the database
	dao := database.NewDAO(nil)

	// Start the server.
	srv := &server.ServerConfig{
		Dao: &dao,
	}
	go srv.StartAndListen(ctx)

	dao.InitializeTables(ctx)

	// Start cluster sync.
	go clustersync.ElectLeaderAndStart(ctx)

	// Listen and wait for termination signal.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigs // Waits for termination signal.
	klog.Warningf("Received termination signal %s. Exiting server and clustersync routines. ", sig)
	exitRoutines()

	// We could use a waitgroup to wait for leader election and server to shutdown
	// but it add more complexity so keeping simple for now.
	time.Sleep(5 * time.Second)
	klog.Warning("Exiting search-indexer.")
}
