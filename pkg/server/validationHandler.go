// Copyright Contributors to the Open Cluster Management project

package server

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/stolostron/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

// ValidateCluster returns the resource and edge totals persisted for a cluster.
func (s *ServerConfig) ValidateCluster(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	clusterName := mux.Vars(r)["id"]

	totalResources, totalEdges, err := s.Dao.ClusterTotals(r.Context(), clusterName)
	if err != nil {
		klog.Warningf("Responding with validation error for cluster %12s. Error: %s", clusterName, err)
		http.Error(w, "Server error while validating cluster data.", http.StatusInternalServerError)
		return
	}

	response := model.ValidationResponse{
		TotalResources: totalResources,
		TotalEdges:     totalEdges,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		klog.Error("Error responding to cluster validation request: ", err)
	}
}
