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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	redisClient *redis.Client
	logger      *zap.Logger
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(redisClient *redis.Client, logger *zap.Logger) *HealthHandler {
	return &HealthHandler{
		redisClient: redisClient,
		logger:      logger,
	}
}

// Liveness handles GET /health/live
// Returns 200 if service is running
func (h *HealthHandler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "alive",
	})
}

// Readiness handles GET /health/ready
// Returns 200 if service is ready to accept requests (Redis is accessible)
func (h *HealthHandler) Readiness(c *gin.Context) {
	// Check Redis connection
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := h.redisClient.Ping(ctx).Err(); err != nil {
		h.logger.Error("Redis health check failed", zap.Error(err))
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "not ready",
			"error":  "redis connection failed",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}

// RegisterRoutes registers all health routes
func (h *HealthHandler) RegisterRoutes(router *gin.Engine) {
	router.GET("/health/live", h.Liveness)
	router.GET("/health/ready", h.Readiness)
}
