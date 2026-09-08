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

// Live capture from wss://chat.goodgame.ru/chat/websocket (spike, 2026-09),
// channel 5 (Miker). This is the exact "message" frame payload shape the
// goodgame-listener forwards as raw_message.
const goodgameLiveFixture = `{
	"channel_id": 5,
	"user_id": 21011,
	"user_name": "runi.",
	"user_rights": 0,
	"premium": 0,
	"premiums": ["1844"],
	"resubs": {"1844": 7},
	"staff": 0,
	"color": "simple",
	"icon": "none",
	"role": "",
	"mobile": 0,
	"payments": 0,
	"paymentsAll": {"1239": 2},
	"gg_plus_tier": 0,
	"isStatus": 0,
	"message_id": 1788879813866,
	"timestamp": 1788879814,
	"text": "hello from the spike"
}`

func goodgameRaw(t *testing.T, rawJSON string) *models.RawChatMessage {
	t.Helper()
	ts := time.Unix(1788879814, 0).UTC()
	return &models.RawChatMessage{
		MessageID:   "1788879813866",
		Platform:    "goodgame",
		OverlayID:   "",
		ChannelID:   "miker",
		ChannelName: "Miker",
		UserID:      "21011",
		Username:    "runi.",
		Text:        "hello from the spike",
		Timestamp:   ts,
		Tags: map[string]string{
			"channel_id": "5",
		},
		RawMessage: []byte(rawJSON),
	}
}

func TestGoodGameNormalizer_LiveFixture(t *testing.T) {
	n := NewGoodGameNormalizer()
	unified, err := n.Normalize(goodgameRaw(t, goodgameLiveFixture), "overlay-1")
	require.NoError(t, err)
	require.NotNil(t, unified)

	assert.Equal(t, "1788879813866", unified.ID)
	assert.Equal(t, "overlay-1", unified.OverlayID)
	assert.Equal(t, "goodgame", unified.Platform)
	assert.Equal(t, "miker", unified.ChannelID)
	assert.Equal(t, "Miker", unified.ChannelName)
	assert.Equal(t, "21011", unified.User.ID)
	assert.Equal(t, "runi.", unified.User.Username)
	assert.Equal(t, "runi.", unified.User.DisplayName)
	assert.Equal(t, "hello from the spike", unified.Message.Text)
	assert.Empty(t, unified.User.Color, "named colour tier must not become a hex colour")

	// Protocol timestamp (unix seconds) wins over the envelope timestamp.
	want := time.Unix(1788879814, 0).UTC()
	assert.True(t, unified.Timestamp.Equal(want), "timestamp: want %v got %v", want, unified.Timestamp)
}

func TestGoodGameNormalizer_HexColorPassesThrough(t *testing.T) {
	n := NewGoodGameNormalizer()
	raw := goodgameRaw(t, goodgameLiveFixture)
	raw.RawMessage = []byte(`{"channel_id":"5","user_id":"1","user_name":"goldie","color":"#6633FF","message_id":"1","timestamp":"1788879814","text":"hi"}`)

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)
	assert.Equal(t, "#6633FF", unified.User.Color, "documented hex colour (protocol example) must pass through")
}

func TestGoodGameNormalizer_BadgesFromRightsAndPremium(t *testing.T) {
	n := NewGoodGameNormalizer()

	raw := goodgameRaw(t, `{"channel_id":"5","user_id":"2","user_name":"mod","user_rights":30,"premium":0,"message_id":"2","timestamp":"1","text":"x"}`)
	unified, err := n.Normalize(raw, "o")
	require.NoError(t, err)
	names := badgeNames(unified.User.Badges)
	assert.Contains(t, names, "rights:30")
	assert.NotContains(t, names, "premium")

	raw = goodgameRaw(t, `{"channel_id":"5","user_id":"3","user_name":"sub","user_rights":0,"premium":1,"message_id":"3","timestamp":"1","text":"x"}`)
	unified, err = n.Normalize(raw, "o")
	require.NoError(t, err)
	names = badgeNames(unified.User.Badges)
	assert.Contains(t, names, "premium")

	// An ordinary viewer (rights=0, premium=0) gets no badges.
	raw = goodgameRaw(t, goodgameLiveFixture)
	unified, err = n.Normalize(raw, "o")
	require.NoError(t, err)
	assert.Empty(t, unified.User.Badges)
}

func TestGoodGameNormalizer_FallsBackToTagsAndEnvelope(t *testing.T) {
	n := NewGoodGameNormalizer()

	// Raw payload unusable: envelope fields must carry the message.
	raw := goodgameRaw(t, `not-json`)
	unified, err := n.Normalize(raw, "overlay-2")
	require.NoError(t, err)
	assert.Equal(t, "runi.", unified.User.Username)
	assert.Equal(t, "21011", unified.User.ID)
	assert.Equal(t, "hello from the spike", unified.Message.Text)
	assert.Equal(t, "1788879813866", unified.ID)
}

func TestGoodGameNormalizer_WireJSONContract(t *testing.T) {
	// The exact chat:raw payload the listener publishes, fed through the real
	// deserialization path (ParseRawMessage -> Normalize).
	wire := []byte(`{
		"message_id":"1788879813866",
		"platform":"goodgame",
		"overlay_id":"",
		"channel_id":"miker",
		"channel_name":"Miker",
		"user_id":"21011",
		"username":"runi.",
		"text":"<a href=\"#\">rich</a> plain text",
		"tags":{"channel_id":"5","user_rights":"0","premium":"0"},
		"raw_message":` + goodgameLiveFixture + `,
		"timestamp":"2026-09-05T12:23:34Z"
	}`)
	parsed, err := models.ParseRawMessage(wire)
	require.NoError(t, err)

	n := NewGoodGameNormalizer()
	unified, err := n.Normalize(parsed, "overlay-3")
	require.NoError(t, err)
	assert.Equal(t, "goodgame", unified.Platform)
	assert.Equal(t, "overlay-3", unified.OverlayID)
	assert.Equal(t, "1788879813866", unified.ID)
	assert.Equal(t, "runi.", unified.User.Username)
	assert.Equal(t, "<a href=\"#\">rich</a> plain text", unified.Message.Text, "text passes through as the server escapes it; consumers strip markup")
}

func TestGoodGameNormalizer_RejectsOtherPlatforms(t *testing.T) {
	n := NewGoodGameNormalizer()
	raw := goodgameRaw(t, `{}`)
	raw.Platform = "twitch"
	_, err := n.Normalize(raw, "o")
	require.Error(t, err)
}

func TestGoodGameNormalizer_RejectsInvalidChannelID(t *testing.T) {
	n := NewGoodGameNormalizer()
	raw := goodgameRaw(t, `{}`)
	raw.ChannelID = "bad channel!"
	_, err := n.Normalize(raw, "o")
	require.Error(t, err)
}

func badgeNames(badges []models.Badge) []string {
	names := make([]string, 0, len(badges))
	for _, b := range badges {
		names = append(names, b.Name)
	}
	return names
}
