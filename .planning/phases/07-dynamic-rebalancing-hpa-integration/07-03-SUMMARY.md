---
phase: 07-dynamic-rebalancing-hpa-integration
plan: 03
subsystem: source-manager/coordination
tags: [throttling, cooldown, thrashing-detection, escalation-override, hybrid-strategy]
dependency_graph:
  requires: [phase-05-sharding, phase-06-migration, phase-07-plan-01, phase-07-plan-02]
  provides: [rebalancing-throttler, cooldown-enforcement, thrashing-detection]
  affects: [coordinator, rebalancer]
tech_stack:
  added: []
  patterns: [cooldown-enforcement, thrashing-detection, escalation-override, hybrid-strategy]
key_files:
  created:
    - services/source-manager/coordination/throttler.go
    - services/source-manager/coordination/throttler_test.go
  modified:
    - shared/metrics/shard_metrics.go
    - services/source-manager/coordination/coordinator.go
    - services/source-manager/coordination/rebalancer.go
    - services/source-manager/coordination/rebalancer_test.go
    - services/source-manager/cmd/main.go
decisions:
  - 5-minute cooldown enforced via Redis key with TTL for automatic expiration
  - Thrashing detection uses Redis Sorted Set with 15-minute sliding window (>3 rebalances)
  - Escalation override allows breaking cooldown when imbalance ratio increases by >0.4
  - Incomplete rebalancing tracked via counter, escalates to hybrid strategy after 3 attempts
  - Thrashing response strategy: Alert-only (log error, enforce cooldown, let operators investigate)
  - Hot channel strategy selects top 1-2 channels with rate >3x pod average, ignores 20% limit
  - Graceful degradation on Redis errors (fail open - allow rebalancing to continue)
metrics:
  duration_minutes: 9
  tasks_completed: 3
  files_created: 2
  files_modified: 5
  commits: 3
  tests_added: 10
  completed_date: 2026-02-20
---

# Phase 07 Plan 03: Cooldown & Throttling Safeguards Summary

**One-liner:** Redis-backed cooldown enforcement (5-min) with thrashing detection (>3 in 15min), escalation overrides (ratio increase >0.4), and hybrid strategy escalation after 3 incomplete attempts.

## Objective

Implement throttling safeguards that prevent rebalancing thrashing via 5-minute cooldown enforcement, detect pathological patterns (>3 rebalances in 15 minutes), allow escalation overrides when imbalance significantly worsens, and escalate to hot-channel strategy after 3 incomplete rebalancing attempts.

## What Was Built

### 1. Throttler Implementation (services/source-manager/coordination/throttler.go)

**Core components:**
- `Throttler` struct with Redis client, cooldown duration (5 min), thrashing window (15 min), escalation threshold (0.4), metrics, logger
- Redis keys: `rebalancing:cooldown` (timestamp, TTL=5min), `rebalancing:history` (sorted set), `rebalancing:last_ratio` (float64)

**Key methods:**
- `CheckCooldown(ctx, currentRatio) (bool, string, error)`: Checks if rebalancing allowed
  - Returns false if cooldown active (< 5 minutes elapsed)
  - **Escalation override:** Allows breaking cooldown if `currentRatio - previousRatio > 0.4`
  - Checks thrashing after cooldown expires
  - Returns true with reason="ok" if allowed, false with reason="cooldown_active" or "thrashing_detected" if blocked
- `detectThrashing(ctx) (bool, error)`: Queries Redis Sorted Set for entries in 15-minute window
  - Uses `ZCOUNT rebalancing:history <cutoff> +inf` for O(log N) query
  - Returns true if count >= 3 (thrashing threshold)
  - Increments `RebalancingThrashing` metric
  - **Alert-only response:** Logs error, blocks rebalancing, lets operators investigate (per RESEARCH.md guidance)
- `RecordRebalancing(ctx, rebalanceID, imbalanceRatio) error`: Records successful operation
  - Uses Redis pipeline for atomic operations:
    - SET cooldown key with TTL=5min
    - ZADD to history sorted set
    - SET last_ratio for escalation comparison
    - ZREMRANGEBYSCORE to cleanup old history (>15 min)
  - Increments `RebalancingTotal` metric

**Graceful degradation:**
- Fail open on Redis errors (allow rebalancing to continue)
- Invalid timestamp parsing → allow rebalancing
- Missing previous ratio → defaults to 0 (first rebalancing always allowed)

### 2. Metrics Integration (shared/metrics/shard_metrics.go)

