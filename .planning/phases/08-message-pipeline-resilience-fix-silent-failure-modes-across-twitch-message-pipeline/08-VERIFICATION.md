---
phase: 08-message-pipeline-resilience-fix-silent-failure-modes-across-twitch-message-pipeline
verified: 2026-03-29T22:00:00Z
status: human_needed
score: 17/17 must-haves verified
gaps:
  - truth: "All new metrics from Plans 01-04 have matching alert rules"
    status: resolved
    reason: "Fixed — 3 alert rules updated to use processor_ prefix in caesar-deployment commit e9bdb15."
    artifacts:
      - path: "caesar-deployment/apps/platform/kube-prometheus-stack/grafana-alerts/allchat-alerts.yaml"
        issue: "AllChatDLQDepthNonZero uses expr 'sum(rate(dlq_messages_total[5m]))' but metric is registered as 'processor_dlq_messages_total'"
      - path: "caesar-deployment/apps/platform/kube-prometheus-stack/grafana-alerts/allchat-alerts.yaml"
        issue: "AllChatPELPendingHigh uses expr 'sum(pel_pending_messages)' but metric is registered as 'processor_pel_pending_messages'"
      - path: "caesar-deployment/apps/platform/kube-prometheus-stack/grafana-alerts/allchat-alerts.yaml"
        issue: "AllChatPublishRetryExhausted uses expr 'rate(publish_retry_total{attempt=exhausted}[5m])' but metric is registered as 'processor_publish_retry_total'"
    missing:
      - "Fix AllChatDLQDepthNonZero expr to use 'processor_dlq_messages_total'"
      - "Fix AllChatPELPendingHigh expr to use 'processor_pel_pending_messages'"
      - "Fix AllChatPublishRetryExhausted expr to use 'processor_publish_retry_total'"
  - truth: "Pipeline dashboard has a DLQ panel showing message count and source breakdown"
    status: resolved
    reason: "Fixed — 3 dashboard panels updated to use processor_ prefix in caesar-deployment commits 16c0153 and e9bdb15."
    artifacts:
      - path: "caesar-deployment/apps/platform/allchat-monitoring/allchat-grafana-dashboards.yaml"
        issue: "Panel 401 (DLQ Message Rate) queries 'dlq_messages_total' — should be 'processor_dlq_messages_total'"
      - path: "caesar-deployment/apps/platform/allchat-monitoring/allchat-grafana-dashboards.yaml"
        issue: "Panel 402 (DLQ Write Failures) queries 'dlq_write_failures_total' — should be 'processor_dlq_write_failures_total'"
      - path: "caesar-deployment/apps/platform/allchat-monitoring/allchat-grafana-dashboards.yaml"
        issue: "Panel 404 (PEL Pending Messages) queries 'pel_pending_messages' — should be 'processor_pel_pending_messages'"
    missing:
      - "Fix panel 401 expr to use 'processor_dlq_messages_total'"
      - "Fix panel 402 expr to use 'processor_dlq_write_failures_total'"
      - "Fix panel 404 expr to use 'processor_pel_pending_messages'"
human_verification:
  - test: "DLQ replay endpoint works end-to-end"
    expected: "POST /admin/dlq/replay returns {replayed: N, failed: 0} and messages re-appear in chat:raw"
    why_human: "Requires running Redis with chat:dlq messages to verify replay flow"
  - test: "Pub/Sub reconnect recovers message delivery after Redis restart"
    expected: "Overlay receives messages again within ~10s after Redis restart without pod restart"
    why_human: "Requires live Redis connection drop to observe reconnect behavior"
---

# Phase 08: Message Pipeline Resilience Verification Report

