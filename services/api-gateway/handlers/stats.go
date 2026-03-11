package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// StatsHandler serves public platform statistics.
type StatsHandler struct {
	redis *redis.Client
}

// NewStatsHandler creates a StatsHandler.
func NewStatsHandler(redis *redis.Client) *StatsHandler {
	return &StatsHandler{redis: redis}
}

// GetPlatformStats returns message counts per platform for the last 24 hours.
// GET /api/v1/stats — public, no auth required.
func (h *StatsHandler) GetPlatformStats(c *gin.Context) {
	ctx := c.Request.Context()

	platforms := []string{"twitch", "youtube", "kick", "tiktok"}
	result := make(map[string]int64, len(platforms))

	for _, platform := range platforms {
		val, err := h.redis.Get(ctx, "chat:stats:24h:"+platform).Int64()
		if err != nil {
			val = 0 // key missing = no messages yet
		}
		result[platform] = val
	}

	c.JSON(http.StatusOK, result)
}
