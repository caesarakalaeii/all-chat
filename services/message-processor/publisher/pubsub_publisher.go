package publisher

import (
	"context"
	"fmt"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// PubSubPublisher publishes messages to Redis Pub/Sub channels
type PubSubPublisher struct {
	client *redis.Client
	logger *zap.Logger
}

// NewPubSubPublisher creates a new Pub/Sub publisher
func NewPubSubPublisher(client *redis.Client, logger *zap.Logger) *PubSubPublisher {
	return &PubSubPublisher{
		client: client,
		logger: logger,
	}
}

// Publish publishes a message to an overlay-specific channel
func (p *PubSubPublisher) Publish(ctx context.Context, overlayID string, msg *models.UnifiedChatMessage) error {
	// Convert message to JSON
	jsonBytes, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Publish to overlay channel
	channel := fmt.Sprintf("overlay:%s", overlayID)
	if err := p.client.Publish(ctx, channel, string(jsonBytes)).Err(); err != nil {
		p.logger.Error("Failed to publish to Pub/Sub",
			zap.String("channel", channel),
			zap.String("message_id", msg.ID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to publish to Redis Pub/Sub: %w", err)
	}

	p.logger.Debug("Published to Pub/Sub",
		zap.String("channel", channel),
		zap.String("message_id", msg.ID),
		zap.String("platform", msg.Platform),
	)

	return nil
}

// PublishToMultiple publishes a message to multiple overlay channels
func (p *PubSubPublisher) PublishToMultiple(ctx context.Context, overlayIDs []string, msg *models.UnifiedChatMessage) error {
	// Convert message to JSON once
	jsonBytes, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	jsonStr := string(jsonBytes)

	// Use pipeline for efficiency
	pipe := p.client.Pipeline()

	for _, overlayID := range overlayIDs {
		channel := fmt.Sprintf("overlay:%s", overlayID)
		pipe.Publish(ctx, channel, jsonStr)
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		p.logger.Error("Failed to publish batch to Pub/Sub",
			zap.Int("overlay_count", len(overlayIDs)),
			zap.String("message_id", msg.ID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to execute batch publish: %w", err)
	}

	p.logger.Debug("Published to multiple overlays",
		zap.Int("overlay_count", len(overlayIDs)),
		zap.String("message_id", msg.ID),
	)

	return nil
}
