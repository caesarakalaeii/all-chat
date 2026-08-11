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

package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

const (
	discordAuthBase  = "https://discord.com/oauth2/authorize"
	discordTokenURL  = "https://discord.com/api/v10/oauth2/token"
	discordAPIBase   = "https://discord.com/api/v10"
	discordUserAgent = "AllChat (https://allch.at, 1.0)"

	// Permission bit values (https://discord.com/developers/docs/topics/permissions)
	PermViewChannel        uint64 = 1024  // 0x400
	PermSendMessages       uint64 = 2048  // 0x800
	PermReadMessageHistory uint64 = 65536 // 0x10000
	RequiredBotPermissions uint64 = PermViewChannel | PermSendMessages | PermReadMessageHistory // 68608

	// Moderation permission bits, requested ONLY through the opt-in moderation re-invite
	// (ADR-0017) — never bundled into the base listener invite. MANAGE_MESSAGES → delete,
	// MODERATE_MEMBERS → timeout, BAN_MEMBERS → ban/unban.
	PermManageMessages  uint64 = 1 << 13 // 8192
	PermBanMembers      uint64 = 1 << 2  // 4
	PermModerateMembers uint64 = 1 << 40
	// ModerationBotPermissions is the elevated permission set the moderation re-invite
	// URL requests: the base listener permissions plus the moderation permissions.
	ModerationBotPermissions uint64 = RequiredBotPermissions | PermManageMessages | PermBanMembers | PermModerateMembers
)

// DiscordOAuth implements OAuthProvider for Discord bot authorization.
// Note: Discord bot auth is NOT a standard user OAuth flow. The invite URL uses scope=bot and
// the callback returns guild_id instead of user identity. GetUserInfo and RefreshToken are stubs.
type DiscordOAuth struct {
	clientID     string
	clientSecret string
	redirectURL  string
	botToken     string // DISCORD_BOT_TOKEN — used for REST API calls
	client       *http.Client
	// apiBase is the Discord REST base, overridable so the REST paths can be asserted
	// against a stub. Defaults to discordAPIBase.
	apiBase string

	// botID caches the bot's own snowflake (see botUserID). A mutex rather than a
	// sync.Once so a transient failure is retried instead of sticking for the pod's life.
	botIDMu sync.Mutex
	botID   string
}

// NewDiscordOAuth creates a new DiscordOAuth provider.
// botToken is the static DISCORD_BOT_TOKEN secret (not the OAuth access token).
func NewDiscordOAuth(clientID, clientSecret, redirectURL string) *DiscordOAuth {
	return &DiscordOAuth{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		client:       &http.Client{Timeout: 10 * time.Second},
		apiBase:      discordAPIBase,
	}
}

// WithBotToken sets the bot token used for REST API permission checks.
func (d *DiscordOAuth) WithBotToken(botToken string) *DiscordOAuth {
	d.botToken = botToken
	return d
}

// GetAuthURL returns the base Discord bot invite URL. This shows a guild picker (not a
// user login page) because scope=bot signals Discord to use the bot authorization flow.
// The base permissions are VIEW_CHANNEL | SEND_MESSAGES | READ_MESSAGE_HISTORY (68608).
func (d *DiscordOAuth) GetAuthURL(state string) string {
	return d.inviteURL(state, RequiredBotPermissions)
}

// GetModerationAuthURL returns the opt-in moderation RE-INVITE URL — the same bot invite
// with the elevated moderation permissions (ADR-0017). Re-authorizing on an existing
// guild upgrades the bot's permissions in place; the streamer then sees the moderation
// controls once the capability endpoint reports the new permissions.
func (d *DiscordOAuth) GetModerationAuthURL(state string) string {
	return d.inviteURL(state, ModerationBotPermissions)
}

// inviteURL builds a bot invite URL requesting the given permission bitfield.
func (d *DiscordOAuth) inviteURL(state string, permissions uint64) string {
	params := url.Values{}
	params.Set("client_id", d.clientID)
	params.Set("scope", "bot")
	params.Set("permissions", strconv.FormatUint(permissions, 10))
	params.Set("state", state)
	params.Set("redirect_uri", d.redirectURL)
	params.Set("response_type", "code")
	return discordAuthBase + "?" + params.Encode()
}

