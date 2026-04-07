# Phase 10: Message Pipeline Resilience — Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-07
**Phase:** 10-message-pipeline-resilience-fix-silent-failure-modes-across-
**Areas discussed:** Scope & priority, Health probe isolation, Redis reconnection strategy, Message durability, Zombie listener detection

---

## Scope & Priority

| Option | Description | Selected |
|--------|-------------|----------|
| High + Medium only | Fix F-01, F-02, F-05, F-06, F-08, F-09, F-10 (7 items). Skip Low-severity. | |
| All 12 failure modes | Fix everything found in the audit. More thorough but larger scope. | ✓ |
| Just the outage pattern | Only F-01, F-05, F-08. Minimal scope. | |

**User's choice:** All 12 failure modes
**Notes:** User wants comprehensive hardening, not just targeted fixes.

---

## Health Probe Isolation

| Option | Description | Selected |
|--------|-------------|----------|
| Atomic flags | Replace RLock-based checks with atomic.Bool/atomic.Int64. Zero lock contention. | ✓ |
| Separate goroutine with cached state | Background goroutine snapshots health state into lockfree cache. | |
| You decide | Claude picks based on codebase patterns. | |

**User's choice:** Atomic flags
**Notes:** Simple, proven Go pattern. Directly addresses the root cause of the April 7 outage.

---

## Redis Reconnection Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Exponential backoff with cap | 1s → 2s → 4s → ... cap 30s, plus jitter. Infinite retries until context cancelled. | ✓ |
| Circuit breaker | Stop trying after N failures, cooldown, half-open probe. | |
| Fixed retry with longer limit | 10 attempts with 2s intervals. | |

**User's choice:** Exponential backoff with cap
**Notes:** Standard cloud-native pattern. Applied to all reconnection paths.

---

## Message Durability

| Option | Description | Selected |
|--------|-------------|----------|
| Drop + structured alert | Accept ephemeral data. Drop with structured log for Grafana alerts. | ✓ |
| Disk-backed overflow buffer | Spill to local file. Drain on recovery. | |
| Backpressure to listener | Pause consuming from IRC when buffer fills. | |

**User's choice:** Drop + structured alert
**Notes:** Chat messages are ephemeral — complexity of disk buffering not warranted.

---

## Zombie Listener Detection

| Option | Description | Selected |
|--------|-------------|----------|
| Received-vs-published drift | Track two atomic counters. If received > 0 but published stalls, liveness fails. | ✓ |
| Per-channel last-published timestamp | Track per-channel staleness. More granular but more state. | |
| You decide | Claude picks based on codebase patterns. | |

**User's choice:** Received-vs-published drift
**Notes:** User identified that a simple message-rate canary would false-positive on offline Twitch channels (which remain "active" sources). The drift approach avoids this because both counters are 0 when no messages are received.

---

## Claude's Discretion

- Exact backoff jitter implementation
- Zombie detection threshold values
- Shared retry utility vs per-service logic
- PEL drain interval tuning
- Structured log event format

## Deferred Ideas

None
