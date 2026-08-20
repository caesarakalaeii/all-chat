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

func TestChatReplayBuffer_AddAndReplayAll(t *testing.T) {
	client, _ := setupTestRedis(t)
	defer client.Close()

	buf := NewRedisChatReplayBuffer(client, 5*time.Minute, 500)
	ctx := context.Background()
	overlayID := "ov-chat-1"

	t0 := time.Unix(1000, 0).UTC()
	for i := 0; i < 3; i++ {
		payload := []byte(fmt.Sprintf(`{"type":"chat_message","data":{"id":"msg-%d"}}`, i))
		require.NoError(t, buf.Add(ctx, overlayID, payload, t0.Add(time.Duration(i)*time.Second)))
	}

	replayed, err := buf.GetSince(ctx, overlayID, 0)
	require.NoError(t, err)
	require.Len(t, replayed.Messages, 3)
	assert.Contains(t, string(replayed.Messages[0]), `"id":"msg-0"`)
	assert.Contains(t, string(replayed.Messages[1]), `"id":"msg-1"`)
	assert.Contains(t, string(replayed.Messages[2]), `"id":"msg-2"`)
}

func TestChatReplayBuffer_GetSinceFiltersOlder(t *testing.T) {
	client, _ := setupTestRedis(t)
	defer client.Close()

	buf := NewRedisChatReplayBuffer(client, 5*time.Minute, 500)
	ctx := context.Background()
	overlayID := "ov-chat-2"

	t0 := time.Unix(2000, 0).UTC()
	for i := 0; i < 5; i++ {
		payload := []byte(fmt.Sprintf(`{"id":"%d"}`, i))
		require.NoError(t, buf.Add(ctx, overlayID, payload, t0.Add(time.Duration(i)*time.Second)))
	}

	// Ask for messages strictly after the second message's timestamp (i=1).
	since := t0.Add(1 * time.Second).UnixMilli()
	replayed, err := buf.GetSince(ctx, overlayID, since)
	require.NoError(t, err)
	require.Len(t, replayed.Messages, 3, "should return msgs 2,3,4 — exclusive lower bound drops msg 1")
}

func TestChatReplayBuffer_DuplicateMillisecondsBothPreserved(t *testing.T) {
	client, _ := setupTestRedis(t)
	defer client.Close()

	buf := NewRedisChatReplayBuffer(client, 5*time.Minute, 500)
	ctx := context.Background()
	overlayID := "ov-chat-3"

	// Two messages at the same ms but different ns — both should be retained.
	base := time.Date(2026, 5, 15, 0, 0, 0, 100_000, time.UTC)
	require.NoError(t, buf.Add(ctx, overlayID, []byte(`{"id":"a"}`), base))
	require.NoError(t, buf.Add(ctx, overlayID, []byte(`{"id":"b"}`), base.Add(500*time.Nanosecond)))

	replayed, err := buf.GetSince(ctx, overlayID, 0)
	require.NoError(t, err)
	assert.Len(t, replayed.Messages, 2, "both messages at the same ms should be preserved via nanosecond suffix")
}

func TestChatReplayBuffer_TrimsToMaxEntries(t *testing.T) {
	client, _ := setupTestRedis(t)
	defer client.Close()

	const cap = 3
	buf := NewRedisChatReplayBuffer(client, 5*time.Minute, cap)
	ctx := context.Background()
	overlayID := "ov-chat-4"

	t0 := time.Unix(3000, 0).UTC()
	for i := 0; i < 10; i++ {
		payload := []byte(fmt.Sprintf(`{"id":"%d"}`, i))
		require.NoError(t, buf.Add(ctx, overlayID, payload, t0.Add(time.Duration(i)*time.Second)))
	}

	replayed, err := buf.GetSince(ctx, overlayID, 0)
	require.NoError(t, err)
	require.Len(t, replayed.Messages, cap, "buffer must trim to MaxEntries")
	// Most recent should remain (ids 7, 8, 9).
	assert.Contains(t, string(replayed.Messages[0]), `"id":"7"`)
	assert.Contains(t, string(replayed.Messages[2]), `"id":"9"`)
}

func TestChatReplayBuffer_EmptyOverlayReturnsNil(t *testing.T) {
	client, _ := setupTestRedis(t)
	defer client.Close()

	buf := NewRedisChatReplayBuffer(client, 5*time.Minute, 500)
	ctx := context.Background()

	replayed, err := buf.GetSince(ctx, "no-such-overlay", 0)
	require.NoError(t, err)
	assert.Empty(t, replayed.Messages)
}

func TestChatReplayBuffer_AddOnceSuppressesDuplicates(t *testing.T) {
	client, _ := setupTestRedis(t)
	defer client.Close()

	buf := NewRedisChatReplayBuffer(client, 5*time.Minute, 500)
	ctx := context.Background()
	overlayID := "ov-chat-addonce"

	t0 := time.Unix(4000, 0).UTC()
	payload := []byte(`{"type":"chat_message","data":{"id":"msg-abc"}}`)

	// First pod sees it: should buffer.
	added, err := buf.AddOnce(ctx, overlayID, "msg-abc", payload, t0)
	require.NoError(t, err)
	require.True(t, added)

	// Second pod tries the same id: must be skipped.
	added, err = buf.AddOnce(ctx, overlayID, "msg-abc", payload, t0.Add(time.Millisecond))
	require.NoError(t, err)
	require.False(t, added)

	replayed, err := buf.GetSince(ctx, overlayID, 0)
	require.NoError(t, err)
	require.Len(t, replayed.Messages, 1, "duplicate id must only produce one buffer entry")
}

func TestChatReplayBuffer_AddOnceEmptyIDFallsBack(t *testing.T) {
	client, _ := setupTestRedis(t)
	defer client.Close()

	buf := NewRedisChatReplayBuffer(client, 5*time.Minute, 500)
	ctx := context.Background()
	overlayID := "ov-chat-addonce-noid"

	t0 := time.Unix(5000, 0).UTC()

	// Two messages with no stable ID — both should buffer (no dedup possible).
	added1, err1 := buf.AddOnce(ctx, overlayID, "", []byte(`{"id":"a"}`), t0)
	require.NoError(t, err1)
	require.True(t, added1)

	added2, err2 := buf.AddOnce(ctx, overlayID, "", []byte(`{"id":"b"}`), t0.Add(time.Second))
	require.NoError(t, err2)
	require.True(t, added2)

	replayed, err := buf.GetSince(ctx, overlayID, 0)
	require.NoError(t, err)
	require.Len(t, replayed.Messages, 2)
}
