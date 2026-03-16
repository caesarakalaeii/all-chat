# Technology Stack: Discord Listener + Relay

**Project:** All-Chat v1.5 — Discord Listener
**Researched:** 2026-03-15
**Scope:** Additions required for discord-listener service (inbound Gateway WebSocket + outbound REST relay)

---

## Context: What Already Exists (Do Not Re-Introduce)

Verified from actual go.mod files in this repository. Pin new dependencies to match these versions:

| Dependency | Version | Source |
|------------|---------|--------|
| Go | 1.25.6 | All go.mod files |
| `redis/go-redis/v9` | v9.18.0 | twitch-listener, kick-listener go.mod |
| `gin-gonic/gin` | v1.12.0 | All services |
| `jackc/pgx/v5` | v5.8.0 | All services |
| `go.uber.org/zap` | v1.27.1 | All services |
| `prometheus/client_golang` | v1.23.2 | All services |
| `go.opentelemetry.io/otel` | v1.42.0 | All services |
| `golang.org/x/oauth2` | v0.36.0 | auth-service go.mod |
| `golang-jwt/jwt/v5` | v5.3.1 | twitch-listener (indirect), auth-service |
| `google/uuid` | v1.6.0 | twitch-listener go.mod |
| `stretchr/testify` | v1.11.1 | twitch-listener go.mod |
| `golang.org/x/time` | v0.15.0 | twitch-listener go.mod |

---

## New Dependencies for discord-listener

### Core Addition: discordgo

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `bwmarrin/discordgo` | v0.28.1 | Discord Gateway WebSocket client + REST API | The canonical Go Discord library. Covers Gateway v10 for inbound message events and REST API v10 for outbound message posting. Handles reconnect, heartbeat, session resume, and per-route rate limiting automatically. No meaningful alternative in the Go ecosystem. |

**Confidence:** MEDIUM — v0.28.1 was the latest stable release as of training cutoff (August 2025). Run `go get github.com/bwmarrin/discordgo@latest` before pinning to confirm the resolved version.

**Why not raw gorilla/websocket + REST?** The kick-listener uses raw `gorilla/websocket` because Kick's protocol is simple Pusher WebSocket with no complex handshake. Discord's Gateway protocol requires: sequence number tracking, heartbeat acknowledgement with `HEARTBEAT_ACK`, session resume on reconnect, optional zlib stream decompression, and ETF or JSON payload selection. discordgo implements all of this correctly. Building it raw would be multiple weeks of protocol work for zero product benefit and high maintenance risk.

**Why not discord-interactions or other Go libraries?** Libraries built around the "interactions" model target slash commands delivered via HTTP webhooks — they cannot receive `MESSAGE_CREATE` Gateway events at all. discordgo is the only full-featured Go option that covers both Gateway (inbound) and REST (outbound).

---

## Discord API Version

**Use: Discord API v10 (Gateway v10)**

discordgo defaults to API v10 as of v0.28.x. Do not override the API version constant.

- v10 is the current stable and actively maintained version (released February 2022)
- v6 and v8 are disabled by Discord — connections are rejected
- v9 is deprecated; Discord recommends migration to v10
- No v11 has been announced as of the research date

**Confidence:** HIGH — Discord API versioning is stable and well-documented. v10 has been the sole recommended version since 2022.

---

## OAuth2 Scopes for Bot Authorization

Discord bot authorization uses a two-scope grant. This differs fundamentally from Twitch/YouTube OAuth flows where the user authenticates their own account. For Discord, the user authorizes the bot to join their server.

**Required scopes for the "Add to Server" OAuth2 flow:**

| Scope | Required | Why |
|-------|----------|-----|
| `bot` | Yes | Grants the bot user presence in the guild; required for all Gateway connections |
| `applications.commands` | No — include anyway | Allows future slash command registration; costs nothing, avoids needing a second authorization grant later |

**Authorization URL pattern:**

```
https://discord.com/oauth2/authorize
  ?client_id={APPLICATION_CLIENT_ID}
  &permissions={PERMISSION_INTEGER}
  &scope=bot%20applications.commands
```

This is NOT a standard `code` exchange flow for per-user access tokens. The user clicks "Authorize", Discord adds the bot to the server, and Discord redirects to `redirect_uri` with a `code`. The code confirms guild membership — it is NOT used to derive an ongoing per-guild token.

**The discord-listener service authenticates using a Bot Token** (`DISCORD_BOT_TOKEN` env var), not per-user OAuth tokens. The Bot Token is static, issued once from the Discord Developer Portal, and stored as a Kubernetes sealed-secret. There is no refresh cycle for bot tokens.

**Confidence:** HIGH — Discord's bot authorization scope model has been unchanged since the introduction of Gateway Intents.

---

## Gateway Intents

