// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package featuregates_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/shared/featuregates"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testableCache is a test helper that wraps FeatureGateCache but allows
// injecting a pre-populated gates map without a DB connection.
type testableCache struct {
	*featuregates.FeatureGateCache
}

// newTestCacheWithGates creates a FeatureGateCache with a pre-populated gates map
// for unit testing IsPremium without hitting a real DB.
func newTestCacheWithGates(gates map[string]bool) *featuregates.FeatureGateCache {
	return featuregates.NewFeatureGateCacheWithGates(gates)
}

// TestIsPremium_KnownKeyTrue verifies that IsPremium returns true when cache has
// the key set to true.
func TestIsPremium_KnownKeyTrue(t *testing.T) {
	cache := newTestCacheWithGates(map[string]bool{
		featuregates.GateSharing: true,
	})

	got := cache.IsPremium(featuregates.GateSharing)
	assert.True(t, got, "IsPremium should return true when gate is premium=true")
}

// TestIsPremium_KnownKeyFalse verifies that IsPremium returns false when cache has
// the key set to false (feature graduated to free for all users).
func TestIsPremium_KnownKeyFalse(t *testing.T) {
	cache := newTestCacheWithGates(map[string]bool{
		featuregates.GateSharing: false,
	})

	got := cache.IsPremium(featuregates.GateSharing)
	assert.False(t, got, "IsPremium should return false when gate is premium=false")
}

// TestIsPremiumUnknownKey verifies that IsPremium returns true (safe default) for
// unknown gate keys — unknown means unreviewed, so we block access by default.
// This is the D-10 pitfall guard: unknown → true (premium required), not false.
func TestIsPremiumUnknownKey(t *testing.T) {
	cache := newTestCacheWithGates(map[string]bool{
		featuregates.GateSharing: true,
	})

	got := cache.IsPremium("non-existent-gate-key-xyz")
	assert.True(t, got, "IsPremium should return true (safe default) for unknown keys")
}

// TestIsPremiumUnknownKey_EmptyCache verifies the safe default even with an empty cache.
func TestIsPremiumUnknownKey_EmptyCache(t *testing.T) {
	cache := newTestCacheWithGates(map[string]bool{})

	got := cache.IsPremium("anything")
	assert.True(t, got, "IsPremium should return true (safe default) even with empty cache")
}

// TestPubSubInvalidationTriggersReload verifies that publishing to the PubSub
// channel causes the cache to reload its in-memory gates map.
func TestPubSubInvalidationTriggersReload(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	// Create cache with no DB — use a reload counter to verify reload fires
	reloadCount := 0
	var mu sync.Mutex
	onReload := func() {
		mu.Lock()
		reloadCount++
		mu.Unlock()
	}

	cache := featuregates.NewFeatureGateCacheForTest(rdb, onReload)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = cache.Start(ctx)
	require.NoError(t, err)

	// Give goroutine time to subscribe
	time.Sleep(50 * time.Millisecond)

	initialCount := func() int {
		mu.Lock()
		defer mu.Unlock()
		return reloadCount
	}()

	// Publish invalidation message
	mr.Publish(featuregates.PubSubChannel, "invalidate")

	// Wait for reload to fire (with timeout)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		count := reloadCount
		mu.Unlock()
		if count > initialCount {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	finalCount := reloadCount
	mu.Unlock()

	assert.Greater(t, finalCount, initialCount, "PubSub invalidation should trigger reload")
}

// TestPeriodicRefreshTriggersReload verifies that the 60s ticker triggers reload.
// We use a short interval in test mode to avoid long waits.
func TestPeriodicRefreshTriggersReload(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	reloadCount := 0
	var mu sync.Mutex
	onReload := func() {
		mu.Lock()
		reloadCount++
		mu.Unlock()
	}

	// Use short ticker interval for test
	cache := featuregates.NewFeatureGateCacheForTestWithInterval(rdb, onReload, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = cache.Start(ctx)
	require.NoError(t, err)

	// Wait for at least 2 ticker fires
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	count := reloadCount
	mu.Unlock()

	assert.GreaterOrEqual(t, count, 2, "periodic ticker should trigger reload multiple times")
}

// TestReloadMapsDBRowsCorrectly verifies that SetGates correctly populates the
// in-memory map (simulating what reload() does after reading DB rows).
func TestReloadMapsDBRowsCorrectly(t *testing.T) {
	cache := newTestCacheWithGates(map[string]bool{
		"sharing":  true,
		"emotes":   false,
		"overlays": true,
	})

	assert.True(t, cache.IsPremium("sharing"))
	assert.False(t, cache.IsPremium("emotes"))
	assert.True(t, cache.IsPremium("overlays"))
	// Unknown key still returns safe default
	assert.True(t, cache.IsPremium("unknown"))
}

// TestPubSubChannelConstant verifies the PubSub channel name is correct.
func TestPubSubChannelConstant(t *testing.T) {
	assert.Equal(t, "feature-gates:invalidate", featuregates.PubSubChannel)
}

// TestGateSharingConstant verifies the sharing gate key constant is correct.
func TestGateSharingConstant(t *testing.T) {
	assert.Equal(t, "sharing", featuregates.GateSharing)
}
