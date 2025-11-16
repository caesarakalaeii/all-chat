package enricher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

// TwitchChannelBadgeResponse represents channel-specific badge metadata
type TwitchChannelBadgeResponse struct {
	BadgeSets map[string]struct {
		Versions map[string]struct {
			ID         string `json:"id"`
			ImageURL1x string `json:"image_url_1x"`
			ImageURL2x string `json:"image_url_2x"`
			ImageURL4x string `json:"image_url_4x"`
		} `json:"versions"`
	} `json:"badge_sets"`
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
	var channelBadges map[string]TwitchBadgeSet
	channelIdentifier := e.extractChannelIdentifier(msg)
	if channelIdentifier != "" {
		channelBadges, err = e.getChannelBadges(ctx, channelIdentifier)
		if err != nil {
			e.logger.Debug("Failed to get channel badges",
				zap.String("channel", channelIdentifier),
				zap.Error(err),
			)
			// Continue with just global badges
		}
	} else {
		e.logger.Debug("Skipping channel badges - missing Twitch room ID",
			zap.String("channel", msg.ChannelID),
		)
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
func (e *BadgeEnricher) getChannelBadges(ctx context.Context, channelID string) (map[string]TwitchBadgeSet, error) {
	if channelID == "" {
		return nil, fmt.Errorf("channel ID is required for channel badges")
	}

	cacheKey := ChannelBadgesCacheKeyPrefix + channelID
	if cached, err := e.redisClient.Get(ctx, cacheKey).Result(); err == nil && cached != "" {
		var badges map[string]TwitchBadgeSet
		if err := json.Unmarshal([]byte(cached), &badges); err == nil {
			return badges, nil
		}
	}

	url := fmt.Sprintf("https://badges.twitch.tv/v1/badges/channels/%s/display", channelID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("channel badges not found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("channel badges API returned status %d", resp.StatusCode)
	}

	var badgeResp TwitchChannelBadgeResponse
	if err := json.NewDecoder(resp.Body).Decode(&badgeResp); err != nil {
		return nil, err
	}

	if len(badgeResp.BadgeSets) == 0 {
		return nil, fmt.Errorf("channel badges response empty")
	}

	badges := make(map[string]TwitchBadgeSet, len(badgeResp.BadgeSets))
	for setID, badgeSet := range badgeResp.BadgeSets {
		versions := make(map[string]TwitchBadgeVer, len(badgeSet.Versions))
		for versionID, ver := range badgeSet.Versions {
			versions[versionID] = TwitchBadgeVer{
				ImageURL1x: ver.ImageURL1x,
				ImageURL2x: ver.ImageURL2x,
				ImageURL4x: ver.ImageURL4x,
			}
		}
		badges[setID] = TwitchBadgeSet{
			SetID:    setID,
			Versions: versions,
		}
	}

	jsonBytes, _ := json.Marshal(badges)
	e.redisClient.Set(ctx, cacheKey, string(jsonBytes), BadgeCacheTTL)

	return badges, nil
}

// extractChannelIdentifier returns the Twitch room ID used by the badges CDN
func (e *BadgeEnricher) extractChannelIdentifier(msg *models.UnifiedChatMessage) string {
	if msg == nil || msg.Metadata == nil {
		return ""
	}
	if roomID, ok := msg.Metadata["twitch_room_id"]; ok {
		var raw string
		switch v := roomID.(type) {
		case string:
			raw = v
		case fmt.Stringer:
			raw = v.String()
		default:
			raw = fmt.Sprint(v)
		}
		trimmed := strings.TrimSpace(raw)
		if trimmed != "" && trimmed != "<nil>" {
			return trimmed
		}
	}
	return ""
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
