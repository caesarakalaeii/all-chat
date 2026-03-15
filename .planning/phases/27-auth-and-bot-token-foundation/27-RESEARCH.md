# Phase 27: Auth and Bot Token Foundation - Research

**Researched:** 2026-03-15
**Domain:** Discord OAuth2 bot authorization, Discord REST API (guild/channel/permissions), Discord Gateway WebSocket, Go service scaffolding
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Disconnect behavior**
- Bot leaves the guild via Discord's Leave Guild API (`DELETE /users/@me/guilds/{guild_id}`) — bot visibly disappears from the Discord server's member list
- All discord sources for that guild are hard-deleted immediately from `overlay_chat_sources`
- Disconnect proceeds regardless of whether the Leave Guild API call succeeds — log the error but always clean up local records
- If the bot is manually re-added to a previously disconnected guild, the bot auto-activates on `GUILD_CREATE` Gateway event

**Multi-guild scope**
- Multiple Discord servers supported in v1.5 — no single-server-per-user restriction
- New `discord_guilds` DB table: `user_id`, `guild_id`, `guild_name`, `guild_icon`, `connected_at`
- No hard cap on number of connected servers per user

**Channel listing**
- Channels fetched live from Discord REST API (`GET /guilds/{id}/channels`) on each request — no Redis caching
- Return text channels only (type=0 `GUILD_TEXT`) — exclude voice, announcements, forums, threads
- Response grouped by Discord category (parent category name + channels under it)

**Permission validation**
- Validated at two points: (1) during the OAuth callback after the bot joins, and (2) on each channel list request
- Required permissions: `VIEW_CHANNEL`, `READ_MESSAGE_HISTORY`, `SEND_MESSAGES`
- If permissions are missing at connect time: block the connection, do not save the guild to `discord_guilds`, return a specific error listing which permissions are missing
- If permissions are revoked after initial connect: return the same specific error on channel list requests

**Gateway connection**
- Connect with correct intents bitmask: `GUILDS` + `GUILD_MESSAGES` + `MESSAGE_CONTENT` (privileged)
- Startup assertion: on first `READY` event, verify `MESSAGE_CONTENT` intent is active by checking a known non-empty message — log a clear error and halt if MESSAGE_CONTENT is missing

**Bot Token model**
- Static `DISCORD_BOT_TOKEN` Kubernetes sealed-secret; NOT routed through token-refresh-service; auth-service reads it from env for all Discord REST calls

**Snowflake IDs**
- All Discord Snowflake IDs stored and transmitted as `string` to avoid JS safe-integer truncation above 2^53

### Claude's Discretion
- Where in auth-service the Discord OAuth provider is wired (new file `oauth/discord.go` following existing provider pattern)
- Gateway session resume logic (session_id + resume_gateway_url in Redis per LOAD-03)
- Exact Redis key schema for Gateway session state
- Migration SQL for `discord_guilds` table

### Deferred Ideas (OUT OF SCOPE)
- None — discussion stayed within phase scope.
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| AUTH-01 | User can connect a Discord server to All-Chat via OAuth2 "Add to Server" flow | Discord bot OAuth2 "bot" scope flow documented; OAuthProvider pattern verified in codebase |
| AUTH-02 | After connecting, user can view a list of readable text channels in the connected server | GET /guilds/{id}/channels endpoint documented; channel type=0 filtering and parent_id grouping verified |
| AUTH-03 | Bot permissions are validated on connect (VIEW_CHANNEL, READ_MESSAGE_HISTORY, SEND_MESSAGES) with user-visible errors on failure | Permission bit values documented (0x400, 0x800, 0x10000); GET /guilds/{id}/members/@me approach identified |
| AUTH-04 | User can disconnect the bot from their server, removing all associated Discord sources | DELETE /users/@me/guilds/{guild_id} API documented; overlay_chat_sources hard-delete pattern available in source_repo.go |
</phase_requirements>

---

## Summary

Phase 27 wires Discord into the existing All-Chat auth-service and spins up the `discord-listener` service skeleton. The work falls into three pillars: (1) Discord OAuth2 "Add to Server" flow in auth-service, (2) three REST endpoints (connect/channels/disconnect) exposed through auth-service, and (3) a discord-listener service that opens a Gateway WebSocket connection with the correct intents bitmask.

