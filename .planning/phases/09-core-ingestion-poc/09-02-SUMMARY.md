---
phase: 09-core-ingestion-poc
plan: 02
subsystem: youtube-listener-innertube
tags: [polling-loop, exponential-backoff, stream-state, error-handling, continuation-tokens]

dependency_graph:
  requires:
    - innertube-http-client
    - innertube-json-parser
  provides:
    - polling-loop-engine
    - exponential-backoff-state-machine
    - stream-state-tracking
  affects:
    - services/youtube-listener-innertube/poller

tech_stack:
  added:
    - github.com/cenkalti/backoff/v4 (exponential backoff library)
  patterns:
    - Continuation-based polling loop with fixed interval
    - Exponential backoff for transient errors (2s → 60s max)
    - State machine for stream lifecycle (active/failed/offline)
    - Graceful shutdown via context cancellation and WaitGroup
    - Interface-based design for testability (ClientInterface)

key_files:
  created:
    - services/youtube-listener-innertube/poller/backoff.go (112 lines)
    - services/youtube-listener-innertube/poller/backoff_test.go (283 lines)
    - services/youtube-listener-innertube/poller/state.go (77 lines)
    - services/youtube-listener-innertube/poller/poller.go (259 lines)
    - services/youtube-listener-innertube/poller/poller_test.go (446 lines)
  modified:
    - services/youtube-listener-innertube/innertube/types.go (fixed error unwrapping bug)
    - services/youtube-listener-innertube/go.mod (added backoff/v4 dependency)
    - services/youtube-listener-innertube/go.sum (dependency checksums)

decisions:
  - decision: Fixed 2-second polling interval (not adaptive)
    rationale: User decision for PoC simplicity; InnerTube pollingIntervalMillis ignored
    trade_offs: May poll more frequently than necessary, but simplifies implementation

  - decision: Infinite backoff retries (MaxElapsedTime = 0)
    rationale: Transient errors should never give up; fatal errors stop polling instead
    trade_offs: Could retry indefinitely on persistent transient errors, but user prefers resilience

  - decision: Service stays alive on fatal errors
    rationale: Prepares for Phase 10 multi-stream support (one stream fails, others continue)
    trade_offs: Failed streams remain in memory until explicitly removed

  - decision: Configurable log levels (debug vs info)
    rationale: Debug mode for PoC troubleshooting; production uses info (errors only)
    trade_offs: Debug mode verbose, but essential for diagnosing InnerTube issues

  - decision: Interface-based client for testability
    rationale: Enables mock client in unit tests without modifying production code
    trade_offs: Slight abstraction overhead, but dramatically simplifies testing

metrics:
  duration: 16 minutes
  tasks_completed: 2
  files_created: 5
  files_modified: 3
  test_coverage: "85%+"
  tests_passing: 100%
  lines_of_code: 1177
  completed_at: "2026-02-21T17:02:18Z"
---

# Phase 9 Plan 2: Polling Loop and Exponential Backoff Summary

**One-liner:** Built continuation-based polling engine with fixed 2s interval, exponential backoff for transient errors (2s → 60s max), stream state tracking (active/failed/offline), and graceful shutdown via context cancellation.

## What Was Built

### Exponential Backoff State Machine (`backoff.go`)

- **Backoff Configuration** (user-specified parameters):
  - `InitialInterval`: 2 seconds
  - `MaxInterval`: 60 seconds (capped)
  - `Multiplier`: 2.0 (doubles each retry: 2s → 4s → 8s → 16s → 32s → 60s)
  - `MaxElapsedTime`: 0 (infinite retries, never give up)
  - Jitter: enabled by default in backoff/v4 (±25% randomization)

- **Wait Logic**:
  - Fatal errors (401/403/404): return immediately, no backoff
  - Transient errors (429/5xx/network): wait with exponential backoff
  - Context cancellation: return `ctx.Err()` immediately
  - Logs backoff duration at WARN level for transient errors

- **Reset Logic**:
  - Called after successful poll to reset backoff to initial 2s interval
  - Logs at DEBUG level

- **Error Classification**:
  - Uses `innertube.IsFatalError()` and `innertube.IsTransientError()` from Plan 01
  - Fixed bug in `types.go`: use `errors.As()` to unwrap error chain (Deviation Rule 1)

### Stream State Tracking (`state.go`)

- **State Machine**:
  - `StateActive`: Polling normally
  - `StateFailed`: Fatal error encountered, polling stopped
  - `StateOffline`: Stream ended (no continuation token)

- **Thread-Safe State Access**:
  - `SetState()` / `GetState()`: Update/read current state
  - `SetError()` / `GetError()`: Record/retrieve last error
  - `UpdatePollTime()` / `GetLastPollTime()`: Track last successful poll
  - All methods protected by `sync.RWMutex`

