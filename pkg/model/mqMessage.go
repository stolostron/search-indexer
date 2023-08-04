// Copyright Contributors to the Open Cluster Management project
package model

type MqMessage struct {
	UID        string                 `json:"uid,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Hash       uint64                 `json:"hash,omitempty"`
}
