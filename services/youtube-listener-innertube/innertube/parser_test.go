package innertube

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestParseMessages(t *testing.T) {
	channelID := "UC_test_channel"

	tests := []struct {
		name         string
		actions      []ChatAction
		channelID    string
		wantCount    int
		wantErr      bool
		validateMsg  func(*testing.T, *RawChatMessage)
	}{
		{
			name:      "empty actions",
			actions:   []ChatAction{},
			channelID: channelID,
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "single text message",
			actions: []ChatAction{
				{
					AddChatItemAction: &AddChatItemAction{
						Item: ChatItem{
							LiveChatTextMessageRenderer: &LiveChatTextMessageRenderer{
								Message: MessageContent{
									Runs: []MessageRun{
										{Text: "Hello world"},
									},
								},
								AuthorName:              SimpleText{SimpleText: "TestUser"},
								AuthorExternalChannelID: "UC123",
								TimestampUsec:           "1640000000000000", // 2021-12-20
							},
						},
					},
				},
			},
			channelID: channelID,
			wantCount: 1,
			wantErr:   false,
			validateMsg: func(t *testing.T, msg *RawChatMessage) {
				if msg.Platform != "youtube" {
					t.Errorf("Platform = %v, want youtube", msg.Platform)
				}
				if msg.UserID != "UC123" {
					t.Errorf("UserID = %v, want UC123", msg.UserID)
				}
				if msg.Username != "TestUser" {
					t.Errorf("Username = %v, want TestUser", msg.Username)
				}
				if msg.Text != "Hello world" {
					t.Errorf("Text = %v, want 'Hello world'", msg.Text)
				}
				if msg.ChannelID != channelID {
					t.Errorf("ChannelID = %v, want %v", msg.ChannelID, channelID)
				}
				if msg.MessageID == "" {
					t.Error("MessageID should be generated")
				}
				if msg.Tags == nil {
					t.Error("Tags should be initialized")
				}
			},
		},
		{
			name: "message with multiple text runs",
			actions: []ChatAction{
				{
					AddChatItemAction: &AddChatItemAction{
						Item: ChatItem{
							LiveChatTextMessageRenderer: &LiveChatTextMessageRenderer{
								Message: MessageContent{
									Runs: []MessageRun{
										{Text: "Hello "},
										{Text: "world "},
										{Emoji: &EmojiData{Shortcuts: []string{":wave:"}}},
									},
								},
								AuthorName:              SimpleText{SimpleText: "TestUser"},
								AuthorExternalChannelID: "UC123",
								TimestampUsec:           "1640000000000000",
							},
						},
					},
				},
			},
			channelID: channelID,
			wantCount: 1,
			wantErr:   false,
			validateMsg: func(t *testing.T, msg *RawChatMessage) {
				expected := "Hello world :wave:"
				if msg.Text != expected {
					t.Errorf("Text = %v, want %v", msg.Text, expected)
				}
			},
		},
		{
			name: "message with badges",
			actions: []ChatAction{
				{
					AddChatItemAction: &AddChatItemAction{
						Item: ChatItem{
							LiveChatTextMessageRenderer: &LiveChatTextMessageRenderer{
								Message: MessageContent{
									Runs: []MessageRun{{Text: "Test"}},
								},
								AuthorName:              SimpleText{SimpleText: "Moderator"},
								AuthorExternalChannelID: "UC456",
								TimestampUsec:           "1640000000000000",
								AuthorBadges: []AuthorBadge{
									{
										LiveChatAuthorBadgeRenderer: LiveChatAuthorBadgeRenderer{
											Tooltip: "Moderator",
										},
									},
								},
							},
						},
					},
				},
			},
			channelID: channelID,
			wantCount: 1,
			wantErr:   false,
			validateMsg: func(t *testing.T, msg *RawChatMessage) {
				if badges, ok := msg.Tags["badges"]; !ok || badges != "Moderator" {
					t.Errorf("Tags[badges] = %v, want 'Moderator'", badges)
				}
			},
		},
		{
			name: "super chat message",
			actions: []ChatAction{
				{
					AddChatItemAction: &AddChatItemAction{
						Item: ChatItem{
							LiveChatPaidMessageRenderer: &LiveChatPaidMessageRenderer{
								Message: MessageContent{
									Runs: []MessageRun{{Text: "Thanks!"}},
								},
								AuthorName:              SimpleText{SimpleText: "Donor"},
								AuthorExternalChannelID: "UC789",
								TimestampUsec:           "1640000000000000",
								PurchaseAmountText:      SimpleText{SimpleText: "$5.00"},
							},
						},
					},
				},
			},
			channelID: channelID,
			wantCount: 1,
			wantErr:   false,
			validateMsg: func(t *testing.T, msg *RawChatMessage) {
				if msg.EventType != "super_chat" {
					t.Errorf("EventType = %v, want super_chat", msg.EventType)
				}
				if msg.Text != "Thanks!" {
					t.Errorf("Text = %v, want 'Thanks!'", msg.Text)
				}
				if amount, ok := msg.EventData["amount"].(string); !ok || amount != "$5.00" {
					t.Errorf("EventData[amount] = %v, want '$5.00'", msg.EventData["amount"])
				}
			},
		},
		{
			name: "membership message",
			actions: []ChatAction{
				{
					AddChatItemAction: &AddChatItemAction{
						Item: ChatItem{
							LiveChatMembershipItemRenderer: &LiveChatMembershipItemRenderer{
								HeaderSubtext: MessageContent{
									Runs: []MessageRun{{Text: "Welcome to membership!"}},
								},
								AuthorName:              SimpleText{SimpleText: "NewMember"},
								AuthorExternalChannelID: "UC999",
								TimestampUsec:           "1640000000000000",
							},
						},
					},
				},
			},
			channelID: channelID,
			wantCount: 1,
			wantErr:   false,
			validateMsg: func(t *testing.T, msg *RawChatMessage) {
				if msg.EventType != "membership" {
					t.Errorf("EventType = %v, want membership", msg.EventType)
				}
				if msg.Text != "Welcome to membership!" {
					t.Errorf("Text = %v, want 'Welcome to membership!'", msg.Text)
				}
			},
		},
		{
			name: "paid sticker",
			actions: []ChatAction{
				{
					AddChatItemAction: &AddChatItemAction{
						Item: ChatItem{
							LiveChatPaidStickerRenderer: &LiveChatPaidStickerRenderer{
								AuthorName:              SimpleText{SimpleText: "StickerFan"},
								AuthorExternalChannelID: "UC111",
								TimestampUsec:           "1640000000000000",
								PurchaseAmountText:      SimpleText{SimpleText: "$2.00"},
							},
						},
					},
				},
			},
			channelID: channelID,
			wantCount: 1,
			wantErr:   false,
			validateMsg: func(t *testing.T, msg *RawChatMessage) {
				if msg.EventType != "paid_sticker" {
					t.Errorf("EventType = %v, want paid_sticker", msg.EventType)
				}
				if msg.Text != "[sticker]" {
					t.Errorf("Text = %v, want '[sticker]'", msg.Text)
				}
			},
		},
		{
			name: "replay action with nested messages",
			actions: []ChatAction{
				{
					ReplayChatItemAction: &ReplayChatItemAction{
						Actions: []ChatAction{
							{
								AddChatItemAction: &AddChatItemAction{
									Item: ChatItem{
										LiveChatTextMessageRenderer: &LiveChatTextMessageRenderer{
											Message: MessageContent{
												Runs: []MessageRun{{Text: "Replayed message"}},
											},
											AuthorName:              SimpleText{SimpleText: "ReplayUser"},
											AuthorExternalChannelID: "UC222",
											TimestampUsec:           "1640000000000000",
										},
									},
								},
							},
						},
					},
				},
			},
			channelID: channelID,
			wantCount: 1,
			wantErr:   false,
			validateMsg: func(t *testing.T, msg *RawChatMessage) {
				if msg.Text != "Replayed message" {
					t.Errorf("Text = %v, want 'Replayed message'", msg.Text)
				}
			},
		},
		{
			name: "skip action without chat item",
			actions: []ChatAction{
				{
					// Empty action - no AddChatItemAction or AddLiveChatTickerItem
				},
			},
			channelID: channelID,
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			messages, err := ParseMessages(tt.actions, tt.channelID)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMessages() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(messages) != tt.wantCount {
				t.Errorf("ParseMessages() returned %d messages, want %d", len(messages), tt.wantCount)
				return
			}

			if tt.validateMsg != nil && len(messages) > 0 {
				tt.validateMsg(t, messages[0])
			}
		})
	}
}

