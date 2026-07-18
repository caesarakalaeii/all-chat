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

package webhooks

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	mpmodels "github.com/caesar/all-chat/services/message-processor/models"
	"github.com/caesar/all-chat/services/message-processor/registry"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/eventsub"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/models"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/publisher"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/status"
	"github.com/caesar/all-chat/shared/metrics"
	"github.com/caesar/all-chat/shared/twitchchat"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// StreamEndEvent is published to Redis "lifecycle:stream_end" when a stream ends.
type StreamEndEvent struct {
	Platform      string    `json:"platform"`
	UserID        string    `json:"user_id"`        // all-chat user UUID
	BroadcasterID string    `json:"broadcaster_id"` // platform-specific ID
	Timestamp     time.Time `json:"timestamp"`
}

// statusHeartbeatInterval bounds how often a channel's "connected" status heartbeat is
// republished on delivered chat. A delivered message is proof the channel.chat.message
// subscription is live, so this is what rehydrates the overlay indicator after an api-gateway
// restart (the in-memory platformState cache is lost) or an eventsub-listener restart (existing
// subscriptions are not re-verified, so the challenge-time publish does not re-fire). Tuned to
// match the claim-refresh cadence so a channel's indicator recovers within one interval of chat
// resuming.
const statusHeartbeatInterval = 60 * time.Second

// Handler handles Twitch EventSub webhook callbacks
type Handler struct {
	secret          []byte
	redis           *redis.Client
	db              *pgxpool.Pool // for twitch_id -> user_id / login lookup
	publisher       *publisher.StreamPublisher
	listenerMetrics *metrics.ListenerMetrics
	statusPublisher *status.Publisher          // emits platform:status for the chat indicator (nil-safe)
	claims          *twitchchat.ClaimStore     // chat-ownership claims for the IRC↔EventSub partition (nil-safe, ADR-0015)
	registry        registry.MessageIDRegistry // native→internal-UUID map for single-message deletions (nil-safe)
	logger          *zap.Logger

	// claimRefreshedAt throttles per-channel claim refreshes (one Redis write per channel per
	// twitchchat.ClaimRefreshInterval) so high chat volume does not amplify into Redis writes.
	claimMu          sync.Mutex
	claimRefreshedAt map[string]time.Time

	// statusPublishedAt throttles the per-channel "connected" status heartbeat (one publish per
	// channel per statusHeartbeatInterval) so high chat volume does not amplify into Redis writes.
	statusMu          sync.Mutex
	statusPublishedAt map[string]time.Time
}

// NewHandler creates a new webhook handler. claims may be nil to disable chat-ownership claims
// (the IRC listener then never sees this listener's channels as claimed). reg may be nil to disable
// native→UUID registration (single-message deletions then can't resolve their target and are
// buffered until they expire).
func NewHandler(secret string, redis *redis.Client, db *pgxpool.Pool, publisher *publisher.StreamPublisher, listenerMetrics *metrics.ListenerMetrics, statusPublisher *status.Publisher, claims *twitchchat.ClaimStore, reg registry.MessageIDRegistry, logger *zap.Logger) *Handler {
	return &Handler{
		secret:            []byte(secret),
		redis:             redis,
		db:                db,
		publisher:         publisher,
		listenerMetrics:   listenerMetrics,
		statusPublisher:   statusPublisher,
		claims:            claims,
		registry:          reg,
		logger:            logger,
		claimRefreshedAt:  make(map[string]time.Time),
		statusPublishedAt: make(map[string]time.Time),
	}
}

