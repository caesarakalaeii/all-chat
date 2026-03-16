---
phase: 30-avatar-frame-flair-system
plan: "03"
subsystem: message-processor
tags: [go, redis, postgres, enricher, avatar-frame, flair, cosmetics]

# Dependency graph
requires:
  - phase: 30-avatar-frame-flair-system
    plan: "01"
    provides: "DB schema (cosmetic_frames, cosmetic_flairs tables) and UserInfo.AvatarFrameURL/AvatarFlairURL type fields"
  - phase: 29-viewer-color-gradient-editor
    plan: "03"
    provides: "ViewerBadgeEnricher with viewerDB interface, pgxPoolAdapter, and name_gradient enrichment pattern"
provides:
  - "ViewerBadgeEnricher resolves and injects avatar_frame_url and avatar_flair_url from DB + Redis cache"
  - "Extended viewerIdentityCache struct with AvatarFrameURL and AvatarFlairURL fields"
  - "5-column DB query with LEFT JOIN cosmetic_frames and LEFT JOIN cosmetic_flairs"
affects:
  - overlay-frontend
  - plan-30-04

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "COALESCE(cf.image_url, '') pattern ensures string scan never receives NULL"
    - "TDD RED-GREEN on enricher extension: test mock extended before production code"

key-files:
  created: []
  modified:
    - services/message-processor/enricher/viewer_badge_enricher.go
    - services/message-processor/enricher/viewer_badge_enricher_test.go

key-decisions:
  - "COALESCE guarantees non-NULL scan into plain string (not *string) — consistent with name_gradient nil-guard pattern"
  - "fakeViewerDB queryFn extended to 6-return signature; noGradientDB helper updated accordingly — all existing tests pass unchanged"

patterns-established:
  - "TDD RED commit precedes GREEN implementation commit for enricher extension work"

requirements-completed:
  - PREM-03
  - PREM-04

# Metrics
duration: ~3min
completed: 2026-03-16
---

# Phase 30 Plan 03: ViewerBadgeEnricher Frame/Flair URL Injection Summary

**ViewerBadgeEnricher extended with 5-column DB query joining cosmetic_frames and cosmetic_flairs, injecting resolved image URLs into msg.User.AvatarFrameURL and msg.User.AvatarFlairURL via DB and Redis cache paths**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-03-15T23:53:14Z
- **Completed:** 2026-03-15T23:56:00Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments
- Extended `viewerIdentityCache` struct with `AvatarFrameURL` and `AvatarFlairURL` string fields
- DB query extended with `LEFT JOIN cosmetic_frames cf ON cf.id = vc.avatar_frame_id` and `LEFT JOIN cosmetic_flairs cfl ON cfl.id = vc.avatar_flair_id`, using COALESCE for null-safe string scan
- Frame/flair URLs injected in both DB-hit path (step 6) and Redis cache-hit path
- 3 new test cases added covering DB path with frame, zero-value guard, and cache-hit path; all 17 enricher tests pass green

## Task Commits

Each task was committed atomically:

1. **TDD RED: failing tests** - `48719f5` (test)
2. **TDD GREEN: implementation** - `be80869` (feat)

_Note: TDD task committed in RED + GREEN steps._

## Files Created/Modified
- `services/message-processor/enricher/viewer_badge_enricher.go` - Extended cache struct, DB query (5-column with LEFT JOINs), scan vars, cache build, and injection steps for frame/flair URLs
- `services/message-processor/enricher/viewer_badge_enricher_test.go` - Extended fakeViewerDB to 6-return queryFn; added TestEnrichWithAvatarFrameURL, TestEnrichWithNoFrameOrFlair, TestEnrichCacheHitWithFrameURL

## Decisions Made
- COALESCE guarantees non-NULL scan into plain `string` (not `*string`), consistent with the COALESCE pattern used for other nullable fields in the query
- `fakeViewerDB` queryFn extended to 6-return signature; `noGradientDB` helper updated to return `"", ""` for frame/flair so all existing tests compile and pass unchanged

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Frame/flair URLs now flow through the message pipeline: DB -> enricher -> UnifiedChatMessage -> WebSocket -> overlay frontend
- Plan 30-04 (frontend overlay rendering) can consume `msg.user.avatar_frame_url` and `msg.user.avatar_flair_url` from the WebSocket payload

---
*Phase: 30-avatar-frame-flair-system*
*Completed: 2026-03-16*
