package handlers

import (
	"net/http"

	"github.com/caesar/all-chat/services/kick-listener/channels"
	"github.com/caesar/all-chat/services/kick-listener/publisher"
	"github.com/caesar/all-chat/services/kick-listener/websocket"
	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check endpoints
type HealthHandler struct {
	wsClient   *websocket.Client
	publisher  *publisher.StreamPublisher
	channelMgr *channels.Manager
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(
	wsClient *websocket.Client,
	publisher *publisher.StreamPublisher,
	channelMgr *channels.Manager,
) *HealthHandler {
	return &HealthHandler{
		wsClient:   wsClient,
		publisher:  publisher,
		channelMgr: channelMgr,
	}
}

// LivenessProbe checks if the service is alive
func (h *HealthHandler) LivenessProbe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "kick-listener",
	})
}

// ReadinessProbe checks if the service is ready to handle requests (KICK-05)
func (h *HealthHandler) ReadinessProbe(c *gin.Context) {
	// Check 1: WebSocket connection established
	if !h.channelMgr.IsConnected() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"reason": "WebSocket not connected",
		})
		return
	}

	// Check 2: Assignments received from coordinator
	assignmentCount := h.channelMgr.GetAssignmentCount()
	if assignmentCount == 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"reason": "no assignments from coordinator",
		})
		return
	}

	// Check 3: Active subscriptions match assignment count (all chatrooms subscribed)
	subscriptionCount := h.channelMgr.GetSubscriptionCount()
	if subscriptionCount < assignmentCount {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":      "not ready",
			"reason":      "subscriptions connecting",
			"expected":    assignmentCount,
			"subscribed":  subscriptionCount,
		})
		return
	}

	// Check 4: Redis connection
	if !h.publisher.IsHealthy(c.Request.Context()) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"reason": "redis not healthy",
		})
		return
	}

	// All checks passed
	c.JSON(http.StatusOK, gin.H{
		"status":        "ready",
		"assignments":   assignmentCount,
		"subscriptions": subscriptionCount,
	})
}

// Status returns detailed service status
func (h *HealthHandler) Status(c *gin.Context) {
	subscriptions := h.channelMgr.GetSubscriptions()

	channels := make([]map[string]interface{}, 0)
	for slug, ch := range subscriptions {
		channels = append(channels, map[string]interface{}{
			"channel_slug":  slug,
			"chatroom_id":   ch.ChatroomID,
			"overlay_ids":   ch.OverlayIDs,
			"overlay_count": len(ch.OverlayIDs),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"status":              "running",
		"websocket_connected": h.wsClient.IsConnected(),
		"redis_healthy":       h.publisher.IsHealthy(c.Request.Context()),
		"subscribed_channels": len(subscriptions),
		"channels":            channels,
	})
}