// HandleEventSubWebhook processes incoming EventSub webhook notifications
func (h *Handler) HandleEventSubWebhook(c *gin.Context) {
	// Read request body. Cap at 1MB before HMAC verification to prevent
	// unauthenticated OOM via oversized POST bodies (audit M9). Twitch
	// EventSub payloads are well under 1KB, so this never clips legitimate traffic.
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		h.logger.Error("Failed to read webhook body", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Verify signature
	if !h.verifySignature(c, body) {
		h.logger.Warn("Invalid webhook signature",
			zap.String("message_id", c.GetHeader("Twitch-Eventsub-Message-Id")),
		)
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid signature"})
		return
	}

	// Get message type
	messageType := c.GetHeader("Twitch-Eventsub-Message-Type")
	messageID := c.GetHeader("Twitch-Eventsub-Message-Id")

	h.logger.Debug("Received EventSub webhook",
		zap.String("message_type", messageType),
		zap.String("message_id", messageID),
	)

	// Handle based on message type
	switch messageType {
	case "webhook_callback_verification":
		h.handleChallenge(c, body)
	case "notification":
		h.handleNotification(c, body, messageID)
	case "revocation":
		h.handleRevocation(c, body)
	default:
		h.logger.Warn("Unknown message type", zap.String("type", messageType))
		c.Status(http.StatusNoContent)
	}
}

// verifySignature verifies the HMAC-SHA256 signature from Twitch
func (h *Handler) verifySignature(c *gin.Context, body []byte) bool {
	messageID := c.GetHeader("Twitch-Eventsub-Message-Id")
	timestamp := c.GetHeader("Twitch-Eventsub-Message-Timestamp")
	signature := c.GetHeader("Twitch-Eventsub-Message-Signature")

	if messageID == "" || timestamp == "" || signature == "" {
		return false
	}

	// Verify timestamp is recent (within 10 minutes) to prevent replay attacks
	eventTime, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		h.logger.Warn("Invalid timestamp format", zap.String("timestamp", timestamp))
		return false
	}

	age := time.Since(eventTime)
	if age > 10*time.Minute || age < -1*time.Minute {
		h.logger.Warn("Timestamp out of acceptable range",
			zap.Duration("age", age),
			zap.String("timestamp", timestamp),
		)
		return false
	}

	// Compute HMAC-SHA256
	// Message format: message_id + timestamp + body
	message := messageID + timestamp + string(body)
	mac := hmac.New(sha256.New, h.secret)
	mac.Write([]byte(message))
	expectedSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	// Constant-time comparison to prevent timing attacks
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// handleChallenge responds to subscription verification challenge
func (h *Handler) handleChallenge(c *gin.Context, body []byte) {
	var challenge struct {
		Challenge    string                    `json:"challenge"`
		Subscription eventsub.SubscriptionInfo `json:"subscription"`
	}

	if err := json.Unmarshal(body, &challenge); err != nil {
		h.logger.Error("Failed to unmarshal challenge", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid challenge"})
		return
	}

	h.logger.Info("Responding to EventSub challenge",
		zap.String("subscription_id", challenge.Subscription.ID),
		zap.String("type", challenge.Subscription.Type),
	)

	// Record webhook verification (connection confirmed)
	if h.listenerMetrics != nil {
		h.listenerMetrics.RecordConnection("twitch-eventsub", "twitch-eventsub-listener", "webhook", true)
	}

	// Respond with challenge string FIRST — Twitch enforces a short verification timeout, so we
	// must not block the response on the status publish below.
	c.String(http.StatusOK, challenge.Challenge)

	// A verified channel.chat.message subscription is now enabled and will deliver chat, so the
	// channel's chat is "connected". Publish in the background to keep the challenge response fast.
	if challenge.Subscription.Type == "channel.chat.message" {
		if bid := conditionBroadcasterID(challenge.Subscription.Condition); bid != "" {
			go h.publishChatStatus(bid, "connected")
		}
	}
}

// handleNotification processes an event notification
func (h *Handler) handleNotification(c *gin.Context, body []byte, messageID string) {
	ctx := context.Background()

	// Check if already processed (deduplication)
	cacheKey := "eventsub:processed:" + messageID
	exists, err := h.redis.Exists(ctx, cacheKey).Result()
	if err != nil {
		h.logger.Warn("Failed to check message ID cache", zap.Error(err))
		// Continue processing despite cache error
	} else if exists > 0 {
		h.logger.Debug("Message already processed", zap.String("message_id", messageID))
		c.Status(http.StatusNoContent)
		return
	}

	// Parse notification envelope
	var notification struct {
		Subscription eventsub.SubscriptionInfo `json:"subscription"`
		Event        json.RawMessage           `json:"event"`
	}

	if err := json.Unmarshal(body, &notification); err != nil {
		h.logger.Error("Failed to unmarshal notification", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid notification"})
		return
	}

	// Record message received
	if h.listenerMetrics != nil {
		h.listenerMetrics.RecordMessage("twitch-eventsub", "twitch-eventsub-listener", "", notification.Subscription.Type)
	}

	// Route to appropriate handler based on subscription type
	if err := h.routeEvent(ctx, notification.Subscription.Type, notification.Event, messageID); err != nil {
		h.logger.Error("Failed to process event",
			zap.String("subscription_type", notification.Subscription.Type),
			zap.Error(err),
		)
		if h.listenerMetrics != nil {
			h.listenerMetrics.RecordPublish("twitch-eventsub", "twitch-eventsub-listener", "error")
			h.listenerMetrics.RecordError("twitch-eventsub", "twitch-eventsub-listener", "publish", "error")
		}
		// Deliberately do NOT mark this message processed. The previous code marked it as
		// processed even on failure, so a Twitch redelivery (same Twitch-Eventsub-Message-Id)
		// would be silently deduped away and the message lost forever. Leaving the dedup key
		// unset lets a redelivery reprocess. Chat publishes are ring-buffered (ADR-0009), so a
		// transient Redis blip never surfaces here as an error; the errors that do reach here are
		// deterministic parse failures where a retry would not help — hence we still ack 204
		// rather than 5xx (a 5xx storm risks notification_failures_exceeded revocation).
	} else {
		if h.listenerMetrics != nil {
			h.listenerMetrics.RecordPublish("twitch-eventsub", "twitch-eventsub-listener", "success")
		}
		// Mark as processed (TTL: 24 hours) only after a successful handoff so a failed message
		// is never recorded as done.
		if err := h.redis.SetEx(ctx, cacheKey, "1", 24*time.Hour).Err(); err != nil {
			h.logger.Warn("Failed to cache message ID", zap.Error(err))
		}
	}

	c.Status(http.StatusNoContent)
}

// handleRevocation logs subscription revocations
func (h *Handler) handleRevocation(c *gin.Context, body []byte) {
	var revocation struct {
		Subscription eventsub.SubscriptionInfo `json:"subscription"`
	}

	if err := json.Unmarshal(body, &revocation); err != nil {
		h.logger.Error("Failed to unmarshal revocation", zap.Error(err))
		c.Status(http.StatusNoContent)
		return
	}

	h.logger.Warn("EventSub subscription revoked",
		zap.String("subscription_id", revocation.Subscription.ID),
		zap.String("type", revocation.Subscription.Type),
		zap.String("status", revocation.Subscription.Status),
	)

	// A revoked chat subscription stops delivering chat. Release the ownership claim so the IRC
	// listener resumes this channel immediately (ADR-0015) instead of waiting for the claim TTL,
	// and clear the channel's indicator.
	if revocation.Subscription.Type == "channel.chat.message" {
		if bid := conditionBroadcasterID(revocation.Subscription.Condition); bid != "" {
			go h.handleChatSubRevoked(bid)
		}
	}

	c.Status(http.StatusNoContent)
}

// conditionBroadcasterID extracts broadcaster_user_id from an EventSub subscription condition
// (typed as map[string]interface{}). Returns "" when absent or not a string.
func conditionBroadcasterID(condition map[string]interface{}) string {
	if condition == nil {
		return ""
	}
	if v, ok := condition["broadcaster_user_id"].(string); ok {
		return v
	}
	return ""
}

// resolveLogin maps a Twitch broadcaster id to its login (the key api-gateway matches against
// overlay_chat_sources.channel_id, and the key chat-ownership claims use). Returns "" when the
// broadcaster is not a registered all-chat Twitch user. The webhook may land on any pod, so this
// always reads the DB rather than an in-memory channel map.
func (h *Handler) resolveLogin(broadcasterID string) string {
	if broadcasterID == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var login string
	if err := h.db.QueryRow(ctx,
		"SELECT username FROM users WHERE twitch_id = $1 AND auth_provider = 'twitch'", broadcasterID).Scan(&login); err != nil {
		h.logger.Debug("Could not resolve login for broadcaster",
			zap.String("broadcaster_id", broadcasterID), zap.Error(err))
		return ""
	}
	return login
}

// publishChatStatus resolves the broadcaster login and emits a platform:status update for the
// channel's chat indicator. Best-effort: a missing user or publish failure is logged and dropped.
func (h *Handler) publishChatStatus(broadcasterID, state string) {
	h.publishChatStatusForLogin(h.resolveLogin(broadcasterID), state)
}

// publishChatStatusForLogin emits the platform:status update for an already-resolved login.
func (h *Handler) publishChatStatusForLogin(login, state string) {
	if h.statusPublisher == nil || login == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h.statusPublisher.Publish(ctx, status.Message{
		Platform:  "twitch",
		ChannelID: strings.ToLower(login),
		Status:    state,
	})
}

// publishChatConnected emits a platform:status "connected" heartbeat for a channel that just
// delivered a chat message, throttled to one publish per channel per statusHeartbeatInterval. A
// delivered message is the definitive proof that the channel.chat.message subscription is live,
// so this heartbeat — not the one-time challenge verification — is what keeps the overlay
// indicator green across api-gateway and eventsub-listener restarts (the challenge-time publish
// only fires when the subscription is first created; existing subscriptions are not re-verified on
// restart). Best-effort and nil-safe; the Redis publish runs on a background goroutine so the
// webhook response stays fast (mirroring the challenge path's `go h.publishChatStatus`).
func (h *Handler) publishChatConnected(login string) {
	if h.statusPublisher == nil || login == "" {
		return
	}
	login = strings.ToLower(login)

	now := time.Now()
	h.statusMu.Lock()
	if last, ok := h.statusPublishedAt[login]; ok && now.Sub(last) < statusHeartbeatInterval {
		h.statusMu.Unlock()
		return
	}
	h.statusPublishedAt[login] = now
	h.statusMu.Unlock()

	go h.publishChatStatusForLogin(login, "connected")
}

// handleChatSubRevoked reacts to a revoked channel.chat.message subscription: it releases the
// chat-ownership claim so the IRC listener resumes the channel immediately (rather than after the
// claim TTL), then clears the chat indicator. Runs on a background goroutine (resolves the login
// from the DB; the webhook may land on any pod).
func (h *Handler) handleChatSubRevoked(broadcasterID string) {
	login := h.resolveLogin(broadcasterID)
	if h.claims != nil && login != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := h.claims.Release(ctx, login); err != nil {
			h.logger.Warn("Failed to release chat-ownership claim on revocation",
				zap.String("login", login), zap.Error(err))
		}
		cancel()
	}
	h.publishChatStatusForLogin(login, "offline")
}

// refreshChatClaim renews the EventSub chat-ownership claim for a channel that just delivered a
// chat message, throttled to one Redis write per channel per twitchchat.ClaimRefreshInterval. The
// live claim is what keeps this channel on the EventSub path and excluded from IRC (ADR-0015);
// letting it lapse hands the channel back to the always-on IRC listener. The login comes straight
// from the event, so no DB lookup is needed on the hot path.
func (h *Handler) refreshChatClaim(login, broadcasterID string) {
	if h.claims == nil || login == "" {
		return
	}
	login = strings.ToLower(login)

	now := time.Now()
	h.claimMu.Lock()
	if last, ok := h.claimRefreshedAt[login]; ok && now.Sub(last) < twitchchat.ClaimRefreshInterval {
		h.claimMu.Unlock()
		return
	}
	h.claimRefreshedAt[login] = now
	h.claimMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := h.claims.Claim(ctx, login, broadcasterID); err != nil {
		h.logger.Warn("Failed to refresh chat-ownership claim",
			zap.String("login", login), zap.Error(err))
		// Roll back the throttle stamp so the next message retries promptly rather than waiting a
		// full interval after a transient Redis error.
		h.claimMu.Lock()
		delete(h.claimRefreshedAt, login)
		h.claimMu.Unlock()
	}
}

// routeEvent routes events to appropriate handlers based on subscription type.
// messageID is the stable Twitch-Eventsub-Message-Id, threaded into the earn-bearing
// handlers so the engagement consumer can dedup across Twitch redeliveries.
func (h *Handler) routeEvent(ctx context.Context, subscriptionType string, eventData json.RawMessage, messageID string) error {
	switch subscriptionType {
	case "channel.chat.message":
		return h.handleChatMessage(ctx, eventData)
	case "channel.chat.message_delete":
		return h.handleChatMessageDelete(ctx, eventData)
	case "channel.chat.clear_user_messages":
		return h.handleChatClearUserMessages(ctx, eventData)
	case "channel.chat.clear":
		return h.handleChatClear(ctx, eventData)
	case "channel.channel_points_custom_reward_redemption.add":
		return h.handleChannelPointsRedemption(ctx, eventData)
	case "channel.subscribe":
		return h.handleSubscribe(ctx, eventData, messageID)
	case "channel.subscription.gift":
		return h.handleSubscriptionGift(ctx, eventData, messageID)
	case "channel.subscription.message":
		return h.handleResubscription(ctx, eventData, messageID)
	case "channel.raid":
		return h.handleRaid(ctx, eventData)
	case "channel.cheer":
		return h.handleCheer(ctx, eventData, messageID)
	case "channel.follow":
		return h.handleFollow(ctx, eventData)
	case "stream.offline":
		return h.handleStreamOffline(ctx, eventData)
	case "stream.online":
		return h.handleStreamOnline(ctx, eventData)
	case "channel.poll.begin":
		return h.handlePollEvent(ctx, eventData, mpmodels.NativeEventBegin)
	case "channel.poll.progress":
		return h.handlePollEvent(ctx, eventData, mpmodels.NativeEventProgress)
	case "channel.poll.end":
		return h.handlePollEvent(ctx, eventData, mpmodels.NativeEventEnd)
	case "channel.prediction.begin":
		return h.handlePredictionEvent(ctx, eventData, mpmodels.NativeEventBegin)
	case "channel.prediction.progress":
		return h.handlePredictionEvent(ctx, eventData, mpmodels.NativeEventProgress)
	case "channel.prediction.lock":
		return h.handlePredictionEvent(ctx, eventData, mpmodels.NativeEventLock)
	case "channel.prediction.end":
		return h.handlePredictionEvent(ctx, eventData, mpmodels.NativeEventEnd)
	default:
		h.logger.Warn("Unhandled subscription type", zap.String("type", subscriptionType))
		return nil
	}
}

// handleStreamOffline publishes a StreamEndEvent to Redis "lifecycle:stream_end"
// after looking up the all-chat user_id from the users table via twitch_id.
func (h *Handler) handleStreamOffline(ctx context.Context, eventData json.RawMessage) error {
	var event struct {
		BroadcasterUserID string `json:"broadcaster_user_id"`
	}
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal stream.offline event: %w", err)
	}
	if event.BroadcasterUserID == "" {
		h.logger.Warn("stream.offline event missing broadcaster_user_id")
		return nil
	}

	// Look up all-chat user_id from users table via twitch_id
	var userID string
	err := h.db.QueryRow(ctx,
		"SELECT id FROM users WHERE twitch_id = $1", event.BroadcasterUserID).Scan(&userID)
	if err != nil {
		// No user found — broadcaster not registered in all-chat; not an error
		h.logger.Debug("No all-chat user found for Twitch broadcaster",
			zap.String("broadcaster_id", event.BroadcasterUserID))
		return nil
	}

	payload := StreamEndEvent{
		Platform:      "twitch",
		UserID:        userID,
		BroadcasterID: event.BroadcasterUserID,
		Timestamp:     time.Now(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal stream end event: %w", err)
	}

	if err := h.redis.Publish(ctx, "lifecycle:stream_end", string(data)).Err(); err != nil {
		h.logger.Error("Failed to publish lifecycle event",
			zap.String("user_id", userID),
			zap.Error(err))
		return fmt.Errorf("failed to publish lifecycle event: %w", err)
	}

	h.logger.Info("Published stream end lifecycle event",
		zap.String("user_id", userID),
		zap.String("broadcaster_id", event.BroadcasterUserID))
	return nil
}

// handleStreamOnline publishes cross-platform event to trigger YouTube discovery reset
// when Twitch stream goes live
func (h *Handler) handleStreamOnline(ctx context.Context, eventData json.RawMessage) error {
	var event struct {
		BroadcasterUserID   string `json:"broadcaster_user_id"`
		BroadcasterUserName string `json:"broadcaster_user_name"`
	}
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal stream.online event: %w", err)
	}

	if event.BroadcasterUserID == "" {
		h.logger.Warn("stream.online event missing broadcaster_user_id")
		return nil
	}

	// Query which overlays have this Twitch channel as a source
	query := `
		SELECT DISTINCT overlay_id
		FROM overlay_chat_sources
		WHERE platform = 'twitch'
		  AND channel_id = $1
		  AND is_active = true
	`

	rows, err := h.db.Query(ctx, query, event.BroadcasterUserID)
	if err != nil {
		return fmt.Errorf("failed to query overlays for stream.online: %w", err)
	}
	defer rows.Close()

	var overlayIDs []string
	for rows.Next() {
		var overlayID string
		if err := rows.Scan(&overlayID); err != nil {
			continue
		}
		overlayIDs = append(overlayIDs, overlayID)
	}

	// Publish cross-platform event for each overlay
	for _, overlayID := range overlayIDs {
		eventChannel := fmt.Sprintf("platform:event:%s", overlayID)
		// Build the cross-platform event payload with json.Marshal instead of
		// fmt.Sprintf so Twitch-supplied BroadcasterUserName cannot break JSON
		// structure or inject fields (audit L19).
		payloadObj := struct {
			Platform  string `json:"platform"`
			Channel   string `json:"channel"`
			Event     string `json:"event"`
			Timestamp string `json:"timestamp"`
		}{
			Platform:  "twitch",
			Channel:   event.BroadcasterUserName,
			Event:     "stream.online",
			Timestamp: time.Now().Format(time.RFC3339),
		}
		eventPayload, err := json.Marshal(payloadObj)
		if err != nil {
			h.logger.Error("Failed to marshal cross-platform stream.online event",
				zap.String("overlay_id", overlayID),
				zap.Error(err),
			)
			continue
		}

		if err := h.redis.Publish(ctx, eventChannel, eventPayload).Err(); err != nil {
			h.logger.Warn("Failed to publish cross-platform stream.online event",
				zap.String("overlay_id", overlayID),
				zap.Error(err),
			)
			continue
		}

		h.logger.Info("Published cross-platform stream.online event",
			zap.String("overlay_id", overlayID),
			zap.String("broadcaster", event.BroadcasterUserName),
		)
	}

	return nil
}

// handleChannelPointsRedemption processes channel point redemption events
func (h *Handler) handleChannelPointsRedemption(ctx context.Context, eventData json.RawMessage) error {
	var event eventsub.ChannelPointsRedemption
	if err := json.Unmarshal(eventData, &event); err != nil {
		return err
	}

	// Determine message text: use user input if provided, otherwise system message
	text := event.UserInput
	if text == "" {
		text = fmt.Sprintf("Redeemed %s", event.Reward.Title)
	}

	// Create raw message for Message Processor
	rawMsg := &models.RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "twitch",
		ChannelID: event.BroadcasterUserLogin,
		UserID:    event.UserID,
		Username:  event.UserLogin,
		Text:      text, // User input if available, otherwise system message
		Timestamp: event.RedeemedAt,
		EventType: "channel_points",
		EventData: map[string]interface{}{
			"reward_id":     event.Reward.ID,
			"reward_title":  event.Reward.Title,
			"reward_cost":   event.Reward.Cost,
			"reward_prompt": event.Reward.Prompt,
			"user_input":    event.UserInput,
			"status":        event.Status,
			"redeemed_at":   event.RedeemedAt,
		},
	}

	return h.publisher.Publish(ctx, rawMsg)
}

