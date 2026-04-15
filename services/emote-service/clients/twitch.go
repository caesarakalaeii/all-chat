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
	twitchTokenURL = "https://id.twitch.tv/oauth2/token"
	twitchAPIBase  = "https://api.twitch.tv"
	twitchUsersURL = "https://api.twitch.tv/helix/users"
)

// TwitchUserLookup defines the ability to resolve Twitch usernames to user IDs.
type TwitchUserLookup interface {
	GetUserID(ctx context.Context, login string) (string, error)
}

type cachedTwitchUser struct {
	id        string
	expiresAt time.Time
}

// TwitchClient resolves Twitch usernames to IDs using the Helix API.
type TwitchClient struct {
	clientID     string
	clientSecret string
	httpClient   *http.Client
	logger       *zap.Logger

	tokenMu     sync.Mutex
	accessToken string
	tokenExpiry time.Time

	cacheMu sync.RWMutex
	cache   map[string]cachedTwitchUser
}

// NewTwitchClient creates a new Twitch Helix client.
func NewTwitchClient(clientID, clientSecret string, logger *zap.Logger) *TwitchClient {
	return &TwitchClient{
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		logger: logger.With(zap.String("component", "twitch-client")),
		cache:  make(map[string]cachedTwitchUser),
	}
}

// GetUserID resolves a Twitch login (username) to its numeric user ID.
func (c *TwitchClient) GetUserID(ctx context.Context, login string) (string, error) {
	login = strings.ToLower(strings.TrimSpace(login))
	if login == "" {
		return "", fmt.Errorf("twitch login cannot be empty")
	}

	if id, ok := c.getCached(login); ok {
		return id, nil
	}

	token, err := c.getAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get twitch access token: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s?login=%s", twitchUsersURL, url.QueryEscape(login)), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create twitch request: %w", err)
	}

	req.Header.Set("Client-Id", c.clientID)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to query twitch users: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("twitch users endpoint returned status %d", resp.StatusCode)
	}

	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("failed to decode twitch response: %w", err)
	}

	if len(body.Data) == 0 || body.Data[0].ID == "" {
		return "", fmt.Errorf("twitch user %q not found", login)
	}

	id := body.Data[0].ID
	c.setCached(login, id)
	return id, nil
}

func (c *TwitchClient) getAccessToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if c.accessToken != "" && time.Until(c.tokenExpiry) > 30*time.Second {
		return c.accessToken, nil
	}

	form := url.Values{}
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, twitchTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create twitch token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to request twitch token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("twitch token endpoint returned status %d", resp.StatusCode)
	}

	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("failed to decode twitch token response: %w", err)
	}

	if body.AccessToken == "" {
		return "", fmt.Errorf("received empty access token from twitch")
	}

	c.accessToken = body.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(body.ExpiresIn) * time.Second)
	return c.accessToken, nil
}

func (c *TwitchClient) getCached(login string) (string, bool) {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()

	if cached, ok := c.cache[login]; ok {
		if time.Now().Before(cached.expiresAt) {
			return cached.id, true
		}
	}
	return "", false
}

func (c *TwitchClient) setCached(login, id string) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	c.cache[login] = cachedTwitchUser{
		id:        id,
		expiresAt: time.Now().Add(10 * time.Minute),
	}
}

func (c *TwitchClient) apiGet(ctx context.Context, path string, query url.Values, v interface{}) error {
	token, err := c.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get twitch access token: %w", err)
	}

	endpoint := twitchAPIBase + path
	if len(query) > 0 {
		endpoint = endpoint + "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create twitch request: %w", err)
	}

	req.Header.Set("Client-Id", c.clientID)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call twitch api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("twitch api %s returned status %d", path, resp.StatusCode)
	}

	if v == nil {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("failed to decode twitch response for %s: %w", path, err)
	}

	return nil
}
