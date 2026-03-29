# Phase 8: Message Pipeline Resilience — Fix silent failure modes across Twitch message pipeline - Context

**Gathered:** 2026-03-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Fix all 24 silent failure modes identified in the robustness audit across the entire message pipeline: Listeners → Redis Streams → Message Processor → Redis Pub/Sub → API Gateway WebSocket. Every fix must eliminate a path where messages are silently dropped without logging or alerting.

</domain>

<decisions>
## Implementation Decisions

### Scope & Priority
- **D-01:** All 24 failure modes are in scope for this phase — no deferral of edge cases
- **D-02:** Fixes grouped by service (all message-processor fixes together, all API gateway fixes together, etc.) — not by severity across services
- **D-03:** Each service's fixes form a natural plan boundary for independent testing and deployment

### Recovery Strategy
- **D-04:** Failed Redis operations use exponential backoff retry (3 attempts: 100ms, 500ms, 2s) then dead-letter to a DLQ Redis stream (`chat:dlq`) for investigation
- **D-05:** No message may be silently dropped — every failure path must either retry successfully or land in DLQ with full context
- **D-06:** Pub/Sub reconnect handled per-subscriber (Subscriber, StatusSubscriber each detect channel closure and re-subscribe independently). go-redis handles TCP-level reconnect; application layer handles re-subscription
- **D-07:** Listener XADD failures buffered in an in-memory ring buffer (capacity: 1000 messages) with retry every 500ms. When buffer full, drop oldest message. Implemented as shared/listener SDK method so all Go listeners get it

### Consumer Naming
- **D-08:** Message-processor consumer names use `os.Hostname()` (maps to K8s pod name). Unique per replica, stable across restarts of same pod. PEL entries tie to specific pod for correct ownership tracking

### DLQ Lifecycle
- **D-09:** DLQ stream auto-trimmed via XTRIM MINID for messages older than 7 days
- **D-10:** Admin endpoint or CLI command to replay DLQ messages back to `chat:raw` for reprocessing
- **D-11:** DLQ messages include original stream ID, source service, failure reason, and retry count as fields

### Rollout Approach
- **D-12:** Deploy service-by-service via existing CI/CD (commit, push, Keel deploys). Validate each service before moving to the next
- **D-13:** No feature flags for these infrastructure fixes — direct deployment with rollback via git revert if needed

### Observability
- **D-14:** Each fix adds targeted Prometheus metrics: `pel_pending_messages`, `pubsub_reconnect_total`, `dlq_messages_total`, `publish_retry_total`, `ring_buffer_depth`, `ring_buffer_drops_total`
- **D-15:** Matching Prometheus alert rules for each new metric (extends Phase 4 alert groups)
- **D-16:** DLQ gets its own Grafana dashboard panel on the Pipeline dashboard showing message count, age distribution, and source service breakdown
- **D-17:** Alert when DLQ depth > 0 for 5 minutes

### Claude's Discretion
- Specific PEL drain strategy (XAUTOCLAIM vs XPENDING+XCLAIM) — researcher/planner decides based on Redis version
- Ring buffer implementation details (channel-based vs mutex-protected slice)
- Exact grouping of the 24 failure modes into service-scoped plans
- Order of service fixes within the phase (which service first)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Message Pipeline Architecture
- `docs/architecture/01-DATA-FLOW.md` — Full message flow from listeners to frontend
- `docs/architecture/04-OBSERVABILITY.md` — Existing metrics and monitoring patterns

### Pipeline Services (read current implementation)
- `services/message-processor/consumer/stream_consumer.go` — Stream consumer with hardcoded ConsumerName, no PEL drain, ACK handling
- `services/api-gateway/subscription/subscriber.go` — Overlay Pub/Sub subscriber with no reconnect logic
- `services/api-gateway/subscription/status_subscriber.go` — Status subscriber with nil channel panic risk
- `services/api-gateway/handlers/websocket.go` — WebSocket handler
- `services/message-processor/cmd/main.go` — Message processor entry point

### Shared Infrastructure
- `shared/listener/` — Listener SDK (ring buffer addition goes here)
- `shared/metrics/` — Existing metrics package (new metrics extend this)

### Phase 4 Observability (extend, don't duplicate)
- `.planning/phases/04-grafana-dashboard-audit-metrics-gap-implementation/04-CONTEXT.md` — Metrics wiring decisions
- `.planning/phases/04-grafana-dashboard-audit-metrics-gap-implementation/04-05-PLAN.md` — Alert rule patterns to follow

### Codebase Concerns (failure modes documented)
- `.planning/codebase/CONCERNS.md` — Message Processor Consumer Group State, Error Handling Inconsistency sections

### ADRs
- `docs/adr/0002-redis-streams-pubsub-hybrid.md` — Why Redis Streams + Pub/Sub architecture was chosen

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `shared/metrics/` — Prometheus metrics package with RecordX() pattern; new pipeline resilience metrics follow same pattern
- `shared/listener/` — SDK with `LeadershipListener` and `ChannelManager`; ring buffer publish retry goes here
- Phase 4 Grafana dashboard ConfigMaps — DLQ panel extends existing pipeline dashboard

### Established Patterns
- Consumer group: `XREADGROUP` with `>` for new messages, `XACK` after processing (stream_consumer.go)
- Pub/Sub: per-overlay subscription with reference counting (subscriber.go)
- Metrics: promauto registration, RecordX() methods, Prometheus alert rules as code in ConfigMap
- Error handling: zap structured logging with `zap.Error(err)` field

### Integration Points
- Message-processor `stream_consumer.go` — PEL drain, unique consumer names, DLQ routing all modify this file
- API gateway `subscriber.go` — Pub/Sub reconnect modifies the listen goroutine
- API gateway `status_subscriber.go` — Nil channel guard modifies the Start goroutine
- Shared listener SDK `shared/listener/` — Ring buffer publish retry adds new file here
- K8s deployment manifests at `~/git/caesar-deployment/apps/workloads/all-chat/` — ServiceMonitor updates for new metrics

</code_context>

<specifics>
## Specific Ideas

- The "24 silent failure modes" list from the robustness audit should be fully enumerated during research — the researcher should audit all pipeline services systematically
- DLQ stream key: `chat:dlq` (parallel to `chat:raw`)
- Ring buffer in listener SDK should be opt-in via config, not forced — listeners that don't need it can skip

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 08-message-pipeline-resilience-fix-silent-failure-modes-across-twitch-message-pipeline*
*Context gathered: 2026-03-29*
