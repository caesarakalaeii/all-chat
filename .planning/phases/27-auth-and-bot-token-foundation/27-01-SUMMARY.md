---
phase: 27-auth-and-bot-token-foundation
plan: 01
subsystem: database
tags: [postgres, discord, migration, overlay-manager]

# Dependency graph
requires: []
provides:
  - "discord_guilds table: UUID PK, user_id FK, guild_id VARCHAR(30), guild_name, guild_icon, connected_at, UNIQUE(user_id,guild_id)"
  - "Two indexes on discord_guilds: idx_discord_guilds_user_id, idx_discord_guilds_guild_id"
  - "GRANT ALL on discord_guilds to allchat_user (CloudNativePG compatibility)"
  - "overlay-manager validPlatforms includes discord:true — ChatSource.Validate() accepts platform='discord'"
affects: [27-03-auth-service-discord-repo, 27-discord-listener, overlay-manager]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Discord Snowflake IDs stored as VARCHAR(30) strings, not BIGINT — avoids JS safe-integer truncation above 2^53"
    - "CloudNativePG migration pattern: GRANT ALL after CREATE TABLE so allchat_user app role has access"

key-files:
  created:
    - migrations/035_discord_guilds.sql
  modified:
    - services/overlay-manager/models/chat_source.go
    - services/overlay-manager/models/chat_source_test.go

key-decisions:
  - "guild_id is VARCHAR(30) not BIGINT — Discord Snowflake IDs exceed JS safe-integer range (2^53), locked project decision"
  - "discord platform registered in overlay-manager validPlatforms as unblocking step for Plan 03 and discord-listener"

patterns-established:
  - "Migration pattern: include GRANT ALL ON <table> TO allchat_user for CloudNativePG"

requirements-completed:
  - AUTH-01
  - AUTH-04

# Metrics
duration: 2min
completed: 2026-03-15
---

# Phase 27 Plan 01: Auth and Bot Token Foundation Summary

**discord_guilds PostgreSQL migration (VARCHAR(30) guild_id, UNIQUE constraint, GRANT) and overlay-manager platform registration unblocking Plans 03 and discord-listener**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-15T20:58:02Z
- **Completed:** 2026-03-15T20:59:33Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Created `migrations/035_discord_guilds.sql` with correct schema: UUID PK, user_id FK with ON DELETE CASCADE, guild_id VARCHAR(30), UNIQUE(user_id, guild_id), two performance indexes, GRANT ALL to allchat_user
- Registered `"discord": true` in overlay-manager `validPlatforms` map — ChatSource.Validate() now accepts platform='discord'
- Updated test expectation for discord IsValidPlatform from false to true; all models tests pass

## Task Commits

Each task was committed atomically:

1. **Task 1: Create discord_guilds migration** - `2a43b84` (feat)
2. **Task 2: Register discord platform in overlay-manager** - `81f4ecf` (feat)

## Files Created/Modified
- `migrations/035_discord_guilds.sql` - discord_guilds table schema with indexes and GRANT statements
- `services/overlay-manager/models/chat_source.go` - Added "discord": true to validPlatforms map
- `services/overlay-manager/models/chat_source_test.go` - Updated discord IsValidPlatform expectation to true

## Decisions Made
- Followed plan exactly: guild_id as VARCHAR(30) per locked project decision (Snowflake IDs as strings)
- No additional model changes for Phase 27 beyond platform registration (Config JSONB handles Discord-specific fields)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed pre-existing test expectation for discord platform**
- **Found during:** Task 2 (Register discord platform in overlay-manager)
- **Issue:** `chat_source_test.go` had `{"discord", false}` — was written anticipating discord not yet registered; became a test failure after adding "discord": true
- **Fix:** Updated test case to `{"discord", true}` with Phase 27 comment
- **Files modified:** services/overlay-manager/models/chat_source_test.go
- **Verification:** `go test ./models/... -v -count=1` passes all subtests
- **Committed in:** `81f4ecf` (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - bug fix)
**Impact on plan:** Required correction; test was a placeholder anticipating this exact change.

## Issues Encountered
None beyond the pre-existing test expectation updated above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `migrations/035_discord_guilds.sql` is ready to apply; Plan 03 (auth-service discord repository) can reference discord_guilds table
- overlay-manager accepts platform='discord' for ChatSource.Validate() — discord-listener plan can create chat sources without validation errors
- No blockers for downstream plans in Phase 27

---
*Phase: 27-auth-and-bot-token-foundation*
*Completed: 2026-03-15*

## Self-Check: PASSED

- migrations/035_discord_guilds.sql: FOUND
- services/overlay-manager/models/chat_source.go: FOUND
- .planning/phases/27-auth-and-bot-token-foundation/27-01-SUMMARY.md: FOUND
- Commit 2a43b84 (Task 1): FOUND
- Commit 81f4ecf (Task 2): FOUND
