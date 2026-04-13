package status

import (
	"context"
	"encoding/json"
	"time"

	"github.com/caesar/all-chat/shared/listener"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	PlatformStatusChannel = "platform:status"
	// maxPublishAttempts is the number of attempts (1 initial + 2 retries = 3 total).
	maxPublishAttempts = 3
)

// Message represents a platform connection status update
type Message struct {
	Platform     string     `json:"platform"`
	ChannelID    string     `json:"channel_id"`
	ChannelName  string     `json:"channel_name,omitempty"`
	Status       string     `json:"status"` // "connected", "reconnecting", "offline"
	NextRetryAt  *time.Time `json:"next_retry_at,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
}

// marshalMessage serializes a Message to a JSON string for Redis Pub/Sub.
func marshalMessage(msg Message) (string, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Publisher publishes platform status updates to Redis Pub/Sub
type Publisher struct {
	redisClient *redis.Client
	logger      *zap.Logger
}

func NewPublisher(redisClient *redis.Client, logger *zap.Logger) *Publisher {
	return &Publisher{redisClient: redisClient, logger: logger}
}

// Publish publishes a platform status message to Redis Pub/Sub.
// On failure it retries up to maxPublishAttempts times with jittered exponential backoff.
// If the context is cancelled between retries, it returns immediately.
// After all attempts are exhausted, logs at Error level with sentinel "status_publish_exhausted".
func (p *Publisher) Publish(ctx context.Context, msg Message) {
	data, err := marshalMessage(msg)
	if err != nil {
		p.logger.Error("Failed to marshal status message", zap.Error(err))
		return
	}

	var lastErr error
	for attempt := 0; attempt < maxPublishAttempts; attempt++ {
		if attempt > 0 {
			// Wait with jittered backoff, but honour context cancellation.
			select {
			case <-ctx.Done():
				return
			case <-time.After(listener.JitteredBackoff(attempt)):
			}
		}

		if err := p.redisClient.Publish(ctx, PlatformStatusChannel, data).Err(); err != nil {
			lastErr = err
			continue
		}
		return // success
	}

	p.logger.Error("status_publish_exhausted",
		zap.Int("attempts", maxPublishAttempts),
		zap.Error(lastErr),
	)
}
