package database

import (
	"sync"
)

var existingClustersCache map[string]interface{} // a map to hold Current clusters and properties
var mux sync.RWMutex

func ReadClustersCache(uid string) (interface{}, bool) {
	mux.RLock()
	defer mux.RUnlock()
	if existingClustersCache == nil {
		existingClustersCache = make(map[string]interface{})
	}
	data, ok := existingClustersCache[uid]
	return data, ok
}

func UpdateClustersCache(uid string, data interface{}) {
	mux.Lock()
	defer mux.Unlock()
	if existingClustersCache == nil {
		existingClustersCache = make(map[string]interface{})
	}
	if uid != "" {
		existingClustersCache[uid] = data
	}
}

func DeleteClustersCache(uid string) {
	mux.Lock()
	defer mux.Unlock()
	delete(existingClustersCache, uid)
}
