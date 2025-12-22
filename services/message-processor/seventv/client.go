package seventv

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
	// EventAPI WebSocket URL
	eventAPIURL = "wss://events.7tv.io/v3"

	// Opcodes
	OpDispatch  = 0
	OpHello     = 1
	OpHeartbeat = 2
	OpReconnect = 4
	OpAck       = 5
	OpError     = 6
	OpEndOfStream = 7
	OpIdentify  = 33
	OpResume    = 34
	OpSubscribe = 35
	OpUnsubscribe = 36
)

// Message represents a 7TV EventAPI message
type Message struct {
	Op int             `json:"op"`
	T  *int64          `json:"t,omitempty"`
	D  json.RawMessage `json:"d,omitempty"`
}

// HelloData contains connection information
type HelloData struct {
	HeartbeatInterval int    `json:"heartbeat_interval"`
	SessionID         string `json:"session_id"`
	SubscriptionLimit int    `json:"subscription_limit"`
}

// SubscribeData is the payload for subscribe operations
type SubscribeData struct {
	Type      string                 `json:"type"`
	Condition map[string]interface{} `json:"condition"`
}

// DispatchData contains event information
type DispatchData struct {
	Type string          `json:"type"`
	Body json.RawMessage `json:"body"`
}

// EmoteSetUpdate represents an emote set update event
type EmoteSetUpdate struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Pushed []struct {
		Key   string `json:"key"`
		Value struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"value"`
	} `json:"pushed,omitempty"`
	Pulled []struct {
		Key   string `json:"key"`
		OldValue struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"old_value"`
	} `json:"pulled,omitempty"`
	Updated []struct {
		Key   string `json:"key"`
		Value struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"value"`
	} `json:"updated,omitempty"`
}

// EventHandler is called when emote set updates are received
type EventHandler func(ctx context.Context, update *EmoteSetUpdate) error

// Client manages the 7TV EventAPI WebSocket connection
type Client struct {
	conn              *websocket.Conn
	logger            *zap.Logger
	sessionID         string
	heartbeatInterval time.Duration
	subscriptions     map[string]bool // track active subscriptions
	eventHandler      EventHandler
	mu                sync.RWMutex
	done              chan struct{}
	reconnectDelay    time.Duration
	reconnecting      bool          // flag to prevent multiple concurrent reconnects
	readLoopDone      chan struct{} // signal when read loop exits
	ready             chan struct{} // signal when connection is ready for subscriptions
	isReady           bool          // ready state flag
	pendingSubscribe  []string      // queue for subscriptions during connection setup
}

// NewClient creates a new 7TV EventAPI client
func NewClient(logger *zap.Logger, handler EventHandler) *Client {
	return &Client{
		logger:           logger,
		subscriptions:    make(map[string]bool),
		eventHandler:     handler,
		done:             make(chan struct{}),
		reconnectDelay:   5 * time.Second,
		readLoopDone:     make(chan struct{}),
		ready:            make(chan struct{}),
		pendingSubscribe: make([]string, 0),
	}
}

// Connect establishes the WebSocket connection and starts listening
func (c *Client) Connect(ctx context.Context) error {
	c.logger.Info("Connecting to 7TV EventAPI", zap.String("url", eventAPIURL))

	// Close old connection if exists and wait for read loop to exit
	c.mu.Lock()
	if c.conn != nil {
		oldConn := c.conn
		c.conn = nil
		c.isReady = false // Mark as not ready
		c.mu.Unlock()
		oldConn.Close()
		// Wait for read loop to fully exit
		<-c.readLoopDone
		c.mu.Lock()
	}

	c.ready = make(chan struct{}) // Reset ready channel
	c.isReady = false              // Mark as not ready
	c.mu.Unlock()

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, eventAPIURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to 7TV EventAPI: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.readLoopDone = make(chan struct{}) // Create new channel for this connection
	c.mu.Unlock()

	c.logger.Info("Connected to 7TV EventAPI")

	// Start message handling
	go c.readMessages(ctx)

	return nil
}

// readMessages reads and processes WebSocket messages
func (c *Client) readMessages(ctx context.Context) {
	defer func() {
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()

		if conn != nil {
			conn.Close()
		}

		// Signal that read loop has exited
		close(c.readLoopDone)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		default:
			// Get connection with lock
			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()

			if conn == nil {
				return
			}

			var msg Message
			if err := conn.ReadJSON(&msg); err != nil {
				c.logger.Error("Failed to read message from 7TV EventAPI",
					zap.Error(err),
				)
				// Attempt reconnection (only if not already reconnecting)
				c.mu.Lock()
				if !c.reconnecting {
					c.reconnecting = true
					c.mu.Unlock()
					go c.reconnect(ctx)
				} else {
					c.mu.Unlock()
				}
				return
			}

			if err := c.handleMessage(ctx, &msg); err != nil {
				c.logger.Error("Failed to handle message",
					zap.Int("opcode", msg.Op),
					zap.Error(err),
				)
			}
		}
	}
}

// handleMessage processes incoming messages based on opcode
func (c *Client) handleMessage(ctx context.Context, msg *Message) error {
	switch msg.Op {
	case OpHello:
		return c.handleHello(ctx, msg.D)
	case OpHeartbeat:
		return c.sendHeartbeat()
	case OpDispatch:
		return c.handleDispatch(ctx, msg.D)
	case OpAck:
		c.logger.Debug("Subscription acknowledged")
		return nil
	case OpError:
		c.logger.Error("Received error from 7TV EventAPI",
			zap.ByteString("data", msg.D),
		)
		return nil
	case OpReconnect:
		c.logger.Info("7TV EventAPI requested reconnection")
		c.mu.Lock()
		if !c.reconnecting {
			c.reconnecting = true
			c.mu.Unlock()
			go c.reconnect(ctx)
		} else {
			c.mu.Unlock()
		}
		return nil
	case OpEndOfStream:
		c.logger.Info("7TV EventAPI stream ended")
		c.mu.Lock()
		if !c.reconnecting {
			c.reconnecting = true
			c.mu.Unlock()
			go c.reconnect(ctx)
		} else {
			c.mu.Unlock()
		}
		return nil
	default:
		c.logger.Warn("Unknown opcode received",
			zap.Int("opcode", msg.Op),
		)
		return nil
	}
}

// handleHello processes the initial HELLO message
func (c *Client) handleHello(ctx context.Context, data json.RawMessage) error {
	var hello HelloData
	if err := json.Unmarshal(data, &hello); err != nil {
		return fmt.Errorf("failed to unmarshal HELLO data: %w", err)
	}

	c.mu.Lock()
	c.sessionID = hello.SessionID
	c.heartbeatInterval = time.Duration(hello.HeartbeatInterval) * time.Millisecond
	c.mu.Unlock()

	c.logger.Info("Received HELLO from 7TV EventAPI",
		zap.String("session_id", hello.SessionID),
		zap.Duration("heartbeat_interval", c.heartbeatInterval),
		zap.Int("subscription_limit", hello.SubscriptionLimit),
	)

	// Start heartbeat loop
	go c.heartbeatLoop(ctx)

	// Mark connection as ready after a small delay
	time.Sleep(100 * time.Millisecond)
	c.mu.Lock()
	c.isReady = true
	oldReady := c.ready
	c.ready = make(chan struct{})
	c.mu.Unlock()
	close(oldReady) // Signal that connection is ready

	// Process pending subscriptions
	c.mu.Lock()
	pending := c.pendingSubscribe
	c.pendingSubscribe = make([]string, 0)
	c.mu.Unlock()

	for _, emoteSetID := range pending {
		time.Sleep(50 * time.Millisecond) // Rate limit subscriptions
		if err := c.subscribeNow(emoteSetID); err != nil {
			c.logger.Error("Failed to send pending subscription",
				zap.String("emote_set_id", emoteSetID),
				zap.Error(err))
		}
	}

	return nil
}

// heartbeatLoop sends periodic heartbeats to keep the connection alive
func (c *Client) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(c.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case <-ticker.C:
			if err := c.sendHeartbeat(); err != nil {
				c.logger.Error("Failed to send heartbeat", zap.Error(err))
			}
		}
	}
}

// sendHeartbeat sends a heartbeat message to the server
func (c *Client) sendHeartbeat() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return fmt.Errorf("connection not established")
	}

	msg := Message{
		Op: OpHeartbeat,
	}

	return c.conn.WriteJSON(msg)
}

// handleDispatch processes DISPATCH events
func (c *Client) handleDispatch(ctx context.Context, data json.RawMessage) error {
	var dispatch DispatchData
	if err := json.Unmarshal(data, &dispatch); err != nil {
		return fmt.Errorf("failed to unmarshal DISPATCH data: %w", err)
	}

	c.logger.Debug("Received DISPATCH event",
		zap.String("type", dispatch.Type),
	)

	// Handle emote set updates
	if dispatch.Type == "emote_set.update" {
		var update EmoteSetUpdate
		if err := json.Unmarshal(dispatch.Body, &update); err != nil {
			return fmt.Errorf("failed to unmarshal emote set update: %w", err)
		}

		if c.eventHandler != nil {
			if err := c.eventHandler(ctx, &update); err != nil {
				c.logger.Error("Event handler failed",
					zap.String("emote_set_id", update.ID),
					zap.Error(err),
				)
			}
		}
	}

	return nil
}

// Subscribe subscribes to emote set updates for a specific emote set ID
func (c *Client) Subscribe(ctx context.Context, emoteSetID string) error {
	c.mu.RLock()
	isReady := c.isReady
	c.mu.RUnlock()

	// If not ready, queue the subscription
	if !isReady {
		c.mu.Lock()
		// Double-check after acquiring write lock
		if !c.isReady {
			c.pendingSubscribe = append(c.pendingSubscribe, emoteSetID)
			c.mu.Unlock()
			c.logger.Debug("Queued subscription until connection ready",
				zap.String("emote_set_id", emoteSetID))
			return nil
		}
		// If became ready while we were acquiring the lock, continue
		c.mu.Unlock()
	}

	// Connection is ready, subscribe immediately
	return c.subscribeNow(emoteSetID)
}

// subscribeNow sends the subscription immediately without checks
func (c *Client) subscribeNow(emoteSetID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("connection not established")
	}

	// Check if already subscribed
	if c.subscriptions[emoteSetID] {
		c.logger.Debug("Already subscribed to emote set",
			zap.String("emote_set_id", emoteSetID),
		)
		return nil
	}

	subscribeData := SubscribeData{
		Type: "emote_set.update",
		Condition: map[string]interface{}{
			"object_id": emoteSetID,
		},
	}

	data, err := json.Marshal(subscribeData)
	if err != nil {
		return fmt.Errorf("failed to marshal subscribe data: %w", err)
	}

	msg := Message{
		Op: OpSubscribe,
		D:  data,
	}

	if err := c.conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("failed to send subscribe message: %w", err)
	}

	c.subscriptions[emoteSetID] = true

	c.logger.Info("Subscribed to emote set updates",
		zap.String("emote_set_id", emoteSetID),
	)

	return nil
}

// Unsubscribe unsubscribes from emote set updates for a specific emote set ID
func (c *Client) Unsubscribe(ctx context.Context, emoteSetID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("connection not established")
	}

	// Check if not subscribed
	if !c.subscriptions[emoteSetID] {
		return nil
	}

	subscribeData := SubscribeData{
		Type: "emote_set.update",
		Condition: map[string]interface{}{
			"object_id": emoteSetID,
		},
	}

	data, err := json.Marshal(subscribeData)
	if err != nil {
		return fmt.Errorf("failed to marshal unsubscribe data: %w", err)
	}

	msg := Message{
		Op: OpUnsubscribe,
		D:  data,
	}

	if err := c.conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("failed to send unsubscribe message: %w", err)
	}

	delete(c.subscriptions, emoteSetID)

	c.logger.Info("Unsubscribed from emote set updates",
		zap.String("emote_set_id", emoteSetID),
	)

	return nil
}

// reconnect attempts to reconnect to the 7TV EventAPI
func (c *Client) reconnect(ctx context.Context) {
	defer func() {
		c.mu.Lock()
		c.reconnecting = false
		c.mu.Unlock()
	}()

	c.logger.Info("Attempting to reconnect to 7TV EventAPI",
		zap.Duration("delay", c.reconnectDelay),
	)

	time.Sleep(c.reconnectDelay)

	if err := c.Connect(ctx); err != nil {
		c.logger.Error("Failed to reconnect", zap.Error(err))
		// Exponential backoff
		c.mu.Lock()
		c.reconnectDelay = c.reconnectDelay * 2
		if c.reconnectDelay > 5*time.Minute {
			c.reconnectDelay = 5 * time.Minute
		}
		c.mu.Unlock()

		// Try again
		time.Sleep(c.reconnectDelay)
		c.reconnect(ctx)
		return
	}

	// Reset reconnect delay on successful connection
	c.mu.Lock()
	c.reconnectDelay = 5 * time.Second
	c.mu.Unlock()

	// Resubscribe to all previous subscriptions
	c.mu.RLock()
	subscriptions := make([]string, 0, len(c.subscriptions))
	for emoteSetID := range c.subscriptions {
		subscriptions = append(subscriptions, emoteSetID)
	}
	c.mu.RUnlock()

	// Clear subscriptions map (Subscribe will re-add them)
	c.mu.Lock()
	c.subscriptions = make(map[string]bool)
	c.mu.Unlock()

	for _, emoteSetID := range subscriptions {
		if err := c.Subscribe(ctx, emoteSetID); err != nil {
			c.logger.Error("Failed to resubscribe",
				zap.String("emote_set_id", emoteSetID),
				zap.Error(err),
			)
		}
	}
}

// Close closes the WebSocket connection
func (c *Client) Close() error {
	close(c.done)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}