**Added metrics:**
- `RebalancingTotal` (Counter): Total rebalancing operations triggered
- `RebalancingCooldownOverrides` (Counter): Escalation overrides that broke cooldown
- `RebalancingThrashing` (Counter): Thrashing events detected (>3 in 15min)

### 3. Coordinator Integration (services/source-manager/coordination/coordinator.go)

**Struct updates:**
- Added `throttler *Throttler` field
- Added `incompleteRebalancingAttempts int` counter for persistent imbalance

**Reconciliation loop changes (Step 2.7):**
```go
if report.ShouldRebalance {
    // Check cooldown before triggering
    allowed, reason, err := c.throttler.CheckCooldown(ctx, report.ImbalanceRatio)
    if !allowed {
        c.logger.Info("Rebalancing skipped", zap.String("reason", reason))
        return nil
    }

    // Plan rebalancing with incomplete attempt count
    plans, err := c.rebalancer.PlanRebalancing(ctx, loads, report.AvgLoad, c.incompleteRebalancingAttempts)

    if err == nil {
        err := c.executeRebalancingPlans(ctx, plans, sourceMap)
        if err == nil {
            // Record successful rebalancing
            rebalanceID := fmt.Sprintf("rebalance-%d", time.Now().UnixNano())
            c.throttler.RecordRebalancing(ctx, rebalanceID, report.ImbalanceRatio)

            // Increment incomplete counter (reset when balanced)
            c.incompleteRebalancingAttempts++
        }
    }
} else {
    // Reset counter when system balanced
    if c.incompleteRebalancingAttempts > 0 {
        c.logger.Info("System balanced, resetting incomplete counter")
        c.incompleteRebalancingAttempts = 0
    }
}
```

**Key behaviors:**
- Cooldown checked before every rebalancing operation
- Incomplete counter increments after each rebalancing (even if successful)
- Counter resets to 0 when system becomes balanced (no imbalance detected)
- Counter persists across reconciliation cycles (field on Coordinator struct)

### 4. Hybrid Strategy Implementation (services/source-manager/coordination/rebalancer.go)

**Updated PlanRebalancing signature:**
```go
func (r *Rebalancer) PlanRebalancing(ctx context.Context, loads []PodLoad, avgLoad float64, attemptCount int) ([]MigrationPlan, error)
```

**Hybrid strategy logic:**
- If `attemptCount >= 3`: Add hot channel migrations to plans
- Logs warning: "Incomplete rebalancing detected, enabling hot channel migration"
- Calls `hotChannelStrategy()` to generate additional migration plans

**hotChannelStrategy() implementation:**
- Selects channels with message rate > 3x pod average (per REBAL-04)
- Ignores 20% migration limit (escalation strategy)
- Selects top 1-2 hot channels per overloaded pod
- Uses round-robin target selection across underutilized pods
- Returns empty if no hot channels qualify (threshold not met)

**Mathematical example:**
- 10 channels: 2 hot (1000, 900 msg/sec), 8 low (10 msg/sec each)
- Average = (1000 + 900 + 80) / 10 = 198 msg/sec
- Hot threshold = 3 * 198 = 594 msg/sec
- channel-hot-1 (1000) > 594 ✓ → qualifies
- channel-hot-2 (900) > 594 ✓ → qualifies
- Both selected for migration (max 2)

### 5. Component Wiring (services/source-manager/cmd/main.go)

**Initialization:**
```go
// Initialize throttler (Phase 7)
throttler := coordination.NewThrottler(redisClient, 5*time.Minute, shardMetrics, log)
log.Info("Initialized throttler", zap.Duration("cooldown_duration", 5*time.Minute))

// Pass to coordinator
coordinator := coordination.NewCoordinator(
    assignmentRegistry,
    assigner,
    repo,
    redisClient,
    heartbeatMonitor,
    migrationPublisher,
    loadMonitor,
    rebalancer,
    throttler,  // New parameter
    shardMetrics,
    log,
)
```

### 6. Comprehensive Unit Tests (throttler_test.go + rebalancer_test.go)

**Throttler tests (9 test cases):**
- `TestCheckCooldown_NoCooldown`: No cooldown key → allowed=true
- `TestCheckCooldown_ActiveCooldown`: 2 min elapsed → blocked (remaining: 3m0s)
- `TestCheckCooldown_ExpiredCooldown`: 6 min elapsed → allowed=true
- `TestCheckCooldown_EscalationOverride`: ratio increase 0.5 (> 0.4 threshold) → allowed with override
- `TestCheckCooldown_EscalationBelowThreshold`: ratio increase 0.3 (< 0.4) → blocked
- `TestDetectThrashing`: 4 events in 15-min window → isThrashing=true
- `TestDetectThrashing_NoThrashing`: 2 events → isThrashing=false
- `TestDetectThrashing_OldEventsIgnored`: 3 recent + 2 old (>15min) → counts only 3 recent
- `TestRecordRebalancing`: Verifies cooldown key TTL, history entry, last_ratio storage, metrics

