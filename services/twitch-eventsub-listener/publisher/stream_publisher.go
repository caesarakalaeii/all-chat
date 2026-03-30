package publisher

import (
	"context"
	"fmt"

	"github.com/caesar/all-chat/services/twitch-eventsub-listener/models"
	sharedlistener "github.com/caesar/all-chat/shared/listener"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// StreamKey is the Redis Streams key to publish to
	StreamKey = "chat:raw"

	// maxStreamLength is the maximum number of messages to keep in the stream (sliding window)
	maxStreamLength = 100000

	// ringBufferCapacity is the number of messages the ring buffer can hold before dropping.
	ringBufferCapacity = 1000
)

// StreamPublisher publishes messages to Redis Streams, wrapped with a
// RingBufferPublisher so transient XADD failures are buffered for retry rather
// than silently dropped (eliminates LI-01, LI-02, LI-03).
type StreamPublisher struct {
	client     *redis.Client
	logger     *zap.Logger
	ringBuffer *sharedlistener.RingBufferPublisher
}

// NewStreamPublisher creates a new stream publisher backed by a RingBufferPublisher.
func NewStreamPublisher(client *redis.Client, logger *zap.Logger) *StreamPublisher {
	p := newStreamPublisherWithRingBuffer(
		buildXAddFunc(client),
		logger,
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
	reg prometheus.Registerer,
) *StreamPublisher {
	rb := sharedlistener.NewRingBufferPublisherWithRegisterer(
		ringBufferCapacity,
		publishFn,
		logger,
		"twitch-eventsub-listener",
		reg,
	)

	return &StreamPublisher{
		logger:     logger,
		ringBuffer: rb,
	}
}

// buildXAddFunc returns a PublishFunc that writes a pre-serialised JSON payload
// to the chat:raw Redis Stream using the "data" field.
func buildXAddFunc(client *redis.Client) sharedlistener.PublishFunc {
	return func(ctx context.Context, payload []byte) error {
		return client.XAdd(ctx, &redis.XAddArgs{
			Stream: StreamKey,
			MaxLen: maxStreamLength,
			Approx: true,
			Values: map[string]interface{}{
				"data": string(payload),
			},
		}).Err()
	}
}

// Publish serialises msg to JSON and delegates to the ring buffer. If the XADD
// call fails, the payload is buffered for retry and nil is returned to the caller.
func (p *StreamPublisher) Publish(ctx context.Context, msg *models.RawChatMessage) error {
	jsonBytes, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return p.ringBuffer.Publish(ctx, jsonBytes)
}

// Stop signals the ring buffer retry goroutine to exit and waits for it to finish.
// Call this during graceful shutdown.
func (p *StreamPublisher) Stop() {
	if p.ringBuffer != nil {
		p.ringBuffer.Stop()
	}
}
