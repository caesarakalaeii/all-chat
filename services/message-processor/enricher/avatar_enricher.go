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

package enricher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// AvatarCacheTTL is how long to cache avatar URLs (24 hours)
	AvatarCacheTTL = 24 * time.Hour

	// AvatarCacheKeyPrefix is the Redis key prefix for avatar URL cache (Twitch)
	AvatarCacheKeyPrefix = "avatar:"

	// AvatarImageCacheKeyPrefix is the Redis key prefix for cached avatar image bytes
	AvatarImageCacheKeyPrefix = "avatar:img:"

	// AvatarNameCacheKeyPrefix is the Redis key prefix for cached Twitch display names.
	// A sibling of the avatar URL cache under the same prefix and TTL, needed because a
	// shared-chat origin channel is rendered with its name as well as its picture.
	AvatarNameCacheKeyPrefix = "avatar:name:"

	// AvatarImageMaxBytes is the maximum size of a cached avatar image
	AvatarImageMaxBytes = 256 * 1024 // 256KB

	// twitchDefaultAvatarURL is the grey placeholder Twitch serves for accounts with no
	// uploaded picture. It is a picture of nobody, so it counts as "no avatar".
	twitchDefaultAvatarURL = "https://static-cdn.jtvnw.net/jtv_user_pictures/-profile_image-70x70.png"
)

// TwitchHelixUser represents the Twitch Helix API user response
type TwitchHelixUser struct {
	Data []struct {
		ID              string `json:"id"`
		Login           string `json:"login"`
		DisplayName     string `json:"display_name"`
		ProfileImageURL string `json:"profile_image_url"`
	} `json:"data"`
}

// Default Twitch endpoints. Kept as struct fields (defaulting to these) so tests can
// point the enricher at an httptest server without reaching the real Twitch API.
const (
	defaultTwitchTokenURL      = "https://id.twitch.tv/oauth2/token"
	defaultTwitchHelixUsersURL = "https://api.twitch.tv/helix/users"
)

// AvatarEnricher fetches and caches user avatars from Twitch Helix API
// and caches TikTok avatar images (which have expiring CDN URLs).
//
// The app access token is shared mutable state; it is read on every Twitch lookup and
// rewritten on refresh. Because messages are enriched concurrently (ADR-0033), all
// access to accessToken goes through mu-guarded token()/setToken() accessors, and the
// token HTTP refresh is performed WITHOUT holding the lock (so a slow refresh never
// serializes unrelated lookups).
type AvatarEnricher struct {
	httpClient     *http.Client
	redisClient    *redis.Client
	clientID       string
	clientSecret   string
	mu             sync.RWMutex
	accessToken    string
	tokenURL       string
	helixUsersURL  string
	gatewayBaseURL string
	logger         *zap.Logger
}

// NewAvatarEnricher creates a new avatar enricher.
// gatewayBaseURL is the base URL of the API gateway (e.g., "http://api-gateway:8080")
// used to rewrite TikTok avatar URLs to the proxy endpoint.
func NewAvatarEnricher(redisClient *redis.Client, clientID, clientSecret, gatewayBaseURL string, logger *zap.Logger) *AvatarEnricher {
	return &AvatarEnricher{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		redisClient:    redisClient,
		clientID:       clientID,
		clientSecret:   clientSecret,
		tokenURL:       defaultTwitchTokenURL,
		helixUsersURL:  defaultTwitchHelixUsersURL,
		gatewayBaseURL: gatewayBaseURL,
		logger:         logger,
	}
}

// token returns the current app access token under a read lock.
func (e *AvatarEnricher) token() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.accessToken
}

// setToken stores a new app access token under a write lock.
func (e *AvatarEnricher) setToken(t string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.accessToken = t
}

// Enrich adds avatar URL to the user info.
// For Twitch: fetches avatar URL from Helix API and caches the URL.
// For TikTok: fetches the avatar image bytes (CDN URLs expire) and caches them,
// then rewrites the URL to the API gateway proxy endpoint.
func (e *AvatarEnricher) Enrich(ctx context.Context, msg *models.UnifiedChatMessage) error {
	switch msg.Platform {
	case "tiktok":
		return e.enrichTikTok(ctx, msg)
	case "twitch":
		return e.enrichTwitch(ctx, msg)
	default:
		return nil
	}
}

