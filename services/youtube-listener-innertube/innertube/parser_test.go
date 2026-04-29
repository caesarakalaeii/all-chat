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
				if amount, ok := msg.EventData["amount_display"].(string); !ok || amount != "$5.00" {
					t.Errorf("EventData[amount_display] = %v, want '$5.00'", msg.EventData["amount_display"])
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
				if msg.EventType != "member_joined" {
					t.Errorf("EventType = %v, want member_joined", msg.EventType)
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
				if msg.EventType != "super_sticker" {
					t.Errorf("EventType = %v, want super_sticker", msg.EventType)
				}
				if msg.Text != "" {
					t.Errorf("Text = %v, want empty string", msg.Text)
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
			got, _ := extractMessageText(tt.message)
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

// TestSuperChatWithMetadata tests Super Chat parsing with rich metadata
func TestSuperChatWithMetadata(t *testing.T) {
	channelID := "UC_test_channel"

	actions := []ChatAction{
		{
			AddChatItemAction: &AddChatItemAction{
				Item: ChatItem{
					LiveChatPaidMessageRenderer: &LiveChatPaidMessageRenderer{
						Message: MessageContent{
							Runs: []MessageRun{{Text: "Great stream!"}},
						},
						AuthorName:              SimpleText{SimpleText: "BigDonor"},
						AuthorExternalChannelID: "UC_donor",
						TimestampUsec:           "1640000000000000",
						PurchaseAmountText:      SimpleText{SimpleText: "$50.00"},
						AmountMicros:            50000000,
						HeaderBackgroundColor:   0x1e88e5, // Blue tier
					},
				},
			},
		},
	}

	messages, err := ParseMessages(actions, channelID)
	if err != nil {
		t.Fatalf("ParseMessages() error = %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("ParseMessages() returned %d messages, want 1", len(messages))
	}

	msg := messages[0]
	if msg.EventType != "super_chat" {
		t.Errorf("EventType = %v, want super_chat", msg.EventType)
	}

	// Verify amount display string. The normalizer reads EventData["amount_display"];
	// historically the parser stored "$X.YZ" under "amount", so superchats normalized
	// to a hardcoded "$0.00" fallback (#254). The key name is now aligned.
	if amount, ok := msg.EventData["amount_display"].(string); !ok || amount != "$50.00" {
		t.Errorf("EventData[amount_display] = %v, want '$50.00'", msg.EventData["amount_display"])
	}

	// Verify amount_micros
	if amountMicros, ok := msg.EventData["amount_micros"].(int64); !ok || amountMicros != 50000000 {
		t.Errorf("EventData[amount_micros] = %v, want 50000000", msg.EventData["amount_micros"])
	}

	// Verify color formatting
	if color, ok := msg.EventData["color"].(string); !ok || color != "#1E88E5" {
		t.Errorf("EventData[color] = %v, want '#1E88E5'", msg.EventData["color"])
	}
}

// TestSuperChatNoMessage tests Super Chat without message text
func TestSuperChatNoMessage(t *testing.T) {
	channelID := "UC_test_channel"

	actions := []ChatAction{
		{
			AddChatItemAction: &AddChatItemAction{
				Item: ChatItem{
					LiveChatPaidMessageRenderer: &LiveChatPaidMessageRenderer{
						AuthorName:              SimpleText{SimpleText: "ShyDonor"},
						AuthorExternalChannelID: "UC_shy",
						TimestampUsec:           "1640000000000000",
						PurchaseAmountText:      SimpleText{SimpleText: "$5.00"},
					},
				},
			},
		},
	}

	messages, err := ParseMessages(actions, channelID)
	if err != nil {
		t.Fatalf("ParseMessages() error = %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("ParseMessages() returned %d messages, want 1", len(messages))
	}

	msg := messages[0]
	if msg.Text != "" {
		t.Errorf("Text = %v, want empty string for Super Chat without message", msg.Text)
	}
}

// TestSuperStickerWithURL tests Super Sticker parsing with sticker URL extraction
func TestSuperStickerWithURL(t *testing.T) {
	channelID := "UC_test_channel"

	actions := []ChatAction{
		{
			AddChatItemAction: &AddChatItemAction{
				Item: ChatItem{
					LiveChatPaidStickerRenderer: &LiveChatPaidStickerRenderer{
						AuthorName:              SimpleText{SimpleText: "StickerLover"},
						AuthorExternalChannelID: "UC_sticker",
						TimestampUsec:           "1640000000000000",
						PurchaseAmountText:      SimpleText{SimpleText: "$2.00"},
						AmountMicros:            2000000,
						Sticker: StickerContent{
							Thumbnails: Thumbnails{
								Thumbnails: []Thumbnail{
									{URL: "https://example.com/sticker.png", Width: 64, Height: 64},
								},
							},
						},
					},
				},
			},
		},
	}

	messages, err := ParseMessages(actions, channelID)
	if err != nil {
		t.Fatalf("ParseMessages() error = %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("ParseMessages() returned %d messages, want 1", len(messages))
	}

	msg := messages[0]
	if msg.EventType != "super_sticker" {
		t.Errorf("EventType = %v, want super_sticker", msg.EventType)
	}

	// Verify sticker URL
	if stickerURL, ok := msg.EventData["sticker_url"].(string); !ok || stickerURL != "https://example.com/sticker.png" {
		t.Errorf("EventData[sticker_url] = %v, want 'https://example.com/sticker.png'", msg.EventData["sticker_url"])
	}

	// Verify amount_micros
	if amountMicros, ok := msg.EventData["amount_micros"].(int64); !ok || amountMicros != 2000000 {
		t.Errorf("EventData[amount_micros] = %v, want 2000000", msg.EventData["amount_micros"])
	}
}

// TestMembershipWelcome tests new member join parsing
func TestMembershipWelcome(t *testing.T) {
	channelID := "UC_test_channel"

	actions := []ChatAction{
		{
			AddChatItemAction: &AddChatItemAction{
				Item: ChatItem{
					LiveChatMembershipItemRenderer: &LiveChatMembershipItemRenderer{
						HeaderSubtext: MessageContent{
							Runs: []MessageRun{{Text: "Welcome to the club!"}},
						},
						AuthorName:              SimpleText{SimpleText: "NewMember"},
						AuthorExternalChannelID: "UC_new",
						TimestampUsec:           "1640000000000000",
						AuthorBadges: []AuthorBadge{
							{
								LiveChatAuthorBadgeRenderer: LiveChatAuthorBadgeRenderer{
									Tooltip: "New member",
								},
							},
						},
					},
				},
			},
		},
	}

	messages, err := ParseMessages(actions, channelID)
	if err != nil {
		t.Fatalf("ParseMessages() error = %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("ParseMessages() returned %d messages, want 1", len(messages))
	}

	msg := messages[0]
	if msg.EventType != "member_joined" {
		t.Errorf("EventType = %v, want member_joined", msg.EventType)
	}

	// Verify level name from badge
	if levelName, ok := msg.EventData["level_name"].(string); !ok || levelName != "New member" {
		t.Errorf("EventData[level_name] = %v, want 'New member'", msg.EventData["level_name"])
	}
}

// TestMembershipMilestone tests membership milestone parsing with month extraction
func TestMembershipMilestone(t *testing.T) {
	channelID := "UC_test_channel"

	tests := []struct {
		name         string
		headerText   string
		wantMonths   int
		wantEventType string
	}{
		{
			name:         "6 month milestone",
			headerText:   "Member for 6 months",
			wantMonths:   6,
			wantEventType: "member_milestone",
		},
		{
			name:         "1 month milestone",
			headerText:   "Member for 1 month",
			wantMonths:   1,
			wantEventType: "member_milestone",
		},
		{
			name:         "12 month milestone",
			headerText:   "Member for 12 months",
			wantMonths:   12,
			wantEventType: "member_milestone",
		},
		{
			name:         "welcome message (no milestone)",
			headerText:   "Welcome to membership!",
			wantMonths:   0,
			wantEventType: "member_joined",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actions := []ChatAction{
				{
					AddChatItemAction: &AddChatItemAction{
						Item: ChatItem{
							LiveChatMembershipItemRenderer: &LiveChatMembershipItemRenderer{
								HeaderSubtext: MessageContent{
									Runs: []MessageRun{{Text: tt.headerText}},
								},
								AuthorName:              SimpleText{SimpleText: "LoyalMember"},
								AuthorExternalChannelID: "UC_loyal",
								TimestampUsec:           "1640000000000000",
							},
						},
					},
				},
			}

			messages, err := ParseMessages(actions, channelID)
			if err != nil {
				t.Fatalf("ParseMessages() error = %v", err)
			}

			if len(messages) != 1 {
				t.Fatalf("ParseMessages() returned %d messages, want 1", len(messages))
			}

			msg := messages[0]
			if msg.EventType != tt.wantEventType {
				t.Errorf("EventType = %v, want %v", msg.EventType, tt.wantEventType)
			}

			if tt.wantMonths > 0 {
				// Verify months extracted
				if months, ok := msg.EventData["months"].(int); !ok || months != tt.wantMonths {
					t.Errorf("EventData[months] = %v, want %v", msg.EventData["months"], tt.wantMonths)
				}
			} else {
				// Should not have months field for welcome messages
				if _, hasMonths := msg.EventData["months"]; hasMonths {
					t.Errorf("EventData should not contain 'months' for welcome message")
				}
			}
		})
	}
}

// TestTickerEventSuperChat tests ticker event (pinned Super Chat)
func TestTickerEventSuperChat(t *testing.T) {
	channelID := "UC_test_channel"

	actions := []ChatAction{
		{
			AddLiveChatTickerItem: &AddLiveChatTickerItem{
				Item: ChatItem{
					LiveChatPaidMessageRenderer: &LiveChatPaidMessageRenderer{
						Message: MessageContent{
							Runs: []MessageRun{{Text: "Pinned message!"}},
						},
						AuthorName:              SimpleText{SimpleText: "Pinner"},
						AuthorExternalChannelID: "UC_pinner",
						TimestampUsec:           "1640000000000000",
						PurchaseAmountText:      SimpleText{SimpleText: "$100.00"},
						AmountMicros:            100000000,
					},
				},
				DurationSec: 300, // 5 minutes
			},
		},
	}

	messages, err := ParseMessages(actions, channelID)
	if err != nil {
		t.Fatalf("ParseMessages() error = %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("ParseMessages() returned %d messages, want 1", len(messages))
	}

	msg := messages[0]
	if msg.EventType != "super_chat" {
		t.Errorf("EventType = %v, want super_chat", msg.EventType)
	}

	// Verify pinned flag
	if pinned, ok := msg.EventData["pinned"].(bool); !ok || !pinned {
		t.Errorf("EventData[pinned] = %v, want true", msg.EventData["pinned"])
	}

	// Verify ticker duration
	if duration, ok := msg.EventData["ticker_duration_sec"].(int); !ok || duration != 300 {
		t.Errorf("EventData[ticker_duration_sec] = %v, want 300", msg.EventData["ticker_duration_sec"])
	}
}

// TestExtractMilestoneMonths tests the milestone month extraction function
func TestExtractMilestoneMonths(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		want  int
	}{
		{
			name: "standard format",
			text: "Member for 6 months",
			want: 6,
		},
		{
			name: "single month",
			text: "Member for 1 month",
			want: 1,
		},
		{
			name: "large number",
			text: "Member for 24 months",
			want: 24,
		},
		{
			name: "no months",
			text: "Welcome to membership!",
			want: 0,
		},
		{
			name: "case insensitive",
			text: "MEMBER FOR 12 MONTHS",
			want: 12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMilestoneMonths(tt.text)
			if got != tt.want {
				t.Errorf("extractMilestoneMonths(%q) = %v, want %v", tt.text, got, tt.want)
			}
		})
	}
}

// TestFormatColorFromInt tests color formatting
func TestFormatColorFromInt(t *testing.T) {
	tests := []struct {
		name  string
		color int
		want  string
	}{
		{
			name:  "blue tier",
			color: 0x1e88e5,
			want:  "#1E88E5",
		},
		{
			name:  "red tier",
			color: 0xe91e63,
			want:  "#E91E63",
		},
		{
			name:  "yellow tier",
			color: 0xffeb3b,
			want:  "#FFEB3B",
		},
		{
			name:  "zero color",
			color: 0,
			want:  "#000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatColorFromInt(tt.color)
			if got != tt.want {
				t.Errorf("formatColorFromInt(%#x) = %v, want %v", tt.color, got, tt.want)
			}
		})
	}
}

// TestParseDeletionEvent tests deletion event parsing
func TestParseDeletionEvent(t *testing.T) {
	channelID := "UC_test_channel"

	t.Run("single deletion event", func(t *testing.T) {
		actions := []ChatAction{
			{
				MarkChatItemAsDeletedAction: &MarkChatItemAsDeletedAction{
					DeletedStateMessage: MessageContent{
						Runs: []MessageRun{{Text: "[message deleted]"}},
					},
					TargetItemID:  "ChwKGkNNT3M4UF9BdTRvRENNeTQ5Z0FkaERaeFhBZw%3D%3D",
					TimestampUsec: "1640000000000000",
				},
			},
		}

		messages, err := ParseMessages(actions, channelID)
		if err != nil {
			t.Fatalf("ParseMessages() error = %v", err)
		}

		if len(messages) != 1 {
			t.Fatalf("ParseMessages() returned %d messages, want 1", len(messages))
		}

		msg := messages[0]
		if msg.EventType != "message_deletion" {
			t.Errorf("EventType = %v, want message_deletion", msg.EventType)
		}
		if msg.Platform != "youtube" {
			t.Errorf("Platform = %v, want youtube", msg.Platform)
		}
		if msg.ChannelID != channelID {
			t.Errorf("ChannelID = %v, want %v", msg.ChannelID, channelID)
		}
		if msg.Text != "" {
			t.Errorf("Text = %v, want empty string", msg.Text)
		}
		if msg.UserID != "" {
			t.Errorf("UserID = %v, want empty string (deletion events have no user)", msg.UserID)
		}
		if msg.Username != "" {
			t.Errorf("Username = %v, want empty string (deletion events have no username)", msg.Username)
		}
		if msg.MessageID == "" {
			t.Error("MessageID should be generated")
		}

		// Verify event data
		if deletionType, ok := msg.EventData["deletion_type"].(string); !ok || deletionType != "single" {
			t.Errorf("EventData[deletion_type] = %v, want 'single'", msg.EventData["deletion_type"])
		}
		if targetMsgID, ok := msg.EventData["target_msg_id"].(string); !ok || targetMsgID == "" {
			t.Errorf("EventData[target_msg_id] = %v, want non-empty string", msg.EventData["target_msg_id"])
		}
		if targetMsgID, ok := msg.EventData["target_msg_id"].(string); ok {
			if targetMsgID != "ChwKGkNNT3M4UF9BdTRvRENNeTQ5Z0FkaERaeFhBZw%3D%3D" {
				t.Errorf("EventData[target_msg_id] = %v, want ChwKGkNNT3M4UF9BdTRvRENNeTQ5Z0FkaERaeFhBZw%%3D%%3D", targetMsgID)
			}
		}
	})

	t.Run("deletion event without timestamp", func(t *testing.T) {
		actions := []ChatAction{
			{
				MarkChatItemAsDeletedAction: &MarkChatItemAsDeletedAction{
					DeletedStateMessage: MessageContent{
						Runs: []MessageRun{{Text: "[message deleted]"}},
					},
					TargetItemID: "test_item_id",
				},
			},
		}

		messages, err := ParseMessages(actions, channelID)
		if err != nil {
			t.Fatalf("ParseMessages() error = %v", err)
		}

		if len(messages) != 1 {
			t.Fatalf("ParseMessages() returned %d messages, want 1", len(messages))
		}

		msg := messages[0]
		if msg.Timestamp.IsZero() {
			t.Error("Timestamp should be set (current time if not provided)")
		}
	})

	t.Run("mixed events - regular messages and deletions", func(t *testing.T) {
		actions := []ChatAction{
			{
				AddChatItemAction: &AddChatItemAction{
					Item: ChatItem{
						LiveChatTextMessageRenderer: &LiveChatTextMessageRenderer{
							Message: MessageContent{
								Runs: []MessageRun{{Text: "Message 1"}},
							},
							AuthorName:              SimpleText{SimpleText: "User1"},
							AuthorExternalChannelID: "UC1",
							TimestampUsec:           "1640000000000000",
						},
					},
				},
			},
			{
				AddChatItemAction: &AddChatItemAction{
					Item: ChatItem{
						LiveChatTextMessageRenderer: &LiveChatTextMessageRenderer{
							Message: MessageContent{
								Runs: []MessageRun{{Text: "Message 2"}},
							},
							AuthorName:              SimpleText{SimpleText: "User2"},
							AuthorExternalChannelID: "UC2",
							TimestampUsec:           "1640000001000000",
						},
					},
				},
			},
			{
				MarkChatItemAsDeletedAction: &MarkChatItemAsDeletedAction{
					TargetItemID:  "deleted_1",
					TimestampUsec: "1640000002000000",
				},
			},
			{
				AddChatItemAction: &AddChatItemAction{
					Item: ChatItem{
						LiveChatTextMessageRenderer: &LiveChatTextMessageRenderer{
							Message: MessageContent{
								Runs: []MessageRun{{Text: "Message 3"}},
							},
							AuthorName:              SimpleText{SimpleText: "User3"},
							AuthorExternalChannelID: "UC3",
							TimestampUsec:           "1640000003000000",
						},
					},
				},
			},
			{
				MarkChatItemAsDeletedAction: &MarkChatItemAsDeletedAction{
					TargetItemID:  "deleted_2",
					TimestampUsec: "1640000004000000",
				},
			},
		}

		messages, err := ParseMessages(actions, channelID)
		if err != nil {
			t.Fatalf("ParseMessages() error = %v", err)
		}

		if len(messages) != 5 {
			t.Fatalf("ParseMessages() returned %d messages, want 5", len(messages))
		}

		// Verify message types
		regularCount := 0
		deletionCount := 0
		for _, msg := range messages {
			if msg.EventType == "message_deletion" {
				deletionCount++
			} else if msg.EventType == "" {
				regularCount++
			}
		}

		if regularCount != 3 {
			t.Errorf("Regular messages = %d, want 3", regularCount)
		}
		if deletionCount != 2 {
			t.Errorf("Deletion events = %d, want 2", deletionCount)
		}
	})
}

// TestDeletionEventSchemaMatch tests deletion event schema matches official listener
func TestDeletionEventSchemaMatch(t *testing.T) {
	channelID := "UC_test_channel"

	actions := []ChatAction{
		{
			MarkChatItemAsDeletedAction: &MarkChatItemAsDeletedAction{
				TargetItemID:  "test_deleted_message_id",
				TimestampUsec: "1640000000000000",
			},
		},
	}

	messages, err := ParseMessages(actions, channelID)
	if err != nil {
		t.Fatalf("ParseMessages() error = %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("ParseMessages() returned %d messages, want 1", len(messages))
	}

	msg := messages[0]

	// Marshal to JSON
	jsonData, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal deletion event: %v", err)
	}

	// Unmarshal back to verify structure
	var decoded map[string]interface{}
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Verify required fields for deletion event
	if eventType, ok := decoded["event_type"].(string); !ok || eventType != "message_deletion" {
		t.Errorf("event_type = %v, want 'message_deletion'", decoded["event_type"])
	}

	if platform, ok := decoded["platform"].(string); !ok || platform != "youtube" {
		t.Errorf("platform = %v, want 'youtube'", decoded["platform"])
	}

	// Verify event_data structure
	eventData, ok := decoded["event_data"].(map[string]interface{})
	if !ok {
		t.Fatal("event_data should be a map")
	}

	if _, ok := eventData["target_msg_id"]; !ok {
		t.Error("event_data should contain target_msg_id")
	}

	if deletionType, ok := eventData["deletion_type"].(string); !ok || deletionType != "single" {
		t.Errorf("event_data.deletion_type = %v, want 'single'", eventData["deletion_type"])
	}

	// Verify deletion events have empty user fields
	if userID, ok := decoded["user_id"].(string); !ok || userID != "" {
		t.Errorf("user_id = %v, want empty string", decoded["user_id"])
	}

	if username, ok := decoded["username"].(string); !ok || username != "" {
		t.Errorf("username = %v, want empty string", decoded["username"])
	}

	if text, ok := decoded["text"].(string); !ok || text != "" {
		t.Errorf("text = %v, want empty string", decoded["text"])
	}
}

// TestValidateRawMessage_DeletionEvent tests validation for deletion events
func TestValidateRawMessage_DeletionEvent(t *testing.T) {
	t.Run("valid deletion event", func(t *testing.T) {
		msg := &RawChatMessage{
			MessageID: "test-id",
			Platform:  "youtube",
			ChannelID: "UC_channel",
			EventType: "message_deletion",
			Timestamp: time.Now(),
			Tags:      make(map[string]string),
			EventData: map[string]interface{}{
				"target_msg_id":  "deleted_id",
				"deletion_type":  "single",
			},
		}

		err := ValidateRawMessage(msg)
		if err != nil {
			t.Errorf("ValidateRawMessage() should pass for deletion event, got error: %v", err)
		}
	})

	t.Run("deletion event without user fields (valid)", func(t *testing.T) {
		msg := &RawChatMessage{
			MessageID: "test-id",
			Platform:  "youtube",
			EventType: "message_deletion",
			Timestamp: time.Now(),
			UserID:    "", // Deletion events don't have user
			Username:  "", // Deletion events don't have username
			Text:      "", // Deletion events don't have text
		}

		err := ValidateRawMessage(msg)
		if err != nil {
			t.Errorf("ValidateRawMessage() should pass for deletion event without user fields, got error: %v", err)
		}
	})
}
