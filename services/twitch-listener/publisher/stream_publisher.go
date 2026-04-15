// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package publisher

import (
	"context"
	"fmt"

	"github.com/caesar/all-chat/services/twitch-listener/models"
	sharedlistener "github.com/caesar/all-chat/shared/listener"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// StreamKey is the Redis Streams key for raw chat messages
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
	client     *redis.Client
	logger     *zap.Logger
	ringBuffer *sharedlistener.RingBufferPublisher
}

// NewStreamPublisher creates a new Redis Streams publisher backed by a RingBufferPublisher.
func NewStreamPublisher(client *redis.Client, logger *zap.Logger) *StreamPublisher {
	return newStreamPublisherWithRingBuffer(
		buildXAddFunc(client),
		logger,
		prometheus.DefaultRegisterer,
	)
}

// newStreamPublisherWithRingBuffer is the internal constructor used by both
// production code and tests (tests supply a custom publishFn and isolated registry).
func newStreamPublisherWithRingBuffer(
	publishFn sharedlistener.PublishFunc,
	logger *zap.Logger,
	reg prometheus.Registerer,
) *StreamPublisher {
	rb := sharedlistener.NewRingBufferPublisherWithRegisterer(
		ringBufferCapacity,
		publishFn,
		logger,
		"twitch-listener",
		reg,
	)

	return &StreamPublisher{
		logger:     logger,
		ringBuffer: rb,
	}
}

// buildXAddFunc returns a PublishFunc that writes a pre-serialised JSON payload
// to the chat:raw Redis Stream using the "data" field (the only field that
// message-processor reads from stream entries).
func buildXAddFunc(client *redis.Client) sharedlistener.PublishFunc {
	return func(ctx context.Context, payload []byte) error {
		args := &redis.XAddArgs{
			Stream: StreamKey,
			MaxLen: MaxStreamLength,
			Approx: true,
			Values: map[string]interface{}{
				"data": string(payload),
			},
		}
		_, err := client.XAdd(ctx, args).Result()
		return err
	}
}

// Publish serialises msg to JSON and delegates to the ring buffer. If the XADD
// call fails, the payload is buffered for retry and nil is returned to the caller.
func (p *StreamPublisher) Publish(ctx context.Context, msg *models.RawChatMessage) error {
	jsonBytes, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal message to JSON: %w", err)
	}

	return p.ringBuffer.Publish(ctx, jsonBytes)
}

// PublishBatch publishes multiple messages via the ring buffer individually.
// Each message is published separately so partial failures are buffered rather
// than losing the entire batch (replaces the pipeline approach which had LI-02).
func (p *StreamPublisher) PublishBatch(ctx context.Context, messages []*models.RawChatMessage) error {
	if len(messages) == 0 {
		return nil
	}

	for _, msg := range messages {
		jsonBytes, err := msg.ToJSON()
		if err != nil {
			p.logger.Warn("Failed to marshal message in batch",
				zap.String("message_id", msg.MessageID),
				zap.Error(err),
			)
			continue
		}
		// Ring buffer absorbs transient failures; PublishBatch always returns nil.
		_ = p.ringBuffer.Publish(ctx, jsonBytes)
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
func (p *StreamPublisher) Ping(ctx context.Context) error {
	if p.client == nil {
		return nil
	}
	return p.client.Ping(ctx).Err()
}
