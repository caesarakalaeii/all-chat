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
	"testing"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeEvent_TwitchSubscription(t *testing.T) {
	normalizer := NewTwitchNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "test-msg-1",
		Platform:  "twitch",
		ChannelID: "xqc",
		UserID:    "12345",
		Username:  "viewer123",
		Text:      "viewer123 subscribed at Tier 1",
		Timestamp: time.Now(),
		Tags: map[string]string{
			"user-id":      "12345",
			"display-name": "Viewer123",
		},
		EventType: "subscription",
		EventData: map[string]interface{}{
			"tier":          "1000",
			"months":        12,
			"streak_months": 6,
			"is_gift":       false,
		},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-1")
	require.NoError(t, err)
	assert.NotNil(t, unified)

	// Verify event info
	assert.NotNil(t, unified.Event)
	assert.Equal(t, "subscription", unified.Event.Type)
	assert.Equal(t, "high", unified.Event.Tier)
	assert.Equal(t, 30, unified.Event.Duration)
	assert.Equal(t, false, unified.Event.IsUpdate)

	// Verify event value
	assert.NotNil(t, unified.Event.Value)
	assert.Equal(t, float64(12), unified.Event.Value.Amount)
	assert.Equal(t, "months", unified.Event.Value.Currency)
	assert.Contains(t, unified.Event.Value.DisplayText, "Tier 1")
	assert.Contains(t, unified.Event.Value.DisplayText, "12 months")

	// Verify basic fields
	assert.Equal(t, "overlay-1", unified.OverlayID)
	assert.Equal(t, "twitch", unified.Platform)
	assert.Equal(t, "xqc", unified.ChannelID)
}

// TestNormalizeEvent_TwitchSubscriptionWithoutMonths covers the EventSub
// `channel.subscribe` payload, which has no cumulative_months field. Before the
// fix in #254 the normalizer formatted DisplayText as "Tier 1 - 0 months", and
// every initial sub overlay entry showed "0 months" tenure for users who had
// in fact been subscribed for years. The duration suffix must be omitted.
func TestNormalizeEvent_TwitchSubscriptionWithoutMonths(t *testing.T) {
	normalizer := NewTwitchNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "test-msg-no-months",
		Platform:  "twitch",
		ChannelID: "xqc",
		UserID:    "12345",
		Username:  "viewer123",
		Text:      "Subscribed at 1000",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "subscription",
		EventData: map[string]interface{}{
			"tier":      "1000",
			"is_gift":   false,
			"plan_name": "1000",
		},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-no-months")
	require.NoError(t, err)
	require.NotNil(t, unified)
	require.NotNil(t, unified.Event.Value)

	assert.Contains(t, unified.Event.Value.DisplayText, "Tier 1")
	assert.NotContains(t, unified.Event.Value.DisplayText, "months",
		"channel.subscribe carries no months data — must not render a 0-month suffix")
}

func TestNormalizeEvent_TwitchGiftSubscription(t *testing.T) {
	normalizer := NewTwitchNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "test-msg-2",
		Platform:  "twitch",
		ChannelID: "shroud",
		UserID:    "99999",
		Username:  "gifter123",
		Text:      "gifter123 gifted a Tier 2 sub to recipient456!",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "gift_subscription",
		EventData: map[string]interface{}{
			"tier":           "2000",
			"recipient_id":   "88888",
			"recipient_name": "recipient456",
			"months":         1,
			"is_gift":        true,
		},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-2")
	require.NoError(t, err)

	assert.Equal(t, "gift_subscription", unified.Event.Type)
	assert.Equal(t, "high", unified.Event.Tier)
	assert.Contains(t, unified.Event.Value.DisplayText, "Tier 2")
	assert.Contains(t, unified.Event.Value.DisplayText, "recipient456")
}

func TestNormalizeEvent_TwitchRaid(t *testing.T) {
	normalizer := NewTwitchNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "test-msg-3",
		Platform:  "twitch",
		ChannelID: "pokimane",
		UserID:    "77777",
		Username:  "raider_channel",
		Text:      "5000 raiders from raider_channel have joined!",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "raid",
		EventData: map[string]interface{}{
			"raider_channel": "raider_channel",
			"viewer_count":   5000,
		},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-3")
	require.NoError(t, err)

	assert.Equal(t, "raid", unified.Event.Type)
	assert.Equal(t, "high", unified.Event.Tier)
	assert.Equal(t, 40, unified.Event.Duration) // Large raid
	assert.Equal(t, float64(5000), unified.Event.Value.Amount)
	assert.Equal(t, "viewers", unified.Event.Value.Currency)
}