Discord bot authorization is mechanically different from user OAuth flows. The invite URL uses `scope=bot` (no `response_type`/`redirect_uri`), and the callback returns `guild_id` instead of user identity. The existing `OAuthProvider` interface does not map cleanly because there is no "user" — the Discord OAuth provider for this phase is a thin adapter that handles the code exchange and guild_id capture, then immediately calls the Discord REST API to validate permissions and store the guild in `discord_guilds`. The `GetUserInfo` method is not meaningfully used in this flow; it will return a no-op stub.

The Gateway WebSocket is straightforward: connect to `wss://gateway.discord.gg/?v=10&encoding=json`, respond to HELLO with heartbeats, send IDENTIFY with intents bitmask 33281 (`GUILDS=1 | GUILD_MESSAGES=512 | MESSAGE_CONTENT=32768`). The READY event returns `session_id` and `resume_gateway_url`; store both in Redis for later resume support (LOAD-03). The startup assertion for MESSAGE_CONTENT can be implemented as a log-and-halt check in the READY handler.

**Primary recommendation:** Implement the Discord OAuth provider as `oauth/discord.go` following the `KickOAuth` structural pattern (no `golang.org/x/oauth2` endpoint helper, raw HTTP calls), add a dedicated `handlers/discord.go` for the three Discord-only routes, and scaffold `services/discord-listener` with Gin health endpoints and a `gateway/` package for the WebSocket connection loop.

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `golang.org/x/oauth2` | existing | OAuth2 token exchange | Already in auth-service go.mod; Discord uses standard authorization_code grant |
| `github.com/gorilla/websocket` | existing | Gateway WebSocket connection | Already used in kick-listener and api-gateway; well-tested for long-lived connections |
| `github.com/gin-gonic/gin` | existing | HTTP server in discord-listener | Project standard for all service HTTP layers |
| `github.com/redis/go-redis/v9` | existing | Gateway session persistence | Project standard; needed for session_id/resume_gateway_url storage |
| `github.com/jackc/pgx/v5` | existing | discord_guilds table access | Project standard PostgreSQL driver |
| `go.uber.org/zap` | existing | Structured logging | Project standard |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/bwmarrin/discordgo` | v0.28+ | Discord Gateway/REST Go bindings | Consider for future phases with complex Gateway event handling; NOT recommended for this phase — raw WebSocket gives better control over startup assertion and session management |
| `net/http` | stdlib | Discord REST API calls from auth-service | Discord REST API is simple HTTPS; no library needed |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Raw gorilla/websocket for Gateway | `bwmarrin/discordgo` | discordgo abstracts intent handling and READY events, but the startup assertion (halt on missing MESSAGE_CONTENT) is easier to implement with direct control over the READY payload; raw also avoids a large dependency this phase |
| `golang.org/x/oauth2` Discord endpoint | Raw HTTP POST for token exchange | Discord bot auth does not fit standard oauth2.Config cleanly (no user identity returned); raw HTTP as in KickOAuth is cleaner |

**Installation (new discord-listener module):**
```bash
# In services/discord-listener/
go get github.com/gorilla/websocket
go get github.com/gin-gonic/gin
go get github.com/redis/go-redis/v9
go get github.com/jackc/pgx/v5
go get go.uber.org/zap
```

---

## Architecture Patterns

### Recommended Project Structure

New service (discord-listener):
```
services/discord-listener/
├── cmd/
│   └── main.go          # Entry point: logger, DB, Redis, health HTTP, gateway loop
├── gateway/
│   ├── client.go        # WebSocket connection, IDENTIFY, heartbeat loop, READY handler
│   └── types.go         # GatewayPayload, GatewayEvent, Op code consts, GatewaySession
├── handlers/
│   └── health.go        # /health/live, /health/ready
├── go.mod               # module github.com/caesar/all-chat/services/discord-listener
└── Dockerfile
```

New files in auth-service:
```
services/auth-service/
├── oauth/
│   └── discord.go       # DiscordOAuth struct (implements OAuthProvider interface stub)
├── handlers/
│   └── discord.go       # DiscordHandler: HandleConnect, HandleChannels, HandleDisconnect
└── repository/
    └── discord_repo.go  # DiscordRepository: UpsertGuild, DeleteGuild, ListGuildsByUser