// handleSubscribe processes subscription events
func (h *Handler) handleSubscribe(ctx context.Context, eventData json.RawMessage, messageID string) error {
	var event eventsub.SubscribeEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return err
	}

	rawMsg := &models.RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "twitch",
		ChannelID: event.BroadcasterUserLogin,
		UserID:    event.UserID,
		Username:  event.UserLogin,
		Text:      fmt.Sprintf("Subscribed at %s", event.Tier),
		Timestamp: time.Now(),
		EventType: "subscription",
		EventData: map[string]interface{}{
			"tier":                event.Tier,
			"is_gift":             event.IsGift,
			"plan_name":           event.Tier,
			"eventsub_message_id": messageID,
		},
	}

	return h.publisher.Publish(ctx, rawMsg)
}

// handleSubscriptionGift processes gift subscription events
func (h *Handler) handleSubscriptionGift(ctx context.Context, eventData json.RawMessage, messageID string) error {
	var event eventsub.SubscriptionGiftEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return err
	}

	rawMsg := &models.RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "twitch",
		ChannelID: event.BroadcasterUserLogin,
		UserID:    event.UserID,
		Username:  event.UserLogin,
		Text:      fmt.Sprintf("Gifted %d subs", event.Total),
		Timestamp: time.Now(),
		EventType: "mystery_gift",
		EventData: map[string]interface{}{
			"tier":                event.Tier,
			"total":               event.Total,
			"cumulative_total":    event.CumulativeTotal,
			"is_anonymous":        event.IsAnonymous,
			"eventsub_message_id": messageID,
		},
	}

	return h.publisher.Publish(ctx, rawMsg)
}

