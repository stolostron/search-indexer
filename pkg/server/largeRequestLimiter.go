// Copyright Contributors to the Open Cluster Management project

package server

import (
	"k8s.io/klog/v2"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/stolostron/search-indexer/pkg/config"
)

var largeRequestCountTracker int
var largeRequestCountTrackerLock = sync.RWMutex{}

// Checks if we are able to accept the incoming request based upon request size
func largeRequestLimiterMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		clusterName := params["id"]
		if r.ContentLength > int64(config.Cfg.LargeRequestLimit) {
			largeRequestCountTrackerLock.RLock()
			largeRequestCount := largeRequestCountTracker
			largeRequestCountTrackerLock.RUnlock()

			if largeRequestCount >= config.Cfg.LargeRequestLimit {
				klog.Warningf("Rejecting large request from %s because there's too many large requests processing. Request size: %dMB",
					clusterName, r.ContentLength/1024/1024)
				http.Error(w, "Too many large requests currently processing, retry later.", http.StatusTooManyRequests)
				return
			}

			largeRequestCountTrackerLock.Lock()
			largeRequestCountTracker++
			largeRequestCountTrackerLock.Unlock()

			defer func() {
				largeRequestCountTrackerLock.Lock()
				largeRequestCountTracker--
				largeRequestCountTrackerLock.Unlock()
			}()
		}

		next.ServeHTTP(w, r)
	})
}