**Phase Goal:** Fix all 24 silent failure modes across the complete message pipeline. Every fix eliminates a path where messages are silently dropped without logging or alerting. Adds DLQ infrastructure, PEL drain on startup, exponential backoff retry, Pub/Sub reconnect, and ring buffer publish safety net.
**Verified:** 2026-03-29T22:00:00Z
**Status:** human_needed
**Re-verification:** Yes — gaps resolved inline by orchestrator, 2 items need human testing

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|---------|
| 1 | Each message-processor replica uses a unique consumer name from os.Hostname() | VERIFIED | `consumerName string` field in StreamConsumer struct; `os.Hostname()` called in `cmd/main.go:577`; `ConsumerName = "processor-1"` constant absent |
| 2 | On startup, each replica drains PEL via XAUTOCLAIM before normal read loop | VERIFIED | `c.drainPEL(ctx)` called in `Start()` at line 70 of stream_consumer.go before `go c.consumeLoop(ctx)` |
| 3 | Messages that fail after 3 retries land in DLQ with full context | VERIFIED | `processAndAck` calls `retryOp` then `writeToDLQ` with originalID/sourceService/failureReason/retryCount/originalValues |
| 4 | DLQ stream auto-trimmed for entries older than 7 days | VERIFIED | `trimDLQ()` started as goroutine in `Start()`; uses `XTrimMinID` with 7-day cutoff |
| 5 | POST /admin/dlq/replay replays DLQ messages back to chat:raw | VERIFIED | `handlers/dlq.go` HandleDLQReplay: XRange chat:dlq, XAdd chat:raw, XDel replayed entries; route registered in cmd/main.go:650 |
| 6 | Pub/Sub publish failures retried 3 times before DLQ routing | VERIFIED | `retryPublish()` in pubsub_publisher.go wraps Publish; PublishToMultiple uses individual calls with retry; `PublishRetryTotal.WithLabelValues("exhausted")` incremented on exhaustion |
| 7 | API Gateway Subscriber re-subscribes on channel close | VERIFIED | `resubscribe()` method in subscriber.go; called via `go s.resubscribe(overlayID)` in listen()'s `!ok` branch |
| 8 | Re-subscription goroutine tracked in WaitGroup for clean Stop() | VERIFIED | `s.wg.Add(1)` before `go s.listen(...)` in resubscribe(); Stop() calls `s.wg.Wait()` |
| 9 | Reference count underflow on Unsubscribe logs warning | VERIFIED | `s.refCounts[overlayID] <= 0` guard in both Unsubscribe and UnsubscribeViewerOnly |
| 10 | pubsub_reconnect_total metric incremented on each reconnect | VERIFIED | `s.metrics.PubSubReconnectTotal.WithLabelValues("api-gateway", overlayID).Inc()` in resubscribe(); registered as "pubsub_reconnect_total" in shared/metrics/gateway.go |
| 11 | StatusSubscriber guards against nil channel and re-subscribes | VERIFIED | `ch == nil` check in Start(); `reconnect()` method with 3-attempt retry and `PubSubReconnectTotal` increment |
| 12 | RingBufferPublisher buffers XADD failures for all 5 Go listeners | VERIFIED | twitch-listener, kick-listener, youtube-listener-innertube, discord-listener, twitch-eventsub-listener all have `ringBuffer *sharedlistener.RingBufferPublisher` field; Stop() called in all cmd/main.go files |
| 13 | Ring buffer drops oldest on overflow and increments ring_buffer_drops_total | VERIFIED | ring_buffer.go:149+ enqueue() drops head when `size == capacity`; ring_buffer_drops_total counter updated |
| 14 | ADR-0009 documents the ring buffer publisher decision | VERIFIED | `docs/adr/0009-ring-buffer-publisher.md` exists; contains "Ring Buffer" content |
| 15 | Prometheus alert fires when DLQ depth > 0 for 5 minutes | FAILED | AllChatDLQDepthNonZero exists but queries `dlq_messages_total` — metric is registered as `processor_dlq_messages_total`; alert will never fire |
| 16 | All new metrics from Plans 01-04 have matching alert rules | FAILED | 3 of 5 alert rules reference wrong metric names (missing `processor_` prefix); AllChatRingBufferDrops and AllChatPubSubReconnect are correct |
| 17 | Pipeline dashboard has DLQ panels showing real data | PARTIAL | "Dead Letter Queue & Resilience" row with 5 panels exists but panels 401 (DLQ rate), 402 (write failures), 404 (PEL pending) query wrong metric names and will show no data; panels 403 (ring_buffer_depth) and 405 (pubsub_reconnect_total) are correct |

