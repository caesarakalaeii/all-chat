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

func TestNormalizeEvent_TikTokGift(t *testing.T) {
	normalizer := NewTikTokNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "tt-msg-1",
		Platform:  "tiktok",
		ChannelID: "creator123",
		UserID:    "user456",
		Username:  "TTViewer",
		Text:      "Sent Rose",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "gift",
		EventData: map[string]interface{}{
			"gift_id":       123,
			"gift_name":     "Rose",
			"gift_type":     1,
			"gift_count":    5,
			"diamond_count": 500,
		},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-tt-1")
	require.NoError(t, err)
	assert.NotNil(t, unified)

	// Verify event info
	assert.NotNil(t, unified.Event)
	assert.Equal(t, "gift", unified.Event.Type)
	assert.Equal(t, "medium", unified.Event.Tier)
	assert.Equal(t, 20, unified.Event.Duration) // 100-999 diamonds = medium

	// Verify event value
	assert.Equal(t, float64(500), unified.Event.Value.Amount)
	assert.Equal(t, "diamonds", unified.Event.Value.Currency)
	assert.Contains(t, unified.Event.Value.DisplayText, "Rose")
	assert.Contains(t, unified.Event.Value.DisplayText, "500")
}

func TestNormalizeEvent_TikTokFollow(t *testing.T) {
	normalizer := NewTikTokNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "tt-msg-2",
		Platform:  "tiktok",
		ChannelID: "creator789",
		UserID:    "user999",
		Username:  "NewFollower",
		Text:      "Followed",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "follow",
		EventData: map[string]interface{}{},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-tt-2")
	require.NoError(t, err)

	assert.Equal(t, "follow", unified.Event.Type)
	assert.Equal(t, "medium", unified.Event.Tier)
	assert.Equal(t, 15, unified.Event.Duration)
	assert.Equal(t, "New follower", unified.Event.Value.DisplayText)
}

