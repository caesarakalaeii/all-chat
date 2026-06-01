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

package listener_test

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/caesar/all-chat/shared/listener"
	"github.com/caesar/all-chat/shared/listener/testutil/redisutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"go.uber.org/zap"
)

// demandCapturingManager records UpdateDemandedSourceIDs calls for test assertions.
type demandCapturingManager struct {
	mu              sync.Mutex
	demandCalls     []map[string]listener.DemandedSource
	demandCallCount atomic.Int64
}

func (m *demandCapturingManager) Start(_ context.Context) error                                { return nil }
func (m *demandCapturingManager) Stop()                                                        {}
func (m *demandCapturingManager) UpdateAssignedSourceIDs(_ map[string]bool)                   {}
func (m *demandCapturingManager) UpdateDemandedSourceIDs(demanded map[string]listener.DemandedSource) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Store a copy so callers can inspect it.
	cp := make(map[string]listener.DemandedSource, len(demanded))
	for k, v := range demanded {
		cp[k] = v
	}
	m.demandCalls = append(m.demandCalls, cp)
	m.demandCallCount.Add(1)
}
func (m *demandCapturingManager) GetFilteredAssignmentCount() int { return 0 }
func (m *demandCapturingManager) GetActiveChannels() []string     { return nil }
func (m *demandCapturingManager) GetActiveChannelCount() int      { return 0 }

// getLastDemandCall returns the last argument passed to UpdateDemandedSourceIDs.
func (m *demandCapturingManager) getLastDemandCall() (map[string]listener.DemandedSource, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.demandCalls) == 0 {
		return nil, false
	}
	return m.demandCalls[len(m.demandCalls)-1], true
}

// demandUpdateJSON is a convenience helper that returns a JSON-encoded demand update
// matching the source:demand Pub/Sub wire format.
func demandUpdateJSON(sources []map[string]string) string {
	type src struct {
		SourceID  string `json:"source_id"`
		ChannelID string `json:"channel_id"`
		Platform  string `json:"platform"`
		OverlayID string `json:"overlay_id"`
	}
	type update struct {
		Type      string `json:"type"`
		Sources   []src  `json:"sources"`
		Timestamp string `json:"timestamp"`
	}
	var srcs []src
	for _, s := range sources {
		srcs = append(srcs, src{
			SourceID:  s["source_id"],
			ChannelID: s["channel_id"],
			Platform:  s["platform"],
			OverlayID: s["overlay_id"],
		})
	}
	b, _ := json.Marshal(update{Type: "demand_update", Sources: srcs, Timestamp: time.Now().Format(time.RFC3339)})
	return string(b)
}

// TestDemandFiltering verifies that demand updates are filtered by platform only.
// Sources from other platforms are excluded; sources from this platform pass through.
func TestDemandFiltering(t *testing.T) {
	mr, rc := redisutil.StartTestRedisWithClient(t)
	defer func() {
		rc.Close()
		mr.Close()
		time.Sleep(10 * time.Millisecond) // let miniredis goroutines drain
		goleak.VerifyNone(t)
	}()

	ll, err := listener.NewLeadershipListener(listener.LeadershipConfig{
		Platform: "kick",
	}, rc, zap.NewNop())
	require.NoError(t, err)

	mgr := &demandCapturingManager{}

	ctx, cancel := context.WithCancel(context.Background())

	require.NoError(t, ll.Start(ctx, mgr))

	// Give the demand subscriber goroutine time to establish its Redis subscription.
	time.Sleep(50 * time.Millisecond)

	// Publish demand update containing kick sources and a twitch source.
	// Only kick sources should pass through the platform filter.
	payload := demandUpdateJSON([]map[string]string{
		{"source_id": "A", "channel_id": "chan-a", "platform": "kick", "overlay_id": "ov-1"},
		{"source_id": "D", "channel_id": "chan-d", "platform": "twitch", "overlay_id": "ov-2"},
	})

	require.NoError(t, rc.Publish(ctx, "source:demand", payload).Err())

	// Wait for the demand loop to process the message.
	require.Eventually(t, func() bool {
		return mgr.demandCallCount.Load() >= 1
	}, 3*time.Second, 20*time.Millisecond, "UpdateDemandedSourceIDs should have been called")

	got, ok := mgr.getLastDemandCall()
	require.True(t, ok)

	assert.Contains(t, got, "A", "A is a kick source — must be in result")
	assert.NotContains(t, got, "D", "D is a twitch source — must NOT be in result after platform filter")

	cancel()
	ll.Stop()
}

