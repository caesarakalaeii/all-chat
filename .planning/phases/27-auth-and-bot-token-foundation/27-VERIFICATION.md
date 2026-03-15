---
phase: 27-auth-and-bot-token-foundation
verified: 2026-03-15T21:31:03Z
status: passed
score: 14/14 must-haves verified
---

# Phase 27: Auth and Bot Token Foundation Verification Report

**Phase Goal:** Establish the authentication and bot token foundation for Discord integration — database layer, discord-listener service scaffold, OAuth flow, guild tracking, and HTTP endpoints.
**Verified:** 2026-03-15T21:31:03Z
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| #  | Truth | Status | Evidence |
|----|-------|--------|----------|
| 1  | Connected guild data stored in discord_guilds (auth-service repository can persist guild records) | VERIFIED | `migrations/035_discord_guilds.sql` exists with UUID PK, user_id FK, guild_id VARCHAR(30), UNIQUE constraint, two indexes, GRANT ALL |
| 2  | Discord chat sources can be added to overlays via platform validation layer | VERIFIED | `services/overlay-manager/models/chat_source.go` line 31: `"discord": true` |
| 3  | discord-listener service compiles and starts with Gin health endpoints | VERIFIED | `go build ./...` clean; `/health/live` and `/health/ready` registered; Dockerfile present |
| 4  | Gateway IDENTIFY payload uses intents bitmask 33281 (GUILDS|GUILD_MESSAGES|MESSAGE_CONTENT) | VERIFIED | `types.go` RequiredIntents = IntentGuilds(1) | IntentGuildMessages(512) | IntentMessageContent(32768) = 33281; `TestGatewayTypes_IntentBitmask` PASS |
| 5  | On READY event, session_id and resume_gateway_url stored in Redis under defined key schema | VERIFIED | `client.go` HandleReady writes to `discord:gateway:shard:0:session_id` and `discord:gateway:shard:0:resume_url`; `TestGatewayClient_ReadyHandler` PASS |
| 6  | On READY event, a WARN log reminds operator to verify MESSAGE_CONTENT in Developer Portal | VERIFIED | `client.go` line 149: `c.log.Warn("MESSAGE_CONTENT is a privileged Gateway intent...")` |
| 7  | GetAuthURL returns a bot invite URL with scope=bot and permissions=68608 | VERIFIED | `oauth/discord.go` GetAuthURL sets scope=bot, permissions=68608; `TestDiscordOAuth_GetAuthURL` PASS |
| 8  | ExchangeCode calls the Discord token endpoint and returns an oauth2.Token | VERIFIED | `oauth/discord.go` ExchangeCode POSTs to discord token URL, returns `*oauth2.Token`; `TestDiscordOAuth_GetPlatform` and `TestDiscordOAuth_GetUserInfo_ReturnsError` PASS |
| 9  | Permission check returns named missing permissions when bits are absent | VERIFIED | `ComputeMissingPermissions` exported; `TestCheckBotPermissions_AllGranted`, `TestCheckBotPermissions_MissingViewChannel`, `TestCheckBotPermissions_MissingMultiple` all PASS |
| 10 | DiscordRepository can UpsertGuild, DeleteGuild, ListGuildsByUser, and DeleteDiscordSourcesByGuildID | VERIFIED | `repository/discord_repo.go` all four methods present, targeting discord_guilds and overlay_chat_sources tables |
| 11 | GET /discord/connect returns a bot invite URL for authenticated users | VERIFIED | `handlers/discord.go` HandleConnect registered at line 308 in main.go; `TestHandleDiscordConnect` PASS |
| 12 | GET /discord/callback blocks guild save and returns 403 with named missing permissions | VERIFIED | `handlers/discord.go` line 220: `c.JSON(http.StatusForbidden, ...)` with "Bot is missing: ..." message; `TestHandleDiscordConnect_MissingPerms` PASS |
| 13 | GET /guilds and GET /guilds/:guild_id/channels return expected data | VERIFIED | Both handlers present and registered; `TestHandleGetGuilds` and `TestHandleGetGuildChannels` PASS |
| 14 | DELETE /guilds/:guild_id always deletes local records regardless of Leave Guild API result | VERIFIED | `handlers/discord.go` lines 487 and 497: DeleteGuild and DeleteDiscordSourcesByGuildID called after best-effort API call; `TestHandleDiscordDisconnect_APIFailure` PASS |

