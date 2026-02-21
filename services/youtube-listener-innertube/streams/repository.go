package streams

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

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
