package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
}

// NewDiscordOAuth creates a new DiscordOAuth provider.
// botToken is the static DISCORD_BOT_TOKEN secret (not the OAuth access token).
func NewDiscordOAuth(clientID, clientSecret, redirectURL string) *DiscordOAuth {
	return &DiscordOAuth{
		clientID:     clientID,
		clientSecret: clientSecret,
		redirectURL:  redirectURL,
		client:       &http.Client{Timeout: 10 * time.Second},
	}
}

// WithBotToken sets the bot token used for REST API permission checks.
func (d *DiscordOAuth) WithBotToken(botToken string) *DiscordOAuth {
	d.botToken = botToken
	return d
}

// GetAuthURL returns the Discord bot invite URL. This shows a guild picker (not a user login page)
// because scope=bot signals Discord to use the bot authorization flow.
// permissions=68608 requests VIEW_CHANNEL | SEND_MESSAGES | READ_MESSAGE_HISTORY.
func (d *DiscordOAuth) GetAuthURL(state string) string {
	params := url.Values{}
	params.Set("client_id", d.clientID)
	params.Set("scope", "bot")
	params.Set("permissions", "68608")
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

// CheckBotPermissions fetches the bot's guild member object and returns the names of any
// missing required permissions. Returns an empty slice if all required permissions are present.
// Uses the DISCORD_BOT_TOKEN (not the OAuth access token).
func (d *DiscordOAuth) CheckBotPermissions(ctx context.Context, guildID string) ([]string, error) {
	reqURL := fmt.Sprintf("%s/guilds/%s/members/@me", discordAPIBase, guildID)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create permissions request: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+d.botToken)
	req.Header.Set("User-Agent", discordUserAgent)

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("permissions check request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discord members/@me returned %d: %s", resp.StatusCode, string(body))
	}

	var member struct {
		Permissions string `json:"permissions"` // string representation of uint64 permission bits
	}
	if err := json.Unmarshal(body, &member); err != nil {
		return nil, fmt.Errorf("failed to decode member response: %w", err)
	}

	effectivePerms, err := parsePermissions(member.Permissions)
	if err != nil {
		return nil, fmt.Errorf("failed to parse permissions string: %w", err)
	}

	return ComputeMissingPermissions(effectivePerms), nil
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

// parsePermissions parses the Discord permissions string (decimal uint64 representation).
func parsePermissions(s string) (uint64, error) {
	if s == "" {
		return 0, nil
	}
	var v uint64
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}
