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

	"github.com/caesar/all-chat/services/picarto-listener/channels"
	"github.com/caesar/all-chat/services/picarto-listener/publisher"
	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check endpoints.
type HealthHandler struct {
	publisher  *publisher.StreamPublisher
	channelMgr *channels.Manager
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(
	publisher *publisher.StreamPublisher,
	channelMgr *channels.Manager,
) *HealthHandler {
	return &HealthHandler{
		publisher:  publisher,
		channelMgr: channelMgr,
	}
}

// LivenessProbe checks if the service is alive.
func (h *HealthHandler) LivenessProbe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "alive",
		"service": "picarto-listener",
	})
}

// ReadinessProbe checks if the service can handle requests.
// Zero subscriptions is valid when no overlays demand any Picarto channel.
func (h *HealthHandler) ReadinessProbe(c *gin.Context) {
	if !h.publisher.IsHealthy(c.Request.Context()) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"reason": "redis not healthy",
		})
		return
	}

	subscriptionCount := h.channelMgr.GetSubscriptionCount()
	filteredCount := h.channelMgr.GetFilteredAssignmentCount()
	if filteredCount > 0 && subscriptionCount < filteredCount {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":     "not ready",
			"reason":     "connections establishing",
			"expected":   filteredCount,
			"subscribed": subscriptionCount,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":        "ready",
		"subscriptions": subscriptionCount,
		"demanded":      filteredCount,
	})
}

// Status returns detailed service status.
func (h *HealthHandler) Status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":              "running",
		"redis_healthy":       h.publisher.IsHealthy(c.Request.Context()),
		"subscribed_channels": h.channelMgr.GetActiveChannelCount(),
		"channels":            h.channelMgr.GetActiveChannels(),
	})
}
