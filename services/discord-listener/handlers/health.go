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

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// gatewayConnectionHealth is the subset of gateway.GatewayClient used by the health handler.
// Defined as an interface so tests can inject a stub without constructing a full Gateway client.
type gatewayConnectionHealth interface {
	LastActivityAt() time.Time
	IsStale() bool
}

// HealthHandler provides liveness and readiness probe handlers for Kubernetes health checks.
type HealthHandler struct {
	redis       *redis.Client
	gatewayConn gatewayConnectionHealth
}

// NewHealthHandler creates a new HealthHandler with the provided Redis client and Gateway client.
// gatewayConn may be nil (liveness probe degrades gracefully when not set).
func NewHealthHandler(rdb *redis.Client, gw gatewayConnectionHealth) *HealthHandler {
	return &HealthHandler{redis: rdb, gatewayConn: gw}
}

// CheckLive handles GET /health/live.
// Returns 503 when the Gateway WebSocket connection has gone silent for longer than
// staleLivenessThreshold (3 minutes). This indicates a zombie connection that the
// reconnect loop has failed to recover. Kubernetes will restart the pod, which cleanly
// re-establishes the Gateway session.
// Returns 200 during startup (lastActivityAt is zero) so the pod is not killed before
// the initial session is established.
func (h *HealthHandler) CheckLive(c *gin.Context) {
	if h.gatewayConn != nil && h.gatewayConn.IsStale() {
		lastAct := h.gatewayConn.LastActivityAt()
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "dead",
			"service": "discord-listener",
			"reason":  "Gateway WebSocket zombie — no activity for over 3 minutes",
			"last_activity_seconds_ago": int(time.Since(lastAct).Seconds()),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "alive",
		"service": "discord-listener",
	})
}

// CheckReady handles GET /health/ready — returns 200 only when Redis is reachable.
func (h *HealthHandler) CheckReady(c *gin.Context) {
	if h.redis == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "redis unavailable", "error": "redis client not configured"})
		return
	}
	ctx := c.Request.Context()
	if err := h.redis.Ping(ctx).Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "redis unavailable", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
