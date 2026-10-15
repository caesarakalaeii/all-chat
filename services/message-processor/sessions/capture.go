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
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// SessionKeyPrefix is the Redis key prefix for active sessions
	SessionKeyPrefix = "session:active:"

	// LeaderboardKeyPrefix is the Redis key prefix for session leaderboards
	LeaderboardKeyPrefix = "session:leaderboard:"

	// EventKeyPrefix is the Redis key prefix for detailed event storage
	EventKeyPrefix = "session:event:"

	// LeaderboardTTL is how long to keep leaderboard data
	LeaderboardTTL = 48 * time.Hour
)

// EventCapture handles capturing events for active sessions
type EventCapture struct {
	redis  *redis.Client
	logger *zap.Logger
}

// NewEventCapture creates a new event capture handler
func NewEventCapture(redis *redis.Client, logger *zap.Logger) *EventCapture {
	return &EventCapture{
		redis:  redis,
		logger: logger,
	}
}

// CaptureIfActive captures event if session is active for this overlay
func (ec *EventCapture) CaptureIfActive(ctx context.Context, msg *models.UnifiedChatMessage) error {
	// Only capture if this is an event
	if msg.Event == nil {
		return nil
	}

	// Check if event should be captured (type allow-list plus per-message gate)
	if !ec.shouldCaptureEvent(msg) || !ec.gate(msg) {
		return nil
	}

	// Check if session is active
	sessionKey := SessionKeyPrefix + msg.OverlayID
	state, err := ec.redis.HGet(ctx, sessionKey, "state").Result()
	if err == redis.Nil {
		// No active session
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check session state: %w", err)
	}

	// Only capture for ACTIVE or ENDING sessions (not COMPLETED)
	if state != "ACTIVE" && state != "ENDING" {
		return nil
	}

	// Get session_id
	sessionID, err := ec.redis.HGet(ctx, sessionKey, "session_id").Result()
	if err == redis.Nil {
		// Partial/corrupted session hash: state field exists but session_id missing.
		// Treat as no active session rather than propagating a noisy error.
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get session_id: %w", err)
	}

	// Store event in leaderboard
	return ec.storeEvent(ctx, sessionID, msg)
}

// shouldCaptureEvent determines if event should be captured for credit roll
func (ec *EventCapture) shouldCaptureEvent(msg *models.UnifiedChatMessage) bool {
	if msg.Event == nil {
		return false
	}

	// Capture monetary and engagement events only. TikTok super_fan_box is the
	// envelope-flavoured treasure chest (the listener guarantees it is never also
	// published as treasure_chest), so it joins that category. like_aggregate is
	// further gated per-message: only the window-final frame is captured (gate).
	captureTypes := map[string]bool{
		"subscription":        true,
		"resubscription":      true,
		"gift_subscription":   true,
		"mystery_gift":        true,
		"bits":                true,
		"power_up":            true, // Bits Power-ups: bits spent (PR #1042)
		"raid":                true,
		"super_chat":          true,
		"super_sticker":       true,
		"new_sponsor":         true,
		"member_milestone":    true,
		"membership_gift":     true,
		"follow":              true,
		"gift":                true, // TikTok/Kick
		"channel_points":      true,
		"tiktok_subscription": true,
		"super_fan_join":      true,
		"super_fan_box":       true,
		"treasure_chest":      true,
		"share":               true,
		"like_aggregate":      true,
	}

	return captureTypes[msg.Event.Type]
}

// getEventCategory maps event types to leaderboard categories
func (ec *EventCapture) getEventCategory(eventType string) string {
	categories := map[string]string{
		"subscription":        "subs",
		"resubscription":      "subs",
		"gift_subscription":   "gifts",
		"mystery_gift":        "gifts",
		"bits":                "bits",
		"power_up":            "bits",
		"raid":                "raids",
		"super_chat":          "super_chats",
		"super_sticker":       "super_chats",
		"new_sponsor":         "memberships",
		"member_milestone":    "memberships",
		"super_fan_join":      "memberships",
		"membership_gift":     "gifts",
		"follow":              "follows",
		"gift":                "gifts", // TikTok/Kick
		"channel_points":      "points",
		"tiktok_subscription": "subs",
		"super_fan_box":       "treasure_chests",
		"treasure_chest":      "treasure_chests",
		"share":               "shares",
		"like_aggregate":      "likes",
	}

	if category, ok := categories[eventType]; ok {
		return category
	}
	return "other"
}

