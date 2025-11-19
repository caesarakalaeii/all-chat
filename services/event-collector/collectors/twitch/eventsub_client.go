package twitch

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/caesar/all-chat/services/event-collector/models"
	"github.com/caesar/all-chat/services/event-collector/normalizers"
	"github.com/caesar/all-chat/services/event-collector/repository"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// EventSubClient manages Twitch EventSub WebSocket connection
type EventSubClient struct {
	conn             *websocket.Conn
	sessionID        string
	logger           *zap.Logger
	eventRepo        *repository.EventRepository
	sessionRepo      *repository.SessionRepository
	normalizer       *normalizers.TwitchNormalizer
	clientID         string
	accessToken      string
	reconnectDelay   time.Duration
	keepaliveTimeout time.Duration
	done             chan struct{}
}

// EventSubMessage represents the WebSocket message format
type EventSubMessage struct {
	Metadata EventSubMetadata    `json:"metadata"`
	Payload  json.RawMessage     `json:"payload"`
}

// EventSubMetadata contains message metadata
type EventSubMetadata struct {
	MessageID           string    `json:"message_id"`
	MessageType         string    `json:"message_type"`
	MessageTimestamp    time.Time `json:"message_timestamp"`
	SubscriptionType    string    `json:"subscription_type,omitempty"`
	SubscriptionVersion string    `json:"subscription_version,omitempty"`
}

// SessionWelcomePayload is sent when connection is established
type SessionWelcomePayload struct {
	Session struct {
		ID                      string `json:"id"`
		Status                  string `json:"status"`
		KeepaliveTimeoutSeconds int    `json:"keepalive_timeout_seconds"`
		ReconnectURL            string `json:"reconnect_url,omitempty"`
		ConnectedAt             string `json:"connected_at"`
	} `json:"session"`
}

// NotificationPayload contains the actual event data
type NotificationPayload struct {
	Subscription struct {
		ID        string                 `json:"id"`
		Status    string                 `json:"status"`
		Type      string                 `json:"type"`
		Version   string                 `json:"version"`
		Cost      int                    `json:"cost"`
		Condition map[string]interface{} `json:"condition"`
		Transport struct {
			Method    string `json:"method"`
			SessionID string `json:"session_id"`
		} `json:"transport"`
		CreatedAt time.Time `json:"created_at"`
	} `json:"subscription"`
	Event json.RawMessage `json:"event"`
}

const (
	EventSubWebSocketURL = "wss://eventsub.wss.twitch.tv/ws"

	// Message types
	MessageTypeWelcome      = "session_welcome"
	MessageTypeKeepalive    = "session_keepalive"
	MessageTypeNotification = "notification"
	MessageTypeReconnect    = "session_reconnect"
	MessageTypeRevocation   = "revocation"
)

// NewEventSubClient creates a new Twitch EventSub WebSocket client
func NewEventSubClient(
	logger *zap.Logger,
	eventRepo *repository.EventRepository,
	sessionRepo *repository.SessionRepository,
	normalizer *normalizers.TwitchNormalizer,
	clientID string,
	accessToken string,
) *EventSubClient {
	return &EventSubClient{
		logger:           logger,
		eventRepo:        eventRepo,
		sessionRepo:      sessionRepo,
		normalizer:       normalizer,
		clientID:         clientID,
		accessToken:      accessToken,
		reconnectDelay:   5 * time.Second,
		keepaliveTimeout: 10 * time.Second,
		done:             make(chan struct{}),
	}
}

// Connect establishes WebSocket connection to Twitch EventSub
func (c *EventSubClient) Connect(ctx context.Context) error {
	c.logger.Info("Connecting to Twitch EventSub WebSocket")

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, EventSubWebSocketURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to EventSub: %w", err)
	}

	c.conn = conn
	c.logger.Info("Connected to Twitch EventSub WebSocket")

	return nil
}

