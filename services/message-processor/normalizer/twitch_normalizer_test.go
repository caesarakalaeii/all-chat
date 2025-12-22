package normalizer

import (
	"testing"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTwitchNormalizer_Normalize(t *testing.T) {
	normalizer := NewTwitchNormalizer()

	tests := []struct {
		name      string
		raw       *models.RawChatMessage
		overlayID string
		check     func(*testing.T, *models.UnifiedChatMessage)
		wantErr   bool
	}{
		{
			name: "invalid channel id",
			raw: &models.RawChatMessage{
				MessageID: "msg-invalid",
				Platform:  "twitch",
				ChannelID: "../bad",
				UserID:    "12345",
				Username:  "viewer123",
				Text:      "Hello World",
				Timestamp: time.Now().UTC(),
				Tags:      map[string]string{},
			},
			overlayID: "overlay-1",
			wantErr:   true,
		},
		{
			name: "basic message",
			raw: &models.RawChatMessage{
				MessageID: "msg-123",
				Platform:  "twitch",
				ChannelID: "xqc",
				UserID:    "12345",
				Username:  "viewer123",
				Text:      "Hello World",
				Timestamp: time.Now().UTC(),
				Tags: map[string]string{
					"display-name": "Viewer123",
					"color":        "#FF0000",
					"badges":       "subscriber/12",
					"subscriber":   "1",
				},
			},
			overlayID: "overlay-1",
			check: func(t *testing.T, msg *models.UnifiedChatMessage) {
				assert.Equal(t, "msg-123", msg.ID)
				assert.Equal(t, "overlay-1", msg.OverlayID)
				assert.Equal(t, "twitch", msg.Platform)
				assert.Equal(t, "xqc", msg.ChannelID)
				assert.Equal(t, "xqc", msg.ChannelName)
				assert.Equal(t, "12345", msg.User.ID)
				assert.Equal(t, "viewer123", msg.User.Username)
				assert.Equal(t, "Viewer123", msg.User.DisplayName)
				assert.Equal(t, "#FF0000", msg.User.Color)
				assertBadgePresent(t, msg.User.Badges, "subscriber")
				assert.Equal(t, "Hello World", msg.Message.Text)
				assert.True(t, msg.Metadata["is_subscriber"].(bool))
			},
		},
		{
			name: "message with emotes",
			raw: &models.RawChatMessage{
				MessageID: "msg-456",
				Platform:  "twitch",
				ChannelID: "summit1g",
				UserID:    "999",
				Username:  "emoteuser",
				Text:      "Kappa test Kappa",
				Timestamp: time.Now().UTC(),
				Tags: map[string]string{
					"display-name": "EmoteUser",
					"emotes":       "25:0-4,11-15",
				},
			},
			overlayID: "overlay-2",
			check: func(t *testing.T, msg *models.UnifiedChatMessage) {
				assert.Equal(t, "Kappa test Kappa", msg.Message.Text)
				assert.Len(t, msg.Message.Emotes, 1)

				emote := msg.Message.Emotes[0]
				assert.Equal(t, "Kappa", emote.Code)
				assert.Equal(t, "twitch", emote.Provider)
				assert.Contains(t, emote.URL, "25")
				assert.Len(t, emote.Positions, 2)
				assert.Equal(t, []int{0, 4}, emote.Positions[0])
				assert.Equal(t, []int{11, 15}, emote.Positions[1])
			},
		},
		{
			name: "message with multiple emote types",
			raw: &models.RawChatMessage{
				MessageID: "msg-789",
				Platform:  "twitch",
				ChannelID: "test",
				UserID:    "111",
				Username:  "user",
				Text:      "Kappa test PogChamp nice",
				Timestamp: time.Now().UTC(),
				Tags: map[string]string{
					"emotes": "25:0-4/88:11-18", // PogChamp is 8 chars, positions 11-18
				},
			},
			overlayID: "overlay-3",
			check: func(t *testing.T, msg *models.UnifiedChatMessage) {
				assert.Len(t, msg.Message.Emotes, 2)
				assert.Equal(t, "Kappa", msg.Message.Emotes[0].Code)
				assert.Equal(t, "PogChamp", msg.Message.Emotes[1].Code)
			},
		},
		{
			name: "message with badges",
			raw: &models.RawChatMessage{
				MessageID: "msg-999",
				Platform:  "twitch",
				ChannelID: "test",
				UserID:    "222",
				Username:  "mod",
				Text:      "test",
				Timestamp: time.Now().UTC(),
				Tags: map[string]string{
					"badges": "moderator/1,subscriber/24,turbo/1",
					"mod":    "1",
				},
			},
			overlayID: "overlay-4",
			check: func(t *testing.T, msg *models.UnifiedChatMessage) {
				assertBadgePresent(t, msg.User.Badges, "moderator")
				assertBadgePresent(t, msg.User.Badges, "subscriber")
				assertBadgePresent(t, msg.User.Badges, "turbo")
				assert.True(t, msg.Metadata["is_moderator"].(bool))
			},
		},
		{
			name: "shared chat - all source tags present",
			raw: &models.RawChatMessage{
				MessageID: "msg-shared-1",
				Platform:  "twitch",
				ChannelID: "hostchannel",
				UserID:    "123456",
				Username:  "guestuser",
				Text:      "Hello from shared chat!",
				Timestamp: time.Now().UTC(),
				Tags: map[string]string{
					"display-name":      "GuestUser",
					"color":             "#00FF00",
					"badges":            "subscriber/6",
					"subscriber":        "1",
					"source-room-id":    "987654321",
					"source-id":         "123456",
					"source-badges":     "subscriber/6,moderator/1",
					"source-badge-info": "subscriber/6",
					"room-id":           "111222333",
				},
			},
			overlayID: "overlay-shared-1",
			check: func(t *testing.T, msg *models.UnifiedChatMessage) {
				assert.Equal(t, "msg-shared-1", msg.ID)
				assert.Equal(t, "overlay-shared-1", msg.OverlayID)
				assert.Equal(t, "hostchannel", msg.ChannelID)
				assert.Equal(t, "guestuser", msg.User.Username)
				assert.Equal(t, "GuestUser", msg.User.DisplayName)
				
				// Verify regular badges
				assertBadgePresent(t, msg.User.Badges, "subscriber")
				
				// Verify source badges extracted
				assert.Len(t, msg.User.SourceBadges, 2)
				assertBadgePresent(t, msg.User.SourceBadges, "subscriber")
				assertBadgePresent(t, msg.User.SourceBadges, "moderator")
				
				// Verify source user ID
				assert.Equal(t, "123456", msg.User.SourceUserID)
				
				// Verify shared chat metadata
				assert.True(t, msg.Metadata["is_shared_chat"].(bool))
				assert.Equal(t, "987654321", msg.Metadata["source_room_id"])
				assert.Equal(t, "111222333", msg.Metadata["twitch_room_id"])
			},
		},
		{
			name: "shared chat - partial source tags",
			raw: &models.RawChatMessage{
				MessageID: "msg-shared-2",
				Platform:  "twitch",
				ChannelID: "hostchannel",
				UserID:    "789",
				Username:  "partialuser",
				Text:      "Partial shared chat",
				Timestamp: time.Now().UTC(),
				Tags: map[string]string{
					"display-name":   "PartialUser",
					"source-room-id": "444555666",
					// No source-badges, source-id
				},
			},
			overlayID: "overlay-shared-2",
			check: func(t *testing.T, msg *models.UnifiedChatMessage) {
				// Verify shared chat is detected
				assert.True(t, msg.Metadata["is_shared_chat"].(bool))
				assert.Equal(t, "444555666", msg.Metadata["source_room_id"])
				
				// Source badges should be empty
				assert.Empty(t, msg.User.SourceBadges)
				assert.Empty(t, msg.User.SourceUserID)
			},
		},
		{
			name: "regular message - no shared chat tags",
			raw: &models.RawChatMessage{
				MessageID: "msg-regular",
				Platform:  "twitch",
				ChannelID: "regularchannel",
				UserID:    "999",
				Username:  "regularuser",
				Text:      "Regular message",
				Timestamp: time.Now().UTC(),
				Tags: map[string]string{
					"display-name": "RegularUser",
					"badges":       "subscriber/3",
					"subscriber":   "1",
				},
			},
			overlayID: "overlay-regular",
			check: func(t *testing.T, msg *models.UnifiedChatMessage) {
				// Verify NOT shared chat
				assert.False(t, msg.Metadata["is_shared_chat"].(bool))
				
				// No source fields should be present
				assert.Empty(t, msg.User.SourceBadges)
				assert.Empty(t, msg.User.SourceUserID)
				assert.NotContains(t, msg.Metadata, "source_room_id")
				
				// Regular badges should work
				assertBadgePresent(t, msg.User.Badges, "subscriber")
			},
		},
		{
			name: "unsupported platform",
			raw: &models.RawChatMessage{
				MessageID: "msg-error",
				Platform:  "youtube",
				ChannelID: "test",
				UserID:    "123",
				Username:  "user",
				Text:      "test",
				Timestamp: time.Now().UTC(),
				Tags:      map[string]string{},
			},
			overlayID: "overlay-5",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := normalizer.Normalize(tt.raw, tt.overlayID)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				if tt.check != nil {
					tt.check(t, result)
				}
			}
		})
	}
}

