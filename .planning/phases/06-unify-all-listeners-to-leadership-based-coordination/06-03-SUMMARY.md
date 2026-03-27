---
phase: 06-unify-all-listeners-to-leadership-based-coordination
plan: 03
subsystem: infra
tags: [go, coordination, source-manager, k8s, cleanup]

# Dependency graph
requires:
  - phase: 06-02
    provides: twitch-listener, kick-listener, twitch-eventsub-listener migrated to LeadershipListener; no shared/coordination imports in those services

provides:
  - shared/coordination package fully deleted
  - services/source-manager/coordination directory fully deleted
  - /assignments and /heartbeat endpoints removed from source-manager
  - source-manager defaults to port 8083
  - All K8s manifests consistent with port 8083
  - kick-listener K8s deployment has no COORDINATOR_URL
  - twitch-eventsub-listener K8s deployment has SOURCE_MANAGER_URL and SOURCE_MANAGER_SECRET

affects: [source-manager, k8s manifests, discord-listener, youtube-listener, youtube-listener-innertube]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Coordinator-free source-manager: leadership/demand APIs only on port 8083
    - All listeners use LeadershipListener exclusively — no assignment-based coordination pattern remains

key-files:
  created: []
  modified:
    - services/source-manager/cmd/main.go
    - deployments/k8s/base/source-manager/deployment.yaml
    - deployments/k8s/base/configmap.yaml
    - deployments/k8s/base/kick-listener/deployment.yaml
    - deployments/k8s/base/twitch-eventsub-listener/deployment.yaml
    - services/discord-listener/cmd/main.go
    - services/discord-listener/cmd/main_sdk_test.go
    - services/youtube-listener/cmd/main.go
    - services/youtube-listener/cmd/main_sdk_test.go
    - services/youtube-listener-innertube/cmd/main.go
    - services/youtube-listener-innertube/cmd/main_sdk_test.go
    - services/source-manager/go.mod
    - shared/go.mod

key-decisions:
  - "shared/coordination fully deleted per D-01/D-02 — no deprecated fallback; Plans 01+02 already removed all callers"
  - "source-manager port changed from 8088 to 8083 per D-05 — coordinator-only port eliminated, consolidates to single leadership/demand API port"
  - "discord-listener, youtube-listener, youtube-listener-innertube fixed to use NewLeadershipListenerFromEnv(platform, redis, logger) — old NewListenerBase+NewLeadershipListenerFromEnv(base, ...) API removed in Plan 01"

patterns-established:
  - "NewLeadershipListenerFromEnv(platform, redisClient, logger) is the sole production constructor for all leadership-based listeners"

requirements-completed: [D-01, D-02, D-03, D-04, D-05, D-16]

# Metrics
duration: 25min
completed: 2026-03-28
---

# Phase 06 Plan 03: Remove Coordinator Infrastructure Summary

**shared/coordination deleted, source-manager coordination directory removed, /assignments+/heartbeat endpoints gone, port consolidated to 8083, K8s manifests updated — coordinator infrastructure fully eliminated**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-03-28T00:00:00Z
- **Completed:** 2026-03-28T00:25:00Z
- **Tasks:** 2
- **Files modified:** 13 (22 deleted, 13 modified)

## Accomplishments