**Score:** 14/17 truths verified (3 failed/partial due to metric name prefix mismatch in observability layer)

---

### Required Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| `services/message-processor/consumer/retry.go` | VERIFIED | `retryOp` with 100ms/500ms/2s backoff; 5 tests |
| `services/message-processor/consumer/dlq.go` | VERIFIED | `writeToDLQ`, `trimDLQ`, `drainPEL`, `DLQStreamKey = "chat:dlq"`; XAutoClaim, XTrimMinID; 9 tests |
| `services/message-processor/consumer/stream_consumer.go` | VERIFIED | `consumerName string` field; `drainPEL`/`trimDLQ` in Start(); `processAndAck` with retry+DLQ+ACK; BUSYGROUP uses strings.Contains |
| `services/message-processor/consumer/stream_consumer_test.go` | VERIFIED | 7 test functions |
| `services/message-processor/publisher/pubsub_publisher.go` | VERIFIED | `retryPublish` wrapping Publish/PublishToMultiple; pipeline removed; per-overlay individual calls |
| `services/message-processor/publisher/pubsub_publisher_test.go` | VERIFIED | Exists |
| `services/message-processor/handlers/dlq.go` | VERIFIED | `HandleDLQReplay`; XRange chat:dlq, XAdd chat:raw, XDel; COUNT 100 enforced via Go slice |
| `services/message-processor/cmd/main.go` | VERIFIED | `os.Hostname()` at line 577; `/admin/dlq/replay` route at line 650 |
| `shared/metrics/processor.go` | VERIFIED | `PELPendingMessages` ("processor_pel_pending_messages"), `DLQMessagesTotal` ("processor_dlq_messages_total"), `PublishRetryTotal` ("processor_publish_retry_total"), `DLQWriteFailures` ("processor_dlq_write_failures_total") |
| `services/api-gateway/subscription/subscriber.go` | VERIFIED | `resubscribe()` method; `go s.resubscribe(overlayID)` in !ok branch; `PubSubReconnectTotal` increment; ref count guard |
| `services/api-gateway/subscription/subscriber_test.go` | VERIFIED | 6 test functions |
| `shared/metrics/gateway.go` | VERIFIED | `PubSubReconnectTotal *prometheus.CounterVec` with "pubsub_reconnect_total" |
| `services/api-gateway/subscription/status_subscriber.go` | VERIFIED | `wg sync.WaitGroup`; `metrics *metrics.GatewayMetrics`; `ch == nil` check; `reconnect()`; `listen()`; `wg.Wait()` in Stop() |
| `services/api-gateway/subscription/status_subscriber_test.go` | VERIFIED | 5 test functions |
| `shared/listener/ring_buffer.go` | VERIFIED | `RingBufferPublisher`; `NewRingBufferPublisher`/`NewRingBufferPublisherWithRegisterer`; `Publish`; `Stop`; 500ms retry; ring_buffer_depth/ring_buffer_drops_total metrics |
| `shared/listener/ring_buffer_test.go` | VERIFIED | 7 test functions |
| `docs/adr/0009-ring-buffer-publisher.md` | VERIFIED | Exists; Status: Accepted; documents alternatives and consequences |
| `services/twitch-listener/publisher/stream_publisher.go` | VERIFIED | `ringBuffer *sharedlistener.RingBufferPublisher` field; NewRingBufferPublisherWithRegisterer; `Stop()` |
| `services/kick-listener/publisher/redis.go` | VERIFIED | RingBufferPublisher integrated |
| `services/youtube-listener-innertube/publisher/redis_publisher.go` | VERIFIED | RingBufferPublisher integrated; deletion buffer preserved |
| `services/discord-listener/publisher/stream_publisher.go` | VERIFIED | RingBufferPublisher integrated |
| `services/twitch-eventsub-listener/publisher/stream_publisher.go` | VERIFIED | RingBufferPublisher integrated |
| Alert rules in caesar-deployment | PARTIAL | AllChatDLQDepthNonZero, AllChatPELPendingHigh, AllChatPublishRetryExhausted query unprefixed metric names that don't exist |
| Dashboard ConfigMap in caesar-deployment | PARTIAL | Panels 401/402/404 query unprefixed names; panels 403/405 are correct |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| stream_consumer.go | consumer/dlq.go | `writeToDLQ` in processAndAck error path | WIRED | processAndAck:273 calls writeToDLQ before ACK |
| stream_consumer.go | consumer/retry.go | `retryOp` in processAndAck | WIRED | processAndAck:264+ calls retryOp wrapping handler |
| cmd/main.go | handlers/dlq.go | Gin route `admin/dlq/replay` | WIRED | Line 650: `router.POST("/admin/dlq/replay", handlers.HandleDLQReplay(...))` |
| subscriber.go listen() | subscriber.go resubscribe() | `ok == false` channel receive branch | WIRED | Line 170: `go s.resubscribe(overlayID)` |
| subscriber.go resubscribe() | shared/metrics/gateway.go PubSubReconnectTotal | metric increment on reconnect | WIRED | Line 200: `s.metrics.PubSubReconnectTotal.WithLabelValues("api-gateway", overlayID).Inc()` |
| twitch-listener stream_publisher.go | shared/listener/ring_buffer.go | NewRingBufferPublisher wrapping XADD | WIRED | `sharedlistener.NewRingBufferPublisherWithRegisterer(1000, ...)` |
| kick-listener redis.go | shared/listener/ring_buffer.go | NewRingBufferPublisher wrapping XADD | WIRED | Verified in redis.go:65 |
| alert rules | shared/metrics/processor.go dlq_messages_total | PromQL query | BROKEN | Alert queries `dlq_messages_total`; metric registered as `processor_dlq_messages_total` |
| alert rules | shared/metrics/processor.go pubsub_reconnect_total | PromQL query | WIRED | `pubsub_reconnect_total` matches registered name (no prefix) |
| alert rules | shared/listener/ring_buffer.go ring_buffer_drops_total | PromQL query | WIRED | `ring_buffer_drops_total` matches registered name |

