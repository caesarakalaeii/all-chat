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
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	avatarCacheKeyPrefix = "avatar:img:"
	avatarCacheTTL       = 24 * time.Hour
	avatarMaxBytes       = 256 * 1024 // 256KB max avatar size
	avatarFetchTimeout   = 5 * time.Second
)

// AvatarProxyHandler serves cached avatar images for platforms with expiring CDN URLs.
type AvatarProxyHandler struct {
	httpClient  *http.Client
	redisClient *redis.Client
	log         *zap.Logger
}

// NewAvatarProxyHandler creates a new avatar proxy handler.
func NewAvatarProxyHandler(redisClient *redis.Client, log *zap.Logger) *AvatarProxyHandler {
	return &AvatarProxyHandler{
		httpClient: &http.Client{
			Timeout: avatarFetchTimeout,
		},
		redisClient: redisClient,
		log:         log.Named("avatar-proxy"),
	}
}

// GetAvatar serves a cached avatar image by platform and user ID.
// GET /api/avatars/:platform/:user_id
func (h *AvatarProxyHandler) GetAvatar(c *gin.Context) {
	platform := c.Param("platform")
	userID := c.Param("user_id")

	if platform == "" || userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform and user_id are required"})
		return
	}

	cacheKey := fmt.Sprintf("%s%s:%s", avatarCacheKeyPrefix, platform, userID)

	imgBytes, err := h.redisClient.Get(c.Request.Context(), cacheKey).Bytes()
	if err != nil {
		// Cache miss or error — return 404 so frontend falls back to initials
		c.Status(http.StatusNotFound)
		return
	}

	contentType := http.DetectContentType(imgBytes)
	c.Header("Cache-Control", "public, max-age=3600")
	c.Data(http.StatusOK, contentType, imgBytes)
}

// CacheAvatar fetches an avatar from a URL and stores it in Redis.
// Called by the message processor enricher, not exposed as an HTTP endpoint.
func CacheAvatar(ctx context.Context, redisClient *redis.Client, httpClient *http.Client, platform, userID, avatarURL string) error {
	if avatarURL == "" {
		return nil
	}

	cacheKey := fmt.Sprintf("%s%s:%s", avatarCacheKeyPrefix, platform, userID)

	// Check if already cached
	exists, err := redisClient.Exists(ctx, cacheKey).Result()
	if err == nil && exists > 0 {
		return nil
	}

	// Fetch the image
	req, err := http.NewRequestWithContext(ctx, "GET", avatarURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch avatar: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("avatar fetch returned status %d", resp.StatusCode)
	}

	imgBytes, err := io.ReadAll(io.LimitReader(resp.Body, avatarMaxBytes))
	if err != nil {
		return fmt.Errorf("read avatar body: %w", err)
	}

	if len(imgBytes) == 0 {
		return fmt.Errorf("empty avatar response")
	}

	// Store in Redis with TTL
	return redisClient.Set(ctx, cacheKey, imgBytes, avatarCacheTTL).Err()
}
