package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	sharedlistener "github.com/caesar/all-chat/shared/listener"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// chatStreamKey is the Redis Stream key for raw chat messages
	chatStreamKey = "chat:raw"

	// ringBufferCapacity is the number of messages the ring buffer can hold before dropping.
	ringBufferCapacity = 1000
)

// RawMessage represents a raw message to be published to Redis Stream.
// This mirrors the pattern from kick-listener/publisher/redis.go.
type RawMessage struct {
	MessageID   string                 `json:"message_id,omitempty"`
	Platform    string                 `json:"platform"`
	OverlayID   string                 `json:"overlay_id"`
	ChannelID   string                 `json:"channel_id"`
	ChannelName string                 `json:"channel_name"`
	UserID      string                 `json:"user_id,omitempty"`
	Username    string                 `json:"username,omitempty"`
	Text        string                 `json:"text,omitempty"`
	Tags        map[string]string      `json:"tags,omitempty"`
	RawMessage  json.RawMessage        `json:"raw_message,omitempty"`
	EventType   string                 `json:"event_type,omitempty"`
	EventData   map[string]interface{} `json:"event_data,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
}

// Publisher is the interface that wraps the Publish method.
type Publisher interface {
	Publish(ctx context.Context, msg *RawMessage) error
}

// StreamPublisher publishes messages to a Redis Stream, backed by a
// RingBufferPublisher so transient XADD failures are buffered for retry rather
// than silently dropped (eliminates LI-01, LI-02, LI-03).
type StreamPublisher struct {
	cmdable    redis.Cmdable
	logger     *zap.Logger
	ringBuffer *sharedlistener.RingBufferPublisher
}

// NewStreamPublisher creates a new StreamPublisher backed by a *redis.Client
// and a RingBufferPublisher for resilient publishing.
func NewStreamPublisher(redisClient *redis.Client, logger *zap.Logger) *StreamPublisher {
	return NewStreamPublisherWithRingBuffer(
		buildXAddFunc(redisClient),
		logger,
		prometheus.DefaultRegisterer,
	)
}

// NewStreamPublisherFromCmdable creates a new StreamPublisher from any redis.Cmdable.
// This is intended for unit testing with mock implementations.
// Note: The ring buffer wraps a data-only XADD; for unit testing the direct Cmdable
// path use NewStreamPublisherWithRingBuffer with a custom publishFn.
func NewStreamPublisherFromCmdable(cmdable redis.Cmdable, logger *zap.Logger) *StreamPublisher {
	rb := sharedlistener.NewRingBufferPublisherWithRegisterer(
		ringBufferCapacity,
		func(ctx context.Context, payload []byte) error {
			_, err := cmdable.XAdd(ctx, &redis.XAddArgs{
				Stream: chatStreamKey,
				Values: map[string]interface{}{
					"data": string(payload),
				},
			}).Result()
			return err
		},
		logger,
		"discord-listener",
		prometheus.NewRegistry(), // isolated registry for test cmdable path
	)

	return &StreamPublisher{
		cmdable:    cmdable,
		logger:     logger,
		ringBuffer: rb,
	}
}

// NewStreamPublisherWithRingBuffer creates a StreamPublisher using a custom
// publishFn and Prometheus registry. Used in tests to inject failures and
// isolate metrics registration.
func NewStreamPublisherWithRingBuffer(
	publishFn sharedlistener.PublishFunc,
	logger *zap.Logger,
	reg prometheus.Registerer,
) *StreamPublisher {
	rb := sharedlistener.NewRingBufferPublisherWithRegisterer(
		ringBufferCapacity,
		publishFn,
		logger,
		"discord-listener",
		reg,
	)

	return &StreamPublisher{
		logger:     logger,
		ringBuffer: rb,
	}
}

// buildXAddFunc returns a PublishFunc that writes a pre-serialised JSON payload
// to the chat:raw Redis Stream using the "data" field.
func buildXAddFunc(redisClient *redis.Client) sharedlistener.PublishFunc {
	return func(ctx context.Context, payload []byte) error {
		_, err := redisClient.XAdd(ctx, &redis.XAddArgs{
			Stream: chatStreamKey,
			Values: map[string]interface{}{
				"data": string(payload),
			},
		}).Result()
		return err
	}
}

// Publish serialises msg to JSON and delegates to the ring buffer. If the XADD
// call fails, the payload is buffered for retry and nil is returned to the caller.
func (p *StreamPublisher) Publish(ctx context.Context, msg *RawMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return p.ringBuffer.Publish(ctx, data)
}

// Stop signals the ring buffer retry goroutine to exit and waits for it to finish.
// Call this during graceful shutdown.
func (p *StreamPublisher) Stop() {
	if p.ringBuffer != nil {
		p.ringBuffer.Stop()
	}
}