// calculateScore determines the SCORE for leaderboard sorting. Gift events are
// scored as estimated USD of viewer spend so a Twitch gift bomb, a YouTube
// membership gift and TikTok diamonds compete on one axis (giftSubPriceUSD
// documents the pricing); every other category scores its native amount
// (1 per event when there is no value). The DISPLAYED total stays in the native
// unit: see storeEvent and RawTotalKeyPrefix.
func (ec *EventCapture) calculateScore(event *models.EventInfo) float64 {
	if ec.getEventCategory(event.Type) == "gifts" {
		return weightedGiftScore(event)
	}
	if event.Value == nil {
		return 1.0 // Count-based (1 per event)
	}
	return event.Value.Amount
}

// giftSubPriceUSD is the documented Twitch gift-sub price table, keyed by the
// raw plan in the event metadata ("1000"/"2000"/"3000" plus "Prime"). These are
// Twitch's per-sub USD prices; a mystery gift of N subs scores N times the
// price of its plan, so gift bombs compete against TikTok diamonds on spend.
var giftSubPriceUSD = map[string]float64{
	"1000":  4.99,
	"Prime": 4.99,
	"2000":  9.99,
	"3000":  24.99,
}

// giftMembershipPriceUSD stands in for the YouTube membership gift price. The
// payload carries only a count, never a price, so this approximation leans on
// the lowest membership tier; the actual price is unknowable here.
const giftMembershipPriceUSD = 4.99

// diamondPriceUSD is what a viewer pays for the coins behind one TikTok diamond:
// a gift worth N coins credits the streamer N diamonds, and coins sell for
// roughly 1.1 to 1.5 US cents depending on pack size and region. APPROXIMATE.
const diamondPriceUSD = 0.013

// weightedGiftScore prices a gifts-category event in estimated USD of viewer
// spend. Unknown sub tiers fall back to the Tier-1 price rather than scoring 0.
func weightedGiftScore(event *models.EventInfo) float64 {
	count := 1.0
	if event.Value != nil && event.Value.Amount > 0 {
		count = event.Value.Amount
	}

	switch {
	case event.Type == "gift_subscription" || event.Type == "mystery_gift":
		tier, _ := event.Metadata["tier"].(string)
		price, ok := giftSubPriceUSD[tier]
		if !ok {
			price = giftSubPriceUSD["1000"]
		}
		return count * price
	case event.Type == "membership_gift":
		// APPROXIMATE: the payload has no price to read.
		return count * giftMembershipPriceUSD
	case event.Value != nil && event.Value.Currency == "diamonds":
		return event.Value.Amount * diamondPriceUSD
	default:
		// A gift with no known price (e.g. a platform added later) still ranks, by count.
		return count
	}
}

// likeWindowFinal reports whether a like_aggregate message closes its 30-second
// aggregation window. The listener publishes a running like_count every 5s and
// marks only the LAST publish of a window with a window_end timestamp; an
// earlier frame carries a window that is still open, so capturing every frame
// would over-count the likes a viewer sent.
func likeWindowFinal(metadata map[string]interface{}) bool {
	end, ok := metadata["window_end"].(string)
	return ok && end != ""
}

// gate is the per-message capture condition on top of the type allow-list.
// Currently only like_aggregate needs it: intermediate frames of its rolling
// like window must not reach the leaderboards.
func (ec *EventCapture) gate(msg *models.UnifiedChatMessage) bool {
	if msg.Event.Type == "like_aggregate" {
		return likeWindowFinal(msg.Event.Metadata)
	}
	return true
}

// MetadataKeyPrefix is the Redis key prefix for leaderboard member metadata
const MetadataKeyPrefix = "session:leaderboard:meta:"

// RawTotalKeyPrefix is the Redis key prefix for per-member native-unit totals
// (HINCRBYFLOAT) whose sorted-set score measures something else (weighted USD
// for gifts). The credit roll displays the raw total and sorts by the score.
const RawTotalKeyPrefix = "session:leaderboard:raw:"

