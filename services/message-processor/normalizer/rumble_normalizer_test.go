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

// Fixtures mirror live SSE captures from web7.rumble.com (PSB chat,
// 2026-09-08) re-wrapped in the listener's envelope.

func rumbleFixture(t *testing.T, msgJSON, usersJSON string) *models.RawChatMessage {
	t.Helper()
	envelope := `{"message":` + msgJSON + `,"users":` + usersJSON + `}`
	return &models.RawChatMessage{
		MessageID:   "",
		Platform:    "rumble",
		OverlayID:   "overlay-1",
		ChannelID:   "445201096",
		ChannelName: "445201096",
		RawMessage:  json.RawMessage(envelope),
		Tags: map[string]string{
			"chatroom_id":  "445201096",
			"channel_slug": "psbnews",
		},
	}
}

func TestRumbleNormalizer_FullMessage(t *testing.T) {
	raw := rumbleFixture(t,
		`{"id":"2737831086443583025","time":"2026-09-08T14:33:53+00:00","user_id":"7211755","text":"hello world","type":"regular"}`,
		`[{"id":"7211755","username":"jaffowhit","link":"/user/jaffowhit","is_follower":true,"color":"#2c93bc","badges":["verified"]}]`)

	n := NewRumbleNormalizer()
	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)
	require.NotNil(t, unified)

	assert.Equal(t, "2737831086443583025", unified.ID)
	assert.Equal(t, "rumble", unified.Platform)
	assert.Equal(t, "overlay-1", unified.OverlayID)
	assert.Equal(t, "445201096", unified.ChannelID)
	assert.Equal(t, "hello world", unified.Message.Text)

	assert.Equal(t, "7211755", unified.User.ID)
	assert.Equal(t, "jaffowhit", unified.User.Username)
	assert.Equal(t, "jaffowhit", unified.User.DisplayName)
	assert.Equal(t, "#2c93bc", unified.User.Color)
	require.Len(t, unified.User.Badges, 1)
	assert.Equal(t, "verified", unified.User.Badges[0].Name)

	expected, _ := time.Parse(time.RFC3339, "2026-09-08T14:33:53+00:00")
	assert.True(t, unified.Timestamp.Equal(expected), "timestamp %v", unified.Timestamp)

	assert.Equal(t, 445201096, unified.Metadata["chatroom_id"])
	assert.Equal(t, "regular", unified.Metadata["message_type"])
}

func TestRumbleNormalizer_RantBecomesDonationEvent(t *testing.T) {
	raw := rumbleFixture(t,
		`{"id":"2737850000000000000","time":"2026-09-08T15:00:00+00:00","user_id":"100","text":"great stream","type":"regular","rant":{"price_cents":500,"duration":300}}`,
		`[{"id":"100","username":"fan","color":"#ffffff","badges":[]}]`)

	n := NewRumbleNormalizer()
	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)
	require.NotNil(t, unified.Event)
	assert.Equal(t, "donation", unified.Event.Type)
	require.NotNil(t, unified.Event.Value)
	assert.Equal(t, 5.0, unified.Event.Value.Amount)
	assert.Equal(t, "USD", unified.Event.Value.Currency)
	assert.Equal(t, "$5.00", unified.Event.Value.DisplayText)
	assert.Equal(t, 300, unified.Event.Duration)
}

func TestRumbleNormalizer_UsernameFallbackNotUserID(t *testing.T) {
	// Listener sets Username to the raw user id (SSE messages carry ids
	// only); the users array must win over that raw id.
	raw := rumbleFixture(t,
		`{"id":"m1","time":"2026-09-08T15:00:00+00:00","user_id":"222","text":"hi"}`,
		`[{"id":"222","username":"realname","color":"#123456","badges":[]}]`)
	raw.Username = "222"

	unified, err := NewRumbleNormalizer().Normalize(raw, "overlay-1")
	require.NoError(t, err)
	assert.Equal(t, "realname", unified.User.Username)
}

func TestRumbleNormalizer_BlocksFallbackWhenTextEmpty(t *testing.T) {
	// Text empty: the normalizer falls back to whatever the listener put in
	// Text; a listener-side blocks join is exercised in the listener tests.
	// Here Text carries the joined value and the normalizer keeps it.
	raw := rumbleFixture(t,
		`{"id":"m2","time":"2026-09-08T15:00:00+00:00","user_id":"222"}`,
		`[{"id":"222","username":"u","color":"","badges":[]}]`)
	raw.Text = "joined from blocks"

	unified, err := NewRumbleNormalizer().Normalize(raw, "overlay-1")
	require.NoError(t, err)
	assert.Equal(t, "joined from blocks", unified.Message.Text)
}

func TestRumbleNormalizer_RejectsWrongPlatform(t *testing.T) {
	_, err := NewRumbleNormalizer().Normalize(&models.RawChatMessage{Platform: "twitch"}, "overlay-1")
	assert.Error(t, err)
}

func TestRumbleNormalizer_RejectsEmptyChannelID(t *testing.T) {
	_, err := NewRumbleNormalizer().Normalize(&models.RawChatMessage{Platform: "rumble"}, "overlay-1")
	assert.Error(t, err)
}

func TestRumbleNormalizer_UnknownUserYieldsEmptyProfile(t *testing.T) {
	raw := rumbleFixture(t,
		`{"id":"m3","time":"2026-09-08T15:00:00+00:00","user_id":"999","text":"anon"}`,
		`[]`)

	unified, err := NewRumbleNormalizer().Normalize(raw, "overlay-1")
	require.NoError(t, err)
	assert.Equal(t, "999", unified.User.ID)
	assert.Equal(t, "", unified.User.Username)
	assert.Equal(t, "", unified.User.Color)
	assert.Empty(t, unified.User.Badges)
}

func TestRumbleNormalizer_MalformedRawDoesNotPanic(t *testing.T) {
	raw := &models.RawChatMessage{
		Platform:   "rumble",
		ChannelID:  "445201096",
		Text:       "fallback text",
		RawMessage: json.RawMessage(`{broken`),
	}
	unified, err := NewRumbleNormalizer().Normalize(raw, "overlay-1")
	require.NoError(t, err)
	assert.Equal(t, "fallback text", unified.Message.Text)
}

func TestRumbleNormalizer_RoleFlags(t *testing.T) {
	raw := rumbleFixture(t,
		`{"id":"m4","time":"2026-09-08T15:00:00+00:00","user_id":"5","text":"x","type":"regular"}`,
		`[{"id":"5","username":"mod","color":"","badges":["admin","recurring_subscription"]}]`)

	unified, err := NewRumbleNormalizer().Normalize(raw, "overlay-1")
	require.NoError(t, err)
	assert.Equal(t, true, unified.Metadata["is_moderator"])
	assert.Equal(t, true, unified.Metadata["is_subscriber"])
	assert.Equal(t, true, unified.Metadata["is_admin"])
}
