package normalizer

import (
	"testing"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/stretchr/testify/assert"
)

func TestNewYouTubeNormalizer(t *testing.T) {
	normalizer := NewYouTubeNormalizer()
	assert.NotNil(t, normalizer)
}

func TestYouTubeNormalizer_Normalize_BasicMessage(t *testing.T) {
	normalizer := NewYouTubeNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "msg-123",
		Platform:  "youtube",
		ChannelID: "UCxxxxxx",
		UserID:    "UCyyyyyy",
		Username:  "TestUser",
		Text:      "Hello world!",
		Timestamp: time.Now(),
		Tags: map[string]string{
			"display_name":  "Test Channel",
			"profile_image": "https://example.com/avatar.jpg",
			"is_verified":   "false",
			"is_owner":      "false",
			"is_sponsor":    "false",
			"is_moderator":  "false",
			"super_chat":    "0",
			"super_sticker": "0",
		},
	}

	unified, err := normalizer.Normalize(raw, "overlay-456")

	assert.NoError(t, err)
	assert.NotNil(t, unified)
	assert.Equal(t, "msg-123", unified.ID)
	assert.Equal(t, "overlay-456", unified.OverlayID)
	assert.Equal(t, "youtube", unified.Platform)
	assert.Equal(t, "UCxxxxxx", unified.ChannelID)
	assert.Equal(t, "Test Channel", unified.ChannelName)
	assert.Equal(t, "UCyyyyyy", unified.User.ID)
	assert.Equal(t, "TestUser", unified.User.Username)
	assert.Equal(t, "TestUser", unified.User.DisplayName)
	assert.Equal(t, "https://example.com/avatar.jpg", unified.User.AvatarURL)
	assert.Empty(t, unified.User.Color) // YouTube doesn't have user colors
	assert.Equal(t, "Hello world!", unified.Message.Text)
	assert.Empty(t, unified.Message.Emotes) // No native YouTube emotes
}

func TestYouTubeNormalizer_Normalize_InvalidChannelID(t *testing.T) {
	normalizer := NewYouTubeNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "msg-123",
		Platform:  "youtube",
		ChannelID: "../evil",
		UserID:    "UCyyyyyy",
		Username:  "TestUser",
		Text:      "Hello world!",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
	}

	_, err := normalizer.Normalize(raw, "overlay-456")
	assert.Error(t, err)
}

func TestYouTubeNormalizer_Normalize_WrongPlatform(t *testing.T) {
	normalizer := NewYouTubeNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "msg-123",
		Platform:  "twitch", // Wrong platform
		ChannelID: "xqc",
		UserID:    "12345",
		Username:  "viewer",
		Text:      "Hello",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
	}

	unified, err := normalizer.Normalize(raw, "overlay-456")

	assert.Error(t, err)
	assert.Nil(t, unified)
	assert.Contains(t, err.Error(), "unsupported platform")
}

func TestYouTubeNormalizer_ExtractBadges_Owner(t *testing.T) {
	normalizer := NewYouTubeNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "msg-123",
		Platform:  "youtube",
		ChannelID: "UCxxxxxx",
		UserID:    "UCxxxxxx", // Same as channel = owner
		Username:  "ChannelOwner",
		Text:      "Hello from owner!",
		Timestamp: time.Now(),
		Tags: map[string]string{
			"display_name": "My Channel",
			"is_owner":     "true",
			"is_verified":  "false",
			"is_sponsor":   "false",
			"is_moderator": "false",
		},
	}

	unified, err := normalizer.Normalize(raw, "overlay-456")

	assert.NoError(t, err)
	assertBadgePresent(t, unified.User.Badges, "owner")
	assert.True(t, unified.Metadata["is_owner"].(bool))
}

func TestYouTubeNormalizer_ExtractBadges_Member(t *testing.T) {
	normalizer := NewYouTubeNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "msg-123",
		Platform:  "youtube",
		ChannelID: "UCxxxxxx",
		UserID:    "UCyyyyyy",
		Username:  "Member",
		Text:      "Member message",
		Timestamp: time.Now(),
		Tags: map[string]string{
			"display_name": "Channel",
			"is_owner":     "false",
			"is_sponsor":   "true", // Member/sponsor
			"is_moderator": "false",
			"is_verified":  "false",
		},
	}

	unified, err := normalizer.Normalize(raw, "overlay-456")

	assert.NoError(t, err)
	assertBadgePresent(t, unified.User.Badges, "member")
	assert.True(t, unified.Metadata["is_sponsor"].(bool))
}

func TestYouTubeNormalizer_ExtractBadges_Moderator(t *testing.T) {
	normalizer := NewYouTubeNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "msg-123",
		Platform:  "youtube",
		ChannelID: "UCxxxxxx",
		UserID:    "UCyyyyyy",
		Username:  "Moderator",
		Text:      "Mod message",
		Timestamp: time.Now(),
		Tags: map[string]string{
			"display_name": "Channel",
			"is_owner":     "false",
			"is_sponsor":   "false",
			"is_moderator": "true",
			"is_verified":  "false",
		},
	}

	unified, err := normalizer.Normalize(raw, "overlay-456")

	assert.NoError(t, err)
	assertBadgePresent(t, unified.User.Badges, "moderator")
	assert.True(t, unified.Metadata["is_moderator"].(bool))
}

