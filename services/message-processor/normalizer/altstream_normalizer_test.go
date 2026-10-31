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
	"os"
	"strings"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixtures in testdata/altstream were captured live on 2026-10-31 (ADR-0082):
// sse_frames.txt from GET /api/chat/stream (the richer shape: authorUsername,
// authorImage, authorPronouns, badges) and rest_events_by_type.json from
// GET /api/chat (the REST shape, which lacks those fields).

// altstreamSSEEvents returns every event of every `data:` frame in the capture.
func altstreamSSEEvents(t *testing.T) []map[string]any {
	t.Helper()
	data, err := os.ReadFile("testdata/altstream/sse_frames.txt")
	require.NoError(t, err)
	var events []map[string]any
	for _, line := range strings.Split(string(data), "\n") {
		payload, ok := strings.CutPrefix(line, "data: ")
		if !ok {
			continue
		}
		var batch []map[string]any
		require.NoError(t, json.Unmarshal([]byte(payload), &batch))
		events = append(events, batch...)
	}
	return events
}

func altstreamSSEEvent(t *testing.T, id string) map[string]any {
	t.Helper()
	for _, ev := range altstreamSSEEvents(t) {
		if ev["id"] == id {
			return ev
		}
	}
	t.Fatalf("no SSE event with id %s in the capture", id)
	return nil
}

func altstreamRESTEvent(t *testing.T, eventType string) map[string]any {
	t.Helper()
	data, err := os.ReadFile("testdata/altstream/rest_events_by_type.json")
	require.NoError(t, err)
	var byType map[string]map[string]any
	require.NoError(t, json.Unmarshal(data, &byType))
	ev, ok := byType[eventType]
	require.True(t, ok, "no REST event of type %s in the capture", eventType)
	return ev
}

// altstreamRaw builds the RawChatMessage altstream-listener publishes for one
// event, per the ADR-0082 listener contract: identity lowercased from
// authorUsername (falling back to author), the highlight text from data.text,
// and raw_message the event verbatim.
func altstreamRaw(t *testing.T, ev map[string]any) *models.RawChatMessage {
	t.Helper()
	str := func(k string) string { s, _ := ev[k].(string); return s }
	username := firstNonEmpty(str("authorUsername"), str("author"))
	text := str("text")
	if data, ok := ev["data"].(map[string]any); ok && str("type") == "highlight" {
		text, _ = data["text"].(string)
	}
	sentAt, _ := ev["sentAt"].(float64)
	rawJSON, err := json.Marshal(ev)
	require.NoError(t, err)
	channel := strings.ToLower(str("channel"))
	return &models.RawChatMessage{
		MessageID:   str("id"),
		Platform:    "altstream",
		OverlayID:   "overlay-1",
		ChannelID:   channel,
		ChannelName: channel,
		UserID:      strings.ToLower(username),
		Username:    username,
		Text:        text,
		Timestamp:   time.UnixMilli(int64(sentAt)).UTC(),
		Tags:        map[string]string{"message_type": str("type")},
		RawMessage:  rawJSON,
	}
}

func TestAltstreamNormalizer_SSEModeratorWithPronouns(t *testing.T) {
	unified, err := NewAltstreamNormalizer().Normalize(altstreamRaw(t, altstreamSSEEvent(t, "246920441")), "overlay-1")
	require.NoError(t, err)

	assert.Equal(t, "246920441", unified.ID)
	assert.Equal(t, "altstream", unified.Platform)
	assert.Equal(t, "overlay-1", unified.OverlayID)
	assert.Equal(t, "achie7240", unified.ChannelID)
	assert.Equal(t, "achie7240", unified.ChannelName)
	assert.Equal(t, models.UserInfo{
		ID:          "dyamode96",
		Username:    "dyamode96",
		DisplayName: "dyamode96",
		AvatarURL:   "https://static-cdn.jtvnw.net/jtv_user_pictures/a818fcdb-dad7-43d4-b74a-47e77a2eadfc-profile_image-150x150.png",
		Color:       "#f97316",
		Pronouns:    "She/Her",
		Badges:      []models.Badge{{Name: "moderator", Version: "1"}},
	}, unified.User)
	assert.Equal(t, "funny xd", unified.Message.Text)
	assert.NotNil(t, unified.Message.Emotes)
	assert.Empty(t, unified.Message.Emotes)
	assert.Equal(t, time.UnixMilli(1793461858000).UTC(), unified.Timestamp)
	assert.Equal(t, map[string]interface{}{"message_type": "message"}, unified.Metadata)
	assert.Nil(t, unified.Event)
}

