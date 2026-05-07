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
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	kickAPIChannelURLTemplate = "https://kick.com/api/v2/channels/%s"
	kickLookupTimeout         = 5 * time.Second
	kickLookupCacheTTL        = 10 * time.Minute
)

// KickUserLookup resolves a Kick channel slug to its numeric broadcaster user ID.
// 7TV stores Kick connections by numeric user ID, so the slug is not directly usable.
type KickUserLookup interface {
	GetUserID(ctx context.Context, slug string) (string, error)
}

type cachedKickUser struct {
	id        string
	expiresAt time.Time
}

// KickClient resolves Kick channel slugs to broadcaster user IDs via Kick's
// public v2 channels endpoint. No authentication is required.
type KickClient struct {
	httpClient  *http.Client
	urlTemplate string
	logger      *zap.Logger

	cacheMu sync.RWMutex
	cache   map[string]cachedKickUser
}

// NewKickClient constructs a Kick lookup client.
func NewKickClient(logger *zap.Logger) *KickClient {
	return &KickClient{
		httpClient:  &http.Client{Timeout: kickLookupTimeout},
		urlTemplate: kickAPIChannelURLTemplate,
		logger:      logger.With(zap.String("component", "kick-client")),
		cache:       make(map[string]cachedKickUser),
	}
}

// GetUserID resolves a Kick channel slug to its numeric user_id.
func (c *KickClient) GetUserID(ctx context.Context, slug string) (string, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return "", fmt.Errorf("kick slug cannot be empty")
	}

	if id, ok := c.getCached(slug); ok {
		return id, nil
	}

	url := fmt.Sprintf(c.urlTemplate, slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create kick request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "All-Chat/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to query kick channel: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("kick channel %q not found", slug)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("kick channel endpoint returned status %d", resp.StatusCode)
	}

	var body struct {
		UserID json.Number `json:"user_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("failed to decode kick response: %w", err)
	}

	id := strings.TrimSpace(body.UserID.String())
	if id == "" || id == "0" {
		return "", fmt.Errorf("kick channel %q returned empty user_id", slug)
	}

	c.setCached(slug, id)
	return id, nil
}

func (c *KickClient) getCached(slug string) (string, bool) {
	c.cacheMu.RLock()
	defer c.cacheMu.RUnlock()

	if cached, ok := c.cache[slug]; ok && time.Now().Before(cached.expiresAt) {
		return cached.id, true
	}
	return "", false
}

func (c *KickClient) setCached(slug, id string) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	c.cache[slug] = cachedKickUser{
		id:        id,
		expiresAt: time.Now().Add(kickLookupCacheTTL),
	}
}
