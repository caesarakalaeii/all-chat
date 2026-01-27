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
	"time"

	"github.com/caesar/all-chat/services/twitch-eventsub-listener/eventsub"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/models"
	"github.com/caesar/all-chat/services/twitch-eventsub-listener/publisher"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Handler handles Twitch EventSub webhook callbacks
type Handler struct {
	secret    []byte
	redis     *redis.Client
	publisher *publisher.StreamPublisher
	logger    *zap.Logger
}

// NewHandler creates a new webhook handler
func NewHandler(secret string, redis *redis.Client, publisher *publisher.StreamPublisher, logger *zap.Logger) *Handler {
	return &Handler{
		secret:    []byte(secret),
		redis:     redis,
		publisher: publisher,
		logger:    logger,
	}
}

// HandleEventSubWebhook processes incoming EventSub webhook notifications
func (h *Handler) HandleEventSubWebhook(c *gin.Context) {
	// Read request body
	body, err := io.ReadAll(c.Request.Body)
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
		Challenge    string                   `json:"challenge"`
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

	// Respond with challenge string
	c.String(http.StatusOK, challenge.Challenge)
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

	// Route to appropriate handler based on subscription type
	if err := h.routeEvent(ctx, notification.Subscription.Type, notification.Event); err != nil {
		h.logger.Error("Failed to process event",
			zap.String("subscription_type", notification.Subscription.Type),
			zap.Error(err),
		)
		// Still return 204 to acknowledge receipt
	}

	// Mark as processed (TTL: 24 hours)
	if err := h.redis.SetEx(ctx, cacheKey, "1", 24*time.Hour).Err(); err != nil {
		h.logger.Warn("Failed to cache message ID", zap.Error(err))
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

	c.Status(http.StatusNoContent)
}

// routeEvent routes events to appropriate handlers based on subscription type
func (h *Handler) routeEvent(ctx context.Context, subscriptionType string, eventData json.RawMessage) error {
	switch subscriptionType {
	case "channel.channel_points_custom_reward_redemption.add":
		return h.handleChannelPointsRedemption(ctx, eventData)
	case "channel.subscribe":
		return h.handleSubscribe(ctx, eventData)
	case "channel.subscription.gift":
		return h.handleSubscriptionGift(ctx, eventData)
	case "channel.subscription.message":
		return h.handleResubscription(ctx, eventData)
	case "channel.raid":
		return h.handleRaid(ctx, eventData)
	case "channel.cheer":
		return h.handleCheer(ctx, eventData)
	case "channel.follow":
		return h.handleFollow(ctx, eventData)
	default:
		h.logger.Warn("Unhandled subscription type", zap.String("type", subscriptionType))
		return nil
	}
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
		MessageID:   uuid.New().String(),
		Platform:    "twitch",
		ChannelID:   event.BroadcasterUserLogin,
		UserID:      event.UserID,
		Username:    event.UserLogin,
		Text:        text, // User input if available, otherwise system message
		Timestamp:   event.RedeemedAt,
		EventType:   "channel_points",
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
func (h *Handler) handleSubscribe(ctx context.Context, eventData json.RawMessage) error {
	var event eventsub.SubscribeEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return err
	}

	rawMsg := &models.RawChatMessage{
		MessageID:   uuid.New().String(),
		Platform:    "twitch",
		ChannelID:   event.BroadcasterUserLogin,
		UserID:      event.UserID,
		Username:    event.UserLogin,
		Text:        fmt.Sprintf("Subscribed at %s", event.Tier),
		Timestamp:   time.Now(),
		EventType:   "subscription",
		EventData: map[string]interface{}{
			"tier":      event.Tier,
			"is_gift":   event.IsGift,
			"plan_name": event.Tier,
		},
	}

	return h.publisher.Publish(ctx, rawMsg)
}

// handleSubscriptionGift processes gift subscription events
func (h *Handler) handleSubscriptionGift(ctx context.Context, eventData json.RawMessage) error {
	var event eventsub.SubscriptionGiftEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return err
	}

	rawMsg := &models.RawChatMessage{
		MessageID:   uuid.New().String(),
		Platform:    "twitch",
		ChannelID:   event.BroadcasterUserLogin,
		UserID:      event.UserID,
		Username:    event.UserLogin,
		Text:        fmt.Sprintf("Gifted %d subs", event.Total),
		Timestamp:   time.Now(),
		EventType:   "mystery_gift",
		EventData: map[string]interface{}{
			"tier":              event.Tier,
			"total":             event.Total,
			"cumulative_total":  event.CumulativeTotal,
			"is_anonymous":      event.IsAnonymous,
		},
	}

	return h.publisher.Publish(ctx, rawMsg)
}

// handleResubscription processes resub messages
func (h *Handler) handleResubscription(ctx context.Context, eventData json.RawMessage) error {
	var event eventsub.ResubscriptionEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return err
	}

	rawMsg := &models.RawChatMessage{
		MessageID:   uuid.New().String(),
		Platform:    "twitch",
		ChannelID:   event.BroadcasterUserLogin,
		UserID:      event.UserID,
		Username:    event.UserLogin,
		Text:        event.Message.Text,
		Timestamp:   time.Now(),
		EventType:   "resubscription",
		EventData: map[string]interface{}{
			"tier":              event.Tier,
			"months":            event.CumulativeMonths,
			"streak":            event.StreakMonths,
			"duration_months":   event.DurationMonths,
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
		MessageID:   uuid.New().String(),
		Platform:    "twitch",
		ChannelID:   event.ToBroadcasterUserLogin,
		UserID:      event.FromBroadcasterUserID,
		Username:    event.FromBroadcasterUserLogin,
		Text:        fmt.Sprintf("Raiding with %d viewers", event.Viewers),
		Timestamp:   time.Now(),
		EventType:   "raid",
		EventData: map[string]interface{}{
			"viewer_count": event.Viewers,
			"from_id":      event.FromBroadcasterUserID,
			"from_name":    event.FromBroadcasterUserName,
		},
	}

	return h.publisher.Publish(ctx, rawMsg)
}

// handleCheer processes bits/cheer events
func (h *Handler) handleCheer(ctx context.Context, eventData json.RawMessage) error {
	var event eventsub.CheerEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return err
	}

	rawMsg := &models.RawChatMessage{
		MessageID:   uuid.New().String(),
		Platform:    "twitch",
		ChannelID:   event.BroadcasterUserLogin,
		UserID:      event.UserID,
		Username:    event.UserLogin,
		Text:        event.Message,
		Timestamp:   time.Now(),
		EventType:   "bits",
		EventData: map[string]interface{}{
			"bits":         event.Bits,
			"is_anonymous": event.IsAnonymous,
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
		MessageID:   uuid.New().String(),
		Platform:    "twitch",
		ChannelID:   event.BroadcasterUserLogin,
		UserID:      event.UserID,
		Username:    event.UserLogin,
		Text:        "Followed",
		Timestamp:   event.FollowedAt,
		EventType:   "follow",
		EventData:   map[string]interface{}{},
	}

	return h.publisher.Publish(ctx, rawMsg)
}