// handleResubscription processes resub messages
func (h *Handler) handleResubscription(ctx context.Context, eventData json.RawMessage, messageID string) error {
	var event eventsub.ResubscriptionEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return err
	}

	rawMsg := &models.RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "twitch",
		ChannelID: event.BroadcasterUserLogin,
		UserID:    event.UserID,
		Username:  event.UserLogin,
		Text:      event.Message.Text,
		Timestamp: time.Now(),
		EventType: "resubscription",
		EventData: map[string]interface{}{
			"tier":                event.Tier,
			"months":              event.CumulativeMonths,
			"streak":              event.StreakMonths,
			"duration_months":     event.DurationMonths,
			"eventsub_message_id": messageID,
		},
	}

	return h.publisher.Publish(ctx, rawMsg)
}

// handleRaid processes raid events
func (h *Handler) handleRaid(ctx context.Context, eventData json.RawMessage) error {
	var event eventsub.RaidEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return err
	}

	rawMsg := &models.RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "twitch",
		ChannelID: event.ToBroadcasterUserLogin,
		UserID:    event.FromBroadcasterUserID,
		Username:  event.FromBroadcasterUserLogin,
		Text:      fmt.Sprintf("Raiding with %d viewers", event.Viewers),
		Timestamp: time.Now(),
		EventType: "raid",
		EventData: map[string]interface{}{
			"viewer_count": event.Viewers,
			"from_id":      event.FromBroadcasterUserID,
			"from_name":    event.FromBroadcasterUserName,
		},
	}

	return h.publisher.Publish(ctx, rawMsg)
}

