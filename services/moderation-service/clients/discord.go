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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
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

	// botID caches the bot's own snowflake, needed to read its guild member record.
	// Guarded by botIDMu rather than a sync.Once so a transient failure is retried
	// instead of disabling Discord capabilities until the pod restarts.
	botIDMu sync.Mutex
	botID   string
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

// do issues an authenticated Discord REST request with the bot token + User-Agent and
// returns the response (caller closes the body). It centralises the auth headers so the
// member-moderation methods (timeout/ban/unban) and the read methods stay consistent.
func (d *DiscordClient) do(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, d.baseURL+path, rdr)
	if err != nil {
		return nil, fmt.Errorf("discord: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bot "+d.botToken)
	req.Header.Set("User-Agent", discordUserAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return d.httpClient.Do(req)
}

// GuildIDForChannel resolves the guild a text channel belongs to via
// GET /channels/{channel_id}. Member-level moderation (timeout/ban/unban) is guild-scoped
// but the normalized chat message and the dashboard request only carry the channel id, so
// the moderation service resolves the guild here. The mapping is immutable, so callers
// cache it (DiscordGuildResolver). A 404 means the bot can no longer see the channel.
func (d *DiscordClient) GuildIDForChannel(ctx context.Context, channelID string) (string, error) {
	resp, err := d.do(ctx, http.MethodGet, "/channels/"+channelID, nil)
	if err != nil {
		return "", fmt.Errorf("discord: channel request failed: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return "", ErrDiscordUnauthorized
	case resp.StatusCode == http.StatusForbidden:
		return "", ErrDiscordForbidden
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("discord: get channel returned %s: %s", strconv.Itoa(resp.StatusCode), string(snippet))
	}

	var ch struct {
		GuildID string `json:"guild_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ch); err != nil {
		return "", fmt.Errorf("discord: decode channel: %w", err)
	}
	if ch.GuildID == "" {
		// A DM/group channel has no guild — moderation is not possible there.
		return "", fmt.Errorf("discord: channel %s is not in a guild", channelID)
	}
	return ch.GuildID, nil
}

// BotUserID returns the bot's own Discord user id, resolving it once via GET /users/@me
// and caching it for the process lifetime (a bot user's id is immutable).
//
// The indirection is not incidental. "@me" is only accepted on routes Discord documents as
// current-user routes, such as /users/@me. As the {user_id} path parameter of
// GET /guilds/{guild_id}/members/{user_id} it is coerced to a snowflake and rejected with
// 400 NUMBER_TYPE_COERCE, and /users/@me/guilds/{guild_id}/member is closed to bots
// outright (403, code 20001) — both measured against the production application. So the
// bot's own member record is reachable only by its explicit id.
func (d *DiscordClient) BotUserID(ctx context.Context) (string, error) {
	d.botIDMu.Lock()
	defer d.botIDMu.Unlock()
	if d.botID != "" {
		return d.botID, nil
	}

	resp, err := d.do(ctx, http.MethodGet, "/users/@me", nil)
	if err != nil {
		return "", fmt.Errorf("discord: bot identity request failed: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return "", ErrDiscordUnauthorized
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("discord: get bot identity returned %s: %s", strconv.Itoa(resp.StatusCode), string(snippet))
	}
	var self struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&self); err != nil {
		return "", fmt.Errorf("discord: decode bot identity: %w", err)
	}
	if self.ID == "" {
		return "", fmt.Errorf("discord: bot identity carried no user id")
	}
	d.botID = self.ID
	return d.botID, nil
}

// GuildBotPermissions computes the bot's EFFECTIVE guild-level permission bits as the OR
// of the permissions of every role the bot holds, plus the @everyone role (whose id is
// the guild id). This is the honest input to ActionsForDiscordPermissions: the capability
// endpoint reports exactly the moderation actions the bot's invite actually granted.
// Guild-level perms are authoritative for ban/timeout (member ops); a channel overwrite
// could still deny delete (MANAGE_MESSAGES), which the dispatcher surfaces as a 403.
//
// An unresolvable bot identity is returned as an error rather than as zero permissions:
// zero is indistinguishable from "invited without moderation permissions", which would
// tell the streamer to re-invite a bot that is already correctly invited.
func (d *DiscordClient) GuildBotPermissions(ctx context.Context, guildID string) (uint64, error) {
	botID, err := d.BotUserID(ctx)
	if err != nil {
		return 0, err
	}
	// The ownership read is skipped deliberately: a bot can never own a guild, so asking
	// would be a wasted call on the capability path.
	standing, err := d.memberRoleStanding(ctx, guildID, botID)
	if err != nil {
		return 0, err
	}
	return standing.Permissions, nil
}

// DiscordMember is one user's live standing in one guild.
//
// It mirrors models.DiscordMemberAuthority field for field, and the duplication is
// deliberate: clients holds no dependency on the domain packages, the same reason
// models.Actor duplicates repository.Role* rather than importing it. The dispatch layer,
// which already imports both, does the one-line conversion.
//
// The zero value is "not a member", which denies everything — so a caller that ignores an
// error still fails closed.
type DiscordMember struct {
	InGuild        bool
	IsGuildOwner   bool
	Permissions    uint64
	HighestRolePos int
}

// MemberAuthority reads what one Discord user may do in one guild: their effective
// guild-level permission bits, the position of their highest role, and whether they own the
// guild.
//
// This is the live read ADR-0048's platform-attested model rests on. Discord never re-checks
// a delegated moderator — every write authenticates as the shared bot — so this read IS the
// authorization input, and every failure mode other than "not a member" is returned as an
// error rather than as a permissive or empty standing.
//
// A 404 is an answer, not a failure: it means the user is not in the guild, and it
// short-circuits before the role and ownership reads, so a stranger costs one call.
func (d *DiscordClient) MemberAuthority(ctx context.Context, guildID, userID string) (DiscordMember, error) {
	standing, err := d.memberRoleStanding(ctx, guildID, userID)
	if err != nil || !standing.InGuild {
		return standing, err
	}

	ownerID, err := d.guildOwnerID(ctx, guildID)
	if err != nil {
		return DiscordMember{}, err
	}
	standing.IsGuildOwner = ownerID != "" && ownerID == userID
	return standing, nil
}

// memberRoleStanding reads a member's roles and folds the guild's role definitions into
// effective permission bits and a highest-role position. It is shared by the bot's own
// permission check and the general member read so the two computations cannot drift.
func (d *DiscordClient) memberRoleStanding(ctx context.Context, guildID, userID string) (DiscordMember, error) {
	memberResp, err := d.do(ctx, http.MethodGet, "/guilds/"+guildID+"/members/"+userID, nil)
	if err != nil {
		return DiscordMember{}, fmt.Errorf("discord: member request failed: %w", err)
	}
	defer memberResp.Body.Close()
	switch {
	case memberResp.StatusCode == http.StatusUnauthorized:
		return DiscordMember{}, ErrDiscordUnauthorized
	case memberResp.StatusCode == http.StatusForbidden:
		return DiscordMember{}, ErrDiscordForbidden
	case memberResp.StatusCode == http.StatusNotFound:
		// Not a member of the guild — they can do nothing here. Discord answers 404
		// "Unknown User" for any snowflake that is not a member.
		return DiscordMember{}, nil
	case memberResp.StatusCode < 200 || memberResp.StatusCode >= 300:
		snippet, _ := io.ReadAll(io.LimitReader(memberResp.Body, 512))
		return DiscordMember{}, fmt.Errorf("discord: get member returned %s: %s", strconv.Itoa(memberResp.StatusCode), string(snippet))
	}
	var member struct {
		Roles []string `json:"roles"`
	}
	if err := json.NewDecoder(memberResp.Body).Decode(&member); err != nil {
		return DiscordMember{}, fmt.Errorf("discord: decode member: %w", err)
	}

	roles, err := d.guildRoles(ctx, guildID)
	if err != nil {
		return DiscordMember{}, err
	}

	// @everyone (role id == guild id) is implicit — Discord does not list it in a member's
	// roles — so it is always held, on top of the assigned roles.
	held := map[string]bool{guildID: true}
	for _, r := range member.Roles {
		held[r] = true
	}
	standing := DiscordMember{InGuild: true}
	for _, r := range roles {
		if !held[r.ID] {
			continue
		}
		bits, perr := strconv.ParseUint(r.Permissions, 10, 64)
		if perr != nil {
			return DiscordMember{}, fmt.Errorf("discord: parse role permissions %q: %w", r.Permissions, perr)
		}
		standing.Permissions |= bits
		if r.Position > standing.HighestRolePos {
			standing.HighestRolePos = r.Position
		}
	}
	return standing, nil
}

// discordRole is one guild role as Discord reports it. Permissions is a decimal string
// because the bitfield exceeds what JSON numbers represent exactly.
type discordRole struct {
	ID          string `json:"id"`
	Permissions string `json:"permissions"`
	Position    int    `json:"position"`
}

// guildRoles lists the guild's role definitions.
func (d *DiscordClient) guildRoles(ctx context.Context, guildID string) ([]discordRole, error) {
	resp, err := d.do(ctx, http.MethodGet, "/guilds/"+guildID+"/roles", nil)
	if err != nil {
		return nil, fmt.Errorf("discord: roles request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("discord: get roles returned %s: %s", strconv.Itoa(resp.StatusCode), string(snippet))
	}
	var roles []discordRole
	if err := json.NewDecoder(resp.Body).Decode(&roles); err != nil {
		return nil, fmt.Errorf("discord: decode roles: %w", err)
	}
	return roles, nil
}

// guildOwnerID reads the guild's owner. Ownership implicitly grants every permission and
// makes a member untouchable by timeout/ban, so it cannot be inferred from permission bits.
func (d *DiscordClient) guildOwnerID(ctx context.Context, guildID string) (string, error) {
	resp, err := d.do(ctx, http.MethodGet, "/guilds/"+guildID, nil)
	if err != nil {
		return "", fmt.Errorf("discord: guild request failed: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return "", ErrDiscordUnauthorized
	case resp.StatusCode == http.StatusForbidden:
		return "", ErrDiscordForbidden
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("discord: get guild returned %s: %s", strconv.Itoa(resp.StatusCode), string(snippet))
	}
	var guild struct {
		OwnerID string `json:"owner_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&guild); err != nil {
		return "", fmt.Errorf("discord: decode guild: %w", err)
	}
	return guild.OwnerID, nil
}

// TimeoutMember sets a member's communication_disabled_until to `until` (a future
// timestamp), muting them until then. Requires MODERATE_MEMBERS.
func (d *DiscordClient) TimeoutMember(ctx context.Context, guildID, userID string, until time.Time) error {
	body, err := json.Marshal(map[string]any{"communication_disabled_until": until.UTC().Format(time.RFC3339)})
	if err != nil {
		return fmt.Errorf("discord: marshal timeout body: %w", err)
	}
	return d.memberWrite(ctx, http.MethodPatch, "/guilds/"+guildID+"/members/"+userID, body, "timeout", false)
}

// BanMember permanently bans a user from the guild. Requires BAN_MEMBERS.
func (d *DiscordClient) BanMember(ctx context.Context, guildID, userID string) error {
	return d.memberWrite(ctx, http.MethodPut, "/guilds/"+guildID+"/bans/"+userID, []byte("{}"), "ban", false)
}

// UnbanMember lifts a guild ban. Requires BAN_MEMBERS. A 404 (the user was not banned)
// is treated as idempotent success — the moderation goal (the user is not banned) holds.
func (d *DiscordClient) UnbanMember(ctx context.Context, guildID, userID string) error {
	return d.memberWrite(ctx, http.MethodDelete, "/guilds/"+guildID+"/bans/"+userID, nil, "unban", true)
}

// memberWrite issues a guild member/ban write and maps the status: 2xx → success;
// 404 → success only when notFoundOK (idempotent unban); 401/403 → the sentinels so the
// dispatcher can tell "re-invite the bot with moderation permissions" from a real error.
func (d *DiscordClient) memberWrite(ctx context.Context, method, path string, body []byte, op string, notFoundOK bool) error {
	resp, err := d.do(ctx, method, path, body)
	if err != nil {
		return fmt.Errorf("discord: %s request failed: %w", op, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == http.StatusNotFound && notFoundOK:
		return nil
	case resp.StatusCode == http.StatusUnauthorized:
		return ErrDiscordUnauthorized
	case resp.StatusCode == http.StatusForbidden:
		return ErrDiscordForbidden
	default:
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("discord: %s returned %s: %s", op, strconv.Itoa(resp.StatusCode), string(snippet))
	}
}

// discordResolverAPI is the subset of DiscordClient the resolver caches (for testability).
type discordResolverAPI interface {
	GuildIDForChannel(ctx context.Context, channelID string) (string, error)
	GuildBotPermissions(ctx context.Context, guildID string) (uint64, error)
	MemberAuthority(ctx context.Context, guildID, userID string) (DiscordMember, error)
}

// DiscordGuildResolver caches the two Discord lookups the moderation service makes per
// guild/channel so member-level moderation and the capability check do not hit the
// Discord API on every request: the (immutable) channel→guild mapping and the bot's
// effective guild permissions. Mirrors the YouTubeLiveChatResolver pattern; falls back to
// the live API on a cache miss.
type DiscordGuildResolver struct {
	api   discordResolverAPI
	redis *redis.Client
}

// NewDiscordGuildResolver wires a resolver over a DiscordClient and the shared Redis.
func NewDiscordGuildResolver(api discordResolverAPI, client *redis.Client) *DiscordGuildResolver {
	return &DiscordGuildResolver{api: api, redis: client}
}

const (
	// discordGuildCacheTTL bounds the channel→guild entry; the mapping is immutable but
	// the TTL caps stale entries for deleted channels.
	discordGuildCacheTTL = 24 * time.Hour
	// discordPermsCacheTTL bounds the per-guild effective-permissions entry: short, so a
	// re-invite that grants moderation permissions is reflected on the dashboard quickly.
	discordPermsCacheTTL = 5 * time.Minute
	// discordMemberAuthorityCacheTTL bounds a delegated moderator's standing in a guild.
	//
	// This is a SECURITY BOUND, not a tuning knob (ADR-0048). Discord is the one platform
	// where nothing external re-checks a delegated moderator, and because the GUILD_MEMBERS
	// privileged intent is off, Discord cannot push us a revocation either. So this TTL is
	// exactly how long someone the streamer just stripped of their roles keeps being able to
	// act. Raising it lengthens that window; the ADR fixes the ceiling at 60 seconds.
	discordMemberAuthorityCacheTTL = 60 * time.Second
)

// GuildID returns the guild id for a channel, from cache when possible.
func (r *DiscordGuildResolver) GuildID(ctx context.Context, channelID string) (string, error) {
	key := "discord:channel:guild:" + channelID
	if r.redis != nil {
		if v, err := r.redis.Get(ctx, key).Result(); err == nil && v != "" {
			return v, nil
		}
	}
	guildID, err := r.api.GuildIDForChannel(ctx, channelID)
	if err != nil {
		return "", err
	}
	if r.redis != nil {
		// Best-effort cache write — a failure just means the next call re-resolves.
		_ = r.redis.Set(ctx, key, guildID, discordGuildCacheTTL).Err()
	}
	return guildID, nil
}

// GuildBotPermissions returns the bot's effective permission bits in the guild, cached
// for discordPermsCacheTTL so the capability endpoint does not re-read the bot's member
// record + the guild roles on every dashboard load. A cache miss recomputes from the live
// API. Errors are deliberately not cached (see the resolver's other methods).
func (r *DiscordGuildResolver) GuildBotPermissions(ctx context.Context, guildID string) (uint64, error) {
	key := "discord:guild:perms:" + guildID
	if r.redis != nil {
		if v, err := r.redis.Get(ctx, key).Uint64(); err == nil {
			return v, nil
		}
	}
	bits, err := r.api.GuildBotPermissions(ctx, guildID)
	if err != nil {
		return 0, err
	}
	if r.redis != nil {
		_ = r.redis.Set(ctx, key, bits, discordPermsCacheTTL).Err()
	}
	return bits, nil
}

// MemberAuthority returns a user's standing in a guild, cached for
// discordMemberAuthorityCacheTTL — the security bound described on that constant, since the
// check runs on every delegated action and Discord cannot notify us of a revocation.
//
// "Not a member" is cached like any other answer: it is a real standing, and it is the most
// likely one for a prober, so re-reading it every time would let an attacker drive our
// Discord API traffic. Errors are never cached — caching one would extend a transient
// Discord outage into a full TTL of denials.
func (r *DiscordGuildResolver) MemberAuthority(ctx context.Context, guildID, userID string) (DiscordMember, error) {
	key := "discord:member:authority:" + guildID + ":" + userID
	if r.redis != nil {
		if v, err := r.redis.Get(ctx, key).Bytes(); err == nil && len(v) > 0 {
			var cached DiscordMember
			if json.Unmarshal(v, &cached) == nil {
				return cached, nil
			}
		}
	}
	standing, err := r.api.MemberAuthority(ctx, guildID, userID)
	if err != nil {
		return DiscordMember{}, err
	}
	if r.redis != nil {
		if payload, merr := json.Marshal(standing); merr == nil {
			_ = r.redis.Set(ctx, key, payload, discordMemberAuthorityCacheTTL).Err()
		}
	}
	return standing, nil
}
