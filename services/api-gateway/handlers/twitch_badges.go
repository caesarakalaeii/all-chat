package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	twitchBadgeGlobalURL  = "https://api.twitch.tv/helix/chat/badges/global"
	twitchBadgeChannelURL = "https://api.twitch.tv/helix/chat/badges"
	twitchOAuthTokenURL   = "https://id.twitch.tv/oauth2/token"
	badgeCacheTTL         = 15 * time.Minute
	tokenRefreshBuffer    = 5 * time.Minute // Refresh token 5 minutes before expiry
)

type badgeCacheEntry struct {
	data    []byte
	expires time.Time
}

// Helix API response structures
type helixBadgeVersion struct {
	ID          string `json:"id"`
	ImageURL1x  string `json:"image_url_1x"`
	ImageURL2x  string `json:"image_url_2x"`
	ImageURL4x  string `json:"image_url_4x"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type helixBadgeSet struct {
	SetID    string              `json:"set_id"`
	Versions []helixBadgeVersion `json:"versions"`
}

type helixBadgeResponse struct {
	Data []helixBadgeSet `json:"data"`
}

// V1 API response structures (what frontend expects)
type v1BadgeVersion struct {
	ID         string `json:"id"`
	ImageURL1x string `json:"image_url_1x"`
	ImageURL2x string `json:"image_url_2x"`
	ImageURL4x string `json:"image_url_4x"`
}

type v1BadgeSet struct {
	Versions map[string]v1BadgeVersion `json:"versions"`
}

type v1BadgeResponse struct {
	BadgeSets map[string]v1BadgeSet `json:"badge_sets"`
}

// TwitchBadgeHandler proxies badge requests through our API gateway.
type TwitchBadgeHandler struct {
	httpClient   *http.Client
	log          *zap.Logger
	cacheMux     sync.RWMutex
	cache        map[string]badgeCacheEntry
	clientID     string
	clientSecret string
	tokenMux     sync.RWMutex
	accessToken  string
	tokenExpiry  time.Time
}

// NewTwitchBadgeHandler creates a new badge handler with sensible defaults.
// If clientSecret is provided, it will automatically refresh the app access token.
// If clientSecret is empty, it will use the provided accessToken without refresh.
func NewTwitchBadgeHandler(log *zap.Logger, clientID, clientSecret string) *TwitchBadgeHandler {
	handler := &TwitchBadgeHandler{
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		log:          log.Named("twitch-badges"),
		cache:        make(map[string]badgeCacheEntry),
		clientID:     clientID,
		clientSecret: clientSecret,
		tokenExpiry:  time.Now(), // Force immediate refresh
	}

	// If client secret is provided, get initial app access token
	if clientSecret != "" {
		if err := handler.refreshAppAccessToken(); err != nil {
			log.Warn("Failed to get initial Twitch app access token, will retry on first request",
				zap.Error(err))
		} else {
			log.Info("Successfully obtained Twitch app access token for badge fetching")
		}
	}

	return handler
}

// GetGlobalBadges proxies the Twitch global badge list.
func (h *TwitchBadgeHandler) GetGlobalBadges(c *gin.Context) {
	h.serveBadges(c, twitchBadgeGlobalURL, "global")
}

// GetChannelBadges proxies the Twitch channel-specific badge list.
func (h *TwitchBadgeHandler) GetChannelBadges(c *gin.Context) {
	roomID := strings.TrimSpace(c.Param("room_id"))
	if roomID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "room_id is required"})
		return
	}

	cacheKey := fmt.Sprintf("channel:%s", roomID)
	url := fmt.Sprintf("%s?broadcaster_id=%s", twitchBadgeChannelURL, roomID)
	h.serveBadges(c, url, cacheKey)
}

func (h *TwitchBadgeHandler) serveBadges(c *gin.Context, url, cacheKey string) {
	payload, err := h.getBadgePayload(url, cacheKey)
	if err != nil {
		h.log.Warn("Failed to fetch Twitch badges", zap.String("url", url), zap.Error(err))
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch Twitch badges"})
		return
	}

	c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", int(badgeCacheTTL.Seconds())))
	c.Data(http.StatusOK, "application/json", payload)
}

func (h *TwitchBadgeHandler) getBadgePayload(url, cacheKey string) ([]byte, error) {
	if data, ok := h.getFromCache(cacheKey); ok {
		return data, nil
	}

	// Ensure we have a valid token before making the request
	if h.clientSecret != "" {
		if err := h.ensureValidToken(); err != nil {
			return nil, fmt.Errorf("failed to ensure valid token: %w", err)
		}
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Client-ID", h.clientID)

	h.tokenMux.RLock()
	token := h.accessToken
	h.tokenMux.RUnlock()

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("twitch badge request failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Parse Helix response
	var helixResp helixBadgeResponse
	if err := json.Unmarshal(body, &helixResp); err != nil {
		return nil, fmt.Errorf("failed to parse helix response: %w", err)
	}

	// Transform to v1 format
	v1Resp := v1BadgeResponse{
		BadgeSets: make(map[string]v1BadgeSet),
	}

	for _, badgeSet := range helixResp.Data {
		versions := make(map[string]v1BadgeVersion)
		for _, version := range badgeSet.Versions {
			versions[version.ID] = v1BadgeVersion{
				ID:         version.ID,
				ImageURL1x: version.ImageURL1x,
				ImageURL2x: version.ImageURL2x,
				ImageURL4x: version.ImageURL4x,
			}
		}
		v1Resp.BadgeSets[badgeSet.SetID] = v1BadgeSet{
			Versions: versions,
		}
	}

	// Marshal v1 response
	payload, err := json.Marshal(v1Resp)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal v1 response: %w", err)
	}

	h.saveToCache(cacheKey, payload)
	return payload, nil
}

func (h *TwitchBadgeHandler) getFromCache(path string) ([]byte, bool) {
	h.cacheMux.RLock()
	entry, ok := h.cache[path]
	h.cacheMux.RUnlock()

	if !ok || time.Now().After(entry.expires) {
		return nil, false
	}
	return entry.data, true
}

func (h *TwitchBadgeHandler) saveToCache(path string, data []byte) {
	h.cacheMux.Lock()
	h.cache[path] = badgeCacheEntry{
		data:    data,
		expires: time.Now().Add(badgeCacheTTL),
	}
	h.cacheMux.Unlock()
}

// ensureValidToken checks if the current token is valid and refreshes if needed
func (h *TwitchBadgeHandler) ensureValidToken() error {
	h.tokenMux.RLock()
	needsRefresh := time.Now().Add(tokenRefreshBuffer).After(h.tokenExpiry)
	h.tokenMux.RUnlock()

	if needsRefresh {
		return h.refreshAppAccessToken()
	}
	return nil
}

// refreshAppAccessToken obtains a new App Access Token using Client Credentials flow
// https://dev.twitch.tv/docs/authentication/getting-tokens-oauth/#client-credentials-grant-flow
func (h *TwitchBadgeHandler) refreshAppAccessToken() error {
	h.log.Info("Refreshing Twitch app access token")

	// Build OAuth request
	url := fmt.Sprintf("%s?client_id=%s&client_secret=%s&grant_type=client_credentials",
		twitchOAuthTokenURL, h.clientID, h.clientSecret)

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create token request: %w", err)
	}

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("token request failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	// Parse token response
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"` // Seconds until expiry
		TokenType   string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to parse token response: %w", err)
	}

	// Update token with write lock
	h.tokenMux.Lock()
	h.accessToken = tokenResp.AccessToken
	h.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	h.tokenMux.Unlock()

	h.log.Info("Successfully refreshed Twitch app access token",
		zap.Time("expires_at", h.tokenExpiry),
		zap.Duration("valid_for", time.Until(h.tokenExpiry)),
	)

	return nil
}