**Rebalancer test (1 new test case):**
- `TestIncompleteRebalancing_HybridStrategy`: Validates escalation after 3 attempts
  - Attempts 0-2: 1 plan (proportional only, 2 channels migrated, 20% of 10)
  - Attempt 3: 2 plans (proportional + hot channel)
  - Proportional plan: 2 low-traffic channels (20% limit enforced)
  - Hot plan: 2 hot channels (ignores 20% limit)
  - Validates hot channels selected based on >3x average threshold

**All tests pass** with 43.6% coverage (up from 39.1% in previous plan).

## Deviations from Plan

None - plan executed exactly as written.

## Key Decisions Made

1. **Cooldown enforcement via Redis TTL:** Set cooldown key with TTL=5min for automatic expiration. No need for manual cleanup or background goroutines. Redis handles expiration atomically.

2. **Thrashing response strategy - Alert-only:** Per RESEARCH.md guidance, thrashing indicates pathological load or misconfigured HPA. Automated response (e.g., disabling rebalancing, increasing cooldown) may mask root cause. Strategy: log error, block rebalancing, let operators investigate via metrics and logs.

3. **Escalation override logic:** Compare `currentRatio - previousRatio > 0.4` to detect significant imbalance worsening. Example: ratio 0.6 → 1.1 (increase 0.5) triggers override, allowing immediate rebalancing despite cooldown. This prevents system from getting stuck in severe imbalance due to cooldown enforcement.

4. **Incomplete rebalancing counter semantics:** Counter increments after EVERY rebalancing, even if successful. Rationale: "incomplete" means imbalance persists after rebalancing, not that rebalancing itself failed. System becomes "complete" when `ShouldRebalance=false` (balanced state), which resets counter to 0.

5. **Hybrid strategy activation threshold:** 3 attempts chosen to balance responsiveness (don't wait too long) with stability (don't escalate prematurely). After 3 cycles (90 seconds with 30s reconciliation interval), persistent imbalance indicates proportional strategy alone is insufficient.

6. **Hot channel threshold (>3x average):** Uses per-pod average, not global average. Rationale: Overloaded pods have higher average rates than underutilized pods. Using pod average ensures hot channels are identified relative to their pod's baseline, not system-wide average.

7. **Hot channel max limit (1-2 per pod):** Limits hot channel migrations to 2 per pod to prevent excessive disruption. Moving 1-2 hot channels is often sufficient to restore balance when combined with proportional migrations.

8. **Graceful degradation on Redis errors:** Fail open (allow rebalancing) rather than fail closed (block rebalancing). Rationale: Rebalancing is a safety mechanism to prevent cascading failures. Blocking rebalancing due to Redis issues could cause worse problems (overloaded pods crash). Better to allow potentially excessive rebalancing than to prevent it entirely.

## Technical Highlights

### Cooldown Enforcement Flow

```go
// Check cooldown
allowed, reason, err := throttler.CheckCooldown(ctx, currentRatio)
// Possible outcomes:
// 1. No cooldown key → allowed=true, reason="ok"
// 2. Cooldown active, no escalation → allowed=false, reason="cooldown_active (remaining: Xs)"
// 3. Cooldown active, escalation override → allowed=true, reason="escalation_override"
// 4. Cooldown expired, no thrashing → allowed=true, reason="ok"
// 5. Cooldown expired, thrashing detected → allowed=false, reason="thrashing_detected"
```

### Thrashing Detection with Redis Sorted Set

**Data structure:**
```
rebalancing:history (sorted set):
  score=1771606002, member="rebalance-123"
  score=1771606122, member="rebalance-456"
  score=1771606242, member="rebalance-789"
```

**Query:**
```go
cutoff := time.Now().Add(-15 * time.Minute).Unix()
count, _ := redisClient.ZCount(ctx, "rebalancing:history", fmt.Sprintf("%d", cutoff), "+inf")
// Returns count of entries with timestamp >= cutoff (within 15-min window)
// O(log N) complexity via B-tree index
```

