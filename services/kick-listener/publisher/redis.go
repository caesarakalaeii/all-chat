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
	// Redis Stream key for raw chat messages
	chatStreamKey = "chat:raw"
)

// RawMessage represents a raw message to be published to Redis Stream
type RawMessage struct {
	Platform    string          `json:"platform"`
	OverlayID   string          `json:"overlay_id"`
	ChannelID   string          `json:"channel_id"`
	ChannelName string          `json:"channel_name"`
	RawMessage  json.RawMessage `json:"raw_message"`
	Timestamp   time.Time       `json:"timestamp"`
}

// StreamPublisher publishes messages to Redis Streams
type StreamPublisher struct {
	redis  *redis.Client
	logger *zap.Logger
}

// NewStreamPublisher creates a new Redis Stream publisher
func NewStreamPublisher(redisClient *redis.Client, logger *zap.Logger) *StreamPublisher {
	return &StreamPublisher{
		redis:  redisClient,
		logger: logger,
	}
}

// Publish publishes a raw message to Redis Stream
func (p *StreamPublisher) Publish(ctx context.Context, msg *RawMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Publish to Redis Stream
	_, err = p.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: chatStreamKey,
		Values: map[string]interface{}{
			"data": string(data),
		},
	}).Result()

	if err != nil {
		p.logger.Error("Failed to publish message to Redis Stream",
			zap.Error(err),
			zap.String("stream", chatStreamKey),
			zap.String("platform", msg.Platform),
			zap.String("overlay_id", msg.OverlayID),
		)
		return fmt.Errorf("failed to publish to Redis Stream: %w", err)
	}

	p.logger.Debug("Published message to Redis Stream",
		zap.String("stream", chatStreamKey),
		zap.String("platform", msg.Platform),
		zap.String("overlay_id", msg.OverlayID),
		zap.String("channel_id", msg.ChannelID),
	)

	return nil
}

// IsHealthy checks if the Redis connection is healthy
func (p *StreamPublisher) IsHealthy(ctx context.Context) bool {
	_, err := p.redis.Ping(ctx).Result()
	return err == nil
}
