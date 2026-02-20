package coordination

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// CoordinationLock provides Redis distributed locking for coordination between rebalancing and HPA operations
type CoordinationLock struct {
	redisClient *redis.Client
	lockKey     string
	lockValue   string // Unique identifier for this operation (ownership verification)
	lockTTL     time.Duration
	logger      *zap.Logger
}

const (
	defaultLockKey = "rebalancing:coordination_lock"
	defaultLockTTL = 60 * time.Second
)

// NewCoordinationLock creates a new coordination lock instance
func NewCoordinationLock(redisClient *redis.Client, logger *zap.Logger) *CoordinationLock {
	return &CoordinationLock{
		redisClient: redisClient,
		lockKey:     defaultLockKey,
		lockTTL:     defaultLockTTL,
		logger:      logger,
	}
}

// AcquireLock attempts to acquire the distributed lock for the specified operation
// Returns true if lock acquired, false if already held by another operation
func (l *CoordinationLock) AcquireLock(ctx context.Context, operation string) (bool, error) {
	// Generate unique lock value for ownership verification
	l.lockValue = fmt.Sprintf("%s-%d", operation, time.Now().UnixNano())

	// Redis SET NX EX: Set if Not eXists with EXpiration (atomic operation)
	// Per RESEARCH.md Pattern 5: This is the standard Redis distributed lock pattern
	result, err := l.redisClient.SetNX(ctx, l.lockKey, l.lockValue, l.lockTTL).Result()
	if err != nil {
		return false, fmt.Errorf("failed to acquire lock: %w", err)
	}

	if result {
		l.logger.Info("Acquired coordination lock",
			zap.String("operation", operation),
			zap.String("lock_value", l.lockValue),
			zap.Duration("ttl", l.lockTTL),
		)
		return true, nil
	}

	// Lock already held - query current holder for logging
	holder, _ := l.redisClient.Get(ctx, l.lockKey).Result()
	l.logger.Info("Lock held by another operation",
		zap.String("holder", holder),
		zap.String("attempted_operation", operation),
	)
	return false, nil
}

// ReleaseLock releases the distributed lock with ownership verification
// Uses Lua script for atomic check-and-delete to prevent releasing another operation's lock
func (l *CoordinationLock) ReleaseLock(ctx context.Context) error {
	// Lua script for atomic check-and-delete (ownership verification)
	// Per RESEARCH.md Pattern 5: Ensures we only delete locks we own
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`

	result, err := l.redisClient.Eval(ctx, script, []string{l.lockKey}, l.lockValue).Result()
	if err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	// result == 1 means lock was deleted (we owned it)
	// result == 0 means lock was not owned by us (already expired or taken by another operation)
	if result == int64(1) {
		l.logger.Info("Released coordination lock",
			zap.String("lock_value", l.lockValue),
		)
		return nil
	}

	l.logger.Warn("Lock not owned by this operation (already expired or taken)",
		zap.String("lock_value", l.lockValue),
	)
	return nil
}

// ExtendLock extends the lock TTL for long-running operations
// Uses Lua script for atomic check-and-expire to ensure ownership
func (l *CoordinationLock) ExtendLock(ctx context.Context) error {
	// Lua script for atomic check-and-expire (ownership verification)
	// Per RESEARCH.md Pattern 5: Extends TTL only if we still own the lock
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("pexpire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`

	result, err := l.redisClient.Eval(ctx, script,
		[]string{l.lockKey},
		l.lockValue,
		l.lockTTL.Milliseconds(),
	).Result()

	if err != nil {
		return fmt.Errorf("failed to extend lock: %w", err)
	}

	if result == int64(0) {
		return fmt.Errorf("lock no longer owned by this operation")
	}

	l.logger.Debug("Extended coordination lock",
		zap.String("lock_value", l.lockValue),
		zap.Duration("ttl", l.lockTTL),
	)
	return nil
}
