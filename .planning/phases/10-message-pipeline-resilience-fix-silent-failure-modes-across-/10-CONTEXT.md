# Phase 10: Message Pipeline Resilience — Fix Silent Failure Modes Across Twitch Message Pipeline - Context

**Gathered:** 2026-04-07
**Status:** Ready for planning

<domain>
## Phase Boundary

Harden the Twitch message pipeline against silent failure modes identified in a production resilience audit. Fix all 12 failure modes (F-01 through F-12) across twitch-listener, message-processor, api-gateway, source-manager, and shared Redis client code. Add zombie listener detection. No new features — purely defensive hardening of the existing message flow.

</domain>

<decisions>
## Implementation Decisions

### Scope & Priority
- **D-01:** Fix all 12 identified failure modes (F-01 through F-12), not just High/Medium severity. Full sweep.
- **D-02:** Add zombie listener detection as a 13th item — this was the pattern behind the April 7 outage and a recurring issue.

### Health Probe Isolation
- **D-03:** Replace RLock-based health probe checks with `atomic.Bool`/`atomic.Int64` flags. Zero lock contention — probes must never block on business logic mutexes.
- **D-04:** Applies to all services where health probe handlers currently acquire any mutex (twitch-listener `verifyCoverageComplete` holds RLock during Redis SCAN, `GetActiveChannelCount`, `IsInitialSyncComplete`).

### Redis Reconnection Strategy
- **D-05:** All Redis reconnection paths must use exponential backoff with cap (1s → 2s → 4s → ... cap 30s) plus jitter. Infinite retries until context cancelled.
- **D-06:** Specifically fix: api-gateway `Subscriber.resubscribe` (currently single-attempt), `StatusSubscriber.reconnect` (currently 3 attempts then permanent give-up), message-processor `XReadGroup` error loop (currently flat 1s sleep).

### Message Durability
- **D-07:** Accept that chat messages are ephemeral. When ring buffer fills or DLQ write fails, drop the message but emit a structured log event (not just a metric counter) that can trigger Grafana alerts.
- **D-08:** No disk-backed buffering, no backpressure to listeners. Keep the architecture simple for recoverable data.

### Zombie Listener Detection
- **D-09:** Track received-vs-published message drift. Two atomic counters: `messages_received` (incremented in IRC callback) and `messages_published` (incremented on confirmed XADD/ring buffer accept). If received > 0 but published stalls for N minutes, liveness probe fails → Kubernetes kills and restarts the pod.
- **D-10:** This avoids false positives on offline channels — when a streamer is offline, both received and published are 0, so no drift is detected.

### Claude's Discretion
- Exact backoff jitter implementation (full jitter vs equal jitter)
- Threshold values for zombie detection (N minutes stall window)
- Whether to use shared/retry utility or per-service retry logic
- PEL drain interval and idle threshold tuning
- Structured log event format for message drops

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Message pipeline architecture
- `docs/architecture/01-DATA-FLOW.md` — Full message flow from listener to overlay, stream/pub/sub topology
- `docs/architecture/00-OVERVIEW.md` — Service map and dependencies

### Debug session evidence (root cause analysis)
- `.planning/debug/resolved/twitch-messages-not-reaching-overlay.md` — April 7 outage: SyncChannels mutex blocked health probes, pod cycling
- `.planning/debug/resolved/twitch-messages-not-reaching-overlay-hesplayingroblox.md` — March 29: same race condition pattern, stale PEL entries

### ADRs
- `docs/adr/README.md` — Architecture Decision Records index
- `docs/adr/0002-redis-streams-pubsub-hybrid.md` — Why Redis Streams + Pub/Sub hybrid (affects retry/reconnect design)

### Codebase maps
- `.planning/codebase/ARCHITECTURE.md` — Service layers, data flow, dependencies
- `.planning/codebase/CONVENTIONS.md` — Go patterns, error handling conventions

</canonical_refs>

<code_context>
## Existing Code Insights

### Failure Modes Identified (Full Audit)

| # | Service | Failure Mode | Severity |
|---|---------|-------------|----------|
| F-01 | twitch-listener | `verifyCoverageComplete` holds `m.mu.RLock()` during Redis SCAN — blocks health probes | Medium |
| F-02 | twitch-listener | Ring buffer silently drops messages (metric only, no structured alert) | Medium |
| F-03 | twitch-listener | `status.Publisher.Publish` has zero retry — status updates lost on Redis hiccup | Low |
| F-04 | message-processor | `XReadGroup` error loop uses flat 1s sleep, not exponential backoff | Low |
| F-05 | message-processor | `writeToDLQ` is best-effort with no retry — messages silently lost | High |
| F-06 | message-processor | PEL orphan window: 5-minute idle threshold before reclaim after pod restart | Medium |
| F-07 | message-processor | Consumer group created with `"$"` — pre-existing stream messages skipped | Low |
| F-08 | api-gateway | `Subscriber.resubscribe` single attempt — overlay permanently loses subscription | High |
| F-09 | api-gateway | `StatusSubscriber.reconnect` gives up after 3 attempts — permanent loss | Medium |
| F-10 | api-gateway | twitch-listener and api-gateway bypass shared Redis client resilience settings | Medium |
| F-11 | source-manager | `RenewLeadership` non-atomic GET+Expire — TOCTOU race window | Low |
| F-12 | source-manager | `RegisterPeer` scans all peer keys on every call — slow under high pod counts | Low |

### Reusable Assets
- `shared/redis/client.go` — `NewClientWithTracing` already has MaxRetries=3, pool tuning. Extend for all services.
- `services/twitch-listener/channels/ring_buffer.go` — Ring buffer publisher with retry goroutine. Needs structured alerting on drops.
- `services/message-processor/consumer/retry.go` — `retryOp` with 3 attempts. Needs exponential backoff + jitter.
- `services/twitch-listener/channels/manager.go` — SyncChannels already fixed for write-lock; read-lock paths still need atomic flag migration.

### Established Patterns
- Graceful shutdown with 25s timeout across all services
- Health checks: `/health/live` (always 200), `/health/ready` (checks DB + Redis)
- `os.Hostname()` used as consumer name in message-processor
- Leader election via Redis SetNX with 10s TTL

### Integration Points
- All Redis reconnection changes affect: twitch-listener, message-processor, api-gateway
- Atomic flag migration touches health probe handlers in twitch-listener
- Shared Redis client changes propagate to all services using `shared/redis`
- Zombie detection adds new counters to twitch-listener IRC callback and publisher paths

</code_context>

<specifics>
## Specific Ideas

- "Zombie pollers" is a recurring pattern — the April 7 outage and the March 29 hesplayingroblox issue were both caused by listeners that appeared alive but weren't delivering messages.
- The received-vs-published drift approach was specifically chosen to avoid false positives on offline Twitch channels (which remain "active" sources in the database even when the streamer isn't live).

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 10-message-pipeline-resilience-fix-silent-failure-modes-across-*
*Context gathered: 2026-04-07*
