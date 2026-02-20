# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-02-19)

**Core value:** Listener instances must efficiently distribute channel workload based on actual message volume, enabling cost-effective scaling and reliable service for both small and high-traffic streams.
**Current focus:** Phase 6 Complete - Ready for Phase 7 (Dynamic Rebalancing & HPA)

## Current Position

Milestone: v1.1 Listener Load Balancing
Phase: 8 of 8 (Observability & Production Readiness)
Plan: 1 of 4 in current phase
Status: In Progress
Last activity: 2026-02-20 — Completed 08-01-PLAN.md (Shard & Migration Metrics)

Progress: [████████░░] 74% (v1.0 partial: 11/11 plans complete, v1.1: 26/31 plans complete)

## Performance Metrics

**Velocity (all phases):**
- Total plans completed: 26
- Average duration: 11.5 min
- Total execution time: 5.12 hours

**By Phase:**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 1. Foundation + Twitch | 5 | 33 min | 6.6 min |
| 2. YouTube Integration | 2 | 9 min | 4.5 min |
| 3. Kick Integration | 4 | 15 min | 3.75 min |
| 5. Sharding Infrastructure | 5 | 22 min | 4.4 min |
| 6. Connection Management | 8 | 217 min | 27.1 min |
| 7. Dynamic Rebalancing | 4 | 22 min | 5.5 min |
| 8. Observability & Production | 1 | 4 min | 4.0 min |

**Recent Trend:**
- Last 5 plans: 33.8 min average (includes 180 min deployment testing)
- Trend: Phase 6 P06 deployment testing significantly higher than typical plan duration (P07/P08 gap closures returned to ~5 min normal)

*Updated after each plan completion*

| Plan | Duration | Tasks | Files |
|------|----------|-------|-------|
| Phase 05 P01 | 5 min | 2 tasks | 7 files |
| Phase 05 P02 | 5 | 4 tasks | 7 files |
| Phase 05 P03 | 4 | 3 tasks | 5 files |
| Phase 05 P04 | 3 | 3 tasks | 5 files |
| Phase 05 P05 | 5 | 3 tasks | 2 files |
| Phase 06 P01 | 2 | 3 tasks | 3 files |
| Phase 06 P02 | 3 | 3 tasks | 4 files |
| Phase 06 P03 | 5 | 3 tasks | 4 files |
| Phase 06 P04 | 5 | 3 tasks | 7 files |
| Phase 06 P05 | 12 | 3 tasks | 3 files |
| Phase 06 P06 | 180 | 2 tasks | 7 files |
| Phase 06 P07 | 7 | 3 tasks | 6 files |
| Phase 06 P08 | 3 | 3 tasks | 5 files |
| Phase 07 P01 | 5 | 2 tasks | 3 files |
| Phase 07 P02 | 4 | 2 tasks | 4 files |
| Phase 07 P03 | 9 | 3 tasks | 7 files |
| Phase 07 P04 | 4 | 3 tasks | 6 files |
| Phase 08 P01 | 4 | 3 tasks | 5 files |
| Phase 08 P02 | 4 | 3 tasks | 7 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.
Recent decisions affecting current work:

