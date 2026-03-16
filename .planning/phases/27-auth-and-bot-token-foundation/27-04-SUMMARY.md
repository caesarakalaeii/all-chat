---
phase: 27-auth-and-bot-token-foundation
plan: 04
subsystem: auth
tags: [discord, oauth, handlers, gin, redis, csrf, permissions, guild-management]

# Dependency graph
requires:
  - phase: 27-auth-and-bot-token-foundation
    provides: DiscordOAuth (Plan 03), DiscordRepository (Plan 03), DiscordGuild model (Plan 03)
provides:
  - DiscordHandler with 5 HTTP endpoints (HandleConnect, HandleCallback, HandleGetGuilds, HandleGetGuildChannels, HandleDisconnect)
  - DiscordOAuthProvider interface for testability
  - DiscordGuildRepo interface for testability
  - stateStorer interface with memStateStore (tests) and redisStateStore (production)
  - Discord routes registered in auth-service main.go
affects:
  - 27-05 (discord-listener — auth-service now serves guild data APIs)
  - Frontend (can call /api/v1/auth/discord/connect, /api/v1/auth/guilds)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "stateStorer interface abstracts Redis for handler unit tests — no miniredis required"
    - "memStateStore in production code (not _test.go) enables direct injection in tests"
    - "Best-effort pattern: Discord Leave Guild API failure logged, never prevents local DB cleanup"
    - "HandleCallback guards: never trust permissions query param — always re-fetch via CheckBotPermissions"
    - "Missing bot permissions returns 403 Forbidden (not 400 Bad Request)"

key-files:
  created:
    - services/auth-service/handlers/discord.go
    - services/auth-service/handlers/discord_test.go
  modified:
    - services/auth-service/cmd/main.go

key-decisions:
  - "stateStorer interface placed in discord.go (not _test.go) so newTestDiscordHandlerNoRedis can use memStateStore without test-file visibility limitation"
  - "fakeStateStore in discord_test.go is a type alias for memStateStore — avoids duplicating interface implementation"
  - "discordAPIBase field is overridable on DiscordHandler struct — allows httptest.Server injection in channel and disconnect tests without interface extraction"
  - "Discord routes only registered when all three env vars present — graceful degradation via WARN log"

patterns-established:
  - "Handler test pattern: newTestDiscordHandlerNoRedis + stateStore injection replaces miniredis dependency"
  - "httpClient overridable via discordAPIBase field for REST call mocking"

requirements-completed: [AUTH-01, AUTH-02, AUTH-03, AUTH-04]

# Metrics
duration: 9min
completed: 2026-03-15
---

# Phase 27 Plan 04: Discord HTTP Endpoints and Route Registration Summary

**DiscordHandler wiring 5 endpoints into auth-service with CSRF state management, permission gating, best-effort disconnect, and full unit test coverage using interface injection**

## Performance

- **Duration:** ~9 min
- **Started:** 2026-03-15T21:16:20Z
- **Completed:** 2026-03-15T21:24:58Z
- **Tasks:** 3 (1a TDD RED, 1b TDD GREEN, 2 route wiring)
- **Files modified:** 3

## Accomplishments

- DiscordOAuthProvider and DiscordGuildRepo interfaces defined in discord.go for full handler testability
- stateStorer interface with redisStateStore (production) and memStateStore (tests) eliminates miniredis dependency
- HandleConnect: generates CSRF state via stateStore, returns `{"bot_invite_url": "..."}` with scope=bot
- HandleCallback: validates state, exchanges code (audit only), re-checks permissions via CheckBotPermissions (403 + named missing perms on failure), UpsertGuild + redirect on success
- HandleGetGuilds: returns JSON array of connected guilds (empty array, not null, when none)
- HandleGetGuildChannels: fetches Discord channels API, filters to type=0 only, groups by parent category, orphans land in "Uncategorized"
- HandleDisconnect: best-effort Leave Guild REST call (500/error logged, not fatal), always calls DeleteGuild + DeleteDiscordSourcesByGuildID
- All 6 unit tests pass; build and vet clean
- auth-service main.go: reads DISCORD_CLIENT_ID/SECRET/BOT_TOKEN, WARN (not fatal) when absent, all 5 routes registered

## Task Commits

Each task was committed atomically:

1. **Task 1a+1b: DiscordHandler TDD RED+GREEN** - `26a0b6e` (feat)
2. **Task 2: Register Discord routes in main.go** - `c83db78` (feat)

## Files Created/Modified

- `services/auth-service/handlers/discord.go` — DiscordHandler, interfaces, stateStorer, memStateStore
- `services/auth-service/handlers/discord_test.go` — 6 unit tests, mock implementations
- `services/auth-service/cmd/main.go` — env var reads, handler init, 5 route registrations

## Decisions Made

- stateStorer interface in discord.go (not _test.go) — avoids compilation issue where test file types can't be referenced by non-test production functions like `newTestDiscordHandlerNoRedis`
- fakeStateStore in tests is a type alias `= memStateStore` — reuses the interface implementation without duplication
- discordAPIBase as an overridable string field on DiscordHandler — allows httptest.Server injection in channel list and disconnect tests without extracting an HTTP client interface
- Discord routes conditionally registered only when all three Discord env vars are set — consistent with existing YouTube/Kick graceful degradation pattern

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] ExchangeCode interface signature corrected**
- **Found during:** Task 1a (writing test mocks)
- **Issue:** Plan's interface spec showed `ExchangeCode` returning `(accessToken string, err error)`, but the actual `DiscordOAuth.ExchangeCode` (Plan 03) returns `(*oauth2.Token, error)`
- **Fix:** Interface defined as `ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error)` to match the concrete implementation — the token is discarded in HandleCallback (audit-only per spec)
- **Files modified:** services/auth-service/handlers/discord.go
- **Commit:** 26a0b6e

**2. [Rule 3 - Blocking] stateStorer moved from _test.go to discord.go**
- **Found during:** Task 1b (compile error)
- **Issue:** `newTestDiscordHandlerNoRedis` in discord.go referenced `fakeStateStore` defined in discord_test.go; non-test Go files cannot reference symbols in _test.go files
- **Fix:** Defined `memStateStore` and `stateStorer` interface in discord.go; `fakeStateStore` in discord_test.go becomes a type alias `= memStateStore`
- **Files modified:** services/auth-service/handlers/discord.go, services/auth-service/handlers/discord_test.go
- **Commit:** 26a0b6e

---

**Total deviations:** 2 auto-fixed (1 type correction, 1 blocking compile fix)
**Impact on plan:** No scope creep. Both fixes were essential for correct compilation and interface compatibility.

## Pre-existing Test Failures (out of scope)

The full `go test ./...` run shows pre-existing failures requiring external infrastructure:
- `TestAuthHandlerLogout` — requires Redis (connection refused on localhost:6379)
- `TestUserRepository_*` — requires PostgreSQL with current migrations (`column "is_premium" does not exist`)

These failures pre-date this plan and are unrelated to the Discord work.

## Next Phase Readiness

- Phase 27 complete — all 4 AUTH requirements fulfilled
- Phase 28 (discord-listener) can proceed: auth-service now exposes guild APIs needed by the listener
- No blockers

## Self-Check: PASSED
