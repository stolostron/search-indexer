// Copyright Contributors to the Open Cluster Management project

package model

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
	ClearAll bool `json:"clearAll,omitempty"`

	AddResources    []Resource
	UpdateResources []Resource
	DeleteResources []DeleteResourceEvent

	AddEdges    []Edge
	DeleteEdges []Edge
	RequestId   int
}

// SyncResponse - Response to a SyncEvent
type SyncResponse struct {
	TotalAdded        int
	TotalUpdated      int
	TotalDeleted      int
	TotalResources    int
	TotalEdgesAdded   int
	TotalEdgesDeleted int
	TotalEdges        int
	AddErrors         []SyncError
	UpdateErrors      []SyncError
	DeleteErrors      []SyncError
	AddEdgeErrors     []SyncError
	DeleteEdgeErrors  []SyncError
	Version           string
	RequestId         int
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