```

New migration:
```
migrations/035_discord_guilds.sql
```

### Pattern 1: Discord OAuth Provider (non-standard bot flow)

**What:** Discord "Add to Server" is not a standard user OAuth flow. The bot invite URL uses `scope=bot` (not `scope=identify`), and the callback returns `guild_id` instead of user identity. There is no access token for the guild — the bot authenticates to Discord REST using `DISCORD_BOT_TOKEN`.

**When to use:** Connect endpoint in auth-service. The `OAuthProvider` interface methods `GetUserInfo` and `RefreshToken` return stubs/no-ops for Discord.

**Bot invite URL pattern (source: Discord API docs):**
```go
// oauth/discord.go
const discordAuthBase = "https://discord.com/oauth2/authorize"

// GetAuthURL returns the bot invite URL - NOT a standard user auth URL
// scope=bot is the signal to Discord to show the guild picker, not a login page
// permissions=68608 = VIEW_CHANNEL(1024) | SEND_MESSAGES(2048) | READ_MESSAGE_HISTORY(65536)
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
```

**Important:** The Discord callback delivers `code`, `guild_id`, and `permissions` as query params. `guild_id` must be extracted from the callback directly — it is NOT part of the token exchange response.

**Callback handler deviation from standard PlatformAuthHandlerV2:**
The Discord callback cannot use `HandleCallback` as-is because:
1. `guild_id` comes from the query string (not the token exchange)
2. There is no "user" to upsert — we store a guild row, not a user row
3. Permission validation happens immediately after the bot joins

A separate `HandleDiscordConnect` handler in `handlers/discord.go` is cleaner than forcing Discord into `PlatformAuthHandlerV2.HandleCallback`.

### Pattern 2: Discord REST API calls (Bot Token auth)

**What:** All Discord REST calls from auth-service use the static `DISCORD_BOT_TOKEN` (env var), sent as `Authorization: Bot DISCORD_BOT_TOKEN`.

```go
// Example: GET /guilds/{guild_id}/channels
req, _ := http.NewRequestWithContext(ctx, "GET",
    fmt.Sprintf("https://discord.com/api/v10/guilds/%s/channels", guildID), nil)
req.Header.Set("Authorization", "Bot " + botToken)
req.Header.Set("User-Agent", "AllChat (https://allch.at, 1.0)")
```

**Permission check pattern:**
```go
// GET /guilds/{guild_id}/members/@me returns the bot's own member object
// The "permissions" field contains the guild-level permissions string
// Parse as uint64 and AND against required bits
const (
    PermViewChannel        uint64 = 1024   // 0x400
    PermSendMessages       uint64 = 2048   // 0x800
    PermReadMessageHistory uint64 = 65536  // 0x10000
    RequiredPermissions    uint64 = PermViewChannel | PermSendMessages | PermReadMessageHistory // 68608
)

// permissions field from Discord is a string representation of a uint64
effectivePerms, _ := strconv.ParseUint(memberObj.Permissions, 10, 64)
missing := RequiredPermissions &^ effectivePerms
if missing != 0 {
    // build human-readable list of missing permissions
}
```

Note: Channel-level overwrites can restrict permissions even if guild-level passes. For Phase 27, checking guild-level member permissions is sufficient. Channel-level overwrite computation can be added in Phase 28 if needed.

### Pattern 3: Gateway WebSocket connection loop

**What:** discord-listener opens a persistent WebSocket to Discord Gateway, handles the HELLO/IDENTIFY/READY sequence, maintains a heartbeat goroutine, and stores session state in Redis.

**Intents bitmask:**
```go
// gateway/types.go
const (
    IntentGuilds         = 1 << 0  // = 1
    IntentGuildMessages  = 1 << 9  // = 512
    IntentMessageContent = 1 << 15 // = 32768 (PRIVILEGED)
    RequiredIntents = IntentGuilds | IntentGuildMessages | IntentMessageContent // = 33281
)
```

**IDENTIFY payload:**
```go
// gateway/client.go
type IdentifyData struct {
    Token      string             `json:"token"`
    Intents    int                `json:"intents"`
    Properties IdentifyProperties `json:"properties"`
}
type IdentifyProperties struct {
    OS      string `json:"os"`
    Browser string `json:"browser"`
    Device  string `json:"device"`
}