// Listen starts listening for EventSub messages
func (c *EventSubClient) Listen(ctx context.Context) error {
	defer c.conn.Close()

	// Set up keepalive timer
	keepaliveTimer := time.NewTimer(c.keepaliveTimeout)
	defer keepaliveTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("EventSub client stopping")
			return nil
		case <-c.done:
			c.logger.Info("EventSub client stopped")
			return nil
		case <-keepaliveTimer.C:
			c.logger.Warn("Keepalive timeout - no message received")
			return fmt.Errorf("keepalive timeout")
		default:
			// Read message with timeout
			c.conn.SetReadDeadline(time.Now().Add(c.keepaliveTimeout))

			var msg EventSubMessage
			if err := c.conn.ReadJSON(&msg); err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					c.logger.Info("WebSocket closed normally")
					return nil
				}
				c.logger.Error("Failed to read message", zap.Error(err))
				return fmt.Errorf("failed to read message: %w", err)
			}

			// Reset keepalive timer
			keepaliveTimer.Reset(c.keepaliveTimeout)

			// Handle message
			if err := c.handleMessage(ctx, &msg); err != nil {
				c.logger.Error("Failed to handle message",
					zap.String("message_type", msg.Metadata.MessageType),
					zap.Error(err),
				)
			}
		}
	}
}

// handleMessage processes incoming EventSub messages
func (c *EventSubClient) handleMessage(ctx context.Context, msg *EventSubMessage) error {
	switch msg.Metadata.MessageType {
	case MessageTypeWelcome:
		return c.handleWelcome(msg.Payload)

	case MessageTypeKeepalive:
		c.logger.Debug("Received keepalive")
		return nil

	case MessageTypeNotification:
		return c.handleNotification(ctx, msg)

	case MessageTypeReconnect:
		c.logger.Warn("Received reconnect message")
		// TODO: Handle reconnect with new URL
		return nil

	case MessageTypeRevocation:
		c.logger.Warn("Received revocation message")
		return nil

	default:
		c.logger.Warn("Unknown message type", zap.String("type", msg.Metadata.MessageType))
		return nil
	}
}

// handleWelcome processes the welcome message and stores session ID
func (c *EventSubClient) handleWelcome(payload json.RawMessage) error {
	var welcome SessionWelcomePayload
	if err := json.Unmarshal(payload, &welcome); err != nil {
		return fmt.Errorf("failed to unmarshal welcome: %w", err)
	}

	c.sessionID = welcome.Session.ID
	c.keepaliveTimeout = time.Duration(welcome.Session.KeepaliveTimeoutSeconds) * time.Second

	c.logger.Info("Received EventSub session welcome",
		zap.String("session_id", c.sessionID),
		zap.Duration("keepalive_timeout", c.keepaliveTimeout),
	)

	return nil
}

// handleNotification processes event notifications
func (c *EventSubClient) handleNotification(ctx context.Context, msg *EventSubMessage) error {
	var notification NotificationPayload
	if err := json.Unmarshal(msg.Payload, &notification); err != nil {
		return fmt.Errorf("failed to unmarshal notification: %w", err)
	}

	c.logger.Debug("Received event notification",
		zap.String("subscription_type", notification.Subscription.Type),
		zap.String("subscription_id", notification.Subscription.ID),
	)

	// Process event based on subscription type
	switch notification.Subscription.Type {
	case "channel.follow":
		return c.handleFollowEvent(ctx, notification.Event)
	case "channel.subscribe":
		return c.handleSubscribeEvent(ctx, notification.Event)
	case "channel.subscription.message":
		return c.handleSubscriptionMessageEvent(ctx, notification.Event)
	case "channel.subscription.gift":
		return c.handleGiftSubEvent(ctx, notification.Event)
	case "channel.cheer":
		return c.handleCheerEvent(ctx, notification.Event)
	case "channel.raid":
		return c.handleRaidEvent(ctx, notification.Event)
	case "stream.online":
		return c.handleStreamOnlineEvent(ctx, notification.Event)
	case "stream.offline":
		return c.handleStreamOfflineEvent(ctx, notification.Event)
	default:
		c.logger.Debug("Unhandled subscription type",
			zap.String("type", notification.Subscription.Type),
		)
		return nil
	}
}

