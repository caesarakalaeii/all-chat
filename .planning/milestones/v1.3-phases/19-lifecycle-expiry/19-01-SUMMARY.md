---
phase: 19-lifecycle-expiry
plan: "01"
subsystem: api
tags: [share-service, expiry, postgres, go, tdd]

# Dependency graph
requires:
  - phase: 19-lifecycle-expiry-00
    provides: Migration 034 with expiry_option and share_expires_at columns on share_requests
provides:
  - ExpiryOption and ShareExpiresAt fields on ShareRequest struct
  - AcceptShareRequest persists expiry_option and share_expires_at in UPDATE query
  - ExpireAcceptedShare transactional single-share expiry method on ShareRepository
  - ExpireTimedAcceptedShares batch expiry method on ShareRepository (queries custom shares past deadline)
  - ExpiryJob extended to call both ExpirePendingRequests and ExpireTimedAcceptedShares each tick
  - LifecycleSubscriber stub (StreamEndEvent, NewLifecycleSubscriber) for Wave 2 RED gate
affects:
  - 19-lifecycle-expiry-02 (lifecycle subscriber Wave 2 — uses LifecycleSubscriber stub and ExpireAcceptedShare)
  - 19-lifecycle-expiry-03 (frontend expiry UI — reads expiry_option from ShareRequest JSON)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - ExpireAcceptedShare mirrors RevokeShareRequest transactional pattern (update share_requests + overlay_chat_sources atomically)
    - ExpireTimedAcceptedShares: query IDs first, then loop with per-share transaction (avoids long-running transaction)
    - ExpiryJob: both expiry paths (pending requests + timed accepted shares) run in single expireOldRequests call each tick

key-files:
  created:
    - services/share-service/jobs/lifecycle_subscriber.go
  modified:
    - services/share-service/models/share_request.go
    - services/share-service/handlers/shares.go
    - services/share-service/repository/share_repo.go
    - services/share-service/jobs/expiry.go
    - services/share-service/jobs/expiry_test.go

key-decisions:
  - "AcceptShareRequest sets shareExpiresAt = NOW()+N hours only for custom option; unlimited and this_stream leave it NULL"
  - "ExpireAcceptedShare is idempotent: RowsAffected()==0 returns nil (already expired or not found)"
  - "ExpireTimedAcceptedShares queries share IDs first (rows.Close before loop), then calls ExpireAcceptedShare per share to avoid holding open cursor during transactions"
  - "LifecycleSubscriber stub created in Wave 1 to unblock jobs package compilation for Wave 2 RED gate"

patterns-established:
  - "Transactional expiry pattern: UPDATE share_requests SET status=expired + UPDATE overlay_chat_sources SET is_active=false in single tx"

requirements-completed: [EXPIRY-01, EXPIRY-04]

# Metrics
duration: 2min
completed: 2026-03-11
---

# Phase 19 Plan 01: Expiry Option Persistence + Batch Expiry Job Summary

**AcceptShareRequest now persists expiry_option and share_expires_at; ExpiryJob extended with ExpireTimedAcceptedShares to atomically expire custom-timed accepted shares**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-11T17:48:07Z
- **Completed:** 2026-03-11T17:50:10Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments

- ShareRequest struct has ExpiryOption and ShareExpiresAt fields; AcceptShareRequest UPDATE query persists them for unlimited/custom/this_stream options
- ExpireAcceptedShare atomically transitions share to 'expired' and deactivates overlay_chat_sources, mirroring RevokeShareRequest pattern
- ExpireTimedAcceptedShares batch method finds all accepted custom shares past their deadline and expires each atomically
- ExpiryJob calls both ExpirePendingRequests and ExpireTimedAcceptedShares each 5-minute tick
- LifecycleSubscriber stub created to allow jobs package to compile (Wave 2 RED gate for stream lifecycle events)

## Task Commits

1. **Task 1: Extend ShareRequest model + AcceptShareRequest handler** - `2498633` (feat)
2. **Task 2: ExpireTimedAcceptedShares repo method + ExpiryJob extension** - `e678bdd` (feat)

## Files Created/Modified

- `services/share-service/models/share_request.go` - Added ExpiryOption and ShareExpiresAt fields to ShareRequest struct
- `services/share-service/handlers/shares.go` - Updated AcceptShareRequest: compute shareExpiresAt for custom option, persist both expiry fields in UPDATE query, set them on shareRequest for JSON response
- `services/share-service/repository/share_repo.go` - Added ExpireAcceptedShare (transactional single-share expiry) and ExpireTimedAcceptedShares (batch expiry for custom-timed shares)
- `services/share-service/jobs/expiry.go` - Extended expireOldRequests to call both ExpirePendingRequests and ExpireTimedAcceptedShares
- `services/share-service/jobs/expiry_test.go` - Updated TestExpiryJob_TimedAcceptedShares from Wave 0 stub to full GREEN integration test
- `services/share-service/jobs/lifecycle_subscriber.go` - Created LifecycleSubscriber stub with StreamEndEvent and NewLifecycleSubscriber for Wave 2

## Decisions Made

- AcceptShareRequest computes shareExpiresAt only for custom option (unlimited and this_stream leave it NULL in the database)
- ExpireAcceptedShare is idempotent: if RowsAffected==0, returns nil (share already expired or not found)
- ExpireTimedAcceptedShares closes the rows cursor before the per-share expiry loop to avoid holding an open cursor during nested transactions
- LifecycleSubscriber stub added in Wave 1 (not Wave 2) to make the jobs package compile so all tests pass with -short

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Created lifecycle_subscriber.go stub**
- **Found during:** Task 2 (jobs package compilation)
- **Issue:** lifecycle_subscriber_test.go (Wave 0 RED stub from Phase 19-00) references StreamEndEvent and NewLifecycleSubscriber which did not exist, causing build failure
- **Fix:** Created minimal lifecycle_subscriber.go with stub types so the jobs package compiles and tests pass with -short
- **Files modified:** services/share-service/jobs/lifecycle_subscriber.go (created)
- **Verification:** go test ./... -short passes with all packages OK
- **Committed in:** e678bdd (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Stub creation was the correct fix per plan's done criteria ("RED stub test now compiles and skips gracefully in short mode"). No scope creep.

## Issues Encountered

None - all changes went as planned.

## Next Phase Readiness

- Expiry infrastructure complete: sharing lifecycle can now expire custom-timed accepted shares automatically
- Wave 2 (lifecycle_subscriber) can implement full Redis Pub/Sub stream-end handling using ExpireAcceptedShare
- Wave 3 (frontend) can read expiry_option from ShareRequest JSON to display expiry status in UI

---
*Phase: 19-lifecycle-expiry*
*Completed: 2026-03-11*
