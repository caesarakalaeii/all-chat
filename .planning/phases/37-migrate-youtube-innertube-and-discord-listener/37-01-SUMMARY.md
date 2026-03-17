---
phase: 37-migrate-youtube-innertube-and-discord-listener
plan: 01
subsystem: infra
tags: [go, listener-sdk, goleak, sourcemanager]

requires:
  - phase: 36-migrate-kick-listener
    provides: LeadershipListener struct with smClient field; established goleak-as-direct-dep pattern
provides:
  - SMClient() accessor on LeadershipListener (shared/listener/leadership.go)
  - goleak v1.3.0 as direct dep in youtube-listener-innertube/go.mod
  - goleak v1.3.0 as direct dep in discord-listener/go.mod
affects:
  - 37-02 (innertube migration — uses SMClient() to pass *sourcemanager.Client to streams.NewManager)
  - 37-03 (discord migration — uses SMClient(); goleak direct dep for smoke test)

tech-stack:
  added: [go.uber.org/goleak v1.3.0 (direct, two services)]
  patterns:
    - "goleak placed in direct require block (not indirect) before any .go file imports it — forward dep for plan smoke tests"
    - "SMClient() accessor mirrors LeadershipCoordinator() pattern exactly in structure and doc comment"

key-files:
  created: []
  modified:
    - shared/listener/leadership.go
    - services/youtube-listener-innertube/go.mod
    - services/youtube-listener-innertube/go.sum
    - services/discord-listener/go.mod
    - services/discord-listener/go.sum

key-decisions:
  - "go mod edit -require used to place goleak in the direct require block; go mod download fetches checksums without running tidy (which would strip it as unused)"
  - "SMClient() returns l.smClient with nil-check caveat in doc comment — mirrors LeadershipCoordinator() nil-safety pattern"

patterns-established:
  - "Direct-dep forward placement: use go mod edit -require + go mod download to register a dep before any .go file imports it"

requirements-completed: [MIGRATE-04, MIGRATE-05]

duration: 2min
completed: 2026-03-17
---

# Phase 37 Plan 01: Wave 1 Prerequisites Summary

**SMClient() accessor added to LeadershipListener and goleak v1.3.0 pinned as direct dep in both innertube and discord listener modules**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-17T22:46:01Z
- **Completed:** 2026-03-17T22:48:00Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments

- Added `SMClient() *sourcemanager.Client` accessor to `LeadershipListener` in `shared/listener/leadership.go`, enabling Plans 02 and 03 to extract the client without struct field access
- Registered `go.uber.org/goleak v1.3.0` as a direct require entry (not indirect) in `services/youtube-listener-innertube/go.mod` — forward dep for the cmd/main_sdk_test.go smoke test in Plan 02
- Registered `go.uber.org/goleak v1.3.0` as a direct require entry in `services/discord-listener/go.mod` — forward dep for the cmd/main_sdk_test.go smoke test in Plan 03

## Task Commits

Each task was committed atomically:

1. **Task 1: Add SMClient() accessor to shared/listener/leadership.go** - `ca2751e` (feat)
2. **Task 2: Add goleak as direct dep to both services' go.mod** - `9c26b4a` (chore)

## Files Created/Modified

- `shared/listener/leadership.go` - Added SMClient() accessor method at end of file
- `services/youtube-listener-innertube/go.mod` - go.uber.org/goleak v1.3.0 in direct require block
- `services/youtube-listener-innertube/go.sum` - checksum entry for goleak
- `services/discord-listener/go.mod` - go.uber.org/goleak v1.3.0 in direct require block
- `services/discord-listener/go.sum` - checksum entry for goleak (already present as transitive)

## Decisions Made

- `go mod edit -require` combined with `go mod download` (not `go get` + `go mod tidy`) used to place goleak in the direct block without removing it as unused — the same forward-dep pattern established in Phases 35 and 36
- discord-listener go.mod required manual edit to move goleak from indirect block to direct block after `go mod edit` placed it alongside existing indirect entries

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] go mod tidy stripped goleak after initial go get**
- **Found during:** Task 2 (Add goleak as direct dep)
- **Issue:** Running `go get go.uber.org/goleak@v1.3.0` followed by `go mod tidy` removed goleak from go.mod because no .go file imports it yet
- **Fix:** Used `go mod edit -require go.uber.org/goleak@v1.3.0` + `go mod download go.uber.org/goleak@v1.3.0` to register it without tidy removing it; manually moved discord-listener entry from indirect block to direct block
- **Files modified:** services/discord-listener/go.mod
- **Verification:** `grep "goleak" go.mod` shows no `// indirect` suffix in both files; `go build ./...` exits 0 in both services and `make build-all` exits 0
- **Committed in:** 9c26b4a (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - build process issue)
**Impact on plan:** Auto-fix was essential to correctly place goleak as direct dep. No scope creep.

## Issues Encountered

None beyond the go mod tidy deviation documented above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plans 02 and 03 are unblocked: `SMClient()` is available on `LeadershipListener` and goleak is ready for import in smoke tests
- `make build-all` passes across all listener modules
- No blockers

---
*Phase: 37-migrate-youtube-innertube-and-discord-listener*
*Completed: 2026-03-17*

## Self-Check: PASSED

- shared/listener/leadership.go: FOUND, SMClient() method count = 1
- services/youtube-listener-innertube/go.mod: FOUND, goleak v1.3.0 (direct)
- services/discord-listener/go.mod: FOUND, goleak v1.3.0 (direct)
- 37-01-SUMMARY.md: FOUND
- Commit ca2751e (Task 1): FOUND
- Commit 9c26b4a (Task 2): FOUND