// TestDemandPlatformOverride verifies that DemandPlatform overrides the platform used to
// filter demand updates, independently of the leadership Platform. This is the
// twitch-eventsub case: it coordinates as "twitch-eventsub" but must match "twitch" sources.
func TestDemandPlatformOverride(t *testing.T) {
	mr, rc := redisutil.StartTestRedisWithClient(t)
	defer func() {
		rc.Close()
		mr.Close()
		time.Sleep(10 * time.Millisecond)
		goleak.VerifyNone(t)
	}()

	ll, err := listener.NewLeadershipListener(listener.LeadershipConfig{
		Platform:       "twitch-eventsub",
		DemandPlatform: "twitch",
	}, rc, zap.NewNop())
	require.NoError(t, err)

	mgr := &demandCapturingManager{}
	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, ll.Start(ctx, mgr))
	time.Sleep(50 * time.Millisecond)

	// A "twitch" source must match (via DemandPlatform); a literal "twitch-eventsub" source must not.
	payload := demandUpdateJSON([]map[string]string{
		{"source_id": "T", "channel_id": "chan-t", "platform": "twitch", "overlay_id": "ov-1"},
		{"source_id": "E", "channel_id": "chan-e", "platform": "twitch-eventsub", "overlay_id": "ov-2"},
	})
	require.NoError(t, rc.Publish(ctx, "source:demand", payload).Err())

	require.Eventually(t, func() bool {
		return mgr.demandCallCount.Load() >= 1
	}, 3*time.Second, 20*time.Millisecond, "UpdateDemandedSourceIDs should have been called")

	got, ok := mgr.getLastDemandCall()
	require.True(t, ok)
	assert.Contains(t, got, "T", "twitch source must match the DemandPlatform override")
	assert.NotContains(t, got, "E", "twitch-eventsub source must NOT match when DemandPlatform=twitch")

	cancel()
	ll.Stop()
}

// TestDemandWithDisableFiltering verifies that when DisableDemandFiltering=true,
// the demand loop exits early without subscribing to Redis.
// The manager's UpdateDemandedSourceIDs must not be called via the demand loop.
func TestDemandWithDisableFiltering(t *testing.T) {
	defer goleak.VerifyNone(t)

	ll, err := listener.NewLeadershipListener(listener.LeadershipConfig{
		DisableDemandFiltering: true,
	}, nil, zap.NewNop())
	require.NoError(t, err)

	mgr := &demandCapturingManager{}

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, ll.Start(ctx, mgr))

	// Give a brief window — UpdateDemandedSourceIDs must NOT be called.
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int64(0), mgr.demandCallCount.Load(),
		"DisableDemandFiltering=true: UpdateDemandedSourceIDs must NOT be called by demand loop")

	cancel()
	ll.Stop()
}

// TestDemandEmptySources verifies that a DemandUpdate with an empty sources array
// calls UpdateDemandedSourceIDs with an empty map, causing all listeners to disconnect.
func TestDemandEmptySources(t *testing.T) {
	mr, rc := redisutil.StartTestRedisWithClient(t)
	defer func() {
		rc.Close()
		mr.Close()
		time.Sleep(10 * time.Millisecond)
		goleak.VerifyNone(t)
	}()

	ll, err := listener.NewLeadershipListener(listener.LeadershipConfig{}, rc, zap.NewNop())
	require.NoError(t, err)

	mgr := &demandCapturingManager{}

	ctx, cancel := context.WithCancel(context.Background())

	require.NoError(t, ll.Start(ctx, mgr))

	// Give the demand subscriber goroutine time to establish its Redis subscription.
	time.Sleep(50 * time.Millisecond)

	// Publish demand update with empty sources.
	payload := demandUpdateJSON(nil)
	require.NoError(t, rc.Publish(ctx, "source:demand", payload).Err())

	require.Eventually(t, func() bool {
		return mgr.demandCallCount.Load() >= 1
	}, 3*time.Second, 20*time.Millisecond)

	got, ok := mgr.getLastDemandCall()
	require.True(t, ok)
	assert.Empty(t, got, "empty sources must result in empty demanded map")

	cancel()
	ll.Stop()
}

