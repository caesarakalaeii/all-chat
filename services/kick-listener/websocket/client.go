package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/kick-listener/metrics"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	// Pusher event types
	pusherConnectionEstablished = "pusher:connection_established"
	pusherPing                  = "pusher:ping"
	pusherPong                  = "pusher:pong"
	pusherError                 = "pusher:error"
	pusherSubscribe             = "pusher:subscribe"
	pusherSubscriptionSucceeded = "pusher_internal:subscription_succeeded"
	pusherUnsubscribe           = "pusher:unsubscribe"

	// Kick chat events
	kickChatMessageEvent       = "App\\Events\\ChatMessageSentEvent"
	kickChatMessageEventAlt    = "App\\Events\\ChatMessageEvent" // Alternative event name
	kickMessageDeletedEvent    = "App\\Events\\ChatMessageDeletedEvent"
	kickMessageDeletedEventAlt = "App\\Events\\MessageDeletedEvent" // Alternative event name

	// Kick channel format (chatrooms.<chatroom_id>.v2)
	kickChannelFormat = "chatrooms.%d.v2"

	// Connection settings
	writeWait      = 10 * time.Second
	pongWait       = 150 * time.Second // Must be > Pusher's activity_timeout (120s)
	pingPeriod     = 30 * time.Second
	maxMessageSize = 512 * 1024 // 512 KB

	defaultProtocolVersion = 7
	defaultClientName      = "js"
	defaultClientVersion   = "8.4.0-rc2"

	// staleLivenessThreshold is how long without any WebSocket activity (Pusher ping/pong
	// or chat message) before the liveness probe returns 503. The Pusher read deadline is
	// 150 s; 5 minutes is well above that, so a healthy connection will always have reset
	// lastActivityAt before this threshold is reached.
	staleLivenessThreshold = 5 * time.Minute
)

// Config controls how the Pusher client connects
type Config struct {
	AppKey        string
	Clusters      []string
	Protocol      int
	ClientName    string
	ClientVersion string
	FlashEnabled  bool
}

func (cfg Config) withDefaults() Config {
	if cfg.Protocol == 0 {
		cfg.Protocol = defaultProtocolVersion
	}
	if cfg.ClientName == "" {
		cfg.ClientName = defaultClientName
	}
	if cfg.ClientVersion == "" {
		cfg.ClientVersion = defaultClientVersion
	}
	cfg.Clusters = sanitizeClusters(cfg.Clusters)
	if len(cfg.Clusters) == 0 {
		cfg.Clusters = []string{"us2"}
	}
	return cfg
}

func sanitizeClusters(in []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(in))
	for _, cluster := range in {
		cluster = strings.TrimSpace(cluster)
		if cluster == "" {
			continue
		}
		if _, ok := seen[cluster]; ok {
			continue
		}
		seen[cluster] = struct{}{}
		result = append(result, cluster)
	}
	return result
}

func (cfg Config) urlForCluster(cluster string) string {
	flashParam := "false"
	if cfg.FlashEnabled {
		flashParam = "true"
	}
	return fmt.Sprintf("wss://ws-%s.pusher.com/app/%s?protocol=%d&client=%s&version=%s&flash=%s",
		cluster,
		cfg.AppKey,
		cfg.Protocol,
		cfg.ClientName,
		cfg.ClientVersion,
		flashParam,
	)
}

// MessageHandler is called when a chat message is received
type MessageHandler func(channel string, message *KickChatMessage)

// DeletionHandler is called when a message deletion event is received
type DeletionHandler func(channel string, event *KickMessageDeletedEvent)

