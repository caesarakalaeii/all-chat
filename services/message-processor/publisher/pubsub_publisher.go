package publisher

import (
	"context"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// retryDelays defines the wait durations between Pub/Sub publish retry attempts.
var retryDelays = []time.Duration{
	100 * time.Millisecond,
	500 * time.Millisecond,
	2 * time.Second,
}

// retryPublish retries fn up to 3 times with exponential backoff.
func retryPublish(ctx context.Context, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if attempt == 2 {
			break
		}
		delay := retryDelays[attempt]
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return lastErr
}

// PubSubPublisher publishes messages to Redis Pub/Sub channels
type PubSubPublisher struct {
	client  *redis.Client
	logger  *zap.Logger
	metrics *metrics.ProcessorMetrics
}

// NewPubSubPublisher creates a new Pub/Sub publisher
func NewPubSubPublisher(client *redis.Client, logger *zap.Logger, m *metrics.ProcessorMetrics) *PubSubPublisher {
	return &PubSubPublisher{
		client:  client,
		logger:  logger,
		metrics: m,
	}
}

// Publish publishes a message to an overlay-specific channel.
// MP-04: Wraps the Redis Publish call in retryPublish for resilience.
func (p *PubSubPublisher) Publish(ctx context.Context, overlayID string, msg *models.UnifiedChatMessage) error {
	// Convert message to JSON
	jsonBytes, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Determine channel based on whether this is an update (TikTok like aggregates)
	channel := fmt.Sprintf("overlay:%s", overlayID)
	isUpdate := msg.Event != nil && msg.Event.IsUpdate

	if isUpdate {
		// For TikTok like aggregates: publish updates to special channel
		channel = fmt.Sprintf("overlay:%s:updates", overlayID)
	}

	jsonStr := string(jsonBytes)
	err = retryPublish(ctx, func() error {
		return p.client.Publish(ctx, channel, jsonStr).Err()
	})
	if err != nil {
		p.logger.Error("Failed to publish to Pub/Sub after retries",
			zap.String("channel", channel),
			zap.String("overlay_id", overlayID),
			zap.String("message_id", msg.ID),
			zap.Bool("is_update", isUpdate),
			zap.Error(err),
		)
		p.metrics.PublishRetryTotal.WithLabelValues("exhausted").Inc()
		return fmt.Errorf("failed to publish to Redis Pub/Sub: %w", err)
	}

	p.logger.Debug("Published to Pub/Sub",
		zap.String("channel", channel),
		zap.String("message_id", msg.ID),
		zap.String("platform", msg.Platform),
		zap.Bool("is_update", isUpdate),
	)

	return nil
}

// PublishToMultiple publishes a message to multiple overlay channels.
// MP-05: Publishes individually (not via pipeline) to isolate per-overlay failures.
// Returns error only if ALL overlays fail; partial failures are logged.
func (p *PubSubPublisher) PublishToMultiple(ctx context.Context, overlayIDs []string, msg *models.UnifiedChatMessage) error {
	if len(overlayIDs) == 0 {
		return nil
	}

	// Convert message to JSON once
	jsonBytes, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	jsonStr := string(jsonBytes)

	successCount := 0
	failCount := 0

	for _, overlayID := range overlayIDs {
		channel := fmt.Sprintf("overlay:%s", overlayID)
		err := retryPublish(ctx, func() error {
			return p.client.Publish(ctx, channel, jsonStr).Err()
		})
		if err != nil {
			p.logger.Error("Failed to publish to overlay Pub/Sub",
				zap.String("overlay_id", overlayID),
				zap.String("message_id", msg.ID),
				zap.Error(err),
			)
			p.metrics.PublishRetryTotal.WithLabelValues("exhausted").Inc()
			failCount++
		} else {
			successCount++
		}
	}

	if successCount == 0 && failCount > 0 {
		// All overlays failed
		return fmt.Errorf("failed to publish to all %d overlays for message %s", failCount, msg.ID)
	}

	if failCount > 0 {
		p.logger.Warn("Partial publish failure",
			zap.Int("success_count", successCount),
			zap.Int("fail_count", failCount),
			zap.String("message_id", msg.ID),
		)
	} else {
		p.logger.Debug("Published to multiple overlays",
			zap.Int("overlay_count", successCount),
			zap.String("message_id", msg.ID),
		)
	}

	return nil
}
