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
	"golang.org/x/oauth2/twitch"
)

// TwitchOAuth handles Twitch OAuth 2.0 flow
type TwitchOAuth struct {
	config *oauth2.Config
	client *http.Client
}

// NewTwitchOAuth creates a new Twitch OAuth handler
func NewTwitchOAuth(clientID, clientSecret, redirectURL string) *TwitchOAuth {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			"channel:read:redemptions",   // Required for EventSub channel points
			"channel:read:subscriptions", // Required for EventSub subscription events
			"bits:read",                  // Required for EventSub cheer/bits events
			"moderator:read:followers",   // Required for EventSub follow events
		},
		Endpoint: twitch.Endpoint,
	}

	return &TwitchOAuth{
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetAuthURL generates the OAuth authorization URL
func (t *TwitchOAuth) GetAuthURL(state string) string {
	return t.config.AuthCodeURL(state)
}

// ExchangeCode exchanges authorization code for tokens
func (t *TwitchOAuth) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := t.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	return token, nil
}

// GetPlatform returns the platform identifier
func (t *TwitchOAuth) GetPlatform() Platform {
	return PlatformTwitch
}

// GetUserInfo fetches user information from Twitch API (returns platform-specific type)
func (t *TwitchOAuth) GetUserInfoTwitch(ctx context.Context, accessToken string) (*models.TwitchUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.twitch.tv/helix/users", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Client-Id", t.config.ClientID)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("twitch API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []models.TwitchUserInfo `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no user data returned from Twitch")
	}

	return &result.Data[0], nil
}

// GetUserInfo fetches user information (generic interface implementation)
func (t *TwitchOAuth) GetUserInfo(ctx context.Context, accessToken string) (PlatformUserInfo, error) {
	twitchInfo, err := t.GetUserInfoTwitch(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	return &TwitchUserInfoWrapper{
		ID:              twitchInfo.ID,
		Login:           twitchInfo.Login,
		DisplayName:     twitchInfo.DisplayName,
		ProfileImageURL: twitchInfo.ProfileImageURL,
	}, nil
}

// RefreshToken refreshes an OAuth token
func (t *TwitchOAuth) RefreshToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}

	tokenSource := t.config.TokenSource(ctx, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	return newToken, nil
}
