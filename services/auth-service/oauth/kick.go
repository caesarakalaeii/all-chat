package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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

// KickOAuth handles Kick OAuth 2.1 flow with PKCE
type KickOAuth struct {
	clientID     string
	clientSecret string
	redirectURL  string
	client       *http.Client
}

// Kick OAuth endpoints
const (
	kickAuthURL  = "https://id.kick.com/oauth/authorize"
	kickTokenURL = "https://id.kick.com/oauth/token"
	kickUserURL  = "https://kick.com/api/v2/user"
)

// NewKickOAuth creates a new Kick OAuth handler
func NewKickOAuth(clientID, clientSecret, redirectURL string) *KickOAuth {
	return &KickOAuth{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		client:       &http.Client{Timeout: 10 * time.Second},
	}
}

// GetAuthURL generates the OAuth authorization URL with PKCE
// Kick requires OAuth 2.1 with PKCE (Proof Key for Code Exchange)
// Scopes: chat:read, chat:write, channel:read, user:read, etc.
func (k *KickOAuth) GetAuthURL(state string) string {
	// Generate PKCE code verifier (43-128 character random string)
	codeVerifier := generateCodeVerifier()

	// Generate code challenge (SHA256 hash of verifier, base64url encoded)
	codeChallenge := generateCodeChallenge(codeVerifier)

	params := url.Values{}
	params.Set("client_id", k.clientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", k.redirectURL)
	params.Set("state", state)
	params.Set("scope", "chat:read user:read channel:read")
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")

	// Store code verifier in state for later use during token exchange
	// In production, you should store this in a session or cache
	// For now, we'll append it to the state (separated by a delimiter)
	// The auth handler will need to extract and use it

	return kickAuthURL + "?" + params.Encode()
}

// GetAuthURLWithPKCE generates the OAuth authorization URL and returns both URL and code verifier
// The caller must store the code verifier to use it during token exchange
func (k *KickOAuth) GetAuthURLWithPKCE(state string) (authURL string, codeVerifier string) {
	// Generate PKCE code verifier (43-128 character random string)
	codeVerifier = generateCodeVerifier()

	// Generate code challenge (SHA256 hash of verifier, base64url encoded)
	codeChallenge := generateCodeChallenge(codeVerifier)

	params := url.Values{}
	params.Set("client_id", k.clientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", k.redirectURL)
	params.Set("state", state)
	params.Set("scope", "chat:read user:read channel:read")
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")

	authURL = kickAuthURL + "?" + params.Encode()
	return authURL, codeVerifier
}

// ExchangeCode exchanges authorization code for tokens using PKCE
func (k *KickOAuth) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	// Note: This method signature doesn't support code_verifier parameter
	// Use ExchangeCodeWithPKCE instead for Kick OAuth
	return nil, fmt.Errorf("use ExchangeCodeWithPKCE for Kick OAuth - PKCE code_verifier required")
}

// ExchangeCodeWithPKCE exchanges authorization code for tokens using PKCE code verifier
func (k *KickOAuth) ExchangeCodeWithPKCE(ctx context.Context, code string, codeVerifier string) (*oauth2.Token, error) {
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

// GetPlatform returns the platform identifier
func (k *KickOAuth) GetPlatform() Platform {
	return PlatformKick
}

// GetUserInfoKick fetches user information from Kick API (returns platform-specific type)
func (k *KickOAuth) GetUserInfoKick(ctx context.Context, accessToken string) (*models.KickUserInfo, error) {
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

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("kick API returned status %d: %s", resp.StatusCode, string(body))
	}

	var userInfo models.KickUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &userInfo, nil
}

// GetUserInfo fetches user information (generic interface implementation)
func (k *KickOAuth) GetUserInfo(ctx context.Context, accessToken string) (PlatformUserInfo, error) {
	kickInfo, err := k.GetUserInfoKick(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	return &KickUserInfoWrapper{
		ID:              fmt.Sprintf("%d", kickInfo.ID),
		Username:        kickInfo.Username,
		Slug:            kickInfo.Slug,
		Bio:             kickInfo.Bio,
		ProfilePic:      kickInfo.ProfilePic,
		Email:           kickInfo.Email,
	}, nil
}

// RefreshToken refreshes an OAuth token
func (k *KickOAuth) RefreshToken(ctx context.Context, refreshToken string) (*oauth2.Token, error) {
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

// generateCodeVerifier generates a cryptographically random code verifier
// Must be 43-128 characters long, using [A-Z], [a-z], [0-9], "-", ".", "_", "~"
func generateCodeVerifier() string {
	b := make([]byte, 32) // 32 bytes = 43 base64url characters
	if _, err := rand.Read(b); err != nil {
		panic(err) // Should never happen
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// generateCodeChallenge generates a code challenge from the verifier
// Uses SHA256 hash and base64url encoding (S256 method)
func generateCodeChallenge(verifier string) string {
	h := sha256.New()
	h.Write([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
