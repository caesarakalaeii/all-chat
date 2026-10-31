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

	"github.com/caesar/all-chat/services/altstream-listener/channels"
	"github.com/caesar/all-chat/services/altstream-listener/publisher"
	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check endpoints.
type HealthHandler struct {
	publisher  *publisher.StreamPublisher
	channelMgr *channels.Manager
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(publisher *publisher.StreamPublisher, channelMgr *channels.Manager) *HealthHandler {
	return &HealthHandler{publisher: publisher, channelMgr: channelMgr}
}

// LivenessProbe always answers 200. A silent stream is not a reason to
// restart the pod: each stream reconnects itself after 75s without a ping.
func (h *HealthHandler) LivenessProbe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "alive",
		"service": "altstream-listener",
	})
}

// ReadinessProbe gates on Redis only. Streams connect in the background and
// channels led by another replica are never streamed here, so gating on the
// stream count would hold a new pod unready behind the old one's leases.
func (h *HealthHandler) ReadinessProbe(c *gin.Context) {
	if !h.publisher.IsHealthy(c.Request.Context()) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"reason": "redis not healthy",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":        "ready",
		"subscriptions": h.channelMgr.GetActiveChannelCount(),
		"connected":     h.channelMgr.ConnectedCount(),
	})
}

// Status returns detailed service status.
func (h *HealthHandler) Status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":              "running",
		"redis_healthy":       h.publisher.IsHealthy(c.Request.Context()),
		"subscribed_channels": h.channelMgr.GetActiveChannelCount(),
		"connected_streams":   h.channelMgr.ConnectedCount(),
		"demanded_channels":   h.channelMgr.GetFilteredAssignmentCount(),
		"channels":            h.channelMgr.GetActiveChannels(),
	})
}