// Client represents a Pusher WebSocket client for Kick
type Client struct {
	conn            *websocket.Conn
	connMu          sync.RWMutex
	writeMu         sync.Mutex // Protects concurrent writes to WebSocket
	handlerMu       sync.RWMutex
	logger          *zap.Logger
	messageHandler  MessageHandler
	deletionHandler DeletionHandler
	config          Config
	clusterOrder    []string
	activeCluster   string

	// Channel subscriptions
	subscribedChannels map[string]int
	channelsMu         sync.RWMutex

	// Connection state
	socketID       string
	connected      bool
	handshakeReady bool
	reconnecting   bool

	// Control channels
	send          chan []byte
	done          chan struct{}
	reconnectChan chan struct{}

	ctx    context.Context
	cancel context.CancelFunc

	// Activity tracking for zombie connection detection
	lastActivityAt time.Time
}

// NewClient creates a new Pusher WebSocket client
func NewClient(cfg Config, messageHandler MessageHandler, logger *zap.Logger) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	cfg = cfg.withDefaults()

	return &Client{
		logger:             logger,
		messageHandler:     messageHandler,
		config:             cfg,
		clusterOrder:       append([]string{}, cfg.Clusters...),
		subscribedChannels: make(map[string]int),
		send:               make(chan []byte, 256),
		done:               make(chan struct{}),
		reconnectChan:      make(chan struct{}, 1),
		ctx:                ctx,
		cancel:             cancel,
	}
}

// SetDeletionHandler sets the deletion event handler
func (c *Client) SetDeletionHandler(handler DeletionHandler) {
	c.handlerMu.Lock()
	defer c.handlerMu.Unlock()
	c.deletionHandler = handler
}

// Connect establishes connection to Pusher WebSocket
func (c *Client) Connect() error {
	if c.config.AppKey == "" {
		return fmt.Errorf("pusher app key is required")
	}

	attempts := c.clusterAttempts()
	var lastErr error

	for _, cluster := range attempts {
		url := c.config.urlForCluster(cluster)
		c.logger.Info("Connecting to Kick Pusher WebSocket",
			zap.String("cluster", cluster),
			zap.String("url", url),
		)

		if err := c.connectToCluster(url); err != nil {
			lastErr = err
			c.logger.Warn("Failed to connect to Kick Pusher WebSocket",
				zap.String("cluster", cluster),
				zap.Error(err),
			)
			continue
		}

		c.activeCluster = cluster
		c.logger.Info("Connected to Kick Pusher WebSocket",
			zap.String("cluster", cluster),
		)
		return nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no Pusher clusters available")
	}

	return fmt.Errorf("failed to connect to any Pusher cluster: %w", lastErr)
}

func (c *Client) connectToCluster(url string) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	c.connMu.Lock()
	c.handshakeReady = false
	c.connMu.Unlock()

	conn, _, err := dialer.Dial(url, nil)
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

	return nil
}

func (c *Client) clusterAttempts() []string {
	order := append([]string{}, c.clusterOrder...)
	if c.activeCluster == "" || len(order) == 0 {
		return order
	}

	attempts := []string{c.activeCluster}
	for _, cluster := range order {
		if cluster == c.activeCluster {
			continue
		}
		attempts = append(attempts, cluster)
	}
	return attempts
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
		c.writeMu.Lock()
		err := c.conn.WriteMessage(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		)
		c.writeMu.Unlock()
		if err != nil {
			c.logger.Warn("Error sending close message", zap.Error(err))
		}

		c.conn.Close()
		c.conn = nil
	}

	c.connected = false
	c.handshakeReady = false
	metrics.SetSocketConnected(false)
	c.logger.Info("Disconnected from Kick Pusher WebSocket")
	return nil
}

// Subscribe subscribes to a Kick chatroom channel
func (c *Client) Subscribe(chatroomID int) error {
	channel := c.formatChannelName(chatroomID)

	c.channelsMu.Lock()
	if _, exists := c.subscribedChannels[channel]; exists {
		c.channelsMu.Unlock()
		c.logger.Debug("Already subscribed to channel", zap.String("channel", channel))
		return nil
	}
	c.subscribedChannels[channel] = chatroomID
	c.channelsMu.Unlock()

	if !c.isReady() {
		c.logger.Debug("Connection not ready yet, deferring subscription",
			zap.String("channel", channel),
			zap.Int("chatroom_id", chatroomID),
		)
		return nil
	}

	return c.sendSubscribe(channel, chatroomID, nil, false)
}

