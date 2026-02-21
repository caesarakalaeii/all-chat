package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// StreamKey is the Redis Streams key for raw chat messages
	// Must match official youtube-listener for drop-in compatibility
	StreamKey = "chat:raw"

	// MaxStreamLength is the maximum number of messages to keep in the stream (sliding window)
	MaxStreamLength = 1000000 // 1 million messages
)

// StreamPublisher publishes raw chat messages to Redis Streams
type StreamPublisher struct {
	client *redis.Client
	logger *zap.Logger
}

// NewStreamPublisher creates a new Redis Streams publisher
func NewStreamPublisher(client *redis.Client, logger *zap.Logger) *StreamPublisher {
	return &StreamPublisher{
		client: client,
		logger: logger,
	}
}

// Publish publishes a raw chat message to Redis Streams
// Contract: Must publish with exact same XADD field mapping as official youtube-listener
// to maintain drop-in compatibility with message-processor
func (p *StreamPublisher) Publish(ctx context.Context, msg *innertube.RawChatMessage) error {
	// Convert message to JSON for the 'data' field
	jsonBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message to JSON: %w", err)
	}

	// Prepare stream entry with exact field mapping from official youtube-listener
	// See: services/youtube-listener/publisher/stream_publisher.go lines 44-53
	values := map[string]interface{}{
		"message_id": msg.MessageID,
		"platform":   msg.Platform, // Must be "youtube"
		"channel_id": msg.ChannelID,
		"user_id":    msg.UserID,
		"username":   msg.Username,
		"text":       msg.Text,
		"timestamp":  msg.Timestamp.Format(time.RFC3339Nano), // RFC3339Nano format required
		"data":       string(jsonBytes),                      // Full JSON for easy processing
	}

	// Publish to stream with MAXLEN ~1000000 (approximate trimming for performance)
	args := &redis.XAddArgs{
		Stream: StreamKey,
		MaxLen: MaxStreamLength,
		Approx: true, // Use ~ for efficient trimming (same as official listener)
		Values: values,
	}

	streamID, err := p.client.XAdd(ctx, args).Result()
	if err != nil {
		// Log error but don't crash service (per user decision)
		p.logger.Error("Failed to publish message to Redis Streams",
			zap.String("stream", StreamKey),
			zap.String("message_id", msg.MessageID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to publish to Redis Streams: %w", err)
	}

	// Debug-level logging for successful publishes (avoid spam at info level)
	p.logger.Debug("Published message to Redis Streams",
		zap.String("stream", StreamKey),
		zap.String("stream_id", streamID),
		zap.String("message_id", msg.MessageID),
		zap.String("platform", msg.Platform),
		zap.String("channel", msg.ChannelID),
		zap.String("username", msg.Username),
	)

	return nil
}

// PublishBatch publishes multiple messages in a single pipeline for better performance
// This is an optimization for high-throughput scenarios (Phase 12)
func (p *StreamPublisher) PublishBatch(ctx context.Context, messages []*innertube.RawChatMessage) error {
	if len(messages) == 0 {
		return nil
	}

	pipe := p.client.Pipeline()

	for _, msg := range messages {
		jsonBytes, err := json.Marshal(msg)
		if err != nil {
			p.logger.Warn("Failed to marshal message in batch",
				zap.String("message_id", msg.MessageID),
				zap.Error(err),
			)
			continue
		}

		values := map[string]interface{}{
			"message_id": msg.MessageID,
			"platform":   msg.Platform,
			"channel_id": msg.ChannelID,
			"user_id":    msg.UserID,
			"username":   msg.Username,
			"text":       msg.Text,
			"timestamp":  msg.Timestamp.Format(time.RFC3339Nano),
			"data":       string(jsonBytes),
		}

		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: StreamKey,
			MaxLen: MaxStreamLength,
			Approx: true,
			Values: values,
		})
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		p.logger.Error("Failed to execute batch publish",
			zap.Int("batch_size", len(messages)),
			zap.Error(err),
		)
		return fmt.Errorf("failed to execute batch publish: %w", err)
	}

	p.logger.Debug("Published batch to Redis Streams",
		zap.String("stream", StreamKey),
		zap.Int("count", len(messages)),
	)

	return nil
}

// Ping checks if Redis connection is alive
// Used by readiness probe to verify Redis connectivity
func (p *StreamPublisher) Ping(ctx context.Context) error {
	return p.client.Ping(ctx).Err()
}
