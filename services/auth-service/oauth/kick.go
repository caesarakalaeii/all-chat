// This file is part of All-Chat.
// Copyright (C) 2026 caesarakalaeii
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
	// tokenURL is Kick's token endpoint, overridable as a test seam (a test can point the
	// exchange at a stub server). Defaults to kickTokenURL; there is a test pinning that.
	tokenURL string
}

// Kick OAuth endpoints
const (
	kickAuthURL  = "https://id.kick.com/oauth/authorize"
	kickTokenURL = "https://id.kick.com/oauth/token"
	kickUserURL  = "https://api.kick.com/public/v1/users"
)

// NewKickOAuth creates a new Kick OAuth handler
func NewKickOAuth(clientID, clientSecret, redirectURL string) *KickOAuth {
	return &KickOAuth{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		client:       &http.Client{Timeout: 10 * time.Second},
		tokenURL:     kickTokenURL,
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
	// Streamer flow only needs `user:read` for identity. `chat:read` is
	// unused — kick-listener consumes chat over WebSocket without an OAuth
	// gate. `channel:read` had no caller. See ADR 0012.
	params.Set("scope", "user:read")
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")

	// Store code verifier in state for later use during token exchange
	// In production, you should store this in a session or cache
	// For now, we'll append it to the state (separated by a delimiter)
	// The auth handler will need to extract and use it

	return kickAuthURL + "?" + params.Encode()
}

// KickSendScope authorizes the Kick public "Send Chat Message" API
// (POST /public/v1/chat). Requested ONLY through the opt-in advanced-controls
// re-consent (alongside moderation:ban), never bundled into login/add-source.
const KickSendScope = "chat:write"

// kickModerationScopeByAction maps a moderation action to the Kick OAuth scope it
// requires. Kick splits moderation across two scopes: ban/timeout/unban behind
// moderation:ban, and single-message delete behind moderation:chat_message:manage
// (DELETE /public/v1/chat/{message_id}). Requested ONLY through the opt-in moderation
// re-consent flow, never bundled into login/add-source (ADR-0017).
//
// Because they are separate grants, a streamer who consented before delete existed holds
// only moderation:ban — enabling delete asks them to re-consent for the second scope, and
// until they do their token legitimately cannot delete.
var kickModerationScopeByAction = map[string]string{
	"delete":  "moderation:chat_message:manage",
	"timeout": "moderation:ban",
	"ban":     "moderation:ban",
	"unban":   "moderation:ban",
}

// KickModerationScopesForActions returns the deduped, minimal set of Kick scopes the
// given moderation actions require. Unknown actions (incl. "delete", unsupported on
// Kick) are ignored, so an empty/garbage query yields no scopes (the caller rejects it).
func KickModerationScopesForActions(actions []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, 1)
	for _, a := range actions {
		if scope, ok := kickModerationScopeByAction[a]; ok && !seen[scope] {
			seen[scope] = true
			out = append(out, scope)
		}
	}
	return out
}

