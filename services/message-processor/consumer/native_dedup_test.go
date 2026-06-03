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

package consumer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/message-processor/dedup"
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/services/message-processor/registry"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func twitchStreamMsg(t *testing.T, internalID, nativeID, text string) redis.XMessage {
	t.Helper()
	raw := &models.RawChatMessage{
		MessageID: internalID, // internal UUID — differs between the IRC and EventSub copies
		Platform:  "twitch",
		ChannelID: "somechannel",
		UserID:    "42",
		Username:  "viewer",
		Text:      text,
		Timestamp: time.Now().UTC(),
		Tags:      map[string]string{"id": nativeID}, // native Twitch id — identical across both paths
	}
	data, err := json.Marshal(raw)
	require.NoError(t, err)
	return redis.XMessage{ID: "0-1", Values: map[string]interface{}{"data": string(data)}}
}

// The same native Twitch id arriving twice (the IRC↔EventSub handoff overlap, or a Twitch webhook
// retry) must reach the handler only once; a different native id passes through (ADR-0015).
func TestProcessMessage_NativeIDDedup(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	logger := zaptest.NewLogger(t)

	var calls int
	handler := func(ctx context.Context, msg *models.RawChatMessage) error { calls++; return nil }

	c := NewStreamConsumer(client, logger, sharedTestMetrics, handler, nil,
		registry.NewRedisDeletionBuffer(client, time.Hour), "host")
	c.SetNativeDeduplicator(dedup.NewDeduplicator(client, logger))

	ctx := context.Background()

	// IRC copy of message "abc".
	require.NoError(t, c.processMessage(ctx, twitchStreamMsg(t, "irc-uuid", "abc", "hello")))
	// EventSub copy of the SAME native message (different internal UUID) — must be dropped.
	require.NoError(t, c.processMessage(ctx, twitchStreamMsg(t, "eventsub-uuid", "abc", "hello")))
	if calls != 1 {
		t.Fatalf("handler called %d times for duplicate native id, want 1", calls)
	}

	// A genuinely different message passes through.
	require.NoError(t, c.processMessage(ctx, twitchStreamMsg(t, "irc-uuid-2", "def", "world")))
	if calls != 2 {
		t.Fatalf("handler called %d times after distinct native id, want 2", calls)
	}
}

// Without a native id (e.g. a malformed/legacy message), dedup must not drop the message.
func TestProcessMessage_NativeIDDedup_EmptyIDPassesThrough(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()

	var calls int
	c := NewStreamConsumer(client, zaptest.NewLogger(t), sharedTestMetrics,
		func(ctx context.Context, msg *models.RawChatMessage) error { calls++; return nil },
		nil, registry.NewRedisDeletionBuffer(client, time.Hour), "host")
	c.SetNativeDeduplicator(dedup.NewDeduplicator(client, zaptest.NewLogger(t)))

	ctx := context.Background()
	require.NoError(t, c.processMessage(ctx, twitchStreamMsg(t, "u1", "", "no native id")))
	require.NoError(t, c.processMessage(ctx, twitchStreamMsg(t, "u2", "", "no native id")))
	if calls != 2 {
		t.Fatalf("messages without a native id must not be deduped; handler called %d times, want 2", calls)
	}
}
