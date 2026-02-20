package coordination

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTestRedisForLock(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	return client, mr
}

func TestAcquireLock_Success(t *testing.T) {
	client, mr := setupTestRedisForLock(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	lock := NewCoordinationLock(client, logger)

	ctx := context.Background()
	acquired, err := lock.AcquireLock(ctx, "rebalancing")

	require.NoError(t, err)
	assert.True(t, acquired, "Lock should be acquired on first attempt")

	// Verify lock exists in Redis
	value, err := client.Get(ctx, defaultLockKey).Result()
	require.NoError(t, err)
	assert.Contains(t, value, "rebalancing-", "Lock value should contain operation name")
}

func TestAcquireLock_AlreadyHeld(t *testing.T) {
	client, mr := setupTestRedisForLock(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	lock1 := NewCoordinationLock(client, logger)
	lock2 := NewCoordinationLock(client, logger)

	ctx := context.Background()

	// First lock acquires successfully
	acquired1, err := lock1.AcquireLock(ctx, "rebalancing")
	require.NoError(t, err)
	assert.True(t, acquired1, "First lock should be acquired")

	// Second lock fails (already held)
	acquired2, err := lock2.AcquireLock(ctx, "scale_up")
	require.NoError(t, err)
	assert.False(t, acquired2, "Second lock should not be acquired")

	// Verify first lock holder unchanged
	value, err := client.Get(ctx, defaultLockKey).Result()
	require.NoError(t, err)
	assert.Equal(t, lock1.lockValue, value, "Lock holder should remain unchanged")
}

func TestReleaseLock_Success(t *testing.T) {
	client, mr := setupTestRedisForLock(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	lock := NewCoordinationLock(client, logger)

	ctx := context.Background()

	// Acquire lock
	acquired, err := lock.AcquireLock(ctx, "rebalancing")
	require.NoError(t, err)
	require.True(t, acquired)

	// Release lock
	err = lock.ReleaseLock(ctx)
	require.NoError(t, err)

	// Verify lock no longer exists
	_, err = client.Get(ctx, defaultLockKey).Result()
	assert.Equal(t, redis.Nil, err, "Lock should be deleted after release")

	// Subsequent acquire should succeed
	lock2 := NewCoordinationLock(client, logger)
	acquired2, err := lock2.AcquireLock(ctx, "scale_down")
	require.NoError(t, err)
	assert.True(t, acquired2, "Lock should be acquirable after release")
}

func TestReleaseLock_WrongOwner(t *testing.T) {
	client, mr := setupTestRedisForLock(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	lock1 := NewCoordinationLock(client, logger)
	lock2 := NewCoordinationLock(client, logger)

	ctx := context.Background()

	// Lock1 acquires
	acquired, err := lock1.AcquireLock(ctx, "rebalancing")
	require.NoError(t, err)
	require.True(t, acquired)

	// Lock2 attempts to release (wrong owner)
	lock2.lockValue = "wrong-operation-12345"
	err = lock2.ReleaseLock(ctx)
	require.NoError(t, err) // Should not error, but should not delete lock

	// Verify lock still held by lock1
	value, err := client.Get(ctx, defaultLockKey).Result()
	require.NoError(t, err)
	assert.Equal(t, lock1.lockValue, value, "Lock should still be held by original owner")
}

func TestExtendLock_Success(t *testing.T) {
	client, mr := setupTestRedisForLock(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	lock := NewCoordinationLock(client, logger)

	ctx := context.Background()

	// Acquire lock
	acquired, err := lock.AcquireLock(ctx, "rebalancing")
	require.NoError(t, err)
	require.True(t, acquired)

	// Fast-forward time to near expiration (simulate long operation)
	mr.FastForward(50 * time.Second)

	// Extend lock
	err = lock.ExtendLock(ctx)
	require.NoError(t, err)

	// Verify lock TTL extended (check if still exists after original TTL)
	mr.FastForward(15 * time.Second) // Total 65s (would have expired without extension)

	value, err := client.Get(ctx, defaultLockKey).Result()
	require.NoError(t, err)
	assert.Equal(t, lock.lockValue, value, "Lock should still exist after extension")
}

func TestExtendLock_Expired(t *testing.T) {
	client, mr := setupTestRedisForLock(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	lock := NewCoordinationLock(client, logger)

	ctx := context.Background()

	// Acquire lock
	acquired, err := lock.AcquireLock(ctx, "rebalancing")
	require.NoError(t, err)
	require.True(t, acquired)

	// Fast-forward past TTL (lock expires)
	mr.FastForward(65 * time.Second)

	// Attempt to extend expired lock
	err = lock.ExtendLock(ctx)
	assert.Error(t, err, "Should error when extending expired lock")
	assert.Contains(t, err.Error(), "lock no longer owned", "Error should indicate lock not owned")
}

func TestCoordinationLock_TTLAutoExpiration(t *testing.T) {
	client, mr := setupTestRedisForLock(t)
	defer mr.Close()
	defer client.Close()

	logger := zap.NewNop()
	lock := NewCoordinationLock(client, logger)

	ctx := context.Background()

	// Acquire lock
	acquired, err := lock.AcquireLock(ctx, "rebalancing")
	require.NoError(t, err)
	require.True(t, acquired)

	// Fast-forward past TTL
	mr.FastForward(65 * time.Second)

	// Verify lock auto-expired
	_, err = client.Get(ctx, defaultLockKey).Result()
	assert.Equal(t, redis.Nil, err, "Lock should auto-expire after TTL")

	// New lock should be acquirable
	lock2 := NewCoordinationLock(client, logger)
	acquired2, err := lock2.AcquireLock(ctx, "scale_up")
	require.NoError(t, err)
	assert.True(t, acquired2, "Lock should be acquirable after auto-expiration")
}