// author is the DISPLAY name; authorUsername is the stable identity.
func TestAltstreamNormalizer_SSEDisplayNameDiffersFromUsername(t *testing.T) {
	unified, err := NewAltstreamNormalizer().Normalize(altstreamRaw(t, altstreamSSEEvent(t, "952046758")), "overlay-1")
	require.NoError(t, err)

	assert.Equal(t, "lemonpoppyseedgames", unified.User.ID)
	assert.Equal(t, "lemonpoppyseedgames", unified.User.Username)
	assert.Equal(t, "LEMONPOPPYSEEDGAMES", unified.User.DisplayName)
	assert.Equal(t, "He/Them", unified.User.Pronouns)
	assert.Equal(t, "#f59e0b", unified.User.Color)
	assert.Equal(t, []models.Badge{}, unified.User.Badges)
}

// The REST shape has no authorUsername/authorImage/authorPronouns/badges.
func TestAltstreamNormalizer_RESTShape(t *testing.T) {
	unified, err := NewAltstreamNormalizer().Normalize(altstreamRaw(t, altstreamRESTEvent(t, "message")), "overlay-1")
	require.NoError(t, err)

	assert.Equal(t, "615594708", unified.ID)
	assert.Equal(t, "vildravn", unified.User.ID)
	assert.Equal(t, "Vildravn", unified.User.Username)
	assert.Equal(t, "Vildravn", unified.User.DisplayName)
	assert.Empty(t, unified.User.AvatarURL)
	assert.Empty(t, unified.User.Pronouns)
	assert.Equal(t, "#8b5cf6", unified.User.Color)
	assert.Equal(t, []models.Badge{}, unified.User.Badges)
	assert.Equal(t, "choo choo", unified.Message.Text)
}

// A highlight renders as a chat message with its data.text, marked highlighted
// and carrying the reward name. Its event text is "[HIGHLIGHT]:{json}", which
// must never reach the overlay, even when the listener leaves text empty.
func TestAltstreamNormalizer_Highlight(t *testing.T) {
	raw := altstreamRaw(t, altstreamRESTEvent(t, "highlight"))
	for _, text := range []string{raw.Text, ""} {
		raw.Text = text
		unified, err := NewAltstreamNormalizer().Normalize(raw, "overlay-1")
		require.NoError(t, err)

		assert.Equal(t, "2024786203", unified.ID)
		assert.Equal(t, "Hi!", unified.Message.Text)
		assert.Equal(t, "#84cc16", unified.User.Color)
		assert.Equal(t, map[string]interface{}{
			"message_type":   "highlight",
			"is_highlighted": true,
			"reward_name":    "Test Highlight",
		}, unified.Metadata)
		assert.Nil(t, unified.Event, "a highlight is chat, not an alert")
	}
}

// The listener resolves :name: tokens against the channel emote catalogs it
// watches and lists the hits in raw_message.emotes. Each becomes an emote whose
// positions span the whole :name: token (byte offsets, inclusive end).
// Unresolved tokens and bare words stay plain text; a listed emote that is not
// in the text yields nothing.
func TestAltstreamNormalizer_Emotes(t *testing.T) {
	ev := altstreamSSEEvent(t, "952046758")
	ev["text"] = "hi :copyPeek: copyPeek :nope: :fletchlaugh::copyPeek:"
	ev["emotes"] = []map[string]string{
		{"name": "copyPeek", "url": "https://altstream.online/api/media/stickers/copygirl_1792322555448_b51313fd3353.png"},
		{"name": "fletchlaugh", "url": "https://altstream.online/api/media/stickers/fletchmakes_1.png"},
		{"name": "copyBlob", "url": "https://altstream.online/api/media/stickers/copygirl_1792770265929_3d5b5182e676.png"},
	}

	unified, err := NewAltstreamNormalizer().Normalize(altstreamRaw(t, ev), "overlay-1")
	require.NoError(t, err)

	assert.Equal(t, "hi :copyPeek: copyPeek :nope: :fletchlaugh::copyPeek:", unified.Message.Text)
	assert.Equal(t, []models.Emote{
		{
			Code:      "copyPeek",
			Provider:  "altstream",
			URL:       "https://altstream.online/api/media/stickers/copygirl_1792322555448_b51313fd3353.png",
			Positions: [][]int{{3, 12}, {43, 52}},
		},
		{
			Code:      "fletchlaugh",
			Provider:  "altstream",
			URL:       "https://altstream.online/api/media/stickers/fletchmakes_1.png",
			Positions: [][]int{{30, 42}},
		},
	}, unified.Message.Emotes)
}

