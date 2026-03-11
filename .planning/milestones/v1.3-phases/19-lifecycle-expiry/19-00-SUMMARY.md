---
phase: 19-lifecycle-expiry
plan: "00"
subsystem: database
tags: [postgres, migration, tdd, testing, lifecycle, expiry, share-service, twitch-eventsub-listener, youtube-listener-innertube]

# Dependency graph
requires:
  - phase: 18-revocation
    provides: revocation status in share_requests, RED test gate pattern
provides:
  - migration 034 with expiry_option + share_expires_at columns on share_requests
  - RED compile-error gates for ExpireTimedAcceptedShares, NewLifecycleSubscriber, SubscribeToStreamOffline
  - t.Skip stub for HandleStreamOffline lifecycle event publisher (YouTube)
affects:
  - 19-01-share-service-expiry
  - 19-02-lifecycle-subscriber
  - 19-03-youtube-lifecycle

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Nyquist compliance: migration DDL + RED test stubs before any implementation"
    - "Compile-error RED gate: undefined methods in test files prevent premature GREEN"
    - "t.Skip RED stub: documents expected behavior for in-progress implementation"

key-files:
  created:
    - migrations/034_share_expiry_fields.sql
    - services/share-service/jobs/lifecycle_subscriber_test.go
    - services/twitch-eventsub-listener/eventsub/subscription_manager_test.go
  modified:
    - services/share-service/jobs/expiry_test.go
    - services/youtube-listener-innertube/poller/lifecycle_test.go
    - services/twitch-eventsub-listener/go.mod
    - services/twitch-eventsub-listener/go.sum

key-decisions:
  - "Migration 034 uses separate share_expires_at column (not expires_at which is the 7-day acceptance window for pending requests)"
  - "Index on (share_expires_at, status) with partial WHERE status='accepted' AND share_expires_at IS NOT NULL for efficient 5-minute expiry job queries"
  - "lifecycle_subscriber_test.go references StreamEndEvent + NewLifecycleSubscriber (both undefined) — compile error is the RED gate"
  - "subscription_manager_test.go needed go mod tidy in twitch-eventsub-listener to add testify dependency"
  - "YouTube lifecycle stub uses t.Skip rather than compile error — HandleStreamOffline already exists, publisher param extension is the RED gate"

patterns-established:
  - "Wave 0 pattern: write migration SQL + RED test stubs before Wave 1 implementation begins"
  - "Partial index on conditional WHERE clause for efficient status-filtered queries"

requirements-completed: [EXPIRY-01, EXPIRY-02, EXPIRY-03, EXPIRY-04, EXPIRY-05]

# Metrics
duration: 8min
completed: 2026-03-11
---

# Phase 19 Plan 00: Lifecycle Expiry Nyquist Compliance Summary

**Migration 034 adds expiry_option + share_expires_at columns, with four RED test stubs gating Wave 1-3 implementation via compile errors**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-03-11T17:38:00Z
- **Completed:** 2026-03-11T17:46:16Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- Created migration 034 with `expiry_option VARCHAR(20) DEFAULT 'unlimited'` and `share_expires_at TIMESTAMP NULL` columns plus a partial index for the 5-minute expiry job
- Appended `TestExpiryJob_TimedAcceptedShares` RED stub to `expiry_test.go` (compile error: `repo.ExpireTimedAcceptedShares undefined`)
- Created `lifecycle_subscriber_test.go` with `TestLifecycleSubscriber_StreamEnd` RED stub (compile errors: `StreamEndEvent undefined`, `NewLifecycleSubscriber undefined`)
- Created `subscription_manager_test.go` with `TestSubscribeToStreamOffline` RED stub (compile error: `sm.SubscribeToStreamOffline undefined`)
- Appended `TestHandleStreamOffline_PublishesLifecycleEvent` t.Skip stub to YouTube `lifecycle_test.go`

## Task Commits

1. **Task 1: Migration 034** - `bc9a81a` (feat)
2. **Task 2: RED test stubs** - `ad20d80` (test)

## Files Created/Modified
- `migrations/034_share_expiry_fields.sql` - Adds expiry_option + share_expires_at columns with partial index for expiry job
- `services/share-service/jobs/expiry_test.go` - Appended TestExpiryJob_TimedAcceptedShares RED stub
- `services/share-service/jobs/lifecycle_subscriber_test.go` - New file with TestLifecycleSubscriber_StreamEnd RED stub
- `services/twitch-eventsub-listener/eventsub/subscription_manager_test.go` - New file with TestSubscribeToStreamOffline RED stub
- `services/youtube-listener-innertube/poller/lifecycle_test.go` - Appended TestHandleStreamOffline_PublishesLifecycleEvent t.Skip stub
- `services/twitch-eventsub-listener/go.mod` - Updated with testify dependency (go mod tidy)
- `services/twitch-eventsub-listener/go.sum` - Updated checksums

## Decisions Made
- Migration 034 uses a separate `share_expires_at` column (not `expires_at`) — `expires_at` is the 7-day acceptance window for pending requests; conflating them would break existing expiry job logic
- Partial index `WHERE status = 'accepted' AND share_expires_at IS NOT NULL` makes the 5-minute expiry job O(matches) not O(table)
- `twitch-eventsub-listener` required `go mod tidy` to add testify/zap test dependencies before the new test file could compile to RED
- YouTube lifecycle stub uses `t.Skip` not compile error — `HandleStreamOffline` already exists, so the RED gate is documenting a parameter extension needed in Wave 3

## Deviations from Plan

**1. [Rule 3 - Blocking] go mod tidy for twitch-eventsub-listener**
- **Found during:** Task 2 (subscription_manager_test.go creation)
- **Issue:** `go: updates to go.mod needed` — testify/zap not in go.mod for this service
- **Fix:** Ran `go mod tidy` to add missing test dependencies
- **Files modified:** services/twitch-eventsub-listener/go.mod, go.sum
- **Verification:** `go vet ./...` shows compile error (SubscribeToStreamOffline undefined) not go.mod error
- **Committed in:** ad20d80 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Required for test file to compile to RED state. No scope creep.

## Issues Encountered
- `go build ./...` does not compile test files — used `go vet ./...` and `go test -list` to confirm RED compile errors exist in test files

## Next Phase Readiness
- Migration 034 ready for `make migrate-up` when database is available
- Wave 1 (plan 19-01): Implement `ExpireTimedAcceptedShares` in `repository.ShareRepository` to turn expiry_test.go GREEN
- Wave 2 (plan 19-02): Implement `LifecycleSubscriber` + `StreamEndEvent` + `SubscribeToStreamOffline` to turn lifecycle_subscriber_test.go and subscription_manager_test.go GREEN
- Wave 3 (plan 19-03): Add publisher parameter to `HandleStreamOffline` to turn YouTube lifecycle_test.go GREEN

---
*Phase: 19-lifecycle-expiry*
*Completed: 2026-03-11*