func TestNormalizeEvent_TikTokLikeAggregate_Initial(t *testing.T) {
	normalizer := NewTikTokNormalizer()

	aggregationID := "agg-123"

	raw := &models.RawChatMessage{
		MessageID: aggregationID,
		Platform:  "tiktok",
		ChannelID: "creator456",
		UserID:    "user789",
		Username:  "LikeSpammer",
		Text:      "Sent 5 likes",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "like_aggregate",
		EventData: map[string]interface{}{
			"aggregation_id": aggregationID,
			"like_count":     5,
			"window_start":   time.Now().Format(time.RFC3339),
			"is_update":      false,
		},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-tt-3")
	require.NoError(t, err)

	assert.Equal(t, "like_aggregate", unified.Event.Type)
	assert.Equal(t, "low", unified.Event.Tier)
	assert.Equal(t, 8, unified.Event.Duration)
	assert.Equal(t, float64(5), unified.Event.Value.Amount)
	assert.Contains(t, unified.Event.Value.DisplayText, "5 likes")

	// Verify aggregation fields
	assert.Equal(t, aggregationID, unified.Event.AggregationID)
	assert.Equal(t, false, unified.Event.IsUpdate)
}

func TestNormalizeEvent_TikTokLikeAggregate_Update(t *testing.T) {
	normalizer := NewTikTokNormalizer()

	aggregationID := "agg-456"

	raw := &models.RawChatMessage{
		MessageID: aggregationID,
		Platform:  "tiktok",
		ChannelID: "creator456",
		UserID:    "user789",
		Username:  "LikeSpammer",
		Text:      "Sent 47 likes",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "like_aggregate",
		EventData: map[string]interface{}{
			"aggregation_id": aggregationID,
			"like_count":     47,
			"window_start":   time.Now().Format(time.RFC3339),
			"is_update":      true, // This is an update
		},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-tt-4")
	require.NoError(t, err)

	// Verify this is marked as update
	assert.Equal(t, true, unified.Event.IsUpdate)
	assert.Equal(t, aggregationID, unified.Event.AggregationID)
	assert.Equal(t, float64(47), unified.Event.Value.Amount)
	assert.Contains(t, unified.Event.Value.DisplayText, "47 likes")
}

func TestNormalizeEvent_TikTokLikeAggregate_Many(t *testing.T) {
	normalizer := NewTikTokNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "agg-789",
		Platform:  "tiktok",
		ChannelID: "creator",
		UserID:    "user",
		Username:  "MegaFan",
		Text:      "Sent 150 likes",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "like_aggregate",
		EventData: map[string]interface{}{
			"aggregation_id": "agg-789",
			"like_count":     150,
			"is_update":      false,
		},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-tt-5")
	require.NoError(t, err)

	// 100+ likes = medium tier
	assert.Equal(t, "medium", unified.Event.Tier)
	assert.Equal(t, 12, unified.Event.Duration)
}

func TestNormalizeEvent_TikTokShare(t *testing.T) {
	normalizer := NewTikTokNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "tt-msg-6",
		Platform:  "tiktok",
		ChannelID: "creator",
		UserID:    "user",
		Username:  "Sharer",
		Text:      "Shared the stream",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "share",
		EventData: map[string]interface{}{},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-tt-6")
	require.NoError(t, err)

	assert.Equal(t, "share", unified.Event.Type)
	assert.Equal(t, "medium", unified.Event.Tier)
	assert.Equal(t, "Shared stream", unified.Event.Value.DisplayText)
}

func TestNormalizeEvent_TikTokTreasureChest(t *testing.T) {
	normalizer := NewTikTokNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "tt-msg-7",
		Platform:  "tiktok",
		ChannelID: "creator",
		UserID:    "user",
		Username:  "ChestSender",
		Text:      "Sent a coin chest",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "treasure_chest",
		EventData: map[string]interface{}{
			"coins":    20,
			"can_open": 1,
		},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-tt-7")
	require.NoError(t, err)

	assert.Equal(t, "treasure_chest", unified.Event.Type)
	// <100 coins = low tier
	assert.Equal(t, "low", unified.Event.Tier)
	assert.Equal(t, 10, unified.Event.Duration)
	assert.Equal(t, float64(20), unified.Event.Value.Amount)
	assert.Equal(t, "coins", unified.Event.Value.Currency)
	assert.Equal(t, "20 coins", unified.Event.Value.DisplayText)

	// can_open is carried through untouched for the frontend
	assert.Equal(t, 1, unified.Event.Metadata["can_open"])
}

// Production messages arrive as JSON over Redis Streams, so every number decodes to
// float64 - the int literals used by the other tests never happen in prod.
func TestNormalizeEvent_TikTokTreasureChest_JSONFloatCoins(t *testing.T) {
	normalizer := NewTikTokNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "tt-msg-8",
		Platform:  "tiktok",
		ChannelID: "creator",
		UserID:    "user",
		Username:  "ChestSender",
		Text:      "Sent a coin chest",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "treasure_chest",
		EventData: map[string]interface{}{
			"coins":    float64(200),
			"can_open": float64(2),
		},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-tt-8")
	require.NoError(t, err)

	assert.Equal(t, float64(200), unified.Event.Value.Amount)
	assert.Equal(t, "200 coins", unified.Event.Value.DisplayText)
	// 100-999 coins = medium tier
	assert.Equal(t, "medium", unified.Event.Tier)
	assert.Equal(t, 20, unified.Event.Duration)
}

// A chest with no coin payload must still produce a value rather than a nil Event.Value.
func TestNormalizeEvent_TikTokTreasureChest_MissingCoins(t *testing.T) {
	normalizer := NewTikTokNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "tt-msg-9",
		Platform:  "tiktok",
		ChannelID: "creator",
		UserID:    "user",
		Username:  "ChestSender",
		Text:      "Sent a coin chest",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "treasure_chest",
		EventData: map[string]interface{}{},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-tt-9")
	require.NoError(t, err)

	require.NotNil(t, unified.Event.Value)
	assert.Equal(t, float64(0), unified.Event.Value.Amount)
	assert.Equal(t, "0 coins", unified.Event.Value.DisplayText)
	// Chests are discrete events, never aggregated like updates
	assert.Equal(t, "", unified.Event.AggregationID)
	assert.Equal(t, false, unified.Event.IsUpdate)
}

func TestNormalizeEvent_TikTokSubscription(t *testing.T) {
	normalizer := NewTikTokNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "tt-msg-10",
		Platform:  "tiktok",
		ChannelID: "creator",
		UserID:    "user",
		Username:  "NewSubscriber",
		Text:      "",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "tiktok_subscription",
		EventData: map[string]interface{}{
			"sub_month": 3,
		},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-tt-10")
	require.NoError(t, err)

	assert.Equal(t, "tiktok_subscription", unified.Event.Type)
	assert.Equal(t, float64(3), unified.Event.Value.Amount)
	assert.Equal(t, "months", unified.Event.Value.Currency)
	assert.Equal(t, "3 months", unified.Event.Value.DisplayText)
}

// Like the Twitch channel.subscribe payload, a subscription with unresolved
// tenure must not render a "0 months" suffix on the overlay.
func TestNormalizeEvent_TikTokSubscription_ZeroMonths(t *testing.T) {
	normalizer := NewTikTokNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "tt-msg-11",
		Platform:  "tiktok",
		ChannelID: "creator",
		UserID:    "user",
		Username:  "NewSubscriber",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "tiktok_subscription",
		EventData: map[string]interface{}{},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-tt-11")
	require.NoError(t, err)

	assert.Equal(t, "New subscriber", unified.Event.Value.DisplayText)
	assert.NotContains(t, unified.Event.Value.DisplayText, "0 month")
}

// Production messages arrive as JSON over Redis Streams, so sub_month decodes
// to float64 there.
func TestNormalizeEvent_TikTokSubscription_JSONFloatMonths(t *testing.T) {
	normalizer := NewTikTokNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "tt-msg-12",
		Platform:  "tiktok",
		ChannelID: "creator",
		UserID:    "user",
		Username:  "NewSubscriber",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "tiktok_subscription",
		EventData: map[string]interface{}{
			"sub_month": float64(12),
		},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-tt-12")
	require.NoError(t, err)

	assert.Equal(t, float64(12), unified.Event.Value.Amount)
	assert.Equal(t, "12 months", unified.Event.Value.DisplayText)
}

func TestNormalizeEvent_TikTokSuperFanJoin(t *testing.T) {
	normalizer := NewTikTokNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "tt-msg-13",
		Platform:  "tiktok",
		ChannelID: "creator",
		UserID:    "user",
		Username:  "BigFan",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "super_fan_join",
		EventData: map[string]interface{}{},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-tt-13")
	require.NoError(t, err)

	assert.Equal(t, "super_fan_join", unified.Event.Type)
	assert.Equal(t, "New Super Fan", unified.Event.Value.DisplayText)
	assert.Equal(t, "", unified.Event.AggregationID)
	assert.Equal(t, false, unified.Event.IsUpdate)
}

func TestNormalizeEvent_TikTokSuperFanBox(t *testing.T) {
	normalizer := NewTikTokNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "tt-msg-14",
		Platform:  "tiktok",
		ChannelID: "creator",
		UserID:    "user",
		Username:  "BigFan",
		Text:      "Sent a Super Fan chest",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "super_fan_box",
		EventData: map[string]interface{}{
			"coins":    150,
			"can_open": 1,
		},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-tt-14")
	require.NoError(t, err)

	assert.Equal(t, "super_fan_box", unified.Event.Type)
	assert.Equal(t, float64(150), unified.Event.Value.Amount)
	assert.Equal(t, "coins", unified.Event.Value.Currency)
	assert.Equal(t, "150 coins", unified.Event.Value.DisplayText)
	// 100-999 coins = medium tier
	assert.Equal(t, "medium", unified.Event.Tier)
	// can_open is carried through untouched for the frontend
	assert.Equal(t, 1, unified.Event.Metadata["can_open"])
}

func TestNormalizeEvent_TikTokQuestion(t *testing.T) {
	normalizer := NewTikTokNormalizer()

	raw := &models.RawChatMessage{
		MessageID: "tt-msg-15",
		Platform:  "tiktok",
		ChannelID: "creator",
		UserID:    "user",
		Username:  "CuriousViewer",
		Text:      "What is your setup?",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "question",
		EventData: map[string]interface{}{
			"question_id": "7123456789",
		},
	}

	unified, err := normalizer.NormalizeEvent(raw, "overlay-tt-15")
	require.NoError(t, err)

	assert.Equal(t, "question", unified.Event.Type)
	// The question itself is the message text; the value only labels the event.
	assert.Equal(t, "What is your setup?", unified.Message.Text)
	assert.Equal(t, "7123456789", unified.Event.Metadata["question_id"])
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		count    int
		expected string
	}{
		{0, "s"},
		{1, ""},
		{2, "s"},
		{100, "s"},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.count)), func(t *testing.T) {
			result := pluralize(tt.count)
			assert.Equal(t, tt.expected, result)
		})
	}
}
