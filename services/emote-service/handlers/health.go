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
