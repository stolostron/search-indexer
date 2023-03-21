// Copyright Contributors to the Open Cluster Management project

package server

import (
	"encoding/json"
	"net/http"
	"time"

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
		s.Dao.ResyncData(syncEvent, clusterName, syncResponse)
	} else {
		s.Dao.SyncData(syncEvent, clusterName, syncResponse)
	}

	totalResources, totalEdges := s.Dao.ClusterTotals(clusterName)
	syncResponse.TotalResources = totalResources
	syncResponse.TotalEdges = totalEdges

	// Send Response
	w.WriteHeader(http.StatusOK)
	encodeError := json.NewEncoder(w).Encode(syncResponse)
	if encodeError != nil {
		klog.Error("Error responding to SyncEvent:", encodeError, syncResponse)
		w.WriteHeader(http.StatusInternalServerError)
	}

	// Log request and metrics.
	klog.V(5).Infof("Request from [%s] took [%v] clearAll [%t] addTotal [%d]",
		clusterName, time.Since(start), syncEvent.ClearAll, len(syncEvent.AddResources))
	// klog.V(5).Infof("Response for [%s]: %+v", clusterName, syncResponse)
}
