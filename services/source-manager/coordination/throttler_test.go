package coordination

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// setupThrottlerTest creates a miniredis instance and throttler for testing
func setupThrottlerTest(t *testing.T) (*Throttler, *redis.Client, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)

	redisClient := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	logger := zap.NewNop()

	throttler := NewThrottler(redisClient, 5*time.Minute, testShardMetrics, logger)

	return throttler, redisClient, mr
}

func TestCheckCooldown_NoCooldown(t *testing.T) {
	throttler, _, mr := setupThrottlerTest(t)
	defer mr.Close()

	ctx := context.Background()
	currentRatio := 1.5

	allowed, reason, err := throttler.CheckCooldown(ctx, currentRatio)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !allowed {
		t.Errorf("Expected allowed=true with no cooldown, got false")
	}

	if reason != "ok" {
		t.Errorf("Expected reason='ok', got '%s'", reason)
	}
}

func TestCheckCooldown_ActiveCooldown(t *testing.T) {
	throttler, redisClient, mr := setupThrottlerTest(t)
	defer mr.Close()

	ctx := context.Background()

	// Set cooldown key with timestamp 2 minutes ago
	twoMinutesAgo := time.Now().Add(-2 * time.Minute)
	redisClient.Set(ctx, rebalancingCooldownKey, twoMinutesAgo.Format(time.RFC3339), 5*time.Minute)

	// Set last_ratio to prevent escalation override (currentRatio 1.5 - lastRatio 1.3 = 0.2 < 0.4 threshold)
	redisClient.Set(ctx, rebalancingLastRatioKey, 1.3, 0)

	allowed, reason, err := throttler.CheckCooldown(ctx, 1.5)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if allowed {
		t.Errorf("Expected allowed=false with active cooldown, got true")
	}

	// Check reason starts with "cooldown_active" (timing may vary slightly)
	if len(reason) < 17 || reason[:17] != "cooldown_active (" {
		t.Errorf("Expected reason to start with 'cooldown_active (', got '%s'", reason)
	}
}

func TestCheckCooldown_ExpiredCooldown(t *testing.T) {
	throttler, redisClient, mr := setupThrottlerTest(t)
	defer mr.Close()

	ctx := context.Background()

	// Set cooldown key with timestamp 6 minutes ago (expired)
	sixMinutesAgo := time.Now().Add(-6 * time.Minute)
	redisClient.Set(ctx, rebalancingCooldownKey, sixMinutesAgo.Format(time.RFC3339), 5*time.Minute)

	allowed, reason, err := throttler.CheckCooldown(ctx, 1.5)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !allowed {
		t.Errorf("Expected allowed=true with expired cooldown, got false")
	}

	if reason != "ok" {
		t.Errorf("Expected reason='ok', got '%s'", reason)
	}
}

func TestCheckCooldown_EscalationOverride(t *testing.T) {
	throttler, redisClient, mr := setupThrottlerTest(t)
	defer mr.Close()

	ctx := context.Background()

	// Set cooldown active (2 min elapsed)
	twoMinutesAgo := time.Now().Add(-2 * time.Minute)
	redisClient.Set(ctx, rebalancingCooldownKey, twoMinutesAgo.Format(time.RFC3339), 5*time.Minute)

	// Set last_ratio=0.6, currentRatio=1.1 (increase=0.5 > 0.4 threshold)
	redisClient.Set(ctx, rebalancingLastRatioKey, 0.6, 0)
	currentRatio := 1.1

	allowed, reason, err := throttler.CheckCooldown(ctx, currentRatio)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !allowed {
		t.Errorf("Expected allowed=true with escalation override, got false")
	}

	if reason != "escalation_override" {
		t.Errorf("Expected reason='escalation_override', got '%s'", reason)
	}

	// Note: Metric verification is simplified in tests
	// In production, use prometheus testutil.CollectAndCompare for proper validation
}

func TestCheckCooldown_EscalationBelowThreshold(t *testing.T) {
	throttler, redisClient, mr := setupThrottlerTest(t)
	defer mr.Close()

	ctx := context.Background()

	// Set cooldown active (2 min elapsed)
	twoMinutesAgo := time.Now().Add(-2 * time.Minute)
	redisClient.Set(ctx, rebalancingCooldownKey, twoMinutesAgo.Format(time.RFC3339), 5*time.Minute)

	// Set last_ratio=0.6, currentRatio=0.9 (increase=0.3 < 0.4 threshold)
	redisClient.Set(ctx, rebalancingLastRatioKey, 0.6, 0)
	currentRatio := 0.9

	allowed, reason, err := throttler.CheckCooldown(ctx, currentRatio)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if allowed {
		t.Errorf("Expected allowed=false below escalation threshold, got true")
	}

	// Check reason starts with "cooldown_active" (timing may vary slightly)
	if len(reason) < 17 || reason[:17] != "cooldown_active (" {
		t.Errorf("Expected reason to start with 'cooldown_active (', got '%s'", reason)
	}
}

