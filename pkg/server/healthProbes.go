package server

import (
	"fmt"
	"net/http"

	"k8s.io/klog/v2"
)

// LivenessProbe is used to check if this service is alive.
func LivenessProbe(w http.ResponseWriter, r *http.Request) {
	klog.V(2).Info("livenessProbe")
	fmt.Fprint(w, "OK")
}

// ReadinessProbe checks if database is available.
func ReadinessProbe(w http.ResponseWriter, r *http.Request) {
	klog.V(2).Info("readinessProbe - Checking Database connection.")

	// TODO - Implement probe.

	fmt.Fprint(w, "OK")
}
