package websocket

import (
	"context"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/api-gateway/models"
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

// Connection wraps a WebSocket connection for an overlay
type Connection struct {
	conn       *websocket.Conn
	overlayID  string
	userID     string
	send       chan []byte
	logger     *zap.Logger
	mu         sync.Mutex
	closed     bool
	isViewer   bool // True for viewer connections (extension, public viewers)
}

// NewConnection creates a new WebSocket connection for overlay owners
func NewConnection(conn *websocket.Conn, overlayID, userID string, logger *zap.Logger) *Connection {
	return &Connection{
		conn:      conn,
		overlayID: overlayID,
		userID:    userID,
		send:      make(chan []byte, 256),
		logger:    logger,
		isViewer:  false,
	}
}

// NewViewerConnection creates a new WebSocket connection for viewers
// Viewer connections should never receive overlay_id in messages
func NewViewerConnection(conn *websocket.Conn, overlayID, userID string, logger *zap.Logger) *Connection {
	return &Connection{
		conn:      conn,
		overlayID: overlayID,
		userID:    userID,
		send:      make(chan []byte, 256),
		logger:    logger,
		isViewer:  true,
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

	// Handle pong responses
	if msg.Type == models.WSMessageTypePong {
		c.logger.Debug("Received pong",
			zap.String("overlay_id", c.overlayID),
		)
	}
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
