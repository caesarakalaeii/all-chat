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
	bttvAPIURL     = "https://api.betterttv.net"
	bttvAPITimeout = 5 * time.Second
)

// BTTVClient implements EmoteClient for BTTV API
type BTTVClient struct {
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

// bttvEmote is the shape shared by BTTV's channel, shared, and global emote lists.
type bttvEmote struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	ImageType string `json:"imageType"`
}

// BTTVResponse represents the BTTV API response for a channel lookup
type BTTVResponse struct {
	ID            string      `json:"id"`
	ChannelEmotes []bttvEmote `json:"channelEmotes"`
	SharedEmotes  []bttvEmote `json:"sharedEmotes"`
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

// FetchEmotes fetches emotes from BTTV for a given channel.
// For "global", only BTTV's global set is returned. For any other channel,
// the global set is always merged in — global emotes (e.g. :tf:, AngelThump)
// render in every channel — with the channel's own emotes taking precedence
// on code collisions. A channel that doesn't exist on BTTV is the common case,
// not a failure: the response is the global set.
func (c *BTTVClient) FetchEmotes(ctx context.Context, channel string) ([]models.Emote, error) {
	if strings.TrimSpace(channel) == "" {
		return nil, fmt.Errorf("channel cannot be empty")
	}

	if strings.EqualFold(channel, "global") {
		return c.fetchGlobalEmotes(ctx, channel)
	}

	channelEmotes, chErr := c.fetchChannelEmotes(ctx, channel)
	if chErr != nil && !errors.Is(chErr, ErrNotFound) {
		return nil, chErr
	}

	globalEmotes, gErr := c.fetchGlobalEmotes(ctx, channel)

	return mergeChannelWithGlobals(c.logger, "bttv", channel,
		channelEmotes, chErr, globalEmotes, gErr)
}

// fetchChannelEmotes fetches a channel's own (channel + shared) BTTV emotes.
func (c *BTTVClient) fetchChannelEmotes(ctx context.Context, channel string) ([]models.Emote, error) {
	url := fmt.Sprintf("%s/3/cached/users/twitch/%s", c.baseURL, channel)

	c.logger.Debug("Fetching BTTV channel emotes",
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
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, rateLimited("bttv", resp)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch emotes: status code %d", resp.StatusCode)
	}

	var apiResp BTTVResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Include both channel emotes and shared emotes
	totalCount := len(apiResp.ChannelEmotes) + len(apiResp.SharedEmotes)
	emotes := make([]models.Emote, 0, totalCount)

	for _, list := range [][]bttvEmote{apiResp.ChannelEmotes, apiResp.SharedEmotes} {
		for _, e := range list {
			emotes = append(emotes, models.Emote{
				Code:     e.Code,
				URL:      fmt.Sprintf("https://cdn.betterttv.net/emote/%s/1x", e.ID),
				Provider: "bttv",
				Channel:  channel,
			})
		}
	}

	return emotes, nil
}

// fetchGlobalEmotes fetches BTTV's global emote set. channel is only used to
// populate the Channel field on the parsed emotes.
func (c *BTTVClient) fetchGlobalEmotes(ctx context.Context, channel string) ([]models.Emote, error) {
	url := fmt.Sprintf("%s/3/cached/emotes/global", c.baseURL)

	c.logger.Debug("Fetching BTTV global emotes",
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
		return nil, rateLimited("bttv", resp)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch global emotes: status code %d", resp.StatusCode)
	}

	var apiEmotes []bttvEmote
	if err := json.NewDecoder(resp.Body).Decode(&apiEmotes); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	emotes := make([]models.Emote, 0, len(apiEmotes))
	for _, e := range apiEmotes {
		emotes = append(emotes, models.Emote{
			Code:     e.Code,
			URL:      fmt.Sprintf("https://cdn.betterttv.net/emote/%s/1x", e.ID),
			Provider: "bttv",
			Channel:  channel,
		})
	}

	c.logger.Debug("Fetched BTTV global emotes",
		zap.Int("count", len(emotes)))

	return emotes, nil
}

// Provider returns the provider name
func (c *BTTVClient) Provider() string {
	return "bttv"
}
