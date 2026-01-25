package eventsub

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	// EventSubWebSocketURL is the Twitch EventSub WebSocket endpoint
	EventSubWebSocketURL = "wss://eventsub.wss.twitch.tv/ws"

	// PingInterval is how often to send ping messages
	PingInterval = 30 * time.Second

	// PongTimeout is how long to wait for pong responses
	PongTimeout = 10 * time.Second
)

// NotificationHandler is called when a notification is received
type NotificationHandler func(event Event, subscription *Subscription)

// ReconnectHandler is called when reconnection is requested
type ReconnectHandler func(reconnectURL string)

// Client manages the EventSub WebSocket connection
type Client struct {
	conn      *websocket.Conn
	sessionID string
	logger    *zap.Logger

	// Handlers
	onNotification NotificationHandler
	onReconnect    ReconnectHandler
	onWelcome      func(sessionID string)

	// State
	mu        sync.RWMutex
	connected bool
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

// NewClient creates a new EventSub WebSocket client
func NewClient(logger *zap.Logger) *Client {
	return &Client{
		logger:   logger,
		stopChan: make(chan struct{}),
	}
}

// SetOnNotification sets the notification handler
func (c *Client) SetOnNotification(handler NotificationHandler) {
	c.onNotification = handler
}

// SetOnReconnect sets the reconnect handler
func (c *Client) SetOnReconnect(handler ReconnectHandler) {
	c.onReconnect = handler
}

// SetOnWelcome sets the welcome handler
func (c *Client) SetOnWelcome(handler func(sessionID string)) {
	c.onWelcome = handler
}

// Connect establishes connection to EventSub WebSocket
func (c *Client) Connect(ctx context.Context) error {
	return c.ConnectTo(ctx, EventSubWebSocketURL)
}

// ConnectTo connects to a specific WebSocket URL (for reconnection)
func (c *Client) ConnectTo(ctx context.Context, url string) error {
	c.logger.Info("Connecting to Twitch EventSub WebSocket", zap.String("url", url))

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to EventSub WebSocket: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.connected = true
	c.mu.Unlock()

	c.logger.Info("Connected to EventSub WebSocket")

	// Start read loop
	c.wg.Add(1)
	go c.readLoop()

	return nil
}

// Disconnect closes the WebSocket connection
func (c *Client) Disconnect() error {
	c.logger.Info("Disconnecting from EventSub WebSocket")

	close(c.stopChan)

	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.connected = false
	c.mu.Unlock()

	c.wg.Wait()

	c.logger.Info("Disconnected from EventSub WebSocket")
	return nil
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// GetSessionID returns the current session ID
func (c *Client) GetSessionID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionID
}

// readLoop reads messages from the WebSocket
func (c *Client) readLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopChan:
			return
		default:
		}

		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		if conn == nil {
			return
		}

		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			c.logger.Error("Failed to read message", zap.Error(err))
			c.mu.Lock()
			c.connected = false
			c.mu.Unlock()
			return
		}

		// Handle message based on type
		if err := c.handleMessage(&msg); err != nil {
			c.logger.Error("Failed to handle message",
				zap.String("message_type", msg.Metadata.MessageType),
				zap.Error(err),
			)
		}
	}
}

// handleMessage processes incoming WebSocket messages
func (c *Client) handleMessage(msg *Message) error {
	switch msg.Metadata.MessageType {
	case "session_welcome":
		return c.handleWelcome(msg)

	case "session_keepalive":
		c.logger.Debug("Received keepalive")
		return nil

	case "notification":
		return c.handleNotification(msg)

	case "session_reconnect":
		return c.handleReconnect(msg)

	case "revocation":
		return c.handleRevocation(msg)

	default:
		c.logger.Warn("Unknown message type",
			zap.String("message_type", msg.Metadata.MessageType),
		)
		return nil
	}
}

// handleWelcome processes session_welcome messages
func (c *Client) handleWelcome(msg *Message) error {
	if msg.Payload.Session == nil {
		return fmt.Errorf("welcome message missing session")
	}

	c.mu.Lock()
	c.sessionID = msg.Payload.Session.ID
	c.mu.Unlock()

	c.logger.Info("Received session welcome",
		zap.String("session_id", msg.Payload.Session.ID),
		zap.Int("keepalive_timeout", msg.Payload.Session.KeepaliveTimeoutSeconds),
	)

	// Call welcome handler to create subscriptions
	if c.onWelcome != nil {
		c.onWelcome(msg.Payload.Session.ID)
	}

	return nil
}

// handleNotification processes notification messages
func (c *Client) handleNotification(msg *Message) error {
	if msg.Payload.Subscription == nil {
		return fmt.Errorf("notification missing subscription")
	}

	c.logger.Debug("Received notification",
		zap.String("subscription_type", msg.Payload.Subscription.Type),
		zap.String("subscription_id", msg.Payload.Subscription.ID),
	)

	// Call notification handler
	if c.onNotification != nil {
		c.onNotification(msg.Payload.Event, msg.Payload.Subscription)
	}

	return nil
}

// handleReconnect processes session_reconnect messages
func (c *Client) handleReconnect(msg *Message) error {
	if msg.Payload.Session == nil || msg.Payload.Session.ReconnectURL == "" {
		return fmt.Errorf("reconnect message missing URL")
	}

	c.logger.Warn("Received reconnect request",
		zap.String("reconnect_url", msg.Payload.Session.ReconnectURL),
	)

	// Call reconnect handler
	if c.onReconnect != nil {
		c.onReconnect(msg.Payload.Session.ReconnectURL)
	}

	return nil
}

// handleRevocation processes revocation messages
func (c *Client) handleRevocation(msg *Message) error {
	if msg.Payload.Subscription == nil {
		return fmt.Errorf("revocation missing subscription")
	}

	c.logger.Warn("Subscription revoked",
		zap.String("subscription_type", msg.Payload.Subscription.Type),
		zap.String("subscription_id", msg.Payload.Subscription.ID),
		zap.String("status", msg.Payload.Subscription.Status),
	)

	return nil
}

// ParseChannelPointsRedemption parses an event into ChannelPointsRedemption
func ParseChannelPointsRedemption(event Event) (*ChannelPointsRedemption, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	var redemption ChannelPointsRedemption
	if err := json.Unmarshal(data, &redemption); err != nil {
		return nil, fmt.Errorf("failed to unmarshal redemption: %w", err)
	}

	return &redemption, nil
}

// ParseSubscribeEvent parses an event into SubscribeEvent
func ParseSubscribeEvent(event Event) (*SubscribeEvent, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	var sub SubscribeEvent
	if err := json.Unmarshal(data, &sub); err != nil {
		return nil, fmt.Errorf("failed to unmarshal subscribe event: %w", err)
	}

	return &sub, nil
}

// ParseSubscriptionGiftEvent parses an event into SubscriptionGiftEvent
func ParseSubscriptionGiftEvent(event Event) (*SubscriptionGiftEvent, error) {
	data, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	var gift SubscriptionGiftEvent
	if err := json.Unmarshal(data, &gift); err != nil {
		return nil, fmt.Errorf("failed to unmarshal gift event: %w", err)
	}

	return &gift, nil
}