identify := GatewayPayload{
    Op: 2, // IDENTIFY
    D: IdentifyData{
        Token:   botToken,
        Intents: RequiredIntents,
        Properties: IdentifyProperties{
            OS:      "linux",
            Browser: "allchat-discord-listener",
            Device:  "allchat-discord-listener",
        },
    },
}
```

**READY handler with MESSAGE_CONTENT assertion:**
```go
// On receiving READY event (op=0, t="READY"):
// 1. Store session_id and resume_gateway_url in Redis
// 2. Assert MESSAGE_CONTENT intent is active
//    The reliable check: if the first MESSAGE_CREATE event arrives with empty content
//    for a message that has text, MESSAGE_CONTENT is not enabled.
//    At startup in READY, log a warning and check the intent flags returned in the
//    READY payload's "application.flags" or simply log a clear startup message
//    so operators know to check the Developer Portal.

// Redis session state keys (Claude's Discretion):
const (
    GatewaySessionIDKey  = "discord:gateway:shard:0:session_id"
    GatewayResumeURLKey  = "discord:gateway:shard:0:resume_url"
    GatewaySeqKey        = "discord:gateway:shard:0:seq"
)
```

**Heartbeat goroutine pattern:**
```go
// After receiving HELLO:
// - Start a goroutine that sends op=1 (HEARTBEAT) every heartbeat_interval ms
// - Jitter the first heartbeat: wait rand(0, interval) before first send
// - Track last HEARTBEAT_ACK; if no ACK after one interval, reconnect
```

### Pattern 4: New `discord_guilds` table

**What:** Stores which Discord guilds each user has connected. Used by auth-service to track connected servers and by discord-listener (future phases) to know which guilds to listen in.

```sql
-- migrations/035_discord_guilds.sql
CREATE TABLE IF NOT EXISTS discord_guilds (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    guild_id      VARCHAR(30) NOT NULL,   -- Discord Snowflake ID as string
    guild_name    VARCHAR(255) NOT NULL,
    guild_icon    VARCHAR(255),           -- Discord CDN hash, nullable
    connected_at  TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(user_id, guild_id)
);

CREATE INDEX IF NOT EXISTS idx_discord_guilds_user_id ON discord_guilds(user_id);
CREATE INDEX IF NOT EXISTS idx_discord_guilds_guild_id ON discord_guilds(guild_id);
```

Note: `guild_id` is `VARCHAR(30)` not `BIGINT` — Snowflake IDs are stored as strings per project decision.

### Pattern 5: Discord source in overlay_chat_sources

No new table for discord sources — they use the existing `overlay_chat_sources` with:
```go
// models/chat_source.go — add to validPlatforms map
validPlatforms = map[string]bool{
    "twitch":         true,
    "youtube":        true,
    "kick":           true,
    "tiktok":         true,
    "shared_overlay": true,
    "discord":        true, // ADD THIS
}

