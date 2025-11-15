package websocket

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
	// Pusher WebSocket URL for Kick
	pusherURL = "wss://ws-us2.pusher.com/app/32cbd69e4b950bf97679?protocol=7&client=js&version=8.4.0-rc2&flash=false"

	// Pusher event types
	pusherConnectionEstablished = "pusher:connection_established"
	pusherPing                  = "pusher:ping"
	pusherPong                  = "pusher:pong"
	pusherError                 = "pusher:error"
	pusherSubscribe             = "pusher:subscribe"
	pusherSubscriptionSucceeded = "pusher_internal:subscription_succeeded"
	pusherUnsubscribe           = "pusher:unsubscribe"

	// Kick chat event
	kickChatMessageEvent = "App\\Events\\ChatMessageSentEvent"

	// Connection settings
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 30 * time.Second
	maxMessageSize = 512 * 1024 // 512 KB
)

// MessageHandler is called when a chat message is received
type MessageHandler func(channel string, message *KickChatMessage)

// Client represents a Pusher WebSocket client for Kick
type Client struct {
	conn           *websocket.Conn
	connMu         sync.RWMutex
	logger         *zap.Logger
	messageHandler MessageHandler

	// Channel subscriptions
	subscribedChannels map[string]bool
	channelsMu         sync.RWMutex

	// Connection state
	socketID       string
	connected      bool
	reconnecting   bool

	// Control channels
	send           chan []byte
	done           chan struct{}
	reconnectChan  chan struct{}

	ctx            context.Context
	cancel         context.CancelFunc
}

// NewClient creates a new Pusher WebSocket client
func NewClient(messageHandler MessageHandler, logger *zap.Logger) *Client {
	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		logger:             logger,
		messageHandler:     messageHandler,
		subscribedChannels: make(map[string]bool),
		send:               make(chan []byte, 256),
		done:               make(chan struct{}),
		reconnectChan:      make(chan struct{}, 1),
		ctx:                ctx,
		cancel:             cancel,
	}
}

// Connect establishes connection to Pusher WebSocket
func (c *Client) Connect() error {
	c.logger.Info("Connecting to Kick Pusher WebSocket", zap.String("url", pusherURL))

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(pusherURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to Pusher: %w", err)
	}

	c.connMu.Lock()
	c.conn = conn
	c.connected = true
	c.connMu.Unlock()

	// Set connection parameters
	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		c.logger.Debug("Received pong from Pusher")
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Start read and write pumps
	go c.readPump()
	go c.writePump()

	c.logger.Info("Connected to Kick Pusher WebSocket")
	return nil
}

// Disconnect closes the WebSocket connection
func (c *Client) Disconnect() error {
	c.logger.Info("Disconnecting from Kick Pusher WebSocket")

	c.cancel() // Cancel context to stop goroutines
	close(c.done)

	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		// Send close message
		err := c.conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		)
		if err != nil {
			c.logger.Warn("Error sending close message", zap.Error(err))
		}

		c.conn.Close()
		c.conn = nil
	}

	c.connected = false
	c.logger.Info("Disconnected from Kick Pusher WebSocket")
	return nil
}

// Subscribe subscribes to a Kick chatroom channel
func (c *Client) Subscribe(chatroomID int) error {
	channel := fmt.Sprintf("chatrooms.%d", chatroomID)

	c.channelsMu.Lock()
	if c.subscribedChannels[channel] {
		c.channelsMu.Unlock()
		c.logger.Debug("Already subscribed to channel", zap.String("channel", channel))
		return nil
	}
	c.subscribedChannels[channel] = true
	c.channelsMu.Unlock()

	subscribeMsg := PusherSubscribe{
		Event: pusherSubscribe,
		Data: PusherSubscribeData{
			Channel: channel,
		},
	}

	data, err := json.Marshal(subscribeMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal subscribe message: %w", err)
	}

	c.logger.Info("Subscribing to Kick channel",
		zap.String("channel", channel),
		zap.Int("chatroom_id", chatroomID),
	)

	select {
	case c.send <- data:
		return nil
	case <-c.ctx.Done():
		return fmt.Errorf("client is shutting down")
	}
}

