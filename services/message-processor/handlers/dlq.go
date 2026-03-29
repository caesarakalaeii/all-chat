package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	dlqStreamKey = "chat:dlq"
	rawStreamKey = "chat:raw"
)

// HandleDLQReplay returns a Gin handler for POST /admin/dlq/replay.
// It reads up to 100 messages from chat:dlq, re-publishes each to chat:raw,
// and deletes successfully replayed entries from chat:dlq.
// Response: {"replayed": N, "failed": M}
func HandleDLQReplay(redisClient *redis.Client, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		// Read up to 100 messages from DLQ
		entries, err := redisClient.XRange(ctx, dlqStreamKey, "-", "+").Result()
		if err != nil {
			logger.Error("Failed to read DLQ entries", zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read DLQ"})
			return
		}

		// Limit to 100 entries per request
		if len(entries) > 100 {
			entries = entries[:100]
		}

		replayed := 0
		failed := 0

		for _, entry := range entries {
			// Extract original_data field for replay
			originalData, _ := entry.Values["original_data"].(string)

			// XADD to chat:raw with original data + replayed flag
			_, addErr := redisClient.XAdd(ctx, &redis.XAddArgs{
				Stream: rawStreamKey,
				ID:     "*",
				Values: map[string]interface{}{
					"data":     originalData,
					"replayed": "true",
				},
			}).Result()
			if addErr != nil {
				logger.Error("Failed to replay DLQ entry to chat:raw",
					zap.String("dlq_id", entry.ID),
					zap.Error(addErr),
				)
				failed++
				continue
			}

			// Delete from DLQ after successful XADD
			if delErr := redisClient.XDel(ctx, dlqStreamKey, entry.ID).Err(); delErr != nil {
				logger.Warn("Failed to delete replayed DLQ entry",
					zap.String("dlq_id", entry.ID),
					zap.Error(delErr),
				)
				// Still count as replayed since we wrote to chat:raw
			}

			replayed++
		}

		logger.Info("DLQ replay complete",
			zap.Int("replayed", replayed),
			zap.Int("failed", failed),
		)

		c.JSON(http.StatusOK, gin.H{
			"replayed": replayed,
			"failed":   failed,
		})
	}
}

