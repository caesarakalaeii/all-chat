package publisher

import (
	"context"
	"fmt"

	"github.com/caesar/all-chat/services/twitch-eventsub-listener/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// StreamKey is the Redis Streams key to publish to
	StreamKey = "chat:raw"
)

// StreamPublisher publishes messages to Redis Streams
type StreamPublisher struct {
	client *redis.Client
	logger *zap.Logger
}

// NewStreamPublisher creates a new stream publisher
func NewStreamPublisher(client *redis.Client, logger *zap.Logger) *StreamPublisher {
	return &StreamPublisher{
		client: client,
		logger: logger,
	}
}

// Publish publishes a message to the Redis Stream
func (p *StreamPublisher) Publish(ctx context.Context, msg *models.RawChatMessage) error {
	// Convert to JSON
	jsonBytes, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Publish to stream
	if err := p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamKey,
		Values: map[string]interface{}{
			"data": string(jsonBytes),
		},
	}).Err(); err != nil {
		return fmt.Errorf("failed to add to stream: %w", err)
	}

	p.logger.Debug("Published event to Redis Stream",
		zap.String("stream", StreamKey),
		zap.String("message_id", msg.MessageID),
		zap.String("event_type", msg.EventType),
		zap.String("channel", msg.ChannelID),
	)

	return nil
}