func TestTwitchNormalizer_ExtractUserInfo(t *testing.T) {
	normalizer := NewTwitchNormalizer()

	raw := &models.RawChatMessage{
		UserID:   "12345",
		Username: "testuser",
		Tags: map[string]string{
			"display-name": "TestUser",
			"color":        "#00FF00",
			"badges":       "broadcaster/1,moderator/1",
		},
	}

	userInfo := normalizer.extractUserInfo(raw)

	assert.Equal(t, "12345", userInfo.ID)
	assert.Equal(t, "testuser", userInfo.Username)
	assert.Equal(t, "TestUser", userInfo.DisplayName)
	assert.Equal(t, "#00FF00", userInfo.Color)
	assertBadgePresent(t, userInfo.Badges, "broadcaster")
	assertBadgePresent(t, userInfo.Badges, "moderator")
}

func assertBadgePresent(t *testing.T, badges []models.Badge, name string) {
	t.Helper()
	for _, badge := range badges {
		if badge.Name == name {
			return
		}
	}
	t.Fatalf("expected badge %q to be present", name)
}

func TestTwitchNormalizer_ExtractUserInfo_NoDisplayName(t *testing.T) {
	normalizer := NewTwitchNormalizer()

	raw := &models.RawChatMessage{
		UserID:   "999",
		Username: "user",
		Tags:     map[string]string{},
	}

	userInfo := normalizer.extractUserInfo(raw)

	// Should fallback to username
	assert.Equal(t, "user", userInfo.DisplayName)
}