// handleCheer processes bits/cheer events
func (h *Handler) handleCheer(ctx context.Context, eventData json.RawMessage, messageID string) error {
	var event eventsub.CheerEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return err
	}

	rawMsg := &models.RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "twitch",
		ChannelID: event.BroadcasterUserLogin,
		UserID:    event.UserID,
		Username:  event.UserLogin,
		Text:      event.Message,
		Timestamp: time.Now(),
		EventType: "bits",
		EventData: map[string]interface{}{
			"bits":                event.Bits,
			"is_anonymous":        event.IsAnonymous,
			"eventsub_message_id": messageID,
		},
	}

	return h.publisher.Publish(ctx, rawMsg)
}

// handleFollow processes follow events
func (h *Handler) handleFollow(ctx context.Context, eventData json.RawMessage) error {
	var event eventsub.FollowEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return err
	}

	rawMsg := &models.RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "twitch",
		ChannelID: event.BroadcasterUserLogin,
		UserID:    event.UserID,
		Username:  event.UserLogin,
		Text:      "Followed",
		Timestamp: event.FollowedAt,
		EventType: "follow",
		EventData: map[string]interface{}{},
	}

	return h.publisher.Publish(ctx, rawMsg)
}

// handleChatMessage processes a channel.chat.message event into an IRC-compatible
// RawChatMessage so the message-processor normalizes and renders it identically to chat
// ingested over IRC by twitch-listener. EventType is left empty so the message flows
// through the regular chat path (Normalize), not the event path (NormalizeEvent).
func (h *Handler) handleChatMessage(ctx context.Context, eventData json.RawMessage) error {
	var event eventsub.ChatMessageEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal channel.chat.message: %w", err)
	}
	if event.MessageID == "" || event.ChatterUserLogin == "" {
		h.logger.Warn("channel.chat.message missing required fields",
			zap.String("broadcaster", event.BroadcasterUserLogin))
		return nil
	}

	rawMsg := &models.RawChatMessage{
		MessageID: uuid.New().String(), // internal UUID; native Twitch id lives in Tags["id"]
		Platform:  "twitch",
		ChannelID: strings.ToLower(event.BroadcasterUserLogin),
		UserID:    event.ChatterUserID,
		Username:  strings.ToLower(event.ChatterUserLogin),
		Text:      event.Message.Text,
		Timestamp: time.Now().UTC(),
		Tags:      buildChatTags(&event),
		EventType: "", // regular chat → message-processor uses Normalize()
	}

	// Register the native→internal-UUID mapping BEFORE publishing (best-effort), mirroring the IRC
	// listener's capture-point registration. This is what lets a later channel.chat.message_delete
	// resolve its native message id to the internal UUID the overlay tracks (message-processor's
	// registry lookup). For chat-scoped channels IRC no longer runs, so EventSub is the only writer
	// of this mapping (ADR-0015); without it single-message deletes would buffer until they expire.
	if h.registry != nil {
		if nativeID := rawMsg.Tags["id"]; nativeID != "" {
			if err := h.registry.Add(ctx, rawMsg.Platform, rawMsg.ChannelID, nativeID, rawMsg.MessageID); err != nil {
				h.logger.Error("Failed to add message to registry",
					zap.String("native_id", nativeID),
					zap.String("internal_uuid", rawMsg.MessageID),
					zap.Error(err))
				// Continue — registration is best-effort; the chat message must still be delivered.
			}
		}
	}

	// A delivered chat message proves EventSub currently owns this channel's chat — refresh the
	// ownership claim (throttled) so it stays excluded from IRC (ADR-0015), and emit a throttled
	// "connected" status heartbeat so the overlay indicator reflects this source as live (the
	// challenge-time publish only fires once, on subscription creation).
	h.refreshChatClaim(event.BroadcasterUserLogin, event.BroadcasterUserID)
	h.publishChatConnected(event.BroadcasterUserLogin)

	return h.publisher.Publish(ctx, rawMsg)
}