func TestValidateRawMessage(t *testing.T) {
	validMessage := &RawChatMessage{
		MessageID: "test-id",
		Platform:  "youtube",
		ChannelID: "UC_channel",
		UserID:    "UC_user",
		Username:  "TestUser",
		Text:      "Test message",
		Timestamp: time.Now(),
		Tags:      make(map[string]string),
	}

	tests := []struct {
		name    string
		msg     *RawChatMessage
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid message",
			msg:     validMessage,
			wantErr: false,
		},
		{
			name:    "nil message",
			msg:     nil,
			wantErr: true,
			errMsg:  "message is nil",
		},
		{
			name: "missing MessageID",
			msg: &RawChatMessage{
				Platform:  "youtube",
				UserID:    "UC_user",
				Username:  "TestUser",
				Text:      "Test",
				Timestamp: time.Now(),
			},
			wantErr: true,
			errMsg:  "MessageID is required",
		},
		{
			name: "missing Platform",
			msg: &RawChatMessage{
				MessageID: "test-id",
				UserID:    "UC_user",
				Username:  "TestUser",
				Text:      "Test",
				Timestamp: time.Now(),
			},
			wantErr: true,
			errMsg:  "Platform is required",
		},
		{
			name: "wrong Platform",
			msg: &RawChatMessage{
				MessageID: "test-id",
				Platform:  "twitch",
				UserID:    "UC_user",
				Username:  "TestUser",
				Text:      "Test",
				Timestamp: time.Now(),
			},
			wantErr: true,
			errMsg:  "Platform must be 'youtube'",
		},
		{
			name: "missing UserID",
			msg: &RawChatMessage{
				MessageID: "test-id",
				Platform:  "youtube",
				Username:  "TestUser",
				Text:      "Test",
				Timestamp: time.Now(),
			},
			wantErr: true,
			errMsg:  "UserID is required",
		},
		{
			name: "missing Username",
			msg: &RawChatMessage{
				MessageID: "test-id",
				Platform:  "youtube",
				UserID:    "UC_user",
				Text:      "Test",
				Timestamp: time.Now(),
			},
			wantErr: true,
			errMsg:  "Username is required",
		},
		{
			name: "missing Text (non-event)",
			msg: &RawChatMessage{
				MessageID: "test-id",
				Platform:  "youtube",
				UserID:    "UC_user",
				Username:  "TestUser",
				Timestamp: time.Now(),
			},
			wantErr: true,
			errMsg:  "Text is required for non-event messages",
		},
		{
			name: "missing Text but has EventType (valid)",
			msg: &RawChatMessage{
				MessageID: "test-id",
				Platform:  "youtube",
				UserID:    "UC_user",
				Username:  "TestUser",
				EventType: "super_chat",
				Timestamp: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "zero Timestamp",
			msg: &RawChatMessage{
				MessageID: "test-id",
				Platform:  "youtube",
				UserID:    "UC_user",
				Username:  "TestUser",
				Text:      "Test",
				Timestamp: time.Time{},
			},
			wantErr: true,
			errMsg:  "Timestamp is required",
		},
		{
			name: "nil Tags (auto-initialized)",
			msg: &RawChatMessage{
				MessageID: "test-id",
				Platform:  "youtube",
				UserID:    "UC_user",
				Username:  "TestUser",
				Text:      "Test",
				Timestamp: time.Now(),
				Tags:      nil, // Should be initialized by ValidateRawMessage
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRawMessage(tt.msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRawMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateRawMessage() error message = %v, want to contain %v", err.Error(), tt.errMsg)
				}
			}

			// Verify Tags initialization
			if !tt.wantErr && tt.msg != nil && tt.name == "nil Tags (auto-initialized)" {
				if tt.msg.Tags == nil {
					t.Error("Tags should be initialized to empty map")
				}
			}
		})
	}
}

