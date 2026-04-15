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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// DBPool defines the interface for database health checks
type DBPool interface {
	Ping(ctx context.Context) error
	Close()
}

// RedisClient defines the interface for Redis health checks
type RedisClient interface {
	Ping(ctx context.Context) *redis.StatusCmd
}

// HealthHandler handles health check requests
type HealthHandler struct {
	db    DBPool
	redis RedisClient
}

// Ensure pgxpool.Pool implements DBPool interface (compile-time check)
var _ DBPool = (*pgxpool.Pool)(nil)

// NewHealthHandler creates a new health handler
func NewHealthHandler(db DBPool, redis RedisClient) *HealthHandler {
	return &HealthHandler{
		db:    db,
		redis: redis,
	}
}

// HandleLiveness handles GET /health/live
func (h *HealthHandler) HandleLiveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "alive",
	})
}

// HandleReadiness handles GET /health/ready
func (h *HealthHandler) HandleReadiness(c *gin.Context) {
	ctx := c.Request.Context()

	// Check database connection
	if h.db != nil {
		if err := h.db.Ping(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not ready",
				"error":  "database unreachable",
			})
			return
		}
	}

	// Check Redis connection
	if h.redis != nil {
		if err := h.redis.Ping(ctx).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "not ready",
				"error":  "redis unreachable",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ready",
	})
}

// RegisterRoutes registers health check routes
func (h *HealthHandler) RegisterRoutes(router *gin.Engine) {
	router.GET("/health/live", h.HandleLiveness)
	router.GET("/health/ready", h.HandleReadiness)
}