func TestYouTubeNormalizer_ExtractBadges_Verified(t *testing.T) {
	normalizer := NewYouTubeNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "msg-123",
		Platform:  "youtube",
		ChannelID: "UCxxxxxx",
		UserID:    "UCyyyyyy",
		Username:  "VerifiedUser",
		Text:      "Verified message",
		Timestamp: time.Now(),
		Tags: map[string]string{
			"display_name": "Channel",
			"is_owner":     "false",
			"is_sponsor":   "false",
			"is_moderator": "false",
			"is_verified":  "true",
		},
	}

	unified, err := normalizer.Normalize(raw, "overlay-456")

	assert.NoError(t, err)
	assertBadgePresent(t, unified.User.Badges, "verified")
	assert.True(t, unified.Metadata["is_verified"].(bool))
}

func TestYouTubeNormalizer_ExtractBadges_MultipleBadges(t *testing.T) {
	normalizer := NewYouTubeNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "msg-123",
		Platform:  "youtube",
		ChannelID: "UCxxxxxx",
		UserID:    "UCyyyyyy",
		Username:  "SuperUser",
		Text:      "VIP message",
		Timestamp: time.Now(),
		Tags: map[string]string{
			"display_name": "Channel",
			"is_owner":     "false",
			"is_sponsor":   "true",
			"is_moderator": "true",
			"is_verified":  "true",
		},
	}

	unified, err := normalizer.Normalize(raw, "overlay-456")

	assert.NoError(t, err)
	assertBadgePresent(t, unified.User.Badges, "member")
	assertBadgePresent(t, unified.User.Badges, "moderator")
	assertBadgePresent(t, unified.User.Badges, "verified")
	assert.Len(t, unified.User.Badges, 3)
}

func TestYouTubeNormalizer_SuperChat(t *testing.T) {
	normalizer := NewYouTubeNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "msg-123",
		Platform:  "youtube",
		ChannelID: "UCxxxxxx",
		UserID:    "UCyyyyyy",
		Username:  "Donor",
		Text:      "Great stream!",
		Timestamp: time.Now(),
		Tags: map[string]string{
			"display_name":        "Channel",
			"super_chat":          "5000000", // $5.00 in micros
			"super_chat_currency": "USD",
			"super_chat_display":  "$5.00",
			"super_sticker":       "0",
		},
	}

	unified, err := normalizer.Normalize(raw, "overlay-456")

	assert.NoError(t, err)
	assert.Equal(t, int64(5000000), unified.Metadata["super_chat_amount"])
	assert.Equal(t, "USD", unified.Metadata["super_chat_currency"])
	assert.Equal(t, "$5.00", unified.Metadata["super_chat_display"])
}

func TestYouTubeNormalizer_SuperSticker(t *testing.T) {
	normalizer := NewYouTubeNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "msg-123",
		Platform:  "youtube",
		ChannelID: "UCxxxxxx",
		UserID:    "UCyyyyyy",
		Username:  "StickerUser",
		Text:      "Thumbs up!",
		Timestamp: time.Now(),
		Tags: map[string]string{
			"display_name":           "Channel",
			"super_chat":             "0",
			"super_sticker":          "2000000", // $2.00 in micros
			"super_sticker_currency": "USD",
			"super_sticker_display":  "$2.00",
			"super_sticker_tier":     "2",
		},
	}

	unified, err := normalizer.Normalize(raw, "overlay-456")

	assert.NoError(t, err)
	assert.Equal(t, int64(2000000), unified.Metadata["super_sticker_amount"])
	assert.Equal(t, "USD", unified.Metadata["super_sticker_currency"])
	assert.Equal(t, "$2.00", unified.Metadata["super_sticker_display"])
	assert.Equal(t, "2", unified.Metadata["super_sticker_tier"])
}

func TestYouTubeNormalizer_Metadata_TwitchFields(t *testing.T) {
	normalizer := NewYouTubeNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "msg-123",
		Platform:  "youtube",
		ChannelID: "UCxxxxxx",
		UserID:    "UCyyyyyy",
		Username:  "User",
		Text:      "Test",
		Timestamp: time.Now(),
		Tags: map[string]string{
			"display_name": "Channel",
		},
	}

	unified, err := normalizer.Normalize(raw, "overlay-456")

	assert.NoError(t, err)
	// Verify Twitch-specific fields are set to default values
	assert.Equal(t, 0, unified.Metadata["bits"])
	assert.Equal(t, false, unified.Metadata["is_subscriber"])
	assert.Equal(t, false, unified.Metadata["is_turbo"])
}
