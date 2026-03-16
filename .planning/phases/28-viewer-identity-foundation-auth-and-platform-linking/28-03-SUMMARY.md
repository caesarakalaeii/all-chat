---
phase: 28-viewer-identity-foundation-auth-and-platform-linking
plan: "03"
subsystem: messaging
tags: [redis, postgres, enricher, viewer-identity, name-color, message-processor]

# Dependency graph
requires:
  - phase: 28-viewer-identity-foundation-auth-and-platform-linking-01
    provides: viewer_platform_identities and viewer_cosmetics DB tables
  - phase: 28-viewer-identity-foundation-auth-and-platform-linking-02
    provides: PATCH /viewer/cosmetics endpoint that invalidates viewer:identity: cache keys

provides:
  - ViewerBadgeEnricher struct with Enrich method (enricher/viewer_badge_enricher.go)
  - Redis cache layer for viewer identity (viewer:identity:{platform}:{user_id}, 5min TTL)
  - Null sentinel caching for unknown viewers to prevent thundering herd
  - name_color injection into msg.User.Color for registered All-Chat viewers
  - viewer_identity_enrichment stage metrics in both CHAT and EVENT paths

affects:
  - message-processor
  - frontend overlay rendering (name_color now populated for registered viewers)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - viewerDB interface + pgxPoolAdapter wraps *pgxpool.Pool for testability without pgxmock dependency
    - miniredis/v2 used for Redis mock in enricher tests (already in go.mod)
    - Null sentinel string "null" cached to prevent DB stampede for unknown platform users

key-files:
  created:
    - services/message-processor/enricher/viewer_badge_enricher.go
    - services/message-processor/enricher/viewer_badge_enricher_test.go
  modified:
    - services/message-processor/cmd/main.go

key-decisions:
  - "viewerDB interface abstracts pgxpool.Pool for unit testing without pgxmock; pgxPoolAdapter bridges the gap in main.go"
  - "Test double fakeViewerDB uses queryFn callback pattern matching existing enricher test conventions"
  - "Null sentinel prevents DB query for every chat message from non-registered viewers (majority of traffic)"

patterns-established:
  - "Enricher soft failure: return nil on all Redis/DB errors, log.Warn, continue message delivery"
  - "Cache key format viewer:identity:{platform}:{user_id} shared with plan 02 PATCH cosmetics invalidation"

requirements-completed:
  - VID-03

# Metrics
duration: 5min
completed: 2026-03-14
---

# Phase 28 Plan 03: ViewerBadgeEnricher Summary

**ViewerBadgeEnricher wired into message-processor enricher chain: Redis-cached (5min) viewer identity lookup with DB fallback injects name_color into chat messages for All-Chat registered viewers**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-03-14T16:04:00Z
- **Completed:** 2026-03-14T16:06:54Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- ViewerBadgeEnricher resolves (platform, user_id) -> viewer_id + name_color via Redis cache with DB fallback
- Null sentinel "null" cached for unregistered viewers, preventing DB stampede on high-traffic streams
- 7 unit tests cover all cache/DB scenarios using miniredis and fakeViewerDB callback pattern
- Wired into both EVENT PATH and CHAT PATH in message-processor/cmd/main.go after cheermote enrichment
- viewer_identity_enrichment Prometheus stage duration metric added to both paths

## Task Commits

Each task was committed atomically:

1. **Task 1: ViewerBadgeEnricher implementation + tests** - `282ad0dff` (feat)
2. **Task 2: Wire ViewerBadgeEnricher into message-processor cmd/main.go** - `2b9154860` (feat)

**Plan metadata:** (docs commit — to be added)

_Note: TDD tasks — tests written first, implementation confirmed all 7 GREEN in one pass_

## Files Created/Modified

- `services/message-processor/enricher/viewer_badge_enricher.go` - ViewerBadgeEnricher struct, viewerDB interface, pgxPoolAdapter, Redis cache/DB fallback logic
- `services/message-processor/enricher/viewer_badge_enricher_test.go` - 7 tests: CacheHit_WithColor, CacheHit_NoColor, NullSentinel, CacheMiss_ViewerFound, CacheMiss_ViewerNotFound, EmptyUserID, PlatformPreservesColor
- `services/message-processor/cmd/main.go` - Instantiation of viewerBadgeEnricher + call in EVENT PATH (after cheermote block) + call in CHAT PATH (after cheermote block)

## Decisions Made

- viewerDB interface + pgxPoolAdapter pattern used instead of importing pgxmock — avoids adding a new test dependency and keeps the enricher package self-contained for testing
- fakeViewerDB uses a queryFn callback pattern, consistent with how other enricher tests use httptest.Server callbacks
- Null sentinel uses the string literal "null" (not JSON null) — unambiguous, zero allocation, fast string comparison

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

Minor: `fakeViewerDB.QueryRow` had a compile error (multi-value return in single-value context) fixed immediately in the test double before running tests.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- ViewerBadgeEnricher is live in the enricher chain; any viewer with a name_color set in viewer_cosmetics will have it injected into their chat messages
- Cache key format `viewer:identity:{platform}:{user_id}` matches what plan 02 PATCH cosmetics handler invalidates — the invalidation path is complete end-to-end
- Phase 28 implementation complete: DB schema (01) + auth/exchange + cosmetics API (02) + enricher runtime (03)

---
*Phase: 28-viewer-identity-foundation-auth-and-platform-linking*
*Completed: 2026-03-14*
