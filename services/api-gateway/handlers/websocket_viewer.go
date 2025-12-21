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

// ViewerWebSocketHandler handles WebSocket connections for viewers
// Viewers connect via /ws/chat/{streamer_username} without knowing the overlay ID
type ViewerWebSocketHandler struct {
	wsManager  *wsconn.Manager
	subscriber *subscription.Subscriber
	repo       *subscription.Repository
	jwtSecret  string
	logger     *zap.Logger
	upgrader   websocket.Upgrader
}

// NewViewerWebSocketHandler creates a new viewer WebSocket handler
func NewViewerWebSocketHandler(
	wsManager *wsconn.Manager,
	subscriber *subscription.Subscriber,
	repo *subscription.Repository,
	jwtSecret string,
	logger *zap.Logger,
) *ViewerWebSocketHandler {
	h := &ViewerWebSocketHandler{
		wsManager:  wsManager,
		subscriber: subscriber,
		repo:       repo,
		jwtSecret:  jwtSecret,
		logger:     logger.Named("viewer-websocket"),
	}

	// Configure WebSocket upgrader with permissive origin check for viewers
	// Viewers connect from browser extensions and web pages
	h.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// Allow all origins for viewer connections
			// This is safe because viewers can't trigger polling or access secrets
			return true
		},
	}

	return h
}

// HandleViewerChatConnection handles WebSocket connection requests for viewers
// Endpoint: GET /ws/chat/:streamer_username
// Authentication: Optional (viewers can connect anonymously to view chat)
// This endpoint does NOT trigger YouTube polling (unlike /ws/overlay/{id})
func (h *ViewerWebSocketHandler) HandleViewerChatConnection(c *gin.Context) {
	streamerUsername := c.Param("streamer_username")
	if streamerUsername == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "streamer_username is required"})
		return
	}

	// Optional JWT authentication from query parameter
	// Anonymous viewers can connect to view chat, authenticated viewers can send messages
	token := c.Query("token")
	var viewerID string
	var viewerUsername string
	isAuthenticated := false

	if token != "" {
		// Try to validate as viewer JWT
		viewerClaims, err := auth.ValidateViewerJWT(token, h.jwtSecret)
		if err == nil && viewerClaims.IsViewer {
			viewerID = viewerClaims.SessionID
			viewerUsername = viewerClaims.Username
			isAuthenticated = true
			h.logger.Info("Authenticated viewer connecting",
				zap.String("viewer", viewerUsername),
				zap.String("streamer", streamerUsername))
		} else {
			h.logger.Warn("Invalid viewer token provided, connecting anonymously",
				zap.Error(err))
		}
	}

	// If not authenticated, use anonymous identifier
	if !isAuthenticated {
		viewerID = "anonymous"
		viewerUsername = "Anonymous Viewer"
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	// Lookup streamer's public overlay by username
	overlayID, err := h.repo.GetPublicOverlayByUsername(ctx, streamerUsername)
	if err != nil {
		h.logger.Error("Failed to lookup public overlay",
			zap.String("streamer", streamerUsername),
			zap.Error(err),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to lookup streamer"})
		return
	}

	if overlayID == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "streamer not found or has no public overlay configured",
		})
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade WebSocket",
			zap.String("streamer", streamerUsername),
			zap.Error(err),
		)
		return
	}

	// Create WebSocket connection wrapper for viewer (marks as viewer connection)
	// Viewer connections will have overlay_id stripped from all messages
	wsConn := wsconn.NewViewerConnection(conn, overlayID, viewerID, h.logger)

	// Subscribe to overlay's Redis Pub/Sub channel WITHOUT publishing connection event
	// This is critical: viewers should NOT trigger YouTube polling
	if err := h.subscriber.SubscribeViewerOnly(context.Background(), overlayID); err != nil {
		h.logger.Error("Failed to subscribe to overlay channel",
			zap.String("overlay_id", overlayID),
			zap.String("streamer", streamerUsername),
			zap.Error(err),
		)
		conn.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to subscribe"})
		return
	}

	// Add connection to manager (but mark it as viewer connection)
	h.wsManager.AddConnection(context.Background(), wsConn)

	// Send connected message (without overlay_id for security)
	connectedMsg := models.NewViewerConnected()
	connectedJSON, _ := connectedMsg.ToJSON()
	wsConn.Send(connectedJSON)

	h.logger.Info("Viewer WebSocket connection established",
		zap.String("overlay_id", overlayID),
		zap.String("streamer", streamerUsername),
		zap.String("viewer_id", viewerID),
		zap.String("viewer", viewerUsername),
		zap.Bool("authenticated", isAuthenticated),
	)

	// Create background context for WebSocket connection
	wsCtx := context.Background()

	// Start connection pumps (runs in goroutines)
	wsConn.Start(wsCtx)

	// Set up cleanup callback when connection closes
	go func() {
		// Wait for connection to close
		for !wsConn.IsClosed() {
			time.Sleep(100 * time.Millisecond)
		}
		// Clean up when closed
		h.wsManager.RemoveConnection(wsConn)
		// Unsubscribe viewer-only (no disconnection event published)
		h.subscriber.UnsubscribeViewerOnly(context.Background(), overlayID)
		h.logger.Info("Viewer WebSocket connection closed",
			zap.String("overlay_id", overlayID),
			zap.String("streamer", streamerUsername),
			zap.String("viewer", viewerUsername),
		)
	}()

	// Return immediately - WebSocket continues in background
}
