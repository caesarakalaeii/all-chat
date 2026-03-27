package enricher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

	// AvatarImageMaxBytes is the maximum size of a cached avatar image
	AvatarImageMaxBytes = 256 * 1024 // 256KB
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

// AvatarEnricher fetches and caches user avatars from Twitch Helix API
// and caches TikTok avatar images (which have expiring CDN URLs).
type AvatarEnricher struct {
	httpClient     *http.Client
	redisClient    *redis.Client
	clientID       string
	clientSecret   string
	accessToken    string
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
		gatewayBaseURL: gatewayBaseURL,
		logger:         logger,
	}
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
	// Check if already has avatar
	if msg.User.AvatarURL != "" && msg.User.AvatarURL != "https://static-cdn.jtvnw.net/jtv_user_pictures/-profile_image-70x70.png" {
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
	avatarURL, err := e.fetchAvatarFromTwitch(ctx, msg.User.ID)
	if err != nil {
		e.logger.Warn("Failed to fetch avatar from Twitch",
			zap.String("user_id", msg.User.ID),
			zap.String("username", msg.User.Username),
			zap.Error(err),
		)
		return nil // Don't fail the whole message
	}

	// Update message
	msg.User.AvatarURL = avatarURL

	// Cache for 24 hours
	e.redisClient.Set(ctx, cacheKey, avatarURL, AvatarCacheTTL)

	return nil
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

// fetchAvatarFromTwitch fetches avatar URL from Twitch Helix API
func (e *AvatarEnricher) fetchAvatarFromTwitch(ctx context.Context, userID string) (string, error) {
	// Ensure we have an access token
	if e.accessToken == "" {
		if err := e.refreshAccessToken(ctx); err != nil {
			return "", fmt.Errorf("failed to get access token: %w", err)
		}
	}

	// Call Twitch Helix API
	url := fmt.Sprintf("https://api.twitch.tv/helix/users?id=%s", userID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.accessToken))
	req.Header.Set("Client-Id", e.clientID)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		// Token expired, refresh and retry
		if err := e.refreshAccessToken(ctx); err != nil {
			return "", fmt.Errorf("failed to refresh token: %w", err)
		}
		return e.fetchAvatarFromTwitch(ctx, userID)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("twitch API returned status %d", resp.StatusCode)
	}

	var helixResp TwitchHelixUser
	if err := json.NewDecoder(resp.Body).Decode(&helixResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(helixResp.Data) == 0 {
		return "", fmt.Errorf("user not found")
	}

	return helixResp.Data[0].ProfileImageURL, nil
}

// refreshAccessToken gets a new app access token from Twitch
func (e *AvatarEnricher) refreshAccessToken(ctx context.Context) error {
	url := fmt.Sprintf("https://id.twitch.tv/oauth2/token?client_id=%s&client_secret=%s&grant_type=client_credentials",
		e.clientID, e.clientSecret)

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

	e.accessToken = tokenResp.AccessToken
	e.logger.Info("Refreshed Twitch app access token",
		zap.Int("expires_in", tokenResp.ExpiresIn),
	)

	return nil
}
