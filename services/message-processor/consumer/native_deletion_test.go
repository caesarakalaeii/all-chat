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
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/services/message-processor/normalizer"
	"github.com/caesar/all-chat/services/message-processor/registry"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// Altstream's listener writes no msgid registry entries (only the Twitch and
// YouTube listeners do), so a native single deletion, which names only the
// Altstream message id, must still resolve to the id the overlay renders.
// These tests run the consumer against real (miniredis) registry and deletion
// buffer, with a handler that normalizes the way cmd/main.go does, and assert
// on the unified frames an overlay would receive.

// pipelineConsumer returns a consumer plus the unified frames it published, in order.
func pipelineConsumer(t *testing.T) (*StreamConsumer, *[]*models.UnifiedChatMessage) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	var published []*models.UnifiedChatMessage
	altstream := normalizer.NewAltstreamNormalizer()
	handler := func(_ context.Context, raw *models.RawChatMessage) error {
		if raw.EventType == "message_deletion" {
			published = append(published, normalizer.NormalizeDeletion(raw))
			return nil
		}
		unified, err := altstream.Normalize(raw, raw.OverlayID)
		if err != nil {
			return err
		}
		published = append(published, unified)
		return nil
	}
	c := NewStreamConsumer(client, zaptest.NewLogger(t), sharedTestMetrics, handler,
		registry.NewRedisRegistry(client, time.Hour), registry.NewRedisDeletionBuffer(client, time.Hour), "host")
	return c, &published
}

func entry(t *testing.T, raw any) redis.XMessage {
	t.Helper()
	data, err := json.Marshal(raw)
	require.NoError(t, err)
	return redis.XMessage{ID: "0-1", Values: map[string]interface{}{"data": string(data)}}
}

// altstreamChat is a chat message as altstream-listener publishes it.
func altstreamChat(id string) *models.RawChatMessage {
	return &models.RawChatMessage{
		MessageID: id, Platform: "altstream", OverlayID: "overlay-1",
		ChannelID: "copygirl", ChannelName: "copygirl",
		UserID: "vildravn", Username: "Vildravn", Text: "choo choo",
		Timestamp: time.Now().UTC(),
		Tags:      map[string]string{"message_type": "message"},
	}
}

// altstreamDeletion is a deletion as altstream-listener publishes it; eventData
// carries deletion_type and, for single, target_msg_id (and target_uuid when
// it is the moderation-service reflect-back).
func altstreamDeletion(id string, eventData map[string]interface{}) *models.RawChatMessage {
	return &models.RawChatMessage{
		MessageID: id, Platform: "altstream", OverlayID: "overlay-1",
		ChannelID: "copygirl", ChannelName: "copygirl",
		Timestamp: time.Now().UTC(),
		EventType: "message_deletion", EventData: eventData,
	}
}

func deletionMeta(t *testing.T, m *models.UnifiedChatMessage) map[string]interface{} {
	t.Helper()
	require.NotNil(t, m.Event, "expected a deletion frame")
	require.Equal(t, "message_deletion", m.Event.Type)
	return m.Event.Metadata
}

func TestAltstreamNativeDeletion_EndToEnd(t *testing.T) {
	c, published := pipelineConsumer(t)
	ctx := context.Background()

	// 1. The message renders under its Altstream id.
	require.NoError(t, c.processMessage(ctx, entry(t, altstreamChat("840417196"))))
	require.Len(t, *published, 1)
	rendered := (*published)[0]
	require.Nil(t, rendered.Event)
	assert.Equal(t, "840417196", rendered.ID)

	// 2. Altstream's own delete event names only the platform id (targetId).
	require.NoError(t, c.processMessage(ctx, entry(t, altstreamDeletion("169457509",
		map[string]interface{}{"deletion_type": "single", "target_msg_id": "840417196"}))))
	require.Len(t, *published, 2, "a native deletion of a known message must be published, not buffered")
	assert.Equal(t, rendered.ID, deletionMeta(t, (*published)[1])["target_uuid"],
		"the overlay removes by target_uuid, which must be the rendered message's id")

	// 3. A deletion that already carries the internal id is trusted as is.
	require.NoError(t, c.processMessage(ctx, entry(t, altstreamDeletion("mod-1",
		map[string]interface{}{"deletion_type": "single", "target_msg_id": "840417196", "target_uuid": rendered.ID}))))
	require.Len(t, *published, 3)
	assert.Equal(t, rendered.ID, deletionMeta(t, (*published)[2])["target_uuid"])

	// 4. A clear wipes the channel.
	require.NoError(t, c.processMessage(ctx, entry(t, altstreamDeletion("793486440",
		map[string]interface{}{"deletion_type": "clear"}))))
	require.Len(t, *published, 4)
	assert.Equal(t, "clear", deletionMeta(t, (*published)[3])["deletion_type"])
}

// A delete that overtakes its message (the consumer processes a batch
// concurrently) is buffered, then applied once the message is published. The
// overlay must receive the message first: a deletion for a message it does
// not have yet is a no-op, and the message would then stay on screen.
func TestAltstreamNativeDeletion_DeleteBeforeMessage(t *testing.T) {
	c, published := pipelineConsumer(t)
	ctx := context.Background()

	require.NoError(t, c.processMessage(ctx, entry(t, altstreamDeletion("169457509",
		map[string]interface{}{"deletion_type": "single", "target_msg_id": "840417196"}))))
	assert.Empty(t, *published, "a deletion for an unseen message is buffered")

	require.NoError(t, c.processMessage(ctx, entry(t, altstreamChat("840417196"))))
	require.Len(t, *published, 2)
	assert.Nil(t, (*published)[0].Event, "the message is published first")
	assert.Equal(t, "840417196", deletionMeta(t, (*published)[1])["target_uuid"])
}

// Kick and GoodGame publish deletions with their fields in Tags and no
// event_type. That shape must be processed as a deletion, never as an empty
// chat message.
func TestTagShapedDeletion_IsADeletion(t *testing.T) {
	c, published := pipelineConsumer(t)
	ctx := context.Background()

	require.NoError(t, c.processMessage(ctx, entry(t, altstreamChat("840417196"))))
	require.NoError(t, c.processMessage(ctx, entry(t, &models.RawChatMessage{
		Platform: "altstream", OverlayID: "overlay-1", ChannelID: "copygirl", ChannelName: "copygirl",
		Timestamp: time.Now().UTC(),
		Tags: map[string]string{
			"event_type":    "message_deletion",
			"deletion_type": "single",
			"target_msg_id": "840417196",
		},
	})))

	require.Len(t, *published, 2)
	meta := deletionMeta(t, (*published)[1])
	assert.Equal(t, "single", meta["deletion_type"])
	assert.Equal(t, "840417196", meta["target_uuid"])
	assert.NotEmpty(t, (*published)[1].ID, "an id keeps two deletions in one second from deduplicating into one")
}
