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

package streams

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// streamStateTTL bounds how long the youtube:stream:state live-chat-id cache survives
// without a refresh. The manager re-publishes it on a heartbeat (well within this window)
// while the stream is actively polled, so a stream that ends — or a pod that dies — drops
// out of the consumers' view within this window even if explicit cleanup is missed.
const streamStateTTL = 2 * time.Minute

// Repository handles persistence of channel→video mappings in Redis
type Repository struct {
	redisClient *redis.Client
	logger      *zap.Logger
}

// NewRepository creates a new Repository instance
func NewRepository(redisClient *redis.Client, logger *zap.Logger) *Repository {
	return &Repository{
		redisClient: redisClient,
		logger:      logger,
	}
}

// SetChannelVideoMapping persists a channel→video mapping in Redis with 24-hour TTL
// Key format: innertube:channel_video:{channelID}
// TTL ensures automatic cleanup when streams end
func (r *Repository) SetChannelVideoMapping(ctx context.Context, channelID, videoID string) error {
	key := fmt.Sprintf("innertube:channel_video:%s", channelID)
	ttl := 24 * time.Hour

	err := r.redisClient.Set(ctx, key, videoID, ttl).Err()
	if err != nil {
		r.logger.Error("failed to set channel video mapping",
			zap.String("channel_id", channelID),
			zap.String("video_id", videoID),
			zap.Error(err),
		)
		return fmt.Errorf("set channel video mapping: %w", err)
	}

	r.logger.Info("persisted channel video mapping",
		zap.String("channel_id", channelID),
		zap.String("video_id", videoID),
		zap.Duration("ttl", ttl),
	)

	return nil
}

// GetChannelVideoMapping retrieves the video ID for a given channel ID
// Returns empty string and redis.Nil error if mapping doesn't exist
func (r *Repository) GetChannelVideoMapping(ctx context.Context, channelID string) (string, error) {
	key := fmt.Sprintf("innertube:channel_video:%s", channelID)

	videoID, err := r.redisClient.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			r.logger.Debug("no channel video mapping found",
				zap.String("channel_id", channelID),
			)
			return "", err
		}

		r.logger.Error("failed to get channel video mapping",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return "", fmt.Errorf("get channel video mapping: %w", err)
	}

	r.logger.Debug("retrieved channel video mapping",
		zap.String("channel_id", channelID),
		zap.String("video_id", videoID),
	)

	return videoID, nil
}

// SetStreamState publishes the youtube:stream:state:<channelID> cache entry that
// auth-service (streamer chat send) and moderation-service (mod actions) read to resolve a
// channel's official live chat. is_live is always true here — the entry exists only while
// we are actively polling a live stream; its absence means "not live". TTL'd so a missed
// cleanup self-heals (see streamStateTTL). See shared contract in streams/livechat.go.
func (r *Repository) SetStreamState(ctx context.Context, channelID, videoID, overlayID, liveChatID string) error {
	key := fmt.Sprintf("youtube:stream:state:%s", channelID)
	state := StreamState{
		ChannelID:   channelID,
		StreamID:    videoID,
		LiveChatID:  liveChatID,
		OverlayID:   overlayID,
		IsLive:      true,
		LastUpdated: time.Now(),
	}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshal stream state: %w", err)
	}
	if err := r.redisClient.Set(ctx, key, data, streamStateTTL).Err(); err != nil {
		r.logger.Warn("failed to set stream state",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return fmt.Errorf("set stream state: %w", err)
	}
	return nil
}

// DeleteStreamState removes the youtube:stream:state live-chat-id cache for a channel,
// called when a stream ends so a streamer send no longer targets a dead live chat.
func (r *Repository) DeleteStreamState(ctx context.Context, channelID string) error {
	key := fmt.Sprintf("youtube:stream:state:%s", channelID)
	if err := r.redisClient.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("delete stream state: %w", err)
	}
	return nil
}

// DeleteChannelVideoMapping removes a channel→video mapping from Redis
// Used when stream ends to force rediscovery on next activation
func (r *Repository) DeleteChannelVideoMapping(ctx context.Context, channelID string) error {
	key := fmt.Sprintf("innertube:channel_video:%s", channelID)

	err := r.redisClient.Del(ctx, key).Err()
	if err != nil {
		r.logger.Error("failed to delete channel video mapping",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
		return fmt.Errorf("delete channel video mapping: %w", err)
	}

	r.logger.Info("deleted channel video mapping",
		zap.String("channel_id", channelID),
	)

	return nil
}
