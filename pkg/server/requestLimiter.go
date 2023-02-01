// Copyright Contributors to the Open Cluster Management project

package server

import (
	"net/http"
	"sync"
	"time"

	klog "k8s.io/klog/v2"

	"github.com/gorilla/mux"
	"github.com/stolostron/search-indexer/pkg/config"
)

var pendingRequests = map[string]time.Time{}
var pendingLock = sync.RWMutex{}

// Checks if we are able to accept the incoming request.
func requestLimiterMiddleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		clusterName := params["id"]

		klog.V(6).Info("Checking is we can process incoming request. Existing pending requests: ", len(pendingRequests))

		if t, exists := pendingRequests[clusterName]; exists {
			klog.Warningf("Rejecting request from %s because there's a previous request processing. Duration: %s",
				clusterName, time.Since(t))
			http.Error(w, "A previous request from this cluster is processing, retry later.", http.StatusTooManyRequests)
			return
		}

		if len(pendingRequests) >= config.Cfg.RequestLimit && clusterName != "local-cluster" {
			klog.Warningf("Too many pending requests (%d). Rejecting sync from %s", len(pendingRequests), clusterName)
			http.Error(w, "Indexer has too many pending requests, retry later.", http.StatusTooManyRequests)
			return
		}

		pendingLock.Lock()
		pendingRequests[clusterName] = time.Now()
		pendingLock.Unlock()

		defer func() {
			pendingLock.Lock()
			delete(pendingRequests, clusterName)
			pendingLock.Unlock()
		}()

		next.ServeHTTP(w, r)
	})
}
