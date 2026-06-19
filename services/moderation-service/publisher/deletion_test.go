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

package publisher

import (
	"context"
	"testing"

	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestBuildSingleDeletion(t *testing.T) {
	msg := BuildSingleDeletion("twitch", "SomeStreamer", "native-123", "uuid-abc")

	assert.Equal(t, "message_deletion", msg.EventType)
	assert.Equal(t, "twitch", msg.Platform)
	assert.Equal(t, "somestreamer", msg.ChannelID, "twitch channel keys must be lower-cased to match the registry/chat path")
	assert.NotEmpty(t, msg.MessageID, "the deletion event needs its own id")
	assert.False(t, msg.Timestamp.IsZero())
	assert.Equal(t, "single", msg.EventData["deletion_type"])
	assert.Equal(t, "native-123", msg.EventData["target_msg_id"])
	assert.Equal(t, "uuid-abc", msg.EventData["target_uuid"],
		"the internal uuid is carried so the consumer can match without the (twitch-only) registry")
}

func TestBuildSingleDeletion_OmitsEmptyTargetUUID(t *testing.T) {
	// With no internal uuid the field is omitted, so the consumer falls back to the
	// registry lookup (the native-deletion behaviour).
	msg := BuildSingleDeletion("twitch", "streamer", "native-123", "")
	_, hasUUID := msg.EventData["target_uuid"]
	assert.False(t, hasUUID, "an empty target_uuid must not be written into the event")
}

func TestBuildSingleDeletion_NonTwitchPreservesChannelCase(t *testing.T) {
	// YouTube channel ids ("UC...") are case-sensitive — lower-casing would break
	// the registry lookup and overlay routing.
	msg := BuildSingleDeletion("youtube", "UCabcDEF", "yt-msg-1", "uuid-1")
	assert.Equal(t, "UCabcDEF", msg.ChannelID)
}

func TestBuildBatchDeletion_TimeoutSetsBanDuration(t *testing.T) {
	msg := BuildBatchDeletion("twitch", "streamer", "user-9", "BadUser", 600)

	assert.Equal(t, "batch", msg.EventData["deletion_type"])
	assert.Equal(t, "user-9", msg.EventData["target_user_id"])
	assert.Equal(t, "BadUser", msg.EventData["target_username"])
	assert.Equal(t, 600, msg.EventData["ban_duration"], "a timeout must carry ban_duration so it isn't shown as a permanent ban")
	assert.Equal(t, "user-9", msg.UserID)
	assert.Equal(t, "BadUser", msg.Username)
}

func TestBuildBatchDeletion_BanOmitsBanDuration(t *testing.T) {
	msg := BuildBatchDeletion("twitch", "streamer", "user-9", "BadUser", 0)

	_, hasDuration := msg.EventData["ban_duration"]
	assert.False(t, hasDuration, "a permanent ban must omit ban_duration")
}

func TestPublish_WritesValidEventThroughPublishFunc(t *testing.T) {
	var captured []byte
	publishFn := func(_ context.Context, payload []byte) error {
		captured = payload // RingBufferPublisher.Publish calls publishFn synchronously on success
		return nil
	}

	p := newDeletionPublisher(publishFn, zap.NewNop(), prometheus.NewRegistry())
	defer p.Stop()

	err := p.Publish(context.Background(), BuildSingleDeletion("twitch", "streamer", "native-1", "uuid-1"))
	require.NoError(t, err)
	require.NotNil(t, captured, "publishFn must receive the serialised event synchronously")

	// The bytes on the wire must deserialise back into the model the consumer reads.
	round, err := mpmodels.ParseRawMessage(captured)
	require.NoError(t, err)
	assert.Equal(t, "message_deletion", round.EventType)
	assert.Equal(t, "twitch", round.Platform)
	assert.Equal(t, "streamer", round.ChannelID)
	assert.Equal(t, "single", round.EventData["deletion_type"])
	assert.Equal(t, "native-1", round.EventData["target_msg_id"])
}