func TestParseTimestampUsec(t *testing.T) {
	tests := []struct {
		name          string
		timestampUsec string
		wantErr       bool
		validate      func(*testing.T, time.Time)
	}{
		{
			name:          "empty timestamp",
			timestampUsec: "",
			wantErr:       true,
		},
		{
			name:          "invalid format",
			timestampUsec: "not-a-number",
			wantErr:       true,
		},
		{
			name:          "valid timestamp",
			timestampUsec: "1640000000000000", // 2021-12-20 13:33:20 UTC
			wantErr:       false,
			validate: func(t *testing.T, ts time.Time) {
				expected := time.Unix(1640000000, 0).UTC()
				if !ts.Equal(expected) {
					t.Errorf("Timestamp = %v, want %v", ts, expected)
				}
			},
		},
		{
			name:          "timestamp with microseconds",
			timestampUsec: "1640000000123456", // 2021-12-20 13:33:20.123456 UTC
			wantErr:       false,
			validate: func(t *testing.T, ts time.Time) {
				expected := time.Unix(1640000000, 123456000).UTC()
				if !ts.Equal(expected) {
					t.Errorf("Timestamp = %v, want %v", ts, expected)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, err := parseTimestampUsec(tt.timestampUsec)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTimestampUsec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, ts)
			}
		})
	}
}

