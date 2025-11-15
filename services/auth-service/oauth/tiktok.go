package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"golang.org/x/oauth2"
)

// TikTokOAuth handles TikTok OAuth 2.0 flow
// Note: TikTok uses "client_key" instead of "client_id" in their API
type TikTokOAuth struct {
	clientKey    string
	clientSecret string
	redirectURL  string
	client       *http.Client
}

// TikTok OAuth endpoints
const (
	tiktokAuthURL  = "https://www.tiktok.com/v2/auth/authorize/"
	tiktokTokenURL = "https://open.tiktokapis.com/v2/oauth/token/"
	tiktokUserURL  = "https://open.tiktokapis.com/v2/user/info/"
)

// NewTikTokOAuth creates a new TikTok OAuth handler
func NewTikTokOAuth(clientKey, clientSecret, redirectURL string) *TikTokOAuth {
	return &TikTokOAuth{
		clientKey:    clientKey,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		client:       &http.Client{Timeout: 10 * time.Second},
	}
}

// GetAuthURL generates the OAuth authorization URL
// TikTok scopes: user.info.basic is the only scope needed for user authentication
func (t *TikTokOAuth) GetAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_key", t.clientKey)
	params.Set("scope", "user.info.basic")
	params.Set("response_type", "code")
	params.Set("redirect_uri", t.redirectURL)
	params.Set("state", state)

	return tiktokAuthURL + "?" + params.Encode()
}

// ExchangeCode exchanges authorization code for tokens
// TikTok requires client_key instead of client_id
func (t *TikTokOAuth) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	data := url.Values{}
	data.Set("client_key", t.clientKey)
	data.Set("client_secret", t.clientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", t.redirectURL)

	req, err := http.NewRequestWithContext(ctx, "POST", tiktokTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tiktok token endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		OpenID       string `json:"open_id"`
		Scope        string `json:"scope"`
		TokenType    string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	// Convert to oauth2.Token
	token := &oauth2.Token{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenType:    result.TokenType,
		Expiry:       time.Now().Add(time.Duration(result.ExpiresIn) * time.Second),
	}

	return token, nil
}

// GetPlatform returns the platform identifier
func (t *TikTokOAuth) GetPlatform() Platform {
	return PlatformTikTok
}

// GetUserInfoTikTok fetches user information from TikTok API (returns platform-specific type)
func (t *TikTokOAuth) GetUserInfoTikTok(ctx context.Context, accessToken string) (*models.TikTokUserInfo, error) {
	// TikTok requires fields parameter
	reqURL := tiktokUserURL + "?fields=open_id,union_id,avatar_url,display_name"

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tiktok API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data struct {
			User models.TikTokUserInfo `json:"user"`
		} `json:"data"`
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error.Code != "" {
		return nil, fmt.Errorf("tiktok API error: %s - %s", result.Error.Code, result.Error.Message)
	}

	return &result.Data.User, nil
}

// GetUserInfo fetches user information (generic interface implementation)
func (t *TikTokOAuth) GetUserInfo(ctx context.Context, accessToken string) (PlatformUserInfo, error) {
	tiktokInfo, err := t.GetUserInfoTikTok(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	return &TikTokUserInfoWrapper{
		OpenID:      tiktokInfo.OpenID,
		UnionID:     tiktokInfo.UnionID,
		DisplayName: tiktokInfo.DisplayName,
		AvatarURL:   tiktokInfo.AvatarURL,
	}, nil
}

// RefreshToken refreshes an OAuth token
// Note: TikTok may return a new refresh_token, which must be used for subsequent refreshes
func (t *TikTokOAuth) RefreshToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	data := url.Values{}
	data.Set("client_key", t.clientKey)
	data.Set("client_secret", t.clientSecret)
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, "POST", tiktokTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tiktok refresh endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		TokenType    string `json:"token_type"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode refresh response: %w", err)
	}

	// Convert to oauth2.Token
	token := &oauth2.Token{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenType:    result.TokenType,
		Expiry:       time.Now().Add(time.Duration(result.ExpiresIn) * time.Second),
	}

	return token, nil
}
