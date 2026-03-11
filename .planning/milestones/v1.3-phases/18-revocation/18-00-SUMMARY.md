---
phase: 18-revocation
plan: "00"
subsystem: testing
tags: [go, tdd, share-service, revocation, red-test]

requires:
  - phase: 17-message-routing
    provides: shared_overlay routing infrastructure that revocation will gate
  - phase: 15-share-acceptance
    provides: ShareHandler struct and AcceptShareRequest handler pattern

provides:
  - RED test stubs for RevokeShareRequest handler (four test functions)
  - mockRevokeRepo asserting RevokeShare(shareID) is called on success

affects:
  - 18-01 (Wave 1 must turn these RED stubs GREEN)

tech-stack:
  added: []
  patterns:
    - "mockRevokeRepo struct pattern: records RevokeShare call with shareID for assertion in SourceDeactivation test"

key-files:
  created:
    - services/share-service/handlers/shares_revoke_test.go
  modified: []

key-decisions:
  - "Compile-error RED gate via handler.RevokeShareRequest (undefined on ShareHandler) satisfies Nyquist requirement without t.Skip"
  - "mockRevokeRepo is kept in test file only (not shared) — no production type created yet"
  - "Four test names match VALIDATION.md verification map: AuthCheck (403), StatusCheck (409), Success (200), SourceDeactivation (RevokeShare called)"

patterns-established:
  - "Revocation test pattern: gin.CreateTestContext + httptest.NewRecorder + c.Set(user_id) + c.Params, matching shares_accepted_test.go style"

requirements-completed:
  - SHARE-06
  - SHARE-07

duration: 1min
completed: 2026-03-10
---

# Phase 18 Plan 00: Revocation RED Test Stubs Summary

**Four compile-failing test stubs for `RevokeShareRequest` handler covering auth check (403), status check (409), success (200), and source deactivation assertion**

## Performance

- **Duration:** 1 min
- **Started:** 2026-03-10T19:00:53Z
- **Completed:** 2026-03-10T19:01:39Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments
- Created `shares_revoke_test.go` with four test stubs that compile-fail on the missing `RevokeShareRequest` method
- RED gate confirmed: `go test ./handlers/... -run TestRevokeShareRequest` exits with build failure citing `undefined (type *ShareHandler has no field or method RevokeShareRequest)` on all four call sites
- `mockRevokeRepo` struct establishes the RevokeShare(shareID) assertion pattern for Wave 1

## Task Commits

Each task was committed atomically:

1. **Task 1: Create shares_revoke_test.go with RED stubs** - `db63198` (test)

**Plan metadata:** (docs commit follows)

## Files Created/Modified
- `services/share-service/handlers/shares_revoke_test.go` - Four RED test stubs for revocation handler with mockRevokeRepo

## Decisions Made
- Compile-error RED gate (referencing undefined method) chosen over assertion-only RED — cleaner Nyquist compliance, no `t.Skip`
- `mockRevokeRepo` placed only in test file — no production interface created in Wave 0 (Wave 1 decides the interface shape)
- Test names kept verbatim from PLAN.md: `TestRevokeShareRequest_AuthCheck`, `TestRevokeShareRequest_StatusCheck`, `TestRevokeShareRequest_Success`, `TestRevokeShareRequest_SourceDeactivation`

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- RED gate in place: Wave 1 (18-01) can immediately start implementing `RevokeShareRequest` on `ShareHandler`
- `mockRevokeRepo.RevokeShare(shareID)` pattern ready for Wave 1 to satisfy with real repo call
- No blockers

## Self-Check: PASSED

- `services/share-service/handlers/shares_revoke_test.go` — FOUND
- `.planning/phases/18-revocation/18-00-SUMMARY.md` — FOUND
- Commit `db63198` — FOUND

---
*Phase: 18-revocation*
*Completed: 2026-03-10*