- Deleted `shared/coordination/` package entirely (5 files: client.go, client_jwt_test.go, migration_subscriber.go, migration_subscriber_test.go, models.go)
- Deleted `services/source-manager/coordination/` directory entirely (17 files: coordinator, assigner, heartbeat, load_monitor, migration_publisher, rebalancer, registry, throttler plus their tests)
- Deleted `services/source-manager/handlers/assignments.go` — `/assignments` and `/heartbeat` endpoints removed
- Updated `services/source-manager/cmd/main.go`: removed all coordinator init/start/stop/routes, removed `shardMetrics`/`businessMetrics` (only used by coordinator), changed default port from 8088 to 8083
- Updated K8s source-manager deployment: all port references 8088 → 8083 (containerPort, PORT env, liveness/readiness probes, Service port/targetPort)
- Updated K8s configmap: `SOURCE_MANAGER_URL` updated from `:8088` to `:8083`
- Updated kick-listener K8s deployment: removed `COORDINATOR_URL` and redundant `SERVICE_JWT_SECRET` env vars
- Updated twitch-eventsub-listener K8s deployment: added `SOURCE_MANAGER_URL` and `SOURCE_MANAGER_SECRET` for LeadershipListener
- Fixed discord-listener, youtube-listener, youtube-listener-innertube: updated from old `NewListenerBase` + `NewLeadershipListenerFromEnv(base, ...)` API to new `NewLeadershipListenerFromEnv(platform, redis, logger)` API
- Updated SDK smoke tests in discord-listener, youtube-listener, youtube-listener-innertube to use `NewLeadershipListener` (removed `HandleMigrationEvent`, `MockCoordinator`, old `ListenerBase` pattern)
- Ran `go mod tidy` in `services/source-manager` and `shared` modules

## Task Commits

1. **Task 1: Remove coordinator from source-manager and delete shared/coordination** — `5f5b51b` (feat)
2. **Task 2: Update K8s manifests for port 8083 and remove coordinator env vars** — `afbca2d` (chore)

## Files Created/Modified

- `services/source-manager/cmd/main.go` — coordinator removed, port 8083, metrics import removed
- `services/source-manager/coordination/` — DELETED (17 files)
- `services/source-manager/handlers/assignments.go` — DELETED
- `services/source-manager/go.mod` — go mod tidy (coordinator deps removed)
- `shared/coordination/` — DELETED (5 files)
- `shared/go.mod` — go mod tidy
- `deployments/k8s/base/source-manager/deployment.yaml` — all 8088 → 8083
- `deployments/k8s/base/configmap.yaml` — SOURCE_MANAGER_URL port 8083
- `deployments/k8s/base/kick-listener/deployment.yaml` — COORDINATOR_URL + SERVICE_JWT_SECRET removed
- `deployments/k8s/base/twitch-eventsub-listener/deployment.yaml` — SOURCE_MANAGER_URL + SOURCE_MANAGER_SECRET added
- `services/discord-listener/cmd/main.go` — NewLeadershipListenerFromEnv updated to new API
- `services/discord-listener/cmd/main_sdk_test.go` — updated to NewLeadershipListener, HandleMigrationEvent removed
- `services/youtube-listener/cmd/main.go` — NewListenerBase/DefaultConfig removed, new API
- `services/youtube-listener/cmd/main_sdk_test.go` — updated to NewLeadershipListener
- `services/youtube-listener-innertube/cmd/main.go` — new LeadershipListener API
- `services/youtube-listener-innertube/cmd/main_sdk_test.go` — updated to NewLeadershipListener

## Decisions Made

- `shared/coordination` deleted completely — no deprecated fallback kept; Plans 01 and 02 had already removed all callers in shared/listener, twitch-listener, kick-listener, and twitch-eventsub-listener
- Port 8083 chosen per D-05 plan direction — source-manager now uses a single unified port for all APIs (leadership, demand, sources)
- Fixed discord-listener, youtube-listener, youtube-listener-innertube `main.go` inline (Rule 1 bug) — these files still referenced old `NewListenerBase`/`NewLeadershipListenerFromEnv(base, ...)` constructor signatures deleted in Plan 01

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Worktree was missing Plans 01/02 changes — merged main branch first**
- **Found during:** Task 1 pre-flight check
- **Issue:** Worktree `worktree-agent-ab5e3828` branch was at commit `63a94b2` (before Plans 01/02). `shared/coordination` grep still found many references.
- **Fix:** Fast-forward merged `main` branch into worktree (`git merge main --no-edit`). All Plan 01/02 changes applied cleanly.
- **Files modified:** (merge of 30 files from Plans 01/02)

