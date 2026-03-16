package yt_emote_cache

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper creates a miniredis server and a go-redis client pointed at it.
func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return mr, rdb
}

// TestCacheYTEmotes_WritesKey tests that CacheYTEmotes writes a Redis key containing
// the emote data after being called.
// NOTE: CacheYTEmotes and EmoteEntry do not yet exist in this package.
// Tests will fail to compile until Plan 02 creates cache.go with these symbols. RED state.
func TestCacheYTEmotes_WritesKey(t *testing.T) {
	mr, rdb := newTestRedis(t)

	err := CacheYTEmotes(context.Background(), rdb, "UCchannel123", []EmoteEntry{
		{ID: "UCemote1", Code: ":e1:", URL: "https://img.png"},
	})
	require.NoError(t, err)

	val, err := mr.Get("yt:emote:UCchannel123:UCemote1")
	require.NoError(t, err, "key should exist in Redis after CacheYTEmotes")
	assert.NotEmpty(t, val, "stored value should not be empty")

	// Verify it's valid JSON containing the emote data
	var stored map[string]interface{}
	err = json.Unmarshal([]byte(val), &stored)
	assert.NoError(t, err, "stored value should be valid JSON")
}

// TestCacheYTEmotes_TTL24h tests that the key TTL is approximately 24 hours.
func TestCacheYTEmotes_TTL24h(t *testing.T) {
	mr, rdb := newTestRedis(t)

	err := CacheYTEmotes(context.Background(), rdb, "UCchannel123", []EmoteEntry{
		{ID: "UCemote1", Code: ":e1:", URL: "https://img.png"},
	})
	require.NoError(t, err)

	ttl := mr.TTL("yt:emote:UCchannel123:UCemote1")
	assert.GreaterOrEqual(t, ttl, 23*time.Hour+50*time.Minute, "TTL should be at least 23h50m")
	assert.LessOrEqual(t, ttl, 24*time.Hour+10*time.Minute, "TTL should be at most 24h10m")
}

// TestCacheYTEmotes_MultipleEmotes tests that two emotes in one call write two distinct Redis keys.
func TestCacheYTEmotes_MultipleEmotes(t *testing.T) {
	mr, rdb := newTestRedis(t)

	err := CacheYTEmotes(context.Background(), rdb, "UCchannel123", []EmoteEntry{
		{ID: "UCemote1", Code: ":e1:", URL: "https://img1.png"},
		{ID: "UCemote2", Code: ":e2:", URL: "https://img2.png"},
	})
	require.NoError(t, err)

	val1, err1 := mr.Get("yt:emote:UCchannel123:UCemote1")
	val2, err2 := mr.Get("yt:emote:UCchannel123:UCemote2")

	assert.NoError(t, err1, "first emote key should exist")
	assert.NoError(t, err2, "second emote key should exist")
	assert.NotEmpty(t, val1)
	assert.NotEmpty(t, val2)
	assert.NotEqual(t, val1, val2, "two distinct keys should have distinct values")
}

// TestCacheYTEmotes_EmptyList tests that an empty emote slice causes no Redis operations and no panic.
func TestCacheYTEmotes_EmptyList(t *testing.T) {
	_, rdb := newTestRedis(t)

	err := CacheYTEmotes(context.Background(), rdb, "UCchannel123", []EmoteEntry{})
	assert.NoError(t, err, "empty list should not error")
}

// TestCacheYTEmotes_RefreshesExistingTTL tests that calling CacheYTEmotes twice with the
// same key resets the TTL (SET not SETNX).
func TestCacheYTEmotes_RefreshesExistingTTL(t *testing.T) {
	mr, rdb := newTestRedis(t)

	emotes := []EmoteEntry{
		{ID: "UCemote1", Code: ":e1:", URL: "https://img.png"},
	}

	// First write
	err := CacheYTEmotes(context.Background(), rdb, "UCchannel", emotes)
	require.NoError(t, err)

	// Fast-forward time by 1 hour to reduce TTL
	mr.FastForward(1 * time.Hour)

	ttlAfterForward := mr.TTL("yt:emote:UCchannel:UCemote1")
	assert.Less(t, ttlAfterForward, 24*time.Hour, "TTL should have decreased after time forward")

	// Second write should reset TTL to 24h
	err = CacheYTEmotes(context.Background(), rdb, "UCchannel", emotes)
	require.NoError(t, err)

	ttlAfterRefresh := mr.TTL("yt:emote:UCchannel:UCemote1")
	assert.GreaterOrEqual(t, ttlAfterRefresh, 23*time.Hour+50*time.Minute, "TTL should be reset to ~24h")
}

// TestCacheYTEmotes_TimeoutContext tests that a 1ns timeout context causes graceful handling
// (logs error, returns without panic, does not block).
func TestCacheYTEmotes_TimeoutContext(t *testing.T) {
	_, rdb := newTestRedis(t)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	// Give the timeout a chance to expire
	time.Sleep(1 * time.Millisecond)

	// Should not panic
	assert.NotPanics(t, func() {
		_ = CacheYTEmotes(ctx, rdb, "UCchannel", []EmoteEntry{
			{ID: "UCemote1", Code: ":e1:", URL: "https://img.png"},
		})
	}, "should not panic on timeout context")
}
