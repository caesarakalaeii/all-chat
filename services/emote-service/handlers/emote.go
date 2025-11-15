package handlers

import (
	"context"
	"errors"
	"net/http"
	"regexp"

	"github.com/caesar/all-chat/services/emote-service/cache"
	"github.com/caesar/all-chat/services/emote-service/models"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// EmoteClient interface for fetching emotes
type EmoteClient interface {
	FetchEmotes(ctx context.Context, channel string) ([]models.Emote, error)
	Provider() string
}

// EmoteCache interface for caching emotes
type EmoteCache interface {
	Get(ctx context.Context, provider, channel string) ([]models.Emote, error)
	Set(ctx context.Context, provider, channel string, emotes []models.Emote) error
}

// EmoteHandler handles emote-related HTTP requests
type EmoteHandler struct {
	clients map[string]EmoteClient
	cache   EmoteCache
	logger  *zap.Logger
}

var channelPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// NewEmoteHandler creates a new emote handler
func NewEmoteHandler(clients map[string]EmoteClient, cache EmoteCache, logger *zap.Logger) *EmoteHandler {
	return &EmoteHandler{
		clients: clients,
		cache:   cache,
		logger:  logger,
	}
}

// GetChannelEmotes handles GET /emotes/channel/:channel
// Returns aggregated emotes from all providers
func (h *EmoteHandler) GetChannelEmotes(c *gin.Context) {
	channel := c.Param("channel")
	if channel == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "channel parameter is required"})
		return
	}
	if !isValidChannel(channel) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid channel identifier"})
		return
	}

	h.logger.Info("Fetching emotes for channel",
		zap.String("channel", channel))

	// Fetch emotes from all providers concurrently
	allEmotes := make([]models.Emote, 0)

	for provider, client := range h.clients {
		emotes, err := h.fetchWithCache(c.Request.Context(), client, provider, channel)
		if err != nil {
			h.logger.Error("Failed to fetch emotes from provider",
				zap.String("provider", provider),
				zap.String("channel", channel),
				zap.Error(err))
			// Continue with other providers even if one fails
			continue
		}
		allEmotes = append(allEmotes, emotes...)
	}

	response := models.EmoteResponse{
		Channel: channel,
		Emotes:  allEmotes,
	}

	h.logger.Info("Fetched emotes for channel",
		zap.String("channel", channel),
		zap.Int("total_count", len(allEmotes)))

	c.JSON(http.StatusOK, response)
}

// GetProviderEmotes handles GET /emotes/:provider/:channel
// Returns emotes from a specific provider
func (h *EmoteHandler) GetProviderEmotes(c *gin.Context) {
	provider := c.Param("provider")
	channel := c.Param("channel")

	if channel == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "channel parameter is required"})
		return
	}
	if !isValidChannel(channel) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid channel identifier"})
		return
	}

	client, ok := h.clients[provider]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid provider"})
		return
	}

	h.logger.Info("Fetching emotes from provider",
		zap.String("provider", provider),
		zap.String("channel", channel))

	emotes, err := h.fetchWithCache(c.Request.Context(), client, provider, channel)
	if err != nil {
		h.logger.Error("Failed to fetch emotes",
			zap.String("provider", provider),
			zap.String("channel", channel),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch emotes"})
		return
	}

	response := models.EmoteResponse{
		Channel: channel,
		Emotes:  emotes,
	}

	h.logger.Info("Fetched emotes from provider",
		zap.String("provider", provider),
		zap.String("channel", channel),
		zap.Int("count", len(emotes)))

	c.JSON(http.StatusOK, response)
}

// fetchWithCache attempts to fetch from cache first, then from API if cache miss
func (h *EmoteHandler) fetchWithCache(ctx context.Context, client EmoteClient, provider, channel string) ([]models.Emote, error) {
	// Try cache first
	emotes, err := h.cache.Get(ctx, provider, channel)
	if err == nil {
		h.logger.Debug("Cache hit",
			zap.String("provider", provider),
			zap.String("channel", channel),
			zap.Int("count", len(emotes)))
		return emotes, nil
	}

	// Cache miss - fetch from API
	if !errors.Is(err, cache.ErrCacheMiss) {
		h.logger.Warn("Cache error, fetching from API",
			zap.String("provider", provider),
			zap.String("channel", channel),
			zap.Error(err))
	}

	h.logger.Debug("Cache miss, fetching from API",
		zap.String("provider", provider),
		zap.String("channel", channel))

	emotes, err = client.FetchEmotes(ctx, channel)
	if err != nil {
		return nil, err
	}

	// Store in cache (best effort - don't fail if cache set fails)
	if err := h.cache.Set(ctx, provider, channel, emotes); err != nil {
		h.logger.Warn("Failed to set cache",
			zap.String("provider", provider),
			zap.String("channel", channel),
			zap.Error(err))
	}

	return emotes, nil
}

// RegisterRoutes registers all emote routes
func (h *EmoteHandler) RegisterRoutes(router *gin.Engine) {
	router.GET("/emotes/channel/:channel", h.GetChannelEmotes)
	router.GET("/emotes/:provider/:channel", h.GetProviderEmotes)
}

func isValidChannel(channel string) bool {
	return channelPattern.MatchString(channel)
}
