package coordination

import (
	"context"
	"fmt"
	"time"

	"github.com/caesar/all-chat/shared/metrics"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Throttler implements cooldown enforcement and thrashing detection for rebalancing operations
// Prevents excessive channel migrations via 5-minute cooldown between operations,
// detects pathological patterns (>3 rebalances in 15 minutes), and allows escalation
// overrides when imbalance significantly worsens.
type Throttler struct {
	redisClient         *redis.Client
	cooldownDuration    time.Duration // 5 minutes per CONTEXT.md
	thrashingWindow     time.Duration // 15 minutes
	thrashingThreshold  int           // 3 rebalances
	escalationThreshold float64       // 0.4 ratio increase (e.g., 0.6 → 1.0)
	metrics             *metrics.ShardMetrics
	logger              *zap.Logger
}

// Redis keys used by Throttler
const (
	// rebalancingCooldownKey stores RFC3339 timestamp of last rebalancing, TTL=5min
	rebalancingCooldownKey = "rebalancing:cooldown"

	// rebalancingHistoryKey is a sorted set (score=timestamp, member=rebalance_id)
	rebalancingHistoryKey = "rebalancing:history"

	// rebalancingLastRatioKey stores last observed imbalance ratio for escalation comparison
	rebalancingLastRatioKey = "rebalancing:last_ratio"
)

// NewThrottler creates a new throttler instance
func NewThrottler(
	redisClient *redis.Client,
	cooldownDuration time.Duration,
	shardMetrics *metrics.ShardMetrics,
	logger *zap.Logger,
) *Throttler {
	return &Throttler{
		redisClient:         redisClient,
		cooldownDuration:    cooldownDuration,
		thrashingWindow:     15 * time.Minute,
		thrashingThreshold:  3,
		escalationThreshold: 0.4,
		metrics:             shardMetrics,
		logger:              logger,
	}
}

// CheckCooldown checks if rebalancing is allowed based on cooldown status, thrashing detection,
// and escalation override logic.
//
// Returns:
//   - allowed: true if rebalancing can proceed, false if blocked
//   - reason: human-readable reason for decision ("ok", "cooldown_active", "escalation_override", "thrashing_detected")
//   - error: non-nil if Redis operation failed
func (t *Throttler) CheckCooldown(ctx context.Context, currentRatio float64) (bool, string, error) {
	// Get last rebalancing timestamp
	lastRebalance, err := t.redisClient.Get(ctx, rebalancingCooldownKey).Result()

	if err == redis.Nil {
		// No cooldown active - rebalancing allowed
		return true, "ok", nil
	}

	if err != nil {
		// Redis error - fail open (allow rebalancing)
		t.logger.Error("Failed to check cooldown", zap.Error(err))
		return true, "ok", err
	}

	// Parse timestamp and calculate elapsed time
	lastTime, err := time.Parse(time.RFC3339, lastRebalance)
	if err != nil {
		t.logger.Error("Failed to parse cooldown timestamp",
			zap.String("timestamp", lastRebalance),
			zap.Error(err))
		// Invalid timestamp - allow rebalancing
		return true, "ok", nil
	}

	elapsed := time.Since(lastTime)

	if elapsed < t.cooldownDuration {
		// Cooldown still active - check escalation override
		previousRatio, err := t.redisClient.Get(ctx, rebalancingLastRatioKey).Float64()
		if err != nil && err != redis.Nil {
			t.logger.Error("Failed to get previous ratio", zap.Error(err))
		}

		ratioIncrease := currentRatio - previousRatio

		// Escalation override: allow breaking cooldown if imbalance significantly worsens
		if ratioIncrease > t.escalationThreshold {
			t.logger.Warn("Cooldown overridden by escalation",
				zap.Duration("elapsed", elapsed),
				zap.Float64("previous_ratio", previousRatio),
				zap.Float64("current_ratio", currentRatio),
				zap.Float64("ratio_increase", ratioIncrease),
				zap.Float64("escalation_threshold", t.escalationThreshold),
			)

			// Increment escalation override metric
			t.metrics.RebalancingCooldownOverrides.Inc()

			return true, "escalation_override", nil
		}

		// Cooldown still active, no escalation override
		remaining := t.cooldownDuration - elapsed
		reason := fmt.Sprintf("cooldown_active (remaining: %s)", remaining.Round(time.Second))

		t.logger.Debug("Rebalancing blocked by cooldown",
			zap.Duration("elapsed", elapsed),
			zap.Duration("remaining", remaining),
		)

		return false, reason, nil
	}

	// Cooldown expired - check thrashing before allowing
	isThrashing, err := t.detectThrashing(ctx)
	if err != nil {
		t.logger.Error("Failed to detect thrashing", zap.Error(err))
		// Fail open - allow rebalancing
		return true, "ok", err
	}

	if isThrashing {
		return false, "thrashing_detected", nil
	}

	return true, "ok", nil
}

// detectThrashing checks if system is experiencing thrashing (>3 rebalances in 15 minutes)
// Thrashing indicates pathological load patterns or misconfigured HPA
//
// Response strategy per RESEARCH.md: Alert-only (log error, enforce cooldown, let operators investigate)
func (t *Throttler) detectThrashing(ctx context.Context) (bool, error) {
	// Calculate cutoff timestamp (15 minutes ago)
	cutoff := time.Now().Add(-t.thrashingWindow).Unix()

	// Query Redis Sorted Set: count entries with score >= cutoff
	count, err := t.redisClient.ZCount(ctx, rebalancingHistoryKey, fmt.Sprintf("%d", cutoff), "+inf").Result()
	if err != nil {
		return false, fmt.Errorf("failed to query rebalancing history: %w", err)
	}

	if count >= int64(t.thrashingThreshold) {
		t.logger.Error("Thrashing detected - excessive rebalancing operations",
			zap.Int64("rebalances_in_window", count),
			zap.Int("threshold", t.thrashingThreshold),
			zap.Duration("window", t.thrashingWindow),
		)

		// Increment thrashing metric
		t.metrics.RebalancingThrashing.Inc()

		return true, nil
	}

	return false, nil
}

// RecordRebalancing records a successful rebalancing operation in Redis
// Sets cooldown timestamp, adds entry to history, stores current imbalance ratio
func (t *Throttler) RecordRebalancing(ctx context.Context, rebalanceID string, imbalanceRatio float64) error {
	now := time.Now()

	// Use Redis pipeline for atomic operations
	pipe := t.redisClient.Pipeline()

	// Set cooldown key with TTL=5 minutes
	pipe.Set(ctx, rebalancingCooldownKey, now.Format(time.RFC3339), t.cooldownDuration)

	// Add to history sorted set (score=timestamp, member=rebalance_id)
	pipe.ZAdd(ctx, rebalancingHistoryKey, redis.Z{
		Score:  float64(now.Unix()),
		Member: rebalanceID,
	})

	// Store current imbalance ratio for escalation comparison
	pipe.Set(ctx, rebalancingLastRatioKey, imbalanceRatio, 0) // No TTL - persistent

	// Cleanup old history entries (>15 minutes)
	cutoff := now.Add(-t.thrashingWindow).Unix()
	pipe.ZRemRangeByScore(ctx, rebalancingHistoryKey, "-inf", fmt.Sprintf("%d", cutoff))

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		t.logger.Error("Failed to record rebalancing",
			zap.String("rebalance_id", rebalanceID),
			zap.Error(err))
		return fmt.Errorf("failed to record rebalancing: %w", err)
	}

	// Increment rebalancing total metric
	t.metrics.RebalancingTotal.Inc()

	t.logger.Info("Recorded rebalancing operation",
		zap.String("rebalance_id", rebalanceID),
		zap.Float64("imbalance_ratio", imbalanceRatio),
		zap.String("cooldown_until", now.Add(t.cooldownDuration).Format(time.RFC3339)),
	)

	return nil
}
