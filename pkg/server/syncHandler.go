package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/open-cluster-management/search-indexer/pkg/config"
	"github.com/open-cluster-management/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

func SyncResources(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	clusterName := params["id"]
	klog.V(2).Infof("Processing request from cluster [%s]", clusterName)

	var syncEvent model.SyncEvent
	err := json.NewDecoder(r.Body).Decode(&syncEvent)
	if err != nil {
		klog.Error("Error decoding body of syncEvent: ", err)
		// respond(http.StatusBadRequest)
		return
	}

	// // The collector sends ClearAll if it's the first time sending or if something goes wrong and it detects
	// // that it needs a full resync with the current state.
	// if syncEvent.ClearAll {
	// 	db.ResyncData(syncEvent, clusterName)
	// } else {
	// 	db.SaveData(syncEvent, clusterName)
	// }
	// // TODO: Process the sync event.
	// db.Insert(syncEvent.AddResources, clusterName)

	response := &model.SyncResponse{Version: config.AGGREGATOR_API_VERSION}
	w.WriteHeader(http.StatusOK)
	encodeError := json.NewEncoder(w).Encode(response)
	if encodeError != nil {
		klog.Error("Error responding to SyncEvent:", encodeError, response)
	}

	klog.V(5).Infof("Request from [%s] took %v", clusterName, time.Since(start))
	// Record metrics.
	OpsProcessed.WithLabelValues(clusterName, r.RequestURI).Inc()
	HttpDuration.WithLabelValues(clusterName, r.RequestURI).Observe(float64(time.Since(start).Milliseconds()))
}