**2. [Rule 1 - Bug] discord-listener, youtube-listener, youtube-listener-innertube used old constructor API**
- **Found during:** Task 1 (go build check after merge)
- **Issue:** These 3 services still called `listener.NewListenerBase(...)` and `listener.NewLeadershipListenerFromEnv(base, platform, log)` — the old two-arg construction pattern deleted in Plan 01
- **Fix:** Updated `cmd/main.go` in each service to use `NewLeadershipListenerFromEnv(platform, redisClient, logger)`. Also removed `listener.DefaultConfig()` and `HOSTNAME` pod name lookup from youtube-listener (no longer needed without ListenerBase).
- **Files modified:** services/discord-listener/cmd/main.go, services/youtube-listener/cmd/main.go, services/youtube-listener-innertube/cmd/main.go
- **Committed in:** 5f5b51b

**3. [Rule 1 - Bug] SDK smoke tests in discord-listener, youtube-listener, youtube-listener-innertube referenced deleted coordination package**
- **Found during:** Task 1 (grep for shared/coordination after deletions)
- **Issue:** 3 test files still imported `shared/coordination` for `MigrationEvent` in `HandleMigrationEvent` mock method, and used `listener.MockCoordinator`/`NewListenerBase` — all deleted in Plan 01
- **Fix:** Rewrote all 3 test files to use `NewLeadershipListener` config constructor, removed `HandleMigrationEvent` from mock (not in current ChannelManager interface), removed `testutil`/`coordination` imports
- **Files modified:** services/discord-listener/cmd/main_sdk_test.go, services/youtube-listener/cmd/main_sdk_test.go, services/youtube-listener-innertube/cmd/main_sdk_test.go
- **Committed in:** 5f5b51b

**4. [Rule 2 - Missing] Removed unused businessMetrics/shardMetrics after coordinator deletion**
- **Found during:** Task 1 (go build caught unused import)
- **Issue:** After removing coordinator, `metrics.NewBusinessMetrics()` and `metrics.NewShardMetrics()` were no longer used in source-manager main.go; caused unused import error
- **Fix:** Removed both metric initializations and the `shared/metrics` import
- **Files modified:** services/source-manager/cmd/main.go
- **Committed in:** 5f5b51b

---

**Total deviations:** 4 auto-fixed (1 worktree merge, 3 cascading API fix from Plan 01's deletions)
**Impact on plan:** All fixes necessary for correctness. No scope creep — all 3 services were part of the phase goal (leadership-only listener pattern). The worktree merge was a pure fast-forward with no conflicts.

## Issues Encountered

**Worktree behind main** — This parallel agent worktree was created at commit `63a94b2`, predating Plans 01/02 commits (`eaa71e4`, `4392503`, `a7d3ee2`, `8269255`, `8e1172a`). The `git merge main` fast-forward resolved this cleanly.

## Known Stubs

None — all changes are structural deletions and API migrations. No data flows are stubbed.

## Next Phase Readiness

- `shared/coordination` package fully deleted — no listener service references it
- `source-manager` coordinator-free, port 8083, all tests pass
- K8s manifests consistent — all services will connect to source-manager:8083
- All 6 Go listener services now use `LeadershipListener` exclusively
- Phase 06 complete: unified leadership-based coordination architecture achieved

---
*Phase: 06-unify-all-listeners-to-leadership-based-coordination*
*Completed: 2026-03-28*

## Self-Check: PASSED

- shared/coordination/: CONFIRMED DELETED
- services/source-manager/coordination/: CONFIRMED DELETED
- services/source-manager/handlers/assignments.go: CONFIRMED DELETED
- services/source-manager/cmd/main.go port 8083: FOUND
- deployments/k8s/base/configmap.yaml SOURCE_MANAGER_URL :8083: FOUND
- deployments/k8s/base/kick-listener/deployment.yaml COORDINATOR_URL: CONFIRMED ABSENT
- deployments/k8s/base/twitch-eventsub-listener/deployment.yaml SOURCE_MANAGER_URL: FOUND
- No shared/coordination references in any .go file: CONFIRMED
- Commit 5f5b51b: FOUND
- Commit afbca2d: FOUND
- 06-03-SUMMARY.md: FOUND
