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

// Package handlers exposes health endpoints. Unlike kick-listener, an
// all-instances-down world is NOT unhealthy: an Owncast instance being
// unreachable is normal (ADR-0058), so liveness only checks nothing is stale,
// and readiness only gates on Redis.
package handlers

import (
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/owncast-listener/channels"
	"github.com/caesar/all-chat/services/owncast-listener/publisher"
	"github.com/gin-gonic/gin"
)

// HealthHandler handles health check endpoints.
type HealthHandler struct {
	publisher  *publisher.StreamPublisher
	channelMgr *channels.Manager
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(publisher *publisher.StreamPublisher, channelMgr *channels.Manager) *HealthHandler {
	return &HealthHandler{
		publisher:  publisher,
		channelMgr: channelMgr,
	}
}

// LivenessProbe: the process is alive. Per-connection health is Normal-state,
// not pod liveness.
func (h *HealthHandler) LivenessProbe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "alive",
		"service": "owncast-listener",
	})
}

// ReadinessProbe checks Redis. Zero connections is valid (no demanded
// instances, or every instance currently down — see ADR-0058).
func (h *HealthHandler) ReadinessProbe(c *gin.Context) {
	if !h.publisher.IsHealthy(c.Request.Context()) {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"reason": "redis not healthy",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":      "ready",
		"connections": h.channelMgr.GetActiveChannelCount(),
		"instances":   h.channelMgr.GetActiveChannels(),
	})
}

// Status returns detailed service status.
func (h *HealthHandler) Status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":          "running",
		"redis_healthy":   h.publisher.IsHealthy(c.Request.Context()),
		"connected_count": h.channelMgr.GetActiveChannelCount(),
		"connected":       h.channelMgr.GetActiveChannels(),
		"checked_at":      time.Now().UTC().Format(time.RFC3339),
	})
}