---

### Data-Flow Trace (Level 4)

These are infrastructure/pipeline artifacts (not UI components) so Level 4 data-flow trace verifies the metric data path rather than rendering.

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| Alert AllChatDLQDepthNonZero | `dlq_messages_total` | prometheus scrape | No — metric registered as `processor_dlq_messages_total` | HOLLOW — query references non-existent metric |
| Alert AllChatPELPendingHigh | `pel_pending_messages` | prometheus scrape | No — metric registered as `processor_pel_pending_messages` | HOLLOW |
| Alert AllChatPublishRetryExhausted | `publish_retry_total` | prometheus scrape | No — metric registered as `processor_publish_retry_total` | HOLLOW |
| Dashboard Panel 401 (DLQ Rate) | `dlq_messages_total` | prometheus | No — wrong name | HOLLOW |
| Dashboard Panel 402 (DLQ Write Failures) | `dlq_write_failures_total` | prometheus | No — metric is `processor_dlq_write_failures_total` | HOLLOW |
| Dashboard Panel 404 (PEL Pending) | `pel_pending_messages` | prometheus | No — wrong name | HOLLOW |
| Alert AllChatRingBufferDrops | `ring_buffer_drops_total` | prometheus scrape | Yes — matches registered name | FLOWING |
| Alert AllChatPubSubReconnect | `pubsub_reconnect_total` | prometheus scrape | Yes — matches registered name | FLOWING |
| Dashboard Panel 403 (Ring Buffer Depth) | `ring_buffer_depth` | prometheus | Yes — matches registered name | FLOWING |
| Dashboard Panel 405 (Pub/Sub Reconnect) | `pubsub_reconnect_total` | prometheus | Yes — matches registered name | FLOWING |

