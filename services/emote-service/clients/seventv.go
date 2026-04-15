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
	"strconv"
	"strings"
	"time"

	"github.com/caesar/all-chat/services/emote-service/models"
	"go.uber.org/zap"
)

const (
	seventvAPIURL     = "https://7tv.io"
	seventvAPITimeout = 5 * time.Second
)

// SevenTVClient implements EmoteClient for 7TV API
type SevenTVClient struct {
	baseURL      string
	httpClient   *http.Client
	logger       *zap.Logger
	twitchClient TwitchUserLookup
}

type sevenTVUserResponse struct {
	EmoteSet sevenTVEmoteSet `json:"emote_set"`
}

type sevenTVEmoteSet struct {
	ID     string               `json:"id"`
	Name   string               `json:"name"`
	Emotes []sevenTVActiveEmote `json:"emotes"`
}

type sevenTVActiveEmote struct {
	ID   string            `json:"id"`
	Name string            `json:"name"`
	Data *sevenTVEmoteData `json:"data"`
}

type sevenTVEmoteData struct {
	Name  string      `json:"name"`
	Host  sevenTVHost `json:"host"`
	Flags int         `json:"flags"`
}

type sevenTVHost struct {
	URL   string            `json:"url"`
	Files []sevenTVHostFile `json:"files"`
}

