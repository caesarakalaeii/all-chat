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

package status

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// Redis channel for platform status updates
	PlatformStatusChannel = "platform:status"
)

// StatusMessage represents a platform connection status message
type StatusMessage struct {
	Platform     string     `json:"platform"`                  // "youtube", "twitch", "kick", "tiktok"
	ChannelID    string     `json:"channel_id"`                // Platform-specific channel identifier
	ChannelName  string     `json:"channel_name,omitempty"`    // Human-readable channel name
	Status       string     `json:"status"`                    // "connected", "reconnecting", "offline", "quota_exceeded"
	NextRetryAt  *time.Time `json:"next_retry_at,omitempty"`   // Timestamp when next reconnection happens (nil if connected)
	ErrorMessage string     `json:"error_message,omitempty"`   // Human-readable error
}

// Publisher publishes platform status updates to Redis
type Publisher struct {
	redisClient *redis.Client
	logger      *zap.Logger
}

// NewPublisher creates a new status publisher
func NewPublisher(redisClient *redis.Client, logger *zap.Logger) *Publisher {
	return &Publisher{
		redisClient: redisClient,
		logger:      logger,
	}
}

// PublishStatus publishes a status message to Redis
func (p *Publisher) PublishStatus(ctx context.Context, msg StatusMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		p.logger.Error("Failed to marshal status message",
			zap.String("platform", msg.Platform),
			zap.String("channel_id", msg.ChannelID),
			zap.Error(err))
		return err
	}

	if err := p.redisClient.Publish(ctx, PlatformStatusChannel, string(data)).Err(); err != nil {
		p.logger.Error("Failed to publish status to Redis",
			zap.String("platform", msg.Platform),
			zap.String("channel_id", msg.ChannelID),
			zap.String("status", msg.Status),
			zap.Error(err))
		return err
	}

	p.logger.Debug("Published platform status",
		zap.String("platform", msg.Platform),
		zap.String("channel_id", msg.ChannelID),
		zap.String("status", msg.Status))

	return nil
}
