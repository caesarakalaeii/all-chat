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
			"tier":   "1000",
			"months": 12,
			"streak_months": 6,
			"is_gift": false,
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
