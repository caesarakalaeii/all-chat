# Phase 8: Message Pipeline Resilience — Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-29
**Phase:** 08-message-pipeline-resilience-fix-silent-failure-modes-across-twitch-message-pipeline
**Areas discussed:** Scope & priority, Recovery strategy, Rollout approach, Observability depth, DLQ lifecycle, Consumer naming, Listener publish failures

---

## Scope & Priority

| Option | Description | Selected |
|--------|-------------|----------|
| Data-loss fixes only | Focus on ~8 fixes that cause silent message drops | |
| All 24 failure modes | Fix everything in one phase | ✓ |
| Critical path only | Fix only the 3-4 most impactful | |

**User's choice:** All 24 failure modes
**Notes:** None

| Option | Description | Selected |
|--------|-------------|----------|
| Group by service | Each plan targets one service | ✓ |
| Group by severity | Critical fixes across all services first | |

**User's choice:** Group by service
**Notes:** Easier to test, review, and deploy independently

---

## Recovery Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| Retry with backoff + DLQ | Exponential backoff (3 retries), then dead-letter stream | ✓ |
| Retry with backoff + log | Exponential backoff, then log at ERROR level | |
| Simple retry + skip | One retry, then skip | |

**User's choice:** Retry with backoff + DLQ
**Notes:** No message silently dropped

| Option | Description | Selected |
|--------|-------------|----------|
| Per-subscriber reconnect | Each subscriber detects closure and re-subscribes | ✓ |
| Shared reconnect wrapper | Build shared/redis wrapper for all subscribers | |

**User's choice:** Per-subscriber reconnect
**Notes:** go-redis already handles TCP reconnect; application layer handles re-subscription

---

## Rollout Approach

| Option | Description | Selected |
|--------|-------------|----------|
| Service-by-service, commit & push | Fix one service, deploy, validate, next | ✓ |
| All at once | Fix all services, deploy together | |
| Feature-flagged | Deploy behind feature gates | |

**User's choice:** Service-by-service, commit & push
**Notes:** Natural rollback per service via git revert

---

## Observability Depth

| Option | Description | Selected |
|--------|-------------|----------|
| Metric per fix + alert rules | Targeted metrics and Prometheus alerts per fix | ✓ |
| Metrics only, no new alerts | Add metrics, rely on Phase 4 alerts | |
| Minimal — log only | Log at ERROR level, no new metrics | |

**User's choice:** Metric per fix + alert rules
**Notes:** Extends Phase 4 dashboards

| Option | Description | Selected |
|--------|-------------|----------|
| Dashboard panel + alert | DLQ panel on Pipeline dashboard + alert | ✓ |
| Alert only | Just alert when DLQ has messages | |

**User's choice:** Dashboard panel + alert
**Notes:** Alert when DLQ depth > 0 for 5 minutes

---

## DLQ Lifecycle

| Option | Description | Selected |
|--------|-------------|----------|
| 7-day TTL + manual replay | Auto-trim after 7 days, admin endpoint to replay | ✓ |
| Inspect-only, no replay | DLQ for investigation only, no replay mechanism | |
| Auto-retry with delay | DLQ consumer auto-retries after 5 minutes | |

**User's choice:** 7-day TTL + manual replay
**Notes:** Replay pushes messages back to chat:raw for reprocessing

---

## Consumer Naming

| Option | Description | Selected |
|--------|-------------|----------|
| Hostname/pod name | os.Hostname() maps to K8s pod name | ✓ |
| UUID per startup | Generate UUID on each startup | |
| Env var override | Default hostname, CONSUMER_NAME env var for dev | |

**User's choice:** Hostname/pod name
**Notes:** Stable across restarts, unique across replicas, PEL entries tie to specific pod

---

## Listener Publish Failures

| Option | Description | Selected |
|--------|-------------|----------|
| In-memory ring buffer + retry | Buffer 1000 messages, retry every 500ms | ✓ |
| Retry 3x then drop | Retry XADD 3 times with backoff, then drop | |
| You decide | Claude picks approach | |

**User's choice:** In-memory ring buffer + retry
**Notes:** Implemented as shared/listener SDK method, all Go listeners get it. Drop oldest when buffer full.

---

## Claude's Discretion

- PEL drain strategy (XAUTOCLAIM vs XPENDING+XCLAIM)
- Ring buffer implementation details
- Exact grouping of 24 failure modes into plans
- Service fix ordering within the phase

## Deferred Ideas

None
