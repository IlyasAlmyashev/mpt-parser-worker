package kafka

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"mpt-parser-worker/internal/logger"
	"mpt-parser-worker/internal/model"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type Producer interface {
	Send(product model.ProductRaw) error
	SendBatch(products []model.ProductRaw) error
	Close() error
}

type KafkaProducer struct {
	producer *kafka.Producer
	topic    string
	logger   logger.Logger
}

// NewProducer creates a new KafkaProducer using the Confluent Kafka Go library.
// brokers is a slice of broker host:port strings, e.g. []string{"localhost:9092"}.
// topic is the target Kafka topic name.
func NewProducer(brokers []string, topic string, logger logger.Logger) *KafkaProducer {
	cfgMap := &kafka.ConfigMap{
		"bootstrap.servers": strings.Join(brokers, ","),
		// You might add other config parameters here if needed, for instance:
		// "acks": "all",
		// "enable.idempotence": true,
	}

	p, err := kafka.NewProducer(cfgMap)
	if err != nil {
		logger.Errorf("Failed to create Kafka producer: %v", err)
		return nil
	}

	logger.Infof("Kafka producer created. Brokers: %v, Topic: %s", brokers, topic)

	return &KafkaProducer{
		producer: p,
		topic:    topic,
		logger:   logger,
	}
}

// Send sends a single ProductRaw message to Kafka.
func (p *KafkaProducer) Send(product model.ProductRaw) error {
	jsonData, err := json.Marshal(product)
	if err != nil {
		p.logger.Errorf("Failed to marshal product: %v", err)
		return fmt.Errorf("failed to marshal product: %w", err)
	}

	deliveryChan := make(chan kafka.Event, 1)
	defer close(deliveryChan)

	// Produce the message asynchronously
	err = p.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &p.topic,
			Partition: kafka.PartitionAny,
		},
		Value: jsonData,
		Headers: []kafka.Header{
			{Key: "marketplace", Value: []byte(product.Marketplace)},
			{Key: "timestamp", Value: []byte(product.Timestamp.Format(time.RFC3339))},
		},
	}, deliveryChan)

	if err != nil {
		p.logger.Errorf("Failed to produce message: %v", err)
		return fmt.Errorf("failed to produce message: %w", err)
	}

	// Wait for a delivery result
	ev := <-deliveryChan
	m, ok := ev.(*kafka.Message)
	if !ok {
		p.logger.Errorf("Unexpected event type: %T", ev)
		return fmt.Errorf("unexpected event type: %T", ev)
	}
	if m.TopicPartition.Error != nil {
		p.logger.Errorf("Failed to deliver message: %v", m.TopicPartition.Error)
		return m.TopicPartition.Error
	}

	p.logger.Infof("Message sent to partition %d at offset %v", m.TopicPartition.Partition, m.TopicPartition.Offset)
	return nil
}

// SendBatch sends multiple ProductRaw messages to Kafka in a batch.
func (p *KafkaProducer) SendBatch(products []model.ProductRaw) error {
	if len(products) == 0 {
		return nil
	}
	p.logger.Debugf("Preparing to send batch of %d products", len(products))

	// Create a channel for delivery reports
	deliveryChan := make(chan kafka.Event, len(products))
	defer close(deliveryChan)

	// Send all messages
	for _, product := range products {
		// Convert product to JSON
		jsonData, err := json.Marshal(product)
		if err != nil {
			p.logger.Errorf("Failed to marshal product: %v", err)
			return fmt.Errorf("failed to marshal product: %w", err)
		}

		err = p.producer.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{
				Topic:     &p.topic,
				Partition: kafka.PartitionAny,
			},
			Value: jsonData,
			Headers: []kafka.Header{
				{Key: "marketplace", Value: []byte(product.Marketplace)},
				{Key: "timestamp", Value: []byte(product.Timestamp.Format(time.RFC3339))},
			},
		}, deliveryChan)

		if err != nil {
			p.logger.Errorf("Failed to produce message batch: %v", err)
			return fmt.Errorf("failed to produce message batch: %w", err)
		}
	}

	var deliveryErr error
	for i := 0; i < len(products); i++ {
		ev := <-deliveryChan
		m, ok := ev.(*kafka.Message)
		if !ok {
			p.logger.Errorf("Unexpected event type in batch: %T", ev)
			if deliveryErr == nil {
				deliveryErr = fmt.Errorf("unexpected event type: %T", ev)
			}
			continue
		}
		if m.TopicPartition.Error != nil {
			p.logger.Errorf("Failed to deliver message in batch: %v", m.TopicPartition.Error)
			deliveryErr = m.TopicPartition.Error
		}
	}

	if deliveryErr != nil {
		return fmt.Errorf("batch delivery error: %w", deliveryErr)
	}
	p.logger.Infof("Successfully sent batch of %d products", len(products))
	return nil
}

// Close flushes messages and closes the producer.
func (p *KafkaProducer) Close() error {
	p.logger.Infof("Flushing producer messages...")
	p.producer.Flush(10_000) // Wait up to the 10s for remaining messages
	p.logger.Infof("Closing producer...")
	p.producer.Close()
	return nil
}
