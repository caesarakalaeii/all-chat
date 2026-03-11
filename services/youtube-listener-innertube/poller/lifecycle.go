package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener-innertube/innertube"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// LifecyclePublisher publishes stream end events to Redis.
// Defined as interface for testability.
type LifecyclePublisher interface {
	Publish(ctx context.Context, channel string, message interface{}) *redis.IntCmd
}

// StreamEndPayload is the JSON payload published to lifecycle:stream_end.
type StreamEndPayload struct {
	Platform      string `json:"platform"`
	UserID        string `json:"user_id"`        // may be "" — share-service resolves via google_id lookup
	BroadcasterID string `json:"broadcaster_id"` // YouTube channel_id
	Timestamp     string `json:"timestamp"`
}

// Repository handles Redis operations for channel-video mappings
// Used for lifecycle management: storing/clearing video IDs when streams go offline
type Repository struct {
	client *redis.Client
	logger *zap.Logger
}

// NewRepository creates a new Redis repository for lifecycle operations
func NewRepository(client *redis.Client, logger *zap.Logger) *Repository {
	return &Repository{
		client: client,
		logger: logger,
	}
}

// DeleteChannelVideoMapping removes the cached video ID for a channel
// This forces rediscovery on the next poll attempt
func (r *Repository) DeleteChannelVideoMapping(ctx context.Context, channelID string) error {
	if r.client == nil {
		return fmt.Errorf("redis client is nil")
	}

	key := fmt.Sprintf("youtube:innertube:channel:%s:video_id", channelID)

	err := r.client.Del(ctx, key).Err()
	if err != nil {
		r.logger.Error("Failed to delete channel video mapping",
			zap.String("channel_id", channelID),
			zap.String("key", key),
			zap.Error(err),
		)
		return fmt.Errorf("failed to delete channel video mapping: %w", err)
	}

	r.logger.Debug("Deleted channel video mapping",
		zap.String("channel_id", channelID),
		zap.String("key", key),
	)

	return nil
}

// GetChannelVideoMapping retrieves the cached video ID for a channel
func (r *Repository) GetChannelVideoMapping(ctx context.Context, channelID string) (string, error) {
	key := fmt.Sprintf("youtube:innertube:channel:%s:video_id", channelID)

	videoID, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		// No mapping exists (not an error)
		return "", nil
	}
	if err != nil {
		r.logger.Error("Failed to get channel video mapping",
			zap.String("channel_id", channelID),
			zap.String("key", key),
			zap.Error(err),
		)
		return "", fmt.Errorf("failed to get channel video mapping: %w", err)
	}

	return videoID, nil
}

// SetChannelVideoMapping stores the video ID for a channel
// TTL: 24 hours (stream discovery cache)
func (r *Repository) SetChannelVideoMapping(ctx context.Context, channelID, videoID string) error {
	key := fmt.Sprintf("youtube:innertube:channel:%s:video_id", channelID)

	err := r.client.Set(ctx, key, videoID, 24*time.Hour).Err()
	if err != nil {
		r.logger.Error("Failed to set channel video mapping",
			zap.String("channel_id", channelID),
			zap.String("video_id", videoID),
			zap.String("key", key),
			zap.Error(err),
		)
		return fmt.Errorf("failed to set channel video mapping: %w", err)
	}

	r.logger.Debug("Set channel video mapping",
		zap.String("channel_id", channelID),
		zap.String("video_id", videoID),
		zap.String("key", key),
	)

	return nil
}

// DetectOffline checks if a live chat response indicates the stream has ended
// Detection method: empty continuations array means stream is offline
func DetectOffline(resp *innertube.LiveChatResponse) bool {
	if resp == nil {
		return true
	}

	// Check if continuations array is empty
	// Empty continuations = no more data to fetch = stream ended
	if resp.ContinuationContents.LiveChatContinuation.Continuations == nil ||
		len(resp.ContinuationContents.LiveChatContinuation.Continuations) == 0 {
		return true
	}

	return false
}

