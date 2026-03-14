package innertube

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtractMessageText_CustomEmoji_ChannelMember tests that a custom channel emoji
// produces a text placeholder and emote entry with URL from index 1 thumbnail.
// NOTE: extractMessageText currently returns (string). Tests assume new signature
// returning (string, []EmoteEntry). Will fail to compile until Plan 02. RED state.
func TestExtractMessageText_CustomEmoji_ChannelMember(t *testing.T) {
	message := MessageContent{
		Runs: []MessageRun{
			{
				Emoji: &EmojiData{
					EmojiID:      "UCxxxxxx/custom_emote",
					IsCustomEmoji: true,
					Shortcuts:    []string{":custom:"},
					Image: Thumbnails{
						Thumbnails: []Thumbnail{
							{URL: "small.png", Width: 32, Height: 32},
							{URL: "large.png", Width: 48, Height: 48},
						},
					},
				},
			},
		},
	}

	text, emotes := extractMessageText(message)

	assert.Equal(t, ":custom:", text, "placeholder should be the first shortcut")
	require.Len(t, emotes, 1, "should have 1 emote entry")
	assert.Equal(t, "large.png", emotes[0].URL, "should use index 1 thumbnail URL (48px)")
	assert.Equal(t, ":custom:", emotes[0].Code)
	assert.Equal(t, "UCxxxxxx/custom_emote", emotes[0].ID)
}

// TestExtractMessageText_CustomEmoji_Global tests that a global custom emoji
// also produces an emote entry.
func TestExtractMessageText_CustomEmoji_Global(t *testing.T) {
	message := MessageContent{
		Runs: []MessageRun{
			{
				Emoji: &EmojiData{
					EmojiID:      "_global_emote",
					IsCustomEmoji: true,
					Shortcuts:    []string{":global:"},
					Image: Thumbnails{
						Thumbnails: []Thumbnail{
							{URL: "global.png", Width: 32, Height: 32},
						},
					},
				},
			},
		},
	}

	text, emotes := extractMessageText(message)

	assert.Equal(t, ":global:", text)
	require.Len(t, emotes, 1, "global custom emoji should produce emote entry")
}

// TestExtractMessageText_UnicodeEmoji_NoCustom tests that a non-custom (unicode) emoji
// does NOT produce an emote entry but still contributes to text.
func TestExtractMessageText_UnicodeEmoji_NoCustom(t *testing.T) {
	message := MessageContent{
		Runs: []MessageRun{
			{
				Emoji: &EmojiData{
					EmojiID:      "emoji_u1f600",
					IsCustomEmoji: false,
					Shortcuts:    []string{":smile:"},
				},
			},
		},
	}

	text, emotes := extractMessageText(message)

	assert.Equal(t, ":smile:", text, "unicode emoji text should still be present")
	assert.Empty(t, emotes, "unicode emoji should not produce an emote entry")
}

// TestExtractMessageText_EmoteURL_UsesIndex1 tests that with 2 thumbnails,
// the emote URL uses index 1 (larger image).
func TestExtractMessageText_EmoteURL_UsesIndex1(t *testing.T) {
	message := MessageContent{
		Runs: []MessageRun{
			{
				Emoji: &EmojiData{
					EmojiID:      "UCtest/emote",
					IsCustomEmoji: true,
					Shortcuts:    []string{":myemote:"},
					Image: Thumbnails{
						Thumbnails: []Thumbnail{
							{URL: "small.png", Width: 32},
							{URL: "large.png", Width: 48},
						},
					},
				},
			},
		},
	}

	_, emotes := extractMessageText(message)

	require.Len(t, emotes, 1)
	assert.Equal(t, "large.png", emotes[0].URL, "should use index 1 (larger) thumbnail")
}

// TestExtractMessageText_EmoteURL_FallbackIndex0 tests that with only 1 thumbnail,
// the emote URL uses index 0 without panicking.
func TestExtractMessageText_EmoteURL_FallbackIndex0(t *testing.T) {
	message := MessageContent{
		Runs: []MessageRun{
			{
				Emoji: &EmojiData{
					EmojiID:      "UCtest/emote",
					IsCustomEmoji: true,
					Shortcuts:    []string{":myemote:"},
					Image: Thumbnails{
						Thumbnails: []Thumbnail{
							{URL: "only.png", Width: 32},
						},
					},
				},
			},
		},
	}

	_, emotes := extractMessageText(message)

	require.Len(t, emotes, 1)
	assert.Equal(t, "only.png", emotes[0].URL, "should fall back to index 0 when only 1 thumbnail")
}

// TestExtractMessageText_EmoteNoShortcut tests that a custom emoji with no shortcuts
// uses the colon-wrapped emojiId as the placeholder.
func TestExtractMessageText_EmoteNoShortcut(t *testing.T) {
	message := MessageContent{
		Runs: []MessageRun{
			{
				Emoji: &EmojiData{
					EmojiID:      "UCxxxxxx/noname",
					IsCustomEmoji: true,
					Shortcuts:    []string{},
					Image: Thumbnails{
						Thumbnails: []Thumbnail{
							{URL: "emote.png", Width: 32},
						},
					},
				},
			},
		},
	}

	text, _ := extractMessageText(message)

	assert.Equal(t, ":UCxxxxxx/noname:", text, "should use colon-wrapped emojiId when no shortcuts")
}

// TestParseMessages_EmoteDataTag tests that ParseMessages with a text message containing
// a custom emoji run results in msg.Tags["emote_data"] being valid JSON.
func TestParseMessages_EmoteDataTag(t *testing.T) {
	channelID := "UC_test_channel"

	actions := []ChatAction{
		{
			AddChatItemAction: &AddChatItemAction{
				Item: ChatItem{
					LiveChatTextMessageRenderer: &LiveChatTextMessageRenderer{
						Message: MessageContent{
							Runs: []MessageRun{
								{Text: "hello "},
								{
									Emoji: &EmojiData{
										EmojiID:      "UCtest/emote1",
										IsCustomEmoji: true,
										Shortcuts:    []string{":emote1:"},
										Image: Thumbnails{
											Thumbnails: []Thumbnail{
												{URL: "emote_small.png", Width: 32},
												{URL: "emote_large.png", Width: 48},
											},
										},
									},
								},
							},
						},
						AuthorName:              SimpleText{SimpleText: "TestUser"},
						AuthorExternalChannelID: "UC123",
						TimestampUsec:           "1640000000000000",
					},
				},
			},
		},
	}

	messages, err := ParseMessages(actions, channelID)
	require.NoError(t, err)
	require.Len(t, messages, 1)

	msg := messages[0]
	emoteDataJSON, ok := msg.Tags["emote_data"]
	assert.True(t, ok, "emote_data tag should be present")
	assert.NotEmpty(t, emoteDataJSON, "emote_data should not be empty")

	// Verify it's valid JSON array
	var emoteData []map[string]string
	err = json.Unmarshal([]byte(emoteDataJSON), &emoteData)
	assert.NoError(t, err, "emote_data should be valid JSON")
	assert.NotEmpty(t, emoteData, "emote_data JSON array should not be empty")
}
