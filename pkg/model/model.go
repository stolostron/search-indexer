// Copyright Contributors to the Open Cluster Management project

package model

import "sync"

// Resource - Describes a resource (node)
type Resource struct {
	Kind           string `json:"kind,omitempty"`
	UID            string `json:"uid,omitempty"`
	ResourceString string `json:"resourceString,omitempty"`
	Properties     map[string]interface{}
}

// Describes a relationship between resources
type Edge struct {
	SourceUID, DestUID   string
	EdgeType             string
	SourceKind, DestKind string
}

// SyncEvent - Object sent by the collector with the resources to change.
type SyncEvent struct {
	AddResources    []Resource
	UpdateResources []Resource
	DeleteResources []DeleteResourceEvent

	AddEdges    []Edge
	DeleteEdges []Edge
}

// SyncResponse - Response to a SyncEvent
type SyncResponse struct {
	mu                sync.Mutex // guards []SyncError fields, Total fields only ever accessed singly threaded
	TotalAdded        int
	TotalUpdated      int
	TotalDeleted      int
	TotalEdgesAdded   int
	TotalEdgesDeleted int
	AddErrors         []SyncError
	UpdateErrors      []SyncError
	DeleteErrors      []SyncError
	AddEdgeErrors     []SyncError
	DeleteEdgeErrors  []SyncError
	Version           string
}

// ValidationResponse contains the resource counts from the database for a cluster.
type ValidationResponse struct {
	TotalResources int
	TotalEdges     int
}

func (r *SyncResponse) AppendError(action string, e SyncError) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch action {
	case "addResource":
		r.AddErrors = append(r.AddErrors, e)
	case "updateResource":
		r.UpdateErrors = append(r.UpdateErrors, e)
	case "deleteResource":
		r.DeleteErrors = append(r.DeleteErrors, e)
	case "addEdge":
		r.AddEdgeErrors = append(r.AddEdgeErrors, e)
	case "deleteEdge":
		r.DeleteEdgeErrors = append(r.DeleteEdgeErrors, e)
	default:
		return false
	}
	return true
}

// SyncError is used to respond with errors.
type SyncError struct {
	ResourceUID string
	Message     string // Often comes out of a golang error using .Error()
}

// DeleteResourceEvent - Contains the information needed to delete an existing resource.
type DeleteResourceEvent struct {
	UID string `json:"uid,omitempty"`
}
