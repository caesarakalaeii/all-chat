package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// HealthHandler provides liveness and readiness probe handlers for Kubernetes health checks.
type HealthHandler struct {
	redis *redis.Client
}

// NewHealthHandler creates a new HealthHandler with the provided Redis client.
func NewHealthHandler(redis *redis.Client) *HealthHandler {
	return &HealthHandler{redis: redis}
}

// CheckLive handles GET /health/live — always returns 200 OK.
func (h *HealthHandler) CheckLive(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// CheckReady handles GET /health/ready — returns 200 only when Redis is reachable.
func (h *HealthHandler) CheckReady(c *gin.Context) {
	ctx := c.Request.Context()
	if err := h.redis.Ping(ctx).Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "redis unavailable", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
