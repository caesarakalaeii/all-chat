---
phase: 28-viewer-identity-foundation-auth-and-platform-linking
plan: "02"
subsystem: auth
tags: [jwt, oauth, twitch, youtube, kick, redis, postgresql, gin, go]

requires:
  - phase: 28-viewer-identity-foundation-auth-and-platform-linking-plan-01
    provides: ViewerIdentityRepository, ViewerClaims with viewer_id/display_name/avatar_url, viewer identity DB tables, RED test scaffolds

provides:
  - POST /viewer/{twitch,youtube,kick}/exchange handlers returning JSON (not redirect)
  - generateViewerJWT updated to accept viewerID and populate viewer_id/display_name/avatar_url in JWT
  - getOrCreateViewerWithIdentity called on all callback handlers (GET + POST)
  - PATCH /viewer/cosmetics handler with hex validation, DB upsert, Redis cache invalidation
  - auth-service cmd/main.go wired with ViewerIdentityRepository and all new routes
  - api-gateway proxy routes for all three exchange POST endpoints and cosmetics PATCH
  - JWT middleware updated to set viewer_id/display_name/avatar_url in gin context

affects: [phase-28-plan-03, phase-28-plan-04, viewer-badge-enricher, browser-extension]

tech-stack:
  added: []
  patterns:
    - "handlePatchCosmeticsLogic extracted to pure function accepting cosmeticsUpsertRepo interface for unit testing without DB"
    - "Exchange handlers follow POST JSON pattern (not GET redirect) for browser extension OAuth flow"
    - "Non-fatal identity creation: viewer_id = uuid.Nil on error, sign-in still succeeds"

key-files:
  created:
    - services/auth-service/handlers/viewer_exchange.go
    - services/auth-service/handlers/viewer_cosmetics.go
    - services/auth-service/handlers/viewer_cosmetics_test.go
  modified:
    - services/auth-service/handlers/viewer_auth.go
    - services/auth-service/handlers/viewer_exchange_test.go
    - services/auth-service/cmd/main.go
    - services/api-gateway/cmd/main.go
    - shared/middleware/auth.go

key-decisions:
  - "handlePatchCosmeticsLogic is a package-private function accepting a cosmeticsUpsertRepo interface to enable unit testing without requiring the concrete ViewerIdentityRepository"
  - "Pre-Phase-28 tokens (empty viewer_id) get 401 on cosmetics PATCH — no fallback lookup by session_id (would require new DB query on every cosmetics write)"
  - "viewer_id = uuid.Nil is used as non-fatal fallback when GetOrCreateViewerByPlatform fails during exchange/callback — sign-in succeeds but viewer_id claim is empty"
  - "JWT middleware (shared/middleware/auth.go) updated to set viewer_id, display_name, avatar_url in gin context from ViewerClaims"

patterns-established:
  - "POST exchange handler pattern: ShouldBindJSON → Redis state verify → provider exchange → session upsert → GetOrCreateViewerByPlatform → generateViewerJWT → JSON response"
  - "Cosmetics handler pattern: get viewer_id from context → uuid.Parse → hex regex validate → DB upsert → Redis cache invalidate → JSON response"

requirements-completed: [VID-03, VID-04, VID-05, VID-06]

duration: 25min
completed: 2026-03-14
---

# Phase 28 Plan 02: Viewer Auth Backend Summary

**POST exchange handlers for browser extension OAuth + PATCH cosmetics endpoint, with viewer_id embedded in all new JWTs via updated generateViewerJWT**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-03-14T15:51:00Z
- **Completed:** 2026-03-14T16:16:00Z
- **Tasks:** 2
- **Files modified:** 8

## Accomplishments

