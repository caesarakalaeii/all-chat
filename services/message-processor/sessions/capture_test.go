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

package sessions

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTestCapture(t *testing.T) (*EventCapture, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	logger := zap.NewNop()
	return NewEventCapture(rdb, logger), mr
}

func makeActiveSession(t *testing.T, mr *miniredis.Miniredis, overlayID, sessionID string) {
	t.Helper()
	key := SessionKeyPrefix + overlayID
	mr.HSet(key, "session_id", sessionID)
	mr.HSet(key, "state", "ACTIVE")
	mr.HSet(key, "started_at", time.Now().UTC().Format(time.RFC3339))
	mr.HSet(key, "event_count", "0")
}

func makeSubEvent(overlayID, userID, displayName, avatarURL string, amount float64) *models.UnifiedChatMessage {
	return &models.UnifiedChatMessage{
		ID:        fmt.Sprintf("msg-%s-%d", userID, time.Now().UnixNano()),
		OverlayID: overlayID,
		Platform:  "twitch",
		User: models.UserInfo{
			ID:          userID,
			DisplayName: displayName,
			AvatarURL:   avatarURL,
		},
		Timestamp: time.Now(),
		Event: &models.EventInfo{
			Type: "subscription",
			Value: &models.EventValue{
				Amount:      amount,
				Currency:    "USD",
				DisplayText: fmt.Sprintf("$%.2f", amount),
			},
		},
	}
}

func TestCaptureIfActive_NoEvent(t *testing.T) {
	ec, _ := setupTestCapture(t)
	msg := &models.UnifiedChatMessage{
		OverlayID: "overlay-1",
		Event:     nil,
	}
	err := ec.CaptureIfActive(context.Background(), msg)
	assert.NoError(t, err)
}

func TestCaptureIfActive_NoSession(t *testing.T) {
	ec, _ := setupTestCapture(t)
	msg := makeSubEvent("overlay-1", "user-1", "Alice", "https://avatar/1", 4.99)
	err := ec.CaptureIfActive(context.Background(), msg)
	assert.NoError(t, err)
}

func TestStoreEvent_DeduplicatesSameUser(t *testing.T) {
	ec, mr := setupTestCapture(t)
	ctx := context.Background()
	overlayID := "overlay-dedup"
	sessionID := "session-dedup"
	makeActiveSession(t, mr, overlayID, sessionID)

	// Same user subscribes twice with different avatar URLs
	msg1 := makeSubEvent(overlayID, "user-42", "Alice", "https://avatar/old.png", 4.99)
	msg2 := makeSubEvent(overlayID, "user-42", "Alice_Updated", "https://avatar/new.png", 9.99)

	require.NoError(t, ec.CaptureIfActive(ctx, msg1))
	require.NoError(t, ec.CaptureIfActive(ctx, msg2))

	// Should have exactly 1 entry in the subs leaderboard (not 2)
	leaderboardKey := fmt.Sprintf("%s%s:subs", LeaderboardKeyPrefix, sessionID)
	members, err := mr.ZMembers(leaderboardKey)
	require.NoError(t, err)
	assert.Len(t, members, 1, "same user should have exactly one entry")

	// Score should be aggregated (4.99 + 9.99 = 14.98)
	score, err := mr.ZScore(leaderboardKey, members[0])
	require.NoError(t, err)
	assert.InDelta(t, 14.98, score, 0.01, "scores should be aggregated")

	// Metadata should reflect latest event (updated display name and avatar)
	metaKey := fmt.Sprintf("%s%s:subs", MetadataKeyPrefix, sessionID)
	metaJSON := mr.HGet(metaKey, members[0])
	require.NotEmpty(t, metaJSON)

	var meta map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(metaJSON), &meta))
	assert.Equal(t, "Alice_Updated", meta["display_name"], "metadata should reflect latest event")
	assert.Equal(t, "https://avatar/new.png", meta["avatar_url"], "avatar should be updated")
}

func TestStoreEvent_DifferentUsersSeparateEntries(t *testing.T) {
	ec, mr := setupTestCapture(t)
	ctx := context.Background()
	overlayID := "overlay-multi"
	sessionID := "session-multi"
	makeActiveSession(t, mr, overlayID, sessionID)

	msg1 := makeSubEvent(overlayID, "user-1", "Alice", "https://avatar/alice.png", 4.99)
	msg2 := makeSubEvent(overlayID, "user-2", "Bob", "https://avatar/bob.png", 9.99)

	require.NoError(t, ec.CaptureIfActive(ctx, msg1))
	require.NoError(t, ec.CaptureIfActive(ctx, msg2))

	leaderboardKey := fmt.Sprintf("%s%s:subs", LeaderboardKeyPrefix, sessionID)
	members, err := mr.ZMembers(leaderboardKey)
	require.NoError(t, err)
	assert.Len(t, members, 2, "different users should have separate entries")
}

