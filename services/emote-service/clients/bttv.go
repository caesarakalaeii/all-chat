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
	bttvAPIURL     = "https://api.betterttv.net"
	bttvAPITimeout = 5 * time.Second
)

// BTTVClient implements EmoteClient for BTTV API
type BTTVClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

// BTTVResponse represents the BTTV API response
type BTTVResponse struct {
	ID            string `json:"id"`
	ChannelEmotes []struct {
		ID        string `json:"id"`
		Code      string `json:"code"`
		ImageType string `json:"imageType"`
	} `json:"channelEmotes"`
	SharedEmotes []struct {
		ID        string `json:"id"`
		Code      string `json:"code"`
		ImageType string `json:"imageType"`
	} `json:"sharedEmotes"`
}

// NewBTTVClient creates a new BTTV API client
func NewBTTVClient(logger *zap.Logger) *BTTVClient {
	return &BTTVClient{
		baseURL: bttvAPIURL,
		httpClient: &http.Client{
			Timeout: bttvAPITimeout,
		},
		logger: logger,
	}
}

// FetchEmotes fetches emotes from BTTV for a given channel
func (c *BTTVClient) FetchEmotes(ctx context.Context, channel string) ([]models.Emote, error) {
	url := fmt.Sprintf("%s/3/cached/users/twitch/%s", c.baseURL, channel)

	c.logger.Debug("Fetching BTTV emotes",
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
		// Channel has no BTTV emotes — the normal case for most channels, not a failure.
		return nil, fmt.Errorf("bttv: no emotes for channel %q: %w", channel, ErrNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch emotes: status code %d", resp.StatusCode)
	}

	var apiResp BTTVResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert BTTV emotes to our internal format
	// Include both channel emotes and shared emotes
	totalCount := len(apiResp.ChannelEmotes) + len(apiResp.SharedEmotes)
	emotes := make([]models.Emote, 0, totalCount)

	// Add channel emotes
	for _, e := range apiResp.ChannelEmotes {
		url := fmt.Sprintf("https://cdn.betterttv.net/emote/%s/1x", e.ID)
		emote := models.Emote{
			Code:     e.Code,
			URL:      url,
			Provider: "bttv",
			Channel:  channel,
		}
		emotes = append(emotes, emote)
	}

	// Add shared emotes
	for _, e := range apiResp.SharedEmotes {
		url := fmt.Sprintf("https://cdn.betterttv.net/emote/%s/1x", e.ID)
		emote := models.Emote{
			Code:     e.Code,
			URL:      url,
			Provider: "bttv",
			Channel:  channel,
		}
		emotes = append(emotes, emote)
	}

	c.logger.Debug("Fetched BTTV emotes",
		zap.String("channel", channel),
		zap.Int("count", len(emotes)))

	return emotes, nil
}

// Provider returns the provider name
func (c *BTTVClient) Provider() string {
	return "bttv"
}