// TestDemandSubscriberReconnect verifies that after a simulated disconnect (context cancel +
// restart), the demand state is restored from the next DemandUpdate.
func TestDemandSubscriberReconnect(t *testing.T) {
	mr, rc := redisutil.StartTestRedisWithClient(t)
	defer func() {
		rc.Close()
		mr.Close()
		time.Sleep(10 * time.Millisecond)
		goleak.VerifyNone(t)
	}()

	mgr := &demandCapturingManager{}

	// First run.
	ll1, err := listener.NewLeadershipListener(listener.LeadershipConfig{}, rc, zap.NewNop())
	require.NoError(t, err)

	ctx1, cancel1 := context.WithCancel(context.Background())

	require.NoError(t, ll1.Start(ctx1, mgr))

	// Give the demand subscriber goroutine time to establish its Redis subscription.
	time.Sleep(50 * time.Millisecond)

	// Publish and confirm receipt.
	payload := demandUpdateJSON([]map[string]string{
		{"source_id": "A", "channel_id": "chan-a", "platform": "", "overlay_id": "ov-1"},
	})
	require.NoError(t, rc.Publish(ctx1, "source:demand", payload).Err())

	require.Eventually(t, func() bool {
		return mgr.demandCallCount.Load() >= 1
	}, 3*time.Second, 20*time.Millisecond)

	// Simulate disconnect: stop the first listener.
	cancel1()
	ll1.Stop()

	prevCount := mgr.demandCallCount.Load()

	// Second run — fresh listener, same manager.
	ll2, err := listener.NewLeadershipListener(listener.LeadershipConfig{}, rc, zap.NewNop())
	require.NoError(t, err)

	ctx2, cancel2 := context.WithCancel(context.Background())

	require.NoError(t, ll2.Start(ctx2, mgr))

	// Give the demand subscriber goroutine time to establish its Redis subscription.
	time.Sleep(50 * time.Millisecond)

	// Publish again; demand loop should process it and call UpdateDemandedSourceIDs again.
	require.NoError(t, rc.Publish(ctx2, "source:demand", payload).Err())

	require.Eventually(t, func() bool {
		return mgr.demandCallCount.Load() > prevCount
	}, 3*time.Second, 20*time.Millisecond, "demand state should be restored after reconnect")

	cancel2()
	ll2.Stop()
}

// TestDemandNoPlatformFilter verifies that when Platform is empty, all sources pass through.
func TestDemandNoPlatformFilter(t *testing.T) {
	mr, rc := redisutil.StartTestRedisWithClient(t)
	defer func() {
		rc.Close()
		mr.Close()
		time.Sleep(10 * time.Millisecond)
		goleak.VerifyNone(t)
	}()

	ll, err := listener.NewLeadershipListener(listener.LeadershipConfig{
		Platform: "", // no platform filter
	}, rc, zap.NewNop())
	require.NoError(t, err)

	mgr := &demandCapturingManager{}

	ctx, cancel := context.WithCancel(context.Background())
	require.NoError(t, ll.Start(ctx, mgr))

	time.Sleep(50 * time.Millisecond)

	payload := demandUpdateJSON([]map[string]string{
		{"source_id": "A", "channel_id": "chan-a", "platform": "kick", "overlay_id": "ov-1"},
		{"source_id": "B", "channel_id": "chan-b", "platform": "twitch", "overlay_id": "ov-2"},
		{"source_id": "C", "channel_id": "chan-c", "platform": "youtube", "overlay_id": "ov-3"},
	})
	require.NoError(t, rc.Publish(ctx, "source:demand", payload).Err())

	require.Eventually(t, func() bool {
		return mgr.demandCallCount.Load() >= 1
	}, 3*time.Second, 20*time.Millisecond)

	got, ok := mgr.getLastDemandCall()
	require.True(t, ok)
	assert.Contains(t, got, "A", "A should pass through with no platform filter")
	assert.Contains(t, got, "B", "B should pass through with no platform filter")
	assert.Contains(t, got, "C", "C should pass through with no platform filter")

	cancel()
	ll.Stop()
}
