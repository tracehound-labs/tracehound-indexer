// Package kafka provides Kafka producer functionality for Tracehound.
// This broadcasts the whale scents to downstream consumers.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
	"github.com/tracehound-labs/tracehound-indexer/internal/blockchain"
)

// ProducerConfig holds configuration for the Kafka producer.
type ProducerConfig struct {
	Brokers      []string
	Topic        string
	BatchSize    int
	BatchTimeout time.Duration
	Compression  string // "snappy", "gzip", "lz4", or "none"
}

// Producer publishes whale scents to Kafka topics.
// This is how we broadcast discoveries to other hounds in the pack.
type Producer struct {
	writer *kafka.Writer
	topic  string
	logger *logrus.Logger
}

// NewProducer creates a new Kafka producer configured for whale scent broadcasting.
func NewProducer(cfg ProducerConfig, logger *logrus.Logger) (*Producer, error) {
	if logger == nil {
		logger = logrus.New()
	}

	if len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("no Kafka brokers specified")
	}

	if cfg.Topic == "" {
		cfg.Topic = "whale_scents"
	}

	if cfg.BatchSize == 0 {
		cfg.BatchSize = 100
	}

	if cfg.BatchTimeout == 0 {
		cfg.BatchTimeout = 10 * time.Millisecond
	}

	// Configure compression
	var compression kafka.Compression
	switch cfg.Compression {
	case "snappy":
		compression = kafka.Snappy
	case "gzip":
		compression = kafka.Gzip
	case "lz4":
		compression = kafka.Lz4
	case "zstd":
		compression = kafka.Zstd
	case "none":
		compression = 0
	default:
		compression = kafka.Snappy // Default to snappy
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(cfg.Brokers...),
		Topic:        cfg.Topic,
		Balancer:     &HashBalancer{}, // Use tx_hash for partitioning
		BatchSize:    cfg.BatchSize,
		BatchTimeout: cfg.BatchTimeout,
		Compression:  compression,
		Async:        false, // Synchronous writes for reliability
		RequiredAcks: kafka.RequireOne,
	}

	logger.WithFields(logrus.Fields{
		"brokers": cfg.Brokers,
		"topic":   cfg.Topic,
	}).Info("📡 Kafka producer initialized")

	return &Producer{
		writer: writer,
		topic:  cfg.Topic,
		logger: logger,
	}, nil
}

// HashBalancer implements kafka.Balancer interface.
// It partitions messages by their key hash (tx_hash).
type HashBalancer struct{}

// Balance returns the partition for the given message based on key hash.
func (b *HashBalancer) Balance(msg kafka.Message, partitions ...int) int {
	if len(partitions) == 0 {
		return 0
	}

	if len(msg.Key) == 0 {
		// If no key, round-robin
		return partitions[0]
	}

	// FNV hash of the key
	h := fnv.New32a()
	h.Write(msg.Key)
	hash := h.Sum32()

	return partitions[int(hash)%len(partitions)]
}

// PublishBatch sends multiple whale scents to Kafka in a single batch.
// This is more efficient than individual publishes.
func (p *Producer) PublishBatch(ctx context.Context, events []blockchain.RawEvent) error {
	if len(events) == 0 {
		return nil
	}

	messages := make([]kafka.Message, 0, len(events))

	for _, event := range events {
		// Serialize the event to JSON
		value, err := json.Marshal(event)
		if err != nil {
			p.logger.WithError(err).Warn("Failed to marshal event for Kafka")
			continue
		}

		msg := kafka.Message{
			Key:   []byte(event.TxHash), // Use tx_hash as key for partitioning
			Value: value,
			Headers: []kafka.Header{
				{Key: "event_type", Value: []byte(event.EventType)},
				{Key: "trader", Value: []byte(event.Trader)},
				{Key: "symbol", Value: []byte(event.Symbol)},
			},
			Time: event.Timestamp,
		}

		messages = append(messages, msg)
	}

	if len(messages) == 0 {
		return nil
	}

	// Write batch to Kafka
	if err := p.writer.WriteMessages(ctx, messages...); err != nil {
		return fmt.Errorf("failed to write messages to Kafka: %w", err)
	}

	p.logger.WithFields(logrus.Fields{
		"count": len(messages),
		"topic": p.topic,
	}).Debug("📤 Scents broadcast to Kafka")

	return nil
}

// Publish sends a single whale scent to Kafka.
func (p *Producer) Publish(ctx context.Context, event blockchain.RawEvent) error {
	return p.PublishBatch(ctx, []blockchain.RawEvent{event})
}

// Stats returns the current writer statistics.
func (p *Producer) Stats() kafka.WriterStats {
	return p.writer.Stats()
}

// Close gracefully shuts down the Kafka producer.
// The hound stops broadcasting after the hunt.
func (p *Producer) Close() error {
	p.logger.Info("📡 Closing Kafka producer...")
	return p.writer.Close()
}
