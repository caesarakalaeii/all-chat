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

	"github.com/caesar/all-chat/services/instagram-listener/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// chatStreamKey is the Redis Stream key for raw chat messages (shared).
	chatStreamKey = "chat:raw"

	// platformStatusChannel is the Redis Pub/Sub channel for platform status
	// events (same channel the other listeners publish to).
	platformStatusChannel = "platform:status"
)

// StreamPublisher publishes raw Instagram messages to Redis Streams via
// buffered XADD, and platform:status events via Pub/Sub.
type StreamPublisher struct {
	redis  *redis.Client
	logger *zap.Logger
}

// NewStreamPublisher creates the publisher.
func NewStreamPublisher(redisClient *redis.Client, logger *zap.Logger) *StreamPublisher {
	return &StreamPublisher{redis: redisClient, logger: logger}
}

// Publish serialises msg and XADDs it to chat:raw.
func (p *StreamPublisher) Publish(ctx context.Context, msg *models.RawChatMessage) error {
	data, err := msg.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	return p.publish(ctx, data)
}

func (p *StreamPublisher) publish(ctx context.Context, payload []byte) error {
	for attempt := 1; attempt <= 3; attempt++ {
		_, err := p.redis.XAdd(ctx, &redis.XAddArgs{
			Stream: chatStreamKey,
			MaxLen: 100000,
			Approx: true,
			Values: map[string]interface{}{
				"data": string(payload),
			},
		}).Result()
		if err == nil {
			return nil
		}
		p.logger.Warn("XADD failed",
			zap.Int("attempt", attempt), zap.Error(err))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt*100) * time.Millisecond):
		}
	}
	return fmt.Errorf("XADD failed after retries")
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

// PublishStatus emits a platform:status event via Redis Pub/Sub (same
// channel/shape as youtube-listener's status publisher).
func (p *StreamPublisher) PublishStatus(ctx context.Context, msg models.StatusMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal status: %w", err)
	}
	return p.redis.Publish(ctx, platformStatusChannel, string(data)).Err()
}

// Stop drains nothing (no ring buffer here); kept for shutdown symmetry with
// the other listeners.
func (p *StreamPublisher) Stop() {}

// IsHealthy checks the Redis connection.
func (p *StreamPublisher) IsHealthy(ctx context.Context) bool {
	if p.redis == nil {
		return true
	}
	_, err := p.redis.Ping(ctx).Result()
	return err == nil
}
