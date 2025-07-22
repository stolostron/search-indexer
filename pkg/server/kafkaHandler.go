package server

import (
	"context"
	"github.com/segmentio/kafka-go"
	"k8s.io/klog/v2"
)

type ResourceEvent struct {
	Type    string      `json:"type"` // addResource, updateResource, deleteResource, addEdge, deleteEdge, CLEAR-ALL-START, CLEAR-ALL-END
	Cluster string      `json:"cluster"`
	Payload interface{} `json:"payload"` // Node, Edge, Deletion, nil if CLEAR-ALL-START/CLEAR-ALL-END
}

func (s *ServerConfig) KafkaResourceHandler(ctx context.Context, reader *kafka.Reader, i int) {
	if err := s.Dao.ResetResourcesKafka(ctx, reader, i); err != nil {
		klog.Errorf("Error processing resync request f: %v", err)
		// TODO: retry resync from CLEAR-ALL-START to CLEAR-ALL-END
	}
}