// ExchangeCode exchanges the authorization code for an OAuth2 token using the Discord token endpoint.
// The returned token's AccessToken belongs to the OAuth application, not to a guild or user.
func (d *DiscordOAuth) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	data := url.Values{}
	data.Set("client_id", d.clientID)
	data.Set("client_secret", d.clientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", d.redirectURL)

	req, err := http.NewRequestWithContext(ctx, "POST", discordTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", discordUserAgent)

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discord token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
		Scope       string `json:"scope"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &oauth2.Token{
		AccessToken: result.AccessToken,
		TokenType:   result.TokenType,
		Expiry:      time.Now().Add(time.Duration(result.ExpiresIn) * time.Second),
	}, nil
}

// GetUserInfo is a no-op stub. Discord bot auth has no user identity — the callback captures
// guild_id instead. The HandleDiscordConnect handler in handlers/discord.go bypasses this method.
func (d *DiscordOAuth) GetUserInfo(_ context.Context, _ string) (PlatformUserInfo, error) {
	return nil, fmt.Errorf("discord bot auth does not support GetUserInfo — use HandleDiscordConnect")
}

// RefreshToken is a no-op stub. The Discord bot uses a static DISCORD_BOT_TOKEN, not a
// refreshable OAuth token.
func (d *DiscordOAuth) RefreshToken(_ context.Context, _ string) (*oauth2.Token, error) {
	return nil, fmt.Errorf("discord bot auth does not support token refresh — bot token is static")
}

// GetPlatform returns the Discord platform identifier.
func (d *DiscordOAuth) GetPlatform() Platform {
	return PlatformDiscord
}

// GuildInfo holds the name and icon hash of a Discord guild.
type GuildInfo struct {
	Name string
	Icon string // may be empty
}

// GetGuildInfo fetches the guild name and icon from Discord using the bot token.
func (d *DiscordOAuth) GetGuildInfo(ctx context.Context, guildID string) (*GuildInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/guilds/%s", d.apiBase, guildID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create guild info request: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+d.botToken)
	req.Header.Set("User-Agent", discordUserAgent)

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("guild info request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("guild info returned %d: %s", resp.StatusCode, string(body))
	}

	var g struct {
		Name string `json:"name"`
		Icon string `json:"icon"`
	}
	if err := json.Unmarshal(body, &g); err != nil {
		return nil, fmt.Errorf("failed to decode guild info: %w", err)
	}
	return &GuildInfo{Name: g.Name, Icon: g.Icon}, nil
}

// botUserID returns the bot's own Discord user id, resolving it once via GET /users/@me
// and caching it (a bot user's id is immutable).
//
// "@me" is accepted only on routes Discord documents as current-user routes, such as
// /users/@me. As the {user_id} path parameter of GET /guilds/{guild_id}/members/{user_id}
// it is coerced to a snowflake and rejected with 400 NUMBER_TYPE_COERCE, and
// /users/@me/guilds/{guild_id}/member is closed to bots outright (403, code 20001) — both
// measured against the production application. So the bot's member record in a guild is
// reachable only by its explicit id.
func (d *DiscordOAuth) botUserID(ctx context.Context) (string, error) {
	d.botIDMu.Lock()
	defer d.botIDMu.Unlock()
	if d.botID != "" {
		return d.botID, nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", d.apiBase+"/users/@me", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create bot identity request: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+d.botToken)
	req.Header.Set("User-Agent", discordUserAgent)

	resp, err := d.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("bot identity request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("discord users/@me returned %d: %s", resp.StatusCode, string(body))
	}
	var self struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &self); err != nil {
		return "", fmt.Errorf("failed to decode bot identity: %w", err)
	}
	if self.ID == "" {
		return "", fmt.Errorf("discord bot identity carried no user id")
	}
	d.botID = self.ID
	return d.botID, nil
}

// CheckBotPermissions confirms the bot is a member of the guild.
// Discord enforces the requested permissions (68608) at invite time via the OAuth URL,
// so we only need to verify the bot joined successfully.
// Uses the DISCORD_BOT_TOKEN (not the OAuth access token).
func (d *DiscordOAuth) CheckBotPermissions(ctx context.Context, guildID string) ([]string, error) {
	botID, err := d.botUserID(ctx)
	if err != nil {
		return nil, err
	}
	reqURL := fmt.Sprintf("%s/guilds/%s/members/%s", d.apiBase, guildID, botID)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create membership request: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+d.botToken)
	req.Header.Set("User-Agent", discordUserAgent)

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("membership check request failed: %w", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body) //nolint:errcheck

	// 200 = bot is in the guild; permissions were enforced by Discord at invite time.
	// 404 = bot did not join (user may have cancelled or removed it immediately). Discord
	// answers 404 "Unknown User" for any snowflake that is not a member, the bot included.
	if resp.StatusCode == http.StatusNotFound {
		return []string{"Bot is not a member of this server"}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discord guild member lookup returned unexpected status %d", resp.StatusCode)
	}

	return nil, nil
}

// ComputeMissingPermissions is exported for unit testing. Given effective permission bits,
// returns human-readable names of permissions missing from RequiredBotPermissions.
func ComputeMissingPermissions(effective uint64) []string {
	missing := RequiredBotPermissions &^ effective
	if missing == 0 {
		return nil
	}
	var names []string
	if missing&PermViewChannel != 0 {
		names = append(names, "View Channels")
	}
	if missing&PermSendMessages != 0 {
		names = append(names, "Send Messages")
	}
	if missing&PermReadMessageHistory != 0 {
		names = append(names, "Read Message History")
	}
	return names
}
