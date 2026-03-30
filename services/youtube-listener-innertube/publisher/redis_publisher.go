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
	sharedlistener "github.com/caesar/all-chat/shared/listener"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// StreamKey is the Redis Streams key for raw chat messages
	// Must match official youtube-listener for drop-in compatibility
	StreamKey = "chat:raw"

	// MaxStreamLength is the maximum number of messages to keep in the stream (sliding window)
	MaxStreamLength = 100000 // 100K messages — consumer is real-time, no need for deep history

	// ringBufferCapacity is the number of messages the ring buffer can hold before dropping.
	ringBufferCapacity = 1000
)

// StreamPublisher publishes raw chat messages to Redis Streams, wrapped with a
// RingBufferPublisher so transient XADD failures are buffered for retry rather
// than silently dropped (eliminates LI-01, LI-02, LI-03).
type StreamPublisher struct {
	client         *redis.Client
	logger         *zap.Logger
	metrics        *metrics.InnerTubeMetrics
	deletionBuffer *deletion.DeletionBuffer
	ringBuffer     *sharedlistener.RingBufferPublisher
}

// NewStreamPublisher creates a new Redis Streams publisher backed by a RingBufferPublisher.
func NewStreamPublisher(client *redis.Client, logger *zap.Logger, m *metrics.InnerTubeMetrics, deletionBuffer *deletion.DeletionBuffer) *StreamPublisher {
	p := newStreamPublisherWithRingBuffer(
		buildXAddFunc(client, m),
		logger,
		m,
		deletionBuffer,
		prometheus.DefaultRegisterer,
	)
	p.client = client
	return p
}

// newStreamPublisherWithRingBuffer is the internal constructor used by both
// production code and tests.
func newStreamPublisherWithRingBuffer(
	publishFn sharedlistener.PublishFunc,
	logger *zap.Logger,
	m *metrics.InnerTubeMetrics,
	deletionBuffer *deletion.DeletionBuffer,
	reg prometheus.Registerer,
) *StreamPublisher {
	rb := sharedlistener.NewRingBufferPublisherWithRegisterer(
		ringBufferCapacity,
		publishFn,
		logger,
		"youtube-listener-innertube",
		reg,
	)

	return &StreamPublisher{
		logger:         logger,
		metrics:        m,
		deletionBuffer: deletionBuffer,
		ringBuffer:     rb,
	}
}

// buildXAddFunc returns a PublishFunc that writes a pre-serialised JSON payload
// to the chat:raw Redis Stream. Metrics are updated on each call.
func buildXAddFunc(client *redis.Client, m *metrics.InnerTubeMetrics) sharedlistener.PublishFunc {
	return func(ctx context.Context, payload []byte) error {
		args := &redis.XAddArgs{
			Stream: StreamKey,
			MaxLen: MaxStreamLength,
			Approx: true,
			Values: map[string]interface{}{
				"data": string(payload),
			},
		}

		if m != nil {
			m.RedisPublishAttempts.WithLabelValues(metrics.ServiceLabel).Inc()
		}

		start := time.Now()
		_, err := client.XAdd(ctx, args).Result()
		duration := time.Since(start)

		if err != nil {
			if m != nil {
				m.Errors.WithLabelValues(metrics.ServiceLabel, metrics.ErrorTypeRedis).Inc()
			}
			return err
		}

		if m != nil {
			m.RedisPublishSuccess.WithLabelValues(metrics.ServiceLabel).Inc()
			m.RedisPublishLatency.WithLabelValues(metrics.ServiceLabel).Observe(duration.Seconds())
		}

		return nil
	}
}

// SetDeletionBuffer sets the deletion buffer (allows initialization after construction to avoid circular dependency)
func (p *StreamPublisher) SetDeletionBuffer(deletionBuffer *deletion.DeletionBuffer) {
	p.deletionBuffer = deletionBuffer
}

// Publish publishes a raw chat message to Redis Streams.
// Contract: Must publish with exact same schema as official youtube-listener
// to maintain drop-in compatibility with message-processor.
//
// Deletion events are routed through the deletion buffer (500ms delay).
// All other events are published immediately via the ring buffer.
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
				// Fallback: publish immediately via ring buffer (degraded mode)
				return p.publishViaRingBuffer(ctx, msg)
			}
			// Successfully buffered, will be published after 500ms
			return nil
		}
		// No buffer configured, publish immediately
		return p.publishViaRingBuffer(ctx, msg)
	}

	// Regular messages publish immediately via ring buffer
	return p.publishViaRingBuffer(ctx, msg)
}

// publishViaRingBuffer serialises msg to JSON and delegates to the ring buffer.
// If XADD fails, the payload is buffered for retry and nil is returned to the caller.
func (p *StreamPublisher) publishViaRingBuffer(ctx context.Context, msg *innertube.RawChatMessage) error {
	jsonBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message to JSON: %w", err)
	}

	// Track per-channel published messages (best-effort, before ring buffer)
	if p.metrics != nil {
		p.metrics.MessagesPublished.WithLabelValues(metrics.ServiceLabel, msg.ChannelID).Inc()
	}

	// Debug-level logging
	p.logger.Debug("Publishing message to Redis Streams",
		zap.String("stream", StreamKey),
		zap.String("message_id", msg.MessageID),
		zap.String("platform", msg.Platform),
		zap.String("channel", msg.ChannelID),
	)

	// Cache YouTube custom emotes (best-effort, non-blocking)
	if p.client != nil {
		if emoteData := msg.Tags["emote_data"]; emoteData != "" {
			var emotes []yt_emote_cache.EmoteEntry
			if err := json.Unmarshal([]byte(emoteData), &emotes); err == nil && len(emotes) > 0 {
				if cacheErr := yt_emote_cache.CacheYTEmotes(ctx, p.client, msg.ChannelID, emotes); cacheErr != nil {
					p.logger.Warn("Failed to cache YouTube emotes",
						zap.String("channel_id", msg.ChannelID),
						zap.Error(cacheErr),
					)
				}
			}
		}
	}

	return p.ringBuffer.Publish(ctx, jsonBytes)
}

// PublishBatch publishes multiple messages via the ring buffer individually.
// Each message is published separately so partial failures are buffered rather
// than losing the entire batch (replaces the pipeline approach which had LI-02).
func (p *StreamPublisher) PublishBatch(ctx context.Context, messages []*innertube.RawChatMessage) error {
	if len(messages) == 0 {
		return nil
	}

	for _, msg := range messages {
		if err := p.Publish(ctx, msg); err != nil {
			p.logger.Warn("Failed to publish message in batch",
				zap.String("message_id", msg.MessageID),
				zap.Error(err),
			)
		}
	}

	p.logger.Debug("Published batch to Redis Streams",
		zap.String("stream", StreamKey),
		zap.Int("count", len(messages)),
	)

	return nil
}

// Stop signals the ring buffer retry goroutine to exit and waits for it to finish.
// Call this during graceful shutdown.
func (p *StreamPublisher) Stop() {
	if p.ringBuffer != nil {
		p.ringBuffer.Stop()
	}
}

// Ping checks if Redis connection is alive.
// Used by readiness probe to verify Redis connectivity.
func (p *StreamPublisher) Ping(ctx context.Context) error {
	if p.client == nil {
		return nil
	}
	return p.client.Ping(ctx).Err()
}
