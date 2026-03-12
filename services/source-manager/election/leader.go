package election

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/source-manager/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// LeaderKeyPrefix is the prefix for leader election keys in Redis
	LeaderKeyPrefix = "leader"

	// DefaultLockTTL is the default TTL for leader locks (10 seconds)
	DefaultLockTTL = 10 * time.Second

	// DefaultHeartbeatInterval is how often to renew the lock (5 seconds)
	DefaultHeartbeatInterval = 5 * time.Second
)

// Manager handles leader election using Redis distributed locks
type Manager struct {
	client     *redis.Client
	instanceID string
	lockTTL    time.Duration
	logger     *zap.Logger
}

// NewManager creates a new leader election manager
func NewManager(client *redis.Client, logger *zap.Logger) *Manager {
	return &Manager{
		client:     client,
		instanceID: uuid.New().String(),
		lockTTL:    DefaultLockTTL,
		logger:     logger,
	}
}

// GetInstanceID returns the current instance ID
func (m *Manager) GetInstanceID() string {
	return m.instanceID
}

// TryAcquireLeadership attempts to acquire leadership for a stream.
// callerID is the stable identity of the requesting service instance; it is stored
// in Redis so that renew/release from the same caller succeed regardless of which
// source-manager pod handles each request.
func (m *Manager) TryAcquireLeadership(ctx context.Context, platform, streamID, callerID string) (bool, error) {
	key := m.leaderKey(platform, streamID)
	if callerID == "" {
		callerID = m.instanceID
	}

	// Try to set the key with NX (only if not exists) and EX (expiry)
	success, err := m.client.SetNX(ctx, key, callerID, m.lockTTL).Result()
	if err != nil {
		m.logger.Error("Failed to acquire leadership",
			zap.String("platform", platform),
			zap.String("stream_id", streamID),
			zap.Error(err),
		)
		return false, fmt.Errorf("failed to acquire leadership: %w", err)
	}

	if success {
		m.logger.Info("Acquired leadership",
			zap.String("platform", platform),
			zap.String("stream_id", streamID),
			zap.String("caller_id", callerID),
		)
	}

	return success, nil
}

// RenewLeadership renews the leadership lock (heartbeat)
// Returns true if renewal was successful, false if lost leadership
func (m *Manager) RenewLeadership(ctx context.Context, platform, streamID, callerID string) (bool, error) {
	key := m.leaderKey(platform, streamID)
	if callerID == "" {
		callerID = m.instanceID
	}

	// Check if we are still the leader
	currentLeader, err := m.client.Get(ctx, key).Result()
	if err == redis.Nil {
		// Lock expired
		return false, nil
	}
	if err != nil {
		m.logger.Error("Failed to check leadership",
			zap.String("platform", platform),
			zap.String("stream_id", streamID),
			zap.Error(err),
		)
		return false, fmt.Errorf("failed to check leadership: %w", err)
	}

	if currentLeader != callerID {
		// Someone else is leader
		m.logger.Warn("Lost leadership",
			zap.String("platform", platform),
			zap.String("stream_id", streamID),
			zap.String("current_leader", currentLeader),
			zap.String("caller_id", callerID),
		)
		return false, nil
	}

	// Renew the lock
	err = m.client.Expire(ctx, key, m.lockTTL).Err()
	if err != nil {
		m.logger.Error("Failed to renew leadership",
			zap.String("platform", platform),
			zap.String("stream_id", streamID),
			zap.Error(err),
		)
		return false, fmt.Errorf("failed to renew leadership: %w", err)
	}

	m.logger.Debug("Renewed leadership",
		zap.String("platform", platform),
		zap.String("stream_id", streamID),
	)

	return true, nil
}

// ReleaseLeadership releases leadership for a stream
func (m *Manager) ReleaseLeadership(ctx context.Context, platform, streamID, callerID string) error {
	key := m.leaderKey(platform, streamID)
	if callerID == "" {
		callerID = m.instanceID
	}

	// Only delete if we are the leader (using Lua script for atomicity)
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`)

	result, err := script.Run(ctx, m.client, []string{key}, callerID).Result()
	if err != nil {
		m.logger.Error("Failed to release leadership",
			zap.String("platform", platform),
			zap.String("stream_id", streamID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to release leadership: %w", err)
	}

	if result.(int64) == 1 {
		m.logger.Info("Released leadership",
			zap.String("platform", platform),
			zap.String("stream_id", streamID),
		)
	}

	return nil
}

// GetLeadership returns the current leadership status for a stream
func (m *Manager) GetLeadership(ctx context.Context, platform, streamID string) (*models.LeadershipStatus, error) {
	key := m.leaderKey(platform, streamID)

	// Get current leader
	currentLeader, err := m.client.Get(ctx, key).Result()
	if err == redis.Nil {
		// No leader
		return &models.LeadershipStatus{
			StreamID:   streamID,
			Platform:   platform,
			LeaderID:   "",
			AcquiredAt: time.Time{},
			ExpiresAt:  time.Time{},
			IsLeader:   false,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get leadership status: %w", err)
	}

	// Get TTL
	ttl, err := m.client.TTL(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get TTL: %w", err)
	}

	isLeader := currentLeader == m.instanceID
	expiresAt := time.Now().Add(ttl)
	acquiredAt := expiresAt.Add(-m.lockTTL)

	return &models.LeadershipStatus{
		StreamID:   streamID,
		Platform:   platform,
		LeaderID:   currentLeader,
		AcquiredAt: acquiredAt,
		ExpiresAt:  expiresAt,
		IsLeader:   isLeader,
	}, nil
}

// GetAllLeadership returns leadership status for all streams
func (m *Manager) GetAllLeadership(ctx context.Context) ([]*models.LeadershipStatus, error) {
	// Scan for all leader keys
	pattern := fmt.Sprintf("%s:*:*", LeaderKeyPrefix)
	iter := m.client.Scan(ctx, 0, pattern, 0).Iterator()

	statuses := make([]*models.LeadershipStatus, 0)

	for iter.Next(ctx) {
		key := iter.Val()

		// Parse platform and stream ID from key
		// Key format: leader:{platform}:{stream_id}
		platform, streamID, err := m.parseLeaderKey(key)
		if err != nil {
			m.logger.Warn("Failed to parse leader key", zap.String("key", key))
			continue
		}

		status, err := m.GetLeadership(ctx, platform, streamID)
		if err != nil {
			m.logger.Warn("Failed to get leadership status",
				zap.String("platform", platform),
				zap.String("stream_id", streamID),
				zap.Error(err),
			)
			continue
		}

		statuses = append(statuses, status)
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan leader keys: %w", err)
	}

	return statuses, nil
}

// leaderKey generates the Redis key for a leader lock
func (m *Manager) leaderKey(platform, streamID string) string {
	return fmt.Sprintf("%s:%s:%s", LeaderKeyPrefix, platform, streamID)
}

// parseLeaderKey parses a leader key into platform and stream ID
func (m *Manager) parseLeaderKey(key string) (platform, streamID string, err error) {
	// Key format: leader:{platform}:{stream_id}
	parts := strings.Split(key, ":")
	if len(parts) != 3 || parts[0] != LeaderKeyPrefix {
		return "", "", fmt.Errorf("invalid leader key format: %s", key)
	}
	return parts[1], parts[2], nil
}
