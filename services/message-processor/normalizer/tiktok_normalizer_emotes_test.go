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

// makeTikTokRawMsgWithText creates a minimal valid TikTok RawChatMessage with
// specific text and tags (the chat text and the emote_data tag the
// tiktok-listener serializes from the payload's native emotes).
func makeTikTokRawMsgWithText(text string, tags map[string]string) *models.RawChatMessage {
	return &models.RawChatMessage{
		MessageID: "test-tt-emote-msg",
		Platform:  "tiktok",
		ChannelID: "creator123",
		UserID:    "user456",
		Username:  "TTViewer",
		Text:      text,
		Timestamp: time.Now(),
		Tags:      tags,
	}
}

// TestTikTokNormalizer_EmoteData_SingleEmote tests that a native emote in
// emote_data becomes an Emote entry with Code, URL, and Provider="tiktok".
func TestTikTokNormalizer_EmoteData_SingleEmote(t *testing.T) {
	n := NewTikTokNormalizer()

	emoteDataJSON, err := json.Marshal([]map[string]string{
		{"code": "[laughcry]", "url": "https://tt.img/laughcry.png", "id": "7123456789"},
	})
	require.NoError(t, err)

	// "[laughcry]" is 10 bytes starting at byte 4 ("hey " = 4 bytes),
	// occupying bytes 4..13 inclusive.
	raw := makeTikTokRawMsgWithText("hey [laughcry]!", map[string]string{
		"emote_data": string(emoteDataJSON),
	})

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)

	require.Len(t, unified.Message.Emotes, 1, "should have 1 emote")
	assert.Equal(t, "[laughcry]", unified.Message.Emotes[0].Code)
	assert.Equal(t, "https://tt.img/laughcry.png", unified.Message.Emotes[0].URL)
	assert.Equal(t, "tiktok", unified.Message.Emotes[0].Provider)
	assert.Equal(t, [][]int{{4, 13}}, unified.Message.Emotes[0].Positions,
		"position should be [4, 13] (inclusive end — '[laughcry]' occupies bytes 4..13)")
}

// TestTikTokNormalizer_EmoteData_MultipleEmotes tests that multiple native
// emotes in emote_data all become Emote entries with their own positions.
func TestTikTokNormalizer_EmoteData_MultipleEmotes(t *testing.T) {
	n := NewTikTokNormalizer()

	emoteDataJSON, err := json.Marshal([]map[string]string{
		{"code": "[wave]", "url": "https://tt.img/wave.png", "id": "7111"},
		{"code": "[laughcry]", "url": "https://tt.img/laughcry.png", "id": "7222"},
	})
	require.NoError(t, err)

	// "[wave]" occupies bytes 3..8, "[laughcry]" bytes 14..23.
	raw := makeTikTokRawMsgWithText("go [wave] now [laughcry]", map[string]string{
		"emote_data": string(emoteDataJSON),
	})

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)

	require.Len(t, unified.Message.Emotes, 2, "should have 2 emotes")
	assert.Equal(t, "[wave]", unified.Message.Emotes[0].Code)
	assert.Equal(t, [][]int{{3, 8}}, unified.Message.Emotes[0].Positions)
	assert.Equal(t, "[laughcry]", unified.Message.Emotes[1].Code)
	assert.Equal(t, [][]int{{14, 23}}, unified.Message.Emotes[1].Positions)
}

// TestTikTokNormalizer_EmoteData_EmoteAtEnd tests an emote as the final token
// of the message (no trailing text after the closing bracket).
func TestTikTokNormalizer_EmoteData_EmoteAtEnd(t *testing.T) {
	n := NewTikTokNormalizer()

	emoteDataJSON, err := json.Marshal([]map[string]string{
		{"code": "[wave]", "url": "https://tt.img/wave.png", "id": "7111"},
	})
	require.NoError(t, err)

	// "[wave]" occupies bytes 5..10 to the end of "nice [wave]".
	raw := makeTikTokRawMsgWithText("nice [wave]", map[string]string{
		"emote_data": string(emoteDataJSON),
	})

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)

	require.Len(t, unified.Message.Emotes, 1)
	assert.Equal(t, [][]int{{5, 10}}, unified.Message.Emotes[0].Positions,
		"position should be [5, 10] — the emote runs to the end of the message")
}

// TestTikTokNormalizer_EmoteData_NonAsciiOffsets tests that positions are byte
// offsets into the UTF-8 text, not rune offsets: multi-byte characters before
// the emote must shift the start accordingly.
func TestTikTokNormalizer_EmoteData_NonAsciiOffsets(t *testing.T) {
	n := NewTikTokNormalizer()

	emoteDataJSON, err := json.Marshal([]map[string]string{
		{"code": "[wave]", "url": "https://tt.img/wave.png", "id": "7111"},
	})
	require.NoError(t, err)

	// "héllo 😀 " is 12 bytes: h=1, é=2, llo=3, space=1, 😀=4, space=1.
	// "[wave]" therefore occupies bytes 12..17, even though it starts at
	// rune index 8 — the pipeline convention is byte offsets into Message.Text.
	raw := makeTikTokRawMsgWithText("héllo 😀 [wave]", map[string]string{
		"emote_data": string(emoteDataJSON),
	})

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)

	require.Len(t, unified.Message.Emotes, 1)
	assert.Equal(t, [][]int{{12, 17}}, unified.Message.Emotes[0].Positions,
		"position should be [12, 17] — byte offsets, not rune offsets")
}

// TestTikTokNormalizer_EmoteData_AbsentTag tests that a message without an
// emote_data tag produces an empty Emotes slice (no regression for the common
// no-emote case).
func TestTikTokNormalizer_EmoteData_AbsentTag(t *testing.T) {
	n := NewTikTokNormalizer()

	raw := makeTikTokRawMsgWithText("hello world", map[string]string{})

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)

	assert.Empty(t, unified.Message.Emotes, "absent emote_data should produce no Emote entries")
}

// TestTikTokNormalizer_EmoteData_InvalidJSON tests that invalid JSON in
// emote_data does not fail Normalize and degrades to no emotes.
func TestTikTokNormalizer_EmoteData_InvalidJSON(t *testing.T) {
	n := NewTikTokNormalizer()

	raw := makeTikTokRawMsgWithText("test message", map[string]string{
		"emote_data": "not-valid-json",
	})

	unified, err := n.Normalize(raw, "overlay-1")
	assert.NoError(t, err, "invalid JSON in emote_data should not cause an error")
	assert.Empty(t, unified.Message.Emotes, "invalid JSON should result in empty Emotes")
}
