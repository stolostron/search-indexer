// Copyright Contributors to the Open Cluster Management project

package server

import (
	"net/http"
	"strings"
	"sync"
	"time"

	klog "k8s.io/klog/v2"

	"github.com/gorilla/mux"
	"github.com/stolostron/search-indexer/pkg/config"
)

var syncRequestTracker = map[string]time.Time{}
var validationRequestTracker = map[string]time.Time{}
var requestTrackerLock = sync.Mutex{}

// Checks if we are able to accept the incoming request.
func requestLimiterMiddleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		params := mux.Vars(r)
		clusterName := params["id"]
		isValidation := r.Method == http.MethodGet && strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), "/validation")

		requestTrackerLock.Lock()
		timeReqReceived, foundClusterProcessing := syncRequestTracker[clusterName]
		if !foundClusterProcessing {
			timeReqReceived, foundClusterProcessing = validationRequestTracker[clusterName]
		}

		if foundClusterProcessing {
			requestTrackerLock.Unlock()
			klog.Warningf("Rejecting request from %s because there's a previous request processing. Duration: %s",
				clusterName, time.Since(timeReqReceived))
			http.Error(w, "A previous request from this cluster is processing, retry later.", http.StatusTooManyRequests)
			return
		}

		syncRequestCount := len(syncRequestTracker)
		validationRequestCount := len(validationRequestTracker)
		klog.V(6).Infof("Checking request capacity. Sync requests: %d / %d, validation requests: %d / %d",
			syncRequestCount, config.Cfg.RequestLimit, validationRequestCount, config.Cfg.ValidationRequestLimit)

		// Give higher priority to requests from the collector in the hub cluster.
		//   request host is stored as plain text service when originating from hub cluster
		//   request host uses internal IP when coming from proxy service, i.e. managed clusters
		hubClusterReq := r.Host == "search-indexer.open-cluster-management.svc:3010"
		requestLimitReached := (!isValidation && syncRequestCount >= config.Cfg.RequestLimit) ||
			(isValidation && validationRequestCount >= config.Cfg.ValidationRequestLimit)
		if requestLimitReached && !hubClusterReq {
			requestTrackerLock.Unlock()
			klog.Warningf("Request limit reached. Rejecting request from %s (validation: %t). "+
				"Sync requests: %d / %d, validation requests: %d / %d",
				clusterName, isValidation, syncRequestCount, config.Cfg.RequestLimit,
				validationRequestCount, config.Cfg.ValidationRequestLimit)
			message := "Indexer has too many pending requests, retry later."
			if isValidation {
				message = "Indexer has too many pending validation requests, retry later."
			}
			http.Error(w, message, http.StatusTooManyRequests)
			return
		}

		now := time.Now()
		if isValidation {
			validationRequestTracker[clusterName] = now
		} else {
			syncRequestTracker[clusterName] = now
		}
		requestTrackerLock.Unlock()

		defer func() { // Using defer to guarantee this gets executed if there's an error processing the request.
			requestTrackerLock.Lock()
			if isValidation {
				delete(validationRequestTracker, clusterName)
			} else {
				delete(syncRequestTracker, clusterName)
			}
			requestTrackerLock.Unlock()
		}()

		next.ServeHTTP(w, r)
	})
}
