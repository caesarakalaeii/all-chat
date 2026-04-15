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

package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/api-gateway/models"
	"github.com/caesar/all-chat/services/api-gateway/replay"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	// PingInterval is how often to send ping messages
	PingInterval = 30 * time.Second

	// PongWait is how long to wait for a pong response
	PongWait = 60 * time.Second

	// WriteWait is the time allowed to write a message
	WriteWait = 10 * time.Second

	// MaxMessageSize is the maximum message size allowed
	MaxMessageSize = 512 * 1024 // 512 KB
)

// AuthCallback is called when a viewer authenticates via WebSocket message.
// Returns true if the token is valid.
type AuthCallback func(token string) (viewerID, username string, ok bool)

// Connection wraps a WebSocket connection for an overlay
type Connection struct {
	conn            *websocket.Conn
	overlayID       string
	userID          string
	send            chan []byte
	replayBuffer    replay.DeletionReplayBuffer
	logger          *zap.Logger
	mu              sync.Mutex
	closed          bool
	isViewer        bool // True for viewer connections (extension, public viewers)
	authenticated   bool
	onAuth          AuthCallback
}

// NewConnection creates a new WebSocket connection for overlay owners
func NewConnection(conn *websocket.Conn, overlayID, userID string, replayBuffer replay.DeletionReplayBuffer, logger *zap.Logger) *Connection {
	return &Connection{
		conn:         conn,
		overlayID:    overlayID,
		userID:       userID,
		send:         make(chan []byte, 256),
		replayBuffer: replayBuffer,
		logger:       logger,
		isViewer:     false,
	}
}

// NewViewerConnection creates a new WebSocket connection for viewers
// Viewer connections should never receive overlay_id in messages
func NewViewerConnection(conn *websocket.Conn, overlayID, userID string, replayBuffer replay.DeletionReplayBuffer, logger *zap.Logger) *Connection {
	return &Connection{
		conn:         conn,
		overlayID:    overlayID,
		userID:       userID,
		send:         make(chan []byte, 256),
		replayBuffer: replayBuffer,
		logger:       logger,
		isViewer:     true,
	}
}

// IsViewer returns true if this is a viewer connection
func (c *Connection) IsViewer() bool {
	return c.isViewer
}

// Start starts the read and write pumps for the connection
func (c *Connection) Start(ctx context.Context) {
	go c.writePump(ctx)
	go c.readPump(ctx)
}

// Send sends a message to the client
func (c *Connection) Send(message []byte) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return false
	}

	select {
	case c.send <- message:
		return true
	default:
		// Channel full, close connection
		c.logger.Warn("Send channel full, closing connection",
			zap.String("overlay_id", c.overlayID),
			zap.String("user_id", c.userID),
		)
		c.close()
		return false
	}
}

// Close closes the WebSocket connection
func (c *Connection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.close()
}

// close closes the connection (must be called with lock held)
func (c *Connection) close() {
	if c.closed {
		return
	}

	c.closed = true
	close(c.send)
	c.conn.Close()

	c.logger.Info("WebSocket connection closed",
		zap.String("overlay_id", c.overlayID),
		zap.String("user_id", c.userID),
	)
}

// readPump reads messages from the WebSocket connection
func (c *Connection) readPump(ctx context.Context) {
	defer c.Close()

	c.conn.SetReadDeadline(time.Now().Add(PongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(PongWait))
		return nil
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					c.logger.Warn("WebSocket read error",
						zap.String("overlay_id", c.overlayID),
						zap.Error(err),
					)
				}
				return
			}

			// Handle incoming messages (currently just pong)
			c.handleMessage(message)
		}
	}
}

// writePump writes messages to the WebSocket connection
func (c *Connection) writePump(ctx context.Context) {
	ticker := time.NewTicker(PingInterval)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return

		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if !ok {
				// Channel closed
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				c.logger.Warn("WebSocket write error",
					zap.String("overlay_id", c.overlayID),
					zap.Error(err),
				)
				return
			}

		case <-ticker.C:
			// Send WebSocket protocol-level ping
			c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.logger.Warn("Failed to send ping",
					zap.String("overlay_id", c.overlayID),
					zap.Error(err),
				)
				return
			}

			c.logger.Debug("Sent ping",
				zap.String("overlay_id", c.overlayID),
			)
		}
	}
}