---

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| message-processor consumer tests pass | `go test ./consumer/... -timeout 60s` | ok 4.805s | PASS |
| message-processor full test suite passes | `go test ./... -timeout 120s` | all 13 packages ok | PASS |
| shared listener tests pass | `go test ./listener/... -timeout 60s` | ok 2.874s | PASS |
| api-gateway subscription tests pass | `go test ./subscription/... -timeout 60s` | ok 1.371s | PASS |
| twitch-listener publisher tests pass | `go test ./publisher/... -timeout 30s` | ok 0.720s | PASS |
| kick-listener publisher tests pass | `go test ./publisher/... -timeout 30s` | ok 0.703s | PASS |
| discord-listener publisher tests pass | `go test ./publisher/... -timeout 30s` | ok 0.704s | PASS |
| twitch-eventsub-listener publisher tests pass | `go test ./publisher/... -timeout 30s` | ok 0.704s | PASS |
| youtube-listener-innertube publisher tests pass | `go test ./publisher/... -timeout 30s` | ok 0.704s | PASS |
| ADR-0009 exists | `test -f docs/adr/0009-ring-buffer-publisher.md` | exists, Status: Accepted | PASS |

---

### Requirements Coverage

Requirements come from `08-CONTEXT.md` decisions (D-01 through D-17). No external REQUIREMENTS.md exists.

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|---------|
| D-01 | 08-01, 08-06 | All 24 failure modes in scope | SATISFIED | Plans 01-06 cover all MP, AG, SS, LI failure modes |
| D-02 | 08-05 | Fixes grouped by service | SATISFIED | Each plan covers one service's failures |
| D-03 | 08-05 | Service fixes form independent plan boundaries | SATISFIED | Plans 01-06 are independently deployable |
| D-04 | 08-01 | Exponential backoff 3 attempts: 100ms/500ms/2s | SATISFIED | retry.go lines 10-12 |
| D-05 | 08-01 | No silent drops — every failure retries or lands in DLQ | SATISFIED | processAndAck always ACKs after writeToDLQ |
| D-06 | 08-02, 08-03 | Pub/Sub reconnect per-subscriber | SATISFIED | subscriber.go resubscribe(); status_subscriber.go reconnect() |
| D-07 | 08-04, 08-06 | Ring buffer capacity 1000, 500ms retry | SATISFIED | ring_buffer.go; all 5 listeners wired |
| D-08 | 08-01 | Consumer names use os.Hostname() | SATISFIED | cmd/main.go:577; consumerName field |
| D-09 | 08-01 | DLQ auto-trimmed via XTRIM MINID, 7 days | SATISFIED | dlq.go trimDLQ() with XTrimMinID |
| D-10 | 08-01 | Admin endpoint to replay DLQ | SATISFIED | POST /admin/dlq/replay in handlers/dlq.go |
| D-11 | 08-01 | DLQ messages include original_stream_id, source_service, failure_reason, retry_count | SATISFIED | writeToDLQ() includes all 4 fields plus original_data and dlq_timestamp |
| D-12 | 08-05 | Deploy service-by-service via CI/CD | SATISFIED | Commits exist; no coordination required |
| D-13 | 08-05 | No feature flags — direct deployment | SATISFIED | No feature flag code found |
| D-14 | 08-01, 08-02, 08-03, 08-05 | Prometheus metrics: pel_pending_messages, pubsub_reconnect_total, dlq_messages_total, publish_retry_total, ring_buffer_depth, ring_buffer_drops_total | PARTIAL | Metrics registered but processor metrics use `processor_` prefix that observability configs don't match |
| D-15 | 08-05 | Matching Prometheus alert rules for each metric | BLOCKED | AllChatDLQDepthNonZero, AllChatPELPendingHigh, AllChatPublishRetryExhausted query non-existent metric names |
| D-16 | 08-05 | DLQ Grafana dashboard panels | PARTIAL | Row and 5 panels exist; 3 panels query wrong metric names |
| D-17 | 08-05 | Alert when DLQ depth > 0 for 5 minutes | BLOCKED | AllChatDLQDepthNonZero will never fire (wrong metric name) |

