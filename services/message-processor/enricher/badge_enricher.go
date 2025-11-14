package enricher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/caesar/all-chat/services/message-processor/models"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// BadgeCacheTTL is how long to cache badge definitions (24 hours)
	BadgeCacheTTL = 24 * time.Hour

	// GlobalBadgesCacheKey is the Redis key for global badges
	GlobalBadgesCacheKey = "badges:global"

	// ChannelBadgesCacheKeyPrefix is the Redis key prefix for channel badges
	ChannelBadgesCacheKeyPrefix = "badges:channel:"
)

// TwitchBadgeSet represents a set of badge versions
type TwitchBadgeSet struct {
	SetID    string                    `json:"set_id"`
	Versions map[string]TwitchBadgeVer `json:"versions"`
}

// TwitchBadgeVer represents a specific badge version
type TwitchBadgeVer struct {
	ImageURL1x string `json:"image_url_1x"`
	ImageURL2x string `json:"image_url_2x"`
	ImageURL4x string `json:"image_url_4x"`
}

// TwitchBadgeResponse is the response from Twitch Helix badges API
type TwitchBadgeResponse struct {
	Data []struct {
		SetID    string `json:"set_id"`
		Versions []struct {
			ID         string `json:"id"`
			ImageURL1x string `json:"image_url_1x"`
			ImageURL2x string `json:"image_url_2x"`
			ImageURL4x string `json:"image_url_4x"`
		} `json:"versions"`
	} `json:"data"`
}

// BadgeEnricher enriches messages with proper Twitch badge icon URLs
type BadgeEnricher struct {
	httpClient   *http.Client
	redisClient  *redis.Client
	clientID     string
	clientSecret string
	accessToken  string
	logger       *zap.Logger
	mu           sync.RWMutex
}

// NewBadgeEnricher creates a new badge enricher
func NewBadgeEnricher(redisClient *redis.Client, clientID, clientSecret string, logger *zap.Logger) *BadgeEnricher {
	return &BadgeEnricher{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		redisClient:  redisClient,
		clientID:     clientID,
		clientSecret: clientSecret,
		logger:       logger,
	}
}

// Enrich updates badge icon URLs for the message
func (e *BadgeEnricher) Enrich(ctx context.Context, msg *models.UnifiedChatMessage) error {
	// Only for Twitch
	if msg.Platform != "twitch" {
		return nil
	}

	// Load global badges
	globalBadges, err := e.getGlobalBadges(ctx)
	if err != nil {
		e.logger.Warn("Failed to get global badges", zap.Error(err))
		return nil // Don't fail the message
	}

	// Load channel-specific badges (for subscriber tiers, etc.)
	channelBadges, err := e.getChannelBadges(ctx, msg.ChannelID)
	if err != nil {
		e.logger.Debug("Failed to get channel badges",
			zap.String("channel", msg.ChannelID),
			zap.Error(err),
		)
		// Continue with just global badges
	}

	// Update badge URLs
	for i := range msg.User.Badges {
		badge := &msg.User.Badges[i]

		// Try channel badges first (for subscriber tiers)
		if channelBadges != nil {
			if badgeSet, ok := channelBadges[badge.Name]; ok {
				if ver, ok := badgeSet.Versions[badge.Version]; ok {
					badge.IconURL = ver.ImageURL1x
					continue
				}
			}
		}

		// Fallback to global badges
		if badgeSet, ok := globalBadges[badge.Name]; ok {
			if ver, ok := badgeSet.Versions[badge.Version]; ok {
				badge.IconURL = ver.ImageURL1x
			}
		}
	}

	return nil
}

// getGlobalBadges fetches global badge definitions from Twitch
func (e *BadgeEnricher) getGlobalBadges(ctx context.Context) (map[string]TwitchBadgeSet, error) {
	// Check cache first
	cached, err := e.redisClient.Get(ctx, GlobalBadgesCacheKey).Result()
	if err == nil && cached != "" {
		var badges map[string]TwitchBadgeSet
		if err := json.Unmarshal([]byte(cached), &badges); err == nil {
			return badges, nil
		}
	}

	// Ensure we have access token
	if e.accessToken == "" {
		if err := e.refreshAccessToken(ctx); err != nil {
			return nil, err
		}
	}

	// Fetch from Twitch API
	url := "https://api.twitch.tv/helix/chat/badges/global"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.accessToken))
	req.Header.Set("Client-Id", e.clientID)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		// Refresh token and retry
		if err := e.refreshAccessToken(ctx); err != nil {
			return nil, err
		}
		return e.getGlobalBadges(ctx)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("badges API returned status %d", resp.StatusCode)
	}

	var badgeResp TwitchBadgeResponse
	if err := json.NewDecoder(resp.Body).Decode(&badgeResp); err != nil {
		return nil, err
	}

	// Convert to map for easy lookup
	badges := make(map[string]TwitchBadgeSet)
	for _, badgeSet := range badgeResp.Data {
		versions := make(map[string]TwitchBadgeVer)
		for _, ver := range badgeSet.Versions {
			versions[ver.ID] = TwitchBadgeVer{
				ImageURL1x: ver.ImageURL1x,
				ImageURL2x: ver.ImageURL2x,
				ImageURL4x: ver.ImageURL4x,
			}
		}
		badges[badgeSet.SetID] = TwitchBadgeSet{
			SetID:    badgeSet.SetID,
			Versions: versions,
		}
	}

	// Cache for 24 hours
	jsonBytes, _ := json.Marshal(badges)
	e.redisClient.Set(ctx, GlobalBadgesCacheKey, string(jsonBytes), BadgeCacheTTL)

	return badges, nil
}

// getChannelBadges fetches channel-specific badge definitions
func (e *BadgeEnricher) getChannelBadges(ctx context.Context, channelName string) (map[string]TwitchBadgeSet, error) {
	// For channel badges, we need the broadcaster ID, not the name
	// This is a limitation - we'd need to look up the ID first
	// For now, skip channel badges and rely on global
	return nil, fmt.Errorf("channel badges require broadcaster ID lookup")
}

// refreshAccessToken gets a new app access token (same as avatar enricher)
func (e *BadgeEnricher) refreshAccessToken(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

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
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return err
	}

	e.accessToken = tokenResp.AccessToken
	e.logger.Debug("Refreshed Twitch access token for badges",
		zap.Int("expires_in", tokenResp.ExpiresIn),
	)

	return nil
}
