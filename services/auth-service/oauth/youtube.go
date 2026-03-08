package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// YouTubeOAuth handles YouTube/Google OAuth 2.0 flow
type YouTubeOAuth struct {
	config *oauth2.Config
	client *http.Client
	apiKey string
}

// YouTubeChannelInfo contains channel metadata resolved via the YouTube API
type YouTubeChannelInfo struct {
	ChannelID string
	Title     string
	Handle    string
}

// NewYouTubeOAuth creates a new YouTube OAuth handler
func NewYouTubeOAuth(clientID, clientSecret, redirectURL, apiKey string) *YouTubeOAuth {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/youtube.readonly",
			"https://www.googleapis.com/auth/youtube.force-ssl",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	return &YouTubeOAuth{
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
		apiKey: apiKey,
	}
}

// GetAuthURL generates the OAuth authorization URL
// Uses "select_account" prompt to support incremental authorization.
// This allows users to choose their account without forcing re-consent on every login,
// which is required for Google OAuth verification.
func (y *YouTubeOAuth) GetAuthURL(state string) string {
	return y.config.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "select_account"),
	)
}

// ExchangeCode exchanges authorization code for tokens
func (y *YouTubeOAuth) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := y.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	return token, nil
}

// GetPlatform returns the platform identifier
func (y *YouTubeOAuth) GetPlatform() Platform {
	return PlatformYouTube
}

// GetPrimaryChannel fetches the authenticated user's primary YouTube channel information
func (y *YouTubeOAuth) GetPrimaryChannel(ctx context.Context, accessToken string) (*YouTubeChannelInfo, error) {
	// When using OAuth with mine=true, don't include API key - OAuth token is sufficient
	url := "https://www.googleapis.com/youtube/v3/channels?part=snippet&mine=true"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := y.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query channels: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("youtube channels API returned status %d: %s (request URL: %s)", resp.StatusCode, string(body), url)
	}

	var result struct {
		Items []struct {
			ID      string `json:"id"`
			Snippet struct {
				Title     string `json:"title"`
				CustomUrl string `json:"customUrl"`
			} `json:"snippet"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode channel response: %w", err)
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("no youtube channels available for this account")
	}

	channel := result.Items[0]
	if channel.ID == "" {
		return nil, fmt.Errorf("youtube channel response missing id")
	}

	return &YouTubeChannelInfo{
		ChannelID: channel.ID,
		Title:     channel.Snippet.Title,
		Handle:    channel.Snippet.CustomUrl,
	}, nil
}

// GetUserInfo fetches user information from Google API (returns platform-specific type)
func (y *YouTubeOAuth) GetUserInfoYouTube(ctx context.Context, accessToken string) (*models.YouTubeUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://www.googleapis.com/oauth2/v2/userinfo", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := y.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("google API returned status %d: %s", resp.StatusCode, string(body))
	}

	var userInfo models.YouTubeUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &userInfo, nil
}

// GetUserInfo fetches user information (generic interface implementation)
func (y *YouTubeOAuth) GetUserInfo(ctx context.Context, accessToken string) (PlatformUserInfo, error) {
	youtubeInfo, err := y.GetUserInfoYouTube(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	return &YouTubeUserInfoWrapper{
		ID:      youtubeInfo.ID,
		Name:    youtubeInfo.Name,
		Picture: youtubeInfo.Picture,
	}, nil
}

// RefreshToken refreshes an OAuth token
func (y *YouTubeOAuth) RefreshToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}

	tokenSource := y.config.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	return newToken, nil
}