func (e *AvatarEnricher) enrichTwitch(ctx context.Context, msg *models.UnifiedChatMessage) error {
	e.enrichSharedChatOrigin(ctx, msg)

	// Check if already has avatar
	if msg.User.AvatarURL != "" && msg.User.AvatarURL != twitchDefaultAvatarURL {
		return nil
	}

	// Check cache first
	cacheKey := fmt.Sprintf("%s%s", AvatarCacheKeyPrefix, msg.User.ID)
	cachedURL, err := e.redisClient.Get(ctx, cacheKey).Result()
	if err == nil && cachedURL != "" {
		msg.User.AvatarURL = cachedURL
		return nil
	}

	// Fetch from Twitch Helix API
	profile, err := e.fetchTwitchProfile(ctx, msg.User.ID)
	if err != nil {
		e.logger.Warn("Failed to fetch avatar from Twitch",
			zap.String("user_id", msg.User.ID),
			zap.String("username", msg.User.Username),
			zap.Error(err),
		)
		return nil // Don't fail the whole message
	}

	// Update message
	msg.User.AvatarURL = profile.AvatarURL

	// Cache for 24 hours
	e.redisClient.Set(ctx, cacheKey, profile.AvatarURL, AvatarCacheTTL)

	return nil
}

// enrichSharedChatOrigin resolves the broadcaster a shared-chat message originated in
// and records the pair the overlay renders in place of a "shared" text pill:
// source_avatar_url and source_display_name. Both key names are a contract with the
// frontend (issue #814).
//
// A shared-chat origin is a channel, and a channel id is a user id, so this is the same
// Helix users lookup and the same avatar: cache the chatter's own avatar uses — a channel
// that also chats is resolved once for both.
//
// Every failure is silent: a key stays unset and the frontend falls back to the text
// pill. An origin avatar is decoration, never a reason to drop or delay a message.
func (e *AvatarEnricher) enrichSharedChatOrigin(ctx context.Context, msg *models.UnifiedChatMessage) {
	if msg.Metadata == nil || msg.Metadata["is_shared_chat"] != true {
		return
	}
	sourceRoomID, ok := msg.Metadata["source_room_id"].(string)
	if !ok || sourceRoomID == "" {
		return
	}

	avatarURL, displayName, err := e.lookupTwitchProfileCached(ctx, sourceRoomID)
	if err != nil {
		e.logger.Warn("Failed to resolve shared chat origin channel",
			zap.String("source_room_id", sourceRoomID),
			zap.Error(err),
		)
		return
	}

	// The grey default is a picture of nobody: leaving the key unset gets the truthful
	// text pill from the frontend instead of the same placeholder on every origin.
	if avatarURL != "" && avatarURL != twitchDefaultAvatarURL {
		msg.Metadata["source_avatar_url"] = avatarURL
	}
	if displayName != "" {
		msg.Metadata["source_display_name"] = displayName
	}
}

// lookupTwitchProfileCached returns a Twitch user's avatar URL and display name,
// serving both from the avatar: cache when present and filling it on a miss. An empty
// avatar URL means the account has no picture of its own; that is not an error.
func (e *AvatarEnricher) lookupTwitchProfileCached(ctx context.Context, userID string) (avatarURL, displayName string, err error) {
	urlKey := AvatarCacheKeyPrefix + userID
	nameKey := AvatarNameCacheKeyPrefix + userID

	cachedURL, urlErr := e.redisClient.Get(ctx, urlKey).Result()
	cachedName, nameErr := e.redisClient.Get(ctx, nameKey).Result()
	if urlErr == nil && nameErr == nil && cachedName != "" {
		return cachedURL, cachedName, nil
	}

	profile, err := e.fetchTwitchProfile(ctx, userID)
	if err != nil {
		return "", "", err
	}

	e.redisClient.Set(ctx, urlKey, profile.AvatarURL, AvatarCacheTTL)
	e.redisClient.Set(ctx, nameKey, profile.DisplayName, AvatarCacheTTL)

	return profile.AvatarURL, profile.DisplayName, nil
}

