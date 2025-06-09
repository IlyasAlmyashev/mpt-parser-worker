package kafka

import (
	"encoding/json"

	"mpt-parser-worker/internal/logger"
	"mpt-parser-worker/internal/model"

	"github.com/IBM/sarama"
)

// Producer defines the interface for message producers
type Producer interface {
	Send(product model.ProductRaw) error
	Close() error
}

// KafkaProducer implements the Producer interface
type KafkaProducer struct {
	producer sarama.SyncProducer
	topic    string
	logger   logger.Logger
}

// NewProducer creates a new KafkaProducer instance
func NewProducer(brokers []string, topic string, logger logger.Logger) Producer {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.RequiredAcks = sarama.WaitForAll

	logger.Infof("Connecting to Kafka brokers: %v", brokers)

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		logger.Errorf("Failed to create Kafka producer: %v", err)
		return nil
	}

	logger.Infof("Successfully connected to Kafka")

	return &KafkaProducer{
		producer: producer,
		topic:    topic,
		logger:   logger,
	}
}

// Send implements Producer.Send
func (p *KafkaProducer) Send(product model.ProductRaw) error {
	value, err := json.Marshal(product)
	if err != nil {
		p.logger.Errorf("Failed to marshal product: %v", err)
		return err
	}

	msg := &sarama.ProducerMessage{
		Topic: p.topic,
		Value: sarama.StringEncoder(value),
	}

	partition, offset, err := p.producer.SendMessage(msg)
	if err != nil {
		p.logger.Errorf("Failed to send message: %v", err)
		return err
	}

	p.logger.Infof("Message sent to partition %d at offset %d", partition, offset)
	return nil
}

// Close implements Producer.Close
func (p *KafkaProducer) Close() error {
	return p.producer.Close()
}
