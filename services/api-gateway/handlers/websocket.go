package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/api-gateway/models"
	"github.com/caesar/all-chat/services/api-gateway/subscription"
	wsconn "github.com/caesar/all-chat/services/api-gateway/websocket"
	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins in development
		// TODO: Configure allowed origins for production
		return true
	},
}

// WebSocketHandler handles WebSocket connections for overlays
type WebSocketHandler struct {
	wsManager  *wsconn.Manager
	subscriber *subscription.Subscriber
	repo       *subscription.Repository
	jwtSecret  string
	logger     *zap.Logger
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(
	wsManager *wsconn.Manager,
	subscriber *subscription.Subscriber,
	repo *subscription.Repository,
	jwtSecret string,
	logger *zap.Logger,
) *WebSocketHandler {
	return &WebSocketHandler{
		wsManager:  wsManager,
		subscriber: subscriber,
		repo:       repo,
		jwtSecret:  jwtSecret,
		logger:     logger,
	}
}

// HandleOverlayConnection handles WebSocket connection requests for overlays
func (h *WebSocketHandler) HandleOverlayConnection(c *gin.Context) {
	overlayID := c.Param("overlay_id")
	if overlayID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "overlay_id is required"})
		return
	}

	// Get JWT token from query parameter
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token is required"})
		return
	}

	// Validate JWT
	claims, err := auth.ValidateJWT(token, h.jwtSecret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return
	}

	// Verify user owns the overlay
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	owns, err := h.repo.VerifyOverlayOwnership(ctx, overlayID, claims.UserID)
	if err != nil {
		h.logger.Error("Failed to verify overlay ownership",
			zap.String("overlay_id", overlayID),
			zap.String("user_id", claims.UserID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify ownership"})
		return
	}

	if !owns {
		c.JSON(http.StatusForbidden, gin.H{"error": "you do not own this overlay"})
		return
	}

	// Check if overlay is active
	isActive, err := h.repo.IsOverlayActive(ctx, overlayID)
	if err != nil {
		h.logger.Error("Failed to check overlay status",
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check overlay status"})
		return
	}

	if !isActive {
		c.JSON(http.StatusForbidden, gin.H{"error": "overlay is not active"})
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade WebSocket",
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)
		return
	}

	// Create WebSocket connection wrapper
	wsConn := wsconn.NewConnection(conn, overlayID, claims.UserID, h.logger)

	// Subscribe to overlay's Redis Pub/Sub channel
	if err := h.subscriber.Subscribe(ctx, overlayID); err != nil {
		h.logger.Error("Failed to subscribe to overlay channel",
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)
		conn.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to subscribe"})
		return
	}

	// Add connection to manager
	h.wsManager.AddConnection(ctx, wsConn)

	// Send connected message
	connectedMsg := models.NewConnected(overlayID)
	connectedJSON, _ := connectedMsg.ToJSON()
	wsConn.Send(connectedJSON)

	h.logger.Info("WebSocket connection established",
		zap.String("overlay_id", overlayID),
		zap.String("user_id", claims.UserID),
		zap.String("username", claims.Username),
	)

	// Start connection pumps (runs in goroutines)
	// This will handle read/write until the connection closes
	// Cleanup is handled by the connection's close callback
	wsConn.Start(ctx)

	// Set up cleanup callback when connection closes
	go func() {
		// Wait for connection to close
		for !wsConn.IsClosed() {
			time.Sleep(100 * time.Millisecond)
		}
		// Clean up when closed
		h.wsManager.RemoveConnection(wsConn)
		h.subscriber.Unsubscribe(context.Background(), overlayID)
		h.logger.Info("WebSocket connection closed",
			zap.String("overlay_id", overlayID),
			zap.String("user_id", claims.UserID),
		)
	}()

	// Return immediately - don't block the HTTP handler
	// The WebSocket connection will continue in the background
}