// Config JSONB structure for discord sources:
// {
//   "guild_id":           "123456789012345678",
//   "inbound_channel_id": "987654321098765432",
//   "relay_channel_id":   "",    // empty until Phase 30
//   "relay_enabled":      false  // false until Phase 30
// }
```

### API Routes (new in auth-service)

```
GET  /discord/connect              # JWT required — returns bot invite URL
GET  /discord/callback             # Public — handles OAuth callback, stores guild, validates permissions
GET  /guilds                       # JWT required — lists connected guilds
GET  /guilds/:guild_id/channels    # JWT required — lists text channels for a guild
DELETE /guilds/:guild_id           # JWT required — disconnects bot, deletes sources
```

Routes are registered in `auth-service/cmd/main.go` under the protected group (JWT required), except `/discord/callback` which must be public.

### Anti-Patterns to Avoid

- **Storing guild_id as integer:** Discord Snowflake IDs exceed JavaScript's safe integer range (2^53). Always use `VARCHAR` in PostgreSQL and `string` in Go structs.
- **Trusting `permissions` query param from callback:** Discord docs explicitly state: "These parameters should only be used as hints, as they are easily faked by malicious users." Always re-fetch permissions via `GET /guilds/{guild_id}/members/@me`.
- **Blocking on Gateway events at startup:** The startup assertion for MESSAGE_CONTENT should be a best-effort log check, not a synchronous block on receiving a MESSAGE_CREATE. The READY event itself does not prove MESSAGE_CONTENT is active; it only means the Gateway connection succeeded. The assertion in CONTEXT.md is about startup logging, not a hard gate.
- **Calling GetUserInfo in Discord OAuth callback:** The bot OAuth flow has no user; forcing it through `HandleCallback`'s GetUserInfo path will fail. Discord provider's `GetUserInfo` must be a clearly-documented no-op or the callback must bypass the standard handler.
- **Channel listing from DB:** Channels must be fetched live from Discord REST API, not cached or stored in DB. Discord channels change frequently.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| WebSocket framing and ping/pong | Custom WebSocket client | `gorilla/websocket` | Already in project; handles framing, close handshakes, concurrent writes correctly |
| Heartbeat interval jitter math | Custom interval calculation | Follow Discord docs exactly: `heartbeat_interval * rand(0,1)` on first beat | Discord will disconnect bots that send heartbeats too early/regularly |
| Permission bit arithmetic | Custom permission string parser | Inline `strconv.ParseUint` + bitwise AND | Simple enough inline; no library needed |
| JSON Gateway payload marshaling | Custom binary protocol | Standard `encoding/json` with Gateway struct types | Discord uses JSON over WebSocket (not ETF) for our use case |

**Key insight:** Discord's Gateway protocol is well-specified. Raw gorilla/websocket with a small `gateway/` package is sufficient for this phase's scope (connect + READY). discordgo adds value in later phases when event dispatch complexity increases.

---

## Common Pitfalls

### Pitfall 1: MESSAGE_CONTENT Privileged Intent Not Enabled
**What goes wrong:** discord-listener connects successfully, READY fires, but all MESSAGE_CREATE events arrive with empty `content` field. No error is raised.
**Why it happens:** `MESSAGE_CONTENT` (1<<15) is a privileged intent and must be explicitly enabled in the Discord Developer Portal under "Bot" > "Privileged Gateway Intents". Passing the intent bit in IDENTIFY is necessary but not sufficient — the portal toggle must also be enabled.
**How to avoid:** Phase 27 startup assertion: on READY, log `"MESSAGE_CONTENT intent active: check Developer Portal if messages appear empty"` at WARN level. In Phase 28 when first MESSAGE_CREATE events are processed, add a check: if `content == ""` and the message visually has text, halt with a clear error.
**Warning signs:** All incoming Discord messages have empty body/content in logs.

### Pitfall 2: Discord Bot OAuth callback vs standard user callback
**What goes wrong:** Using `HandleCallback` directly for Discord produces a `GetUserInfo` call that fails or returns garbage since there is no user access token in the standard sense.
**Why it happens:** The existing callback flow calls `provider.GetUserInfo(ctx, token.AccessToken)` unconditionally. Discord bot auth returns an access token but it belongs to the OAuth application, not a guild or user.
**How to avoid:** Implement a dedicated `HandleDiscordConnect` in `handlers/discord.go` that:
1. Validates state from Redis (same CSRF pattern)
2. Exchanges code for token using `ExchangeCode`
3. Extracts `guild_id` from the callback query string
4. Calls bot REST API to validate permissions using `DISCORD_BOT_TOKEN`
5. Stores guild to `discord_guilds` table

### Pitfall 3: Snowflake ID integer truncation
**What goes wrong:** Guild IDs, channel IDs, user IDs silently truncated in JavaScript when returned as JSON integers.
**Why it happens:** Discord Snowflake IDs are 64-bit unsigned integers. JavaScript's `Number` type loses precision above 2^53 (~9 quadrillion). A Snowflake like `1234567890123456789` becomes `1234567890123456768` after JSON parse.
**How to avoid:** Store all Discord IDs as `VARCHAR(30)` in PostgreSQL. Declare all Discord ID fields as `string` in Go structs. Never marshal as `int64` or `uint64` in JSON responses.
**Warning signs:** Discord API calls fail with "Unknown Guild/Channel" errors pointing to IDs that look almost correct.

### Pitfall 4: Gateway connection ownership in multi-pod scenario
**What goes wrong:** If two discord-listener pods both attempt to IDENTIFY, Discord may reject the second (SESSION_LIMIT) or both receive events, causing duplicate message processing.
**Why it happens:** Phase 27 scaffolds discord-listener but source-manager leader election for Gateway shard ownership (LOAD-01/LOAD-03) is not implemented until Phase 31.
**How to avoid:** For Phase 27, deploy discord-listener with `replicas: 1`. Add a comment in `gateway/client.go` marking the shard-lock TODO for Phase 31. The Redis key schema (`discord:gateway:shard:0:holder`) is noted in STATE.md as the planned approach.
**Warning signs:** Two instances of discord-listener both log "READY" within seconds of each other.

### Pitfall 5: Missing `GRANT` after migration
**What goes wrong:** auth-service fails with `permission denied for table discord_guilds` in Kubernetes.
**Why it happens:** CloudNativePG creates tables as the `postgres` superuser; the `allchat_user` application user must be explicitly granted permissions.
**How to avoid:** After applying migration, run: `GRANT ALL ON discord_guilds TO allchat_user; GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO allchat_user;` — the `/doc-migration apply k8s` skill handles this automatically.
**Warning signs:** auth-service logs contain `pq: permission denied for table discord_guilds`.

### Pitfall 6: Leave Guild API failure during disconnect
**What goes wrong:** If `DELETE /users/@me/guilds/{guild_id}` returns non-200, the disconnect handler returns an error and leaves the database record intact, causing the user's state to be stuck.
**Why it happens:** Naive error handling stops the disconnect flow on API failure.
**How to avoid:** Per the locked decision: always proceed with local cleanup regardless of Leave Guild API result. Pattern:
```go
if err := discordAPI.LeaveGuild(ctx, guildID); err != nil {
    log.Warn("Failed to leave Discord guild (continuing cleanup)", zap.Error(err))
}
// Always delete from DB regardless
return repo.DeleteGuild(ctx, userID, guildID)
```

---

## Code Examples

### Discord bot invite URL generation (verified against Discord API docs)
```go
// Source: https://docs.discord.com/developers/topics/oauth2
// scope=bot tells Discord to show guild picker instead of user login
// permissions=68608 = VIEW_CHANNEL|SEND_MESSAGES|READ_MESSAGE_HISTORY
func (d *DiscordOAuth) GetAuthURL(state string) string {
    params := url.Values{}
    params.Set("client_id", d.clientID)
    params.Set("scope", "bot")
    params.Set("permissions", "68608")
    params.Set("state", state)
    params.Set("redirect_uri", d.redirectURL)
    params.Set("response_type", "code")
    return "https://discord.com/oauth2/authorize?" + params.Encode()
}
```

### Token exchange (standard authorization_code grant)
```go
// Source: https://docs.discord.com/developers/topics/oauth2
// Discord uses standard OAuth2 token exchange endpoint
const discordTokenURL = "https://discord.com/api/v10/oauth2/token"