// handleFollowEvent processes channel.follow events
func (c *EventSubClient) handleFollowEvent(ctx context.Context, eventData json.RawMessage) error {
	var followEvent normalizers.TwitchFollowEvent
	if err := json.Unmarshal(eventData, &followEvent); err != nil {
		return fmt.Errorf("failed to unmarshal follow event: %w", err)
	}

	// TODO: Get actual user ID and session ID from context/state
	userID := uuid.MustParse("00000000-0000-0000-0000-000000000000") // Placeholder

	// Get active session for broadcaster
	session, err := c.sessionRepo.GetActiveSession(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get active session: %w", err)
	}
	if session == nil {
		c.logger.Warn("No active session found for follow event")
		return nil
	}

	// Normalize event
	streamEvent := c.normalizer.NormalizeFollow(&followEvent, session.ID, userID)

	// Store event
	if err := c.eventRepo.CreateEvent(ctx, streamEvent); err != nil {
		return fmt.Errorf("failed to store follow event: %w", err)
	}

	// Update session stats
	if err := c.eventRepo.UpdateSessionStats(ctx, session.ID, models.EventTypeFollow, 1); err != nil {
		c.logger.Error("Failed to update session stats", zap.Error(err))
	}

	c.logger.Info("Processed follow event",
		zap.String("user", followEvent.UserName),
		zap.String("session_id", session.ID.String()),
	)

	return nil
}

// handleSubscribeEvent processes channel.subscribe events
func (c *EventSubClient) handleSubscribeEvent(ctx context.Context, eventData json.RawMessage) error {
	var subEvent normalizers.TwitchSubscribeEvent
	if err := json.Unmarshal(eventData, &subEvent); err != nil {
		return fmt.Errorf("failed to unmarshal subscribe event: %w", err)
	}

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000000") // Placeholder

	session, err := c.sessionRepo.GetActiveSession(ctx, userID)
	if err != nil || session == nil {
		c.logger.Warn("No active session found for subscribe event")
		return err
	}

	streamEvent := c.normalizer.NormalizeSubscribe(&subEvent, session.ID, userID)

	if err := c.eventRepo.CreateEvent(ctx, streamEvent); err != nil {
		return fmt.Errorf("failed to store subscribe event: %w", err)
	}

	if err := c.eventRepo.UpdateSessionStats(ctx, session.ID, models.EventTypeSub, 1); err != nil {
		c.logger.Error("Failed to update session stats", zap.Error(err))
	}

	c.logger.Info("Processed subscribe event",
		zap.String("user", subEvent.UserName),
		zap.String("tier", subEvent.Tier),
	)

	return nil
}

// handleSubscriptionMessageEvent processes channel.subscription.message events (resubs)
func (c *EventSubClient) handleSubscriptionMessageEvent(ctx context.Context, eventData json.RawMessage) error {
	var resubEvent normalizers.TwitchSubscriptionMessageEvent
	if err := json.Unmarshal(eventData, &resubEvent); err != nil {
		return fmt.Errorf("failed to unmarshal resub event: %w", err)
	}

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000000") // Placeholder

	session, err := c.sessionRepo.GetActiveSession(ctx, userID)
	if err != nil || session == nil {
		c.logger.Warn("No active session found for resub event")
		return err
	}

	streamEvent := c.normalizer.NormalizeSubscriptionMessage(&resubEvent, session.ID, userID)

	if err := c.eventRepo.CreateEvent(ctx, streamEvent); err != nil {
		return fmt.Errorf("failed to store resub event: %w", err)
	}

	if err := c.eventRepo.UpdateSessionStats(ctx, session.ID, models.EventTypeSub, 1); err != nil {
		c.logger.Error("Failed to update session stats", zap.Error(err))
	}

	c.logger.Info("Processed resub event",
		zap.String("user", resubEvent.UserName),
		zap.Int("months", resubEvent.CumulativeMonths),
	)

	return nil
}

