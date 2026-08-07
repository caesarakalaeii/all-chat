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
	"net/url"
	"time"

	"go.uber.org/zap"
)

const defaultDiscordAPIBase = "https://discord.com/api/v10"

// Sentinel errors from the Discord REST calls. They are distinguished because the caller's
// response differs: a missing channel is a client mistake, a forbidden one means the bot
// needs re-inviting, and anything else must fail closed as a transient error.
var (
	ErrDiscordChannelNotFound = errors.New("discord: channel not found")
	ErrDiscordForbidden       = errors.New("discord: forbidden")
	ErrDiscordUnauthorized    = errors.New("discord: bot token rejected")
	ErrDiscordNoGuild         = errors.New("discord: channel belongs to no guild")
)

// DiscordClient performs the minimal bot-authenticated reads overlay-manager needs to
// validate that a Discord channel may be attached to an overlay.
//
// It deliberately holds only read capability: overlay-manager must be able to answer
// "which guild does this channel belong to" without gaining any ability to write to Discord.
type DiscordClient struct {
	botToken   string
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewDiscordClient creates a Discord REST client authenticated as the shared bot.
func NewDiscordClient(botToken string, logger *zap.Logger) *DiscordClient {
	return &DiscordClient{
		botToken:   botToken,
		baseURL:    defaultDiscordAPIBase,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
	}
}

// WithBaseURL overrides the API base (tests).
func (d *DiscordClient) WithBaseURL(base string) *DiscordClient {
	d.baseURL = base
	return d
}

// GuildIDForChannel resolves the guild a channel belongs to, as Discord itself reports it.
//
// This is the authoritative binding: it must never be replaced by a client-supplied
// guild_id, because the whole point is to stop a caller from pairing a channel they do not
// control with a guild they do.
func (d *DiscordClient) GuildIDForChannel(ctx context.Context, channelID string) (string, error) {
	endpoint := fmt.Sprintf("%s/channels/%s", d.baseURL, url.PathEscape(channelID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("discord: build channel request: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+d.botToken)
	req.Header.Set("User-Agent", "AllChat (https://allch.at, 1.0)")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("discord: channel request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// fall through to decode
	case http.StatusNotFound:
		return "", ErrDiscordChannelNotFound
	case http.StatusForbidden:
		return "", ErrDiscordForbidden
	case http.StatusUnauthorized:
		return "", ErrDiscordUnauthorized
	default:
		return "", fmt.Errorf("discord: unexpected status %d resolving channel", resp.StatusCode)
	}

	var body struct {
		ID      string `json:"id"`
		GuildID string `json:"guild_id"`
		Type    int    `json:"type"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("discord: decode channel: %w", err)
	}

	// DMs and group DMs carry no guild_id. Returning "" here would hand an empty guild id to
	// an ownership check that could match nothing (or, worse, a row with an empty guild_id).
	if body.GuildID == "" {
		return "", ErrDiscordNoGuild
	}

	return body.GuildID, nil
}
