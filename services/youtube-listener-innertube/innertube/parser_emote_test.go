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
// without thumbnails resolves to the actual Unicode character (not the shortcode).
// YouTube encodes standard emoji as emojiId "emoji_u{hex}", e.g. "emoji_u1f600" → 😀.
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

	assert.Equal(t, "😀", text, "unicode emoji should resolve to actual Unicode character, not shortcode")
	assert.Empty(t, emotes, "unicode emoji should not produce an emote entry")
}

// TestExtractMessageText_BuiltInEmoji_WithThumbnails tests that a non-custom emoji
// WITH thumbnails (YouTube built-in emoji like :face-blue-smiling:) produces an EmoteEntry.
func TestExtractMessageText_BuiltInEmoji_WithThumbnails(t *testing.T) {
	message := MessageContent{
		Runs: []MessageRun{
			{
				Emoji: &EmojiData{
					EmojiID:       "face-blue-smiling",
					IsCustomEmoji: false,
					Shortcuts:     []string{":face-blue-smiling:"},
					Image: Thumbnails{
						Thumbnails: []Thumbnail{
							{URL: "https://www.youtube.com/s/gaming/emoji/small.png", Width: 24, Height: 24},
							{URL: "https://www.youtube.com/s/gaming/emoji/large.png", Width: 48, Height: 48},
						},
					},
				},
			},
		},
	}

	text, emotes := extractMessageText(message)

	assert.Equal(t, ":face-blue-smiling:", text, "built-in emoji text should use shortcut")
	require.Len(t, emotes, 1, "built-in emoji with thumbnails should produce an emote entry")
	assert.Equal(t, ":face-blue-smiling:", emotes[0].Code)
	assert.Equal(t, "https://www.youtube.com/s/gaming/emoji/large.png", emotes[0].URL, "should use index 1 thumbnail")
	assert.Equal(t, "face-blue-smiling", emotes[0].ID)
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

// TestResolveUnicodeEmojiID tests the emoji_u{hex} → Unicode character conversion.
func TestResolveUnicodeEmojiID(t *testing.T) {
	tests := []struct {
		name    string
		emojiID string
		want    string
	}{
		{"face with tears of joy", "emoji_u1f602", "😂"},
		{"grinning face", "emoji_u1f600", "😀"},
		{"flag: china (multi-codepoint)", "emoji_u1f1e8_1f1f3", "🇨🇳"},
		{"ZWJ family sequence", "emoji_u1f468_200d_1f469_200d_1f467", "👨‍👩‍👧"},
		{"non-emoji prefix", "face-fuchsia-tongue-out", ""},
		{"empty string", "", ""},
		{"prefix only", "emoji_u", ""},
		{"invalid hex", "emoji_uzzzz", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveUnicodeEmojiID(tt.emojiID)
			if got != tt.want {
				t.Errorf("resolveUnicodeEmojiID(%q) = %q, want %q", tt.emojiID, got, tt.want)
			}
		})
	}
}

// TestExtractMessageText_UnicodeEmoji_ResolvedToChar tests that a standard Unicode emoji
// (emoji_u{hex} ID, no thumbnails) is converted to the actual Unicode character in text.
func TestExtractMessageText_UnicodeEmoji_ResolvedToChar(t *testing.T) {
	message := MessageContent{
		Runs: []MessageRun{
			{Text: "lol "},
			{
				Emoji: &EmojiData{
					EmojiID:      "emoji_u1f602",
					IsCustomEmoji: false,
					Shortcuts:    []string{":face_with_tears_of_joy:"},
					// No thumbnails — standard Unicode emoji
				},
			},
		},
	}

	text, emotes := extractMessageText(message)

	assert.Equal(t, "lol 😂", text, "should resolve emoji_u1f602 to 😂")
	assert.Empty(t, emotes, "standard Unicode emoji should produce no emote entry")
}

// TestExtractMessageText_MultiCodepointEmoji tests that flag and ZWJ sequence emoji are
// correctly resolved to their Unicode character sequences.
func TestExtractMessageText_MultiCodepointEmoji(t *testing.T) {
	message := MessageContent{
		Runs: []MessageRun{
			{
				Emoji: &EmojiData{
					EmojiID:      "emoji_u1f1e8_1f1f3",
					IsCustomEmoji: false,
					Shortcuts:    []string{":flag_cn:"},
				},
			},
		},
	}

	text, emotes := extractMessageText(message)

	assert.Equal(t, "🇨🇳", text, "should resolve multi-codepoint flag emoji")
	assert.Empty(t, emotes, "flag emoji should produce no emote entry")
}