// A sticker post is a message with empty text and a site-relative stickerUrl
// (the field Altstream's own chat client renders); it becomes an image
// attachment so the overlay shows the sticker instead of a blank line.
func TestAltstreamNormalizer_StickerBecomesAttachment(t *testing.T) {
	ev := altstreamSSEEvent(t, "952046758")
	ev["text"] = ""
	ev["stickerUrl"] = "/api/media/stickers/achie7240_1792753158937_75a7b4fbf642.png"

	unified, err := NewAltstreamNormalizer().Normalize(altstreamRaw(t, ev), "overlay-1")
	require.NoError(t, err)

	assert.Equal(t, "", unified.Message.Text)
	assert.Equal(t, []models.Attachment{{
		Type:     "image",
		URL:      "https://altstream.online/api/media/stickers/achie7240_1792753158937_75a7b4fbf642.png",
		Filename: "Sticker",
	}}, unified.Message.Attachments)

	ev["stickerUrl"] = "javascript:alert(1)"
	unified, err = NewAltstreamNormalizer().Normalize(altstreamRaw(t, ev), "overlay-1")
	require.NoError(t, err)
	assert.Empty(t, unified.Message.Attachments)
}

// Altstream serves its own media site-relative (channel avatars, sticker
// URLs); anything rendered on the overlay must be absolute. A colour that is
// not a hex value is dropped rather than passed into the overlay's CSS.
func TestAltstreamNormalizer_SanitizesMediaAndColor(t *testing.T) {
	ev := altstreamSSEEvent(t, "952046758")
	ev["authorImage"] = "/api/media/avatars/copygirl_1791992321584_6bb51914beb6.png"
	ev["userColor"] = "red;background:url(x)"
	ev["color"] = "red;background:url(x)"
	ev["text"] = ":copyPeek:"
	ev["emotes"] = []map[string]string{{"name": "copyPeek", "url": "/api/media/stickers/copygirl_1792322555448_b51313fd3353.png"}}

	unified, err := NewAltstreamNormalizer().Normalize(altstreamRaw(t, ev), "overlay-1")
	require.NoError(t, err)

	assert.Equal(t, "https://altstream.online/api/media/avatars/copygirl_1791992321584_6bb51914beb6.png", unified.User.AvatarURL)
	assert.Equal(t, "https://altstream.online/api/media/stickers/copygirl_1792322555448_b51313fd3353.png", unified.Message.Emotes[0].URL)
	assert.Empty(t, unified.User.Color)
}

// roles and badges merge into one de-duplicated list, roles first.
func TestAltstreamNormalizer_RolesAndBadges(t *testing.T) {
	ev := altstreamSSEEvent(t, "246920441")
	ev["roles"] = map[string]bool{"broadcaster": true, "moderator": true, "subscriber": true, "supporter": true}
	ev["badges"] = []string{"moderator", "Founder"}

	unified, err := NewAltstreamNormalizer().Normalize(altstreamRaw(t, ev), "overlay-1")
	require.NoError(t, err)

	names := make([]string, 0, len(unified.User.Badges))
	for _, b := range unified.User.Badges {
		names = append(names, b.Name)
	}
	assert.Equal(t, []string{"broadcaster", "moderator", "subscriber", "supporter", "founder"}, names)
}

func TestAltstreamNormalizer_RejectsBadInput(t *testing.T) {
	raw := altstreamRaw(t, altstreamSSEEvent(t, "246920441"))
	raw.Platform = "picarto"
	_, err := NewAltstreamNormalizer().Normalize(raw, "overlay-1")
	assert.Error(t, err)

	raw = altstreamRaw(t, altstreamSSEEvent(t, "246920441"))
	raw.ChannelID = ""
	_, err = NewAltstreamNormalizer().Normalize(raw, "overlay-1")
	assert.Error(t, err)
}
