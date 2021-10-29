package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/open-cluster-management/search-indexer/pkg/config"
	db "github.com/open-cluster-management/search-indexer/pkg/database"
	"github.com/open-cluster-management/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

func SyncResources(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	clusterName := params["id"]
	// klog.V(2).Infof("Processing request from cluster [%s]", clusterName)

	var syncEvent model.SyncEvent
	err := json.NewDecoder(r.Body).Decode(&syncEvent)
	if err != nil {
		klog.Error("Error decoding body of syncEvent: ", err)
		// respond(http.StatusBadRequest)
		return
	}

	// The collector sends 2 types of requests:
	// 1. ReSync [ClearAll=true]  - It has the complete current state. It must overwrite any previous state.
	// 2. Sync   [ClearAll=false] - This is the delta changes from the previous state.
	if syncEvent.ClearAll {
		db.ResyncData(syncEvent, clusterName)
	} else {
		db.SyncData(syncEvent, clusterName)
	}

	response := &model.SyncResponse{Version: config.COMPONENT_VERSION}
	w.WriteHeader(http.StatusOK)
	encodeError := json.NewEncoder(w).Encode(response)
	if encodeError != nil {
		klog.Error("Error responding to SyncEvent:", encodeError, response)
	}

	klog.V(5).Infof("Request from [%s] took [%v] clearAll [%t] addTotal [%d]", clusterName, time.Since(start), syncEvent.ClearAll, len(syncEvent.AddResources))
	// Record metrics.
	OpsProcessed.WithLabelValues(clusterName, r.RequestURI).Inc()
	HttpDuration.WithLabelValues(clusterName, r.RequestURI).Observe(float64(time.Since(start).Milliseconds()))
}
