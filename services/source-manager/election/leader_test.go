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