// GetAuthURLWithScopesPKCE builds a Kick consent URL (PKCE) requesting the base login
// scope plus `extra` (deduped), and returns the code verifier the caller must store for
// the token exchange. The moderation re-consent passes extra = (existing granted scopes
// ∪ moderation:ban), so the issued token is a SUPERSET of the stored grant and never
// trips the scope-downgrade guard.
func (k *KickOAuth) GetAuthURLWithScopesPKCE(state string, extra []string) (authURL string, codeVerifier string) {
	codeVerifier = generateCodeVerifier()
	codeChallenge := generateCodeChallenge(codeVerifier)

	seen := make(map[string]bool)
	scopes := make([]string, 0, len(extra)+1)
	add := func(list []string) {
		for _, s := range list {
			if s != "" && !seen[s] {
				seen[s] = true
				scopes = append(scopes, s)
			}
		}
	}
	add([]string{"user:read"}) // base identity scope (resolve the streamer's own channel)
	add(extra)

	params := url.Values{}
	params.Set("client_id", k.clientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", k.redirectURL)
	params.Set("state", state)
	params.Set("scope", strings.Join(scopes, " "))
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")

	return kickAuthURL + "?" + params.Encode(), codeVerifier
}

// GetModConsentAuthURLPKCE builds the consent URL for a delegated moderator granting their own
// Kick moderation scopes (ADR-0048), and returns the PKCE verifier the caller must stash.
//
// Kick's counterpart to TwitchOAuth.GetModConsentAuthURL, with one deliberate difference: it does
// request `user:read`. That is not a login bundle — it is the identity read the callback needs to
// know WHICH Kick account just consented, so the credential can be attributed to it. Twitch's base
// scopes (channel points, subscriptions, bits, followers) are what a volunteer must not be asked
// for; a single identity scope is the minimum this flow can work with at all.
//
// Returns "" when no moderation scopes are requested: a consent screen granting nothing would only
// fail later, at the first moderation call, as a confusing missing-scope error.
func (k *KickOAuth) GetModConsentAuthURLPKCE(state string, scopes []string) (authURL string, codeVerifier string) {
	seen := make(map[string]bool)
	minimal := make([]string, 0, len(scopes)+1)
	for _, s := range scopes {
		if s != "" && !seen[s] {
			seen[s] = true
			minimal = append(minimal, s)
		}
	}
	if len(minimal) == 0 {
		return "", ""
	}
	if !seen["user:read"] {
		minimal = append(minimal, "user:read")
	}

	codeVerifier = generateCodeVerifier()
	params := url.Values{}
	params.Set("client_id", k.clientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", k.redirectURL)
	params.Set("state", state)
	params.Set("scope", strings.Join(minimal, " "))
	params.Set("code_challenge", generateCodeChallenge(codeVerifier))
	params.Set("code_challenge_method", "S256")

	return kickAuthURL + "?" + params.Encode(), codeVerifier
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
	// Streamer flow only needs `user:read` for identity. `chat:read` is
	// unused — kick-listener consumes chat over WebSocket without an OAuth
	// gate. `channel:read` had no caller. See ADR 0012.
	params.Set("scope", "user:read")
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

	req, err := http.NewRequestWithContext(ctx, "POST", k.tokenURL, strings.NewReader(data.Encode()))
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

	// Carry the granted scope through where ExtractGrantedScopes looks for it.
	//
	// This response field was parsed and then dropped, which made every Kick grant land with an
	// EMPTY granted_scopes: the moderation re-consent appeared to succeed while the capability
	// endpoint kept reporting missing_scope and the dispatcher's pre-check refused every Kick
	// action. Nothing downstream could recover it, because a refresh grant deliberately never
	// widens scopes. The `oauth2.Token` construction is hand-rolled here (Kick's OAuth 2.1 + PKCE
	// flow does not go through oauth2.Config.Exchange, which would have populated Extra itself),
	// so the field has to be attached explicitly.
	if result.Scope != "" {
		token = token.WithExtra(map[string]interface{}{"scope": result.Scope})
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
		// Log the actual response body for debugging
		return nil, fmt.Errorf("failed to decode response (status %d, body preview: %.200s...): %w", resp.StatusCode, string(body), err)
	}

	if len(response.Data) == 0 {
		return nil, fmt.Errorf("kick API returned empty user array")
	}

	return &response.Data[0], nil
}

// GetUserInfo fetches user information (generic interface implementation)
func (k *KickOAuth) GetUserInfo(ctx context.Context, accessToken string) (PlatformUserInfo, error) {
	kickInfo, err := k.GetUserInfoKick(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	return &KickUserInfoWrapper{
		ID:          fmt.Sprintf("%d", kickInfo.UserID),
		Username:    kickInfo.Name,
		DisplayName: kickInfo.Name,
		ProfilePic:  kickInfo.ProfilePicture,
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
