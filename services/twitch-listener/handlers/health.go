package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/twitch-listener/channels"
	"github.com/caesar/all-chat/services/twitch-listener/irc"
	"github.com/caesar/all-chat/services/twitch-listener/publisher"
	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check endpoints
type HealthHandler struct {
	ircConn   *irc.ConnectionManager
	publisher *publisher.StreamPublisher
	chanMgr   *channels.Manager
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(
	ircConn *irc.ConnectionManager,
	pub *publisher.StreamPublisher,
	chanMgr *channels.Manager,
) *HealthHandler {
	return &HealthHandler{
		ircConn:   ircConn,
		publisher: pub,
		chanMgr:   chanMgr,
	}
}

// LivenessProbe checks if the service is alive (HTTP 200 = alive)
func (h *HealthHandler) LivenessProbe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "alive",
		"service": "twitch-listener",
	})
}

// ReadinessProbe checks if the service is ready to handle requests
// Checks: IRC connection, Redis connection, active channels
func (h *HealthHandler) ReadinessProbe(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	checks := make(map[string]interface{})
	ready := true

	// Check IRC connection
	ircConnected := h.ircConn.IsConnected()
	checks["irc_connected"] = ircConnected
	if !ircConnected {
		ready = false
	}

	// Check Redis connection
	redisErr := h.publisher.Ping(ctx)
	checks["redis_connected"] = redisErr == nil
	if redisErr != nil {
		ready = false
		checks["redis_error"] = redisErr.Error()
	}

	// Check active channels
	activeChannels := h.chanMgr.GetActiveChannelCount()
	checks["active_channels"] = activeChannels

	status := "ready"
	statusCode := http.StatusOK
	if !ready {
		status = "not_ready"
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, gin.H{
		"status": status,
		"checks": checks,
	})
}

// Status provides detailed status information
func (h *HealthHandler) Status(c *gin.Context) {
	activeChannels := h.chanMgr.GetActiveChannels()

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"irc": gin.H{
			"connected":       h.ircConn.IsConnected(),
			"active_channels": len(activeChannels),
			"channels":        activeChannels,
		},
	})
}
