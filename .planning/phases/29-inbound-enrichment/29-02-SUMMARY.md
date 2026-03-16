---
phase: 29-inbound-enrichment
plan: "02"
subsystem: messaging
tags: [discord, gateway, redis, mention-resolution, guild-cache, regex]

requires:
  - phase: 29-inbound-enrichment/29-01
    provides: MessageDeleteBulk dispatch, HandleMessageCreate with channel registry filter

provides:
  - GuildCache interface with SetChannelName/GetChannelName/DeleteChannelName/SetRoleName/GetRoleName/DeleteRoleName
  - HandleGuildCreate: populates channel and role name caches from GUILD_CREATE event
  - HandleChannelUpdate/HandleChannelDelete: keep channel name cache current
  - HandleGuildRoleUpdate/HandleGuildRoleDelete: keep role name cache current
  - ResolveMentions: replaces <@ID>, <@!ID>, <#ID>, <@&ID> tokens with human-readable names
  - redisGuildCache: Redis-backed implementation at discord:guild:channels: and discord:guild:roles: key prefixes
  - NewGatewayClient now accepts GuildCache as 7th parameter

affects:
  - 29-03 (if any future relay or enrichment needs guild context)
  - 30-relay (Discord relay uses same gateway client)

tech-stack:
  added: ["regexp (stdlib) — used for mention token regex substitution"]
  patterns:
    - "GuildCache interface in gateway package isolates Redis for unit testability"
    - "ResolveMentions exported package-level function (not method) — testable without GatewayClient"
    - "Cache handlers best-effort: errors logged at WARN, never halt Connect() loop"
    - "Mention resolution order: user first, then channel, then role (avoids ambiguity)"
    - "guildCache nil-guard in HandleMessageCreate enables graceful degradation when cache not wired"

key-files:
  created:
    - services/discord-listener/gateway/guild_cache_test.go
    - services/discord-listener/gateway/mention_test.go
  modified:
    - services/discord-listener/gateway/types.go
    - services/discord-listener/gateway/client.go
    - services/discord-listener/cmd/main.go
    - services/discord-listener/gateway/message_create_test.go
    - services/discord-listener/gateway/message_delete_test.go

key-decisions:
  - "ResolveMentions is a package-level exported function (not a method) so it can be unit-tested without constructing a GatewayClient"
  - "Cache population from GUILD_CREATE is best-effort: per-entry errors logged at WARN but do not halt Connect()"
  - "Mention resolution nil-guards guildCache: if cache is nil, ResolveMentions is skipped entirely (graceful degradation)"
  - "Existing tests updated to pass nil as GuildCache parameter to NewGatewayClient (backward-compatible nil behaviour)"

patterns-established:
  - "Exported handler methods (HandleGuildCreate etc.) follow same pattern as HandleMessageCreate — directly testable without exposing Connect()"
  - "All new dispatch cases in Connect() follow existing if-payload.T pattern rather than adding to outer switch"

requirements-completed: [INBD-04]

duration: 5min
completed: 2026-03-16
---

# Phase 29 Plan 02: GuildCache and Mention Resolution Summary

**GuildCache interface with Redis impl, GUILD_CREATE/CHANNEL_*/GUILD_ROLE_* dispatch handlers, and ResolveMentions replacing raw Snowflake mention tokens with human-readable names before publishing**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-03-16T07:44:22Z
- **Completed:** 2026-03-16T07:49:08Z
- **Tasks:** 2 (TDD: RED + GREEN)
- **Files modified:** 5 (+ 2 created)

## Accomplishments

- GuildCache interface defined in gateway package with 6 methods (SetChannelName, GetChannelName, DeleteChannelName, SetRoleName, GetRoleName, DeleteRoleName)
- GUILD_CREATE handler populates channel and role name caches; CHANNEL_*/GUILD_ROLE_* events keep them current
- ResolveMentions function resolves all Discord mention token types (<@ID>, <@!ID>, <#ID>, <@&ID>) with graceful fallbacks
- redisGuildCache wired in cmd/main.go using distinct key prefixes (discord:guild:channels:, discord:guild:roles:) separate from channel registry (discord:channels:)
- 15 new tests pass (6 cache + 9 mention) with 0 regressions across 29 total tests

## Task Commits

Each task was committed atomically:

1. **Task 1 (RED): Add guild types + GuildCache interface + write failing tests** - `232d73b` (test)
2. **Task 2 (GREEN): Implement GuildCache handlers + mention resolution + wire in cmd/main.go** - `99e0776` (feat)

**Plan metadata:** TBD (docs: complete plan)

_Note: TDD tasks had two commits (test → feat)_

## Files Created/Modified

- `services/discord-listener/gateway/types.go` - Added GuildCreateData, DiscordChannel, DiscordRole, ChannelUpdateData, GuildRoleUpdateData, GuildRoleDeleteData; added Mentions field to MessageCreateData
- `services/discord-listener/gateway/client.go` - Added GuildCache interface, guildCache field on GatewayClient, 5 handler methods, ResolveMentions function, dispatch wiring in Connect(), mention resolution call in HandleMessageCreate
- `services/discord-listener/cmd/main.go` - Added redisGuildCache type implementing all 6 GuildCache methods, instantiated and passed to NewGatewayClient
- `services/discord-listener/gateway/guild_cache_test.go` - 6 tests for cache population and maintenance (GUILD_CREATE, CHANNEL_UPDATE, CHANNEL_DELETE, GUILD_ROLE_UPDATE, GUILD_ROLE_DELETE)
- `services/discord-listener/gateway/mention_test.go` - 9 tests: 7 ResolveMentions unit tests + 1 HandleMessageCreate integration test
- `services/discord-listener/gateway/message_create_test.go` - Updated 4 calls to pass nil GuildCache to NewGatewayClient
- `services/discord-listener/gateway/message_delete_test.go` - Updated 5 calls to pass nil GuildCache to NewGatewayClient

## Decisions Made

- ResolveMentions exported as package-level function (not a method) so tests can call it without constructing a full GatewayClient
- guildCache nil-guard in HandleMessageCreate provides graceful degradation: if no cache wired, raw mentions pass through unchanged rather than crashing
- Cache population errors in HandleGuildCreate are logged at WARN but do not halt the Connect() loop — cache is best-effort
- Existing tests updated with nil GuildCache (backwards-compatible nil behaviour built in from the start)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

Minor: `replace_all` on the NewGatewayClient calls in message_create_test.go only replaced calls matching the exact 6-argument pattern on the first pass; two batches of replacements needed due to slight comment differences. Resolved cleanly.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- discord-listener gateway now resolves all mention types before publishing to Redis Streams
- Overlay renderer will receive human-readable names instead of raw Snowflake IDs for @user, #channel, @role mentions
- redisGuildCache uses distinct key namespace from channel registry (discord:guild: prefix), no collision risk
- All 29 tests pass, build and vet clean

## Self-Check: PASSED

All created files exist on disk. Both task commits (232d73b, 99e0776) verified in git log.

---
*Phase: 29-inbound-enrichment*
*Completed: 2026-03-16*