### Polling Loop Engine (`poller.go`)

- **Poller Structure**:
  - `ClientInterface`: Interface for InnerTube client (enables mock testing)
  - `continuation`: Current continuation token
  - `channelID`: YouTube channel ID for message attribution
  - `interval`: Fixed polling interval (default: 2s)
  - `backoff`: Exponential backoff state machine
  - `state`: Stream state tracker
  - `logger`: Structured logger (zap)
  - `logLevel`: "debug" or "info" (controls verbosity)

- **Polling Loop**:
  1. Wait for interval ticker (2s)
  2. Call `GetLiveChatReplay()` with current continuation token
  3. Extract next continuation token
  4. Parse messages (using `innertube.ParseMessages()` from Plan 01)
  5. Handle errors (transient → backoff, fatal → stop)
  6. Update continuation token and state
  7. Repeat until context cancelled

- **Error Handling**:
  - **Transient errors**: Log at WARN, call `backoff.Wait()`, continue polling
  - **Fatal errors**: Log at ERROR, set state to `StateFailed`, stop polling (cancel context)
  - **Stream ended**: No continuation token → set state to `StateOffline`
  - **Parse errors**: Log at WARN, continue polling (non-fatal)

- **Graceful Shutdown**:
  - `Start()`: Launches polling loop in background goroutine, increments WaitGroup
  - `Stop()`: Cancels context, waits for WaitGroup (blocks until current poll completes)
  - Shutdown completes within ~2s (one poll cycle)

- **Configurable Logging**:
  - **Debug mode** (`logLevel == "debug"`):
    - Logs every poll attempt with continuation token preview
    - Logs every parsed message
  - **Default mode** (`logLevel == "info"`):
    - Logs errors only (after backoff retries)
    - Logs state changes (active → failed)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed error classification to handle wrapped errors**
- **Found during:** Task 1 backoff testing
- **Issue:** `ClassifyError()` used direct type assertion `err.(*HTTPStatusError)`, which failed when errors were wrapped with `fmt.Errorf(..., %w, ...)`
- **Fix:** Changed to `errors.As(err, &httpErr)` to properly unwrap error chain
- **Files modified:** `services/youtube-listener-innertube/innertube/types.go`
- **Commit:** f086f98 (included in Task 1 commit)
- **Impact:** Fatal errors were being classified as transient, causing unnecessary backoff instead of immediate stop

## Implementation Details

### Backoff Progression (with jitter)

| Retry | Expected | Actual Range (±25% jitter) |
|-------|----------|----------------------------|
| 1st   | 2s       | 1.5s - 2.5s                |
| 2nd   | 4s       | 3s - 5s                    |
| 3rd   | 8s       | 6s - 10s                   |
| 4th   | 16s      | 12s - 20s                  |
| 5th   | 32s      | 24s - 40s                  |
| 6th+  | 60s      | 45s - 75s (capped)         |

### State Transitions

```
Initial: Active
  ↓
  ├─ Successful poll → Active (backoff reset)
  ├─ Transient error → Active (backoff wait, then continue)
  ├─ Fatal error → Failed (stop polling)
  └─ No continuation → Offline (stream ended)
```

### Nil Safety

Added nil checks to prevent panics when InnerTube returns unexpected responses:
- Check `resp != nil` before accessing nested fields
- Check `resp.ContinuationContents.LiveChatContinuation.Actions != nil` before parsing
- Gracefully handle empty action lists (parse 0 messages)

### Interface Design for Testability

- **Production**: `*innertube.Client` satisfies `ClientInterface` automatically
- **Testing**: `MockClient` implements `ClientInterface` for unit tests
- No changes to production code needed for testing

## Test Coverage

### Backoff Tests (`backoff_test.go`)

- `TestNewBackoff`: Verifies default configuration (2s initial, 60s max, infinite retries)
- `TestBackoff_Wait_FatalError`: Fatal errors return immediately (< 100ms)
- `TestBackoff_Wait_TransientError`: First backoff ~2s (1s-3s with jitter)
- `TestBackoff_Wait_ContextCancellation`: Context cancelled during wait → return `ctx.Err()`
- `TestBackoff_Wait_UnknownError`: Unknown errors treated as transient (backoff applied)
- `TestBackoff_Reset`: Backoff resets to initial interval after successful operation
- `TestBackoffSequence`: Verifies exponential progression (2s → 4s)
- `TestBackoff_AllTransientErrorTypes`: All transient errors trigger backoff (429/500/502/503/504/network)

### Poller Tests (`poller_test.go`)

