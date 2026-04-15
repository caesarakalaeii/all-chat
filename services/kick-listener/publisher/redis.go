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

	// maxStreamLength is the maximum number of messages to keep in the stream (sliding window)
	maxStreamLength = 100000

	// ringBufferCapacity is the number of messages the ring buffer can hold before dropping.
	ringBufferCapacity = 1000
)

// RawMessage represents a raw message to be published to Redis Stream
type RawMessage struct {
	MessageID   string            `json:"message_id,omitempty"`
	Platform    string            `json:"platform"`
	OverlayID   string            `json:"overlay_id"`
	ChannelID   string            `json:"channel_id"`
	ChannelName string            `json:"channel_name"`
	UserID      string            `json:"user_id,omitempty"`
	Username    string            `json:"username,omitempty"`
	Text        string            `json:"text,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	RawMessage  json.RawMessage   `json:"raw_message,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
}

// StreamPublisher publishes messages to Redis Streams, wrapped with a
// RingBufferPublisher so transient XADD failures are buffered for retry rather
// than silently dropped (eliminates LI-01, LI-02, LI-03).
type StreamPublisher struct {
	redis      *redis.Client
	logger     *zap.Logger
	ringBuffer *sharedlistener.RingBufferPublisher
}

// NewStreamPublisher creates a new Redis Stream publisher backed by a RingBufferPublisher.
func NewStreamPublisher(redisClient *redis.Client, logger *zap.Logger) *StreamPublisher {
	p := newStreamPublisherWithRingBuffer(
		buildXAddFunc(redisClient),
		logger,
		prometheus.DefaultRegisterer,
	)
	p.redis = redisClient
	return p
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
		"kick-listener",
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
			MaxLen: maxStreamLength,
			Approx: true,
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

// IsHealthy checks if the Redis connection is healthy.
func (p *StreamPublisher) IsHealthy(ctx context.Context) bool {
	if p.redis == nil {
		return true // test path without real client
	}
	_, err := p.redis.Ping(ctx).Result()
	return err == nil
}