**Score:** 14/14 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `migrations/035_discord_guilds.sql` | discord_guilds table with VARCHAR(30) guild_id, UNIQUE constraint, indexes, GRANT | VERIFIED | Exact schema present; GRANT ALL to allchat_user included |
| `services/overlay-manager/models/chat_source.go` | validPlatforms map includes "discord": true | VERIFIED | Line 31 confirmed |
| `services/discord-listener/gateway/types.go` | GatewayPayload, intent consts, op code consts, GatewaySession, RequiredIntents | VERIFIED | All constants and types present; RequiredIntents = 33281 |
| `services/discord-listener/gateway/client.go` | GatewayClient with NewGatewayClient, Connect, Close; BuildIdentifyPayload; HandleReady | VERIFIED | All exported functions present; SessionStore interface for testability |
| `services/discord-listener/gateway/client_test.go` | 4 unit tests covering bitmask, op codes, identify payload, READY handler | VERIFIED | All 4 tests PASS |
| `services/discord-listener/cmd/main.go` | Service entry point with gin.New(), Redis, health HTTP, gateway goroutine | VERIFIED | gin.New() at line 75; gateway.NewGatewayClient wired at line 47 |
| `services/discord-listener/Dockerfile` | Multi-stage Go build container | VERIFIED | File exists with golang:1.23-alpine builder stage |
| `services/auth-service/oauth/discord.go` | DiscordOAuth implementing OAuthProvider; GetAuthURL, ExchangeCode, CheckBotPermissions, ComputeMissingPermissions | VERIFIED | All methods present; PlatformDiscord constant in platform.go |
| `services/auth-service/oauth/discord_test.go` | 6 tests: GetAuthURL, GetPlatform, GetUserInfo stub, 3 permission tests | VERIFIED | All 6 tests PASS |
| `services/auth-service/repository/discord_repo.go` | DiscordRepository with 5 methods; queries target discord_guilds and overlay_chat_sources | VERIFIED | All methods present; DeleteDiscordSourcesByGuildID deletes from overlay_chat_sources |
| `services/auth-service/models/discord_guild.go` | DiscordGuild model with GuildID as string | VERIFIED | GuildID typed as string with comment explaining Snowflake ID rationale |
| `services/auth-service/handlers/discord.go` | DiscordHandler with 5 handler methods; DiscordOAuthProvider and DiscordGuildRepo interfaces | VERIFIED | All 5 handlers present; interfaces defined for testability |
| `services/auth-service/handlers/discord_test.go` | 6 unit tests for all 5 endpoints | VERIFIED | All 6 handler tests PASS |
| `services/auth-service/cmd/main.go` | Discord routes registered; DISCORD_BOT_TOKEN env var; WARN when absent | VERIFIED | All 5 routes registered at lines 268-311; WARN at line 101 |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `discord_repo.go` | `migrations/035_discord_guilds.sql` | SQL queries target discord_guilds table | VERIFIED | INSERT/SELECT/DELETE all reference discord_guilds |
| `overlay-manager/models/chat_source.go` | `overlay_chat_sources` | validPlatforms map gates ChatSource.Validate() | VERIFIED | "discord": true at line 31 |
| `gateway/client.go` | Redis | READY handler writes session_id + resume_gateway_url | VERIFIED | HandleReady calls store.Set with RedisKeySessionID and RedisKeyResumeURL |
| `cmd/main.go` (discord-listener) | `gateway/client.go` | main starts gateway.Connect() in goroutine | VERIFIED | gateway.NewGatewayClient at line 47; goroutine at line 53 |
| `handlers/discord.go` | `oauth/discord.go` | DiscordHandler calls CheckBotPermissions and ExchangeCode | VERIFIED | CheckBotPermissions called at line 209; ExchangeCode called in HandleCallback |
| `handlers/discord.go` | `repository/discord_repo.go` | HandleConnect calls UpsertGuild; HandleDisconnect calls DeleteGuild + DeleteDiscordSourcesByGuildID | VERIFIED | repo.UpsertGuild at line 232; DeleteGuild at line 487; DeleteDiscordSourcesByGuildID at line 497 |
| `auth-service/cmd/main.go` | `handlers/discord.go` | Route registration with JWT-protected and public groups | VERIFIED | discordHandler wired at line 176; 5 routes registered at lines 268-311 |

---

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|-------------|-------------|--------|---------|
| AUTH-01 | 27-01, 27-02, 27-03, 27-04 | User can connect a Discord server via OAuth2 "Add to Server" flow | SATISFIED | GetAuthURL returns bot invite URL with scope=bot; HandleCallback stores guild via UpsertGuild; handler tests cover full flow |
| AUTH-02 | 27-02, 27-03, 27-04 | After connecting, user can view a list of readable text channels | SATISFIED | HandleGetGuildChannels fetches Discord channels API, filters type=0, groups by category; TestHandleGetGuildChannels PASS |
| AUTH-03 | 27-03, 27-04 | Bot permissions validated on connect with user-visible errors on failure | SATISFIED | CheckBotPermissions re-fetches via API; 403 + "Bot is missing: X, Y" returned; UpsertGuild NOT called on failure |
| AUTH-04 | 27-01, 27-03, 27-04 | User can disconnect bot, removing all associated Discord sources | SATISFIED | HandleDisconnect: best-effort Leave Guild API, always calls DeleteGuild + DeleteDiscordSourcesByGuildID; test verifies DB cleanup despite API failure |

No orphaned requirements — all four AUTH requirements declared in plan frontmatter and confirmed satisfied.

---

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `services/discord-listener/gateway/client.go` | 148 | `// TODO(Phase 28): halt if first MESSAGE_CREATE has empty content` | Info | Intentional — Phase 28 future work; does not block Phase 27 goal |

No blockers. The single TODO is explicitly scoped to a future phase and documents a known limitation of the Phase 27 scope.

---

### Human Verification Required

None. All observable truths for Phase 27 are verifiable programmatically:

- Database migration schema — confirmed by file inspection
- Compilation — confirmed by `go build ./...`
- Unit test pass/fail — confirmed by `go test` runs
- Route registration — confirmed by grep on main.go
- Intent bitmask arithmetic — confirmed by test assertions

The only behavior requiring live infrastructure (real Discord Gateway connection, actual OAuth callback flow with a live bot) is explicitly deferred to integration testing and is outside Phase 27's verification scope. The unit tests cover all handler logic with mocks.

---

## Gaps Summary

No gaps. All 14 observable truths verified. All artifacts substantive and wired. All four AUTH requirements satisfied by implementation evidence in the codebase.

**Test results summary:**
- `services/discord-listener/gateway/...`: 4/4 PASS
- `services/auth-service/oauth/...` (Discord): 6/6 PASS
- `services/auth-service/handlers/...` (Discord): 6/6 PASS
- Build: `go build ./...` clean on both services

---

_Verified: 2026-03-15T21:31:03Z_
_Verifier: Claude (gsd-verifier)_
