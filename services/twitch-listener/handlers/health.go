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
// Checks: IRC connection, Redis connection, coordinator assignments, active channels
// Per CONTEXT.md: Pod reports ready AFTER successfully connecting to all assigned channels
func (h *HealthHandler) ReadinessProbe(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	checks := make(map[string]interface{})
	ready := true
	reason := ""

	// Check 1: IRC connection established
	ircConnected := h.ircConn.IsConnected()
	checks["irc_connected"] = ircConnected
	if !ircConnected {
		ready = false
		reason = "IRC not connected"
	}

	// Check 2: Redis connection
	redisErr := h.publisher.Ping(ctx)
	checks["redis_connected"] = redisErr == nil
	if redisErr != nil {
		ready = false
		checks["redis_error"] = redisErr.Error()
		if reason == "" {
			reason = "Redis not connected"
		}
	}

	// Check 3: Assignments received from coordinator (TWITCH-06, TWITCH-07)
	assignmentCount := h.chanMgr.GetAssignmentCount()
	checks["assignments"] = assignmentCount
	if assignmentCount == 0 {
		ready = false
		if reason == "" {
			reason = "no assignments from coordinator"
		}
	}

	// Check 4: Active channels match assignment count (all channels connected)
	activeChannelCount := h.chanMgr.GetActiveChannelCount()
	checks["active_channels"] = activeChannelCount
	if activeChannelCount < assignmentCount {
		ready = false
		if reason == "" {
			reason = "channels connecting"
		}
		checks["expected"] = assignmentCount
		checks["connected"] = activeChannelCount
	}

	status := "ready"
	statusCode := http.StatusOK
	if !ready {
		status = "not_ready"
		statusCode = http.StatusServiceUnavailable
		checks["reason"] = reason
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
