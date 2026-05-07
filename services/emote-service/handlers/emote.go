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
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/emote-service/cache"
	"github.com/caesar/all-chat/services/emote-service/models"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// EmoteClient interface for fetching emotes
type EmoteClient interface {
	FetchEmotes(ctx context.Context, channel string) ([]models.Emote, error)
	Provider() string
}

// CombinedEmoteClient is an optional interface for clients that support user emotes.
// twitchChannel is an optional hint from a sibling Twitch source on the same overlay,
// allowing 7TV channel emote lookup for non-Twitch platforms. seventvSetID is an
// optional per-overlay 7TV emote-set override that's merged into the result.
type CombinedEmoteClient interface {
	EmoteClient
	FetchCombinedEmotes(ctx context.Context, channel, platform, userID, twitchChannel, seventvSetID string) ([]models.Emote, error)
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
	apiCalls     *prometheus.CounterVec
}

// Allow human-readable channel names (letters, numbers, spaces, dash, dot, underscore)
var channelPattern = regexp.MustCompile(`^[A-Za-z0-9 _.-]+$`)

// NewEmoteHandler creates a new emote handler.
// apiCalls is a counter tracking emote provider API calls (may be nil — skipped if nil).
func NewEmoteHandler(clients map[string]EmoteClient, cache EmoteCache, logger *zap.Logger, apiCalls *prometheus.CounterVec) *EmoteHandler {
	return &EmoteHandler{
		clients:      clients,
		cache:        cache,
		logger:       logger,
		fetchTimeout: 3 * time.Second,
		apiCalls:     apiCalls,
	}
}

// GetChannelEmotes handles GET /emotes/channel/:channel?user_id=123&platform=twitch
// Returns aggregated emotes from all providers
// If user_id and platform are provided, includes user-specific emotes from 7TV
// For non-Twitch platforms, includes Twitch global emotes
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
	twitchChannel := c.Query("twitch_channel")
	seventvSetID := c.Query("seventv_set_id")

	h.logger.Info("Fetching emotes for channel",
		zap.String("channel", channel),
		zap.String("user_id", userID),
		zap.String("platform", platform),
		zap.String("twitch_channel", twitchChannel),
		zap.String("seventv_set_id", seventvSetID))

	ctx := c.Request.Context()
	type providerResult struct {
		emotes   []models.Emote
		err      error
		provider string
	}

	results := make(chan providerResult, len(h.clients)+1) // +1 for potential Twitch global
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

			emotes, err := h.fetchWithCacheAndUser(providerCtx, client, provider, channel, platform, userID, twitchChannel, seventvSetID)
			select {
			case results <- providerResult{emotes: emotes, err: err, provider: provider}:
			case <-ctx.Done():
			}
		}(provider, client)
	}

	// For non-Twitch platforms, fetch Twitch global emotes
	if platform != "" && platform != "twitch" {
		if twitchClient, ok := h.clients["twitch"]; ok {
			wg.Add(1)
			go func() {
				defer wg.Done()
				providerCtx := ctx
				cancel := func() {}
				if h.fetchTimeout > 0 {
					providerCtx, cancel = context.WithTimeout(ctx, h.fetchTimeout)
				}
				defer cancel()

				emotes, err := h.fetchWithCache(providerCtx, twitchClient, "twitch", "global")
				select {
				case results <- providerResult{emotes: emotes, err: err, provider: "twitch-global"}:
				case <-ctx.Done():
				}
			}()
		}
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
		zap.String("platform", platform),
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
		if h.apiCalls != nil {
			h.apiCalls.WithLabelValues("emote-service", provider, "error").Inc()
		}
		return nil, err
	}

	if h.apiCalls != nil {
		h.apiCalls.WithLabelValues("emote-service", provider, "success").Inc()
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

// fetchWithCacheAndUser fetches emotes with optional user context.
// For providers that support user emotes (7TV), fetches combined channel + user emotes.
// For non-Twitch platforms, routes through the combined path even without a user ID so
// that providers like 7TV can apply platform-aware logic. When twitchChannel is provided
// (from a sibling Twitch source on the same overlay), it's used for 7TV channel emote lookup.
// When seventvSetID is provided (per-overlay override), the 7TV client merges that set
// into the response.
func (h *EmoteHandler) fetchWithCacheAndUser(ctx context.Context, client EmoteClient, provider, channel, platform, userID, twitchChannel, seventvSetID string) ([]models.Emote, error) {
	// Check if this client supports combined emotes
	combinedClient, supportsCombined := client.(CombinedEmoteClient)

	// Route through the combined path when the client supports it and either:
	//  - a user ID is present, or
	//  - the platform is a known non-Twitch platform, or
	//  - a per-overlay 7TV override was supplied
	// For Twitch without a user ID and no override, fall back to the simple channel
	// fetch so the existing cache keys remain valid.
	isNonTwitchPlatform := platform != "" && platform != "twitch"
	useCombinedPath := supportsCombined && (userID != "" || isNonTwitchPlatform || seventvSetID != "")

	if !useCombinedPath {
		return h.fetchWithCache(ctx, client, provider, channel)
	}

	// Build cache key that includes all parameters that vary the response
	cacheKey := fmt.Sprintf("%s:%s:%s:%s", channel, userID, twitchChannel, seventvSetID)

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

	emotes, err = combinedClient.FetchCombinedEmotes(ctx, channel, platform, userID, twitchChannel, seventvSetID)
	if err != nil {
		if h.apiCalls != nil {
			h.apiCalls.WithLabelValues("emote-service", provider, "error").Inc()
		}
		return nil, err
	}

	if h.apiCalls != nil {
		h.apiCalls.WithLabelValues("emote-service", provider, "success").Inc()
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
