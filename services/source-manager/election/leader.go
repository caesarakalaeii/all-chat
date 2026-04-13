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

	// PeerKeyPrefix is the prefix for peer registration keys in Redis
	PeerKeyPrefix = "peer"

	// PeerTTL is the TTL for peer registration keys (30 seconds, matches sync interval)
	PeerTTL = 30 * time.Second
)

// renewScript atomically checks whether the caller owns the lock and renews its TTL.
// Returns 1 if renewed, 0 if the caller does not own the lock or the key has expired.
// This eliminates the TOCTOU window between GET and EXPIRE in the previous implementation.
var renewScript = redis.NewScript(`
	if redis.call("get", KEYS[1]) == ARGV[1] then
		return redis.call("expire", KEYS[1], ARGV[2])
	else
		return 0
	end
`)

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

// RenewLeadership renews the leadership lock (heartbeat).
// Returns true if renewal was successful, false if lost leadership.
// Uses a Lua script for atomic ownership check + TTL renewal, eliminating the
// TOCTOU race window that existed between the previous GET and EXPIRE calls.
func (m *Manager) RenewLeadership(ctx context.Context, platform, streamID, callerID string) (bool, error) {
	key := m.leaderKey(platform, streamID)
	if callerID == "" {
		callerID = m.instanceID
	}

	result, err := renewScript.Run(ctx, m.client, []string{key},
		callerID, int(m.lockTTL.Seconds())).Int()
	if err != nil {
		m.logger.Error("Failed to renew leadership",
			zap.String("platform", platform),
			zap.String("stream_id", streamID),
			zap.Error(err),
		)
		return false, fmt.Errorf("failed to renew leadership: %w", err)
	}

	if result == 1 {
		m.logger.Debug("Renewed leadership",
			zap.String("platform", platform),
			zap.String("stream_id", streamID),
		)
	} else {
		m.logger.Warn("Lost leadership",
			zap.String("platform", platform),
			zap.String("stream_id", streamID),
			zap.String("caller_id", callerID),
		)
	}

	return result == 1, nil
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

// RegisterPeer registers a caller as an active peer for the given platform and returns
// the total number of active peers. Uses a Redis sorted set where each member is a
// callerID and its score is the Unix expiry timestamp. This gives O(log N) registration
// and O(1) count via ZCARD, replacing the previous O(N) SCAN approach.
func (m *Manager) RegisterPeer(ctx context.Context, platform, callerID string) (int, error) {
	key := fmt.Sprintf("peers:%s", platform)
	expiry := float64(time.Now().Add(PeerTTL).Unix())

	// ZADD: add or update the member's expiry score (overwrites existing member on re-registration)
	if err := m.client.ZAdd(ctx, key, redis.Z{
		Score:  expiry,
		Member: callerID,
	}).Err(); err != nil {
		return 0, fmt.Errorf("failed to register peer: %w", err)
	}

	// Remove expired members (score < current Unix time)
	now := fmt.Sprintf("%d", time.Now().Unix())
	m.client.ZRemRangeByScore(ctx, key, "-inf", now)

	// Count remaining active members (O(1))
	count, err := m.client.ZCard(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to count peers: %w", err)
	}

	// Set key TTL to prevent orphaned sorted sets (2x PeerTTL)
	m.client.Expire(ctx, key, PeerTTL*2)

	return int(count), nil
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