type sevenTVHostFile struct {
	Name       string `json:"name"`
	StaticName string `json:"static_name"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

// NewSevenTVClient creates a new 7TV API client
func NewSevenTVClient(logger *zap.Logger, twitchClient TwitchUserLookup) *SevenTVClient {
	return &SevenTVClient{
		baseURL: seventvAPIURL,
		httpClient: &http.Client{
			Timeout: seventvAPITimeout,
		},
		logger:       logger,
		twitchClient: twitchClient,
	}
}

// FetchEmotes fetches emotes from 7TV for a given channel
// For non-global channels, this fetches both channel-specific and global emotes
func (c *SevenTVClient) FetchEmotes(ctx context.Context, channel string) ([]models.Emote, error) {
	if strings.TrimSpace(channel) == "" {
		return nil, fmt.Errorf("channel cannot be empty")
	}

	// If requesting global emotes only, fetch just those
	if strings.EqualFold(channel, "global") {
		return c.fetchEmoteSet(ctx, "global", channel)
	}

	// For channels, fetch both channel-specific and global emotes
	channelEmotes, err := c.fetchChannelEmotes(ctx, channel)
	if err != nil {
		return nil, err
	}

	// Fetch global emotes
	globalEmotes, err := c.fetchEmoteSet(ctx, "global", channel)
	if err != nil {
		// Log warning but don't fail - channel emotes are still valid
		c.logger.Warn("Failed to fetch global emotes, returning channel emotes only",
			zap.String("channel", channel),
			zap.Error(err))
		return channelEmotes, nil
	}

	// Merge emotes, avoiding duplicates (channel emotes take precedence)
	emoteMap := make(map[string]models.Emote)
	
	// Add global emotes first
	for _, emote := range globalEmotes {
		emoteMap[emote.Code] = emote
	}
	
	// Add channel emotes (overwrites globals if same code exists)
	for _, emote := range channelEmotes {
		emoteMap[emote.Code] = emote
	}

	// Convert map back to slice
	allEmotes := make([]models.Emote, 0, len(emoteMap))
	for _, emote := range emoteMap {
		allEmotes = append(allEmotes, emote)
	}

	c.logger.Debug("Fetched 7TV emotes",
		zap.String("channel", channel),
		zap.Int("channel_emotes", len(channelEmotes)),
		zap.Int("global_emotes", len(globalEmotes)),
		zap.Int("total", len(allEmotes)))

	return allEmotes, nil
}

// fetchChannelEmotes fetches channel-specific emotes
func (c *SevenTVClient) fetchChannelEmotes(ctx context.Context, channel string) ([]models.Emote, error) {
	resolved := channel
	if !isNumeric(channel) {
		if c.twitchClient == nil {
			return nil, fmt.Errorf("twitch client is not configured")
		}
		twitchID, err := c.twitchClient.GetUserID(ctx, channel)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve twitch user: %w", err)
		}
		resolved = twitchID
	}

	urlPath := fmt.Sprintf("%s/v3/users/twitch/%s", c.baseURL, resolved)

	c.logger.Debug("Fetching channel 7TV emotes",
		zap.String("channel", channel),
		zap.String("url", urlPath))

	req, err := http.NewRequestWithContext(ctx, "GET", urlPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "All-Chat/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch emotes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch emotes: status code %d", resp.StatusCode)
	}

	var apiResp sevenTVUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return c.parseEmoteSet(apiResp.EmoteSet, channel), nil
}

// FetchUserEmotes fetches a user's personal emote set from 7TV
func (c *SevenTVClient) FetchUserEmotes(ctx context.Context, platform, userID string) ([]models.Emote, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, fmt.Errorf("user ID cannot be empty")
	}
	if strings.TrimSpace(platform) == "" {
		return nil, fmt.Errorf("platform cannot be empty")
	}

	urlPath := fmt.Sprintf("%s/v3/users/%s/%s", c.baseURL, platform, userID)

	c.logger.Debug("Fetching user 7TV emotes",
		zap.String("platform", platform),
		zap.String("user_id", userID),
		zap.String("url", urlPath))

	req, err := http.NewRequestWithContext(ctx, "GET", urlPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "All-Chat/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user emotes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// User doesn't have 7TV account or emote set - return empty list
		c.logger.Debug("User not found on 7TV or has no emote set",
			zap.String("platform", platform),
			zap.String("user_id", userID))
		return []models.Emote{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch user emotes: status code %d", resp.StatusCode)
	}

	var apiResp sevenTVUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Use a special marker for user emotes (will be used for cache key)
	return c.parseEmoteSet(apiResp.EmoteSet, fmt.Sprintf("user:%s:%s", platform, userID)), nil
}

// fetchEmotesForPlatform fetches 7TV emotes with platform awareness.
// For Twitch, it fetches both channel-specific and global emotes.
// For non-Twitch platforms, if a twitchChannel hint is provided (from a sibling Twitch
// source on the same overlay), channel emotes are fetched using that Twitch channel.
// Otherwise only global emotes are returned.
func (c *SevenTVClient) fetchEmotesForPlatform(ctx context.Context, channel, platform, twitchChannel string) ([]models.Emote, error) {
	if platform != "twitch" {
		if twitchChannel != "" {
			// Use the sibling Twitch channel for 7TV channel emote lookup
			return c.FetchEmotes(ctx, twitchChannel)
		}
		// Non-Twitch platform without Twitch hint: only global 7TV emotes
		return c.fetchEmoteSet(ctx, "global", channel)
	}
	// Twitch: fetch both channel-specific and global emotes
	return c.FetchEmotes(ctx, channel)
}

// FetchCombinedEmotes fetches channel + user emotes and merges them.
// User emotes take precedence over channel/global emotes with the same code.
// For non-Twitch platforms, if twitchChannel is provided (from a sibling Twitch source
// on the same overlay), 7TV channel emotes are fetched using that Twitch channel.
// Otherwise only global emotes are returned for non-Twitch platforms.
func (c *SevenTVClient) FetchCombinedEmotes(ctx context.Context, channel, platform, userID, twitchChannel string) ([]models.Emote, error) {
	if strings.TrimSpace(channel) == "" {
		return nil, fmt.Errorf("channel cannot be empty")
	}

	// Fetch channel emotes (includes global + channel).
	// For non-Twitch platforms, use the twitchChannel hint if available to look up
	// 7TV channel emotes via the sibling Twitch source. Otherwise only global emotes.
	channelEmotes, err := c.fetchEmotesForPlatform(ctx, channel, platform, twitchChannel)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch channel emotes: %w", err)
	}

	// If no user ID provided, just return channel emotes
	if strings.TrimSpace(userID) == "" {
		return channelEmotes, nil
	}

	// Fetch user emotes
	userEmotes, err := c.FetchUserEmotes(ctx, platform, userID)
	if err != nil {
		// Log warning but don't fail - channel emotes are still valid
		c.logger.Warn("Failed to fetch user emotes, returning channel emotes only",
			zap.String("platform", platform),
			zap.String("user_id", userID),
			zap.Error(err))
		return channelEmotes, nil
	}

	// Merge emotes with user emotes taking precedence
	emoteMap := make(map[string]models.Emote)

	// Add channel emotes first (global + channel)
	for _, emote := range channelEmotes {
		emoteMap[emote.Code] = emote
	}

	// Add user emotes (overwrites channel/global if same code exists)
	for _, emote := range userEmotes {
		emoteMap[emote.Code] = emote
	}

	// Convert map back to slice
	allEmotes := make([]models.Emote, 0, len(emoteMap))
	for _, emote := range emoteMap {
		allEmotes = append(allEmotes, emote)
	}

	c.logger.Debug("Fetched combined 7TV emotes",
		zap.String("channel", channel),
		zap.String("user_id", userID),
		zap.Int("channel_emotes", len(channelEmotes)),
		zap.Int("user_emotes", len(userEmotes)),
		zap.Int("total", len(allEmotes)))

	return allEmotes, nil
}

// fetchEmoteSet fetches a specific emote set (e.g., global)
func (c *SevenTVClient) fetchEmoteSet(ctx context.Context, setType, channel string) ([]models.Emote, error) {
	var urlPath string
	if setType == "global" {
		urlPath = fmt.Sprintf("%s/v3/emote-sets/global", c.baseURL)
	} else {
		return nil, fmt.Errorf("unsupported emote set type: %s", setType)
	}

	c.logger.Debug("Fetching 7TV emote set",
		zap.String("type", setType),
		zap.String("url", urlPath))

	req, err := http.NewRequestWithContext(ctx, "GET", urlPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "All-Chat/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch emote set: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch emote set: status code %d", resp.StatusCode)
	}

	var emoteSet sevenTVEmoteSet
	if err := json.NewDecoder(resp.Body).Decode(&emoteSet); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return c.parseEmoteSet(emoteSet, channel), nil
}

// parseEmoteSet converts a 7TV emote set response to our emote model
func (c *SevenTVClient) parseEmoteSet(emoteSet sevenTVEmoteSet, channel string) []models.Emote {
	emotes := make([]models.Emote, 0, len(emoteSet.Emotes))
	for _, e := range emoteSet.Emotes {
		if e.Data == nil {
			continue
		}
		url := buildSevenTVURL(e.Data.Host)
		if url == "" {
			continue
		}
		emotes = append(emotes, models.Emote{
			Code:     e.Name,
			URL:      url,
			Provider: "7tv",
			Channel:  channel,
		})
	}
	return emotes
}

// Provider returns the provider name
func (c *SevenTVClient) Provider() string {
	return "7tv"
}

func buildSevenTVURL(host sevenTVHost) string {
	if host.URL == "" || len(host.Files) == 0 {
		return ""
	}

	base := host.URL
	if !strings.HasPrefix(base, "http") {
		base = "https:" + base
	}

	best := host.Files[0]
	for _, file := range host.Files[1:] {
		if file.Width > best.Width {
			best = file
		}
	}

	return fmt.Sprintf("%s/%s", strings.TrimRight(base, "/"), strings.TrimLeft(best.Name, "/"))
}

func isNumeric(value string) bool {
	if value == "" {
		return false
	}
	_, err := strconv.ParseUint(value, 10, 64)
	return err == nil
}
