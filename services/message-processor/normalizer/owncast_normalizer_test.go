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

// Fixture shape grounded in owncast/owncast develop:
//   - services/chat/events/userMessageEvent.go GetBroadcastPayload:
//     {id, timestamp, body, user, type, visible}
//   - models/user.go (serialized to chat clients): {id, displayName,
//     displayColor, isBot, authenticated, createdAt, previousNames}
//   - models/eventType.go: MessageSent == "CHAT"
const owncastInstanceURL = "https://watch.example.org"

func owncastFixture(rawBody string, visible bool, displayColor int) json.RawMessage {
	payload := map[string]interface{}{
		"id":        "aBcDeFgHi",
		"timestamp": "2026-09-08T12:34:56.789012345Z",
		"type":      "CHAT",
		"body":      rawBody,
		"visible":   visible,
		"user": map[string]interface{}{
			"id":            "bFzGqYvXc",
			"displayName":   "gabek",
			"displayColor":  displayColor,
			"isBot":         false,
			"authenticated": false,
		},
	}
	b, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return b
}

func owncastRaw(rawMessage json.RawMessage) *models.RawChatMessage {
	return &models.RawChatMessage{
		MessageID:  "owncast-msg-1",
		Platform:   "owncast",
		ChannelID:  owncastInstanceURL,
		UserID:     "",
		Username:   "",
		Timestamp:  time.Now(),
		Tags:       map[string]string{"instance_url": owncastInstanceURL},
		RawMessage: rawMessage,
	}
}

func TestOwncastNormalizer_FullFixture(t *testing.T) {
	n := NewOwncastNormalizer()
	unified, err := n.Normalize(owncastRaw(owncastFixture("hello from self-hosted chat", true, 3)), "overlay-1")
	require.NoError(t, err)

	assert.Equal(t, "owncast", unified.Platform)
	assert.Equal(t, "overlay-1", unified.OverlayID)
	assert.Equal(t, owncastInstanceURL, unified.ChannelID)
	assert.Equal(t, owncastInstanceURL, unified.ChannelName)
	assert.Equal(t, "aBcDeFgHi", unified.ID)
	assert.Equal(t, "bFzGqYvXc", unified.User.ID)
	assert.Equal(t, "gabek", unified.User.Username)
	assert.Equal(t, "gabek", unified.User.DisplayName)
	assert.Equal(t, "owncast:3", unified.User.Color, "displayColor index passed through as owncast:<n> tag")
	assert.Equal(t, "hello from self-hosted chat", unified.Message.Text)
	assert.True(t, unified.Metadata["visible"].(bool))
	assert.Equal(t, owncastInstanceURL, unified.Metadata["instance_url"])

	expectedTs, _ := time.Parse(time.RFC3339Nano, "2026-09-08T12:34:56.789012345Z")
	assert.True(t, unified.Timestamp.Equal(expectedTs), "timestamp must come from the event payload")
}

// Owncast renders chat bodies server-side (markdown + emoji -> sanitized
// HTML, chat/events RenderAndSanitize); the body arrives as HTML and is
// passed through text-only, with no emote/badge extraction (out of scope).
func TestOwncastNormalizer_HtmlBodyPassedThroughNoEmotes(t *testing.T) {
	n := NewOwncastNormalizer()
	unified, err := n.Normalize(owncastRaw(owncastFixture("<p>hello <strong>world</strong> :smile:</p>", true, 1)), "overlay-1")
	require.NoError(t, err)

	assert.Equal(t, "<p>hello <strong>world</strong> :smile:</p>", unified.Message.Text)
	assert.Empty(t, unified.Message.Emotes, "owncast has no client-side emote tokens")
	assert.Empty(t, unified.User.Badges, "owncast carries no badge list in chat events")
}

func TestOwncastNormalizer_RejectsWrongPlatform(t *testing.T) {
	n := NewOwncastNormalizer()
	raw := owncastRaw(owncastFixture("hi", true, 0))
	raw.Platform = "kick"
	_, err := n.Normalize(raw, "overlay-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported platform")
}

func TestOwncastNormalizer_RejectsNonURLChannelID(t *testing.T) {
	n := NewOwncastNormalizer()
	raw := owncastRaw(owncastFixture("hi", true, 0))
	raw.ChannelID = "just-a-channel"
	_, err := n.Normalize(raw, "overlay-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not an http(s) URL")
}

func TestOwncastNormalizer_EmptyChannelIDRejected(t *testing.T) {
	n := NewOwncastNormalizer()
	raw := owncastRaw(owncastFixture("hi", true, 0))
	raw.ChannelID = ""
	_, err := n.Normalize(raw, "overlay-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

// A broken raw_message envelope must not kill the message: fields fall back
// to the outer RawChatMessage (defensive log-and-drop per the shared contract).
func TestOwncastNormalizer_ToleratesBrokenEnvelope(t *testing.T) {
	n := NewOwncastNormalizer()
	raw := owncastRaw(json.RawMessage("not-json"))
	raw.UserID = "manual-user"
	raw.Username = "manual-name"
	raw.Text = "fallback text"

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)
	assert.Equal(t, "fallback text", unified.Message.Text)
	assert.Equal(t, "manual-user", unified.User.ID)
	assert.Equal(t, "manual-name", unified.User.DisplayName)
	assert.NotContains(t, unified.Metadata, "visible", "no envelope means no visibility claim")
}

func TestOwncastNormalizer_FallsBackToNanosecondID(t *testing.T) {
	n := NewOwncastNormalizer()
	fixture := json.RawMessage(`{"id":"","timestamp":"2026-09-08T12:34:56Z","type":"CHAT","body":"hi","visible":true,"user":{"id":"u","displayName":"n","displayColor":0}}`)
	raw := owncastRaw(fixture)
	raw.RawMessage = fixture
	raw.MessageID = ""
	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)
	assert.Contains(t, unified.ID, "owncast-")
}
