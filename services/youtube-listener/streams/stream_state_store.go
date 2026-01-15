package streams

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/youtube-listener/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// StreamState represents the active streaming state for a channel
// This allows new pods to immediately resume streaming without waiting for detection
type StreamState struct {
	ChannelID     string    `json:"channel_id"`
	StreamID      string    `json:"stream_id"`
	LiveChatID    string    `json:"live_chat_id"`
	ChannelName   string    `json:"channel_name"`
	OverlayID     string    `json:"overlay_id"`
	NextPageToken string    `json:"next_page_token,omitempty"`
	IsLive        bool      `json:"is_live"`
	LastUpdated   time.Time `json:"last_updated"`
}

// StreamStateStore handles persistent storage of active stream state in Redis
type StreamStateStore struct {
	redis  *redis.Client
	logger *zap.Logger
	ttl    time.Duration // 30 minutes default (longer than typical detection interval)
}

// NewStreamStateStore creates a new stream state store
func NewStreamStateStore(redis *redis.Client, logger *zap.Logger) *StreamStateStore {
	return &StreamStateStore{
		redis:  redis,
		logger: logger,
		ttl:    30 * time.Minute, // Keep state for 30 minutes
	}
}

// SaveStreamState persists active stream state to Redis
// This allows new pods to immediately resume streaming for this channel
func (s *StreamStateStore) SaveStreamState(ctx context.Context, stream *models.YouTubeStream) error {
	key := fmt.Sprintf("youtube:stream:state:%s", stream.ChannelID)

	state := &StreamState{
		ChannelID:     stream.ChannelID,
		StreamID:      stream.StreamID,
		LiveChatID:    stream.LiveChatID,
		ChannelName:   stream.ChannelName,
		OverlayID:     stream.OverlayID,
		NextPageToken: stream.NextPageToken,
		IsLive:        true,
		LastUpdated:   time.Now(),
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal stream state: %w", err)
	}

	err = s.redis.Set(ctx, key, data, s.ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to save stream state: %w", err)
	}

	s.logger.Debug("Saved stream state to Redis",
		zap.String("channel_id", stream.ChannelID),
		zap.String("stream_id", stream.StreamID),
		zap.String("live_chat_id", stream.LiveChatID),
	)

	return nil
}

// LoadStreamState retrieves active stream state from Redis
// Returns nil if no state exists (channel not currently streaming)
func (s *StreamStateStore) LoadStreamState(ctx context.Context, channelID string) (*StreamState, error) {
	key := fmt.Sprintf("youtube:stream:state:%s", channelID)

	data, err := s.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		// No state exists - channel not currently streaming
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load stream state: %w", err)
	}

	var state StreamState
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stream state: %w", err)
	}

	// Check if state is stale (older than 30 minutes)
	if time.Since(state.LastUpdated) > 30*time.Minute {
		s.logger.Warn("Stream state is stale, ignoring",
			zap.String("channel_id", channelID),
			zap.Duration("age", time.Since(state.LastUpdated)),
		)
		// Delete stale state
		s.redis.Del(ctx, key)
		return nil, nil
	}

	s.logger.Debug("Loaded stream state from Redis",
		zap.String("channel_id", channelID),
		zap.String("stream_id", state.StreamID),
		zap.Bool("is_live", state.IsLive),
	)

	return &state, nil
}

// ClearStreamState removes stream state (called when stream ends or overlay disconnects)
func (s *StreamStateStore) ClearStreamState(ctx context.Context, channelID string) error {
	key := fmt.Sprintf("youtube:stream:state:%s", channelID)

	err := s.redis.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to clear stream state: %w", err)
	}

	s.logger.Debug("Cleared stream state",
		zap.String("channel_id", channelID),
	)

	return nil
}

// RefreshTTL extends the TTL for an active stream
// Should be called periodically while stream is active
func (s *StreamStateStore) RefreshTTL(ctx context.Context, channelID string) error {
	key := fmt.Sprintf("youtube:stream:state:%s", channelID)

	err := s.redis.Expire(ctx, key, s.ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to refresh stream state TTL: %w", err)
	}

	return nil
}
