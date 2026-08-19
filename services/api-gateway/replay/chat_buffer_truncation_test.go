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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A watermark older than the oldest surviving entry means messages between the
// two existed and are gone. That is the case the flag exists to surface: the
// client would otherwise receive a short replay and believe it was caught up.
func TestChatReplayBuffer_TruncatedWhenWatermarkPredatesBuffer(t *testing.T) {
	client, _ := setupTestRedis(t)
	defer client.Close()

	buf := NewRedisChatReplayBuffer(client, 5*time.Minute, 500)
	ctx := context.Background()
	overlayID := "ov-trunc-1"

	t0 := time.Unix(10_000, 0).UTC()
	for i := 0; i < 3; i++ {
		payload := []byte(fmt.Sprintf(`{"id":"%d"}`, i))
		require.NoError(t, buf.Add(ctx, overlayID, payload, t0.Add(time.Duration(i)*time.Second)))
	}

	// Ask from one minute before the oldest buffered message.
	stale := t0.Add(-time.Minute).UnixMilli()
	replayed, err := buf.GetSince(ctx, overlayID, stale)
	require.NoError(t, err)

	assert.True(t, replayed.Truncated,
		"a watermark predating the oldest entry must be reported as truncated")
	assert.Len(t, replayed.Messages, 3, "everything still held is returned regardless")
}

// A watermark inside the window the buffer still covers is fully served, so
// nothing is missing and the flag must stay false.
func TestChatReplayBuffer_NotTruncatedWhenWatermarkInsideWindow(t *testing.T) {
	client, _ := setupTestRedis(t)
	defer client.Close()

	buf := NewRedisChatReplayBuffer(client, 5*time.Minute, 500)
	ctx := context.Background()
	overlayID := "ov-trunc-2"

	t0 := time.Unix(20_000, 0).UTC()
	for i := 0; i < 5; i++ {
		payload := []byte(fmt.Sprintf(`{"id":"%d"}`, i))
		require.NoError(t, buf.Add(ctx, overlayID, payload, t0.Add(time.Duration(i)*time.Second)))
	}

	// Watermark sits on the second message, well inside the buffered range.
	replayed, err := buf.GetSince(ctx, overlayID, t0.Add(time.Second).UnixMilli())
	require.NoError(t, err)

	assert.False(t, replayed.Truncated, "a watermark inside the window loses nothing")
	assert.Len(t, replayed.Messages, 3)
}

// A watermark landing exactly on the oldest entry is not a gap: that message is
// the one the client already saw, and everything after it is still held.
func TestChatReplayBuffer_NotTruncatedAtExactOldestScore(t *testing.T) {
	client, _ := setupTestRedis(t)
	defer client.Close()

	buf := NewRedisChatReplayBuffer(client, 5*time.Minute, 500)
	ctx := context.Background()
	overlayID := "ov-trunc-3"

	t0 := time.Unix(30_000, 0).UTC()
	require.NoError(t, buf.Add(ctx, overlayID, []byte(`{"id":"a"}`), t0))
	require.NoError(t, buf.Add(ctx, overlayID, []byte(`{"id":"b"}`), t0.Add(time.Second)))

	replayed, err := buf.GetSince(ctx, overlayID, t0.UnixMilli())
	require.NoError(t, err)

	assert.False(t, replayed.Truncated, "watermark == oldest score is boundary-inclusive, not a gap")
	assert.Len(t, replayed.Messages, 1, "exclusive lower bound drops the message at the watermark")
}

// sinceMs <= 0 means "replay everything". The caller is making no claim about
// what it already saw, so there is no gap to measure and the flag must be false
// even though the buffer has certainly dropped older messages by now.
func TestChatReplayBuffer_ReplayEverythingIsNeverTruncated(t *testing.T) {
	client, _ := setupTestRedis(t)
	defer client.Close()

	buf := NewRedisChatReplayBuffer(client, 5*time.Minute, 500)
	ctx := context.Background()
	overlayID := "ov-trunc-4"

	t0 := time.Unix(40_000, 0).UTC()
	require.NoError(t, buf.Add(ctx, overlayID, []byte(`{"id":"a"}`), t0))

	for _, since := range []int64{0, -1} {
		replayed, err := buf.GetSince(ctx, overlayID, since)
		require.NoError(t, err)
		assert.False(t, replayed.Truncated, "since=%d means replay-everything, which cannot be truncated", since)
	}
}

// An empty buffer has no oldest entry to be older than, so a client with any
// watermark is told nothing is missing rather than that everything is.
func TestChatReplayBuffer_EmptyBufferIsNotTruncated(t *testing.T) {
	client, _ := setupTestRedis(t)
	defer client.Close()

	buf := NewRedisChatReplayBuffer(client, 5*time.Minute, 500)
	ctx := context.Background()

	replayed, err := buf.GetSince(ctx, "ov-trunc-empty", time.Unix(50_000, 0).UnixMilli())
	require.NoError(t, err)

	assert.False(t, replayed.Truncated)
	assert.Empty(t, replayed.Messages)
}

// The MaxEntries cap is the other way a replay goes short: a burst during a
// gap evicts the oldest entries even though the TTL window has not rolled.
func TestChatReplayBuffer_TruncatedByMaxEntriesOverflow(t *testing.T) {
	client, _ := setupTestRedis(t)
	defer client.Close()

	const maxEntries = 3
	buf := NewRedisChatReplayBuffer(client, 5*time.Minute, maxEntries)
	ctx := context.Background()
	overlayID := "ov-trunc-5"

	t0 := time.Unix(60_000, 0).UTC()
	// The client's watermark: it saw message 0 and then went away.
	watermark := t0.UnixMilli()

	for i := 0; i < 10; i++ {
		payload := []byte(fmt.Sprintf(`{"id":"%d"}`, i))
		require.NoError(t, buf.Add(ctx, overlayID, payload, t0.Add(time.Duration(i)*time.Second)))
	}

	replayed, err := buf.GetSince(ctx, overlayID, watermark)
	require.NoError(t, err)

	assert.True(t, replayed.Truncated,
		"messages 1..6 were evicted by the entry cap; the client must be told its replay is short")
	assert.Len(t, replayed.Messages, maxEntries)
}
