---
phase: 05-tiktok-listener-demand-driven-polling-only-poll-when-overlay-has-connected-clients
plan: "03"
subsystem: tiktok-listener
tags:
  - demand-driven
  - redis-pubsub
  - tiktok
  - typescript
  - tdd
dependency_graph:
  requires:
    - 05-01 (source-manager demand publisher)
    - 05-02 (demand infrastructure)
  provides:
    - TikTok listener subscribes to source:demand Pub/Sub
    - TikTok listener goes fully idle on zero demand
  affects:
    - services/tiktok-listener/src/index.ts
    - services/tiktok-listener/src/demand/subscriber.ts
    - services/tiktok-listener/src/coordination/client.ts
tech_stack:
  added:
    - vitest (test framework)
  patterns:
    - DemandSubscriber mirrors MigrationSubscriber (duplicate Redis connection for Pub/Sub)
    - Full-snapshot demand: handler receives Map<username, DemandSource> on each update
    - LiveStreamPoller lifecycle controlled by demand (stop on zero, start on first)
key_files:
  created:
    - services/tiktok-listener/src/demand/subscriber.ts
    - services/tiktok-listener/src/demand/subscriber.test.ts
    - services/tiktok-listener/vitest.config.ts
  modified:
    - services/tiktok-listener/src/index.ts
    - services/tiktok-listener/src/coordination/client.ts
    - services/tiktok-listener/package.json
decisions:
  - "assignedSourceIDs changed from Map<string,boolean> to Set<string> to align with DemandSubscriber constructor signature"
  - "getDemand() added to CoordinatorClient to avoid exposing raw auth headers; uses existing axios interceptor for JWT"
  - "livePollerRunning boolean guards LiveStreamPoller start/stop — poller does not start until first non-empty demand update"
  - "pollDemandFallback skips execution when coordinatorClient is absent (non-coordinator deployments remain fully functional)"
  - "vi.fn() cast as unknown as MockInstance & DemandHandler to satisfy strict TypeScript checking of DemandHandler type"
metrics:
  duration: "6m20s"
  completed_date: "2026-03-27"
  tasks_completed: 2
  files_modified: 6
---

# Phase 05 Plan 03: TikTok Demand Subscriber Summary

**One-liner:** DemandSubscriber subscribes to source:demand Redis Pub/Sub, replacing 30s DB+Redis-key-scan polling with event-driven connect/disconnect and 60s safety-net fallback.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 (TDD RED) | Add failing tests for DemandSubscriber | 795c10d | subscriber.test.ts, vitest.config.ts, package.json |
| 1 (TDD GREEN) | Implement DemandSubscriber class | ed13107 | subscriber.ts |
| 2 | Wire DemandSubscriber into index.ts, delete old polling | e61ecca | index.ts, client.ts, subscriber.test.ts |

## What Was Built

### DemandSubscriber class (`src/demand/subscriber.ts`)

- Subscribes to `source:demand` Redis Pub/Sub channel using a duplicated Redis connection (node-redis requirement)
- Receives full-snapshot `DemandUpdate` messages from source-manager
- Filters to `platform === 'tiktok'` sources only
- Applies `assignedSourceIDs` set filter when non-empty (coordinator sharding)
- Passes `Map<username, DemandSource>` to handler on each update
- `updateAssignedSourceIDs()` allows live update when coordinator migration events arrive

### index.ts changes

- **Deleted:** `startPolling()`, `startDatabaseListener()`, `pollActiveStreams()` methods
- **Deleted:** `POLL_INTERVAL_MS`, `NOTIFICATION_DEBOUNCE_MS` constants
- **Deleted:** `pg.Client` (listenClient) for pg_notify LISTEN
- **Deleted:** `pollTimer`, `listenClient`, `notificationDebounceTimer`, `pendingNotificationCount` fields
- **Added:** `handleDemandUpdate()` — connects/disconnects streams based on demand snapshot
- **Added:** `pollDemandFallback()` — 60s safety-net via `coordinatorClient.getDemand('tiktok')`
- **Added:** `demandSubscriber` field, `demandSafetyInterval`, `livePollerRunning` boolean
- **Changed:** `assignedSourceIDs` from `Map<string, boolean>` to `Set<string>`
- **Changed:** `LiveStreamPoller.start()` deferred to first non-empty demand update; `stop()` called on zero demand

### CoordinatorClient changes (`src/coordination/client.ts`)

- Added `getDemand(platform?)` method that calls `GET /demand?platform=tiktok` using existing axios JWT interceptor

### Test setup

- Added `vitest` to devDependencies
- Created `vitest.config.ts` for ESM/Node environment
- Updated `npm test` script from `echo "No tests yet"` to `vitest run`
- 6 unit tests: subscribe call, tiktok filter, empty sources, non-tiktok filter, assignedSourceIDs filter, malformed JSON

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] TypeScript type error on vi.fn() as DemandHandler**
- **Found during:** Task 2 TypeScript compile check
- **Issue:** `vi.fn().mockResolvedValue(undefined)` inferred as `Mock<Procedure | Constructable>` which does not satisfy the `DemandHandler` function signature under strict TypeScript
- **Fix:** Cast as `unknown as MockInstance & DemandHandler` in test file
- **Files modified:** `src/demand/subscriber.test.ts`
- **Commit:** e61ecca

**2. [Rule 2 - Missing functionality] getDemand() not on CoordinatorClient**
- **Found during:** Task 2 implementation of `pollDemandFallback()`
- **Issue:** Plan referenced `coordinatorClient.getAuthHeaders()` which doesn't exist; client uses axios interceptors
- **Fix:** Added `getDemand(platform?)` method to CoordinatorClient using existing authenticated axios instance
- **Files modified:** `src/coordination/client.ts`
- **Commit:** e61ecca

**3. [Rule 1 - Bug] pg.Client import no longer needed**
- **Found during:** Task 2 cleanup
- **Issue:** `import { Pool, Client, Notification } from 'pg'` — Client and Notification unused after deleting startDatabaseListener
- **Fix:** Changed to `import { Pool } from 'pg'`
- **Files modified:** `src/index.ts`
- **Commit:** e61ecca

## Self-Check

### Created files exist:
- `/home/moersener/Hobby/all-chat/services/tiktok-listener/src/demand/subscriber.ts` — FOUND
- `/home/moersener/Hobby/all-chat/services/tiktok-listener/src/demand/subscriber.test.ts` — FOUND
- `/home/moersener/Hobby/all-chat/services/tiktok-listener/vitest.config.ts` — FOUND

### Commits exist:
- 795c10d — FOUND
- ed13107— FOUND
- e61ecca — FOUND

### Acceptance criteria:
- `export class DemandSubscriber` — FOUND in subscriber.ts
- `subscribe.*source:demand` — FOUND
- `redisClient.duplicate()` — FOUND
- `platform === 'tiktok'` — FOUND
- `new DemandSubscriber` — FOUND in index.ts
- `handleDemandUpdate` — FOUND in index.ts
- `livePoller.stop()` in handleDemandUpdate — FOUND
- `pollActiveStreams` absent — CONFIRMED (only in comment)
- `startDatabaseListener` absent — CONFIRMED (only in comment)
- `startPolling` absent — CONFIRMED
- TypeScript compiles — PASSED
- All 6 tests pass — PASSED

## Self-Check: PASSED