func TestStoreEvent_MemberKeyFormat(t *testing.T) {
	ec, mr := setupTestCapture(t)
	ctx := context.Background()
	overlayID := "overlay-key"
	sessionID := "session-key"
	makeActiveSession(t, mr, overlayID, sessionID)

	msg := makeSubEvent(overlayID, "user-99", "TestUser", "https://avatar/test.png", 4.99)
	require.NoError(t, ec.CaptureIfActive(ctx, msg))

	leaderboardKey := fmt.Sprintf("%s%s:subs", LeaderboardKeyPrefix, sessionID)
	members, err := mr.ZMembers(leaderboardKey)
	require.NoError(t, err)
	require.Len(t, members, 1)

	// Member key should be platform:user_id (not JSON)
	assert.Equal(t, "twitch:user-99", members[0], "member key should be platform:user_id")
}

func TestStoreEvent_CountBasedEvents(t *testing.T) {
	ec, mr := setupTestCapture(t)
	ctx := context.Background()
	overlayID := "overlay-follow"
	sessionID := "session-follow"
	makeActiveSession(t, mr, overlayID, sessionID)

	msg := &models.UnifiedChatMessage{
		ID:        "msg-follow-1",
		OverlayID: overlayID,
		Platform:  "twitch",
		User: models.UserInfo{
			ID:          "user-follower",
			DisplayName: "Follower",
		},
		Timestamp: time.Now(),
		Event: &models.EventInfo{
			Type: "follow",
		},
	}

	require.NoError(t, ec.CaptureIfActive(ctx, msg))

	leaderboardKey := fmt.Sprintf("%s%s:follows", LeaderboardKeyPrefix, sessionID)
	members, err := mr.ZMembers(leaderboardKey)
	require.NoError(t, err)
	require.Len(t, members, 1)

	score, err := mr.ZScore(leaderboardKey, members[0])
	require.NoError(t, err)
	assert.Equal(t, 1.0, score, "follow event should have score 1.0")
}

func TestStoreEvent_SessionCounterIncremented(t *testing.T) {
	ec, mr := setupTestCapture(t)
	ctx := context.Background()
	overlayID := "overlay-counter"
	sessionID := "session-counter"
	makeActiveSession(t, mr, overlayID, sessionID)

	msg := makeSubEvent(overlayID, "user-1", "Alice", "", 4.99)
	require.NoError(t, ec.CaptureIfActive(ctx, msg))

	sessionKey := SessionKeyPrefix + overlayID
	count := mr.HGet(sessionKey, "event_count")
	assert.Equal(t, "1", count)

	// Second event
	msg2 := makeSubEvent(overlayID, "user-2", "Bob", "", 9.99)
	require.NoError(t, ec.CaptureIfActive(ctx, msg2))

	count = mr.HGet(sessionKey, "event_count")
	assert.Equal(t, "2", count)
}

// makeEvent is a generic event message builder; scores and categories are
// asserted per test below.
func makeEvent(eventType string, value *models.EventValue, metadata map[string]interface{}) *models.UnifiedChatMessage {
	return &models.UnifiedChatMessage{
		ID:        fmt.Sprintf("msg-%s-%d", eventType, time.Now().UnixNano()),
		OverlayID: "overlay-events",
		Platform:  "tiktok",
		User: models.UserInfo{
			ID:          "user-tt",
			DisplayName: "TikTok User",
		},
		Timestamp: time.Now(),
		Event: &models.EventInfo{
			Type:     eventType,
			Value:    value,
			Metadata: metadata,
		},
	}
}

func TestStoreEvent_NewCategories(t *testing.T) {
	tests := []struct {
		name      string
		event     *models.UnifiedChatMessage
		category  string
		wantScore float64
	}{
		{
			name: "tiktok_subscription lands in subs",
			event: makeEvent("tiktok_subscription", &models.EventValue{
				Amount: 3, Currency: "months", DisplayText: "3 months",
			}, nil),
			category:  "subs",
			wantScore: 3,
		},
		{
			name:      "super_fan_join lands in memberships and scores 1",
			event:     makeEvent("super_fan_join", &models.EventValue{Amount: 1, Currency: "super_fan"}, nil),
			category:  "memberships",
			wantScore: 1,
		},
		{
			name: "super_fan_box lands in treasure_chests",
			event: makeEvent("super_fan_box", &models.EventValue{
				Amount: 150, Currency: "coins", DisplayText: "150 coins",
			}, nil),
			category:  "treasure_chests",
			wantScore: 150,
		},
		{
			name:      "share lands in shares",
			event:     makeEvent("share", &models.EventValue{Amount: 1, Currency: "share"}, nil),
			category:  "shares",
			wantScore: 1,
		},
		{
			name: "power_up lands in bits",
			event: makeEvent("power_up", &models.EventValue{
				Amount: 500, Currency: "bits", DisplayText: "On-Screen Celebration",
			}, nil),
			category:  "bits",
			wantScore: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec, mr := setupTestCapture(t)
			ctx := context.Background()
			const sessionID = "session-new-cats"
			makeActiveSession(t, mr, "overlay-events", sessionID)

			require.NoError(t, ec.CaptureIfActive(ctx, tt.event))

			key := fmt.Sprintf("%s%s:%s", LeaderboardKeyPrefix, sessionID, tt.category)
			members, err := mr.ZMembers(key)
			require.NoError(t, err)
			require.Len(t, members, 1)

			score, err := mr.ZScore(key, members[0])
			require.NoError(t, err)
			assert.Equal(t, tt.wantScore, score)
		})
	}
}

