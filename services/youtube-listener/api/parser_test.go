package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/api/youtube/v3"
)

func TestNewParser(t *testing.T) {
	parser := NewParser()
	assert.NotNil(t, parser)
}

func TestParseChatMessage_TextMessage(t *testing.T) {
	parser := NewParser()

	msg := &youtube.LiveChatMessage{
		Snippet: &youtube.LiveChatMessageSnippet{
			Type:        "textMessageEvent",
			PublishedAt: "2025-11-13T10:00:00Z",
			TextMessageDetails: &youtube.LiveChatTextMessageDetails{
				MessageText: "Hello world!",
			},
		},
		AuthorDetails: &youtube.LiveChatMessageAuthorDetails{
			ChannelId:       "UCyyyyyy",
			ChannelUrl:      "https://youtube.com/channel/UCyyyyyy",
			DisplayName:     "TestUser",
			ProfileImageUrl: "https://example.com/avatar.jpg",
			IsVerified:      false,
			IsChatOwner:     false,
			IsChatSponsor:   true,
			IsChatModerator: false,
		},
	}

	rawMsg, err := parser.ParseChatMessage(msg, "UCxxxxxx", "stream123")

	assert.NoError(t, err)
	assert.NotNil(t, rawMsg)
	assert.Equal(t, "youtube", rawMsg.Platform)
	assert.Equal(t, "UCxxxxxx", rawMsg.ChannelID)
	assert.Equal(t, "stream123", rawMsg.StreamID)
	assert.Equal(t, "UCyyyyyy", rawMsg.UserID)
	assert.Equal(t, "TestUser", rawMsg.Username)
	assert.Equal(t, "Hello world!", rawMsg.Text)
	assert.Equal(t, "false", rawMsg.Tags["is_verified"])
	assert.Equal(t, "false", rawMsg.Tags["is_owner"])
	assert.Equal(t, "true", rawMsg.Tags["is_sponsor"])
	assert.Equal(t, "false", rawMsg.Tags["is_moderator"])
}

func TestParseChatMessage_SuperChat(t *testing.T) {
	parser := NewParser()

	msg := &youtube.LiveChatMessage{
		Snippet: &youtube.LiveChatMessageSnippet{
			Type:        "superChatEvent",
			PublishedAt: "2025-11-13T10:00:00Z",
			SuperChatDetails: &youtube.LiveChatSuperChatDetails{
				UserComment:         "Great stream!",
				AmountMicros:        5000000, // $5.00
				Currency:            "USD",
				AmountDisplayString: "$5.00",
			},
		},
		AuthorDetails: &youtube.LiveChatMessageAuthorDetails{
			ChannelId:   "UCyyyyyy",
			DisplayName: "SuperFan",
		},
	}

	rawMsg, err := parser.ParseChatMessage(msg, "UCxxxxxx", "stream123")

	assert.NoError(t, err)
	assert.NotNil(t, rawMsg)
	assert.Equal(t, "Great stream!", rawMsg.Text)
	assert.Equal(t, "5000000", rawMsg.Tags["super_chat"])
	assert.Equal(t, "USD", rawMsg.Tags["super_chat_currency"])
	assert.Equal(t, "$5.00", rawMsg.Tags["super_chat_display"])
}

func TestParseChatMessage_SuperSticker(t *testing.T) {
	parser := NewParser()

	msg := &youtube.LiveChatMessage{
		Snippet: &youtube.LiveChatMessageSnippet{
			Type:        "superStickerEvent",
			PublishedAt: "2025-11-13T10:00:00Z",
			SuperStickerDetails: &youtube.LiveChatSuperStickerDetails{
				AmountMicros:        2000000, // $2.00
				Currency:            "USD",
				AmountDisplayString: "$2.00",
				Tier:                2,
				SuperStickerMetadata: &youtube.SuperStickerMetadata{
					AltText: "Thumbs up!",
				},
			},
		},
		AuthorDetails: &youtube.LiveChatMessageAuthorDetails{
			ChannelId:   "UCyyyyyy",
			DisplayName: "StickerFan",
		},
	}

	rawMsg, err := parser.ParseChatMessage(msg, "UCxxxxxx", "stream123")

	assert.NoError(t, err)
	assert.NotNil(t, rawMsg)
	assert.Equal(t, "Thumbs up!", rawMsg.Text)
	assert.Equal(t, "2000000", rawMsg.Tags["super_sticker"])
	assert.Equal(t, "USD", rawMsg.Tags["super_sticker_currency"])
	assert.Equal(t, "$2.00", rawMsg.Tags["super_sticker_display"])
	assert.Equal(t, "2", rawMsg.Tags["super_sticker_tier"])
}

