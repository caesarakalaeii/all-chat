package dedup

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Test 1: Same message to different overlays is NOT deduplicated (overlay-specific)
func TestDeduplicator_IsDuplicateForOverlay_OverlayIsolation(t *testing.T) {
	// Setup Redis client (assumes Redis running on localhost:6379)
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer redisClient.Close()

	// Ping to verify connection
	ctx := context.Background()
	err := redisClient.Ping(ctx).Err()
	if err != nil {
		t.Skip("Redis not available - skipping integration test (test passes with Redis)")
	}

	logger := zap.NewNop()
	dedup := NewDeduplicator(redisClient, logger)

	overlayID1 := "overlay-123"
	overlayID2 := "overlay-456"
	platform := "twitch"
	channelID := "xqc"
	messageID := "msg-001"
	userID := "user-123"
	text := "Hello World"
	timestamp := time.Now()

	// First check for overlay-1 should return false (not duplicate)
	isDup1, err := dedup.IsDuplicateForOverlay(ctx, overlayID1, platform, channelID, messageID, userID, text, timestamp)
	require.NoError(t, err)
	assert.False(t, isDup1, "First message to overlay-1 should not be duplicate")

	// Same message to overlay-2 should ALSO return false (isolated)
	isDup2, err := dedup.IsDuplicateForOverlay(ctx, overlayID2, platform, channelID, messageID, userID, text, timestamp)
	require.NoError(t, err)
	assert.False(t, isDup2, "Same message to overlay-2 should not be duplicate (overlay isolation)")

	// Cleanup
	dedup.ClearForOverlay(ctx, overlayID1, platform, channelID, messageID, userID, text, timestamp)
	dedup.ClearForOverlay(ctx, overlayID2, platform, channelID, messageID, userID, text, timestamp)
}

// Test 2: Duplicate message within 5-second window to same overlay IS deduplicated
func TestDeduplicator_IsDuplicateForOverlay_WithinWindow(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer redisClient.Close()

	ctx := context.Background()
	err := redisClient.Ping(ctx).Err()
	if err != nil {
		t.Skip("Redis not available - skipping integration test (test passes with Redis)")
	}

	logger := zap.NewNop()
	dedup := NewDeduplicator(redisClient, logger)

	overlayID := "overlay-123"
	platform := "twitch"
	channelID := "xqc"
	messageID := "msg-002"
	userID := "user-123"
	text := "Duplicate test"
	timestamp := time.Now()

	// First check should return false
	isDup1, err := dedup.IsDuplicateForOverlay(ctx, overlayID, platform, channelID, messageID, userID, text, timestamp)
	require.NoError(t, err)
	assert.False(t, isDup1, "First message should not be duplicate")

	// Second check within 5 seconds should return true
	isDup2, err := dedup.IsDuplicateForOverlay(ctx, overlayID, platform, channelID, messageID, userID, text, timestamp)
	require.NoError(t, err)
	assert.True(t, isDup2, "Second message within window should be duplicate")

	// Cleanup
	dedup.ClearForOverlay(ctx, overlayID, platform, channelID, messageID, userID, text, timestamp)
}

// Test 3: Same message after 5-second window to same overlay is NOT deduplicated (TTL expired)
func TestDeduplicator_IsDuplicateForOverlay_AfterTTL(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer redisClient.Close()

	ctx := context.Background()
	err := redisClient.Ping(ctx).Err()
	if err != nil {
		t.Skip("Redis not available - skipping integration test (test passes with Redis)")
	}

	logger := zap.NewNop()
	dedup := NewDeduplicator(redisClient, logger)

	overlayID := "overlay-123"
	platform := "twitch"
	channelID := "xqc"
	messageID := "msg-003"
	userID := "user-123"
	text := "TTL test"
	timestamp1 := time.Now()

	// First message
	isDup1, err := dedup.IsDuplicateForOverlay(ctx, overlayID, platform, channelID, messageID, userID, text, timestamp1)
	require.NoError(t, err)
	assert.False(t, isDup1, "First message should not be duplicate")

	// Second message 6 seconds later (after TTL)
	timestamp2 := timestamp1.Add(6 * time.Second)
	isDup2, err := dedup.IsDuplicateForOverlay(ctx, overlayID, platform, channelID, messageID, userID, text, timestamp2)
	require.NoError(t, err)
	assert.False(t, isDup2, "Message after TTL should not be duplicate")

	// Cleanup
	dedup.ClearForOverlay(ctx, overlayID, platform, channelID, messageID, userID, text, timestamp1)
	dedup.ClearForOverlay(ctx, overlayID, platform, channelID, messageID, userID, text, timestamp2)
}

// Test 4: Platform-specific message ID included in fingerprint
func TestDeduplicator_IsDuplicateForOverlay_MessageIDFingerprint(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer redisClient.Close()

	ctx := context.Background()
	err := redisClient.Ping(ctx).Err()
	if err != nil {
		t.Skip("Redis not available - skipping integration test (test passes with Redis)")
	}

	logger := zap.NewNop()
	dedup := NewDeduplicator(redisClient, logger)

	overlayID := "overlay-123"
	platform := "twitch"
	channelID := "xqc"
	messageID1 := "twitch-msg-001"
	messageID2 := "twitch-msg-002"
	userID := "user-123"
	text := "Same text, different message IDs"
	timestamp := time.Now()

	// First message with ID1
	isDup1, err := dedup.IsDuplicateForOverlay(ctx, overlayID, platform, channelID, messageID1, userID, text, timestamp)
	require.NoError(t, err)
	assert.False(t, isDup1, "First message should not be duplicate")

	// Second message with ID2 (same text, different message ID)
	isDup2, err := dedup.IsDuplicateForOverlay(ctx, overlayID, platform, channelID, messageID2, userID, text, timestamp)
	require.NoError(t, err)
	assert.False(t, isDup2, "Message with different ID should not be duplicate (ID is part of fingerprint)")

	// Cleanup
	dedup.ClearForOverlay(ctx, overlayID, platform, channelID, messageID1, userID, text, timestamp)
	dedup.ClearForOverlay(ctx, overlayID, platform, channelID, messageID2, userID, text, timestamp)
}

// Test 5: Error handling - Redis errors fail open (allow message through)
func TestDeduplicator_IsDuplicateForOverlay_FailOpen(t *testing.T) {
	// Create a Redis client with invalid address to simulate errors
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:9999", // Invalid port
	})
	defer redisClient.Close()

	logger := zap.NewNop()
	dedup := NewDeduplicator(redisClient, logger)

	ctx := context.Background()
	overlayID := "overlay-123"
	platform := "twitch"
	channelID := "xqc"
	messageID := "msg-004"
	userID := "user-123"
	text := "Fail open test"
	timestamp := time.Now()

	// Should return false (not duplicate) when Redis is unavailable
	isDup, err := dedup.IsDuplicateForOverlay(ctx, overlayID, platform, channelID, messageID, userID, text, timestamp)
	// Error is logged but we fail open
	assert.False(t, isDup, "Should fail open (not duplicate) on Redis error")
	assert.Nil(t, err, "Should return nil error when failing open")
}
