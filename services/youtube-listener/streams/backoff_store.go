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

// BackoffState represents persistent backoff state for a channel
type BackoffState struct {
	LastCheckTime      time.Time     `json:"last_check_time"`
	CurrentInterval    time.Duration `json:"current_interval"`
	FailureCount       int           `json:"failure_count"`
	LastSeenLive       time.Time     `json:"last_seen_live,omitempty"`
	ConsecutiveOffline int           `json:"consecutive_offline"`
}

// BackoffStore handles persistent storage of backoff state in Redis
type BackoffStore struct {
	redis  *redis.Client
	logger *zap.Logger
	ttl    time.Duration // 24 hours default
}

// NewBackoffStore creates a new backoff store
func NewBackoffStore(redis *redis.Client, logger *zap.Logger) *BackoffStore {
	return &BackoffStore{
		redis:  redis,
		logger: logger,
		ttl:    24 * time.Hour,
	}
}

// SaveBackoffState persists backoff state to Redis
func (s *BackoffStore) SaveBackoffState(ctx context.Context, channelID string, state *BackoffState) error {
	key := fmt.Sprintf("youtube:backoff:%s", channelID)

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal backoff state: %w", err)
	}

	err = s.redis.Set(ctx, key, data, s.ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to save backoff state: %w", err)
	}

	s.logger.Debug("Saved backoff state to Redis",
		zap.String("channel_id", channelID),
		zap.Duration("current_interval", state.CurrentInterval),
		zap.Int("failure_count", state.FailureCount),
		zap.Int("consecutive_offline", state.ConsecutiveOffline),
	)

	return nil
}

// LoadBackoffState retrieves backoff state from Redis
func (s *BackoffStore) LoadBackoffState(ctx context.Context, channelID string) (*BackoffState, error) {
	key := fmt.Sprintf("youtube:backoff:%s", channelID)

	data, err := s.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		// No state exists - return nil (not an error)
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load backoff state: %w", err)
	}

	var state BackoffState
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal backoff state: %w", err)
	}

	s.logger.Debug("Loaded backoff state from Redis",
		zap.String("channel_id", channelID),
		zap.Duration("current_interval", state.CurrentInterval),
		zap.Int("failure_count", state.FailureCount),
	)

	return &state, nil
}

// SetNegativeCache marks channel as offline for TTL duration
func (s *BackoffStore) SetNegativeCache(ctx context.Context, channelID string, consecutiveOffline int) error {
	key := fmt.Sprintf("youtube:negative:%s", channelID)
	ttl := s.calculateNegativeCacheTTL(consecutiveOffline)

	// Don't cache if TTL is zero
	if ttl == 0 {
		return nil
	}

	err := s.redis.Set(ctx, key, "offline", ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set negative cache: %w", err)
	}

	s.logger.Debug("Set negative cache",
		zap.String("channel_id", channelID),
		zap.Duration("ttl", ttl),
		zap.Int("consecutive_offline", consecutiveOffline),
	)

	return nil
}

// IsNegativeCached checks if channel is in negative cache
func (s *BackoffStore) IsNegativeCached(ctx context.Context, channelID string) (bool, error) {
	key := fmt.Sprintf("youtube:negative:%s", channelID)

	exists, err := s.redis.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check negative cache: %w", err)
	}

	return exists > 0, nil
}

// ClearBackoff removes backoff state (called when stream detected or manually reset)
func (s *BackoffStore) ClearBackoff(ctx context.Context, channelID string) error {
	backoffKey := fmt.Sprintf("youtube:backoff:%s", channelID)
	negativeKey := fmt.Sprintf("youtube:negative:%s", channelID)

	pipe := s.redis.Pipeline()
	pipe.Del(ctx, backoffKey)
	pipe.Del(ctx, negativeKey)
	_, err := pipe.Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to clear backoff state: %w", err)
	}

	s.logger.Debug("Cleared backoff and negative cache",
		zap.String("channel_id", channelID),
	)

	return nil
}

// calculateNegativeCacheTTL returns appropriate TTL based on consecutive offline checks
// Reduced TTL values to allow faster recovery when channels go live
func (s *BackoffStore) calculateNegativeCacheTTL(consecutiveOffline int) time.Duration {
	switch {
	case consecutiveOffline < 2:
		return 0 // No caching for first offline
	case consecutiveOffline < 4:
		return 2 * time.Minute // Was 5 min → now 2 min
	case consecutiveOffline < 7:
		return 5 * time.Minute // Was 15 min → now 5 min
	default:
		return 10 * time.Minute // Was 30 min → now 10 min
	}
}
