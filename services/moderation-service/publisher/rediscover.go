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

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// rediscoverChannel is the Redis Pub/Sub channel the youtube-listener-innertube
// subscribes to for owner-triggered stream re-discovery. Keep the channel name and
// the {action, overlay_id, channel_id} payload in sync with
// services/youtube-listener-innertube/cmd/main.go.
const rediscoverChannel = "youtube:control"

// rediscoverCooldown bounds how often a single YouTube channel may be force
// re-discovered, so a streamer mashing the /view button can't churn the poller or
// hammer YouTube's InnerTube endpoints. Enforced per-channel via a Redis SETNX TTL.
const rediscoverCooldown = 15 * time.Second

// RediscoverPublisher publishes owner-triggered "force re-discover" commands for a
// YouTube channel onto the listener's control channel, with a per-channel cooldown.
type RediscoverPublisher struct {
	client *redis.Client
	logger *zap.Logger
}

// NewRediscoverPublisher creates a RediscoverPublisher over the given Redis client.
func NewRediscoverPublisher(client *redis.Client, logger *zap.Logger) *RediscoverPublisher {
	return &RediscoverPublisher{client: client, logger: logger}
}

// Publish broadcasts a re-discover command for (overlayID, channelID). It returns
// published=false (without error) when the channel is within its cooldown window,
// so the caller can report a 429 rather than silently churning the stream.
func (p *RediscoverPublisher) Publish(ctx context.Context, overlayID, channelID string) (bool, error) {
	cooldownKey := fmt.Sprintf("youtube:control:cooldown:%s", channelID)
	acquired, err := p.client.SetNX(ctx, cooldownKey, "1", rediscoverCooldown).Result()
	if err != nil {
		return false, fmt.Errorf("rediscover cooldown check: %w", err)
	}
	if !acquired {
		return false, nil // within cooldown window
	}

	payload, err := json.Marshal(map[string]string{
		"action":     "rediscover",
		"overlay_id": overlayID,
		"channel_id": channelID,
	})
	if err != nil {
		return false, fmt.Errorf("marshal rediscover command: %w", err)
	}
	if err := p.client.Publish(ctx, rediscoverChannel, payload).Err(); err != nil {
		return false, fmt.Errorf("publish rediscover command: %w", err)
	}

	p.logger.Info("Published YouTube rediscover command",
		zap.String("overlay_id", overlayID),
		zap.String("channel_id", channelID),
	)
	return true, nil
}
