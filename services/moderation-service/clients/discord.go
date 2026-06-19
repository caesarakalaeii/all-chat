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
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Discord moderation is delete-only and authenticates with a single shared bot token
// (a service credential), not a per-user OAuth grant — so there is no scope or refresh
// logic here. The bot must be a member of the guild with the MANAGE_MESSAGES
// permission, granted when the streamer invites it; without that permission Discord
// returns 403 and the delete is reported as failed (never a false reflect-back).
//
// Auth header format is "Bot <token>" (NOT "Bearer"), mirroring the existing Go caller
// in services/discord-listener/relay/webhook_provisioner.go.
var (
	// ErrDiscordUnauthorized indicates the bot token itself is invalid (HTTP 401). This
	// is a service-credential misconfiguration, not a per-user re-consent situation.
	ErrDiscordUnauthorized = errors.New("discord: bot token unauthorized")
	// ErrDiscordForbidden indicates the bot lacks the MANAGE_MESSAGES permission in the
	// channel/guild (HTTP 403). The remedy is re-inviting the bot with the permission,
	// not an OAuth re-consent — so the dispatcher surfaces it as a platform failure.
	ErrDiscordForbidden = errors.New("discord: forbidden (bot lacks Manage Messages permission or channel access)")
)

// defaultDiscordBaseURL pins the Discord REST API version used across the codebase
// (services/discord-listener/relay/webhook_provisioner.go).
const defaultDiscordBaseURL = "https://discord.com/api/v10"

// discordUserAgent identifies the client to Discord, which requires a User-Agent on
// REST calls. Matches the format used by the auth-service Discord calls.
const discordUserAgent = "AllChat (https://allch.at, 1.0)"

// DiscordClient deletes Discord chat messages via the bot REST API.
type DiscordClient struct {
	httpClient *http.Client
	botToken   string
	baseURL    string
}

// NewDiscordClient builds a client authenticated with the given Discord bot token
// (the same DISCORD_BOT_TOKEN the discord-listener already uses).
func NewDiscordClient(botToken string) *DiscordClient {
	return &DiscordClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		botToken:   botToken,
		baseURL:    defaultDiscordBaseURL,
	}
}

// DeleteMessage removes a single Discord message.
// DELETE /channels/{channel_id}/messages/{message_id} — requires MANAGE_MESSAGES (for
// messages authored by other users). A 404 is treated as success: DELETE is idempotent
// and a missing message means the moderation goal (the message is gone) is already met,
// which also neutralises double-clicks racing the WS echo.
func (d *DiscordClient) DeleteMessage(ctx context.Context, channelID, messageID string) error {
	path := fmt.Sprintf("/channels/%s/messages/%s", channelID, messageID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, d.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("discord: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+d.botToken)
	req.Header.Set("User-Agent", discordUserAgent)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("discord: request failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusNotFound:
		// Already deleted / unknown message — idempotent success.
		return nil
	case resp.StatusCode == http.StatusUnauthorized:
		return ErrDiscordUnauthorized
	case resp.StatusCode == http.StatusForbidden:
		return ErrDiscordForbidden
	default:
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("discord: delete returned %s: %s", strconv.Itoa(resp.StatusCode), string(snippet))
	}
}
