// Copyright Contributors to the Open Cluster Management project

package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"sync"
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
	dao.InitializeTables(ctx)

	// WaitGroup to ensure graceful shutdown of goroutines
	var wg sync.WaitGroup

	// Start cluster sync.
	wg.Add(1)
	go func() {
		defer wg.Done()
		clustersync.ElectLeaderAndStart(ctx)
	}()

	// Start the server.
	srv := &server.ServerConfig{
		Dao: &dao,
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		srv.StartAndListen(ctx)
	}()

	// Listen and wait for termination signal.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigs // Waits for termination signal.
	klog.Warningf("Received termination signal %s. Exiting server and clustersync routines. ", sig)
	exitRoutines()

	// Wait for all goroutines to finish gracefully
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		klog.Info("All goroutines stopped gracefully.")
	case <-time.After(10 * time.Second):
		klog.Warning("Timeout waiting for goroutines to stop.")
	}
	klog.Warning("Exiting search-indexer.")
}
