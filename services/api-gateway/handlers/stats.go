package handlers

import (
	"net/http"
	"time"

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

// GetPlatformStats returns message counts per platform for the last 7 days.
// GET /api/v1/stats — public, no auth required.
func (h *StatsHandler) GetPlatformStats(c *gin.Context) {
	ctx := c.Request.Context()

	platforms := []string{"twitch", "youtube", "kick", "tiktok"}
	result := make(map[string]int64, len(platforms))

	// Build list of the last 7 daily bucket suffixes (YYYY-MM-DD).
	now := time.Now().UTC()
	days := make([]string, 7)
	for i := range days {
		days[i] = now.AddDate(0, 0, -i).Format("2006-01-02")
	}

	for _, platform := range platforms {
		var total int64
		for _, day := range days {
			val, err := h.redis.Get(ctx, "chat:stats:daily:"+platform+":"+day).Int64()
			if err == nil {
				total += val
			}
		}
		result[platform] = total
	}

	c.JSON(http.StatusOK, result)
}

// GetActiveOverlays returns the IDs of overlays with active WebSocket connections.
// GET /api/v1/admin/overlays/active — requires admin auth.
func (h *StatsHandler) GetActiveOverlays(c *gin.Context) {
	ctx := c.Request.Context()

	var activeIDs []string
	var cursor uint64
	for {
		keys, next, err := h.redis.Scan(ctx, cursor, "overlay:connected:*", 100).Result()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to scan active overlays"})
			return
		}
		for _, key := range keys {
			// key format: "overlay:connected:{id}"
			id := key[len("overlay:connected:"):]
			if id != "" {
				activeIDs = append(activeIDs, id)
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}

	if activeIDs == nil {
		activeIDs = []string{}
	}
	c.JSON(http.StatusOK, activeIDs)
}
