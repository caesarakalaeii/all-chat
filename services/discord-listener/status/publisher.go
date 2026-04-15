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

const PlatformStatusChannel = "platform:status"

// Message represents a platform connection status update
type Message struct {
	Platform     string     `json:"platform"`
	ChannelID    string     `json:"channel_id"`
	ChannelName  string     `json:"channel_name,omitempty"`
	Status       string     `json:"status"` // "connected", "reconnecting", "offline"
	NextRetryAt  *time.Time `json:"next_retry_at,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
}

// Publisher publishes platform status updates to Redis Pub/Sub
type Publisher struct {
	redisClient *redis.Client
	logger      *zap.Logger
}

func NewPublisher(redisClient *redis.Client, logger *zap.Logger) *Publisher {
	return &Publisher{redisClient: redisClient, logger: logger}
}

func (p *Publisher) Publish(ctx context.Context, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		p.logger.Error("Failed to marshal status message", zap.Error(err))
		return
	}
	if err := p.redisClient.Publish(ctx, PlatformStatusChannel, string(data)).Err(); err != nil {
		p.logger.Warn("Failed to publish platform status", zap.String("status", msg.Status), zap.Error(err))
	}
}
