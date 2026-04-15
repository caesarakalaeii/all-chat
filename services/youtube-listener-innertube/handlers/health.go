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
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RedisHealthChecker defines the interface for checking Redis connectivity
type RedisHealthChecker interface {
	Ping(ctx context.Context) error
}

// InnerTubeClientChecker defines the interface for checking InnerTube client initialization
type InnerTubeClientChecker interface {
	IsInitialized() bool
}

// HealthHandler handles health check endpoints
type HealthHandler struct {
	publisher      RedisHealthChecker
	innertubeReady InnerTubeClientChecker
	logger         *zap.Logger
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(publisher RedisHealthChecker, innertubeReady InnerTubeClientChecker, logger *zap.Logger) *HealthHandler {
	return &HealthHandler{
		publisher:      publisher,
		innertubeReady: innertubeReady,
		logger:         logger,
	}
}

// LivenessProbe handles the liveness probe
// Returns 200 OK if the service is running (no deadlock detection for PoC)
// Per user decision: future enhancement will detect deadlocks and return 500
func (h *HealthHandler) LivenessProbe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "alive",
	})
}

// ReadinessProbe handles the readiness probe
// Checks if Redis connection is healthy AND InnerTube client is initialized
// Per user decision: "ready even if no stream actively monitored yet"
func (h *HealthHandler) ReadinessProbe(c *gin.Context) {
	ctx := c.Request.Context()

	// Check 1: Redis connection via publisher.Ping()
	if err := h.publisher.Ping(ctx); err != nil {
		h.logger.Warn("Readiness check failed: Redis connection unavailable",
			zap.Error(err),
		)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"error":  "redis connection failed",
			"detail": err.Error(),
		})
		return
	}

	// Check 2: InnerTube client initialized
	if h.innertubeReady != nil && !h.innertubeReady.IsInitialized() {
		h.logger.Warn("Readiness check failed: InnerTube client not initialized")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"error":  "innertube client not initialized",
		})
		return
	}

	// All checks passed
	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}

// Status returns detailed status information (useful for debugging)
// Not critical for PoC but helpful for Phase 10 multi-stream tracking
func (h *HealthHandler) Status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":        "running",
		"poller_state":  "active",
		"message":       "InnerTube PoC service - single stream monitoring",
	})
}
