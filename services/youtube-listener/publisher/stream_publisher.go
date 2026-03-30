package publisher

import (
	"context"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// StreamKey is the Redis Streams key for raw chat messages (same as Twitch)
	StreamKey = "chat:raw"

	// MaxStreamLength is the maximum number of messages to keep in the stream (sliding window)
	MaxStreamLength = 100000 // 100K messages — consumer is real-time, no need for deep history
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
func (p *StreamPublisher) Publish(ctx context.Context, msg *models.RawChatMessage) error {
	// Convert message to JSON
	jsonBytes, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal message to JSON: %w", err)
	}

	// Prepare stream entry
	values := map[string]interface{}{
		"message_id": msg.MessageID,
		"platform":   msg.Platform,
		"channel_id": msg.ChannelID,
		"user_id":    msg.UserID,
		"username":   msg.Username,
		"text":       msg.Text,
		"timestamp":  msg.Timestamp.Format(time.RFC3339Nano),
		"data":       string(jsonBytes), // Full JSON for easy processing
	}

	// Publish to stream with MAXLEN ~1000000 (approximate trimming)
	args := &redis.XAddArgs{
		Stream: StreamKey,
		MaxLen: MaxStreamLength,
		Approx: true, // Use ~ for efficient trimming
		Values: values,
	}

	streamID, err := p.client.XAdd(ctx, args).Result()
	if err != nil {
		p.logger.Error("Failed to publish message to Redis Streams",
			zap.String("stream", StreamKey),
			zap.String("message_id", msg.MessageID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to publish to Redis Streams: %w", err)
	}

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
func (p *StreamPublisher) PublishBatch(ctx context.Context, messages []*models.RawChatMessage) error {
	if len(messages) == 0 {
		return nil
	}

	pipe := p.client.Pipeline()

	for _, msg := range messages {
		jsonBytes, err := msg.ToJSON()
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
func (p *StreamPublisher) Ping(ctx context.Context) error {
	return p.client.Ping(ctx).Err()
}
