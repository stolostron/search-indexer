// Copyright Contributors to the Open Cluster Management project

package server

import (
	"encoding/json"
	"net/http"
	"os"
	"runtime/pprof"
	"time"

	"github.com/stolostron/search-indexer/pkg/metrics"

	"github.com/gorilla/mux"
	"github.com/stolostron/search-indexer/pkg/config"
	"github.com/stolostron/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

func (s *ServerConfig) SyncResources(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	clusterName := params["id"]

	// Decode SyncEvent from request body.
	var syncEvent model.SyncEvent
	err := json.NewDecoder(r.Body).Decode(&syncEvent)
	if err != nil {
		klog.Errorf("Error decoding request body from cluster [%s]. Error: %+v\n", clusterName, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	resourceTotal := len(syncEvent.AddResources) + len(syncEvent.UpdateResources) + len(syncEvent.DeleteResources)
	metrics.RequestSize.Observe(float64(resourceTotal))

	// Initialize SyncResponse object.
	syncResponse := &model.SyncResponse{
		Version:          config.COMPONENT_VERSION,
		RequestId:        syncEvent.RequestId,
		AddErrors:        make([]model.SyncError, 0),
		UpdateErrors:     make([]model.SyncError, 0),
		DeleteErrors:     make([]model.SyncError, 0),
		AddEdgeErrors:    make([]model.SyncError, 0),
		DeleteEdgeErrors: make([]model.SyncError, 0),
	}

	// The collector sends 2 types of requests:
	// 1. ReSync [ClearAll=true]  - It has the complete current state. It must overwrite any previous state.
	// 2. Sync   [ClearAll=false] - This is the delta changes from the previous state.
	if syncEvent.ClearAll {
		err = s.Dao.ResyncData(r.Context(), syncEvent, clusterName, syncResponse)
	} else {
		err = s.Dao.SyncData(r.Context(), syncEvent, clusterName, syncResponse)
	}
	if err != nil {
		klog.Warningf("Responding with error to request from %12s. RequestId: %s  Error: %s",
			clusterName, syncEvent.RequestId, err)
		http.Error(w, "Server error while processing the request.", http.StatusInternalServerError)
		return
	}

	// Get the total cluster resources for validation by the collector.
	totalResources, totalEdges, validateErr := s.Dao.ClusterTotals(r.Context(), clusterName)
	if validateErr != nil {
		klog.Warningf("Responding with error to request from %12s. RequestId: %s  Error: %s",
			clusterName, syncEvent.RequestId, validateErr)
		http.Error(w, "Server error while processing the request.", http.StatusInternalServerError)
		return
	}
	syncResponse.TotalResources = totalResources
	syncResponse.TotalEdges = totalEdges

	// Send Response
	w.WriteHeader(http.StatusOK)
	encodeError := json.NewEncoder(w).Encode(syncResponse)
	if encodeError != nil {
		klog.Error("Error responding to SyncEvent:", encodeError, syncResponse)
		w.WriteHeader(http.StatusInternalServerError)
	}

	// var m runtime.MemStats
	// runtime.ReadMemStats(&m)
	// // For info on each, see: https://golang.org/pkg/runtime/#MemStats
	// fmt.Printf("Alloc = %v MiB", bToMb(m.Alloc))
	// fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
	// fmt.Printf("\tSys = %v MiB", bToMb(m.Sys))
	// fmt.Printf("\tNumGC = %v\n", m.NumGC)

	// Log request.
	klog.V(5).Infof("Request from [%12s] took [%v] clearAll [%t] addTotal [%d]",
		clusterName, time.Since(start), syncEvent.ClearAll, len(syncEvent.AddResources))
	// klog.V(5).Infof("Response for [%s]: %+v", clusterName, syncResponse)

	f, err := os.Create("mem.prof")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := pprof.WriteHeapProfile(f); err != nil {
		panic(err)
	}
}

// func bToMb(b uint64) uint64 {
// 	return b / 1024 / 1024
// }