// handleGiftSubEvent processes channel.subscription.gift events
func (c *EventSubClient) handleGiftSubEvent(ctx context.Context, eventData json.RawMessage) error {
	var giftEvent normalizers.TwitchSubscriptionGiftEvent
	if err := json.Unmarshal(eventData, &giftEvent); err != nil {
		return fmt.Errorf("failed to unmarshal gift sub event: %w", err)
	}

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000000") // Placeholder

	session, err := c.sessionRepo.GetActiveSession(ctx, userID)
	if err != nil || session == nil {
		c.logger.Warn("No active session found for gift sub event")
		return err
	}

	streamEvent := c.normalizer.NormalizeGiftSub(&giftEvent, session.ID, userID)

	if err := c.eventRepo.CreateEvent(ctx, streamEvent); err != nil {
		return fmt.Errorf("failed to store gift sub event: %w", err)
	}

	if err := c.eventRepo.UpdateSessionStats(ctx, session.ID, models.EventTypeGiftSub, giftEvent.Total); err != nil {
		c.logger.Error("Failed to update session stats", zap.Error(err))
	}

	c.logger.Info("Processed gift sub event",
		zap.String("user", giftEvent.UserName),
		zap.Int("count", giftEvent.Total),
	)

	return nil
}

// handleCheerEvent processes channel.cheer events
func (c *EventSubClient) handleCheerEvent(ctx context.Context, eventData json.RawMessage) error {
	var cheerEvent normalizers.TwitchCheerEvent
	if err := json.Unmarshal(eventData, &cheerEvent); err != nil {
		return fmt.Errorf("failed to unmarshal cheer event: %w", err)
	}

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000000") // Placeholder

	session, err := c.sessionRepo.GetActiveSession(ctx, userID)
	if err != nil || session == nil {
		c.logger.Warn("No active session found for cheer event")
		return err
	}

	streamEvent := c.normalizer.NormalizeCheer(&cheerEvent, session.ID, userID)

	if err := c.eventRepo.CreateEvent(ctx, streamEvent); err != nil {
		return fmt.Errorf("failed to store cheer event: %w", err)
	}

	if err := c.eventRepo.UpdateSessionStats(ctx, session.ID, models.EventTypeBits, cheerEvent.Bits); err != nil {
		c.logger.Error("Failed to update session stats", zap.Error(err))
	}

	c.logger.Info("Processed cheer event",
		zap.String("user", cheerEvent.UserName),
		zap.Int("bits", cheerEvent.Bits),
	)

	return nil
}

// handleRaidEvent processes channel.raid events
func (c *EventSubClient) handleRaidEvent(ctx context.Context, eventData json.RawMessage) error {
	var raidEvent normalizers.TwitchRaidEvent
	if err := json.Unmarshal(eventData, &raidEvent); err != nil {
		return fmt.Errorf("failed to unmarshal raid event: %w", err)
	}

	userID := uuid.MustParse("00000000-0000-0000-0000-000000000000") // Placeholder

	session, err := c.sessionRepo.GetActiveSession(ctx, userID)
	if err != nil || session == nil {
		c.logger.Warn("No active session found for raid event")
		return err
	}

	streamEvent := c.normalizer.NormalizeRaid(&raidEvent, session.ID, userID)

	if err := c.eventRepo.CreateEvent(ctx, streamEvent); err != nil {
		return fmt.Errorf("failed to store raid event: %w", err)
	}

	c.logger.Info("Processed raid event",
		zap.String("from", raidEvent.FromBroadcasterUserName),
		zap.Int("viewers", raidEvent.Viewers),
	)

	return nil
}

// handleStreamOnlineEvent processes stream.online events
func (c *EventSubClient) handleStreamOnlineEvent(ctx context.Context, eventData json.RawMessage) error {
	c.logger.Info("Stream went online")
	// TODO: Create new stream session
	return nil
}

// handleStreamOfflineEvent processes stream.offline events
func (c *EventSubClient) handleStreamOfflineEvent(ctx context.Context, eventData json.RawMessage) error {
	c.logger.Info("Stream went offline")
	// TODO: End current stream session
	return nil
}

// Close gracefully closes the WebSocket connection
func (c *EventSubClient) Close() error {
	close(c.done)
	if c.conn != nil {
		return c.conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		)
	}
	return nil
}

// GetSessionID returns the EventSub session ID
func (c *EventSubClient) GetSessionID() string {
	return c.sessionID
}
