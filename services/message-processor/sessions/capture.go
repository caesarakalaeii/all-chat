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

	// Check if event should be captured
	if !ec.shouldCaptureEvent(msg) {
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

	// Capture monetary and engagement events only
	captureTypes := map[string]bool{
		"subscription":       true,
		"resubscription":     true,
		"gift_subscription":  true,
		"mystery_gift":       true,
		"bits":               true,
		"raid":               true,
		"super_chat":         true,
		"super_sticker":      true,
		"new_sponsor":        true,
		"member_milestone":   true,
		"membership_gift":    true,
		"follow":             true,
		"gift":               true, // TikTok/Kick
		"channel_points":     true,
	}

	return captureTypes[msg.Event.Type]
}

// getEventCategory maps event types to leaderboard categories
func (ec *EventCapture) getEventCategory(eventType string) string {
	categories := map[string]string{
		"subscription":       "subs",
		"resubscription":     "subs",
		"gift_subscription":  "gifts",
		"mystery_gift":       "gifts",
		"bits":               "bits",
		"raid":               "raids",
		"super_chat":         "super_chats",
		"super_sticker":      "super_chats",
		"new_sponsor":        "memberships",
		"member_milestone":   "memberships",
		"membership_gift":    "gifts",
		"follow":             "follows",
		"gift":               "gifts", // TikTok/Kick
		"channel_points":     "points",
	}

	if category, ok := categories[eventType]; ok {
		return category
	}
	return "other"
}

// calculateScore determines the score for leaderboard sorting
func (ec *EventCapture) calculateScore(event *models.EventInfo) float64 {
	if event.Value == nil {
		return 1.0 // Count-based (1 per event)
	}
	return event.Value.Amount
}

// MetadataKeyPrefix is the Redis key prefix for leaderboard member metadata
const MetadataKeyPrefix = "session:leaderboard:meta:"

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
	}

	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal member metadata: %w", err)
	}

	// ZINCRBY with stable key — aggregates score across multiple events per user
	score := ec.calculateScore(msg.Event)
	pipe.ZIncrBy(ctx, leaderboardKey, score, memberKey)
	pipe.Expire(ctx, leaderboardKey, LeaderboardTTL)

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