Gateway Intents are mandatory in Discord API v10. Undeclared event types are silently dropped by Discord's Gateway — there is no error, messages simply never arrive.

**Required intents for reading channel messages:**

| Intent | Bit | Value | Type | Required For |
|--------|-----|-------|------|-------------|
| `GUILDS` | 1 << 0 | 1 | Non-privileged | Guild and channel metadata; needed to resolve channel names for `RawChatMessage.ChannelName` |
| `GUILD_MESSAGES` | 1 << 9 | 512 | Non-privileged | Receive `MESSAGE_CREATE` events in guilds (but not message content without the next intent) |
| `MESSAGE_CONTENT` | 1 << 15 | 32768 | **Privileged** | Receive actual text content of messages; without this, `message.Content` is always empty string |

`MESSAGE_CONTENT` is a privileged intent. It must be explicitly enabled in the Discord Developer Portal under **Bot > Privileged Gateway Intents**. If not enabled in the portal, the bot connects successfully but receives empty message bodies — this failure mode is silent and confusing to debug.

**In discordgo:**

```go
session.Identify.Intents = discordgo.IntentsGuilds |
    discordgo.IntentsGuildMessages |
    discordgo.IntentsMessageContent
```

**Confidence:** HIGH — The privileged intent requirement for `MESSAGE_CONTENT` has been enforced by Discord since April 2022. The discordgo constants `IntentsGuilds`, `IntentsGuildMessages`, `IntentsMessageContent` are exported constants in the package. No changes anticipated.

---

## Bot Permissions Integer

The `permissions` parameter in the authorization URL controls what the bot can do in channels. This is a bitmask of Discord permission flags.

**Required permissions for inbound + outbound use case:**

| Permission | Bit | Decimal | Required For |
|------------|-----|---------|-------------|
| `VIEW_CHANNEL` | 10 | 1024 | Read any channel; required for message ingestion |
| `READ_MESSAGE_HISTORY` | 16 | 65536 | Access historical messages; useful for context on startup |
| `SEND_MESSAGES` | 11 | 2048 | Post relay messages to outbound channel |

**Minimum permission integer:** `1024 + 65536 + 2048 = 68608`

Optionally add `EMBED_LINKS` (1 << 14 = 16384) if relay messages use rich embeds: `84992`.

**Confidence:** HIGH — Permission bit values are stable Discord API constants.

---

## What NOT to Add

| Temptation | Why to Avoid |
|------------|-------------|
| `gorilla/websocket` as a direct dependency | discordgo bundles its own WebSocket handling internally. Adding gorilla/websocket as a separate dependency creates no benefit and risks version conflicts. The kick-listener uses gorilla/websocket directly only because Kick has no Go library. |
| Separate relay microservice | The relay (outbound posting) is a single `session.ChannelMessageSend()` REST call. It does not justify a separate deployment. Loop-safe filtering is straightforward: check `msg.Platform != "discord"` on the `UnifiedChatMessage.Platform` field before relaying. One service, two directions. |
| Discord Webhook API instead of Bot API | Webhooks can only post messages — they cannot receive incoming messages. They also lack centralized auth management. The bot token approach supports both directions with one credential. |
| Any additional WebSocket library | discordgo already handles the WebSocket connection. No additional WebSocket library is needed. |
| The existing discord-bot (Node.js) service | The existing `services/discord-bot` is a Node.js YouTube quota monitor that posts to a single hardcoded Discord channel. It has no overlap with the chat listener use case. The discord-listener is an independent Go service. |
| `nhooyr.io/websocket` | Same rationale as gorilla/websocket — discordgo handles this internally. |

---

## Integration with Existing Stack

### Inbound: Discord Gateway → Redis Streams

discordgo fires `MessageCreate` events via registered handler callbacks. Map each event to `RawChatMessage` and publish to the `chat:raw` stream using the identical publisher pattern from twitch-listener (verified: `StreamKey = "chat:raw"`, `XADD` with `MaxLen: 1000000`, `Approx: true`).

```
discordgo AddHandler(MessageCreate)
  → map to RawChatMessage{
      MessageID:   m.Message.ID,
      Platform:    "discord",
      ChannelID:   m.Message.ChannelID,
      ChannelName: (resolved from session.State.Channel cache),
      UserID:      m.Message.Author.ID,
      Username:    m.Message.Author.Username,
      Text:        m.Message.Content,
      Tags:        map[string]string{
                     "guild_id":   m.Message.GuildID,
                     "avatar_url": discordgo.EndpointUserAvatar(author.ID, author.Avatar),
                   },
    }
  → StreamPublisher.Publish(ctx, msg)
```

The `RawChatMessage` schema requires no changes — all fields already exist (verified from `services/message-processor/models/message.go`). `Platform: "discord"` is a new platform string value. A `discord_normalizer.go` in message-processor is needed (pattern: identical to `kick_normalizer.go`).

