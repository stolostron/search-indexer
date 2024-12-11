// Copyright Contributors to the Open Cluster Management project

package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/pprof"

	//"time"

	//"github.com/stolostron/search-indexer/pkg/metrics"

	"github.com/gorilla/mux"
	"github.com/stolostron/search-indexer/pkg/config"
	"github.com/stolostron/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

func (s *ServerConfig) SyncResources(w http.ResponseWriter, r *http.Request) {
	//start := time.Now()
	w.Header().Set("Content-Type", "application/json")
	params := mux.Vars(r)
	clusterName := params["id"]

	syncEventMin := model.SyncEventMin{ClearAll: true}
	resourceChannels := map[string]chan interface{}{
		"addResources":    make(chan interface{}, 100),
		"updateResources": make(chan interface{}),
		"deleteResources": make(chan interface{}),
		"addEdges":        make(chan interface{}, 100),
		"deleteEdges":     make(chan interface{}),
		"clearAll":        make(chan interface{}),
		"requestId":       make(chan interface{}),
	}
	// Decode SyncEvent from request body.
	err := decodeSyncEvent(r.Body, clusterName, resourceChannels)
	if err != nil {
		klog.Error(err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	//resourceTotal := len(syncEvent.AddResources) + len(syncEvent.UpdateResources) + len(syncEvent.DeleteResources)
	//metrics.RequestSize.Observe(float64(resourceTotal)) // TODO: this

	// Initialize SyncResponse object.
	syncResponse := &model.SyncResponse{
		Version:          config.COMPONENT_VERSION,
		RequestId:        0, // TODO: get this later
		AddErrors:        make([]model.SyncError, 0),
		UpdateErrors:     make([]model.SyncError, 0),
		DeleteErrors:     make([]model.SyncError, 0),
		AddEdgeErrors:    make([]model.SyncError, 0),
		DeleteEdgeErrors: make([]model.SyncError, 0),
	}

	// The collector sends 2 types of requests:
	// 1. ReSync [ClearAll=true]  - It has the complete current state. It must overwrite any previous state.
	// 2. Sync   [ClearAll=false] - This is the delta changes from the previous state.
	if syncEventMin.ClearAll {
		err = s.Dao.ResyncData(r.Context(), resourceChannels, clusterName, syncResponse)
	} else {
		err = s.Dao.SyncData(r.Context(), resourceChannels, clusterName, syncResponse)
	}
	if err != nil {
		klog.Warningf("Responding with error to request from %12s. RequestId: %s  Error: %s",
			clusterName, syncEventMin.RequestId, err)
		http.Error(w, "Server error while processing the request.", http.StatusInternalServerError)
		return
	}

	// Get the total cluster resources for validation by the collector.
	totalResources, totalEdges, validateErr := s.Dao.ClusterTotals(r.Context(), clusterName)
	if validateErr != nil {
		klog.Warningf("Responding with error to request from %12s. RequestId: %s  Error: %s",
			clusterName, syncEventMin.RequestId, validateErr)
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
	//klog.V(5).Infof("Request from [%12s] took [%v] clearAll [%t] addTotal [%d]",
	//	clusterName, time.Since(start), syncEventMin.ClearAll, len(syncEvent.AddResources)) // TODO: get len
	// klog.V(5).Infof("Response for [%s]: %+v", clusterName, syncResponse)
	f, err := os.Create("mem.prof")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := pprof.WriteHeapProfile(f); err != nil {
		fmt.Println("err: err")
	}
	fmt.Println("DONE")
}

func closeChannels(channels map[string]chan interface{}) {
	for name, ch := range channels {
		fmt.Println("closing ch:  ", name)
		close(ch)
	}
}

func decodeSyncEvent(body io.ReadCloser, clusterName string, resourceChannels map[string]chan interface{}) error {
	dec := json.NewDecoder(body)
	//var syncEvent model.SyncEvent
	//if err := dec.Decode(&syncEvent); err != nil {
	//	return err
	//}
	//
	//go func() {
	//	defer close(resourceChannels["addResources"])
	//	for _, resource := range syncEvent.AddResources {
	//		resourceChannels["addResources"] <- resource
	//	}
	//}()
	//
	//go func() {
	//	defer close(resourceChannels["addEdges"])
	//	for _, edge := range syncEvent.AddEdges {
	//		resourceChannels["addEdges"] <- edge
	//	}
	//}()

	go func() {
		defer closeChannels(resourceChannels)
		if _, err := dec.Token(); err != nil {
			//return syncEvent, fmt.Errorf("error decoding SyncEvent token from cluster \"%s\": %v", clusterName, err)
		}
		for dec.More() {
			field, err := dec.Token()
			if err != nil {
				//return fmt.Errorf("error decoding field name from SyncEvent request body from cluster \"%s\": %v", clusterName, err)
			}

			switch field {
			case "clearAll":
				fmt.Println("in clearAll")
				var clearAll bool
				if err := dec.Decode(&clearAll); err != nil {
					//return syncEvent, fmt.Errorf("failed to decode \"%s\" as clearAll: %v", field, err)
				}
				//resourceChannels["clearAll"] <- clearAll
			case "requestId":
				fmt.Println("in requestId")
				var requestId int
				if err := dec.Decode(&requestId); err != nil {
					//return syncEvent, fmt.Errorf("failed to decode \"%s\" as requestId: %v", field, err)
				}
				//resourceChannels["requestId"] <- requestId
			case "addResources":
				fmt.Println("in add resources decode array loop")
				if err := decodeArray(dec, resourceChannels["addResources"], clusterName, field); err != nil {
					//return syncEvent, err
				}
			case "updateResources":
				if err := decodeArray(dec, resourceChannels["updateResources"], clusterName, field); err != nil {
					//return syncEvent, err
				}
			case "deleteResources":
				if err := decodeArray(dec, resourceChannels["deleteResources"], clusterName, field); err != nil {
					//return syncEvent, err
				}
			case "addEdges":
				fmt.Println("in add edges")
				if err := decodeArray(dec, resourceChannels["addEdges"], clusterName, field); err != nil {
					//return syncEvent, err
				}
			case "deleteEdges":
				if err := decodeArray(dec, resourceChannels["deleteEdges"], clusterName, field); err != nil {
					//return syncEvent, err
				}
			}
		}
		return
	}()

	return nil
}

func decodeArray(dec *json.Decoder, target chan interface{}, clusterName string, field interface{}) error {
	// consume opening token
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("error reading start of array for \"%s\" from cluster \"%s\": %v", field, clusterName, err)
	}
	switch field {
	case "addResources":
		for dec.More() {
			var addResource model.Resource
			if err := dec.Decode(&addResource); err != nil {
				return fmt.Errorf("failed to decode \"%s\" as resource from cluster \"%s\": %v", field, clusterName, err)
			}
			target <- addResource
		}
	case "updateResources":
		for dec.More() {
			var updateResource model.Resource
			if err := dec.Decode(&updateResource); err != nil {
				return fmt.Errorf("failed to decode \"%s\" as resource from cluster \"%s\": %v", field, clusterName, err)
			}
			//target <- updateResource
		}
	case "deleteResources":
		for dec.More() {
			var deleteResource model.DeleteResourceEvent
			if err := dec.Decode(&deleteResource); err != nil {
				return fmt.Errorf("failed to decode \"%s\" as resource from cluster \"%s\": %v", field, clusterName, err)
			}
			//target <- deleteResource
		}
	case "addEdges":
		for dec.More() {
			var addEdge model.Edge
			if err := dec.Decode(&addEdge); err != nil {
				return fmt.Errorf("failed to decode \"%s\" as resource from cluster \"%s\": %v", field, clusterName, err)
			}
			target <- addEdge
		}
	case "deleteEdges":
		for dec.More() {
			var deleteEdge model.Edge
			if err := dec.Decode(&deleteEdge); err != nil {
				return fmt.Errorf("failed to decode \"%s\" as resource from cluster \"%s\": %v", field, clusterName, err)
			}
			//target <- deleteEdge
		}
	default:
		return fmt.Errorf("unsupported field array type \"%s\" to decode for SyncEvent from cluster \"%s\"", field, clusterName)
	}
	// consume closing token
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("error reading end of array for \"%s\" from cluster \"%s\": %v", field, clusterName, err)
	}

	return nil
}
