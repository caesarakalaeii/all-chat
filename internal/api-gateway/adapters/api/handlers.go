package api

import (
	"net/http"

	"github.com/caesar/all-chat/internal/api-gateway/adapters/websocket"
	"github.com/caesar/all-chat/pkg/auth"
	"github.com/gin-gonic/gin"
	ws "github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = ws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Configure allowed origins for production
		return true
	},
}

// Handler handles HTTP requests for the API Gateway
type Handler struct {
	hub       *websocket.Hub
	jwtSecret string
	logger    *zap.Logger
}

// NewHandler creates a new HTTP handler
func NewHandler(hub *websocket.Hub, jwtSecret string, logger *zap.Logger) *Handler {
	return &Handler{
		hub:       hub,
		jwtSecret: jwtSecret,
		logger:    logger,
	}
}

// HandleWebSocket upgrades the HTTP connection to WebSocket
func (h *Handler) HandleWebSocket(c *gin.Context) {
	overlayID := c.Param("id")
	token := c.Query("token")

	// Validate JWT token
	claims, err := auth.ValidateToken(token, h.jwtSecret)
	if err != nil {
		h.logger.Warn("Invalid token for WebSocket connection",
			zap.String("overlay_id", overlayID),
			zap.Error(err))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	h.logger.Info("WebSocket connection authorized",
		zap.String("overlay_id", overlayID),
		zap.String("user_id", claims.UserID))

	// Upgrade connection
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade connection",
			zap.String("overlay_id", overlayID),
			zap.Error(err))
		return
	}

	// Create and register client
	client := websocket.NewClient(h.hub, conn, overlayID, h.logger)
	h.hub.Register(client)

	// Start client's read/write pumps
	client.Start()
}

// HandleHealth returns the health status of the gateway
func (h *Handler) HandleHealth(c *gin.Context) {
	stats := h.hub.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"stats":  stats,
	})
}

// HandleReadiness checks if the gateway is ready to accept connections
func (h *Handler) HandleReadiness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}
