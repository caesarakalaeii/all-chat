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

// makeFacebookRaw builds a RawChatMessage in the exact shape the
// facebook-listener publishes to chat:raw.
func makeFacebookRaw() *models.RawChatMessage {
	return &models.RawChatMessage{
		MessageID:   "comment_1021",
		Platform:    "facebook",
		ChannelID:   "1122334455",
		ChannelName: "My Gaming Page",
		UserID:      "user_555",
		Username:    "Bea Commenter",
		Text:        "gg wp, that clutch was wild",
		Timestamp:   time.Date(2026, 9, 1, 10, 0, 5, 0, time.UTC),
		Tags: map[string]string{
			"live_video_id": "vid_777",
		},
	}
}

// TestFacebookNormalizer_HappyPath pins the listener→processor contract
// against the Graph comment shape documented at
// https://developers.facebook.com/docs/graph-api/reference/live-video/comments/.
func TestFacebookNormalizer_HappyPath(t *testing.T) {
	n := NewFacebookNormalizer()
	raw := makeFacebookRaw()

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)

	assert.Equal(t, "comment_1021", unified.ID)
	assert.Equal(t, "overlay-1", unified.OverlayID)
	assert.Equal(t, "facebook", unified.Platform)
	assert.Equal(t, "1122334455", unified.ChannelID)
	assert.Equal(t, "My Gaming Page", unified.ChannelName)
	assert.Equal(t, "user_555", unified.User.ID)
	assert.Equal(t, "Bea Commenter", unified.User.Username)
	assert.Equal(t, "Bea Commenter", unified.User.DisplayName)
	assert.Equal(t, "gg wp, that clutch was wild", unified.Message.Text)
	assert.Empty(t, unified.User.Badges)
	assert.Empty(t, unified.Message.Emotes)

	// Moderation-relevant ids must survive normalization.
	assert.Equal(t, "comment_1021", unified.Metadata["facebook_comment_id"])
	assert.Equal(t, "vid_777", unified.Metadata["live_video_id"])
	assert.NotContains(t, unified.Metadata, "parent_comment_id")
}

// TestFacebookNormalizer_ReplyCarriesParent covers the reply shape: a
// from{id,name} + parent{id} comment as Graph returns it for comment replies.
func TestFacebookNormalizer_ReplyCarriesParent(t *testing.T) {
	n := NewFacebookNormalizer()
	raw := makeFacebookRaw()
	raw.Tags["parent_comment_id"] = "comment_1000"

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)
	assert.Equal(t, "comment_1000", unified.Metadata["parent_comment_id"])
}

// TestFacebookNormalizer_WrongPlatform mirrors the other normalizers' guard.
func TestFacebookNormalizer_WrongPlatform(t *testing.T) {
	n := NewFacebookNormalizer()
	raw := makeFacebookRaw()
	raw.Platform = "twitch"
	_, err := n.Normalize(raw, "overlay-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported platform")
}

// TestFacebookNormalizer_EmptyChannelID fails closed on a missing channel id.
func TestFacebookNormalizer_EmptyChannelID(t *testing.T) {
	n := NewFacebookNormalizer()
	raw := makeFacebookRaw()
	raw.ChannelID = ""
	_, err := n.Normalize(raw, "overlay-1")
	require.Error(t, err)
}

// TestFacebookNormalizer_EmptyFieldsFallBack covers defensive defaults:
// zero timestamp and a missing channel_name must not panic or blank out.
func TestFacebookNormalizer_EmptyFieldsFallBack(t *testing.T) {
	n := NewFacebookNormalizer()
	raw := makeFacebookRaw()
	raw.ChannelName = ""
	raw.Timestamp = time.Time{}
	raw.UserID = ""
	raw.Username = ""

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)
	assert.Equal(t, "1122334455", unified.ChannelName)
	assert.False(t, unified.Timestamp.IsZero())
}

// TestFacebookNormalizer_ParsesWireJSON feeds the exact JSON the
// facebook-listener publishes to chat:raw through the real deserialization
// path (ParseRawMessage -> Normalize), pinning the cross-service contract.
func TestFacebookNormalizer_ParsesWireJSON(t *testing.T) {
	wire := []byte(`{
		"message_id":"comment_9001","platform":"facebook","overlay_id":"","channel_id":"1122334455",
		"channel_name":"My Gaming Page","user_id":"user_555","username":"Bea Commenter",
		"text":"first!","tags":{"live_video_id":"vid_777","parent_comment_id":"comment_88"},
		"timestamp":"2026-09-01T10:00:05Z"
	}`)

	raw, err := models.ParseRawMessage(wire)
	require.NoError(t, err)

	unified, err := NewFacebookNormalizer().Normalize(raw, "overlay-1")
	require.NoError(t, err)
	assert.Equal(t, "comment_9001", unified.ID)
	assert.Equal(t, "facebook", unified.Platform)
	assert.Equal(t, "first!", unified.Message.Text)
	assert.Equal(t, "vid_777", unified.Metadata["live_video_id"])
	assert.Equal(t, "comment_88", unified.Metadata["parent_comment_id"])
}