func TestNormalizeEvent_TwitchChannelPoints(t *testing.T) {
	normalizer := NewTwitchNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "test-msg-4",
		Platform:  "twitch",
		ChannelID: "lirik",
		UserID:    "33333",
		Username:  "viewer789",
		Text:      "Redeemed Hydrate",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "channel_points",
		EventData: map[string]interface{}{
			"reward_id":    "reward-uuid",
			"reward_title": "Hydrate",
			"reward_cost":  500,
			"user_input":   "Time to drink water!",
		},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-4")
	require.NoError(t, err)

	assert.Equal(t, "channel_points", unified.Event.Type)
	assert.Equal(t, "medium", unified.Event.Tier)
	assert.Equal(t, float64(500), unified.Event.Value.Amount)
	assert.Contains(t, unified.Event.Value.DisplayText, "Hydrate")
	assert.Contains(t, unified.Event.Value.DisplayText, "500 points")
}

func TestNormalizeEvent_WrongPlatform(t *testing.T) {
	normalizer := NewTwitchNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "test-msg-5",
		Platform:  "youtube", // Wrong platform
		EventType: "subscription",
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-5")
	assert.Error(t, err)
	assert.Nil(t, unified)
	assert.Contains(t, err.Error(), "unsupported platform")
}

func TestNormalizeEvent_MissingEventType(t *testing.T) {
	normalizer := NewTwitchNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "test-msg-6",
		Platform:  "twitch",
		EventType: "", // Missing event type
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-6")
	assert.Error(t, err)
	assert.Nil(t, unified)
	assert.Contains(t, err.Error(), "missing event type")
}

// TestNormalizeEvent_TwitchWatchStreak covers the bug this path was added for: the watch-streak
// notice carries the viewer's own chat message, which must survive normalization as message text
// (with its Twitch emotes) alongside the streak decoration — see ADR-0046.
func TestNormalizeEvent_TwitchWatchStreak(t *testing.T) {
	normalizer := NewTwitchNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "test-msg-streak",
		Platform:  "twitch",
		ChannelID: "streamer",
		UserID:    "999",
		Username:  "viewer",
		Text:      "hey Kappa",
		Timestamp: time.Now(),
		Tags: map[string]string{
			"user-id":      "999",
			"display-name": "Viewer",
			"emotes":       "25:4-8",
		},
		EventType: "watch_streak",
		EventData: map[string]interface{}{
			"streak_count":           5,
			"channel_points_awarded": 350,
		},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-1")
	require.NoError(t, err)
	require.NotNil(t, unified)

	assert.Equal(t, "watch_streak", unified.Event.Type)
	assert.Equal(t, "hey Kappa", unified.Message.Text, "the viewer's message must not be dropped")
	require.Len(t, unified.Message.Emotes, 1, "native emotes in the notice message must be extracted")
	assert.Equal(t, "Kappa", unified.Message.Emotes[0].Code)
	assert.Equal(t, "twitch", unified.Message.Emotes[0].Provider)

	require.NotNil(t, unified.Event.Value)
	assert.Equal(t, float64(5), unified.Event.Value.Amount)
	assert.Equal(t, "streams", unified.Event.Value.Currency)
	assert.Equal(t, "5-stream watch streak", unified.Event.Value.DisplayText)
	assert.Equal(t, 350, unified.Event.Metadata["channel_points_awarded"])
}

// EventData survives a JSON round-trip through the chat:raw stream, so ints arrive as float64.
func TestNormalizeEvent_TwitchWatchStreakFromJSONNumbers(t *testing.T) {
	normalizer := NewTwitchNormalizer()

	unified, err := normalizer.NormalizeEvent(&models.RawChatMessage{
		MessageID: "test-msg-streak-json",
		Platform:  "twitch",
		ChannelID: "streamer",
		Username:  "viewer",
		Text:      "hello",
		Timestamp: time.Now(),
		EventType: "watch_streak",
		EventData: map[string]interface{}{"streak_count": float64(9)},
	}, "overlay-1")
	require.NoError(t, err)
	require.NotNil(t, unified.Event.Value)
	assert.Equal(t, "9-stream watch streak", unified.Event.Value.DisplayText)
}

