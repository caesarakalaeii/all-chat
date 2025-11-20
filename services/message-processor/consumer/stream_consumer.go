package consumer

import (
	"context"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// StreamKey is the Redis Streams key to consume from
	StreamKey = "chat:raw"

	// ConsumerGroup is the consumer group name
	ConsumerGroup = "message-processor"

	// ConsumerName is this consumer's identifier
	ConsumerName = "processor-1"

	// ReadCount is how many messages to read in one batch
	ReadCount = 100

	// ReadBlockTime is how long to block waiting for messages
	ReadBlockTime = 5 * time.Second
)

// MessageHandler is called for each consumed message
type MessageHandler func(ctx context.Context, msg *models.RawChatMessage) error

// StreamConsumer consumes messages from Redis Streams
type StreamConsumer struct {
	client  *redis.Client
	logger  *zap.Logger
	metrics *metrics.ProcessorMetrics
	handler MessageHandler
	stopCh  chan struct{}
}

// NewStreamConsumer creates a new Redis Streams consumer
func NewStreamConsumer(client *redis.Client, logger *zap.Logger, m *metrics.ProcessorMetrics, handler MessageHandler) *StreamConsumer {
	return &StreamConsumer{
		client:  client,
		logger:  logger,
		metrics: m,
		handler: handler,
		stopCh:  make(chan struct{}),
	}
}

// Start begins consuming messages from the stream
func (c *StreamConsumer) Start(ctx context.Context) error {
	// Create consumer group if it doesn't exist
	if err := c.createConsumerGroup(ctx); err != nil {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	c.logger.Info("Stream consumer started",
		zap.String("stream", StreamKey),
		zap.String("group", ConsumerGroup),
		zap.String("consumer", ConsumerName),
	)

	// Start consuming
	go c.consumeLoop(ctx)

	return nil
}

// Stop gracefully stops the consumer
func (c *StreamConsumer) Stop() {
	close(c.stopCh)
	c.logger.Info("Stream consumer stopped")
}

// createConsumerGroup creates the consumer group if it doesn't exist
func (c *StreamConsumer) createConsumerGroup(ctx context.Context) error {
	// Try to create the consumer group
	// $ means start from the end (only new messages)
	err := c.client.XGroupCreateMkStream(ctx, StreamKey, ConsumerGroup, "$").Err()
	if err != nil {
		// BUSYGROUP error means group already exists, which is fine
		if err.Error() != "BUSYGROUP Consumer Group name already exists" {
			return err
		}
		c.logger.Debug("Consumer group already exists",
			zap.String("group", ConsumerGroup),
		)
	} else {
		c.logger.Info("Created consumer group",
			zap.String("group", ConsumerGroup),
		)
	}

	return nil
}

// consumeLoop continuously reads and processes messages
func (c *StreamConsumer) consumeLoop(ctx context.Context) {
	for {
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		default:
			if err := c.readAndProcess(ctx); err != nil {
				c.logger.Error("Error reading messages", zap.Error(err))
				time.Sleep(1 * time.Second) // Back off on error
			}
		}
	}
}

// readAndProcess reads messages and processes them
func (c *StreamConsumer) readAndProcess(ctx context.Context) error {
	// Read messages from the stream
	// ">" means only new messages not yet delivered to this consumer
	streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    ConsumerGroup,
		Consumer: ConsumerName,
		Streams:  []string{StreamKey, ">"},
		Count:    ReadCount,
		Block:    ReadBlockTime,
	}).Result()

	if err != nil {
		// redis.Nil means no messages available (timeout)
		if err == redis.Nil {
			return nil
		}
		return fmt.Errorf("failed to read from stream: %w", err)
	}

	// Process each stream (we only have one)
	for _, stream := range streams {
		for _, message := range stream.Messages {
			if err := c.processMessage(ctx, message); err != nil {
				c.logger.Error("Failed to process message",
					zap.String("stream_id", message.ID),
					zap.Error(err),
				)
				// Continue processing other messages even if one fails
				continue
			}

			// ACK the message after successful processing
			if err := c.client.XAck(ctx, StreamKey, ConsumerGroup, message.ID).Err(); err != nil {
				c.logger.Warn("Failed to ACK message",
					zap.String("stream_id", message.ID),
					zap.Error(err),
				)
			}
		}
	}

	return nil
}

// processMessage processes a single message
func (c *StreamConsumer) processMessage(ctx context.Context, msg redis.XMessage) error {
	// Extract the "data" field which contains the full JSON
	dataStr, ok := msg.Values["data"].(string)
	if !ok {
		c.metrics.RecordStreamError("message-processor", "invalid_data")
		return fmt.Errorf("missing or invalid 'data' field in message")
	}

	// Parse the raw message
	rawMsg, err := models.ParseRawMessage([]byte(dataStr))
	if err != nil {
		c.metrics.RecordStreamError("message-processor", "parse_error")
		return fmt.Errorf("failed to parse raw message: %w", err)
	}

	// Record message consumed
	c.metrics.RecordMessageConsumed("message-processor", rawMsg.Platform, ConsumerGroup)

	c.logger.Debug("Processing message",
		zap.String("message_id", rawMsg.MessageID),
		zap.String("platform", rawMsg.Platform),
		zap.String("channel", rawMsg.ChannelID),
		zap.String("user", rawMsg.Username),
	)

	// Call the handler (which does normalization + enrichment + publishing)
	start := time.Now()
	if err := c.handler(ctx, rawMsg); err != nil {
		c.metrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "handler", "failed")
		return fmt.Errorf("handler failed: %w", err)
	}

	// Record successful processing and duration
	c.metrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "handler", "success")
	c.metrics.ProcessingDuration.WithLabelValues("message-processor", rawMsg.Platform).Observe(time.Since(start).Seconds())

	return nil
}

// GetPendingCount returns the number of pending messages for this consumer
func (c *StreamConsumer) GetPendingCount(ctx context.Context) (int64, error) {
	pending, err := c.client.XPending(ctx, StreamKey, ConsumerGroup).Result()
	if err != nil {
		return 0, err
	}
	return pending.Count, nil
}
