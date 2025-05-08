// Copyright Contributors to the Open Cluster Management project

package server

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	klog "k8s.io/klog/v2"

	"github.com/gorilla/mux"
	"github.com/stolostron/search-indexer/pkg/config"
)

var requestTracker = map[string]time.Time{}
var requestTrackerLock = sync.RWMutex{}

// Checks if we are able to accept the incoming request.
func requestLimiterMiddleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		clusterName := params["id"]

		requestTrackerLock.RLock()
		requestCount := len(requestTracker)
		klog.V(6).Info("Checking if we can process incoming request. Current requests: ", requestCount)
		timeReqReceived, foundClusterProcessing := requestTracker[clusterName]
		requestTrackerLock.RUnlock()

		if foundClusterProcessing {
			klog.Warningf("Rejecting request from %s because there's a previous request processing. Duration: %s",
				clusterName, time.Since(timeReqReceived))
			http.Error(w, "A previous request from this cluster is processing, retry later.", http.StatusTooManyRequests)
			return
		}

		hubClusterRequestHeader := r.Header.Get("X-Hub-Cluster-Reqeust")
		hubClusterRequest, hubClusterRequestErr := strconv.ParseBool(hubClusterRequestHeader)
		if hubClusterRequestErr != nil {
			klog.V(1).Infof("Invalid X-Hub-Cluster-Request header value [%s] from cluster[%s]: %v", hubClusterRequestHeader, clusterName, hubClusterRequestErr)
			hubClusterRequest = false
		}

		if requestCount >= config.Cfg.RequestLimit && !hubClusterRequest {
			klog.Warningf("Too many pending requests (%d). Rejecting sync from %s", requestCount, clusterName)
			http.Error(w, "Indexer has too many pending requests, retry later.", http.StatusTooManyRequests)
			return
		}

		requestTrackerLock.Lock()
		requestTracker[clusterName] = time.Now()
		requestTrackerLock.Unlock()

		defer func() { // Using defer to guarantee this gets executed if there's an error processing the request.
			requestTrackerLock.Lock()
			delete(requestTracker, clusterName)
			requestTrackerLock.Unlock()
		}()

		next.ServeHTTP(w, r)
	})
}
