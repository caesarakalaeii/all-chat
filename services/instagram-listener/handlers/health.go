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

	"github.com/caesar/all-chat/services/instagram-listener/channels"
	"github.com/caesar/all-chat/services/instagram-listener/publisher"
	"github.com/gin-gonic/gin"
)

// HealthHandler exposes liveness/readiness/status for k8s probes.
type HealthHandler struct {
	publisher *publisher.StreamPublisher
	mgr       *channels.Manager
}

// NewHealthHandler wires the health endpoints.
func NewHealthHandler(pub *publisher.StreamPublisher, mgr *channels.Manager) *HealthHandler {
	return &HealthHandler{publisher: pub, mgr: mgr}
}

// LivenessProbe always reports the process alive.
func (h *HealthHandler) LivenessProbe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive", "service": "instagram-listener"})
}

// ReadinessProbe checks Redis (the only downstream dependency).
func (h *HealthHandler) ReadinessProbe(c *gin.Context) {
	if !h.publisher.IsHealthy(c.Request.Context()) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "reason": "redis"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready", "service": "instagram-listener"})
}

// Status reports the current poller footprint.
func (h *HealthHandler) Status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"service":        "instagram-listener",
		"active_pollers": h.mgr.ActivePollers(),
		"redis_healthy":  h.publisher.IsHealthy(c.Request.Context()),
	})
}
