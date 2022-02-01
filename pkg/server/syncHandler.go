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
	// klog.V(2).Infof("Processing request from cluster [%s]", clusterName)

	var syncEvent model.SyncEvent
	err := json.NewDecoder(r.Body).Decode(&syncEvent)
	if err != nil {
		klog.Error("Error decoding body of syncEvent: ", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

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

	klog.Infof("SyncResponse: %+v", syncResponse)

	w.WriteHeader(http.StatusOK)
	encodeError := json.NewEncoder(w).Encode(syncResponse)
	if encodeError != nil {
		klog.Error("Error responding to SyncEvent:", encodeError, syncResponse)
	}

	klog.V(5).Infof("Request from [%s] took [%v] clearAll [%t] addTotal [%d]", clusterName, time.Since(start), syncEvent.ClearAll, len(syncEvent.AddResources))
	// Record metrics.
	OpsProcessed.WithLabelValues(clusterName, r.RequestURI).Inc()
	HttpDuration.WithLabelValues(clusterName, r.RequestURI).Observe(float64(time.Since(start).Milliseconds()))
}