func TestParseChatMessage_StripsAtPrefix(t *testing.T) {
	parser := NewParser()

	msg := &youtube.LiveChatMessage{
		Snippet: &youtube.LiveChatMessageSnippet{
			Type:        "textMessageEvent",
			PublishedAt: "2025-11-13T10:00:00Z",
			TextMessageDetails: &youtube.LiveChatTextMessageDetails{
				MessageText: "Hello",
			},
		},
		AuthorDetails: &youtube.LiveChatMessageAuthorDetails{
			ChannelId:   "UCzzzzzz",
			DisplayName: "@HandleName",
		},
	}

	rawMsg, err := parser.ParseChatMessage(msg, "UCxxxxxx", "stream123")

	assert.NoError(t, err)
	assert.Equal(t, "HandleName", rawMsg.Username)
	assert.Equal(t, "HandleName", rawMsg.Tags["display_name"])
}

func TestParseChatMessage_InvalidMessage_NoSnippet(t *testing.T) {
	parser := NewParser()

	msg := &youtube.LiveChatMessage{
		AuthorDetails: &youtube.LiveChatMessageAuthorDetails{
			ChannelId:   "UCyyyyyy",
			DisplayName: "TestUser",
		},
	}

	rawMsg, err := parser.ParseChatMessage(msg, "UCxxxxxx", "stream123")

	assert.Error(t, err)
	assert.Nil(t, rawMsg)
	assert.Contains(t, err.Error(), "missing snippet")
}

func TestParseChatMessage_InvalidMessage_NoAuthorDetails(t *testing.T) {
	parser := NewParser()

	msg := &youtube.LiveChatMessage{
		Snippet: &youtube.LiveChatMessageSnippet{
			Type:        "textMessageEvent",
			PublishedAt: "2025-11-13T10:00:00Z",
		},
	}

	rawMsg, err := parser.ParseChatMessage(msg, "UCxxxxxx", "stream123")

	assert.Error(t, err)
	assert.Nil(t, rawMsg)
	assert.Contains(t, err.Error(), "author details")
}

func TestParseBatch(t *testing.T) {
	parser := NewParser()

	messages := []*youtube.LiveChatMessage{
		{
			Snippet: &youtube.LiveChatMessageSnippet{
				Type:        "textMessageEvent",
				PublishedAt: "2025-11-13T10:00:00Z",
				TextMessageDetails: &youtube.LiveChatTextMessageDetails{
					MessageText: "Message 1",
				},
			},
			AuthorDetails: &youtube.LiveChatMessageAuthorDetails{
				ChannelId:   "UCyyyyyy",
				DisplayName: "User1",
			},
		},
		{
			Snippet: &youtube.LiveChatMessageSnippet{
				Type:        "textMessageEvent",
				PublishedAt: "2025-11-13T10:01:00Z",
				TextMessageDetails: &youtube.LiveChatTextMessageDetails{
					MessageText: "Message 2",
				},
			},
			AuthorDetails: &youtube.LiveChatMessageAuthorDetails{
				ChannelId:   "UCzzzzzz",
				DisplayName: "User2",
			},
		},
	}

	rawMessages, err := parser.ParseBatch(messages, "UCxxxxxx", "stream123")

	assert.NoError(t, err)
	assert.Len(t, rawMessages, 2)
	assert.Equal(t, "Message 1", rawMessages[0].Text)
	assert.Equal(t, "Message 2", rawMessages[1].Text)
}

func TestExtractPollingInterval(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name     string
		response *youtube.LiveChatMessageListResponse
		expected int
	}{
		{
			name: "with polling interval",
			response: &youtube.LiveChatMessageListResponse{
				PollingIntervalMillis: 3000,
			},
			expected: 3000,
		},
		{
			name:     "without polling interval",
			response: &youtube.LiveChatMessageListResponse{},
			expected: 5000, // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			interval := parser.ExtractPollingInterval(tt.response)
			assert.Equal(t, tt.expected, interval)
		})
	}
}

func TestExtractNextPageToken(t *testing.T) {
	parser := NewParser()

	response := &youtube.LiveChatMessageListResponse{
		NextPageToken: "next-page-token-123",
	}

	token := parser.ExtractNextPageToken(response)
	assert.Equal(t, "next-page-token-123", token)
}
