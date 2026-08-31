// Copyright Contributors to the Open Cluster Management project

package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
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

	overwriteStateHeader := r.Header.Get("X-Overwrite-State")
	overwriteState, overwriteStateErr := strconv.ParseBool(overwriteStateHeader)
	if overwriteStateErr != nil {
		klog.V(1).Infof("Invalid X-Overwrite-State header value [%s] from cluster[%s]: %v", overwriteStateHeader, clusterName, overwriteStateErr)
		overwriteState = false
	}

	// Initialize SyncResponse object.
	syncResponse := &model.SyncResponse{
		Version:          config.COMPONENT_VERSION,
		AddErrors:        make([]model.SyncError, 0),
		UpdateErrors:     make([]model.SyncError, 0),
		DeleteErrors:     make([]model.SyncError, 0),
		AddEdgeErrors:    make([]model.SyncError, 0),
		DeleteEdgeErrors: make([]model.SyncError, 0),
	}

	// The collector sends 2 types of requests with the header:
	// 1. ReSync [X-Overwrite-State=true]  - Complete current state; decoded as a streaming JSON reader
	//    to avoid buffering the full (potentially large) body in memory.
	// 2. Sync   [X-Overwrite-State=false] - Delta changes; small enough to decode fully.
	var err error
	var syncEvent model.SyncEvent
	if overwriteState {
		err = s.Dao.ResyncData(r.Context(), clusterName, syncResponse, r.Body)
	} else {
		// Delta sync payloads are significantly smaller, so buffering is acceptable here.
		bodyBytes, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			klog.Errorf("Error reading request body from cluster [%s]. Error: %+v\n", clusterName, readErr)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		err = json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&syncEvent)
		if err != nil {
			klog.Errorf("Error decoding request body from cluster [%s]. Error: %+v\n", clusterName, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		metrics.RequestSize.Observe(float64(len(syncEvent.AddResources) + len(syncEvent.UpdateResources) + len(syncEvent.DeleteResources)))
		err = s.Dao.SyncData(r.Context(), syncEvent, clusterName, syncResponse)
	}
	if err != nil {
		klog.Warningf("Responding with error to request from %12s. Error: %s",
			clusterName, err)
		http.Error(w, "Server error while processing the request.", http.StatusInternalServerError)
		return
	}

	// Get the total cluster resources for validation by the collector.
	totalResources, totalEdges, validateErr := s.Dao.ClusterTotals(r.Context(), clusterName)
	if validateErr != nil {
		klog.Warningf("Responding with error to request from %12s. Error: %s",
			clusterName, validateErr)
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

	// Log request.
	klog.V(5).Infof("Request from [%12s] took [%v] overwriteState [%t] addTotal [%d]",
		clusterName, time.Since(start), overwriteState, len(syncEvent.AddResources))
	// klog.V(5).Infof("Response for [%s]: %+v", clusterName, syncResponse)
}