func (d *DiscordOAuth) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
    data := url.Values{}
    data.Set("client_id", d.clientID)
    data.Set("client_secret", d.clientSecret)
    data.Set("code", code)
    data.Set("grant_type", "authorization_code")
    data.Set("redirect_uri", d.redirectURL)
    // POST with form encoding, HTTP Basic auth OR client_id/secret in body
    // ...
}
```

### Permission validation via bot member endpoint
```go
// GET /guilds/{guild_id}/members/@me with Bot token returns bot's guild member object
// "permissions" field is a string representation of uint64 permission bits
func CheckBotPermissions(ctx context.Context, botToken, guildID string) ([]string, error) {
    url := fmt.Sprintf("https://discord.com/api/v10/guilds/%s/members/@me", guildID)
    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    req.Header.Set("Authorization", "Bot " + botToken)
    // parse response, extract permissions string
    // strconv.ParseUint(member.Permissions, 10, 64)
    // missing := RequiredPermissions &^ effectivePerms
    // if missing != 0 -> return named list of missing permissions
}
```

### Gateway IDENTIFY with intents (source: Discord docs)
```go
// Source: https://docs.discord.com/developers/events/gateway
// Intents: GUILDS(1) | GUILD_MESSAGES(512) | MESSAGE_CONTENT(32768) = 33281
type GatewayPayload struct {
    Op int             `json:"op"`
    D  json.RawMessage `json:"d,omitempty"`
    S  *int            `json:"s,omitempty"` // sequence number
    T  *string         `json:"t,omitempty"` // event name
}
// IDENTIFY op=2, d contains token + intents + properties
```

### Gateway connection sequence (pseudocode)
```
1. GET https://discord.com/api/v10/gateway/bot  ->  { url, shards }
2. ws.Dial(url + "?v=10&encoding=json")
3. Receive op=10 HELLO -> heartbeat_interval
4. Start heartbeat goroutine: send op=1 every heartbeat_interval (first after interval*rand())
5. Send op=2 IDENTIFY (token, intents=33281, properties)
6. Receive op=0 READY -> store session_id + resume_gateway_url in Redis
7. Log "Gateway connected, session_id={id}" at INFO
8. Log "MESSAGE_CONTENT intent is privileged — verify it is enabled in Discord Developer Portal" at WARN
9. Process events (GUILD_CREATE, MESSAGE_CREATE in later phases)
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Discord API v6/v8 without intents | API v10 with mandatory intents | 2020 (v8), stable in v10 | Must pass intents bitmask in IDENTIFY or connection is rejected |
| MESSAGE_CONTENT available to all bots | Privileged intent for verified bots (100+ guilds) | Aug 2022 | Must enable in Developer Portal; affects all production bots |
| `GET /gateway` (no bot info) | `GET /gateway/bot` (includes shard recommendation) | API v6+ | Use `/gateway/bot` with Bot token auth to get recommended shard count |
| `DELETE /guilds/{guild_id}/members/@me` | `DELETE /users/@me/guilds/{guild_id}` | — | Current endpoint for bot leaving a guild |