// SubscribeWithAuth subscribes to a Kick chatroom channel with authentication token
func (c *Client) SubscribeWithAuth(chatroomID int, authToken string) error {
	channel := c.formatChannelName(chatroomID)

	c.channelsMu.Lock()
	if _, exists := c.subscribedChannels[channel]; exists {
		c.channelsMu.Unlock()
		c.logger.Debug("Already subscribed to channel", zap.String("channel", channel))
		return nil
	}
	c.subscribedChannels[channel] = chatroomID
	c.channelsMu.Unlock()

	if !c.isReady() {
		c.logger.Debug("Connection not ready yet, deferring subscription",
			zap.String("channel", channel),
			zap.Int("chatroom_id", chatroomID),
		)
		return nil
	}

	return c.sendSubscribe(channel, chatroomID, &authToken, false)
}

// Unsubscribe unsubscribes from a Kick chatroom channel
func (c *Client) Unsubscribe(chatroomID int) error {
	channel := c.formatChannelName(chatroomID)

	c.channelsMu.Lock()
	if _, exists := c.subscribedChannels[channel]; !exists {
		c.channelsMu.Unlock()
		c.logger.Debug("Not subscribed to channel", zap.String("channel", channel))
		return nil
	}
	delete(c.subscribedChannels, channel)
	c.channelsMu.Unlock()

	if !c.isReady() {
		c.logger.Debug("Connection not ready yet, unsubscribe deferred",
			zap.String("channel", channel),
			zap.Int("chatroom_id", chatroomID),
		)
		return nil
	}

	return c.sendUnsubscribe(channel, chatroomID)
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.connected
}

// GetSocketID returns the current Pusher socket ID (needed for channel auth)
func (c *Client) GetSocketID() string {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.socketID
}

// recordActivity updates the last activity timestamp.
// Called on every successful WebSocket read and on every chat/deletion message received.
func (c *Client) recordActivity() {
	c.connMu.Lock()
	c.lastActivityAt = time.Now()
	c.connMu.Unlock()
}

// LastActivityAt returns when the last WebSocket message was received.
// Used by health checks to detect silently dead connections.
func (c *Client) LastActivityAt() time.Time {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.lastActivityAt
}

// IsStale returns true when the WebSocket connection has gone silent for longer than
// staleLivenessThreshold. A zero lastActivityAt (never connected) returns false so
// the pod is not killed before the initial connection is established.
// Used by the liveness probe to trigger a Kubernetes pod restart when the Pusher
// connection is zombie (read deadline extended but no actual messages flowing).
func (c *Client) IsStale() bool {
	c.connMu.RLock()
	last := c.lastActivityAt
	c.connMu.RUnlock()
	if last.IsZero() {
		return false
	}
	return time.Since(last) > staleLivenessThreshold
}

// formatChannelName returns a Kick chat channel string
func (c *Client) formatChannelName(chatroomID int) string {
	return fmt.Sprintf(kickChannelFormat, chatroomID)
}

// isReady returns true when the connection is established and handshake completed
func (c *Client) isReady() bool {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.connected && c.handshakeReady
}

