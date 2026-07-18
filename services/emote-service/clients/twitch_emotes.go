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
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/caesar/all-chat/services/emote-service/models"
	"go.uber.org/zap"
)

const twitchTemplateFallback = "https://static-cdn.jtvnw.net/emoticons/v2/{id}/{format}/{theme_mode}/{scale}"

type twitchChatEmoteResponse struct {
	Template string            `json:"template"`
	Data     []twitchChatEmote `json:"data"`
}

type twitchChatEmote struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Format    []string `json:"format"`
	ThemeMode []string `json:"theme_mode"`
	Scale     []string `json:"scale"`
}

type TwitchEmoteClient struct {
	helix  *TwitchClient
	logger *zap.Logger
}

func NewTwitchEmoteClient(helix *TwitchClient, logger *zap.Logger) *TwitchEmoteClient {
	return &TwitchEmoteClient{
		helix:  helix,
		logger: logger.With(zap.String("provider", "twitch")),
	}
}

func (c *TwitchEmoteClient) Provider() string {
	return "twitch"
}

func (c *TwitchEmoteClient) FetchEmotes(ctx context.Context, channel string) ([]models.Emote, error) {
	channel = strings.TrimSpace(channel)
	if channel == "" {
		channel = "global"
	}

	global, err := c.fetchGlobal(ctx)
	if err != nil {
		return nil, err
	}

	if strings.EqualFold(channel, "global") {
		return global, nil
	}

	broadcasterID := channel
	if !isNumeric(channel) {
		resolved, err := c.helix.GetUserID(ctx, channel)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve twitch user: %w", err)
		}
		broadcasterID = resolved
	}

	channelEmotes, err := c.fetchChannel(ctx, broadcasterID, channel)
	if err != nil {
		return nil, err
	}

	return mergeEmoteSets(global, channelEmotes), nil
}

func (c *TwitchEmoteClient) fetchGlobal(ctx context.Context) ([]models.Emote, error) {
	resp, err := c.fetchFromHelix(ctx, "/helix/chat/emotes/global", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch global emotes: %w", err)
	}
	emotes := convertTwitchEmotes(resp, "global")
	c.logger.Debug("Fetched Twitch global emotes", zap.Int("count", len(emotes)))
	return emotes, nil
}

func (c *TwitchEmoteClient) fetchChannel(ctx context.Context, broadcasterID, channelName string) ([]models.Emote, error) {
	resp, err := c.fetchFromHelix(ctx, "/helix/chat/emotes", url.Values{
		"broadcaster_id": []string{broadcasterID},
		"included_sets":  []string{"0"}, // ensures global set is returned for completeness
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch channel emotes: %w", err)
	}
	return convertTwitchEmotes(resp, channelName), nil
}

func (c *TwitchEmoteClient) fetchFromHelix(ctx context.Context, path string, query url.Values) (*twitchChatEmoteResponse, error) {
	var resp twitchChatEmoteResponse
	if err := c.helix.apiGet(ctx, path, query, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func convertTwitchEmotes(resp *twitchChatEmoteResponse, channel string) []models.Emote {
	if resp == nil {
		return nil
	}

	template := resp.Template
	if template == "" {
		template = twitchTemplateFallback
	}

	emotes := make([]models.Emote, 0, len(resp.Data))
	for _, emote := range resp.Data {
		if emote.ID == "" || emote.Name == "" {
			continue
		}
		url := buildTwitchEmoteURL(template, emote)
		if url == "" {
			continue
		}
		emotes = append(emotes, models.Emote{
			Code:     emote.Name,
			URL:      url,
			Provider: "twitch",
			Channel:  channel,
		})
	}
	return emotes
}

func buildTwitchEmoteURL(template string, emote twitchChatEmote) string {
	format := choosePreferred(emote.Format, []string{"default", "animated", "static"})
	if format == "" {
		format = "default"
	}
	theme := choosePreferred(emote.ThemeMode, []string{"dark", "light"})
	if theme == "" {
		theme = "dark"
	}
	scale := chooseLargestScale(emote.Scale)
	if scale == "" {
		scale = "2.0"
	}

	replacer := strings.NewReplacer(
		"{id}", emote.ID,
		"{{id}}", emote.ID,
		"{format}", strings.ToLower(format),
		"{{format}}", strings.ToLower(format),
		"{theme_mode}", strings.ToLower(theme),
		"{{theme_mode}}", strings.ToLower(theme),
		"{scale}", scale,
		"{{scale}}", scale,
	)
	return replacer.Replace(template)
}

func choosePreferred(options, preferred []string) string {
	if len(options) == 0 {
		return ""
	}
	for _, want := range preferred {
		for _, opt := range options {
			if strings.EqualFold(opt, want) {
				return opt
			}
		}
	}
	return options[0]
}

func chooseLargestScale(scales []string) string {
	if len(scales) == 0 {
		return ""
	}
	best := scales[0]
	bestValue := parseScale(best)
	for _, scale := range scales[1:] {
		if value := parseScale(scale); value > bestValue {
			best = scale
			bestValue = value
		}
	}
	return best
}

func parseScale(value string) float64 {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func mergeEmoteSets(global, channel []models.Emote) []models.Emote {
	index := make(map[string]models.Emote)
	for _, emote := range global {
		index[strings.ToLower(emote.Code)] = emote
	}
	for _, emote := range channel {
		index[strings.ToLower(emote.Code)] = emote
	}

	merged := make([]models.Emote, 0, len(index))
	for _, emote := range index {
		merged = append(merged, emote)
	}

	sort.Slice(merged, func(i, j int) bool {
		return strings.ToLower(merged[i].Code) < strings.ToLower(merged[j].Code)
	})

	return merged
}