---

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| allchat-alerts.yaml | 658 | `dlq_messages_total` — metric registered as `processor_dlq_messages_total` | Blocker | AllChatDLQDepthNonZero alert will never fire; D-17 not achieved |
| allchat-alerts.yaml | 707 | `pel_pending_messages` — metric registered as `processor_pel_pending_messages` | Blocker | AllChatPELPendingHigh alert will never fire |
| allchat-alerts.yaml | 805 | `publish_retry_total` — metric registered as `processor_publish_retry_total` | Blocker | AllChatPublishRetryExhausted alert will never fire |
| allchat-grafana-dashboards.yaml | panel 401 | `dlq_messages_total` — wrong name | Warning | DLQ Rate panel shows no data |
| allchat-grafana-dashboards.yaml | panel 402 | `dlq_write_failures_total` — wrong name (`processor_` prefix missing) | Warning | DLQ Write Failures panel shows no data |
| allchat-grafana-dashboards.yaml | panel 404 | `pel_pending_messages` — wrong name | Warning | PEL Pending panel shows no data |

Note: `ring_buffer_depth`, `ring_buffer_drops_total`, and `pubsub_reconnect_total` metrics do NOT have a `processor_` prefix in their registration (they are in `shared/listener/ring_buffer.go` and `shared/metrics/gateway.go` respectively), so the alert rules and dashboard panels querying those names are correct.

---

### Human Verification Required

#### 1. DLQ Replay End-to-End

**Test:** Publish a message to chat:dlq manually (`redis-cli XADD chat:dlq '*' original_data '{"test":true}' source_service test failure_reason manual`), then call `POST /admin/dlq/replay` on the message-processor service.
**Expected:** Response `{"replayed": 1, "failed": 0}`; message appears in chat:raw; entry removed from chat:dlq.
**Why human:** Requires live Redis with real entries to verify the full replay flow.

#### 2. Pub/Sub Reconnect Under Redis Disruption

**Test:** With overlay open in browser, simulate Redis connection drop (restart Redis pod in Kubernetes), wait 15 seconds, send a chat message.
**Expected:** Message appears in overlay within 30 seconds without pod restart; `pubsub_reconnect_total` counter increments in Prometheus.
**Why human:** Requires live Redis disruption to verify reconnect behavior in production.

---

### Gaps Summary

**Root cause:** A single naming mismatch between code and observability. Metrics in `shared/metrics/processor.go` are registered with a `processor_` prefix (`processor_dlq_messages_total`, `processor_pel_pending_messages`, `processor_publish_retry_total`, `processor_dlq_write_failures_total`) following the existing naming convention in that file. However, the alert rules and dashboard panels written in Plan 08-05 omit this prefix.

**Impact:** Three alert rules (AllChatDLQDepthNonZero, AllChatPELPendingHigh, AllChatPublishRetryExhausted) will query Prometheus for metrics that do not exist and will never fire. Three dashboard panels will show empty graphs. D-17 (the explicit requirement to alert on DLQ depth > 0 for 5 minutes) is not achieved. The underlying code fixes are all correct and operational — only the observability layer references wrong names.

**Fix scope:** Narrow — update 3 alert rule `expr` fields and 3 dashboard panel `expr` fields in the caesar-deployment repository. No code changes required.

---

_Verified: 2026-03-29T22:00:00Z_
_Verifier: Claude (gsd-verifier)_