func (c *Client) sendSubscribe(channel string, chatroomID int, authToken *string, resubscribe bool) error {
	subscribeMsg := PusherSubscribe{
		Event: pusherSubscribe,
		Data: PusherSubscribeData{
			Channel: channel,
			Auth:    authToken,
		},
	}

	data, err := json.Marshal(subscribeMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal subscribe message: %w", err)
	}

	action := "Subscribing"
	if resubscribe {
		action = "Re-subscribing"
	}

	c.logger.Info(fmt.Sprintf("%s to Kick channel", action),
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

func (c *Client) sendUnsubscribe(channel string, chatroomID int) error {
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

func (c *Client) resubscribeAll() {
	if !c.isReady() {
		return
	}

	c.channelsMu.RLock()
	defer c.channelsMu.RUnlock()

	for channel, chatroomID := range c.subscribedChannels {
		if err := c.sendSubscribe(channel, chatroomID, nil, true); err != nil {
			c.logger.Error("Failed to re-subscribe to channel",
				zap.String("channel", channel),
				zap.Int("chatroom_id", chatroomID),
				zap.Error(err),
			)
		}
	}
}

func (c *Client) sendControlEvent(event string) error {
	controlMsg := map[string]string{"event": event}
	data, err := json.Marshal(controlMsg)
	if err != nil {
		return err
	}

	select {
	case c.send <- data:
		return nil
	case <-c.ctx.Done():
		return fmt.Errorf("client is shutting down")
	}
}

func (c *Client) handleConnectionEstablished(data json.RawMessage) {
	var connData PusherConnectionEstablished

	// Pusher may send data as a JSON-encoded string, so try to unmarshal twice
	var dataStr string
	if err := json.Unmarshal(data, &dataStr); err == nil {
		// Data is a string, unmarshal the string as JSON
		if err := json.Unmarshal([]byte(dataStr), &connData); err != nil {
			c.logger.Error("Failed to unmarshal connection data from string", zap.Error(err))
			return
		}
	} else {
		// Data is already an object, unmarshal directly
		if err := json.Unmarshal(data, &connData); err != nil {
			c.logger.Error("Failed to unmarshal connection data", zap.Error(err))
			return
		}
	}

	c.connMu.Lock()
	c.socketID = connData.SocketID
	c.handshakeReady = true
	c.connMu.Unlock()

	c.logger.Info("Pusher connection established",
		zap.String("socket_id", connData.SocketID),
		zap.Int("activity_timeout", connData.ActivityTimeout),
	)

	metrics.SetSocketConnected(true)
	c.resubscribeAll()
}

func (c *Client) handlePusherError(data json.RawMessage) {
	var errMsg PusherErrorMessage
	if err := json.Unmarshal(data, &errMsg); err != nil {
		c.logger.Error("Failed to unmarshal Pusher error", zap.Error(err))
		return
	}

	c.logger.Error("Pusher error received",
		zap.Int("code", errMsg.Code),
		zap.String("message", errMsg.Message),
	)

	c.forceReconnect(fmt.Sprintf("pusher error %d", errMsg.Code))
}

// readPump reads messages from the WebSocket connection
func (c *Client) readPump() {
	defer func() {
		c.logger.Info("Read pump stopped")
		c.triggerReconnect("read loop stopped")
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

		// Record activity on every successful read so the liveness probe and any
		// future watchdog logic can detect a silently dead connection.
		c.recordActivity()

		// Reset read deadline on every message (Pusher uses application-level ping/pong, not WebSocket frames)
		conn.SetReadDeadline(time.Now().Add(pongWait))

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

			c.writeMu.Lock()
			conn.SetWriteDeadline(time.Now().Add(writeWait))

			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				c.writeMu.Unlock()
				return
			}

			err := conn.WriteMessage(websocket.TextMessage, message)
			c.writeMu.Unlock()

			if err != nil {
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

			// Send Pusher ping
			pingMsg := map[string]string{"event": pusherPing}
			data, _ := json.Marshal(pingMsg)

			c.writeMu.Lock()
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			err := conn.WriteMessage(websocket.TextMessage, data)
			c.writeMu.Unlock()

			if err != nil {
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
		c.handleConnectionEstablished(msg.Data)

	case pusherPong:
		c.logger.Debug("Received pong from Pusher")

	case pusherPing:
		c.logger.Debug("Received ping from Pusher")
		if err := c.sendControlEvent(pusherPong); err != nil {
			c.logger.Error("Failed to send pong response", zap.Error(err))
		}

	case pusherSubscriptionSucceeded:
		c.logger.Info("Successfully subscribed to channel", zap.String("channel", msg.Channel))

	case pusherError:
		c.handlePusherError(msg.Data)

	case kickChatMessageEvent, kickChatMessageEventAlt:
		c.handleChatMessage(msg.Channel, msg.Data)

	case kickMessageDeletedEvent, kickMessageDeletedEventAlt:
		c.handleMessageDeleted(msg.Channel, msg.Data)

	default:
		// Log any unhandled events containing "delete" or "Delete" for validation
		if strings.Contains(strings.ToLower(msg.Event), "delete") {
			c.logger.Warn("Unhandled deletion-related Pusher event",
				zap.String("event", msg.Event),
				zap.String("channel", msg.Channel),
			)
		}
		c.logger.Debug("Unhandled Pusher event", zap.String("event", msg.Event))
	}
}

// handleChatMessage processes a Kick chat message
func (c *Client) handleChatMessage(channel string, data json.RawMessage) {
	var chatMsg KickChatMessage

	// Pusher may send data as a JSON-encoded string, so try to unmarshal twice
	var dataStr string
	if err := json.Unmarshal(data, &dataStr); err == nil {
		// Data is a string, unmarshal the string as JSON
		if err := json.Unmarshal([]byte(dataStr), &chatMsg); err != nil {
			c.logger.Error("Failed to unmarshal chat message from string", zap.Error(err))
			return
		}
	} else {
		// Data is already an object, unmarshal directly
		if err := json.Unmarshal(data, &chatMsg); err != nil {
			c.logger.Error("Failed to unmarshal chat message", zap.Error(err))
			return
		}
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

// handleMessageDeleted processes a Kick message deletion event
func (c *Client) handleMessageDeleted(channel string, data json.RawMessage) {
	var deletedEvent KickMessageDeletedEvent

	// Pusher may send data as a JSON-encoded string, so try to unmarshal twice
	var dataStr string
	if err := json.Unmarshal(data, &dataStr); err == nil {
		// Data is a string, unmarshal the string as JSON
		if err := json.Unmarshal([]byte(dataStr), &deletedEvent); err != nil {
			c.logger.Error("Failed to unmarshal deletion event from string", zap.Error(err))
			return
		}
	} else {
		// Data is already an object, unmarshal directly
		if err := json.Unmarshal(data, &deletedEvent); err != nil {
			c.logger.Error("Failed to unmarshal deletion event", zap.Error(err))
			return
		}
	}

	c.logger.Debug("Received message deletion",
		zap.String("channel", channel),
		zap.String("deleted_message_id", deletedEvent.DeletedMessage.ID),
		zap.Int("deleted_by", deletedEvent.DeletedMessage.DeletedBy),
	)

	// Call deletion handler (wired in main.go)
	c.handlerMu.RLock()
	handler := c.deletionHandler
	c.handlerMu.RUnlock()

	if handler != nil {
		handler(channel, &deletedEvent)
	}
}

// triggerReconnect triggers a reconnection attempt
func (c *Client) forceReconnect(reason string) {
	c.connMu.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.connMu.Unlock()

	c.triggerReconnect(reason)
}

// triggerReconnect triggers a reconnection attempt
func (c *Client) triggerReconnect(reason string) {
	if c.ctx.Err() != nil {
		// Service shutting down; skip reconnection.
		return
	}

	c.connMu.Lock()
	c.connected = false
	c.handshakeReady = false
	c.connMu.Unlock()

	c.logger.Warn("Scheduling Kick Pusher reconnect", zap.String("reason", reason))
	metrics.SetSocketConnected(false)
	metrics.IncReconnect(reason)

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