// Unsubscribe unsubscribes from a Kick chatroom channel
func (c *Client) Unsubscribe(chatroomID int) error {
	channel := fmt.Sprintf("chatrooms.%d", chatroomID)

	c.channelsMu.Lock()
	if !c.subscribedChannels[channel] {
		c.channelsMu.Unlock()
		c.logger.Debug("Not subscribed to channel", zap.String("channel", channel))
		return nil
	}
	delete(c.subscribedChannels, channel)
	c.channelsMu.Unlock()

	unsubscribeMsg := PusherUnsubscribe{
		Event: pusherUnsubscribe,
		Data: PusherUnsubscribeData{
			Channel: channel,
		},
	}

	data, err := json.Marshal(unsubscribeMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal unsubscribe message: %w", err)
	}

	c.logger.Info("Unsubscribing from Kick channel",
		zap.String("channel", channel),
		zap.Int("chatroom_id", chatroomID),
	)

	select {
	case c.send <- data:
		return nil
	case <-c.ctx.Done():
		return fmt.Errorf("client is shutting down")
	}
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.connected
}

// readPump reads messages from the WebSocket connection
func (c *Client) readPump() {
	defer func() {
		c.logger.Info("Read pump stopped")
		c.triggerReconnect()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		c.connMu.RLock()
		conn := c.conn
		c.connMu.RUnlock()

		if conn == nil {
			c.logger.Warn("Connection is nil in read pump")
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Error("Unexpected WebSocket close", zap.Error(err))
			} else {
				c.logger.Info("WebSocket connection closed", zap.Error(err))
			}
			return
		}

		c.handleMessage(message)
	}
}

// writePump writes messages to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.logger.Info("Write pump stopped")
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		case message, ok := <-c.send:
			c.connMu.RLock()
			conn := c.conn
			c.connMu.RUnlock()

			if conn == nil {
				return
			}

			conn.SetWriteDeadline(time.Now().Add(writeWait))

			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				c.logger.Error("Failed to write message", zap.Error(err))
				return
			}

		case <-ticker.C:
			c.connMu.RLock()
			conn := c.conn
			c.connMu.RUnlock()

			if conn == nil {
				return
			}

			conn.SetWriteDeadline(time.Now().Add(writeWait))

			// Send Pusher ping
			pingMsg := map[string]string{"event": pusherPing}
			data, _ := json.Marshal(pingMsg)

			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				c.logger.Error("Failed to send ping", zap.Error(err))
				return
			}

			c.logger.Debug("Sent ping to Pusher")
		}
	}
}

// handleMessage processes incoming Pusher messages
func (c *Client) handleMessage(data []byte) {
	var msg PusherMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		c.logger.Error("Failed to unmarshal Pusher message", zap.Error(err))
		return
	}

	c.logger.Debug("Received Pusher message",
		zap.String("event", msg.Event),
		zap.String("channel", msg.Channel),
	)

	switch msg.Event {
	case pusherConnectionEstablished:
		var connData PusherConnectionEstablished
		if err := json.Unmarshal(msg.Data, &connData); err != nil {
			c.logger.Error("Failed to unmarshal connection data", zap.Error(err))
			return
		}
		c.socketID = connData.SocketID
		c.logger.Info("Pusher connection established",
			zap.String("socket_id", connData.SocketID),
			zap.Int("activity_timeout", connData.ActivityTimeout),
		)

	case pusherPong:
		c.logger.Debug("Received pong from Pusher")

	case pusherSubscriptionSucceeded:
		c.logger.Info("Successfully subscribed to channel", zap.String("channel", msg.Channel))

	case pusherError:
		var errMsg PusherErrorMessage
		json.Unmarshal(data, &errMsg)
		c.logger.Error("Pusher error",
			zap.Int("code", errMsg.Code),
			zap.String("message", errMsg.Message),
		)

	case kickChatMessageEvent:
		c.handleChatMessage(msg.Channel, msg.Data)

	default:
		c.logger.Debug("Unhandled Pusher event", zap.String("event", msg.Event))
	}
}

// handleChatMessage processes a Kick chat message
func (c *Client) handleChatMessage(channel string, data json.RawMessage) {
	var chatMsg KickChatMessage
	if err := json.Unmarshal(data, &chatMsg); err != nil {
		c.logger.Error("Failed to unmarshal chat message", zap.Error(err))
		return
	}

	c.logger.Debug("Received chat message",
		zap.String("channel", channel),
		zap.String("sender", chatMsg.Sender.Username),
		zap.String("message", chatMsg.Content),
	)

	// Call message handler
	if c.messageHandler != nil {
		c.messageHandler(channel, &chatMsg)
	}
}

// triggerReconnect triggers a reconnection attempt
func (c *Client) triggerReconnect() {
	c.connMu.Lock()
	c.connected = false
	c.connMu.Unlock()

	select {
	case c.reconnectChan <- struct{}{}:
	default:
		// Reconnect already triggered
	}
}

// ReconnectChan returns the reconnect notification channel
func (c *Client) ReconnectChan() <-chan struct{} {
	return c.reconnectChan
}
