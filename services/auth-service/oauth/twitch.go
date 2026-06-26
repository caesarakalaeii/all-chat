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
	"golang.org/x/oauth2/twitch"
)

// TwitchChatScopes are requested only in the add-source flow, where a streamer
// authorizes their OWN channel for EventSub channel.chat.message reading. Because
// the broadcaster is also the chatter, one consent grants everything the webhook
// subscription needs: user:read:chat + user:bot (chatter side) and channel:bot
// (broadcaster side). See services/twitch-eventsub-listener for the consumer.
var TwitchChatScopes = []string{"user:read:chat", "user:bot", "channel:bot"}

// TwitchSendScope authorizes the Helix "Send Chat Message" API
// (POST /helix/chat/messages) for a user token where the broadcaster is also the
// sender. Requested ONLY through the opt-in advanced-controls re-consent (alongside
// the moderation scopes), never bundled into login/add-source (least privilege).
const TwitchSendScope = "user:write:chat"

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

// GetAuthURLWithChatScopes builds the consent URL with the base login scopes PLUS
// the chat scopes (TwitchChatScopes). force_verify=true is REQUIRED: without it
// Twitch may silently reissue a token for a prior, narrower grant, so an
// already-connected streamer would never actually be prompted to grant the new
// chat scopes and would stay stuck on the IRC listener.
func (t *TwitchOAuth) GetAuthURLWithChatScopes(state string) string {
	scopes := append(append([]string{}, t.config.Scopes...), TwitchChatScopes...)
	return t.config.AuthCodeURL(state,
		oauth2.SetAuthURLParam("scope", strings.Join(scopes, " ")),
		oauth2.SetAuthURLParam("force_verify", "true"),
	)
}

// twitchModerationScopeByAction maps a moderation action to the single Twitch OAuth
// scope it requires. These scopes are requested ONLY through the dedicated opt-in
// moderation re-consent flow and are never bundled into login or add-source
// (ADR-0017, least privilege).
var twitchModerationScopeByAction = map[string]string{
	"delete":  "moderator:manage:chat_messages",
	"timeout": "moderator:manage:banned_users",
	"ban":     "moderator:manage:banned_users",
	"unban":   "moderator:manage:banned_users",
}

// ModerationScopesForActions returns the deduped, minimal set of Twitch scopes the
// given moderation actions require. Unknown actions are ignored, so an empty/garbage
// query yields no scopes (the caller then rejects the request).
func ModerationScopesForActions(actions []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, 2)
	for _, a := range actions {
		if scope, ok := twitchModerationScopeByAction[a]; ok && !seen[scope] {
			seen[scope] = true
			out = append(out, scope)
		}
	}
	return out
}

// GetAuthURLWithScopes builds a consent URL requesting the base login scopes plus
// `extra` (deduped). force_verify=true is REQUIRED so an already-connected streamer
// is actually re-prompted — without it Twitch may silently reissue the prior, narrower
// grant, so the new moderation scopes would never be requested. The moderation
// re-consent flow passes extra = (existing granted scopes ∪ minimal action scopes), so
// the resulting token is always a SUPERSET of the stored grant and never trips the
// scope-downgrade guard.
func (t *TwitchOAuth) GetAuthURLWithScopes(state string, extra []string) string {
	seen := make(map[string]bool)
	scopes := make([]string, 0, len(t.config.Scopes)+len(extra))
	add := func(list []string) {
		for _, s := range list {
			if s != "" && !seen[s] {
				seen[s] = true
				scopes = append(scopes, s)
			}
		}
	}
	add(t.config.Scopes)
	add(extra)
	return t.config.AuthCodeURL(state,
		oauth2.SetAuthURLParam("scope", strings.Join(scopes, " ")),
		oauth2.SetAuthURLParam("force_verify", "true"),
	)
}

// ExtractGrantedScopes pulls the granted scope list out of a Twitch token exchange
// or refresh response. Twitch returns "scope" as a JSON array, which oauth2 surfaces
// via Extra("scope") as []interface{}. Returns nil when absent. The string case is
// defensive (some providers return a space-delimited scope string).
func ExtractGrantedScopes(token *oauth2.Token) []string {
	if token == nil {
		return nil
	}
	switch v := token.Extra("scope").(type) {
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, s := range v {
			if str, ok := s.(string); ok && str != "" {
				out = append(out, str)
			}
		}
		return out
	case []string:
		return v
	case string:
		if v == "" {
			return nil
		}
		return strings.Fields(v)
	default:
		return nil
	}
}

// ExchangeCode exchanges authorization code for tokens
func (t *TwitchOAuth) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := t.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	return token, nil
}

// GetAuthURLWithPKCE generates the OAuth authorization URL with PKCE (audit L4).
// Returns the auth URL and the code verifier that the caller must store for the
// token exchange.
func (t *TwitchOAuth) GetAuthURLWithPKCE(state string) (string, string) {
	verifier := oauth2.GenerateVerifier()
	authURL := t.config.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))
	return authURL, verifier
}

// ExchangeCodeWithVerifier exchanges the authorization code using a PKCE code
// verifier (audit L4).
func (t *TwitchOAuth) ExchangeCodeWithVerifier(ctx context.Context, code, codeVerifier string) (*oauth2.Token, error) {
	token, err := t.config.Exchange(ctx, code, oauth2.VerifierOption(codeVerifier))
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code with PKCE: %w", err)
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