**Cleanup:**
```go
// Remove entries older than 15 minutes
cutoff := now.Add(-15 * time.Minute).Unix()
redisClient.ZRemRangeByScore(ctx, "rebalancing:history", "-inf", fmt.Sprintf("%d", cutoff))
// Executed in pipeline with RecordRebalancing for atomicity
```

### Escalation Override Example

**Scenario:** System experiencing rapid imbalance increase

1. **T=0s:** Imbalance detected (ratio=0.6), rebalancing triggered, cooldown starts
2. **T=120s:** Imbalance worsens (ratio=1.1), reconciliation cycle runs
3. **Cooldown check:**
   - Elapsed: 2 minutes (< 5 min cooldown)
   - Previous ratio: 0.6 (stored in Redis)
   - Current ratio: 1.1
   - Ratio increase: 1.1 - 0.6 = 0.5
   - Threshold: 0.4
   - Decision: 0.5 > 0.4 → Escalation override → allowed=true
4. **Result:** Rebalancing proceeds immediately despite cooldown, preventing cascading failure

### Incomplete Rebalancing Flow

**Normal flow (system balances quickly):**
1. Cycle 0: Imbalance detected (ratio=1.6) → rebalancing → counter=1
2. Cycle 1: Balanced (ratio=1.0) → no rebalancing → counter reset to 0

**Persistent imbalance flow (hybrid strategy activation):**
1. Cycle 0: Imbalance detected (ratio=1.6) → proportional rebalancing → counter=1
2. Cycle 1: Still imbalanced (ratio=1.5) → proportional rebalancing → counter=2
3. Cycle 2: Still imbalanced (ratio=1.4) → proportional rebalancing → counter=3
4. Cycle 3: Still imbalanced (ratio=1.3) → **hybrid strategy** (proportional + hot) → counter=4
5. Cycle 4: Balanced (ratio=1.0) → no rebalancing → counter reset to 0

**Why hybrid works:** Hot channels consume disproportionate resources. Proportional strategy avoids them (20% limit). After 3 failed attempts, system escalates to hot channel migration, accepting higher risk (connection disruption) to resolve persistent imbalance.

## Integration Points

### Phase 5 Sharding Infrastructure

Throttler uses Redis Sorted Set (same pattern as `shard:load` for load tracking):
- Efficient queries: `ZCOUNT`, `ZADD`, `ZREMRANGEBYSCORE`
- Automatic ordering by timestamp (score)
- Sliding window cleanup with single command

### Phase 6 Migration Infrastructure

Rebalancing continues using existing migration publisher:
- No changes to migration event format
- Throttler adds pre-check before triggering migrations
- Incomplete rebalancing counter influences migration planning

### Phase 7 Plan 01 (Load Monitoring)

Throttler receives `ImbalanceReport.ImbalanceRatio` for escalation override:
- Current ratio compared to previous ratio stored in Redis
- Significant increase (>0.4) triggers escalation
- Example: ratio 0.6 → 1.0 = increase 0.4 (exactly at threshold)

### Phase 7 Plan 02 (Proportional Redistribution)

Hybrid strategy extends proportional strategy:
- Proportional plans generated first (normal operation)
- Hot channel plans appended when `attemptCount >= 3`
- Both plan types execute via same `executeRebalancingPlans()` method

## Verification

### Build Verification

```bash
$ cd services/source-manager && go build ./cmd/main.go
# Success - no errors
```

### Test Results

```bash
$ go test ./coordination -v -cover
=== RUN   TestCheckCooldown_NoCooldown
--- PASS: TestCheckCooldown_NoCooldown (0.00s)
=== RUN   TestCheckCooldown_ActiveCooldown
--- PASS: TestCheckCooldown_ActiveCooldown (0.00s)
=== RUN   TestCheckCooldown_ExpiredCooldown
--- PASS: TestCheckCooldown_ExpiredCooldown (0.00s)
=== RUN   TestCheckCooldown_EscalationOverride
--- PASS: TestCheckCooldown_EscalationOverride (0.00s)
=== RUN   TestCheckCooldown_EscalationBelowThreshold
--- PASS: TestCheckCooldown_EscalationBelowThreshold (0.00s)
=== RUN   TestDetectThrashing
--- PASS: TestDetectThrashing (0.00s)
=== RUN   TestDetectThrashing_NoThrashing
--- PASS: TestDetectThrashing_NoThrashing (0.00s)
=== RUN   TestDetectThrashing_OldEventsIgnored
--- PASS: TestDetectThrashing_OldEventsIgnored (0.00s)
=== RUN   TestRecordRebalancing
--- PASS: TestRecordRebalancing (0.00s)
=== RUN   TestIncompleteRebalancing_HybridStrategy
--- PASS: TestIncompleteRebalancing_HybridStrategy (0.00s)
PASS
coverage: 43.6% of statements
ok  	github.com/caesar/all-chat/services/source-manager/coordination	4.181s	coverage: 43.6% of statements
```