func TestExtractMessageText(t *testing.T) {
	tests := []struct {
		name    string
		message MessageContent
		want    string
	}{
		{
			name:    "empty message",
			message: MessageContent{},
			want:    "",
		},
		{
			name: "single text run",
			message: MessageContent{
				Runs: []MessageRun{
					{Text: "Hello"},
				},
			},
			want: "Hello",
		},
		{
			name: "multiple text runs",
			message: MessageContent{
				Runs: []MessageRun{
					{Text: "Hello "},
					{Text: "world"},
				},
			},
			want: "Hello world",
		},
		{
			name: "text with emoji",
			message: MessageContent{
				Runs: []MessageRun{
					{Text: "Hello "},
					{Emoji: &EmojiData{Shortcuts: []string{":wave:", ":)"}}},
					{Text: " world"},
				},
			},
			want: "Hello :wave: world",
		},
		{
			name: "emoji without shortcuts",
			message: MessageContent{
				Runs: []MessageRun{
					{Text: "Hello"},
					{Emoji: &EmojiData{Shortcuts: []string{}}},
				},
			},
			want: "Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMessageText(tt.message)
			if got != tt.want {
				t.Errorf("extractMessageText() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSchemaCompatibility(t *testing.T) {
	// Test that RawChatMessage can be JSON-marshaled and matches expected schema
	msg := &RawChatMessage{
		MessageID: "550e8400-e29b-41d4-a716-446655440000",
		Platform:  "youtube",
		ChannelID: "UC_test_channel",
		StreamID:  "stream_123",
		UserID:    "UC_user_456",
		Username:  "TestUser",
		Text:      "Hello world",
		Timestamp: time.Unix(1640000000, 0).UTC(),
		Tags: map[string]string{
			"badges": "Moderator",
		},
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal RawChatMessage: %v", err)
	}

	// Unmarshal back to verify structure
	var decoded map[string]interface{}
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify required fields exist
	requiredFields := []string{
		"message_id", "platform", "channel_id", "stream_id",
		"user_id", "username", "text", "timestamp", "tags",
	}

	for _, field := range requiredFields {
		if _, ok := decoded[field]; !ok {
			t.Errorf("JSON missing required field: %s", field)
		}
	}

	// Verify no extra fields (strict schema compliance)
	allowedFields := map[string]bool{
		"message_id": true,
		"platform":   true,
		"channel_id": true,
		"stream_id":  true,
		"user_id":    true,
		"username":   true,
		"text":       true,
		"timestamp":  true,
		"tags":       true,
		"event_type": true, // Optional
		"event_data": true, // Optional
	}

	for field := range decoded {
		if !allowedFields[field] {
			t.Errorf("JSON contains extra field not in schema: %s", field)
		}
	}

	// Verify platform is "youtube"
	if platform, ok := decoded["platform"].(string); !ok || platform != "youtube" {
		t.Errorf("Platform = %v, want 'youtube'", decoded["platform"])
	}
}
