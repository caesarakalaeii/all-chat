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

	"github.com/caesar/all-chat/services/facebook-listener/models"
	sharedlistener "github.com/caesar/all-chat/shared/listener"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// chatStreamKey is the Redis Stream key for raw chat messages (shared).
	chatStreamKey = "chat:raw"

	// maxStreamLength is the maximum number of messages to keep (sliding window).
	maxStreamLength = 100000

	// ringBufferCapacity is the number of messages buffered before dropping.
	ringBufferCapacity = 1000
)

// StreamPublisher publishes raw Facebook messages to Redis Streams, wrapped
// with a RingBufferPublisher so transient XADD failures are buffered for
// retry rather than silently dropped (same shape as kick-listener).
type StreamPublisher struct {
	redis      *redis.Client
	logger     *zap.Logger
	ringBuffer *sharedlistener.RingBufferPublisher
}

// NewStreamPublisher creates the publisher backed by a ring buffer.
func NewStreamPublisher(redisClient *redis.Client, logger *zap.Logger) *StreamPublisher {
	p := newStreamPublisherWithRingBuffer(
		buildXAddFunc(redisClient),
		logger,
		prometheus.DefaultRegisterer,
	)
	p.redis = redisClient
	return p
}

func newStreamPublisherWithRingBuffer(
	publishFn sharedlistener.PublishFunc,
	logger *zap.Logger,
	reg prometheus.Registerer,
) *StreamPublisher {
	rb := sharedlistener.NewRingBufferPublisherWithRegisterer(
		ringBufferCapacity,
		publishFn,
		logger,
		"facebook-listener",
		reg,
	)
	return &StreamPublisher{logger: logger, ringBuffer: rb}
}

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

// Publish serialises msg and delegates to the ring buffer. XADD failures are
// buffered for retry and nil is returned to the caller.
func (p *StreamPublisher) Publish(ctx context.Context, msg *models.RawChatMessage) error {
	data, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	return p.ringBuffer.Publish(ctx, data)
}

// PublishBatch publishes each message; failure of one does not stop the rest.
func (p *StreamPublisher) PublishBatch(ctx context.Context, messages []*models.RawChatMessage) error {
	var firstErr error
	for _, m := range messages {
		if err := p.Publish(ctx, m); err != nil {
			p.logger.Warn("Batch publish failed for message",
				zap.String("message_id", m.MessageID), zap.Error(err))
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

// Stop drains the ring buffer retry goroutine before shutdown.
func (p *StreamPublisher) Stop() {
	if p.ringBuffer != nil {
		p.ringBuffer.Stop()
	}
}

// IsHealthy checks the Redis connection.
func (p *StreamPublisher) IsHealthy(ctx context.Context) bool {
	if p.redis == nil {
		return true
	}
	_, err := p.redis.Ping(ctx).Result()
	return err == nil
}