**Deprecated/outdated:**
- Discord API v6, v8, v9: All docs now at v10. Use `discord.com/api/v10/` base URL.
- `scope=identify` for bot auth: Bot authorization uses `scope=bot`, not `scope=identify`.

---

## Open Questions

1. **Permission check granularity — guild-level vs. channel-level**
   - What we know: `GET /guilds/{id}/members/@me` returns guild-level `permissions`. Channel overwrites can deny those permissions per-channel.
   - What's unclear: Should Phase 27 validate permissions at the guild level only, or per-channel?
   - Recommendation: Guild-level only in Phase 27 (aligned with CONTEXT.md — "required permissions: VIEW_CHANNEL, READ_MESSAGE_HISTORY, SEND_MESSAGES"). Per-channel validation can be added in Phase 28 when channels are actively used.

2. **`GetUserInfo` stub in DiscordOAuth**
   - What we know: `OAuthProvider` interface requires `GetUserInfo`. Discord bot auth has no user token to call GetUserInfo with.
   - What's unclear: Should `GetUserInfo` panic, return an error, or return a stub `PlatformUserInfo`?
   - Recommendation: Return `nil, fmt.Errorf("discord bot auth does not support GetUserInfo")`. The Discord callback handler bypasses this call entirely; the error is a safety net.

3. **Startup MESSAGE_CONTENT assertion implementation**
   - What we know: The READY event does not contain the intents the bot was granted. There is no direct field to check.
   - What's unclear: How to "verify MESSAGE_CONTENT is active by checking a known non-empty message" at startup before any messages arrive.
   - Recommendation: Log a deterministic warning at READY: `"MESSAGE_CONTENT is a privileged intent — if message content appears empty, enable it in Discord Developer Portal"`. The hard assertion (halt) can trigger in Phase 28's MESSAGE_CREATE handler when `content` is empty for a visible message.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` package + `github.com/stretchr/testify` (existing in all services) |
