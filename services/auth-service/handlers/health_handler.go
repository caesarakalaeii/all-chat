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
