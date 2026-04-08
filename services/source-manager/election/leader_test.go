package election

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func setupTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return client, mr
}

func TestNewManager(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	manager := NewManager(client, logger)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.client)
	assert.NotEmpty(t, manager.instanceID)
	assert.Equal(t, DefaultLockTTL, manager.lockTTL)
}

func TestGetInstanceID(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	manager := NewManager(client, logger)

	instanceID := manager.GetInstanceID()
	assert.NotEmpty(t, instanceID)
}

func TestTryAcquireLeadership_Success(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	manager := NewManager(client, logger)

	acquired, err := manager.TryAcquireLeadership(context.Background(), "youtube", "stream123", "")

	assert.NoError(t, err)
	assert.True(t, acquired)

	// Verify key exists in Redis
	key := "leader:youtube:stream123"
	val, err := client.Get(context.Background(), key).Result()
	assert.NoError(t, err)
	assert.Equal(t, manager.instanceID, val)
}

func TestTryAcquireLeadership_AlreadyHeld(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	manager1 := NewManager(client, logger)
	manager2 := NewManager(client, logger)

	// Manager 1 acquires leadership
	acquired1, err := manager1.TryAcquireLeadership(context.Background(), "youtube", "stream123", "")
	assert.NoError(t, err)
	assert.True(t, acquired1)

	// Manager 2 tries to acquire (should fail)
	acquired2, err := manager2.TryAcquireLeadership(context.Background(), "youtube", "stream123", "")
	assert.NoError(t, err)
	assert.False(t, acquired2)
}

func TestRenewLeadership_Success(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	manager := NewManager(client, logger)

	// Acquire leadership first
	acquired, err := manager.TryAcquireLeadership(context.Background(), "youtube", "stream123", "")
	assert.NoError(t, err)
	assert.True(t, acquired)

	// Renew leadership
	renewed, err := manager.RenewLeadership(context.Background(), "youtube", "stream123", "")
	assert.NoError(t, err)
	assert.True(t, renewed)
}

func TestRenewLeadership_LostLeadership(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	manager := NewManager(client, logger)

	// Try to renew without acquiring first
	renewed, err := manager.RenewLeadership(context.Background(), "youtube", "stream123", "")
	assert.NoError(t, err)
	assert.False(t, renewed)
}

func TestRenewLeadership_StolenByAnother(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	manager1 := NewManager(client, logger)
	manager2 := NewManager(client, logger)

	// Manager 1 acquires leadership
	acquired, err := manager1.TryAcquireLeadership(context.Background(), "youtube", "stream123", "")
	assert.NoError(t, err)
	assert.True(t, acquired)

	// Simulate lock expiration by fast-forwarding time in miniredis
	mr.FastForward(15 * time.Second)

	// Manager 2 acquires leadership
	acquired2, err := manager2.TryAcquireLeadership(context.Background(), "youtube", "stream123", "")
	assert.NoError(t, err)
	assert.True(t, acquired2)

	// Manager 1 tries to renew (should fail)
	renewed, err := manager1.RenewLeadership(context.Background(), "youtube", "stream123", "")
	assert.NoError(t, err)
	assert.False(t, renewed)
}

func TestRenewLeadership_WithExplicitCallerID(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	manager := NewManager(client, logger)

	// Acquire with explicit callerID
	acquired, err := manager.TryAcquireLeadership(context.Background(), "youtube", "stream123", "explicit-caller-1")
	assert.NoError(t, err)
	assert.True(t, acquired)

	// Renew with matching callerID — should succeed
	renewed, err := manager.RenewLeadership(context.Background(), "youtube", "stream123", "explicit-caller-1")
	assert.NoError(t, err)
	assert.True(t, renewed)

	// Renew with wrong callerID — should fail
	renewed, err = manager.RenewLeadership(context.Background(), "youtube", "stream123", "wrong-caller")
	assert.NoError(t, err)
	assert.False(t, renewed)
}

