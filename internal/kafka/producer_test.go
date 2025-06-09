package kafka

import (
	"testing"
	"time"

	"mpt-parser-worker/internal/logger"
	"mpt-parser-worker/internal/model"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
	"github.com/stretchr/testify/assert"
)

func TestProducer_Send(t *testing.T) {
	// Create mock logger
	mockLogger := logger.NewMockLogger()

	// Create mock Sarama config
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true

	// Create mock sync producer
	mockProducer := mocks.NewSyncProducer(t, config)

	// Create test product
	testProduct := model.ProductRaw{
		Title:       "Test Product",
		Price:       1000,
		URL:         "https://test.com",
		Marketplace: "test",
		Timestamp:   time.Now(),
	}

	// Set up expected behavior
	mockProducer.ExpectSendMessageAndSucceed()

	// Create our producer with mock
	producer := &KafkaProducer{
		producer: mockProducer,
		topic:    "test-topic",
		logger:   mockLogger,
	}

	// Test sending message
	err := producer.Send(testProduct)

	// Assertions
	assert.NoError(t, err)
	assert.Len(t, mockLogger.InfoMessages, 1)
	assert.Contains(t, mockLogger.InfoMessages[0], "Message sent to partition")
	assert.Len(t, mockLogger.ErrorMessages, 0)

	// Verify all expectations were met
	err = mockProducer.Close()
	assert.NoError(t, err)
}

func TestProducer_SendError(t *testing.T) {
	// Create mock logger
	mockLogger := logger.NewMockLogger()

	// Create mock Sarama config
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true

	// Create mock sync producer
	mockProducer := mocks.NewSyncProducer(t, config)

	// Create test product
	testProduct := model.ProductRaw{
		Title:       "Test Product",
		Price:       1000,
		URL:         "https://test.com",
		Marketplace: "test",
		Timestamp:   time.Now(),
	}

	// Set up expected behavior - simulate error
	mockProducer.ExpectSendMessageAndFail(sarama.ErrInvalidMessage)

	// Create our producer with mock
	producer := &KafkaProducer{
		producer: mockProducer,
		topic:    "test-topic",
		logger:   mockLogger,
	}

	// Test sending message
	err := producer.Send(testProduct)

	// Assertions
	assert.Error(t, err)
	assert.Equal(t, sarama.ErrInvalidMessage, err)
	assert.Len(t, mockLogger.InfoMessages, 0)
	assert.Len(t, mockLogger.ErrorMessages, 1)
	assert.Contains(t, mockLogger.ErrorMessages[0], "Failed to send message")

	// Verify all expectations were met
	err = mockProducer.Close()
	assert.NoError(t, err)
}
