---
phase: 32-integration-wiring-fixes
plan: "03"
subsystem: api
tags: [api-gateway, proxy, routing, cosmetics, frames, flairs]

# Dependency graph
requires:
  - phase: 30-avatar-frame-flair-system
    provides: auth-service frame/flair catalog and admin cosmetics endpoints

provides:
  - 2 public proxy routes for viewer cosmetics catalog (frames, flairs)
  - 6 protected proxy routes for admin cosmetics catalog management (GET/POST/DELETE frames and flairs)

affects: [frontend settings/viewer page, frontend admin/cosmetics page]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Dual-layer auth enforcement: gateway protectedAPI applies JWTAuth, auth-service admin group applies AdminOnly — non-admin JWTs pass gateway but get 403 from auth-service"
    - "Public catalog routes placed in publicAPI group (no JWT), not protectedAPI — consistent with catalog being read-only, non-sensitive data"

key-files:
  created: []
  modified:
    - services/api-gateway/cmd/main.go

key-decisions:
  - "Catalog routes (viewer/catalog/frames and viewer/catalog/flairs) placed in publicAPI group matching auth-service's public /viewer/catalog/* group — no JWT required"
  - "Admin cosmetics routes placed in protectedAPI with JWT-only enforcement at gateway; auth-service applies AdminOnly role check as second layer"

patterns-established: []

requirements-completed:
  - PREM-03
  - PREM-04
  - PREM-05
  - WEB-03
  - WEB-04

# Metrics
duration: 2min
completed: 2026-03-16
---

# Phase 32 Plan 03: Integration Wiring Fixes — Gateway Proxy Routes Summary

**8 missing proxy routes added to API gateway, unblocking frame/flair catalog pages and admin cosmetics management from permanent 404 responses**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-16T09:08:41Z
- **Completed:** 2026-03-16T09:10:30Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments

- Added 2 public catalog routes (`GET /auth/viewer/catalog/frames`, `GET /auth/viewer/catalog/flairs`) to `publicAPI` Gin group — no JWT required
- Added 6 admin cosmetics routes (`GET/POST/DELETE /admin/cosmetics/frames`, `GET/POST/DELETE /admin/cosmetics/flairs`) to `protectedAPI` Gin group — JWT required at gateway, admin role enforced at auth-service
- `go build ./...` from `services/api-gateway/` passes with no errors

## Task Commits

1. **Task 1: Register 8 proxy routes in API gateway** - `63add6a` (feat)

**Plan metadata:** (docs commit follows)

## Files Created/Modified

- `services/api-gateway/cmd/main.go` - Added 8 proxy route registrations (2 public catalog, 6 protected admin cosmetics)

## Decisions Made

- Catalog routes in `publicAPI` group — they are read-only non-sensitive catalog data matching auth-service's own public grouping
- Admin routes in `protectedAPI` group only — dual-layer enforcement follows existing `/admin/users` pattern; auth-service's `AdminOnly` middleware is the authoritative role check

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All 8 proxy routes are registered and the gateway will correctly forward requests to auth-service
- Frame/flair catalog pages (`/settings/viewer`) can now load catalog data
- Admin cosmetics page (`/admin/cosmetics`) can now manage the catalog
- PREM-03, PREM-04, PREM-05, WEB-03, WEB-04 requirements are satisfied

---
*Phase: 32-integration-wiring-fixes*
*Completed: 2026-03-16*