- Three POST exchange handlers (Twitch/YouTube/Kick) return JSON `{token, expires_in, viewer_info}` — no redirect, for browser extension use
- `generateViewerJWT` signature updated to accept `viewerID uuid.UUID`; populates `viewer_id`, `display_name`, `avatar_url` in JWT claims
- All existing GET callback handlers (HandleTwitchCallback, HandleYouTubeCallback, HandleKickCallback) now call `GetOrCreateViewerByPlatform` before generating JWT
- `PATCH /viewer/cosmetics` validates hex color (`^#[0-9a-fA-F]{6}$`), upserts `viewer_cosmetics` table, invalidates Redis cache key `viewer:identity:{platform}:{platform_user_id}`
- Pre-Phase-28 tokens with empty `viewer_id` claim return 401 from cosmetics endpoint (no fallback DB lookup)
- Auth-service and API gateway fully wired; all handler tests green

## Task Commits

1. **Task 1: POST exchange handlers + updated generateViewerJWT** - `b49978a16` (feat)
2. **Task 2: PATCH /viewer/cosmetics handler + route wiring** - `489f72e49` (feat)

## Files Created/Modified

- `services/auth-service/handlers/viewer_exchange.go` - Real exchange handler implementations (replaces stub file)
- `services/auth-service/handlers/viewer_cosmetics.go` - PATCH cosmetics handler + `handlePatchCosmeticsLogic` helper
- `services/auth-service/handlers/viewer_cosmetics_test.go` - 5 unit tests for cosmetics handler using mock repo
- `services/auth-service/handlers/viewer_auth.go` - Added `identityRepo` field, updated constructor, updated `generateViewerJWT`, updated callbacks
- `services/auth-service/handlers/viewer_exchange_test.go` - Updated with real GREEN tests + `TestGenerateViewerJWT_HasViewerID`
- `services/auth-service/cmd/main.go` - Instantiates `ViewerIdentityRepository`, wires it, registers all new routes
- `services/api-gateway/cmd/main.go` - Adds proxy routes for exchange + cosmetics endpoints
- `shared/middleware/auth.go` - Sets `viewer_id`, `display_name`, `avatar_url` in gin context from viewer JWT

## Decisions Made

- `handlePatchCosmeticsLogic` extracted as package-private function accepting `cosmeticsUpsertRepo` interface so tests can use mocks without needing the concrete `*repository.ViewerIdentityRepository`
- Empty `viewer_id` → 401 without fallback: adding a `GetViewerBySessionID` query would create unnecessary DB load during a brief migration window; re-authentication is the correct path
- JWT middleware extended to include the three new viewer claim fields for downstream handlers

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Updated shared JWT middleware to set viewer_id, display_name, avatar_url in context**
- **Found during:** Task 2 (PATCH cosmetics handler relies on `c.Get("viewer_id")`)
- **Issue:** The plan specified that cosmetics handler reads `viewer_id` from gin context, but the existing JWT middleware only set `session_id`, `username`, `platform`, `platform_user_id`, and `is_viewer` — not `viewer_id`
- **Fix:** Added `c.Set("viewer_id", ...)`, `c.Set("display_name", ...)`, `c.Set("avatar_url", ...)` in `JWTAuth` middleware for viewer tokens
- **Files modified:** `shared/middleware/auth.go`
- **Verification:** Cosmetics tests pass with the viewer_id set in context by the simulated middleware
- **Committed in:** `b49978a16` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 2 - missing critical functionality in middleware)
**Impact on plan:** Required for correct cosmetics handler operation. The middleware update is backward-compatible — it only adds new keys to context for viewer tokens.

## Issues Encountered

- The Wave-0 stub file `viewer_exchange_stubs.go` defined three methods on `ViewerAuthHandler` that were replaced by the real implementations in `viewer_exchange.go`. Deleted the stub file to avoid duplicate method definitions.
- `handlePatchCosmeticsLogic` references `c.Writer.Status()` to check success before Redis invalidation — works correctly because `c.JSON(200, ...)` sets the status code before returning.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 28 Plan 03 (ViewerBadgeEnricher) can now rely on `viewer_id` being present in the `viewer_platform_identities` table on every sign-in (both GET callbacks and POST exchange handlers populate it)
- Phase 28 Plan 04 (browser extension) can call the three POST exchange endpoints and the PATCH cosmetics endpoint
- All exchange/cosmetics routes are registered in both auth-service and API gateway

---
*Phase: 28-viewer-identity-foundation-auth-and-platform-linking*
*Completed: 2026-03-14*