// HandleStreamOffline handles the stream offline event.
// Actions:
//  1. Log offline detection
//  2. Delete Redis mapping to force rediscovery
//  3. Publish lifecycle event to Redis (if publisher is non-nil)
//  4. Return error to signal manager to stop polling
func HandleStreamOffline(ctx context.Context, channelID string, videoID string, repository *Repository, publisher LifecyclePublisher, logger *zap.Logger) error {
	logger.Info("Stream went offline",
		zap.String("channel_id", channelID),
		zap.String("video_id", videoID),
	)

	// Delete Redis mapping to force rediscovery on next poll
	if err := repository.DeleteChannelVideoMapping(ctx, channelID); err != nil {
		logger.Warn("Failed to delete channel mapping (non-fatal)",
			zap.String("channel_id", channelID),
			zap.Error(err),
		)
	}

	// Publish lifecycle event if publisher available
	if publisher != nil {
		payload := StreamEndPayload{
			Platform:      "youtube",
			UserID:        "",        // share-service resolves via google_id lookup
			BroadcasterID: channelID, // YouTube channel_id = google_id in users table
			Timestamp:     time.Now().UTC().Format(time.RFC3339),
		}
		data, err := json.Marshal(payload)
		if err != nil {
			logger.Warn("Failed to marshal lifecycle event", zap.Error(err))
		} else {
			if err := publisher.Publish(ctx, "lifecycle:stream_end", string(data)).Err(); err != nil {
				logger.Warn("Failed to publish lifecycle event",
					zap.String("channel_id", channelID),
					zap.Error(err))
			} else {
				logger.Info("Published YouTube stream end lifecycle event",
					zap.String("channel_id", channelID))
			}
		}
	}

	// Return error to signal polling should stop
	return fmt.Errorf("stream offline for channel %s", channelID)
}

// Discovery handles stream discovery for auto-resume
type Discovery struct {
	client ClientInterface
	logger *zap.Logger
}

// NewDiscovery creates a new discovery helper
func NewDiscovery(client ClientInterface, logger *zap.Logger) *Discovery {
	return &Discovery{
		client: client,
		logger: logger,
	}
}

// DiscoverStream attempts to discover if a channel is currently live
// Returns video ID if live, empty string if offline, error on failure
func (d *Discovery) DiscoverStream(ctx context.Context, channelID string) (string, error) {
	// For Phase 10 PoC: simplified discovery logic
	// Production implementation (Phase 11+) would use YouTube Data API search.list
	// or InnerTube browse endpoint to discover active streams

	d.logger.Debug("Attempting stream discovery",
		zap.String("channel_id", channelID),
	)

	// Placeholder: Real implementation would call YouTube API
	// For now, return empty string to indicate offline (auto-resume not yet implemented)
	return "", nil
}

// StartDiscoveryLoop runs a background loop to discover new streams after one ends
// Discovery strategy: exponential backoff polling (1m → 2m → 5m → 10m max)
// Continues until:
//   - New stream discovered (success)
//   - Context cancelled (shutdown)
//   - All overlays disconnected (no longer needed)
func StartDiscoveryLoop(
	ctx context.Context,
	channelID string,
	discovery *Discovery,
	repository *Repository,
	logger *zap.Logger,
) error {
	logger.Info("Starting discovery loop for channel",
		zap.String("channel_id", channelID),
	)

	// Exponential backoff intervals: 1m → 2m → 5m → 10m (max)
	intervals := []time.Duration{
		1 * time.Minute,
		2 * time.Minute,
		5 * time.Minute,
		10 * time.Minute,
	}
	intervalIndex := 0

	for {
		// Use current backoff interval
		currentInterval := intervals[intervalIndex]
		if intervalIndex < len(intervals)-1 {
			intervalIndex++
		}

		logger.Debug("Discovery loop sleeping",
			zap.String("channel_id", channelID),
			zap.Duration("interval", currentInterval),
		)

		// Wait for next discovery attempt
		select {
		case <-ctx.Done():
			logger.Info("Discovery loop cancelled",
				zap.String("channel_id", channelID),
			)
			return ctx.Err()
		case <-time.After(currentInterval):
			// Proceed to discovery attempt
		}

		// Attempt to discover stream
		videoID, err := discovery.DiscoverStream(ctx, channelID)
		if err != nil {
			logger.Warn("Discovery attempt failed",
				zap.String("channel_id", channelID),
				zap.Error(err),
			)
			// Continue backoff loop
			continue
		}

		// Check if stream found
		if videoID == "" {
			logger.Debug("No stream found yet",
				zap.String("channel_id", channelID),
			)
			// Continue backoff loop
			continue
		}

		// Stream discovered! Cache it and return
		logger.Info("Stream discovered",
			zap.String("channel_id", channelID),
			zap.String("video_id", videoID),
		)

		if err := repository.SetChannelVideoMapping(ctx, channelID, videoID); err != nil {
			logger.Warn("Failed to cache discovered video ID",
				zap.String("channel_id", channelID),
				zap.String("video_id", videoID),
				zap.Error(err),
			)
		}

		return nil // Success - stream found
	}
}
