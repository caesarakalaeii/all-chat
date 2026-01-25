package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/emote-service/clients"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// CheermoteClient interface for fetching cheermotes
type CheermoteClient interface {
	FetchCheermotes(ctx context.Context, channelID string) ([]clients.CheermoteData, error)
}

// CheermoteHandler handles cheermote-related HTTP requests
type CheermoteHandler struct {
	client      CheermoteClient
	redisClient *redis.Client
	logger      *zap.Logger
	cacheTTL    time.Duration
}

// NewCheermoteHandler creates a new cheermote handler
func NewCheermoteHandler(client CheermoteClient, redisClient *redis.Client, logger *zap.Logger) *CheermoteHandler {
	return &CheermoteHandler{
		client:      client,
		redisClient: redisClient,
		logger:      logger,
		cacheTTL:    1 * time.Hour, // Cheermotes rarely change
	}
}

// GetCheermotes handles GET /emotes/cheermotes/:channel_id
// Returns cheermote data for a specific channel
func (h *CheermoteHandler) GetCheermotes(c *gin.Context) {
	channelID := c.Param("channel_id")
	if channelID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "channel_id parameter is required"})
		return
	}

	h.logger.Info("Fetching cheermotes for channel",
		zap.String("channel_id", channelID))

	ctx := c.Request.Context()
	cacheKey := "emote-service:cheermotes:v1:" + channelID

	// Try cache first
	if h.redisClient != nil {
		cached, err := h.redisClient.Get(ctx, cacheKey).Bytes()
		if err == nil {
			h.logger.Debug("Cheermote cache hit", zap.String("channel_id", channelID))
			c.Header("X-Cache", "HIT")
			c.Header("Content-Type", "application/json")
			c.Data(http.StatusOK, "application/json", cached)
			return
		} else if err != redis.Nil {
			h.logger.Warn("Cheermote cache error",
				zap.String("channel_id", channelID),
				zap.Error(err))
		}
	}

	// Cache miss - fetch from Twitch
	cheermotes, err := h.client.FetchCheermotes(ctx, channelID)
	if err != nil {
		h.logger.Error("Failed to fetch cheermotes",
			zap.String("channel_id", channelID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch cheermotes"})
		return
	}

	// Build response
	response := gin.H{
		"channel_id":  channelID,
		"cheermotes":  cheermotes,
	}

	// Cache the response
	if h.redisClient != nil {
		if data, err := json.Marshal(response); err == nil {
			if err := h.redisClient.Set(ctx, cacheKey, data, h.cacheTTL).Err(); err != nil {
				h.logger.Warn("Failed to cache cheermotes",
					zap.String("channel_id", channelID),
					zap.Error(err))
			}
		}
	}

	c.Header("X-Cache", "MISS")
	c.JSON(http.StatusOK, response)
}
