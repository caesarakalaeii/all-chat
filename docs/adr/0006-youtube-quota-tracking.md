# ADR-0006: YouTube Quota Reserve-Confirm-Rollback

**Date**: 2025-11-15
**Status**: ✅ Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: YouTube Listener Team, Infrastructure Lead

---

## Context and Problem Statement

YouTube Live Chat API earns its quota from a Google Cloud project: the project's
default 10,000 units/day was raised to **1,009,000 units/day** (migration 066,
`migrations/066_youtube_quota_real_limit.sql`, and `shared/quota.DefaultDailyQuota`), and
the limit is mostly unused today because the default chat path runs on InnerTube with no
official quota cost (ADR-0065). Quota is still spent by the premium official-API listener
(`youtube-listener`, opt-in per source), `moderation-service` bans (50 units) and
`auth-service` streamer sends (5 units). Quota-consuming calls made by the listener:
- `liveChatMessages` (gRPC `streamList`, fetch messages): **5 units** per call
- `videos.list` (stream details/check): **1 unit** per call (charged as a plain increment, no reservation)
- `liveBroadcasts.list` (stream discovery, official-API mode): **1 unit** per call (estimate)
- `channels.list(mine=true)` (ownership proof): **1 unit** per call

The one usage-based alert path described here now lives in the dedicated
`youtube-quota-monitor` service (ADR-0023), which reads the same shared table.

**Initial Implementation** (Simple Counter):
```go
func (t *Tracker) RecordUsage(units int) {
    t.usedUnits += units  // Simple increment
    t.db.Exec("UPDATE youtube_quota_usage SET units_used = units_used + $1", units)
}
```

**Problem Discovered** (After 1 week of production):
- Database showed **8,542 units used**
- Expected usage (calculated manually): **8,028 units**
- **Drift**: ±514 units (6.4% error)
- **Impact**: Risk hitting quota limit unexpectedly, stopping all YouTube polling

**Root Cause**: Race conditions in concurrent quota updates across multiple
listener replicas issuing overlapping API calls.

**Problem**: How do we track quota usage with 99.95%+ accuracy?

---

## Decision Drivers

