// Copyright Contributors to the Open Cluster Management project
package mq

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"log"
	"time"

	"github.com/IBM/sarama"
	"github.com/stolostron/search-indexer/pkg/config"
	"github.com/stolostron/search-indexer/pkg/database"
	"github.com/stolostron/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

func StartKafkaConsumer(ctx context.Context) {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Net.TLS.Enable = true
	saramaConfig.Net.TLS.Config = &tls.Config{InsecureSkipVerify: true} // #nosec G402 - POC code, won't merge.
	saramaConfig.Consumer.Return.Errors = true
	saramaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest // TODO this will replay all previous messages. Change to sarama.OffsetNewest to only get new messages.

	main, err := sarama.NewConsumer(config.Cfg.KafkaBrokerList, saramaConfig)
	if err != nil {
		log.Panic(err)
	}

	defer func() {
		if err := main.Close(); err != nil {
			log.Panic(err)
		}
	}()

	consumer, err := main.ConsumePartition(config.Cfg.KafkaTopic, config.Cfg.KafkaPartition, saramaConfig.Consumer.Offsets.Initial)
	if err != nil {
		log.Panic(err)
	}

	// TODO: Discover existing topics.
	// client, clientErr := sarama.NewClient(brokerList, saramaConfig)
	// if clientErr != nil {
	// 	log.Panic(clientErr)
	// }
	// offset, offsetErr := client.GetOffset(topic, partition, sarama.OffsetNewest)
	// klog.Infof("Existing messages offset: %+v \toffsetErr:%+v\n", offset, offsetErr)

	dao := database.NewDAO(nil)
	batch := database.NewBatchWithRetry(ctx, &dao, &model.SyncResponse{})

	ticker := time.NewTicker(10 * time.Second)
	for {
		select {
		case err := <-consumer.Errors():
			klog.Error(err)
		case msg := <-consumer.Messages():

			var mqMessage model.MqMessage
			err := json.Unmarshal(msg.Value, &mqMessage)
			if err != nil {
				klog.Errorf("Error unmarshalling message: %+v\n", err)
			}

			klog.Infof("Received mq event. UID: %s\t Kind: %s\t Name: %+v\n",
				mqMessage.UID, mqMessage.Properties["kind"], mqMessage.Properties["name"])

			batchErr := batch.QueueMQ(config.Cfg.KafkaTopic, mqMessage)
			if batchErr != nil {
				klog.Errorf("Error queueing message: %+v\n", batchErr)
				continue
			}

		case <-ticker.C:
			klog.Infof(">>> Flushing batch to database every 10 seconds.\n")
			batch.FlushMQ()
		}
	}
}
