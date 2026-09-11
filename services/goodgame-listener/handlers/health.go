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

package handlers

import (
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/goodgame-listener/channels"
	"github.com/caesar/all-chat/services/goodgame-listener/publisher"
	"github.com/caesar/all-chat/services/goodgame-listener/websocket"
	"github.com/gin-gonic/gin"
)

// wsConnectionHealth is the subset of websocket.Client used by the health
// handler. An interface so tests can inject a stub.
type wsConnectionHealth interface {
	IsConnected() bool
	LastActivityAt() time.Time
	IsStale() bool
}

// HealthHandler handles health check endpoints.
type HealthHandler struct {
	wsConn     wsConnectionHealth
	wsClient   *websocket.Client
	publisher  *publisher.StreamPublisher
	channelMgr *channels.Manager
}

// NewHealthHandler creates a new health handler.
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

// LivenessProbe returns 503 when the connection went silent beyond the stale
// threshold (zombie connection; Kubernetes restarts the pod).
func (h *HealthHandler) LivenessProbe(c *gin.Context) {
	if h.wsConn.IsStale() {
		lastAct := h.wsConn.LastActivityAt()
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":                    "dead",
			"service":                   "goodgame-listener",
			"reason":                    "GoodGame WebSocket zombie — no activity for over 5 minutes",
			"last_activity_seconds_ago": int(time.Since(lastAct).Seconds()),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "alive",
		"service": "goodgame-listener",
	})
}

// ReadinessProbe checks connection state and publisher health.
func (h *HealthHandler) ReadinessProbe(c *gin.Context) {
	if !h.channelMgr.IsConnected() {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"reason": "WebSocket not connected",
		})
		return
	}

	if !h.publisher.IsHealthy(c.Request.Context()) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"reason": "redis not healthy",
		})
		return
	}

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

	c.JSON(http.StatusOK, gin.H{
		"status":        "ready",
		"subscriptions": subscriptionCount,
		"demanded":      filteredAssignmentCount,
	})
}

// Status returns detailed service status.
func (h *HealthHandler) Status(c *gin.Context) {
	activeChannels := h.channelMgr.GetActiveChannels()

	c.JSON(http.StatusOK, gin.H{
		"status":              "running",
		"websocket_connected": h.wsClient.IsConnected(),
		"redis_healthy":       h.publisher.IsHealthy(c.Request.Context()),
		"subscribed_channels": len(activeChannels),
		"channels":            activeChannels,
	})
}
