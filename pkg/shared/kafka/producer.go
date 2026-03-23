// Package kafka provides Kafka producer and consumer utilities with CloudEvents support.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CloudEvent represents a CloudEvents v1.0 envelope.
type CloudEvent struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Type            string          `json:"type"`
	Source          string          `json:"source"`
	Time            string          `json:"time"`
	DataContentType string          `json:"datacontenttype"`
	Data            json.RawMessage `json:"data"`
	TraceParent     string          `json:"traceparent,omitempty"`
}

// NewCloudEvent creates a new CloudEvents envelope.
func NewCloudEvent(eventType, source string, data interface{}) (*CloudEvent, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal event data: %w", err)
	}

	return &CloudEvent{
		SpecVersion:     "1.0",
		ID:              uuid.New().String(),
		Type:            eventType,
		Source:          source,
		Time:            time.Now().UTC().Format(time.RFC3339),
		DataContentType: "application/json",
		Data:            dataBytes,
	}, nil
}

// WithTraceParent sets the W3C traceparent on the event.
func (e *CloudEvent) WithTraceParent(traceparent string) *CloudEvent {
	e.TraceParent = traceparent
	return e
}

// Marshal serializes the CloudEvent to JSON bytes.
func (e *CloudEvent) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// Producer defines the interface for publishing events to Kafka.
type Producer interface {
	// Publish sends a CloudEvent to the specified topic.
	Publish(ctx context.Context, topic string, key string, event *CloudEvent) error
	// Close gracefully shuts down the producer.
	Close() error
}

// Consumer defines the interface for consuming events from Kafka.
type Consumer interface {
	// Subscribe starts consuming from the specified topics.
	Subscribe(ctx context.Context, topics []string, handler MessageHandler) error
	// Close gracefully shuts down the consumer.
	Close() error
}

// MessageHandler processes a received CloudEvent.
type MessageHandler func(ctx context.Context, event *CloudEvent) error

// TopicName generates a standardized topic name.
// Format: opsnexus.{domain}.{event_type}
func TopicName(domain, eventType string) string {
	return fmt.Sprintf("opsnexus.%s.%s", domain, eventType)
}

// ConsumerGroup generates a standardized consumer group name.
// Format: {consuming-service}.{topic-short-name}
func ConsumerGroup(service, topicShortName string) string {
	return fmt.Sprintf("%s.%s", service, topicShortName)
}
