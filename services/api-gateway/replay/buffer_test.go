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

package replay

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	return client, mr
}

func TestReplayBuffer_AddAndGetSince(t *testing.T) {
	client, _ := setupTestRedis(t)
	defer client.Close()

	buffer := NewRedisDeletionReplayBuffer(client, 60*time.Second)
	ctx := context.Background()
	overlayID := "test-overlay-1"

	// Add deletion events at different timestamps
	event1 := &DeletionEvent{
		DeletionType: "single",
		TargetUUID:   "msg-1",
		Platform:     "twitch",
		Timestamp:    time.Unix(0, 1000*1000000), // 1000ms
	}
	event2 := &DeletionEvent{
		DeletionType: "single",
		TargetUUID:   "msg-2",
		Platform:     "twitch",
		Timestamp:    time.Unix(0, 2000*1000000), // 2000ms
	}
	event3 := &DeletionEvent{
		DeletionType: "batch",
		TargetUserID: "user-1",
		Platform:     "youtube",
		Timestamp:    time.Unix(0, 3000*1000000), // 3000ms
	}

	// Add events
	require.NoError(t, buffer.Add(ctx, overlayID, event1))
	require.NoError(t, buffer.Add(ctx, overlayID, event2))
	require.NoError(t, buffer.Add(ctx, overlayID, event3))

	// Query events since 1500ms (should get event2 and event3)
	events, err := buffer.GetSince(ctx, overlayID, 1500)
	require.NoError(t, err)
	require.Len(t, events, 2)

	assert.Equal(t, "msg-2", events[0].TargetUUID)
	assert.Equal(t, "user-1", events[1].TargetUserID)
}

func TestReplayBuffer_GetSinceExclusiveBound(t *testing.T) {
	client, _ := setupTestRedis(t)
	defer client.Close()

	buffer := NewRedisDeletionReplayBuffer(client, 60*time.Second)
	ctx := context.Background()
	overlayID := "test-overlay-2"

	// Add event at exact timestamp 2000ms
	event := &DeletionEvent{
		DeletionType: "single",
		TargetUUID:   "msg-1",
		Platform:     "kick",
		Timestamp:    time.Unix(0, 2000*1000000),
	}
	require.NoError(t, buffer.Add(ctx, overlayID, event))

	// Query with exact timestamp (exclusive) - should return empty
	events, err := buffer.GetSince(ctx, overlayID, 2000)
	require.NoError(t, err)
	assert.Empty(t, events, "Exclusive range should not include exact timestamp")

	// Query with timestamp just before - should return the event
	events, err = buffer.GetSince(ctx, overlayID, 1999)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "msg-1", events[0].TargetUUID)
}

func TestReplayBuffer_TTLExpiration(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer client.Close()

	buffer := NewRedisDeletionReplayBuffer(client, 60*time.Second)
	ctx := context.Background()
	overlayID := "test-overlay-3"

	// Add event
	event := &DeletionEvent{
		DeletionType: "single",
		TargetUUID:   "msg-1",
		Platform:     "twitch",
		Timestamp:    time.Now(),
	}
	require.NoError(t, buffer.Add(ctx, overlayID, event))

	// Verify event exists
	events, err := buffer.GetSince(ctx, overlayID, 0)
	require.NoError(t, err)
	require.Len(t, events, 1)

	// Fast forward 61 seconds
	mr.FastForward(61 * time.Second)

	// Event should be expired
	events, err = buffer.GetSince(ctx, overlayID, 0)
	require.NoError(t, err)
	assert.Empty(t, events, "Events should expire after TTL")
}

func TestReplayBuffer_EmptyBuffer(t *testing.T) {
	client, _ := setupTestRedis(t)
	defer client.Close()

	buffer := NewRedisDeletionReplayBuffer(client, 60*time.Second)
	ctx := context.Background()
	overlayID := "test-overlay-4"

	// Query empty buffer - should return empty array, not error
	events, err := buffer.GetSince(ctx, overlayID, 0)
	require.NoError(t, err)
	assert.NotNil(t, events)
	assert.Empty(t, events)
}

func TestReplayBuffer_MultipleOverlaysNoConflict(t *testing.T) {
	client, _ := setupTestRedis(t)
	defer client.Close()

	buffer := NewRedisDeletionReplayBuffer(client, 60*time.Second)
	ctx := context.Background()

	// Add events for different overlays
	event1 := &DeletionEvent{
		DeletionType: "single",
		TargetUUID:   "overlay1-msg",
		Platform:     "twitch",
		Timestamp:    time.Now(),
	}
	event2 := &DeletionEvent{
		DeletionType: "single",
		TargetUUID:   "overlay2-msg",
		Platform:     "youtube",
		Timestamp:    time.Now(),
	}

	require.NoError(t, buffer.Add(ctx, "overlay-1", event1))
	require.NoError(t, buffer.Add(ctx, "overlay-2", event2))

	// Query overlay-1 should only get overlay-1 events
	events, err := buffer.GetSince(ctx, "overlay-1", 0)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "overlay1-msg", events[0].TargetUUID)

	// Query overlay-2 should only get overlay-2 events
	events, err = buffer.GetSince(ctx, "overlay-2", 0)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "overlay2-msg", events[0].TargetUUID)
}

func TestReplayBuffer_Prune(t *testing.T) {
	client, _ := setupTestRedis(t)
	defer client.Close()

	buffer := NewRedisDeletionReplayBuffer(client, 60*time.Second)
	ctx := context.Background()
	overlayID := "test-overlay-5"

	// Add events at different timestamps
	oldEvent := &DeletionEvent{
		DeletionType: "single",
		TargetUUID:   "old-msg",
		Platform:     "twitch",
		Timestamp:    time.Unix(0, 1000*1000000), // 1000ms
	}
	newEvent := &DeletionEvent{
		DeletionType: "single",
		TargetUUID:   "new-msg",
		Platform:     "twitch",
		Timestamp:    time.Unix(0, 3000*1000000), // 3000ms
	}

	require.NoError(t, buffer.Add(ctx, overlayID, oldEvent))
	require.NoError(t, buffer.Add(ctx, overlayID, newEvent))

	// Prune events older than 2000ms
	require.NoError(t, buffer.Prune(ctx, overlayID, 2000))

	// Only new event should remain
	events, err := buffer.GetSince(ctx, overlayID, 0)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "new-msg", events[0].TargetUUID)
}

func TestReplayBuffer_MalformedEvent(t *testing.T) {
	client, mr := setupTestRedis(t)
	defer client.Close()

	ctx := context.Background()
	overlayID := "test-overlay-6"
	key := "replay:deletions:" + overlayID

	// Manually add malformed JSON to sorted set
	mr.ZAdd(key, 1000, "not-valid-json")
	mr.ZAdd(key, 2000, `{"type":"single","target_uuid":"valid-msg","platform":"twitch","timestamp":"2026-01-01T00:00:00Z"}`)

	buffer := NewRedisDeletionReplayBuffer(client, 60*time.Second)

	// Should skip malformed event and continue processing
	events, err := buffer.GetSince(ctx, overlayID, 0)
	require.NoError(t, err)
	require.Len(t, events, 1, "Should skip malformed event and return valid one")
	assert.Equal(t, "valid-msg", events[0].TargetUUID)
}
