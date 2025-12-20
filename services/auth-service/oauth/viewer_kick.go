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

// ViewerKickOAuth handles Kick OAuth 2.1 flow with PKCE for viewers (with chat write permissions)
type ViewerKickOAuth struct {
	clientID     string
	clientSecret string
	redirectURL  string
	client       *http.Client
}

// NewViewerKickOAuth creates a new Kick OAuth handler for viewers
func NewViewerKickOAuth(clientID, clientSecret, redirectURL string) *ViewerKickOAuth {
	return &ViewerKickOAuth{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		client:       &http.Client{Timeout: 10 * time.Second},
	}
}

// GetAuthURLWithPKCE generates the OAuth authorization URL with chat:write scope and returns both URL and code verifier
// The caller must store the code verifier to use it during token exchange
func (k *ViewerKickOAuth) GetAuthURLWithPKCE(state string) (authURL string, codeVerifier string) {
	// Generate PKCE code verifier (43-128 character random string)
	codeVerifier = generateCodeVerifier()

	// Generate code challenge (SHA256 hash of verifier, base64url encoded)
	codeChallenge := generateCodeChallenge(codeVerifier)

	params := url.Values{}
	params.Set("client_id", k.clientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", k.redirectURL)
	params.Set("state", state)
	params.Set("scope", "chat:read chat:write user:read channel:read") // Added chat:write for sending messages
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")

	authURL = kickAuthURL + "?" + params.Encode()
	return authURL, codeVerifier
}

// ExchangeCodeWithPKCE exchanges authorization code for tokens using PKCE code verifier
func (k *ViewerKickOAuth) ExchangeCodeWithPKCE(ctx context.Context, code string, codeVerifier string) (*oauth2.Token, error) {
	data := url.Values{}
	data.Set("client_id", k.clientID)
	data.Set("client_secret", k.clientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", k.redirectURL)
	data.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, "POST", kickTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := k.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kick token endpoint returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
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

// GetUserInfoKick fetches user information from Kick API
func (k *ViewerKickOAuth) GetUserInfoKick(ctx context.Context, accessToken string) (*models.KickUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", kickUserURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "AllChat/1.0")
	req.Header.Set("Content-Type", "application/json")

	resp, err := k.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("failed to read response body: %w", readErr)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kick API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Kick API returns response wrapped in {"data": [...], "message": "OK"}
	var response struct {
		Data    []models.KickUserInfo `json:"data"`
		Message string                `json:"message"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to decode response (status %d, body preview: %.200s...): %w", resp.StatusCode, string(body), err)
	}

	if len(response.Data) == 0 {
		return nil, fmt.Errorf("kick API returned empty user array")
	}

	return &response.Data[0], nil
}

// GetPlatform returns the platform identifier
func (k *ViewerKickOAuth) GetPlatform() Platform {
	return PlatformKick
}

// RefreshToken refreshes an OAuth token
func (k *ViewerKickOAuth) RefreshToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
	data := url.Values{}
	data.Set("client_id", k.clientID)
	data.Set("client_secret", k.clientSecret)
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, "POST", kickTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := k.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kick refresh endpoint returned status %d: %s", resp.StatusCode, string(body))
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
