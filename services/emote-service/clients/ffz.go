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
	"errors"
	"fmt"
	"net/http"
	"strings"
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

// FFZResponse represents the FFZ API response for a room lookup
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

// FetchEmotes fetches emotes from FFZ for a given channel.
// For "global", only FFZ's global set is returned. For any other channel,
// the global set is always merged in — global emotes (e.g. BeanieHipster)
// render in every channel — with the channel's own emotes taking precedence
// on code collisions. A channel without an FFZ room is the common case,
// not a failure: the response is the global set.
func (c *FFZClient) FetchEmotes(ctx context.Context, channel string) ([]models.Emote, error) {
	if strings.TrimSpace(channel) == "" {
		return nil, fmt.Errorf("channel cannot be empty")
	}

	if strings.EqualFold(channel, "global") {
		return c.fetchGlobalEmotes(ctx, channel)
	}
	channelEmotes, chErr := c.fetchRoomEmotes(ctx, channel)
	if chErr != nil && !errors.Is(chErr, ErrNotFound) {
		return nil, chErr
	}

	globalEmotes, gErr := c.fetchGlobalEmotes(ctx, channel)
	if gErr != nil {
		// Global emotes are a bonus, not a requirement — failing to fetch them
		// must not lose channel emotes. But with nothing else to return, the
		// fetch either missed (ErrNotFound) or returned no emotes (a room with
		// only emotes lacking a 1x URL), so propagate the real error instead
		// of caching an empty result.
		if len(channelEmotes) == 0 {
			return nil, gErr
		}
		c.logger.Warn("Failed to fetch FFZ global emotes, returning channel emotes only",
			zap.String("channel", channel),
			zap.Error(gErr))
		return channelEmotes, nil
	}

	if errors.Is(chErr, ErrNotFound) {
		return globalEmotes, nil
	}

	merged := mergeEmoteSets(globalEmotes, channelEmotes)

	c.logger.Debug("Fetched FFZ emotes",
		zap.String("channel", channel),
		zap.Int("channel_emotes", len(channelEmotes)),
		zap.Int("global_emotes", len(globalEmotes)),
		zap.Int("total", len(merged)))

	return merged, nil
}

// fetchRoomEmotes fetches a channel's room emote sets from FFZ.
func (c *FFZClient) fetchRoomEmotes(ctx context.Context, channel string) ([]models.Emote, error) {
	url := fmt.Sprintf("%s/v1/room/%s", c.baseURL, channel)

	c.logger.Debug("Fetching FFZ room emotes",
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
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, rateLimited("ffz", resp)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch emotes: status code %d", resp.StatusCode)
	}

	var apiResp FFZResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

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

			emotes = append(emotes, models.Emote{
				Code:     e.Name,
				URL:      url,
				Provider: "ffz",
				Channel:  channel,
			})
		}
	}

	return emotes, nil
}

// fetchGlobalEmotes fetches FFZ's global emote set. channel is only used to
// populate the Channel field on the parsed emotes.
func (c *FFZClient) fetchGlobalEmotes(ctx context.Context, channel string) ([]models.Emote, error) {
	url := fmt.Sprintf("%s/v1/set/global", c.baseURL)

	c.logger.Debug("Fetching FFZ global emotes",
		zap.String("url", url))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "All-Chat/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch global emotes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, rateLimited("ffz", resp)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch global emotes: status code %d", resp.StatusCode)
	}

	var apiResp struct {
		Sets map[string]struct {
			Emoticons []struct {
				ID   int               `json:"id"`
				Name string            `json:"name"`
				URLs map[string]string `json:"urls"`
			} `json:"emoticons"`
		} `json:"sets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	emotes := make([]models.Emote, 0)
	for _, set := range apiResp.Sets {
		for _, e := range set.Emoticons {
			// Get 1x size URL
			url, ok := e.URLs["1"]
			if !ok {
				continue
			}

			emotes = append(emotes, models.Emote{
				Code:     e.Name,
				URL:      url,
				Provider: "ffz",
				Channel:  channel,
			})
		}
	}

	c.logger.Debug("Fetched FFZ global emotes",
		zap.Int("count", len(emotes)))

	return emotes, nil
}

// Provider returns the provider name
func (c *FFZClient) Provider() string {
	return "ffz"
}