// like_aggregate publishes a running like_count every 5s; only the frame that
// closes the window (window_end set) carries the true total, so intermediate
// frames must not reach the likes leaderboard.
func TestCaptureIfActive_LikeAggregateFinalFrameOnly(t *testing.T) {
	ec, mr := setupTestCapture(t)
	ctx := context.Background()
	overlayID, sessionID := "overlay-events", "session-likes"
	makeActiveSession(t, mr, overlayID, sessionID)

	mid := makeEvent("like_aggregate", &models.EventValue{
		Amount: 5, Currency: "likes", DisplayText: "5 likes",
	}, map[string]interface{}{"aggregation_id": "agg-1", "window_end": nil, "is_update": false})
	assert.NoError(t, ec.CaptureIfActive(ctx, mid))

	final := makeEvent("like_aggregate", &models.EventValue{
		Amount: 30, Currency: "likes", DisplayText: "30 likes",
	}, map[string]interface{}{"aggregation_id": "agg-1", "window_end": "2026-10-15T00:00:00Z", "is_update": true})
	assert.NoError(t, ec.CaptureIfActive(ctx, final))

	key := fmt.Sprintf("%s%s:likes", LeaderboardKeyPrefix, sessionID)
	score, _ := mr.ZScore(key, "tiktok:user-tt")
	assert.Equal(t, float64(30), score)
	members, _ := mr.ZMembers(key)
	require.Len(t, members, 1, "only the window-final frame may be captured")
}

// Top Gifters ranks by estimated USD spend, but the displayed total stays the
// native count in the accompanying raw hash.
func TestStoreEvent_WeightedGiftScoreKeepsRawTotal(t *testing.T) {
	ec, mr := setupTestCapture(t)
	ctx := context.Background()
	overlayID, sessionID := "overlay-gifts", "session-gifts"
	makeActiveSession(t, mr, overlayID, sessionID)

	// A 5-gift Tier 1 bomb scores 5 x 4.99 in the sorted set.
	bomb := &models.UnifiedChatMessage{
		ID:        "msg-bomb",
		OverlayID: overlayID,
		Platform:  "twitch",
		User:      models.UserInfo{ID: "gifter", DisplayName: "Gifter"},
		Timestamp: time.Now(),
		Event: &models.EventInfo{
			Type:     "mystery_gift",
			Value:    &models.EventValue{Amount: 5, Currency: "gifts", DisplayText: "5 gift subs"},
			Metadata: map[string]interface{}{"tier": "1000"},
		},
	}
	require.NoError(t, ec.CaptureIfActive(ctx, bomb))

	key := fmt.Sprintf("%s%s:gifts", LeaderboardKeyPrefix, sessionID)
	score, err := mr.ZScore(key, "twitch:gifter")
	require.NoError(t, err)
	assert.InDelta(t, 5*4.99, score, 0.0001)

	rawKey := fmt.Sprintf("%s%s:gifts", RawTotalKeyPrefix, sessionID)
	raw := mr.HGet(rawKey, "twitch:gifter")
	assert.Equal(t, "5", raw, "displayed total stays the native gift count")
}

func TestWeightedGiftScore_TiersAndPriorities(t *testing.T) {
	assert.Equal(t, 24.99*3, weightedGiftScore(&models.EventInfo{
		Type:     "mystery_gift",
		Value:    mpEventValuePtr(3),
		Metadata: map[string]interface{}{"tier": "3000"},
	}))
	// Unknown tier falls back to Tier 1 rather than scoring 0.
	assert.Equal(t, 4.99*2, weightedGiftScore(&models.EventInfo{
		Type:     "mystery_gift",
		Value:    mpEventValuePtr(2),
		Metadata: map[string]interface{}{},
	}))
	// YouTube membership gift uses the flat stand-in price.
	assert.Equal(t, 4.99*2, weightedGiftScore(&models.EventInfo{
		Type:  "membership_gift",
		Value: mpEventValuePtr(2),
	}))
	// TikTok diamonds are priced as the coins a viewer bought, so 300 diamonds
	// (about 3.90 USD) rank below a single 4.99 gift sub instead of far above it.
	diamonds := weightedGiftScore(&models.EventInfo{
		Type:  "gift",
		Value: &models.EventValue{Amount: 300, Currency: "diamonds"},
	})
	assert.InDelta(t, 300*0.013, diamonds, 0.0001)
	assert.Less(t, diamonds, giftSubPriceUSD["1000"])
}

func mpEventValue(amount int) models.EventValue {
	return models.EventValue{Amount: float64(amount), Currency: "gifts"}
}

func mpEventValuePtr(amount int) *models.EventValue {
	v := mpEventValue(amount)
	return &v
}