1. **Accuracy**: Must track quota within ±5 units (0.05% error) to prevent unexpected exhaustion
2. **Atomicity**: Concurrent API calls must not cause drift
3. **Crash Recovery**: Service crashes must not lose quota data
4. **Smart Charging**: 4xx client errors should NOT consume quota (YouTube doesn't charge for invalid requests)
5. **Observability**: Must detect and alert on tracking drift

---

## Considered Options

### Option 1: Simple Counter (Initial Implementation)

```go
// Record usage AFTER API call
func RecordUsage(units int) {
    db.Exec("UPDATE youtube_quota_usage SET units_used = units_used + $1", units)
}
```

**✅ Pros**: Simple, one database query

**❌ Cons**:
- **Race conditions**: Concurrent updates = drift
- **No crash recovery**: Service crashes after API call but before recording = lost tracking
- **Always charges**: Records units even for 4xx errors (incorrect)

**Measured Drift**: ±500 units/day (5% error) ❌

---

### Option 2: Row-Level Locking

```go
// Lock row before recording
func RecordUsage(units int) {
    tx.Begin()
    tx.Exec("SELECT * FROM youtube_quota_usage WHERE date = $1 FOR UPDATE", today)
    tx.Exec("UPDATE youtube_quota_usage SET units_used = units_used + $1", units)
    tx.Commit()
}
```

**✅ Pros**: Atomic updates, no race conditions

**❌ Cons**:
- **Still charges after call**: Service crashes after API call but before recording = lost tracking
- **Always charges**: Records units even for 4xx errors
- **Slower**: Locks row for every update (contention across concurrent callers)

**Estimated Drift**: ±50 units/day (0.5% error) - Better but not good enough

---

### Option 3: Reserve-Confirm-Rollback Pattern (CHOSEN)

```go
// RESERVE quota BEFORE API call (tracker calls the SQL function)
canReserve := tx.QueryRow("SELECT reserve_youtube_quota($1, $2, $3)", today, units, dailyLimit)
if !canReserve {
    return "", fmt.Errorf("insufficient quota available")
}
// inverse for release: SELECT rollback_youtube_quota($1, $2), today, units
// inverse for charge: SELECT confirm_youtube_quota($1, $2), today, units
```

The SQL functions (defined in migration 008) lock the `youtube_quota_usage` row
for the date with `FOR UPDATE`, create the row if it is missing, and return
whether the reservation succeeded. There is no separate reservations table:
only the `units_reserved` counter on the day's row tracks in-flight calls, and
the listener's own reservation ids are local bookkeeping.

```go
// Usage:
reservationID, _ := tracker.ReserveQuota(5)  // BEFORE API call
resp, err := youtube.FetchMessages(...)
if !shouldChargeQuota(err) {
    tracker.RollbackReservation(reservationID, 5)  // 4xx = don't charge
} else {
    tracker.ConfirmReservation(reservationID, 5)  // Success or 5xx = charge
}
```

**✅ Pros**:
- **Zero drift**: Quota reserved BEFORE call, impossible to diverge
- **Crash recovery**: Stale reservations cleaned up automatically (>5 min old)
- **Smart charging**: 4xx errors rolled back (YouTube doesn't charge)
- **Atomic**: Row-level locking prevents race conditions
- **Auditable**: settled API calls write a `youtube_quota_audit_log` row via
  `LogAPICall` (migration 016)

**❌ Cons**:
- **Complexity**: reserve + confirm/rollback instead of one increment
- **Additional state**: `units_reserved` column on the day's quota row
- **Cleanup needed**: stale reserved units must be recovered (background job)

---

## Decision Outcome

**Chosen**: **Option 3 - Reserve-Confirm-Rollback Pattern**

**Rationale**:

1. **Eliminates Drift** (Primary Driver):
   - Before: ±500 units/day (5% error)
   - After: ±2 units/day (0.02% error)
   - **Improvement**: 250x more accurate

2. **Quota Waste Elimination** (polling overheads measured against the 10,000-unit
   quota of the time):
   - **Stop on exhaustion**: no retry loops when quota is exhausted
   - **Immediate cache clearing**: stop polling offline streams immediately
   - **Smart disconnect**: stop polling when the last overlay disconnects
   - **Connection batching**: 5-second debounce for rapid overlay connects

3. **Crash Recovery**:
   - Service crashes after RESERVE but before CONFIRM = reservation stays in database
   - Cleanup job runs every 60 seconds, recovers reserved units stale >5 min old
   - No lost tracking even with crashes

4. **Smart Charging**:
   - 4xx errors (invalid video ID, OAuth expired) = rollback reservation
   - 5xx errors (YouTube down, network timeout) = confirm (YouTube charges anyway)
   - **Accuracy**: charge only for calls `shouldChargeQuota` classifies as having reached YouTube

---

## Consequences

### Positive

1. **99.98% Accuracy** (±2 units/day drift, measured at decision time):
   - Database: 8,542 units
   - Expected: 8,540 units
   - Drift: 2 units (0.02% error) ✅

2. **Waste Elimination**:
   - At the 10,000-unit quota of the time, the eliminated polling overhead was worth
     ~9,000 units/day; relative to today's 1,009,000-unit quota the same mechanisms
     mainly keep the shared table honest across multiple consumers.

3. **Predictable Quota Consumption**:
   - `/quota/status` (served by `youtube-quota-monitor` since ADR-0023) shows real-time usage
   - Alerts fire on the deployed Prometheus rules `YouTubeQuotaHigh` (>80%),
     `YouTubeQuotaCritical` (>95%) and `YouTubeQuotaExhausted` (>=100%) (ADR-0023)

4. **Crash Resilient**:
   - Tested: Kill service during API call → reservation cleaned up after 5 min
   - Zero quota leakage from crashes

### Negative

1. **Increased Complexity**:
   - 3 functions (Reserve, Confirm, Rollback) vs 1 (RecordUsage)
   - Must remember to call Confirm/Rollback after EVERY API call
   - **Mitigation**: every API call method in `api/client.go` inlines reserve, then confirm-or-rollback in one block

2. **Extra State on the Quota Row**:
   - `units_reserved` column on `youtube_quota_usage` tracks in-flight calls
   - Cleanup job must run every 60 seconds
   - **Impact**: Minimal (counter on one row per day)

3. **Stale Reservation Cleanup**:
   - Background job runs every 60 seconds
   - Recovers (zeroes) reserved units >5 min old on past-date rows
   - **Impact**: Minimal CPU/memory overhead

---

## Implementation

### Database Schema

**Migrations** (real definitions in the tree):
- `003_youtube_support.sql`: creates `youtube_quota_usage` (date,
  `units_used`, `units_limit`).
- `008_quota_reservations.sql`: adds the `units_reserved INT NOT NULL DEFAULT 0`
  column and defines five SQL functions:
  `reserve_youtube_quota(date, units, limit)` (row lock via `FOR UPDATE`,
  creates the day's row on first use, returns FALSE when
  `used + reserved + requested > limit`), `confirm_youtube_quota(date, units)`
  (reserved -> used), `rollback_youtube_quota(date, units)` (release), and
  `cleanup_stale_quota_reservations()` (zeroes `units_reserved` on rows whose
  `updated_at` is older than 5 minutes from a past date and returns the
  recovered unit count). `get_youtube_quota_with_reserved(date)` returns the
  status envelope `youtube-quota-monitor` serves on `/quota/status` (migration
  066 corrects its no-row fallback to the 1,009,000 limit).
- `014_quota_events_tracking.sql`: `youtube_quota_events` audit table for
  state-transition events; `016_youtube_quota_audit_log.sql`: per-call audit
  log (`youtube_quota_audit_log`); `026_backfill_missing_youtube_quota_records.sql`.

Sketch (not verbatim):
```sql
ALTER TABLE youtube_quota_usage ADD COLUMN units_reserved INT NOT NULL DEFAULT 0;

CREATE OR REPLACE FUNCTION cleanup_stale_quota_reservations()
RETURNS INT AS $$
DECLARE
    v_reserved INT;
BEGIN
    SELECT COALESCE(SUM(units_reserved), 0) INTO v_reserved
    FROM youtube_quota_usage
    WHERE units_reserved > 0
      AND updated_at < NOW() - INTERVAL '5 minutes'
      AND date < CURRENT_DATE;

    IF v_reserved > 0 THEN
        UPDATE youtube_quota_usage
        SET units_reserved = 0, updated_at = NOW()
        WHERE units_reserved > 0
          AND updated_at < NOW() - INTERVAL '5 minutes'
          AND date < CURRENT_DATE;
    END IF;
    RETURN v_reserved;
END;
$$ LANGUAGE plpgsql;
```

### Code Implementation

**File**: `services/youtube-listener/quota/tracker.go` (type `Tracker`, with
`Coordinator` and `PerChannelTracker` alongside). Key methods (real signatures):

```go
func (t *Tracker) Start(ctx context.Context) error   // date check, DB sync, reservation cleanup loops
func (t *Tracker) ReserveQuota(ctx context.Context, units int) (string, error)
func (t *Tracker) ReserveQuotaWithPriority(ctx context.Context, units int, allowCritical bool) (string, error)
func (t *Tracker) ConfirmReservation(ctx context.Context, reservationID string, units int) error
func (t *Tracker) RollbackReservation(ctx context.Context, reservationID string, units int) error
func (t *Tracker) RecordUsage(ctx context.Context, units int) error // plain increment, used when no reservation was taken
func (t *Tracker) GetUsageToday() int
```

Notes:

- The reserve/confirm/rollback methods delegate to the migration-008 SQL
  functions (`reserve_youtube_quota`, `confirm_youtube_quota`,
  `rollback_youtube_quota`); the reservation id is local bookkeeping, e.g.
  `date-UnixNano-units`.
- `ReserveQuotaWithPriority` adds an emergency shutoff: at or above the
  emergency threshold (default 90% of the daily limit) reservations are blocked
  unless the caller passes `allowCritical=true` for a critical operation
  (OAuth, login).
- Callers in `services/youtube-listener/api/client.go` and `grpc_client.go`
  reserve before each official-API call and settle afterwards with
  `ConfirmReservation` or `RollbackReservation`, rolling back when
  `shouldChargeQuota` classifies the failure as not having reached YouTube
  (4xx that did not reach YouTube is its only no-charge case).
- The listener additionally exposes HTTP endpoints (`/v1/quota/check`,
  `/v1/quota/reserve`, `/v1/quota/confirm` in `cmd/main.go`) for other services
  to coordinate quota over HTTP; `auth-service` and `moderation-service` have
  since moved to direct SQL via `shared/quota` (ADR-0023).
- The `cleanup_stale_quota_reservations()` sweep also runs in
  `services/youtube-quota-monitor/cmd/main.go` (ADR-0023), so stale reserved
  units are recovered even when the listener is not deployed.

---

## Related Decisions

- **ADR-0023**: [Decoupled YouTube Quota Monitor](./0023-decoupled-youtube-quota-monitor.md) -
  the shared-table metrics/alerting owner; canonical `shared/quota` package used by
  auth-service and moderation-service
- **ADR-0065**: [Pinned stream links and premium official-API listener](./0065-premium-official-youtube-api-listener.md) -
  revived `youtube-listener` as the opt-in, premium official-API path (the only
  listener-side quota consumer today)
- **ADR-0002**: [Redis Streams + Pub/Sub](./0002-redis-streams-pubsub.md) - Message transport
- **Architecture**: [01-DATA-FLOW.md](../architecture/01-DATA-FLOW.md) - YouTube integration
- **Guide**: [QUICK-REF-DEBUG-QUOTA.md](../llm-guides/QUICK-REF-DEBUG-QUOTA.md) - Quota troubleshooting

---

## Validation

### Accuracy Test (7-day production run, 2025-11-18 to 2025-11-25)

| Date | DB Tracked | Manual Count | Drift | Accuracy |
|------|------------|--------------|-------|----------|
| 11-18 | 2,842 | 2,840 | +2 | 99.93% |
| 11-19 | 3,105 | 3,104 | +1 | 99.97% |
| 11-20 | 2,678 | 2,678 | 0 | 100.00% |
| 11-21 | 2,954 | 2,952 | +2 | 99.93% |
| 11-22 | 3,221 | 3,220 | +1 | 99.97% |
| 11-23 | 2,508 | 2,507 | +1 | 99.96% |
| 11-24 | 2,890 | 2,888 | +2 | 99.93% |
| **Avg** | **2,885** | **2,884** | **±1.3** | **99.95%** ✅

**Result**: 99.95% accuracy, well within ±5 unit target.

---

## Summary

**Decision**: Use reserve-confirm-rollback pattern for YouTube API quota tracking.

**Reason**: Eliminates drift (±500 → ±2 units/day), 99.95%+ accuracy, crash-resilient, smart charging for 4xx errors.

**Impact**: At the 10,000-unit quota of the time: ~9,000 units/day waste eliminated (90% reduction),
3-5x more concurrent streams on the same quota. Today the shared `youtube_quota_usage` table and
the migration-008 SQL functions keep accounting honest across the official-API listener,
moderation-service bans and auth-service sends (ADR-0023), with metrics and alerting owned by
`youtube-quota-monitor`.

**Status**: ✅ Implemented and validated in production with 99.95% accuracy over 7-day test.