**Coverage increased from 39.1% to 43.6%** with comprehensive throttler and hybrid strategy tests.

## Success Criteria Met

- [x] Cooldown enforces 5-minute wait between rebalancing operations
- [x] Thrashing detection identifies >3 rebalances in 15-minute window and blocks further operations
- [x] Escalation override allows breaking cooldown when imbalance ratio increases by >0.4
- [x] Incomplete rebalancing counter tracks persistent imbalance, escalates to hybrid strategy after 3 attempts
- [x] Redis Sorted Set automatically cleans up old history entries (>15 minutes)
- [x] Metrics expose rebalancing cycles, cooldown overrides, thrashing events
- [x] Unit tests validate cooldown, thrashing, escalation with comprehensive coverage
- [x] Thrashing response: Alert-only (log error, enforce cooldown, let operators investigate)

## Next Steps

**Plan 04** will implement:
- HPA coordination via Redis distributed locks (`rebalancing:lock` key)
- Scale-up interaction: abort current rebalancing when HPA triggers
- Scale-down handling: proactive migration before pod termination
- Staggered pod startup: random delay (0-30s) before querying coordinator
- Scale event detection using Kubernetes Watch API

**Deployment considerations:**
- Monitor `shard_rebalancing_total` metric for rebalancing frequency
- Alert on `shard_rebalancing_thrashing_total` (indicates pathological load or HPA misconfiguration)
- Track `shard_rebalancing_cooldown_overrides_total` (frequent overrides may indicate cooldown duration needs tuning)
- Tune cooldown duration (5 min) based on production load patterns
- Tune escalation threshold (0.4) if system experiences false positives (too aggressive) or misses escalation (too conservative)

**Tuning guidance:**
- **Cooldown too short:** Excessive rebalancing, high migration overhead, resource exhaustion
- **Cooldown too long:** System stuck in imbalance, overloaded pods may crash before next rebalancing
- **Escalation threshold too low:** Frequent cooldown overrides, defeats cooldown purpose
- **Escalation threshold too high:** Misses severe imbalance worsening, system remains stuck
- **Incomplete attempt threshold too low:** Premature hot channel migration, unnecessary disruption
- **Incomplete attempt threshold too high:** Persistent imbalance for too long, overloaded pods may fail

## Self-Check: PASSED

**Files created:**
```bash
$ ls -la services/source-manager/coordination/throttler*
-rw-r--r-- 1 caesar caesar  6834 Feb 20 16:51 services/source-manager/coordination/throttler.go
-rw-r--r-- 1 caesar caesar 10247 Feb 20 16:52 services/source-manager/coordination/throttler_test.go
```

**Files modified:**
```bash
$ git diff --stat HEAD~3
 services/source-manager/cmd/main.go                     |   4 +
 services/source-manager/coordination/coordinator.go     |  48 +++++--
 services/source-manager/coordination/rebalancer.go      |  94 +++++++++++-
 services/source-manager/coordination/rebalancer_test.go | 115 +++++++++++++++-
 services/source-manager/coordination/throttler.go       | 238 +++++++++++++++++++++++++++++++
 services/source-manager/coordination/throttler_test.go  | 312 +++++++++++++++++++++++++++++++++++++++++
 shared/metrics/shard_metrics.go                         |  12 ++
 7 files changed, 808 insertions(+), 15 deletions(-)
```

**Commits exist:**
```bash
$ git log --oneline -3
b40169f test(07-03): add unit test for incomplete rebalancing hybrid strategy
72f79d4 feat(07-03): integrate throttler with coordinator and track incomplete rebalancing
301ea78 feat(07-03): implement Throttler with cooldown and thrashing detection
```

**Build succeeds:**
```bash
$ go build ./services/source-manager/cmd/main.go
# Exit code 0
```

**Tests pass:**
```bash
$ go test ./coordination -v -cover
# All tests pass, coverage: 43.6%
```

## Commits

- `301ea78`: feat(07-03): implement Throttler with cooldown and thrashing detection
- `72f79d4`: feat(07-03): integrate throttler with coordinator and track incomplete rebalancing
- `b40169f`: test(07-03): add unit test for incomplete rebalancing hybrid strategy