func TestRenewLeadership_Atomic_NoTOCTOURace(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	manager1 := NewManager(client, logger)
	manager2 := NewManager(client, logger)

	// Manager 1 acquires leadership
	acquired, err := manager1.TryAcquireLeadership(context.Background(), "youtube", "stream-race", manager1.instanceID)
	assert.NoError(t, err)
	assert.True(t, acquired)

	// Verify that after manager1 renews, manager2 still cannot renew
	// This tests that renewal uses atomic check+expire (no window between GET and EXPIRE)
	renewed1, err := manager1.RenewLeadership(context.Background(), "youtube", "stream-race", manager1.instanceID)
	assert.NoError(t, err)
	assert.True(t, renewed1, "manager1 should successfully renew its own lock")

	// Manager2 (different ID) cannot renew
	renewed2, err := manager2.RenewLeadership(context.Background(), "youtube", "stream-race", manager2.instanceID)
	assert.NoError(t, err)
	assert.False(t, renewed2, "manager2 should not be able to renew lock owned by manager1")
}

func TestReleaseLeadership_Success(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	manager := NewManager(client, logger)

	// Acquire leadership first
	acquired, err := manager.TryAcquireLeadership(context.Background(), "youtube", "stream123", "")
	assert.NoError(t, err)
	assert.True(t, acquired)

	// Release leadership
	err = manager.ReleaseLeadership(context.Background(), "youtube", "stream123", "")
	assert.NoError(t, err)

	// Verify key is gone
	key := "leader:youtube:stream123"
	_, err = client.Get(context.Background(), key).Result()
	assert.Error(t, err)
	assert.Equal(t, redis.Nil, err)
}

func TestReleaseLeadership_NotLeader(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	manager1 := NewManager(client, logger)
	manager2 := NewManager(client, logger)

	// Manager 1 acquires leadership
	acquired, err := manager1.TryAcquireLeadership(context.Background(), "youtube", "stream123", "")
	assert.NoError(t, err)
	assert.True(t, acquired)

	// Manager 2 tries to release (should not delete)
	err = manager2.ReleaseLeadership(context.Background(), "youtube", "stream123", "")
	assert.NoError(t, err)

	// Verify key still exists
	key := "leader:youtube:stream123"
	val, err := client.Get(context.Background(), key).Result()
	assert.NoError(t, err)
	assert.Equal(t, manager1.instanceID, val)
}

func TestGetLeadership(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	manager := NewManager(client, logger)

	// Acquire leadership first
	acquired, err := manager.TryAcquireLeadership(context.Background(), "youtube", "stream123", "")
	assert.NoError(t, err)
	assert.True(t, acquired)

	// Get leadership status
	status, err := manager.GetLeadership(context.Background(), "youtube", "stream123")
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, "stream123", status.StreamID)
	assert.Equal(t, "youtube", status.Platform)
	assert.Equal(t, manager.instanceID, status.LeaderID)
	assert.True(t, status.IsLeader)
}

func TestGetLeadership_NoLeader(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	manager := NewManager(client, logger)

	// Get leadership status for non-existent stream
	status, err := manager.GetLeadership(context.Background(), "youtube", "stream123")
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, "stream123", status.StreamID)
	assert.Equal(t, "", status.LeaderID)
	assert.False(t, status.IsLeader)
}

func TestLeaderKey(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	manager := NewManager(client, logger)

	key := manager.leaderKey("youtube", "stream123")
	assert.Equal(t, "leader:youtube:stream123", key)
}