// handleChatMessageDelete processes a channel.chat.message_delete event (a moderator removed a
// single message) into a "single" message_deletion RawChatMessage, shaped identically to
// twitch-listener's ParseClearMessage so the message-processor resolves and applies it the same way.
func (h *Handler) handleChatMessageDelete(ctx context.Context, eventData json.RawMessage) error {
	var event eventsub.ChatMessageDeleteEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal channel.chat.message_delete: %w", err)
	}
	rawMsg := buildSingleDeletion(&event)
	if rawMsg == nil {
		h.logger.Warn("channel.chat.message_delete missing required fields",
			zap.String("broadcaster", event.BroadcasterUserLogin),
			zap.String("message_id", event.MessageID))
		return nil
	}
	return h.publisher.Publish(ctx, rawMsg)
}

// handleChatClearUserMessages processes a channel.chat.clear_user_messages event (a user's messages
// were removed via timeout or ban) into a "batch" message_deletion. Twitch omits the duration, so
// no ban_duration is set; the frontend filters by target_user_id either way.
func (h *Handler) handleChatClearUserMessages(ctx context.Context, eventData json.RawMessage) error {
	var event eventsub.ChatClearUserMessagesEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal channel.chat.clear_user_messages: %w", err)
	}
	rawMsg := buildBatchDeletion(&event)
	if rawMsg == nil {
		h.logger.Warn("channel.chat.clear_user_messages missing required fields",
			zap.String("broadcaster", event.BroadcasterUserLogin),
			zap.String("target_user_id", event.TargetUserID))
		return nil
	}
	return h.publisher.Publish(ctx, rawMsg)
}