func (e *AvatarEnricher) enrichTikTok(ctx context.Context, msg *models.UnifiedChatMessage) error {
	if msg.User.AvatarURL == "" {
		return nil
	}

	proxyURL := fmt.Sprintf("%s/api/avatars/tiktok/%s", e.gatewayBaseURL, msg.User.ID)

	// Check if image already cached
	cacheKey := fmt.Sprintf("%s%s:%s", AvatarImageCacheKeyPrefix, "tiktok", msg.User.ID)
	exists, err := e.redisClient.Exists(ctx, cacheKey).Result()
	if err == nil && exists > 0 {
		msg.User.AvatarURL = proxyURL
		return nil
	}

	// Fetch and cache the avatar image bytes
	if err := e.fetchAndCacheImage(ctx, cacheKey, msg.User.AvatarURL); err != nil {
		e.logger.Warn("Failed to cache TikTok avatar, keeping original URL",
			zap.String("user_id", msg.User.ID),
			zap.String("username", msg.User.Username),
			zap.Error(err),
		)
		return nil // Don't fail the whole message
	}

	msg.User.AvatarURL = proxyURL
	return nil
}

func (e *AvatarEnricher) fetchAndCacheImage(ctx context.Context, cacheKey, imageURL string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("image fetch returned status %d", resp.StatusCode)
	}

	imgBytes, err := io.ReadAll(io.LimitReader(resp.Body, AvatarImageMaxBytes))
	if err != nil {
		return fmt.Errorf("read image body: %w", err)
	}

	if len(imgBytes) == 0 {
		return fmt.Errorf("empty image response")
	}

	return e.redisClient.Set(ctx, cacheKey, imgBytes, AvatarCacheTTL).Err()
}

// twitchProfile is the renderable part of a Twitch Helix user, exactly as Helix
// reported it — AvatarURL may be twitchDefaultAvatarURL.
type twitchProfile struct {
	AvatarURL   string
	DisplayName string
}

// fetchTwitchProfile fetches a user's avatar URL and display name from the Twitch Helix API
func (e *AvatarEnricher) fetchTwitchProfile(ctx context.Context, userID string) (twitchProfile, error) {
	// Ensure we have an access token
	if e.token() == "" {
		if err := e.refreshAccessToken(ctx); err != nil {
			return twitchProfile{}, fmt.Errorf("failed to get access token: %w", err)
		}
	}

	// Call Twitch Helix API
	url := fmt.Sprintf("%s?id=%s", e.helixUsersURL, userID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return twitchProfile{}, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.token()))
	req.Header.Set("Client-Id", e.clientID)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return twitchProfile{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		// Token expired, refresh and retry
		if err := e.refreshAccessToken(ctx); err != nil {
			return twitchProfile{}, fmt.Errorf("failed to refresh token: %w", err)
		}
		return e.fetchTwitchProfile(ctx, userID)
	}

	if resp.StatusCode != http.StatusOK {
		return twitchProfile{}, fmt.Errorf("twitch API returned status %d", resp.StatusCode)
	}

	var helixResp TwitchHelixUser
	if err := json.NewDecoder(resp.Body).Decode(&helixResp); err != nil {
		return twitchProfile{}, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(helixResp.Data) == 0 {
		return twitchProfile{}, fmt.Errorf("user not found")
	}

	return twitchProfile{
		AvatarURL:   helixResp.Data[0].ProfileImageURL,
		DisplayName: helixResp.Data[0].DisplayName,
	}, nil
}

// refreshAccessToken gets a new app access token from Twitch. The HTTP round-trip is
// performed without holding e.mu; only the final store takes the write lock, so a slow
// token refresh never blocks concurrent avatar lookups.
func (e *AvatarEnricher) refreshAccessToken(ctx context.Context) error {
	url := fmt.Sprintf("%s?client_id=%s&client_secret=%s&grant_type=client_credentials",
		e.tokenURL, e.clientID, e.clientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token refresh failed with status %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return err
	}

	e.setToken(tokenResp.AccessToken)
	e.logger.Info("Refreshed Twitch app access token",
		zap.Int("expires_in", tokenResp.ExpiresIn),
	)

	return nil
}
