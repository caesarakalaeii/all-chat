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

package normalizer

import (
	"testing"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeInstagramRaw builds a RawChatMessage in the exact shape the
// instagram-listener publishes to chat:raw.
func makeInstagramRaw() *models.RawChatMessage {
	return &models.RawChatMessage{
		MessageID:   "comment_1789",
		Platform:    "instagram",
		ChannelID:   "17841400000000001",
		ChannelName: "mygaminghandle",
		UserID:      "user_9876",
		Username:    "ig_viewer",
		Text:        "that clip is insane",
		Timestamp:   time.Date(2026, 9, 1, 10, 0, 5, 0, time.UTC),
		Tags:        map[string]string{},
	}
}

// TestInstagramNormalizer_HappyPath pins the listener→processor contract
// against the live_comments shape documented at
// https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/live-comments/.
func TestInstagramNormalizer_HappyPath(t *testing.T) {
	n := NewInstagramNormalizer()
	raw := makeInstagramRaw()

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)

	assert.Equal(t, "comment_1789", unified.ID)
	assert.Equal(t, "overlay-1", unified.OverlayID)
	assert.Equal(t, "instagram", unified.Platform)
	assert.Equal(t, "17841400000000001", unified.ChannelID)
	assert.Equal(t, "mygaminghandle", unified.ChannelName)
	assert.Equal(t, "user_9876", unified.User.ID)
	assert.Equal(t, "ig_viewer", unified.User.Username)
	assert.Equal(t, "ig_viewer", unified.User.DisplayName)
	assert.Equal(t, "that clip is insane", unified.Message.Text)
	assert.Empty(t, unified.User.Badges)
	assert.Empty(t, unified.User.Color)
	assert.Equal(t, time.Date(2026, 9, 1, 10, 0, 5, 0, time.UTC), unified.Timestamp)
	assert.Empty(t, unified.Message.Emotes)

	assert.Equal(t, "comment_1789", unified.Metadata["instagram_comment_id"])
	assert.Equal(t, 0, unified.Metadata["bits"])
	assert.Equal(t, false, unified.Metadata["is_subscriber"])
	assert.Equal(t, false, unified.Metadata["is_turbo"])
}

// TestInstagramNormalizer_MissingUsernameFallsBack covers the empty-username
// shape: live_comments can omit username, so the commenter id must stand in.
func TestInstagramNormalizer_MissingUsernameFallsBack(t *testing.T) {
	n := NewInstagramNormalizer()
	raw := makeInstagramRaw()
	raw.Username = ""

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)
	assert.Equal(t, "user_9876", unified.User.Username)
	assert.Equal(t, "user_9876", unified.User.DisplayName)
}

// TestInstagramNormalizer_WrongPlatform mirrors the other normalizers' guard.
func TestInstagramNormalizer_WrongPlatform(t *testing.T) {
	n := NewInstagramNormalizer()
	raw := makeInstagramRaw()
	raw.Platform = "twitch"
	_, err := n.Normalize(raw, "overlay-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported platform")
}

// TestInstagramNormalizer_EmptyChannelID fails closed on a missing channel id.
func TestInstagramNormalizer_EmptyChannelID(t *testing.T) {
	n := NewInstagramNormalizer()
	raw := makeInstagramRaw()
	raw.ChannelID = ""
	_, err := n.Normalize(raw, "overlay-1")
	require.Error(t, err)
}

// TestInstagramNormalizer_EmptyFieldsFallBack covers defensive defaults:
// zero timestamp and a missing channel_name must not panic or blank out.
func TestInstagramNormalizer_EmptyFieldsFallBack(t *testing.T) {
	n := NewInstagramNormalizer()
	raw := makeInstagramRaw()
	raw.ChannelName = ""
	raw.Timestamp = time.Time{}
	raw.UserID = ""
	raw.Username = ""

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)
	assert.Equal(t, "17841400000000001", unified.ChannelName)
	assert.False(t, unified.Timestamp.IsZero())
}

// TestInstagramNormalizer_ParsesWireJSON feeds the exact JSON the
// instagram-listener publishes to chat:raw through the real deserialization
// path (ParseRawMessage -> Normalize), pinning the cross-service contract.
func TestInstagramNormalizer_ParsesWireJSON(t *testing.T) {
	wire := []byte(`{
		"message_id":"comment_1789","platform":"instagram","overlay_id":"","channel_id":"17841400000000001",
		"channel_name":"mygaminghandle","user_id":"user_9876","username":"ig_viewer",
		"text":"first!","tags":{},
		"timestamp":"2026-09-01T10:00:05Z"
	}`)

	raw, err := models.ParseRawMessage(wire)
	require.NoError(t, err)

	unified, err := NewInstagramNormalizer().Normalize(raw, "overlay-1")
	require.NoError(t, err)
	assert.Equal(t, "comment_1789", unified.ID)
	assert.Equal(t, "instagram", unified.Platform)
	assert.Equal(t, "first!", unified.Message.Text)
	assert.Equal(t, "comment_1789", unified.Metadata["instagram_comment_id"])
}
