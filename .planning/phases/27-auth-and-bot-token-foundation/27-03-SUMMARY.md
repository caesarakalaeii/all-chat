---
phase: 27-auth-and-bot-token-foundation
plan: 03
subsystem: auth
tags: [discord, oauth, bot-token, permissions, postgresql, pgx, snowflake]

# Dependency graph
requires:
  - phase: 27-auth-and-bot-token-foundation
    provides: discord_guilds table migration (Plan 01), overlay-manager discord platform registration (Plan 02)
provides:
  - DiscordOAuth struct implementing OAuthProvider with GetAuthURL (bot invite URL), ExchangeCode, stubs for GetUserInfo/RefreshToken
  - ComputeMissingPermissions pure function for bit-level permission checking
  - CheckBotPermissions method using Bot token to call /guilds/{id}/members/@me
  - DiscordGuild model with GuildID as string (Snowflake-safe)
  - DiscordRepository with UpsertGuild, DeleteGuild, ListGuildsByUser, GetGuild, DeleteDiscordSourcesByGuildID
  - ErrNotFound sentinel in repository/errors.go
affects:
  - 27-04 (handlers + routes — depends directly on DiscordOAuth and DiscordRepository types)
  - 27-05 (discord-listener — may use DiscordRepository for guild registry)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Discord bot auth is not user OAuth: scope=bot, guild_id in callback, no per-user token"
    - "PermissionBits: uint64 bitmask with ComputeMissingPermissions exported for unit testability"
    - "Snowflake IDs as string always — never int64 or uint64"
    - "WithBotToken builder method keeps constructor signature clean"

key-files:
  created:
    - services/auth-service/oauth/discord.go
    - services/auth-service/oauth/discord_test.go
    - services/auth-service/models/discord_guild.go
    - services/auth-service/repository/discord_repo.go
  modified:
    - services/auth-service/oauth/platform.go
    - services/auth-service/repository/errors.go

key-decisions:
  - "ComputeMissingPermissions is exported (not package-private) to enable pure unit tests without HTTP mocking"
  - "GetUserInfo returns error stub — Discord bot auth has no user identity; handler bypasses this method"
  - "ErrNotFound added to errors.go as general sentinel (distinct from ErrUserNotFound)"

patterns-established:
  - "Discord OAuth pattern: GetAuthURL → guild picker → callback with guild_id (not user tokens)"
  - "Repository pattern: pgxpool.Pool injected, no cipher needed (no encrypted tokens)"

requirements-completed: [AUTH-01, AUTH-02, AUTH-03, AUTH-04]

# Metrics
duration: 12min
completed: 2026-03-15
---

# Phase 27 Plan 03: Discord OAuth Provider and Guild Repository Summary

**DiscordOAuth implementing OAuthProvider with bot-invite URL generation, permission bit checking via CheckBotPermissions, and DiscordRepository covering full guild lifecycle including cross-service source cleanup**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-03-15T21:10:00Z
- **Completed:** 2026-03-15T21:22:00Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- DiscordOAuth satisfies the OAuthProvider interface with correct bot invite URL (scope=bot, permissions=68608) and a documented stub pattern for GetUserInfo/RefreshToken
- ComputeMissingPermissions pure function maps permission bits to human-readable names; 6 unit tests covering all branches pass
- DiscordRepository provides full guild lifecycle: UpsertGuild (ON CONFLICT upsert), DeleteGuild (idempotent), ListGuildsByUser, GetGuild, DeleteDiscordSourcesByGuildID
- GuildID typed as string throughout — Snowflake IDs stored and transmitted safely

## Task Commits

Each task was committed atomically:

1. **Task 1: Discord OAuth provider (TDD RED + GREEN)** - `11566fa` (feat)
2. **Task 2: DiscordGuild model and DiscordRepository** - `bab4699` (feat)

## Files Created/Modified
- `services/auth-service/oauth/platform.go` - Added PlatformDiscord constant
- `services/auth-service/oauth/discord.go` - DiscordOAuth struct, CheckBotPermissions, ComputeMissingPermissions
- `services/auth-service/oauth/discord_test.go` - 6 unit tests (GetAuthURL, GetPlatform, GetUserInfo stub, permission checks)
- `services/auth-service/models/discord_guild.go` - DiscordGuild struct with string GuildID
- `services/auth-service/repository/discord_repo.go` - DiscordRepository with 5 methods
- `services/auth-service/repository/errors.go` - Added ErrNotFound sentinel

## Decisions Made
- ComputeMissingPermissions exported for testability — avoids needing to mock HTTP calls just to test bit logic
- GetUserInfo intentionally returns an error (not nil, not a dummy struct) to make misuse visible at runtime
- ErrNotFound added as a general sentinel distinct from the existing ErrUserNotFound

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Added ErrNotFound to repository/errors.go**
- **Found during:** Task 2 (DiscordRepository implementation)
- **Issue:** Plan documented that ErrNotFound should be used in GetGuild, but errors.go only had ErrUserNotFound
- **Fix:** Added `var ErrNotFound = errors.New("not found")` to errors.go
- **Files modified:** services/auth-service/repository/errors.go
- **Verification:** go build ./... succeeds; GetGuild compiles referencing ErrNotFound
- **Committed in:** bab4699 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 missing critical)
**Impact on plan:** Essential addition required for correct compilation. No scope creep.

## Issues Encountered
None — build and tests passed on first attempt.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Plan 04 (HTTP handlers + routes) can proceed immediately — DiscordOAuth and DiscordRepository types are defined and tested
- DiscordRepository integration tests will require the discord_guilds migration from Plan 01 (deferred to verify-work phase)
- No blockers

## Self-Check: PASSED

All created files confirmed present on disk. All task commits (11566fa, bab4699) confirmed in git log.

---
*Phase: 27-auth-and-bot-token-foundation*
*Completed: 2026-03-15*