### Outbound: Overlay Pub/Sub → Discord REST

The relay component subscribes to `overlay:{overlay_id}` Redis Pub/Sub channels (same pattern as api-gateway). For each `UnifiedChatMessage` received:

1. Check `msg.Platform != "discord"` to prevent relay loops — this field is on `UnifiedChatMessage` directly (verified from message.go)
2. Look up the configured outbound channel ID for this overlay from PostgreSQL
3. Call `session.ChannelMessageSend(outboundChannelID, formattedText)` via discordgo

**Rate limiting:** Discord enforces 5 messages per 5 seconds per channel for bots. Use `golang.org/x/time/rate` (already a dependency via twitch-listener) with rate `1/s` and burst `5` per outbound channel.

### Auth Service Integration

The existing auth-service `OAuthProvider` interface (verified from `oauth/platform.go`) expects `GetAuthURL`, `ExchangeCode`, `GetUserInfo`, `RefreshToken`, `GetPlatform`. A `DiscordOAuth` struct implementing this interface is needed in auth-service.

**Key difference from Twitch/YouTube:** Discord's callback code exchange confirms guild membership; it does not return an ongoing access token for API calls. Store the `guild_id` returned in the callback, not a token pair. No entry in the token-refresh-service is needed for Discord.

Add `PlatformDiscord Platform = "discord"` to `oauth/platform.go`.

**Bot token:** Static credential in Kubernetes sealed-secret `DISCORD_BOT_TOKEN`. Not managed by auth-service.

---

## Installation

```bash
# In services/discord-listener/
go mod init github.com/caesar/all-chat/services/discord-listener
go get github.com/bwmarrin/discordgo@latest

# Verify resolved version — expected v0.28.1 or higher
cat go.sum | grep discordgo

# Copy all standard listener dependencies from twitch-listener go.mod:
# gin, pgx, go-redis, zap, prometheus, otel, uuid, testify, golang.org/x/time
```

---

## Environment Variables (New)

| Variable | Required | Description |
|----------|----------|-------------|
| `DISCORD_BOT_TOKEN` | Yes | Bot token from Discord Developer Portal — static, no refresh |
| `DISCORD_CLIENT_ID` | Yes | Application client ID — used to generate "Add to Server" OAuth URL |
| `DISCORD_CLIENT_SECRET` | Yes | Application client secret — for exchanging OAuth2 callback code |
| `DISCORD_REDIRECT_URL` | Yes | OAuth2 callback URL, e.g. `https://app.example.com/api/v1/auth/discord/callback` |

---

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| Go Discord library | `bwmarrin/discordgo` | Raw gorilla/websocket + net/http | Discord's Gateway protocol is complex; discordgo handles reconnect, heartbeat, rate limits correctly. Building raw is weeks of protocol work with high maintenance risk. |
| Go Discord library | `bwmarrin/discordgo` | `diamondburned/arikawa` | arikawa is higher quality code but has a smaller community, less documentation, and fewer real-world examples. For a service that closely follows existing patterns, discordgo's larger ecosystem and wider usage reduce integration risk. |
| Service topology | Single discord-listener (inbound + outbound) | Separate discord-relay service | Relay is a single REST call. A separate service adds Kubernetes deployment overhead, inter-service latency, and operational complexity for trivial logic. |
| Bot auth | Bot Token (static) | Per-user OAuth access tokens | Discord's bot model is designed around static bot tokens. There is no per-user token needed for reading guild messages with a bot; the bot token IS the auth credential. |

---

## Sources

- Codebase: `services/twitch-listener/go.mod`, `services/kick-listener/go.mod`, `services/auth-service/go.mod`, `shared/go.mod` — all dependency versions verified (HIGH confidence)
- Codebase: `services/message-processor/models/message.go` — `RawChatMessage` schema verified (HIGH confidence)
- Codebase: `services/twitch-listener/publisher/stream_publisher.go` — Redis Streams publish pattern (`chat:raw`, XADD, MaxLen 1000000) verified (HIGH confidence)
- Codebase: `services/auth-service/oauth/platform.go`, `oauth/twitch.go` — `OAuthProvider` interface pattern verified (HIGH confidence)
- `github.com/bwmarrin/discordgo` — v0.28.1 latest as of August 2025 training cutoff; verify before pinning (MEDIUM confidence)
- Discord API v10 reference (training knowledge) — Gateway v10, intent bitmask values, bot scope model (MEDIUM confidence — Discord API versioning stable since 2022, v10 unchanged)
- `MESSAGE_CONTENT` privileged intent (HIGH confidence — enforced since April 2022, well-documented, no changes anticipated)
- Discord bot permission integers (HIGH confidence — standard bitmask, stable)