// storeEvent stores event in appropriate Redis leaderboard
func (ec *EventCapture) storeEvent(ctx context.Context, sessionID string, msg *models.UnifiedChatMessage) error {
	pipe := ec.redis.Pipeline()

	// Determine category and leaderboard key
	category := ec.getEventCategory(msg.Event.Type)
	leaderboardKey := fmt.Sprintf("%s%s:%s", LeaderboardKeyPrefix, sessionID, category)

	// Use stable key for sorted set member (platform:user_id) so ZINCRBY
	// correctly aggregates multiple events from the same user
	memberKey := fmt.Sprintf("%s:%s", msg.Platform, msg.User.ID)

	// Store volatile metadata (display name, avatar, etc.) in companion hash
	// so it stays up-to-date without creating duplicate sorted set entries
	metadataKey := fmt.Sprintf("%s%s:%s", MetadataKeyPrefix, sessionID, category)
	meta := map[string]interface{}{
		"user_id":      msg.User.ID,
		"display_name": msg.User.DisplayName,
		"avatar_url":   msg.User.AvatarURL,
		"platform":     msg.Platform,
		"event_type":   msg.Event.Type,
	}

	if msg.Event.Value != nil {
		meta["currency"] = msg.Event.Value.Currency
		meta["display_text"] = msg.Event.Value.DisplayText
	}

	if msg.Event.Metadata != nil {
		if tier, ok := msg.Event.Metadata["tier"].(string); ok {
			meta["tier"] = tier
		}
		if months, ok := msg.Event.Metadata["months"].(int); ok {
			meta["months"] = months
		}
		if viewerCount, ok := msg.Event.Metadata["viewer_count"].(int); ok {
			meta["viewer_count"] = viewerCount
		}
		if pointsName, ok := msg.Event.Metadata["points_name"].(string); ok {
			meta["points_name"] = pointsName
		}
	}

	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal member metadata: %w", err)
	}

	// ZINCRBY with stable key — aggregates score across multiple events per user.
	// For gifts the score is weighted USD, so the DISPLAYED total is tracked
	// separately in the native unit (gifts) in a companion raw-total hash.
	score := ec.calculateScore(msg.Event)
	pipe.ZIncrBy(ctx, leaderboardKey, score, memberKey)
	pipe.Expire(ctx, leaderboardKey, LeaderboardTTL)

	if nativeDelta := ec.nativeTotalDelta(msg.Event); nativeDelta != 0 {
		rawKey := fmt.Sprintf("%s%s:%s", RawTotalKeyPrefix, sessionID, category)
		pipe.HIncrByFloat(ctx, rawKey, memberKey, nativeDelta)
		pipe.Expire(ctx, rawKey, LeaderboardTTL)
	}

	// Update metadata hash with latest info for this user
	pipe.HSet(ctx, metadataKey, memberKey, string(metaJSON))
	pipe.Expire(ctx, metadataKey, LeaderboardTTL)

	// Increment session counters
	sessionKey := SessionKeyPrefix + msg.OverlayID
	pipe.HIncrBy(ctx, sessionKey, "event_count", 1)
	pipe.HSet(ctx, sessionKey, "last_event_at", msg.Timestamp.Format(time.RFC3339))

	// For complex events (raids, mystery gifts), store full event details
	if msg.Event.Type == "raid" || msg.Event.Type == "mystery_gift" {
		eventKey := fmt.Sprintf("%s%s:%s", EventKeyPrefix, sessionID, msg.ID)
		eventJSON, err := json.Marshal(msg)
		if err == nil {
			pipe.HSet(ctx, eventKey, "data", eventJSON)
			pipe.Expire(ctx, eventKey, LeaderboardTTL)
		}
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to store event: %w", err)
	}

	ec.logger.Debug("Captured event for session",
		zap.String("session_id", sessionID),
		zap.String("overlay_id", msg.OverlayID),
		zap.String("event_type", msg.Event.Type),
		zap.String("category", category),
		zap.Float64("score", score),
	)

	return nil
}

// nativeTotalDelta is how much THIS event adds to the member's displayed total
// in the event's own unit. Non-zero only in the gifts category, whose sorted-set
// score is weighted USD; every other category scores natively, so the score IS
// the displayed total and no companion hash is written.
func (ec *EventCapture) nativeTotalDelta(event *models.EventInfo) float64 {
	if ec.getEventCategory(event.Type) != "gifts" {
		return 0
	}
	if event.Value != nil && event.Value.Amount > 0 {
		return event.Value.Amount
	}
	return 1
}