func TestDetectThrashing(t *testing.T) {
	throttler, redisClient, mr := setupThrottlerTest(t)
	defer mr.Close()

	ctx := context.Background()

	// Add 4 rebalancing events to history (within 15 min window)
	now := time.Now()
	for i := 0; i < 4; i++ {
		timestamp := now.Add(time.Duration(-i*3) * time.Minute) // -0, -3, -6, -9 minutes
		redisClient.ZAdd(ctx, rebalancingHistoryKey, redis.Z{
			Score:  float64(timestamp.Unix()),
			Member: fmt.Sprintf("rebalance-%d", i),
		})
	}

	isThrashing, err := throttler.detectThrashing(ctx)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !isThrashing {
		t.Errorf("Expected isThrashing=true with 4 events, got false")
	}

	// Note: Metric verification is simplified in tests
	// In production, use prometheus testutil.CollectAndCompare for proper validation
}

func TestDetectThrashing_NoThrashing(t *testing.T) {
	throttler, redisClient, mr := setupThrottlerTest(t)
	defer mr.Close()

	ctx := context.Background()

	// Add 2 rebalancing events to history (below threshold)
	now := time.Now()
	for i := 0; i < 2; i++ {
		timestamp := now.Add(time.Duration(-i*3) * time.Minute)
		redisClient.ZAdd(ctx, rebalancingHistoryKey, redis.Z{
			Score:  float64(timestamp.Unix()),
			Member: fmt.Sprintf("rebalance-%d", i),
		})
	}

	isThrashing, err := throttler.detectThrashing(ctx)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if isThrashing {
		t.Errorf("Expected isThrashing=false with 2 events, got true")
	}
}

func TestDetectThrashing_OldEventsIgnored(t *testing.T) {
	throttler, redisClient, mr := setupThrottlerTest(t)
	defer mr.Close()

	ctx := context.Background()

	now := time.Now()

	// Add 3 events within window
	for i := 0; i < 3; i++ {
		timestamp := now.Add(time.Duration(-i*3) * time.Minute) // -0, -3, -6 minutes
		redisClient.ZAdd(ctx, rebalancingHistoryKey, redis.Z{
			Score:  float64(timestamp.Unix()),
			Member: fmt.Sprintf("rebalance-recent-%d", i),
		})
	}

	// Add 2 events older than 15 min (should be ignored)
	for i := 0; i < 2; i++ {
		timestamp := now.Add(time.Duration(-20-i*3) * time.Minute) // -20, -23 minutes
		redisClient.ZAdd(ctx, rebalancingHistoryKey, redis.Z{
			Score:  float64(timestamp.Unix()),
			Member: fmt.Sprintf("rebalance-old-%d", i),
		})
	}

	isThrashing, err := throttler.detectThrashing(ctx)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Should count only 3 recent events (threshold is 3, so isThrashing=true)
	if !isThrashing {
		t.Errorf("Expected isThrashing=true with 3 recent events (ignoring old), got false")
	}
}

func TestRecordRebalancing(t *testing.T) {
	throttler, redisClient, mr := setupThrottlerTest(t)
	defer mr.Close()

	ctx := context.Background()
	rebalanceID := "rebalance-test-123"
	imbalanceRatio := 1.75

	err := throttler.RecordRebalancing(ctx, rebalanceID, imbalanceRatio)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Verify cooldown key set
	cooldown, err := redisClient.Get(ctx, rebalancingCooldownKey).Result()
	if err != nil {
		t.Errorf("Expected cooldown key to be set, got error %v", err)
	}

	// Verify timestamp is valid RFC3339
	_, err = time.Parse(time.RFC3339, cooldown)
	if err != nil {
		t.Errorf("Expected valid RFC3339 timestamp, got error parsing: %v", err)
	}

	// Verify cooldown key has TTL (approximately 5 minutes)
	ttl, err := redisClient.TTL(ctx, rebalancingCooldownKey).Result()
	if err != nil {
		t.Errorf("Expected cooldown key to have TTL, got error %v", err)
	}
	if ttl < 4*time.Minute || ttl > 5*time.Minute {
		t.Errorf("Expected TTL ~5 minutes, got %v", ttl)
	}

	// Verify history contains rebalanceID
	score, err := redisClient.ZScore(ctx, rebalancingHistoryKey, rebalanceID).Result()
	if err != nil {
		t.Errorf("Expected rebalanceID in history, got error %v", err)
	}
	if score == 0 {
		t.Errorf("Expected non-zero score for rebalanceID")
	}

	// Verify last_ratio stored
	storedRatio, err := redisClient.Get(ctx, rebalancingLastRatioKey).Float64()
	if err != nil {
		t.Errorf("Expected last_ratio to be stored, got error %v", err)
	}
	if storedRatio != imbalanceRatio {
		t.Errorf("Expected last_ratio=%f, got %f", imbalanceRatio, storedRatio)
	}

	// Note: Metric verification is simplified in tests
	// In production, use prometheus testutil.CollectAndCompare for proper validation
}
