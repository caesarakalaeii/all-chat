package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/deletion"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/metrics"
	"github.com/caesar/all-chat/services/youtube-listener-innertube/yt_emote_cache"
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
	client         *redis.Client
	logger         *zap.Logger
	metrics        *metrics.InnerTubeMetrics
	deletionBuffer *deletion.DeletionBuffer
}

// NewStreamPublisher creates a new Redis Streams publisher
func NewStreamPublisher(client *redis.Client, logger *zap.Logger, m *metrics.InnerTubeMetrics, deletionBuffer *deletion.DeletionBuffer) *StreamPublisher {
	return &StreamPublisher{
		client:         client,
		logger:         logger,
		metrics:        m,
		deletionBuffer: deletionBuffer,
	}
}

// SetDeletionBuffer sets the deletion buffer (allows initialization after construction to avoid circular dependency)
func (p *StreamPublisher) SetDeletionBuffer(deletionBuffer *deletion.DeletionBuffer) {
	p.deletionBuffer = deletionBuffer
}

// Publish publishes a raw chat message to Redis Streams
// Contract: Must publish with exact same XADD field mapping as official youtube-listener
// to maintain drop-in compatibility with message-processor
func (p *StreamPublisher) Publish(ctx context.Context, msg *innertube.RawChatMessage) error {
	// Route deletion events through buffer (500ms delay)
	if msg.EventType == "message_deletion" {
		if p.deletionBuffer != nil {
			if err := p.deletionBuffer.Add(msg.ChannelID, msg); err != nil {
				p.logger.Error("Failed to buffer deletion event",
					zap.String("channel_id", msg.ChannelID),
					zap.String("message_id", msg.MessageID),
					zap.Error(err),
				)
				// Fallback: publish immediately (degraded mode)
				return p.publishToRedis(ctx, msg)
			}
			// Successfully buffered, will be published after 500ms
			return nil
		}
		// No buffer configured, publish immediately
		return p.publishToRedis(ctx, msg)
	}

	// Regular messages publish immediately (no delay)
	return p.publishToRedis(ctx, msg)
}

// publishToRedis handles the actual Redis XADD operation
func (p *StreamPublisher) publishToRedis(ctx context.Context, msg *innertube.RawChatMessage) error {
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

	// Track Redis publish attempt
	if p.metrics != nil {
		p.metrics.RedisPublishAttempts.WithLabelValues(metrics.ServiceLabel).Inc()
	}

	// Measure publish latency
	start := time.Now()
	streamID, err := p.client.XAdd(ctx, args).Result()
	duration := time.Since(start)

	if err != nil {
		// Track Redis error
		if p.metrics != nil {
			p.metrics.Errors.WithLabelValues(metrics.ServiceLabel, metrics.ErrorTypeRedis).Inc()
		}

		// Log error but don't crash service (per user decision)
		p.logger.Error("Failed to publish message to Redis Streams",
			zap.String("stream", StreamKey),
			zap.String("message_id", msg.MessageID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to publish to Redis Streams: %w", err)
	}

	// Track successful publish
	if p.metrics != nil {
		p.metrics.RedisPublishSuccess.WithLabelValues(metrics.ServiceLabel).Inc()
		p.metrics.MessagesPublished.WithLabelValues(metrics.ServiceLabel, msg.ChannelID).Inc()
		p.metrics.RedisPublishLatency.WithLabelValues(metrics.ServiceLabel).Observe(duration.Seconds())
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

	// Cache YouTube custom emotes (best-effort, non-blocking)
	if emoteData := msg.Tags["emote_data"]; emoteData != "" {
		var emotes []yt_emote_cache.EmoteEntry
		if err := json.Unmarshal([]byte(emoteData), &emotes); err == nil && len(emotes) > 0 {
			if cacheErr := yt_emote_cache.CacheYTEmotes(ctx, p.client, msg.ChannelID, emotes); cacheErr != nil {
				// Best-effort: log warning but do not fail the publish
				p.logger.Warn("Failed to cache YouTube emotes",
					zap.String("channel_id", msg.ChannelID),
					zap.Error(cacheErr),
				)
			}
		}
	}

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
