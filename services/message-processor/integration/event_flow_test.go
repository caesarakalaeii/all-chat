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

package integration

import (
	"testing"
	"time"

	"github.com/caesar/all-chat/services/message-processor/classifier"
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/services/message-processor/normalizer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEventFlow_TwitchSubscription tests the complete flow from raw event to unified message
func TestEventFlow_TwitchSubscription(t *testing.T) {
	// Step 1: Raw event from Twitch Listener (simulates IRC USERNOTICE)
	rawEvent := &models.RawChatMessage{
		MessageID: "test-sub-1",
		Platform:  "twitch",
		ChannelID: "xqc",
		UserID:    "12345",
		Username:  "viewer123",
		Text:      "viewer123 subscribed at Tier 1. They've subscribed for 12 months!",
		Timestamp: time.Now(),
		Tags: map[string]string{
			"user-id":      "12345",
			"display-name": "Viewer123",
			"msg-id":       "sub",
		},
		EventType: "subscription",
		EventData: map[string]interface{}{
			"tier":          "1000",
			"months":        12,
			"streak_months": 6,
		},
	}

	// Step 2: Normalize event (Message Processor)
	twitchNormalizer := normalizer.NewTwitchNormalizer()
	unified, err := twitchNormalizer.NormalizeEvent(rawEvent, "overlay-test-1")
	require.NoError(t, err)
	require.NotNil(t, unified)

	// Step 3: Verify unified message structure
	assert.Equal(t, "test-sub-1", unified.ID)
	assert.Equal(t, "overlay-test-1", unified.OverlayID)
	assert.Equal(t, "twitch", unified.Platform)
	assert.Equal(t, "xqc", unified.ChannelID)

	// Step 4: Verify user info
	assert.Equal(t, "12345", unified.User.ID)
	assert.Equal(t, "viewer123", unified.User.Username)

	// Step 5: Verify event info exists
	require.NotNil(t, unified.Event)
	assert.Equal(t, "subscription", unified.Event.Type)
	assert.Equal(t, "high", unified.Event.Tier)
	assert.Equal(t, 30, unified.Event.Duration)

	// Step 6: Verify event value
	require.NotNil(t, unified.Event.Value)
	assert.Equal(t, float64(12), unified.Event.Value.Amount)
	assert.Equal(t, "months", unified.Event.Value.Currency)
	assert.Contains(t, unified.Event.Value.DisplayText, "12 months")

	// Step 7: Verify event metadata preserved
	assert.Equal(t, rawEvent.EventData, unified.Event.Metadata)

	// Step 8: Verify message doesn't have emotes (events skip emote enrichment)
	assert.Empty(t, unified.Message.Emotes)

	// Step 9: Verify JSON serialization works
	jsonBytes, err := unified.ToJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, jsonBytes)

	// Step 10: Verify deserialization
	decoded, err := models.FromJSON(jsonBytes)
	require.NoError(t, err)
	assert.Equal(t, unified.ID, decoded.ID)
	assert.NotNil(t, decoded.Event)
}

// TestEventFlow_YouTubeSuperChat tests YouTube Super Chat flow
func TestEventFlow_YouTubeSuperChat(t *testing.T) {
	rawEvent := &models.RawChatMessage{
		MessageID: "test-sc-1",
		Platform:  "youtube",
		ChannelID: "UC1234567890",
		UserID:    "UC9876543210",
		Username:  "GenerousViewer",
		Text:      "Amazing content!",
		Timestamp: time.Now(),
		Tags: map[string]string{
			"display_name": "GenerousViewer",
		},
		EventType: "super_chat",
		EventData: map[string]interface{}{
			"amount_micros":  int64(50000000), // $50
			"currency":       "USD",
			"amount_display": "$50.00",
			"tier":           int64(5),
		},
	}

	youtubeNormalizer := normalizer.NewYouTubeNormalizer()
	unified, err := youtubeNormalizer.NormalizeEvent(rawEvent, "overlay-test-2")
	require.NoError(t, err)

	assert.Equal(t, "super_chat", unified.Event.Type)
	assert.Equal(t, "high", unified.Event.Tier)
	assert.Equal(t, 60, unified.Event.Duration) // $50+ = 60s
	assert.Equal(t, "$50.00", unified.Event.Value.DisplayText)
}

