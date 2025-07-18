package server

import (
	"context"
	"encoding/json"
	"github.com/segmentio/kafka-go"
	"k8s.io/klog/v2"
)

type ResourceEvent struct {
	Type    string      `json:"type"` // addResource, updateResource, deleteResource, addEdge, deleteEdge, CLEAR-ALL-START, CLEAR-ALL-END
	Cluster string      `json:"cluster"`
	Payload interface{} `json:"payload"` // Node, Edge, Deletion, nil if CLEAR-ALL-START/CLEAR-ALL-END
}

func (s *ServerConfig) KafkaResourceHandler(ctx context.Context, reader *kafka.Reader) {
	for {
		m, err := reader.ReadMessage(ctx)
		if err != nil {
			klog.Errorf("Error reading kafka message: %v", err)
		}

		re := ResourceEvent{}
		if err = json.Unmarshal(m.Value, &re); err != nil {
			klog.Errorf("Error unmarshalling resource event: %v", err)
			continue
		}

		if len(m.Headers) > 0 {
			for _, v := range m.Headers {
				// ONLY HANDLING RESYNC REQUESTS NOW - FOR SAKE OF GENERATING WORST CASE PERFORMANCE DATA
				if v.Key == "ClearAll" {
					if err = s.Dao.ResetResourcesKafka(ctx, reader, re.Cluster); err != nil {
						klog.Errorf("Error processing resync request from cluster %s: %v", re.Cluster, err)
						// TODO: retry resync from CLEAR-ALL-START to CLEAR-ALL-END
					}
				}
			}
		}
	}
}
