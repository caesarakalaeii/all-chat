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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// HealthHandler handles health check endpoints
type HealthHandler struct {
	db    *pgxpool.Pool
	redis *redis.Client
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db *pgxpool.Pool, redis *redis.Client) *HealthHandler {
	return &HealthHandler{
		db:    db,
		redis: redis,
	}
}

// CheckLive returns a simple liveness probe
func (h *HealthHandler) CheckLive(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}

// CheckReady returns a readiness probe with dependency checks
func (h *HealthHandler) CheckReady(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	status := gin.H{
		"status":   "ready",
		"database": "unknown",
		"redis":    "unknown",
	}

	// Check PostgreSQL
	if err := h.db.Ping(ctx); err != nil {
		status["status"] = "not ready"
		status["database"] = "unhealthy"
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}
	status["database"] = "healthy"

	// Check Redis
	if err := h.redis.Ping(ctx).Err(); err != nil {
		status["status"] = "not ready"
		status["redis"] = "unhealthy"
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}
	status["redis"] = "healthy"

	c.JSON(http.StatusOK, status)
}