// TestEventFlow_TikTokLikeAggregation tests like aggregation with updates
func TestEventFlow_TikTokLikeAggregation(t *testing.T) {
	tiktokNormalizer := normalizer.NewTikTokNormalizer()
	aggregationID := "agg-test-123"

	// First message: 5 likes
	raw1 := &models.RawChatMessage{
		MessageID: aggregationID,
		Platform:  "tiktok",
		ChannelID: "creator",
		UserID:    "user123",
		Username:  "LikeFan",
		Text:      "Sent 5 likes",
		Timestamp: time.Now(),
		EventType: "like_aggregate",
		EventData: map[string]interface{}{
			"aggregation_id": aggregationID,
			"like_count":     5,
			"is_update":      false,
		},
	}

	unified1, err := tiktokNormalizer.NormalizeEvent(raw1, "overlay-test-3")
	require.NoError(t, err)
	assert.Equal(t, false, unified1.Event.IsUpdate)
	assert.Equal(t, aggregationID, unified1.Event.AggregationID)
	assert.Equal(t, float64(5), unified1.Event.Value.Amount)

	// Update message: 47 likes total
	raw2 := &models.RawChatMessage{
		MessageID: aggregationID, // Same ID
		Platform:  "tiktok",
		ChannelID: "creator",
		UserID:    "user123",
		Username:  "LikeFan",
		Text:      "Sent 47 likes",
		Timestamp: time.Now(),
		EventType: "like_aggregate",
		EventData: map[string]interface{}{
			"aggregation_id": aggregationID, // Same aggregation ID
			"like_count":     47,            // Updated count
			"is_update":      true,          // This is an update
		},
	}

	unified2, err := tiktokNormalizer.NormalizeEvent(raw2, "overlay-test-3")
	require.NoError(t, err)
	assert.Equal(t, true, unified2.Event.IsUpdate) // Should be marked as update
	assert.Equal(t, aggregationID, unified2.Event.AggregationID)
	assert.Equal(t, aggregationID, unified2.ID) // Message ID matches aggregation ID
	assert.Equal(t, float64(47), unified2.Event.Value.Amount)
	assert.Contains(t, unified2.Event.Value.DisplayText, "47 likes")
}

// TestEventFlow_TierClassification tests tier classification for various events
func TestEventFlow_TierClassification(t *testing.T) {
	tests := []struct {
		name         string
		platform     string
		eventType    string
		value        *models.EventValue
		expectedTier string
		minDuration  int
	}{
		{
			name:         "Twitch Subscription - High",
			platform:     "twitch",
			eventType:    "subscription",
			value:        nil,
			expectedTier: "high",
			minDuration:  30,
		},
		{
			name:      "YouTube Large Super Chat - High",
			platform:  "youtube",
			eventType: "super_chat",
			value: &models.EventValue{
				Amount:   100000000, // $100
				Currency: "USD",
			},
			expectedTier: "high",
			minDuration:  60,
		},
		{
			name:      "TikTok Small Gift - Low",
			platform:  "tiktok",
			eventType: "gift",
			value: &models.EventValue{
				Amount:   50,
				Currency: "diamonds",
			},
			expectedTier: "low",
			minDuration:  10,
		},
		{
			name:         "TikTok Follow - Medium",
			platform:     "tiktok",
			eventType:    "follow",
			value:        nil,
			expectedTier: "medium",
			minDuration:  15,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier, duration := classifier.ClassifyEvent(tt.platform, tt.eventType, tt.value)
			assert.Equal(t, tt.expectedTier, tier)
			assert.GreaterOrEqual(t, duration, tt.minDuration)
		})
	}
}

// TestEventFlow_MessageVsEvent tests that events and chat messages are distinguished
func TestEventFlow_MessageVsEvent(t *testing.T) {
	twitchNormalizer := normalizer.NewTwitchNormalizer()

	// Chat message (no event fields)
	chatRaw := &models.RawChatMessage{
		MessageID: "chat-1",
		Platform:  "twitch",
		ChannelID: "xqc",
		UserID:    "12345",
		Username:  "viewer",
		Text:      "Hello chat!",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		// NO EventType or EventData
	}

	chatUnified, err := twitchNormalizer.Normalize(chatRaw, "overlay-1")
	require.NoError(t, err)
	assert.Nil(t, chatUnified.Event) // Chat message has no event

	// Event message (has event fields)
	eventRaw := &models.RawChatMessage{
		MessageID: "event-1",
		Platform:  "twitch",
		ChannelID: "xqc",
		UserID:    "12345",
		Username:  "viewer",
		Text:      "Subscribed",
		Timestamp: time.Now(),
		Tags:      map[string]string{},
		EventType: "subscription",
		EventData: map[string]interface{}{
			"tier":   "1000",
			"months": 12,
		},
	}

	eventUnified, err := twitchNormalizer.NormalizeEvent(eventRaw, "overlay-1")
	require.NoError(t, err)
	assert.NotNil(t, eventUnified.Event) // Event message has event info
	assert.Equal(t, "subscription", eventUnified.Event.Type)
}
