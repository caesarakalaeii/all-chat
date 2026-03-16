package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// chatStreamKey is the Redis Stream key for raw chat messages
	chatStreamKey = "chat:raw"
)

// RawMessage represents a raw message to be published to Redis Stream.
// This mirrors the pattern from kick-listener/publisher/redis.go.
type RawMessage struct {
	MessageID   string            `json:"message_id,omitempty"`
	Platform    string            `json:"platform"`
	OverlayID   string            `json:"overlay_id"`
	ChannelID   string            `json:"channel_id"`
	ChannelName string            `json:"channel_name"`
	UserID      string            `json:"user_id,omitempty"`
	Username    string            `json:"username,omitempty"`
	Text        string            `json:"text,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	RawMessage  json.RawMessage   `json:"raw_message,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
}

// Publisher is the interface that wraps the Publish method.
type Publisher interface {
	Publish(ctx context.Context, msg *RawMessage) error
}

// StreamPublisher publishes messages to a Redis Stream.
type StreamPublisher struct {
	cmdable redis.Cmdable
	logger  *zap.Logger
}

// NewStreamPublisher creates a new StreamPublisher backed by a *redis.Client.
func NewStreamPublisher(redisClient *redis.Client, logger *zap.Logger) *StreamPublisher {
	return &StreamPublisher{
		cmdable: redisClient,
		logger:  logger,
	}
}

// NewStreamPublisherFromCmdable creates a new StreamPublisher from any redis.Cmdable.
// This is intended for unit testing with mock implementations.
func NewStreamPublisherFromCmdable(cmdable redis.Cmdable, logger *zap.Logger) *StreamPublisher {
	return &StreamPublisher{
		cmdable: cmdable,
		logger:  logger,
	}
}

// Publish publishes a RawMessage to the chat:raw Redis Stream with a single "data" field.
func (p *StreamPublisher) Publish(ctx context.Context, msg *RawMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	_, err = p.cmdable.XAdd(ctx, &redis.XAddArgs{
		Stream: chatStreamKey,
		Values: map[string]interface{}{
			"data": string(data),
		},
	}).Result()

	if err != nil {
		if p.logger != nil {
			p.logger.Error("Failed to publish message to Redis Stream",
				zap.Error(err),
				zap.String("stream", chatStreamKey),
				zap.String("platform", msg.Platform),
				zap.String("overlay_id", msg.OverlayID),
			)
		}
		return fmt.Errorf("failed to publish to Redis Stream: %w", err)
	}

	if p.logger != nil {
		p.logger.Debug("Published message to Redis Stream",
			zap.String("stream", chatStreamKey),
			zap.String("platform", msg.Platform),
			zap.String("overlay_id", msg.OverlayID),
			zap.String("channel_id", msg.ChannelID),
		)
	}

	return nil
}
