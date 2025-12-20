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

// ViewerYouTubeOAuth handles YouTube OAuth for viewers (chat participants)
// Requires youtube.force-ssl scope to insert live chat messages
type ViewerYouTubeOAuth struct {
	config *oauth2.Config
	client *http.Client
}

// NewViewerYouTubeOAuth creates a new YouTube OAuth handler for viewers
func NewViewerYouTubeOAuth(clientID, clientSecret, redirectURL string) *ViewerYouTubeOAuth {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/youtube.force-ssl", // Required for sending messages
			"https://www.googleapis.com/auth/userinfo.profile",  // For user info
		},
		Endpoint: google.Endpoint,
	}

	return &ViewerYouTubeOAuth{
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetAuthURL generates the OAuth authorization URL
func (y *ViewerYouTubeOAuth) GetAuthURL(state string) string {
	return y.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

// ExchangeCode exchanges authorization code for tokens
func (y *ViewerYouTubeOAuth) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := y.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	return token, nil
}

// GetUserInfo fetches the authenticated user's YouTube/Google profile
func (y *ViewerYouTubeOAuth) GetUserInfo(ctx context.Context, accessToken string) (*models.YouTubeUserInfo, error) {
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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("youtube user info request failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var userInfo models.YouTubeUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode user info: %w", err)
	}

	return &userInfo, nil
}

// GetPlatform returns the platform identifier
func (y *ViewerYouTubeOAuth) GetPlatform() Platform {
	return PlatformYouTube
}
