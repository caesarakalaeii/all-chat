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
	"time"

	"github.com/caesar/all-chat/services/emote-service/models"
	"go.uber.org/zap"
)

const (
	ffzAPIURL     = "https://api.frankerfacez.com"
	ffzAPITimeout = 5 * time.Second
)

// FFZClient implements EmoteClient for FrankerFaceZ API
type FFZClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

// FFZResponse represents the FFZ API response
type FFZResponse struct {
	Room struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	} `json:"room"`
	Sets map[string]struct {
		Emoticons []struct {
			ID   int               `json:"id"`
			Name string            `json:"name"`
			URLs map[string]string `json:"urls"`
		} `json:"emoticons"`
	} `json:"sets"`
}

// NewFFZClient creates a new FFZ API client
func NewFFZClient(logger *zap.Logger) *FFZClient {
	return &FFZClient{
		baseURL: ffzAPIURL,
		httpClient: &http.Client{
			Timeout: ffzAPITimeout,
		},
		logger: logger,
	}
}

// FetchEmotes fetches emotes from FFZ for a given channel
func (c *FFZClient) FetchEmotes(ctx context.Context, channel string) ([]models.Emote, error) {
	url := fmt.Sprintf("%s/v1/room/%s", c.baseURL, channel)

	c.logger.Debug("Fetching FFZ emotes",
		zap.String("channel", channel),
		zap.String("url", url))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "All-Chat/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch emotes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// Channel has no FFZ room — the normal case for most channels, not a failure.
		return nil, fmt.Errorf("ffz: no emotes for channel %q: %w", channel, ErrNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch emotes: status code %d", resp.StatusCode)
	}

	var apiResp FFZResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert FFZ emotes to our internal format
	// FFZ has multiple "sets" of emotes per channel
	emotes := make([]models.Emote, 0)

	for _, set := range apiResp.Sets {
		for _, e := range set.Emoticons {
			// Get 1x size URL
			url, ok := e.URLs["1"]
			if !ok {
				// Skip emotes without 1x URL
				continue
			}

			emote := models.Emote{
				Code:     e.Name,
				URL:      url,
				Provider: "ffz",
				Channel:  channel,
			}
			emotes = append(emotes, emote)
		}
	}

	c.logger.Debug("Fetched FFZ emotes",
		zap.String("channel", channel),
		zap.Int("count", len(emotes)))

	return emotes, nil
}

// Provider returns the provider name
func (c *FFZClient) Provider() string {
	return "ffz"
}
