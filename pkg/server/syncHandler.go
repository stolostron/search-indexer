package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/open-cluster-management/search-indexer/pkg/config"
	"k8s.io/klog/v2"
)

func SyncResources(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	clusterName := params["id"]
	klog.V(2).Infof("Processing request from cluster [%s]", clusterName)

	var syncEvent SyncEvent
	err := json.NewDecoder(r.Body).Decode(&syncEvent)
	if err != nil {
		klog.Error("Error decoding body of syncEvent: ", err)
		// respond(http.StatusBadRequest)
		return
	}

	// TODO: Process the sync event.

	response := &SyncResponse{Version: config.AGGREGATOR_API_VERSION}
	w.WriteHeader(http.StatusOK)
	encodeError := json.NewEncoder(w).Encode(response)
	if encodeError != nil {
		klog.Error("Error responding to SyncEvent:", encodeError, response)
	}

	// Record metrics.
	OpsProcessed.WithLabelValues(clusterName, r.RequestURI).Inc()
	HttpDuration.WithLabelValues(clusterName, r.RequestURI).Observe(float64(time.Since(start).Milliseconds()))
}