func TestNormalizeEvent_TwitchAnnouncementHasNoValuePill(t *testing.T) {
	normalizer := NewTwitchNormalizer()

	unified, err := normalizer.NormalizeEvent(&models.RawChatMessage{
		MessageID: "test-msg-announce",
		Platform:  "twitch",
		ChannelID: "streamer",
		Username:  "mod_user",
		Text:      "Raid at the end!",
		Timestamp: time.Now(),
		EventType: "announcement",
		EventData: map[string]interface{}{"announcement_color": "PURPLE"},
	}, "overlay-1")
	require.NoError(t, err)

	assert.Equal(t, "announcement", unified.Event.Type)
	assert.Equal(t, "Raid at the end!", unified.Message.Text)
	assert.Nil(t, unified.Event.Value, "an announcement has nothing to quantify")
	assert.Equal(t, "PURPLE", unified.Event.Metadata["announcement_color"])
}

func TestNormalizeEvent_TwitchCharityDonationAmount(t *testing.T) {
	normalizer := NewTwitchNormalizer()

	unified, err := normalizer.NormalizeEvent(&models.RawChatMessage{
		MessageID: "test-msg-charity",
		Platform:  "twitch",
		ChannelID: "streamer",
		Username:  "viewer",
		Text:      "viewer donated 12.34 EUR to Some Charity",
		Timestamp: time.Now(),
		EventType: "charity_donation",
		EventData: map[string]interface{}{
			"charity_name":          "Some Charity",
			"amount_value":          float64(1234),
			"amount_decimal_places": float64(2),
			"amount_currency":       "EUR",
		},
	}, "overlay-1")
	require.NoError(t, err)

	require.NotNil(t, unified.Event.Value)
	assert.InDelta(t, 12.34, unified.Event.Value.Amount, 0.001)
	assert.Equal(t, "EUR", unified.Event.Value.Currency)
	assert.Equal(t, "12.34 EUR", unified.Event.Value.DisplayText)
}

func TestNormalizeEvent_TwitchNoticeUpgradesAndModiversary(t *testing.T) {
	normalizer := NewTwitchNormalizer()

	tests := []struct {
		name      string
		eventType string
		data      map[string]interface{}
		want      string
	}{
		{
			name:      "prime paid upgrade names the new tier",
			eventType: "prime_paid_upgrade",
			data:      map[string]interface{}{"tier": "1000"},
			want:      "Prime → Tier 1",
		},
		{
			name:      "gift paid upgrade credits the original gifter",
			eventType: "gift_paid_upgrade",
			data:      map[string]interface{}{"gifter_name": "GenerousGifter"},
			want:      "Continuing the gift sub from GenerousGifter",
		},
		{
			name:      "gift paid upgrade without a known gifter",
			eventType: "gift_paid_upgrade",
			data:      map[string]interface{}{"gifter_is_anonymous": true},
			want:      "Continuing their gift sub",
		},
		{
			name:      "pay it forward credits the original gifter",
			eventType: "pay_it_forward",
			data:      map[string]interface{}{"gifter_name": "GenerousGifter"},
			want:      "Paying forward GenerousGifter's gift",
		},
		{
			name:      "modiversary counts months",
			eventType: "modiversary",
			data:      map[string]interface{}{"months": 24},
			want:      "24 months as moderator",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unified, err := normalizer.NormalizeEvent(&models.RawChatMessage{
				MessageID: "test-msg",
				Platform:  "twitch",
				ChannelID: "streamer",
				Username:  "viewer",
				Timestamp: time.Now(),
				EventType: tt.eventType,
				EventData: tt.data,
			}, "overlay-1")
			require.NoError(t, err)
			require.NotNil(t, unified.Event.Value)
			assert.Equal(t, tt.want, unified.Event.Value.DisplayText)
		})
	}
}

func TestGetTierName(t *testing.T) {
	tests := []struct {
		tier     string
		expected string
	}{
		{"1000", "Tier 1"},
		{"2000", "Tier 2"},
		{"3000", "Tier 3"},
		{"Prime", "Prime"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			result := getTierName(tt.tier)
			assert.Equal(t, tt.expected, result)
		})
	}
}
