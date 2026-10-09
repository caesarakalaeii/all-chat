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
	"encoding/json"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixture from the ADR-0059 spike: a real chat-batch item captured live on
// the wss://chat.picarto.tv/chat/token=<jwt> feed (channel "Saca").
const picartoSpikeFixture = `{"t":"c","c":"1489462","u":"1489462","n":"Saca","rn":"Saca","i":"ptvimages/1/14/1489462/avatars/gjU7pa9Cojx9iEm9Ap4z6AGGNLh8qQ4q8YPKoB4m.png","y":"F","m":"Im going to bed.:ptv-sleepy:Thank you everyone, see you tomorrow~:ptv-hearts:","id":"10a4cad0-ab98-11f1-90f7-ab89c31c47cb","d":1788880502781,"s":true,"o":true,"cb":true,"k":"ffc2dd","rc":"ffc2dd"}`

// picartoRaw builds a RawChatMessage the way picarto-listener's
// handleChatMessage publishes to chat:raw.
func picartoRaw(t *testing.T, text string, tags map[string]string) *models.RawChatMessage {
	t.Helper()
	return &models.RawChatMessage{
		MessageID:   "10a4cad0-ab98-11f1-90f7-ab89c31c47cb",
		Platform:    "picarto",
		OverlayID:   "overlay-1",
		ChannelID:   "Saca",
		ChannelName: "Saca",
		UserID:      "1489462",
		Username:    "Saca",
		Text:        text,
		Timestamp:   time.UnixMilli(1788880502781),
		Tags:        tags,
		RawMessage:  json.RawMessage(picartoSpikeFixture),
	}
}

func TestPicartoNormalizer_SpikeFixture(t *testing.T) {
	n := NewPicartoNormalizer()

	raw := picartoRaw(t,
		"Im going to bed.:ptv-sleepy:Thank you everyone, see you tomorrow~:ptv-hearts:",
		map[string]string{
			"display_name": "Saca",
			"avatar_url":   "https://images.picarto.tv/ptvimages/1/14/1489462/avatars/gjU7pa9Cojx9iEm9Ap4z6AGGNLh8qQ4q8YPKoB4m.png",
			"color":        "ffc2dd",
		},
	)

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)
	require.NotNil(t, unified)

	assert.Equal(t, "10a4cad0-ab98-11f1-90f7-ab89c31c47cb", unified.ID)
	assert.Equal(t, "picarto", unified.Platform)
	assert.Equal(t, "overlay-1", unified.OverlayID)
	assert.Equal(t, "Saca", unified.ChannelID)
	assert.Equal(t, "Saca", unified.User.Username)
	assert.Equal(t, "Saca", unified.User.DisplayName)
	assert.Equal(t, "1489462", unified.User.ID)
	// Emotes are out of scope: shortcodes are stripped to plain text.
	assert.Equal(t, "Im going to bed.Thank you everyone, see you tomorrow~", unified.Message.Text)
	assert.Empty(t, unified.Message.Emotes)
	// Server sends 6 hex digits without '#'; unified format wants '#rrggbb'.
	assert.Equal(t, "#ffc2dd", unified.User.Color)
	assert.Equal(t, time.UnixMilli(1788880502781), unified.Timestamp)
}

func TestPicartoNormalizer_RejectsOtherPlatform(t *testing.T) {
	n := NewPicartoNormalizer()
	raw := picartoRaw(t, "hello", nil)
	raw.Platform = "kick"
	_, err := n.Normalize(raw, "overlay-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported platform")
}

func TestPicartoNormalizer_RejectsInvalidChannelID(t *testing.T) {
	n := NewPicartoNormalizer()
	raw := picartoRaw(t, "hello", nil)
	raw.ChannelID = "bad channel/id"
	_, err := n.Normalize(raw, "overlay-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid channel ID")
}

func TestPicartoNormalizer_DisplayNameFallsBackToUsername(t *testing.T) {
	n := NewPicartoNormalizer()
	raw := picartoRaw(t, "hello", nil)
	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)
	assert.Equal(t, "Saca", unified.User.DisplayName)
}

func TestPicartoNormalizer_ColorHashing(t *testing.T) {
	n := NewPicartoNormalizer()

	withHex, err := n.Normalize(picartoRaw(t, "x", map[string]string{"color": "6B5FD1"}), "o")
	require.NoError(t, err)
	assert.Equal(t, "#6B5FD1", withHex.User.Color)

	withHash, err := n.Normalize(picartoRaw(t, "x", map[string]string{"color": "#c4073a"}), "o")
	require.NoError(t, err)
	assert.Equal(t, "#c4073a", withHash.User.Color)

	without, err := n.Normalize(picartoRaw(t, "x", nil), "o")
	require.NoError(t, err)
	assert.Empty(t, without.User.Color)
}

func TestPicartoNormalizer_EmoteStripping(t *testing.T) {
	n := NewPicartoNormalizer()

	cases := []struct{ in, want string }{
		{"hello :ptv-wave: world", "hello  world"},
		{":ptv-hearts:", ""},
		{"no emotes here", "no emotes here"},
		{"username:like:token", "usernametoken"}, // worst case: greedy strip is fine for emote-scope exclusion
	}
	for _, tc := range cases {
		unified, err := n.Normalize(picartoRaw(t, tc.in, nil), "o")
		require.NoError(t, err)
		assert.Equal(t, tc.want, unified.Message.Text)
	}
}
