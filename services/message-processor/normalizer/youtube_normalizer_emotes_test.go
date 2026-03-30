package normalizer

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeRawMsgWithText creates a minimal valid RawChatMessage with specific text and tags.
func makeRawMsgWithText(text string, tags map[string]string) *models.RawChatMessage {
	return &models.RawChatMessage{
		MessageID: "test-emote-msg",
		Platform:  "youtube",
		ChannelID: "UCxxxxxx",
		UserID:    "UCyyyyyy",
		Username:  "TestUser",
		Text:      text,
		Timestamp: time.Now(),
		Tags:      tags,
	}
}

// TestYouTubeNormalizer_EmoteData_ChannelEmote tests that a channel emote in emote_data
// results in an Emote entry with Code, URL, and Provider="youtube".
// This test will FAIL until Plan 03 adds emote_data parsing to the normalizer.
func TestYouTubeNormalizer_EmoteData_ChannelEmote(t *testing.T) {
	n := NewYouTubeNormalizer()

	emoteDataJSON, err := json.Marshal([]map[string]string{
		{"code": ":custom:", "url": "https://yt.img/emote.png", "id": "UCxxxxx/custom"},
	})
	require.NoError(t, err)

	raw := makeRawMsgWithText(":custom:", map[string]string{
		"emote_data": string(emoteDataJSON),
	})

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)

	require.Len(t, unified.Message.Emotes, 1, "should have 1 emote")
	assert.Equal(t, ":custom:", unified.Message.Emotes[0].Code)
	assert.Equal(t, "https://yt.img/emote.png", unified.Message.Emotes[0].URL)
	assert.Equal(t, "youtube", unified.Message.Emotes[0].Provider)
}

// TestYouTubeNormalizer_EmoteData_GlobalEmote tests that a global emote (ID starting with "_")
// in emote_data also results in an Emote entry.
// This test will FAIL until Plan 03.
func TestYouTubeNormalizer_EmoteData_GlobalEmote(t *testing.T) {
	n := NewYouTubeNormalizer()

	emoteDataJSON, err := json.Marshal([]map[string]string{
		{"code": ":smile:", "url": "https://yt.img/smile.png", "id": "_global_smile"},
	})
	require.NoError(t, err)

	raw := makeRawMsgWithText(":smile:", map[string]string{
		"emote_data": string(emoteDataJSON),
	})

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)

	require.Len(t, unified.Message.Emotes, 1, "global emote should be included")
	assert.Equal(t, ":smile:", unified.Message.Emotes[0].Code)
	assert.Equal(t, "youtube", unified.Message.Emotes[0].Provider)
}

// TestYouTubeNormalizer_EmoteData_MultipleEmotes tests that multiple emotes in emote_data
// all result in Emote entries.
// This test will FAIL until Plan 03.
func TestYouTubeNormalizer_EmoteData_MultipleEmotes(t *testing.T) {
	n := NewYouTubeNormalizer()

	emoteDataJSON, err := json.Marshal([]map[string]string{
		{"code": ":emote1:", "url": "https://yt.img/e1.png", "id": "UCtest/emote1"},
		{"code": ":emote2:", "url": "https://yt.img/e2.png", "id": "UCtest/emote2"},
	})
	require.NoError(t, err)

	raw := makeRawMsgWithText(":emote1: hello :emote2:", map[string]string{
		"emote_data": string(emoteDataJSON),
	})

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)

	assert.Len(t, unified.Message.Emotes, 2, "should have 2 emotes")
}

// TestYouTubeNormalizer_EmoteData_UnicodeNoEmotes tests that when emote_data is absent/empty,
// the Emotes slice is empty (YTEMOTE-03 regression — unicode emoji should not become emotes).
// This test should PASS immediately (existing normalizer returns empty Emotes by default).
func TestYouTubeNormalizer_EmoteData_UnicodeNoEmotes(t *testing.T) {
	n := NewYouTubeNormalizer()

	// No emote_data tag — simulates unicode emoji message from old format
	raw := makeRawMsgWithText("hello :smile: world", map[string]string{})

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)

	assert.Empty(t, unified.Message.Emotes, "unicode emoji without emote_data should produce no Emote entries")
}

// TestYouTubeNormalizer_EmoteData_InvalidJSON tests that invalid JSON in emote_data
// does not cause Normalize to return an error, and results in empty Emotes (graceful degradation).
// This test should PASS immediately.
func TestYouTubeNormalizer_EmoteData_InvalidJSON(t *testing.T) {
	n := NewYouTubeNormalizer()

	raw := makeRawMsgWithText("test message", map[string]string{
		"emote_data": "not-valid-json",
	})

	unified, err := n.Normalize(raw, "overlay-1")
	assert.NoError(t, err, "invalid JSON in emote_data should not cause an error")
	assert.Empty(t, unified.Message.Emotes, "invalid JSON should result in empty Emotes")
}

// TestYouTubeNormalizer_EmoteData_Positions tests that when emote_data contains an emote
// whose code appears in the message text, the Positions field is populated.
// This test will FAIL until Plan 03 adds position calculation logic.
func TestYouTubeNormalizer_EmoteData_Positions(t *testing.T) {
	n := NewYouTubeNormalizer()

	emoteDataJSON, err := json.Marshal([]map[string]string{
		{"code": ":custom:", "url": "https://yt.img/custom.png", "id": "UCtest/custom"},
	})
	require.NoError(t, err)

	// ":custom:" starts at byte index 6 ("hello " = 6 bytes) and ends at byte index 13 inclusive
	// (":custom:" is 8 bytes: indices 6,7,8,9,10,11,12,13). Positions use inclusive end to match
	// the Twitch IRC convention and the frontend renderMessage renderer expectation.
	raw := makeRawMsgWithText("hello :custom: world", map[string]string{
		"emote_data": string(emoteDataJSON),
	})

	unified, err := n.Normalize(raw, "overlay-1")
	require.NoError(t, err)

	require.Len(t, unified.Message.Emotes, 1)
	assert.NotEmpty(t, unified.Message.Emotes[0].Positions, "Positions should be non-empty for emote present in text")
	assert.Equal(t, []int{6, 13}, unified.Message.Emotes[0].Positions[0], "position should be [6, 13] (inclusive end — ':custom:' occupies bytes 6..13)")
}
