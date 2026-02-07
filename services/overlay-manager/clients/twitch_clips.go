package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	twitchClipsURL = "https://api.twitch.tv/helix/clips"
)

// ClipData represents a single clip from Twitch API
type ClipData struct {
	ID           string  `json:"id"`
	URL          string  `json:"url"`
	EmbedURL     string  `json:"embed_url"`
	BroadcasterID string `json:"broadcaster_id"`
	Title        string  `json:"title"`
	ViewCount    int     `json:"view_count"`
	CreatedAt    string  `json:"created_at"` // RFC3339
	ThumbnailURL string  `json:"thumbnail_url"`
	Duration     float64 `json:"duration"`
}

// ClipsResponse represents the Twitch clips API response
type ClipsResponse struct {
	Data []ClipData `json:"data"`
}

// TwitchClipsClient handles fetching clips from Twitch Helix API
type TwitchClipsClient struct {
	clientID     string
	clientSecret string
	httpClient   *http.Client
	logger       *zap.Logger

	// Token management
	tokenMu     sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

// NewTwitchClipsClient creates a new Twitch clips client
func NewTwitchClipsClient(clientID, clientSecret string, logger *zap.Logger) *TwitchClipsClient {
	return &TwitchClipsClient{
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger.With(zap.String("component", "twitch-clips-client")),
	}
}

// GetClips fetches clips for a broadcaster within a time range
func (c *TwitchClipsClient) GetClips(ctx context.Context, broadcasterID string, startedAt, endedAt time.Time, maxCount int) ([]ClipData, error) {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token: %w", err)
	}

	params := url.Values{}
	params.Add("broadcaster_id", broadcasterID)
	if !startedAt.IsZero() {
		params.Add("started_at", startedAt.Format(time.RFC3339))
	}
	if !endedAt.IsZero() {
		params.Add("ended_at", endedAt.Format(time.RFC3339))
	}
	if maxCount > 0 {
		if maxCount > 100 {
			maxCount = 100
		}
		params.Add("first", fmt.Sprintf("%d", maxCount))
	}

	urlStr := twitchClipsURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Client-Id", c.clientID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call clips API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("twitch API returned status %d", resp.StatusCode)
	}

	var clipResp ClipsResponse
	if err := json.NewDecoder(resp.Body).Decode(&clipResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return clipResp.Data, nil
}

// getAccessToken gets or refreshes app access token using client credentials flow
func (c *TwitchClipsClient) getAccessToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	// Return cached token if still valid
	if c.accessToken != "" && time.Until(c.tokenExpiry) > 30*time.Second {
		return c.accessToken, nil
	}

	// OAuth client credentials flow
	form := url.Values{}
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://id.twitch.tv/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to request token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned status %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("received empty access token")
	}

	c.accessToken = tokenResp.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return c.accessToken, nil
}
