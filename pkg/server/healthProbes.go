// Copyright Contributors to the Open Cluster Management project

package server

import (
	"fmt"
	"net/http"

	"k8s.io/klog/v2"
)

// LivenessProbe is used to check if this service is alive.
func (s *ServerConfig) LivenessProbe(w http.ResponseWriter, r *http.Request) {
	klog.V(7).Info("livenessProbe")
	fmt.Fprint(w, "OK")
}

// ReadinessProbe checks if this service is available.
func (s *ServerConfig) ReadinessProbe(w http.ResponseWriter, r *http.Request) {
	klog.V(7).Info("readinessProbe")
	if !s.Dao.DBInitialized {
		fmt.Fprint(w, "error: Postgres db not initialized")
	}
	fmt.Fprint(w, "OK")
}
