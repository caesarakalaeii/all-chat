package handlers

import (
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/kick-listener/channels"
	"github.com/caesar/all-chat/services/kick-listener/publisher"
	"github.com/caesar/all-chat/services/kick-listener/websocket"
	"github.com/gin-gonic/gin"
)

// wsConnectionHealth is the subset of websocket.Client used by the health handler.
// Defined as an interface so that tests can inject a stub without constructing a full
// Pusher WebSocket client.
type wsConnectionHealth interface {
	IsConnected() bool
	LastActivityAt() time.Time
	IsStale() bool
}

// HealthHandler handles health check endpoints
type HealthHandler struct {
	wsConn     wsConnectionHealth
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
		wsConn:     wsClient,
		wsClient:   wsClient,
		publisher:  publisher,
		channelMgr: channelMgr,
	}
}

// LivenessProbe checks if the service is alive (HTTP 200 = alive, 503 = restart needed).
// Returns 503 when the Pusher WebSocket connection has gone silent for longer than the
// stale liveness threshold. This indicates the connection is zombie (read deadline keeps
// being extended but no actual messages are flowing). Kubernetes will restart the pod,
// which cleanly re-establishes the WebSocket connection.
func (h *HealthHandler) LivenessProbe(c *gin.Context) {
	if h.wsConn.IsStale() {
		lastAct := h.wsConn.LastActivityAt()
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "dead",
			"service": "kick-listener",
			"reason":  "Pusher WebSocket zombie — no activity for over 5 minutes",
			"last_activity_seconds_ago": int(time.Since(lastAct).Seconds()),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "alive",
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

	// Check 2: Redis connection
	if !h.publisher.IsHealthy(c.Request.Context()) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"reason": "redis not healthy",
		})
		return
	}

	// Check 3: Active subscriptions match filtered demand count.
	// Phase 06 uses demand-based coordination: channels are only subscribed when an overlay
	// is actively watching. Zero subscriptions is valid when no overlays are connected.
	// We only gate readiness on subscription count when demand is non-nil and > 0
	// (i.e. there are sources with active viewers that we should be subscribed to).
	subscriptionCount := h.channelMgr.GetSubscriptionCount()
	filteredAssignmentCount := h.channelMgr.GetFilteredAssignmentCount()
	if filteredAssignmentCount > 0 && subscriptionCount < filteredAssignmentCount {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":     "not ready",
			"reason":     "subscriptions connecting",
			"expected":   filteredAssignmentCount,
			"subscribed": subscriptionCount,
		})
		return
	}

	// All checks passed
	c.JSON(http.StatusOK, gin.H{
		"status":        "ready",
		"subscriptions": subscriptionCount,
		"demanded":      filteredAssignmentCount,
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