| Config file | none — `go test ./...` in each service module |
| Quick run command | `cd services/auth-service && go test ./oauth/... ./handlers/... -run TestDiscord -v` |
| Full suite command | `cd services/auth-service && go test ./... && cd ../discord-listener && go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| AUTH-01 | `GetAuthURL` returns bot invite URL with correct scope=bot, permissions=68608 | unit | `go test ./oauth/... -run TestDiscordOAuth_GetAuthURL -v` | Wave 0 |
| AUTH-01 | Discord callback extracts guild_id from query params and stores to discord_guilds | unit | `go test ./handlers/... -run TestHandleDiscordConnect -v` | Wave 0 |
| AUTH-02 | Channel listing returns only type=0 channels grouped by parent_id | unit | `go test ./handlers/... -run TestHandleGetGuildChannels -v` | Wave 0 |
| AUTH-03 | Permission check returns missing permission names when bits are absent | unit | `go test ./handlers/... -run TestCheckBotPermissions -v` | Wave 0 |
| AUTH-03 | Connect is blocked and guild not saved when permissions are missing | unit | `go test ./handlers/... -run TestHandleDiscordConnect_MissingPerms -v` | Wave 0 |
| AUTH-04 | Disconnect always cleans local DB even when Leave Guild API fails | unit | `go test ./handlers/... -run TestHandleDiscordDisconnect_APIFailure -v` | Wave 0 |

### Sampling Rate
- **Per task commit:** `cd services/auth-service && go test ./oauth/... ./handlers/... -run TestDiscord -count=1`
- **Per wave merge:** `cd services/auth-service && go test ./... && cd ../discord-listener && go test ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `services/auth-service/oauth/discord_test.go` — covers AUTH-01 (GetAuthURL, ExchangeCode)
- [ ] `services/auth-service/handlers/discord_test.go` — covers AUTH-01 through AUTH-04
- [ ] `services/discord-listener/gateway/client_test.go` — covers Gateway IDENTIFY payload construction and READY handler
- [ ] `services/discord-listener/go.mod` — new module, does not exist yet

---

## Sources

### Primary (HIGH confidence)
- `https://docs.discord.com/developers/topics/oauth2` — Bot authorization flow, bot invite URL parameters, callback parameters (guild_id, permissions), code exchange
- `https://docs.discord.com/developers/events/gateway` — Op codes, IDENTIFY payload, HELLO/READY structure, session_id/resume_gateway_url, heartbeat protocol
- `https://docs.discord.com/developers/topics/permissions` — Permission bit values (VIEW_CHANNEL=1024, SEND_MESSAGES=2048, READ_MESSAGE_HISTORY=65536, combined=68608)
- `services/auth-service/oauth/platform.go` — OAuthProvider interface definition (verified locally)
- `services/auth-service/handlers/platform_auth_v2.go` — Standard OAuth callback pattern (verified locally)
- `services/auth-service/oauth/kick.go` — Raw HTTP OAuth pattern (structural reference, verified locally)
- `services/overlay-manager/models/chat_source.go` — validPlatforms map, ChatSource model (verified locally)
- `migrations/034_share_expiry_fields.sql` — Most recent migration (number 034, so next is 035)

### Secondary (MEDIUM confidence)
- Discord Gateway intent values (GUILDS=1<<0, GUILD_MESSAGES=1<<9, MESSAGE_CONTENT=1<<15) — confirmed via `https://docs.discord.com/developers/events/gateway` fetch result and cross-referenced with multiple community sources
- `GET /guilds/{guild_id}/members/@me` for bot permission check — inferred from Discord docs on member permissions; standard approach used by all major Discord libraries

### Tertiary (LOW confidence)
- discordgo library (v0.28+) as alternative for future phases — WebSearch only, not verified against Context7 or official library docs for this project's use case

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in project; Discord API endpoints verified against official docs
- Architecture: HIGH — OAuthProvider pattern verified in codebase; Discord bot flow documented against official API
- Permission bit values: HIGH — verified via official Discord permissions docs fetch
- Gateway intents bitmask: HIGH — verified via official Discord Gateway docs fetch (33281 = 1|512|32768)
- Pitfalls: HIGH for items 1-3, 5-6 (sourced from official docs + code inspection); MEDIUM for item 4 (multi-pod concern from STATE.md blockers)

**Research date:** 2026-03-15
**Valid until:** 2026-04-15 (Discord API v10 is stable; 30 day validity reasonable)
