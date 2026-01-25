package normalizer

import (
	"testing"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeEvent_YouTubeSuperChat(t *testing.T) {
	normalizer := NewYouTubeNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "yt-msg-1",
		Platform:  "youtube",
		ChannelID: "UC1234567890",
		UserID:    "UC9876543210",
		Username:  "YTViewer",
		Text:      "Great stream!",
		Timestamp: time.Now(),
		Tags: map[string]string{
			"display_name": "YTViewer",
		},
		EventType: "super_chat",
		EventData: map[string]interface{}{
			"amount_micros":  int64(50000000), // $50
			"currency":       "USD",
			"amount_display": "$50.00",
		},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-yt-1")
	require.NoError(t, err)
	assert.NotNil(t, unified)

	// Verify event info
	assert.NotNil(t, unified.Event)
	assert.Equal(t, "super_chat", unified.Event.Type)
	assert.Equal(t, "high", unified.Event.Tier)
	assert.Equal(t, 60, unified.Event.Duration) // $50+ = 60s

	// Verify event value
	assert.Equal(t, float64(50000000), unified.Event.Value.Amount)
	assert.Equal(t, "USD", unified.Event.Value.Currency)
	assert.Equal(t, "$50.00", unified.Event.Value.DisplayText)
}

func TestNormalizeEvent_YouTubeSuperSticker(t *testing.T) {
	normalizer := NewYouTubeNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "yt-msg-2",
		Platform:  "youtube",
		ChannelID: "UC1234567890",
		UserID:    "UC1111111111",
		Username:  "StickerFan",
		Text:      "Awesome!",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "super_sticker",
		EventData: map[string]interface{}{
			"amount_micros":  int64(10000000), // $10
			"currency":       "USD",
			"amount_display": "$10.00",
		},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-yt-2")
	require.NoError(t, err)

	assert.Equal(t, "super_sticker", unified.Event.Type)
	assert.Equal(t, "high", unified.Event.Tier)
	assert.Equal(t, 40, unified.Event.Duration) // $10+ = 40s
}

func TestNormalizeEvent_YouTubeNewSponsor(t *testing.T) {
	normalizer := NewYouTubeNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "yt-msg-3",
		Platform:  "youtube",
		ChannelID: "UC1234567890",
		UserID:    "UC2222222222",
		Username:  "NewMember",
		Text:      "Became a member",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "new_sponsor",
		EventData: map[string]interface{}{
			"is_upgrade": false,
		},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-yt-3")
	require.NoError(t, err)

	assert.Equal(t, "new_sponsor", unified.Event.Type)
	assert.Equal(t, "high", unified.Event.Tier)
	assert.Equal(t, 30, unified.Event.Duration)
	assert.Equal(t, "New member", unified.Event.Value.DisplayText)
}

func TestNormalizeEvent_YouTubeMemberMilestone(t *testing.T) {
	normalizer := NewYouTubeNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "yt-msg-4",
		Platform:  "youtube",
		ChannelID: "UC1234567890",
		UserID:    "UC3333333333",
		Username:  "LoyalMember",
		Text:      "Been a member for 12 months!",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "member_milestone",
		EventData: map[string]interface{}{
			"member_months":     12,
			"member_level_name": "Gold",
		},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-yt-4")
	require.NoError(t, err)

	assert.Equal(t, "member_milestone", unified.Event.Type)
	assert.Equal(t, "high", unified.Event.Tier)
	assert.Equal(t, float64(12), unified.Event.Value.Amount)
	assert.Contains(t, unified.Event.Value.DisplayText, "12 month")
}

func TestNormalizeEvent_YouTubeMembershipGift(t *testing.T) {
	normalizer := NewYouTubeNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "yt-msg-5",
		Platform:  "youtube",
		ChannelID: "UC1234567890",
		UserID:    "UC4444444444",
		Username:  "Gifter",
		Text:      "Gifted 10 memberships",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "membership_gift",
		EventData: map[string]interface{}{
			"gift_count":        10,
			"member_level_name": "Silver",
		},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-yt-5")
	require.NoError(t, err)

	assert.Equal(t, "membership_gift", unified.Event.Type)
	assert.Equal(t, "high", unified.Event.Tier)
	assert.Equal(t, 40, unified.Event.Duration) // 10+ gifts
	assert.Equal(t, float64(10), unified.Event.Value.Amount)
	assert.Contains(t, unified.Event.Value.DisplayText, "10 gift")
}

func TestNormalizeEvent_YouTubeWrongPlatform(t *testing.T) {
	normalizer := NewYouTubeNormalizer()

	raw := &models.RawChatMessage{
		Platform:  "twitch",
		EventType: "super_chat",
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-yt-6")
	assert.Error(t, err)
	assert.Nil(t, unified)
	assert.Contains(t, err.Error(), "unsupported platform")
}
