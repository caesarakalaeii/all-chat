package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"time"

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

// CombinedEmoteClient is an optional interface for clients that support user emotes
type CombinedEmoteClient interface {
	EmoteClient
	FetchCombinedEmotes(ctx context.Context, channel, platform, userID string) ([]models.Emote, error)
}

// EmoteCache interface for caching emotes
type EmoteCache interface {
	Get(ctx context.Context, provider, channel string) ([]models.Emote, error)
	Set(ctx context.Context, provider, channel string, emotes []models.Emote) error
}

// EmoteHandler handles emote-related HTTP requests
type EmoteHandler struct {
	clients      map[string]EmoteClient
	cache        EmoteCache
	logger       *zap.Logger
	fetchTimeout time.Duration
}

// Allow human-readable channel names (letters, numbers, spaces, dash, dot, underscore)
var channelPattern = regexp.MustCompile(`^[A-Za-z0-9 _.-]+$`)

// NewEmoteHandler creates a new emote handler
func NewEmoteHandler(clients map[string]EmoteClient, cache EmoteCache, logger *zap.Logger) *EmoteHandler {
	return &EmoteHandler{
		clients:      clients,
		cache:        cache,
		logger:       logger,
		fetchTimeout: 3 * time.Second,
	}
}

// GetChannelEmotes handles GET /emotes/channel/:channel?user_id=123&platform=twitch
// Returns aggregated emotes from all providers
// If user_id and platform are provided, includes user-specific emotes from 7TV
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

	userID := c.Query("user_id")
	platform := c.Query("platform")

	h.logger.Info("Fetching emotes for channel",
		zap.String("channel", channel),
		zap.String("user_id", userID),
		zap.String("platform", platform))

	ctx := c.Request.Context()
	type providerResult struct {
		emotes   []models.Emote
		err      error
		provider string
	}

	results := make(chan providerResult, len(h.clients))
	var wg sync.WaitGroup

	for provider, client := range h.clients {
		wg.Add(1)
		go func(provider string, client EmoteClient) {
			defer wg.Done()
			providerCtx := ctx
			cancel := func() {}
			if h.fetchTimeout > 0 {
				providerCtx, cancel = context.WithTimeout(ctx, h.fetchTimeout)
			}
			defer cancel()

			emotes, err := h.fetchWithCacheAndUser(providerCtx, client, provider, channel, platform, userID)
			select {
			case results <- providerResult{emotes: emotes, err: err, provider: provider}:
			case <-ctx.Done():
			}
		}(provider, client)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	allEmotes := make([]models.Emote, 0)
	for res := range results {
		if res.err != nil {
			h.logger.Warn("Failed to fetch emotes from provider",
				zap.String("provider", res.provider),
				zap.String("channel", channel),
				zap.Error(res.err))
			continue
		}
		allEmotes = append(allEmotes, res.emotes...)
	}

	response := models.EmoteResponse{
		Channel: channel,
		Emotes:  allEmotes,
	}

	h.logger.Info("Fetched emotes for channel",
		zap.String("channel", channel),
		zap.String("user_id", userID),
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

// fetchWithCacheAndUser fetches emotes with optional user context
// For providers that support user emotes (7TV), fetches combined channel + user emotes
func (h *EmoteHandler) fetchWithCacheAndUser(ctx context.Context, client EmoteClient, provider, channel, platform, userID string) ([]models.Emote, error) {
	// Check if this client supports combined emotes
	combinedClient, supportsCombined := client.(CombinedEmoteClient)

	// If no user ID provided or client doesn't support combined emotes, use regular fetch
	if userID == "" || !supportsCombined {
		return h.fetchWithCache(ctx, client, provider, channel)
	}

	// Build cache key that includes user ID for combined emotes
	cacheKey := fmt.Sprintf("%s:%s", channel, userID)

	// Try cache first
	emotes, err := h.cache.Get(ctx, provider, cacheKey)
	if err == nil {
		h.logger.Debug("Cache hit (with user)",
			zap.String("provider", provider),
			zap.String("channel", channel),
			zap.String("user_id", userID),
			zap.Int("count", len(emotes)))
		return emotes, nil
	}

	// Cache miss - fetch from API
	if !errors.Is(err, cache.ErrCacheMiss) {
		h.logger.Warn("Cache error, fetching from API",
			zap.String("provider", provider),
			zap.String("channel", channel),
			zap.String("user_id", userID),
			zap.Error(err))
	}

	h.logger.Debug("Cache miss, fetching combined emotes from API",
		zap.String("provider", provider),
		zap.String("channel", channel),
		zap.String("user_id", userID))

	emotes, err = combinedClient.FetchCombinedEmotes(ctx, channel, platform, userID)
	if err != nil {
		return nil, err
	}

	// Store in cache (best effort - don't fail if cache set fails)
	if err := h.cache.Set(ctx, provider, cacheKey, emotes); err != nil {
		h.logger.Warn("Failed to set cache",
			zap.String("provider", provider),
			zap.String("channel", channel),
			zap.String("user_id", userID),
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