// handleChatClear processes a channel.chat.clear event (the entire chat was cleared) into a "clear"
// message_deletion; the frontend drops all messages for the channel.
func (h *Handler) handleChatClear(ctx context.Context, eventData json.RawMessage) error {
	var event eventsub.ChatClearEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal channel.chat.clear: %w", err)
	}
	rawMsg := buildClearDeletion(&event)
	if rawMsg == nil {
		h.logger.Warn("channel.chat.clear missing broadcaster_user_login")
		return nil
	}
	return h.publisher.Publish(ctx, rawMsg)
}

// buildSingleDeletion converts a channel.chat.message_delete event into the message_deletion
// RawChatMessage the pipeline expects: EventType "message_deletion", deletion_type "single",
// target_msg_id = the native message id (looked up against the registry to find the internal UUID).
// ChannelID is the lower-cased broadcaster login, matching the chat path and the registry key.
// Returns nil when the event lacks the fields needed to act on it.
func buildSingleDeletion(e *eventsub.ChatMessageDeleteEvent) *models.RawChatMessage {
	if e.MessageID == "" || e.BroadcasterUserLogin == "" {
		return nil
	}
	return &models.RawChatMessage{
		MessageID: uuid.New().String(), // UUID for the deletion event itself
		Platform:  "twitch",
		ChannelID: strings.ToLower(e.BroadcasterUserLogin),
		Username:  strings.ToLower(e.TargetUserLogin), // author of the removed message, if provided
		Timestamp: time.Now().UTC(),
		EventType: "message_deletion",
		EventData: map[string]interface{}{
			"deletion_type": "single",
			"target_msg_id": e.MessageID,
		},
	}
}

// buildBatchDeletion converts a channel.chat.clear_user_messages event into a "batch"
// message_deletion (all of a user's messages removed). target_user_id is what the frontend filters
// on. No ban_duration is set — Twitch does not provide it on this event, so a timeout is
// indistinguishable from a ban here and is treated as a ban downstream. Returns nil when the event
// lacks the fields needed to act on it.
func buildBatchDeletion(e *eventsub.ChatClearUserMessagesEvent) *models.RawChatMessage {
	if e.TargetUserID == "" || e.BroadcasterUserLogin == "" {
		return nil
	}
	return &models.RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "twitch",
		ChannelID: strings.ToLower(e.BroadcasterUserLogin),
		UserID:    e.TargetUserID,
		Username:  strings.ToLower(e.TargetUserLogin),
		Timestamp: time.Now().UTC(),
		EventType: "message_deletion",
		EventData: map[string]interface{}{
			"deletion_type":   "batch",
			"target_user_id":  e.TargetUserID,
			"target_username": strings.ToLower(e.TargetUserLogin),
		},
	}
}

// buildClearDeletion converts a channel.chat.clear event into a "clear" message_deletion (the whole
// chat was cleared). No target fields — the frontend drops every message for the channel. Returns
// nil when the broadcaster login is absent (without it the deletion can't be routed to overlays).
func buildClearDeletion(e *eventsub.ChatClearEvent) *models.RawChatMessage {
	if e.BroadcasterUserLogin == "" {
		return nil
	}
	return &models.RawChatMessage{
		MessageID: uuid.New().String(),
		Platform:  "twitch",
		ChannelID: strings.ToLower(e.BroadcasterUserLogin),
		Timestamp: time.Now().UTC(),
		EventType: "message_deletion",
		EventData: map[string]interface{}{
			"deletion_type": "clear",
		},
	}
}

// buildChatTags reconstructs the IRC-style tag map (see twitch-listener/irc/parser.go and
// the keys consumed by message-processor/normalizer/twitch_normalizer.go) from a
// channel.chat.message payload, so downstream normalization and enrichment behave
// identically for EventSub- and IRC-sourced chat.
func buildChatTags(e *eventsub.ChatMessageEvent) map[string]string {
	tags := make(map[string]string)

	if e.ChatterUserName != "" {
		tags["display-name"] = e.ChatterUserName
	}
	if e.Color != "" {
		tags["color"] = e.Color
	}

	// Boolean flags are derived from badges (EventSub has no separate sub/mod/turbo fields).
	tags["subscriber"], tags["mod"], tags["turbo"] = "0", "0", "0"
	if len(e.Badges) > 0 {
		// "set_id/id,set_id/id" — for most sets EventSub's badge id IS the version the
		// badge enricher expects, but predictions are special (see badgeTagVersion).
		parts := make([]string, 0, len(e.Badges))
		for _, b := range e.Badges {
			parts = append(parts, b.SetID+"/"+badgeTagVersion(b))
			switch b.SetID {
			case "subscriber":
				tags["subscriber"] = "1"
			case "moderator":
				tags["mod"] = "1"
			case "turbo":
				tags["turbo"] = "1"
			}
		}
		tags["badges"] = strings.Join(parts, ",")
	}

	tags["id"] = e.MessageID
	// CRITICAL: the normalizer surfaces room-id as twitch_room_id, which every enricher
	// (emote/badge/cheermote) uses as the channel key. Missing it silently breaks
	// per-channel emotes and badge tiers.
	tags["room-id"] = e.BroadcasterUserID
	tags["tmi-sent-ts"] = strconv.FormatInt(time.Now().UnixMilli(), 10)

	// bits: prefer the top-level cheer total, else sum cheermote fragments.
	bits := 0
	if e.Cheer != nil {
		bits = e.Cheer.Bits
	}
	if bits == 0 {
		for _, f := range e.Message.Fragments {
			if f.Type == "cheermote" && f.Cheermote != nil {
				bits += f.Cheermote.Bits
			}
		}
	}
	if bits > 0 {
		tags["bits"] = strconv.Itoa(bits)
	}

	if em := buildEmotesTag(e.Message.Fragments); em != "" {
		tags["emotes"] = em
	}

	if gf := buildGifsTag(e.Message.Fragments); gf != "" {
		tags["gifs"] = gf
	}

	// Shared-chat parity (mirrors the IRC parser's source-* tags). Note: the normalizer
	// reads source-id as the source user id, but Twitch's IRC source-id tag is actually the
	// source message id; we set the same value the IRC path would, so behaviour matches.
	if e.SourceBroadcasterUserID != "" {
		tags["source-room-id"] = e.SourceBroadcasterUserID
		if e.SourceMessageID != "" {
			tags["source-id"] = e.SourceMessageID
		}
		if len(e.SourceBadges) > 0 {
			parts := make([]string, 0, len(e.SourceBadges))
			for _, b := range e.SourceBadges {
				parts = append(parts, b.SetID+"/"+badgeTagVersion(b))
			}
			tags["source-badges"] = strings.Join(parts, ",")
		}
	}

	return tags
}

