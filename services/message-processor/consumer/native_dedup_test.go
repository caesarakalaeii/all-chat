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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func twitchStreamMsg(t *testing.T, internalID, nativeID, text string) redis.XMessage {
	t.Helper()
	return twitchStreamEventMsg(t, internalID, nativeID, text, "")
}

// twitchStreamEventMsg builds a Twitch stream message with an explicit event type, so tests can
// cover chat notices (ADR-0046) as well as plain chat.
func twitchStreamEventMsg(t *testing.T, internalID, nativeID, text, eventType string) redis.XMessage {
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
		EventType: eventType,
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

// A chat notice (watch streak, announcement) carries the same native Twitch message id that a
// channel.chat.message for the same message would, so the two must collapse to one rendered message
// in EITHER arrival order (ADR-0046). Twitch documents the subscriptions as disjoint; keying on the
// native id makes that an assumption we cannot get wrong.
func TestProcessMessage_NativeIDDedup_ChatNoticeVersusChatMessage(t *testing.T) {
	tests := []struct {
		name   string
		first  string // event type of the copy that arrives first
		second string
	}{
		{name: "notice then chat message", first: "watch_streak", second: ""},
		{name: "chat message then notice", first: "", second: "watch_streak"},
		{name: "announcement then chat message", first: "announcement", second: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr, err := miniredis.Run()
			require.NoError(t, err)
			defer mr.Close()
			client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			defer client.Close()
			logger := zaptest.NewLogger(t)

			var calls int
			c := NewStreamConsumer(client, logger, sharedTestMetrics,
				func(ctx context.Context, msg *models.RawChatMessage) error { calls++; return nil },
				nil, registry.NewRedisDeletionBuffer(client, time.Hour), "host")
			c.SetNativeDeduplicator(dedup.NewDeduplicator(client, logger))

			ctx := context.Background()
			require.NoError(t, c.processMessage(ctx,
				twitchStreamEventMsg(t, "uuid-1", "native-1", "morning all", tt.first)))
			require.NoError(t, c.processMessage(ctx,
				twitchStreamEventMsg(t, "uuid-2", "native-1", "morning all", tt.second)))

			if calls != 1 {
				t.Fatalf("handler called %d times for one native message id, want 1", calls)
			}
		})
	}
}

// A moderator can delete a watch-streak message or an announcement like any other message. When the
// deletion races ahead of the message it is buffered and drained on arrival — but that drain used to
// be gated on plain chat only, so a notice (which arrives with an event type set) never drained it
// and the deleted message stayed visible on the overlay (ADR-0046).
func TestProcessMessage_BufferedDeletionDrainsForChatNotices(t *testing.T) {
	for _, eventType := range []string{"watch_streak", "announcement", ""} {
		name := eventType
		if name == "" {
			name = "plain_chat"
		}
		t.Run(name, func(t *testing.T) {
			mr, err := miniredis.Run()
			require.NoError(t, err)
			defer mr.Close()
			client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			defer client.Close()
			logger := zaptest.NewLogger(t)

			buffer := registry.NewRedisDeletionBuffer(client, time.Hour)
			msgIDs := registry.NewRedisRegistry(client, time.Hour)
			var handled []*models.RawChatMessage
			c := NewStreamConsumer(client, logger, sharedTestMetrics,
				func(ctx context.Context, msg *models.RawChatMessage) error {
					handled = append(handled, msg)
					return nil
				},
				msgIDs, buffer, "host")
			c.SetNativeDeduplicator(dedup.NewDeduplicator(client, logger))

			ctx := context.Background()

			// The listener registers native id → internal UUID at its capture point, before
			// publishing, so the drain can resolve which rendered message to remove.
			require.NoError(t, msgIDs.Add(ctx, "twitch", "somechannel", "native-early", "msg-uuid"))

			// The moderator's deletion arrives first and is buffered against the native id.
			require.NoError(t, buffer.Add(ctx, "twitch", "somechannel", "native-early", &models.RawChatMessage{
				MessageID: "del-uuid",
				Platform:  "twitch",
				ChannelID: "somechannel",
				Timestamp: time.Now().UTC(),
				EventType: "message_deletion",
				EventData: map[string]interface{}{"deletion_type": "single", "target_msg_id": "native-early"},
			}))

			// Then the message itself lands.
			require.NoError(t, c.processMessage(ctx,
				twitchStreamEventMsg(t, "msg-uuid", "native-early", "morning all", eventType)))

			// The buffered deletion must have drained: the buffer entry is gone...
			pending, err := buffer.Get(ctx, "twitch", "somechannel", "native-early")
			require.NoError(t, err)
			assert.Nil(t, pending, "buffered deletion was never drained, so the message stays visible")

			// ...and the deletion reached the handler alongside the message itself.
			var sawDeletion bool
			for _, m := range handled {
				if m.EventType == "message_deletion" {
					sawDeletion = true
				}
			}
			assert.True(t, sawDeletion, "the drained deletion must be applied")
		})
	}
}

// Deletion events carry no Tags["id"] (the target id lives in EventData), so widening dedup to
// events must never swallow them — two deletions in a row still both apply.
func TestProcessMessage_NativeIDDedup_DeletionEventsUnaffected(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer client.Close()
	logger := zaptest.NewLogger(t)

	var calls int
	c := NewStreamConsumer(client, logger, sharedTestMetrics,
		func(ctx context.Context, msg *models.RawChatMessage) error { calls++; return nil },
		nil, registry.NewRedisDeletionBuffer(client, time.Hour), "host")
	c.SetNativeDeduplicator(dedup.NewDeduplicator(client, logger))

	// Two separate clear events for the same channel — both must be processed.
	clear := func(id string) redis.XMessage {
		raw := &models.RawChatMessage{
			MessageID: id,
			Platform:  "twitch",
			ChannelID: "somechannel",
			Timestamp: time.Now().UTC(),
			EventType: "message_deletion",
			EventData: map[string]interface{}{"deletion_type": "clear"},
		}
		data, err := json.Marshal(raw)
		require.NoError(t, err)
		return redis.XMessage{ID: "0-1", Values: map[string]interface{}{"data": string(data)}}
	}

	ctx := context.Background()
	require.NoError(t, c.processMessage(ctx, clear("del-1")))
	require.NoError(t, c.processMessage(ctx, clear("del-2")))
	if calls != 2 {
		t.Fatalf("both deletion events must be applied, not deduped; handler called %d times, want 2", calls)
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