func TestTwitchNormalizer_ExtractMetadata(t *testing.T) {
	normalizer := NewTwitchNormalizer()

	raw := &models.RawChatMessage{
		Tags: map[string]string{
			"subscriber": "1",
			"mod":        "0",
			"turbo":      "1",
			"id":         "twitch-msg-id",
			"room-id":    "71092938",
		},
	}

	metadata := normalizer.extractMetadata(raw)

	assert.True(t, metadata["is_subscriber"].(bool))
	assert.False(t, metadata["is_moderator"].(bool))
	assert.True(t, metadata["is_turbo"].(bool))
	assert.Equal(t, "twitch-msg-id", metadata["twitch_message_id"])
	assert.Equal(t, "71092938", metadata["twitch_room_id"])
	assert.Equal(t, 0, metadata["bits"])
	assert.Equal(t, 0, metadata["super_chat_amount"])
}

func TestTwitchNormalizer_ExtractMetadata_SharedChat(t *testing.T) {
normalizer := NewTwitchNormalizer()

raw := &models.RawChatMessage{
Tags: map[string]string{
"subscriber":     "1",
"mod":            "0",
"source-room-id": "987654321",
"room-id":        "111222333",
},
}

metadata := normalizer.extractMetadata(raw)

assert.True(t, metadata["is_shared_chat"].(bool))
assert.Equal(t, "987654321", metadata["source_room_id"])
assert.Equal(t, "111222333", metadata["twitch_room_id"])
assert.True(t, metadata["is_subscriber"].(bool))
}

func TestTwitchNormalizer_ExtractUserInfo_SharedChat(t *testing.T) {
normalizer := NewTwitchNormalizer()

raw := &models.RawChatMessage{
UserID:   "12345",
Username: "guestuser",
Tags: map[string]string{
"display-name":  "GuestUser",
"color":         "#00FF00",
"badges":        "subscriber/6",
"source-id":     "12345",
"source-badges": "subscriber/6,moderator/1,vip/1",
},
}

userInfo := normalizer.extractUserInfo(raw)

assert.Equal(t, "12345", userInfo.ID)
assert.Equal(t, "guestuser", userInfo.Username)
assert.Equal(t, "GuestUser", userInfo.DisplayName)
assert.Equal(t, "#00FF00", userInfo.Color)

// Regular badges
assertBadgePresent(t, userInfo.Badges, "subscriber")

// Source badges
assert.Len(t, userInfo.SourceBadges, 3)
assertBadgePresent(t, userInfo.SourceBadges, "subscriber")
assertBadgePresent(t, userInfo.SourceBadges, "moderator")
assertBadgePresent(t, userInfo.SourceBadges, "vip")

// Source user ID
assert.Equal(t, "12345", userInfo.SourceUserID)
}