// badgeTagVersion returns the badge version to put in the IRC-style "badges" tag.
// For most sets EventSub's badge id already equals the version the badge enricher
// looks up. The "predictions" set is the exception: EventSub sends the numeric
// outcome index (e.g. "0", "1") rather than the color-coded version that the badge
// image API keys on ("blue-1", "pink-2", ...), so the index can never be resolved
// to an icon and the badge silently vanishes from overlays. IRC always delivered
// the color version directly; this restores parity by translating the index.
func badgeTagVersion(b eventsub.ChatBadge) string {
	if b.SetID == "predictions" {
		return predictionBadgeVersion(b.ID)
	}
	return b.ID
}

// predictionBadgeVersion maps an EventSub prediction outcome index to the
// color-coded badge version Twitch uses for the image CDN. It follows Twitch's
// two-outcome convention (outcome 0 = "blue-1", outcome 1 = "pink-2"); predictions
// with additional outcomes use "blue-N" (badge set: blue-1..blue-10). The outcome
// count is not present on the chat message, so a multi-outcome prediction's second
// outcome (index 1) renders as "pink-2" rather than "blue-2" — a rare cosmetic
// edge case that still shows a badge instead of none. Unknown ids pass through.
func predictionBadgeVersion(id string) string {
	switch id {
	case "0":
		return "blue-1"
	case "1":
		return "pink-2"
	default:
		if n, err := strconv.Atoi(id); err == nil && n >= 2 && n <= 9 {
			return fmt.Sprintf("blue-%d", n+1)
		}
		return id
	}
}

// buildEmotesTag renders the IRC "emotes" tag ("id:start-end,start-end/id:...") from the
// ordered message fragments. Positions are inclusive byte offsets into Message.Text,
// matching the byte slicing in message-processor's extractTwitchEmotes. Without this,
// first-party Twitch emotes would not render (a regression versus IRC); third-party
// 7TV/BTTV/FFZ emotes are added separately by the emote enricher from message text.
func buildEmotesTag(frags []eventsub.ChatMessageFragment) string {
	type span struct{ start, end int }
	positions := make(map[string][]span)
	order := make([]string, 0)

	offset := 0
	for _, f := range frags {
		n := len(f.Text) // byte length — IRC emote positions are byte offsets into the text
		if f.Type == "emote" && f.Emote != nil && f.Emote.ID != "" && n > 0 {
			if _, seen := positions[f.Emote.ID]; !seen {
				order = append(order, f.Emote.ID)
			}
			positions[f.Emote.ID] = append(positions[f.Emote.ID], span{start: offset, end: offset + n - 1})
		}
		offset += n
	}

	if len(order) == 0 {
		return ""
	}

	groups := make([]string, 0, len(order))
	for _, id := range order {
		spans := positions[id]
		ps := make([]string, 0, len(spans))
		for _, s := range spans {
			ps = append(ps, fmt.Sprintf("%d-%d", s.start, s.end))
		}
		groups = append(groups, id+":"+strings.Join(ps, ","))
	}
	return strings.Join(groups, "/")
}

// buildGifsTag renders the IRC "gifs" tag ("start-end|gif_id|url,start-end|gif_id|url")
// from the ordered message fragments, matching Twitch's documented IRC format so the
// message-processor normalizes EventSub- and IRC-sourced chat GIFs identically
// (ADR-0037). Positions are inclusive byte offsets into Message.Text — the same span the
// fragment's alt caption occupies — consistent with buildEmotesTag. Without this,
// EventSub chat GIFs would arrive as bare "[alt caption]" text with no image.
func buildGifsTag(frags []eventsub.ChatMessageFragment) string {
	parts := make([]string, 0)
	offset := 0
	for _, f := range frags {
		n := len(f.Text) // byte length — positions are byte offsets into the text
		if f.Type == "gif" && f.Gif != nil && f.Gif.URL != "" && n > 0 {
			parts = append(parts, fmt.Sprintf("%d-%d|%s|%s", offset, offset+n-1, f.Gif.GifID, f.Gif.URL))
		}
		offset += n
	}
	return strings.Join(parts, ",")
}