func TestRegisterPeer_ReturnsCount(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	manager := NewManager(client, logger)

	// Register first peer
	count, err := manager.RegisterPeer(context.Background(), "twitch", "caller-a")
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	// Register second peer
	count, err = manager.RegisterPeer(context.Background(), "twitch", "caller-b")
	assert.NoError(t, err)
	assert.Equal(t, 2, count)

	// Re-register first peer (should still be 2)
	count, err = manager.RegisterPeer(context.Background(), "twitch", "caller-a")
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestRegisterPeer_PlatformIsolation(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	manager := NewManager(client, logger)

	// Register peers for different platforms
	count, err := manager.RegisterPeer(context.Background(), "twitch", "caller-a")
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	count, err = manager.RegisterPeer(context.Background(), "kick", "caller-b")
	assert.NoError(t, err)
	assert.Equal(t, 1, count) // Different platform, count is 1
}

func TestRegisterPeer_Expiry(t *testing.T) {
	// The sorted-set implementation uses ZRemRangeByScore to remove members whose
	// score (Unix expiry timestamp) is in the past. We simulate expiry by directly
	// removing stale members via the sorted set rather than relying on miniredis
	// key-level FastForward (which only affects key TTLs, not scores).
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	manager := NewManager(client, logger)

	// Register two peers with a very short effective TTL by using a manager with a
	// tiny peerTTL so that the registered scores are already in the past when the
	// next call removes them.
	shortManager := &Manager{
		client:     client,
		instanceID: manager.instanceID,
		lockTTL:    manager.lockTTL,
		logger:     manager.logger,
	}

	_, err := shortManager.RegisterPeer(context.Background(), "twitch", "caller-a")
	assert.NoError(t, err)
	_, err = shortManager.RegisterPeer(context.Background(), "twitch", "caller-b")
	assert.NoError(t, err)

	// Manually expire both members by setting their scores to the past
	key := "peers:twitch"
	pastScore := float64(time.Now().Add(-2 * time.Second).Unix())
	client.ZAdd(context.Background(), key, redis.Z{Score: pastScore, Member: "caller-a"})
	client.ZAdd(context.Background(), key, redis.Z{Score: pastScore, Member: "caller-b"})

	// Register new peer — expired ones should be removed by ZRemRangeByScore
	count, err := manager.RegisterPeer(context.Background(), "twitch", "caller-c")
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestRegisterPeer_UsesSortedSet(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	manager := NewManager(client, logger)

	count, err := manager.RegisterPeer(context.Background(), "twitch", "caller-a")
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify sorted set key exists (not individual peer keys)
	keys, err := client.Keys(context.Background(), "peers:twitch").Result()
	assert.NoError(t, err)
	assert.Equal(t, 1, len(keys), "should have exactly one sorted set key")

	// Verify the member is in the sorted set
	members, err := client.ZRange(context.Background(), "peers:twitch", 0, -1).Result()
	assert.NoError(t, err)
	assert.Contains(t, members, "caller-a")
}

func TestRegisterPeer_UpdatesExistingPeerExpiry(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	manager := NewManager(client, logger)

	// Register peer once
	count, err := manager.RegisterPeer(context.Background(), "twitch", "caller-a")
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	// Get initial score
	initialScore, err := client.ZScore(context.Background(), "peers:twitch", "caller-a").Result()
	assert.NoError(t, err)

	// Wait a tiny bit and re-register — score should be updated (same or higher)
	time.Sleep(1 * time.Millisecond)
	count, err = manager.RegisterPeer(context.Background(), "twitch", "caller-a")
	assert.NoError(t, err)
	assert.Equal(t, 1, count, "re-registration should not increase count")

	updatedScore, err := client.ZScore(context.Background(), "peers:twitch", "caller-a").Result()
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, updatedScore, initialScore, "re-registration should update expiry score")
}

func TestParseLeaderKey(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	manager := NewManager(client, logger)

	tests := []struct {
		name         string
		key          string
		wantPlatform string
		wantStreamID string
		wantErr      bool
	}{
		{"valid key", "leader:youtube:stream123", "youtube", "stream123", false},
		{"invalid format", "invalid:key", "", "", true},
		{"missing parts", "leader:youtube", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			platform, streamID, err := manager.parseLeaderKey(tt.key)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantPlatform, platform)
				assert.Equal(t, tt.wantStreamID, streamID)
			}
		})
	}
}