- `TestNewPoller`: Verifies default options (2s interval, info log level)
- `TestPoller_SuccessfulPolling`: Continuation token updates on each poll
- `TestPoller_TransientError`: Transient error triggers backoff, then resumes polling
- `TestPoller_FatalError`: Fatal error stops polling and sets state to Failed
- `TestPoller_StreamEnded`: No continuation token → state set to Offline
- `TestPoller_GracefulShutdown`: Stop() completes within 1 second
- `TestPoller_ContextCancellation`: Context cancellation stops polling gracefully
- `TestState_ThreadSafety`: Concurrent reads/writes to state (no data races)

### Coverage Metrics

- **Overall**: 85%+ coverage across poller package
- **All tests passing**: 100% (16 tests, 0 failures)
- **Total test execution time**: ~34 seconds (includes backoff waits)

## Known Limitations

### Deferred to Later Phases

- **Redis Publishing** (Phase 03): Parsed messages not yet published to Redis Streams (TODO comment in `poll()`)
- **Initial Continuation Extraction** (Phase 10): Requires extracting continuation from stream HTML (currently passed as parameter)
- **Multi-Stream Management** (Phase 10): Single-stream PoC only (no stream registry integration)
- **Adaptive Polling** (Not planned): User decision to use fixed 2s interval (InnerTube's `pollingIntervalMillis` ignored)

### Out of Scope for PoC

- Rate limiting (Phase 12): No IP-based rate limit tracking
- Metrics/observability (Phase 11): No Prometheus metrics for polling state
- Stream discovery (Phase 10): Channel → video ID resolution not implemented

## User Decisions Validated

### Fixed Polling Interval (Not Adaptive)

- **User decision**: Use fixed 2-second interval, ignore InnerTube's `pollingIntervalMillis` recommendation
- **Implementation**: Hardcoded `interval = 2 * time.Second` in `NewPoller()`
- **Rationale**: Simplifies PoC implementation; adaptive polling deferred to production optimization

### Infinite Backoff Retries

- **User decision**: Set `MaxElapsedTime = 0` (never give up on transient errors)
- **Implementation**: Backoff library configured with `MaxElapsedTime: 0`
- **Rationale**: Fatal errors stop polling immediately; transient errors should retry indefinitely

### Service Stays Alive on Fatal Errors

- **User decision**: Poller stops polling but service remains running
- **Implementation**: Fatal errors cancel polling context but don't crash service
- **Rationale**: Prepares for Phase 10 multi-stream support (one failed stream doesn't crash all streams)

### Configurable Log Levels

- **User decision**: Debug mode for PoC troubleshooting, production uses info level
- **Implementation**: `logLevel` parameter controls verbosity (debug logs every poll, info logs errors only)
- **Rationale**: InnerTube behavior unpredictable; debug mode essential for diagnosing issues

## Next Steps (Plan 03)

1. **Redis Integration**: Publish parsed messages to Redis Streams
2. **Stream Registry**: Track active streams and their polling state
3. **Control Plane**: API endpoints to start/stop stream monitoring
4. **Initial Continuation Extraction**: Extract continuation token from stream HTML
5. **Integration Testing**: End-to-end test with real InnerTube API

## Files Delivered

```
services/youtube-listener-innertube/poller/
├── backoff.go          # Exponential backoff state machine (112 lines)
├── backoff_test.go     # Backoff unit tests (283 lines)
├── state.go            # Stream state tracking (77 lines)
├── poller.go           # Polling loop engine (259 lines)
└── poller_test.go      # Poller unit tests (446 lines)

services/youtube-listener-innertube/innertube/
└── types.go            # Modified: fixed error unwrapping bug (3 lines changed)

services/youtube-listener-innertube/
├── go.mod              # Modified: added backoff/v4 dependency
└── go.sum              # Modified: dependency checksums
```

**Total**: 5 files created, 3 files modified, 1,177 lines of code, 729 lines of tests.

## Self-Check: PASSED

### Created Files Verification

```bash
✓ FOUND: services/youtube-listener-innertube/poller/backoff.go
✓ FOUND: services/youtube-listener-innertube/poller/backoff_test.go
✓ FOUND: services/youtube-listener-innertube/poller/state.go
✓ FOUND: services/youtube-listener-innertube/poller/poller.go
✓ FOUND: services/youtube-listener-innertube/poller/poller_test.go
```

### Commits Verification

```bash
✓ FOUND: f086f98 (feat(09-02): implement exponential backoff state machine)
✓ FOUND: 518f5f6 (feat(09-02): implement polling loop with stream state tracking)
```

### Test Verification

```bash
✓ go test ./poller -run TestBackoffSequence -v: PASSED (2s → 4s progression verified)
✓ go test ./poller -run TestPoller_FatalError -v: PASSED (state transitions to Failed)
✓ go test ./poller -run TestPoller_GracefulShutdown -v: PASSED (completes < 1s)
✓ go test ./poller -v: ALL TESTS PASSED (16/16, 85%+ coverage)
```

All deliverables verified successfully.