// handleMessage processes incoming messages from the client
func (c *Connection) handleMessage(data []byte) {
	// Parse message
	msg, err := models.ParseWSMessage(data)
	if err != nil {
		c.logger.Warn("Failed to parse WebSocket message",
			zap.String("overlay_id", c.overlayID),
			zap.Error(err),
		)
		return
	}

	// Handle different message types
	switch msg.Type {
	case models.WSMessageTypePong:
		c.logger.Debug("Received pong",
			zap.String("overlay_id", c.overlayID),
		)

	case "replay_request":
		c.handleReplayRequest(msg.Data)

	case "auth":
		c.handleAuth(msg.Data)

	default:
		c.logger.Debug("Unhandled message type",
			zap.String("overlay_id", c.overlayID),
			zap.String("type", string(msg.Type)),
		)
	}
}

// handleReplayRequest processes replay request from client
func (c *Connection) handleReplayRequest(data interface{}) {
	// Parse the data as JSON
	var request struct {
		Since int64 `json:"since"` // Unix milliseconds
	}

	// Convert interface{} to JSON and back to struct
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		c.logger.Warn("Invalid replay request data",
			zap.String("overlay_id", c.overlayID),
			zap.Error(err),
		)
		return
	}

	if err := json.Unmarshal(jsonBytes, &request); err != nil {
		c.logger.Warn("Invalid replay request format",
			zap.String("overlay_id", c.overlayID),
			zap.Error(err),
		)
		return
	}

	// Query replay buffer for missed deletions
	deletions, err := c.replayBuffer.GetSince(context.Background(), c.overlayID, request.Since)
	if err != nil {
		c.logger.Error("Failed to retrieve replay buffer",
			zap.String("overlay_id", c.overlayID),
			zap.Error(err),
		)
		return
	}

	if len(deletions) == 0 {
		c.logger.Debug("No missed deletions to replay",
			zap.String("overlay_id", c.overlayID),
			zap.Int64("since", request.Since),
		)
		return
	}

	// Send replay response
	response := models.WSMessage{
		Type:      "replay_response",
		Data:      deletions,
		Timestamp: time.Now().UTC(),
	}
	responseJSON, _ := json.Marshal(response)
	c.Send(responseJSON)

	c.logger.Info("Replayed missed deletions",
		zap.String("overlay_id", c.overlayID),
		zap.Int("count", len(deletions)),
		zap.Int64("since", request.Since),
	)
}

// SetOnAuth sets the callback invoked when a viewer sends an auth message.
func (c *Connection) SetOnAuth(cb AuthCallback) {
	c.onAuth = cb
}

// handleAuth processes an incoming auth message from the client.
func (c *Connection) handleAuth(data interface{}) {
	if c.authenticated {
		c.logger.Debug("Already authenticated, ignoring auth message",
			zap.String("overlay_id", c.overlayID))
		return
	}

	if c.onAuth == nil {
		c.logger.Warn("Auth message received but no auth callback configured")
		return
	}

	var req struct {
		Token string `json:"token"`
	}
	raw, _ := json.Marshal(data)
	if err := json.Unmarshal(raw, &req); err != nil || req.Token == "" {
		resp := models.WSMessage{Type: "auth_error", Data: map[string]string{"error": "invalid auth payload"}, Timestamp: time.Now().UTC()}
		b, _ := json.Marshal(resp)
		c.Send(b)
		return
	}

	viewerID, username, ok := c.onAuth(req.Token)
	if !ok {
		resp := models.WSMessage{Type: "auth_error", Data: map[string]string{"error": "invalid or expired token"}, Timestamp: time.Now().UTC()}
		b, _ := json.Marshal(resp)
		c.Send(b)
		return
	}

	c.mu.Lock()
	c.authenticated = true
	c.userID = viewerID
	c.mu.Unlock()

	resp := models.WSMessage{Type: "auth_success", Data: map[string]string{"viewer_id": viewerID, "username": username}, Timestamp: time.Now().UTC()}
	b, _ := json.Marshal(resp)
	c.Send(b)

	c.logger.Info("Viewer authenticated via WebSocket message",
		zap.String("overlay_id", c.overlayID),
		zap.String("viewer", username))
}

// IsAuthenticated returns whether this connection has been authenticated.
func (c *Connection) IsAuthenticated() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.authenticated
}

// OverlayID returns the overlay ID for this connection
func (c *Connection) OverlayID() string {
	return c.overlayID
}

// UserID returns the user ID for this connection
func (c *Connection) UserID() string {
	return c.userID
}

// IsClosed returns whether the connection is closed
func (c *Connection) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}
