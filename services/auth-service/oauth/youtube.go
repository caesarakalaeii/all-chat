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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/auth-service/models"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// YouTubeReadonlyScope is the Google OAuth scope required to resolve a streamer's
// YouTube channel (channels?part=...&mine=true) in GetPrimaryChannel and used by the
// youtube-listener for polling. Google's granular consent screen lets a user approve
// the profile scope while declining this one, so the callback must verify it was
// actually granted before relying on it. See ADR 0012.
const YouTubeReadonlyScope = "https://www.googleapis.com/auth/youtube.readonly"

// YouTubeOAuth handles YouTube/Google OAuth 2.0 flow
type YouTubeOAuth struct {
	config *oauth2.Config
	client *http.Client
}

// YouTubeChannelInfo contains channel metadata resolved via the YouTube API
type YouTubeChannelInfo struct {
	ChannelID string
	Title     string
	Handle    string
}

// NewYouTubeOAuth creates a new YouTube OAuth handler
func NewYouTubeOAuth(clientID, clientSecret, redirectURL string) *YouTubeOAuth {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		// `youtube.force-ssl` was dropped in v1.6.0 — backend's
		// sendStreamerYouTubeMessage path is no longer called by any current
		// client (streamer-side YouTube sending happens via the streamer's
		// own browser session in the extension or YT Studio directly).
		// `youtube.readonly` is required by youtube-listener for polling
		// liveChat/messages/stream against the streamer's account.
		// See ADR 0012.
		Scopes: []string{
			YouTubeReadonlyScope,
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	return &YouTubeOAuth{
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
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

// youtubeModerationScopeByAction maps a moderation action to the Google OAuth scope it
// requires. force-ssl was dropped from login (ADR-0012) and is re-added ONLY through the
// opt-in moderation re-consent flow (ADR-0017). YouTube moderation is ban-only in v1.
var youtubeModerationScopeByAction = map[string]string{
	"ban": "https://www.googleapis.com/auth/youtube.force-ssl",
}

// YouTubeModerationScopesForActions returns the deduped, minimal set of YouTube scopes
// the given moderation actions require. Unknown/unsupported actions are ignored.
func YouTubeModerationScopesForActions(actions []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, 1)
	for _, a := range actions {
		if scope, ok := youtubeModerationScopeByAction[a]; ok && !seen[scope] {
			seen[scope] = true
			out = append(out, scope)
		}
	}
	return out
}

// GetAuthURLWithScopes builds a consent URL requesting the base login scopes plus
// `extra` (deduped). prompt=consent is REQUIRED so an already-connected streamer is
// actually re-prompted for the new force-ssl scope (and so Google reissues a refresh
// token); access_type=offline keeps the refresh token. The moderation re-consent passes
// extra = (existing granted scopes ∪ force-ssl), so the token is always a superset.
func (y *YouTubeOAuth) GetAuthURLWithScopes(state string, extra []string) string {
	seen := make(map[string]bool)
	scopes := make([]string, 0, len(y.config.Scopes)+len(extra))
	add := func(list []string) {
		for _, s := range list {
			if s != "" && !seen[s] {
				seen[s] = true
				scopes = append(scopes, s)
			}
		}
	}
	add(y.config.Scopes)
	add(extra)
	return y.config.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
		oauth2.SetAuthURLParam("scope", strings.Join(scopes, " ")),
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

// GetAuthURLWithPKCE generates the OAuth authorization URL with PKCE (audit L4).
// Returns the auth URL and the code verifier that the caller must store for the
// token exchange.
func (y *YouTubeOAuth) GetAuthURLWithPKCE(state string) (string, string) {
	verifier := oauth2.GenerateVerifier()
	authURL := y.config.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "select_account"),
		oauth2.S256ChallengeOption(verifier),
	)
	return authURL, verifier
}

// ExchangeCodeWithVerifier exchanges the authorization code using a PKCE code
// verifier (audit L4).
func (y *YouTubeOAuth) ExchangeCodeWithVerifier(ctx context.Context, code, codeVerifier string) (*oauth2.Token, error) {
	token, err := y.config.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code with PKCE: %w", err)
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