- v1.1: Hybrid hash-based + load-aware approach for predictable under normal load, adapts to real-world usage patterns
- v1.1: Consistent hashing for channel assignment to minimize reassignments when pods scale
- v1.1: Redis for assignment registry (centralized state, atomic updates, survives pod restarts)
- v1.1: Reuse source-manager for coordination (already has leader election, knows active sources)
- [Phase 05]: CRC32 hash function for consistent hashing (simple, fast, sufficient)
- [Phase 05]: Redis Sorted Set for O(log N) load tracking queries
- [Phase 05]: Kubernetes Lease-based leader election for split-brain prevention (automatic fencing via resourceVersion)
- [Phase 05]: 30s lease duration, 15s renew deadline provides 50% safety margin for network hiccups
- [Phase 05]: Redis Sorted Set for heartbeat monitoring (single ZRANGEBYSCORE query vs O(N) GETs, historical data for debugging)
- [Phase 05]: Service JWT authentication for assignment endpoints (reuse existing middleware, consistent security pattern)
- [Phase 05]: Query parameter for pod_id filtering in GET /assignments (RESTful, simple, easier to debug)
- [Phase 05]: 15-second heartbeat timeout for fast recovery (user constraint: 60s would be catastrophic for fast-acting streams)
- [Phase 05]: Split chaos testing: 3 automated scenarios (leader/listener/simultaneous failures), 2 manual (network partition/Redis latency require privileged access)
- [Phase 06]: Blocks indefinitely on QueryAssignments until coordinator responds (per CONTEXT.md user decision)
- [Phase 06]: Hybrid Redis Pub/Sub for migration events (5-20ms latency vs 15s polling)
- [Phase 06 P04]: TypeScript coordinator integration for TikTok listener (no Go rewrite, mirror Go patterns with axios + ioredis)
- [Phase 06 P05]: Dual publishing (Pub/Sub + Streams) for migration events (notification + observability)
- [Phase 06 P05]: 60s confirmation timeout per CONTEXT.md constraint (old pod disconnect timeout)
- [Phase 06 P06]: Coordinator assigns to Running pods (not Ready) to break readiness probe chicken-and-egg
- [Phase 06 P06]: Skip migration confirmation wait for failed pods (prevents reconciliation blocking)
- [Phase 06 P06]: 60s periodic assignment refresh in listeners (handles transient GetAssignments failures)
- [Phase 06 P06]: Generate signed JWTs from SERVICE_JWT_SECRET (not raw secret as token)
- [Phase 06 P06]: Filter by coordinator assignments even when map is empty (preserves migration protocol)
- [Phase 06 P07]: Non-blocking channel send for Twitch first message signaling (prevents deadlock when no migration waiting)
- [Phase 06 P07]: Callback pattern for TikTok first message detection (TypeScript idiomatic, simpler than event emitters)
- [Phase 06 P07]: Sequence number always 0 for migration confirmations (gap detection deferred to future phase)
- [Phase 06 P08]: Capture filtered count before lock acquisition (thread-safe field update pattern)
- [Phase 06 P08]: Add filtered tracking to TikTok despite no bug (consistency across all listeners)
- [Phase 07 P01]: Composite load score uses 70% message rate + 30% channel count (message processing dominates CPU per research)
- [Phase 07 P01]: Dual-condition gating requires BOTH ratio > 0.5 AND maxMsgRate > 100 (prevents unnecessary rebalancing when idle)
- [Phase 07 P01]: Graceful degradation to channel count only when Prometheus unavailable (system continues operating)
- [Phase 07 P01]: Per-pod load scores exposed via Prometheus PodLoadScore metric with pod_id label
- [Phase 07 P02]: Proportional redistribution strategy selects low-traffic channels first (sorted ascending by message rate)
- [Phase 07 P02]: 20% per-pod migration limit with minimum 1 channel enforcement
- [Phase 07 P02]: Round-robin target pod selection distributes migrations evenly across underutilized pods
- [Phase 07 P02]: 5-minute Prometheus query window for channel rate stability (longer than 30s monitoring window)
- [Phase 07]: 5-minute cooldown enforced via Redis TTL for automatic expiration
- [Phase 07]: Thrashing response: Alert-only (log error, enforce cooldown, let operators investigate)
- [Phase 07]: Incomplete rebalancing counter increments after every rebalancing, resets when balanced
- [Phase 07]: Hybrid strategy escalation after 3 attempts (proportional + hot channel migrations)
- [Phase 07 P04]: 60-second coordination lock TTL for automatic failover (balances operation time vs recovery speed)
- [Phase 07 P04]: Lua scripts for atomic lock release/extend with ownership verification (prevents releasing other operations' locks)
- [Phase 07 P04]: 0-30 second startup jitter prevents thundering herd during HPA scale-up (spreads coordinator queries)
- [Phase 07 P04]: 30-second wait after scale-up detection allows jittered pods to fully start (0-30s jitter window)
- [Phase 08]: Use [1,5,10,30,60,120]s buckets for migration duration histogram
- [Phase 08]: Record pod channel count after channelMgr.Start() to capture filtered assignments

### Pending Todos

None yet.

### Blockers/Concerns

**Phase 6 Complete - All blockers resolved:**
- ✅ Twitch/Kick Listener Critical Issue RESOLVED: Root cause was readiness probe comparing active channels against raw coordinator assignments instead of filtered count. Fixed by tracking filtered assignment count (assigned sources with database channels).
- ✅ All platforms (Twitch, Kick, TikTok) successfully scale with HPA and report ready status
- ✅ Migration protocol validated with zero message loss
- ✅ Readiness probe bug fixed: Pods reach Ready state after connecting to all filtered assigned channels

**Phase 7 Considerations:**
- YouTube quota exhaustion risk during scale-up (circuit breaker required in Phase 7)
- Readiness probe strictness may need tuning for inactive sources
- Migration confirmation timeout edge cases need more chaos testing

## Session Continuity

Last session: 2026-02-20
Stopped at: Completed 08-01-PLAN.md (Shard & Migration Metrics)
Resume file: None

**Next action:** Continue Phase 8 with Plan 02 (Health Check Enhancements) or Plan 04 (Grafana Dashboards).
