package handlers

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/api-gateway/models"
	"github.com/caesar/all-chat/services/api-gateway/subscription"
	wsconn "github.com/caesar/all-chat/services/api-gateway/websocket"
	"github.com/caesar/all-chat/shared/auth"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// WebSocketHandler handles WebSocket connections for overlays
type WebSocketHandler struct {
	wsManager       *wsconn.Manager
	subscriber      *subscription.Subscriber
	repo            *subscription.Repository
	jwtSecret       string
	logger          *zap.Logger
	upgrader        websocket.Upgrader
	allowAllOrigins bool
	allowedOrigins  map[string]struct{}
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(
	wsManager *wsconn.Manager,
	subscriber *subscription.Subscriber,
	repo *subscription.Repository,
	jwtSecret string,
	logger *zap.Logger,
) *WebSocketHandler {
	allowedOrigins, allowAll := loadAllowedOrigins()
	h := &WebSocketHandler{
		wsManager:       wsManager,
		subscriber:      subscriber,
		repo:            repo,
		jwtSecret:       jwtSecret,
		logger:          logger,
		allowedOrigins:  allowedOrigins,
		allowAllOrigins: allowAll,
	}

	h.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     h.checkOrigin,
	}

	if h.allowAllOrigins {
		logger.Info("WebSocket origin allowlist disabled; allowing all origins")
	} else {
		logger.Info("Configured WebSocket origin allowlist",
			zap.Int("count", len(h.allowedOrigins)),
		)
	}

	return h
}

// HandleOverlayConnection handles WebSocket connection requests for overlays
func (h *WebSocketHandler) HandleOverlayConnection(c *gin.Context) {
	overlayID := c.Param("overlay_id")
	if overlayID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "overlay_id is required"})
		return
	}

	// Get JWT token from query parameter (optional for OBS)
	token := c.Query("token")
	var userID string
	var username string

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// If token provided, validate and check ownership
	if token != "" {
		claims, err := auth.ValidateJWT(token, h.jwtSecret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		userID = claims.UserID
		username = claims.Username

		// Verify user owns the overlay
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
	} else {
		// No token provided (OBS mode) - use anonymous connection
		userID = "obs"
		username = "OBS"
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
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade WebSocket",
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)
		return
	}

	// Create WebSocket connection wrapper
	wsConn := wsconn.NewConnection(conn, overlayID, userID, h.logger)

	// Subscribe to overlay's Redis Pub/Sub channel
	// Use background context - subscription must outlive the HTTP request
	if err := h.subscriber.Subscribe(context.Background(), overlayID); err != nil {
		h.logger.Error("Failed to subscribe to overlay channel",
			zap.String("overlay_id", overlayID),
			zap.Error(err),
		)
		conn.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to subscribe"})
		return
	}

	// Add connection to manager
	// Use background context here too since connection lives beyond HTTP request
	h.wsManager.AddConnection(context.Background(), wsConn)

	// Send connected message
	connectedMsg := models.NewConnected(overlayID)
	connectedJSON, _ := connectedMsg.ToJSON()
	wsConn.Send(connectedJSON)

	h.logger.Info("WebSocket connection established",
		zap.String("overlay_id", overlayID),
		zap.String("user_id", userID),
		zap.String("username", username),
	)

	// Create background context for WebSocket connection
	// Don't use the HTTP request context - it gets cancelled when handler returns!
	wsCtx := context.Background()

	// Start connection pumps (runs in goroutines)
	// This will handle read/write until the connection closes
	wsConn.Start(wsCtx)

	// Set up cleanup callback when connection closes
	go func() {
		defer func() {
			if r := recover(); r != nil {
				h.logger.Error("Panic in WebSocket cleanup",
					zap.String("overlay_id", overlayID),
					zap.String("user_id", userID),
					zap.Any("panic", r),
				)
			}
			// Always clean up
			h.wsManager.RemoveConnection(wsConn)
			h.subscriber.Unsubscribe(context.Background(), overlayID)
			h.logger.Info("WebSocket connection closed",
				zap.String("overlay_id", overlayID),
				zap.String("user_id", userID),
			)
		}()

		// Wait for connection to close
		for !wsConn.IsClosed() {
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Return immediately - don't block the HTTP handler
	// The WebSocket connection will continue in the background
}

func loadAllowedOrigins() (map[string]struct{}, bool) {
	value := strings.TrimSpace(os.Getenv("WEBSOCKET_ALLOWED_ORIGINS"))
	if value == "" {
		return nil, true
	}

	allowed := make(map[string]struct{})
	allowAll := false
	for _, origin := range strings.Split(value, ",") {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if origin == "*" {
			allowAll = true
			continue
		}
		allowed[origin] = struct{}{}
	}

	if allowAll {
		return nil, true
	}

	return allowed, false
}

func (h *WebSocketHandler) checkOrigin(r *http.Request) bool {
	if h.allowAllOrigins {
		return true
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	if _, ok := h.allowedOrigins[origin]; ok {
		return true
	}

	h.logger.Warn("Blocked WebSocket connection from disallowed origin",
		zap.String("origin", origin),
	)
	return false
}
