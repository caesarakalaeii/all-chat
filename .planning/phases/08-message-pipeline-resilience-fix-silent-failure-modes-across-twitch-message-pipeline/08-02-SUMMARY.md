---
phase: 08-message-pipeline-resilience-fix-silent-failure-modes-across-twitch-message-pipeline
plan: "02"
subsystem: api-gateway
tags: [resilience, pubsub, reconnect, metrics, ref-count]
dependency_graph:
  requires: []
  provides: [api-gateway-pubsub-reconnect, pubsub_reconnect_total-metric]
  affects: [api-gateway, shared/metrics]
tech_stack:
  added: []
  patterns: [nil-safe metrics injection, per-test prometheus registry, stopChan-guarded goroutine lifecycle]
key_files:
  created:
    - services/api-gateway/subscription/subscriber_test.go
  modified:
    - services/api-gateway/subscription/subscriber.go
    - services/api-gateway/cmd/main.go
    - shared/metrics/gateway.go
decisions:
  - "resubscribe() uses context.Background() — subscriptions outlive HTTP requests, lifecycle is governed by stopChan"
  - "viewerOnly map[string]bool tracks subscription type so resubscribe recreates the correct channel set"
  - "NewGatewayMetricsForTest() uses promauto.With(fresh registry) — prevents duplicate registration panics when tests create multiple metrics instances"
  - "metrics parameter is nil-safe in NewSubscriber — allows callers that don't have metrics yet to pass nil"
metrics:
  duration: 252s
  completed: "2026-03-29"
  tasks_completed: 1
  files_modified: 4
---

# Phase 08 Plan 02: API Gateway Subscriber Resilience Summary

Implement automatic Pub/Sub re-subscription on channel close, goroutine lifecycle tracking, reference count underflow guard, and the `pubsub_reconnect_total` metric — fixing all 5 AG failure modes (AG-01 through AG-05).

## What Was Done

Added reconnect logic to the API Gateway Subscriber so that when Redis drops the Pub/Sub connection and go-redis closes the message channel, the Subscriber automatically creates a new subscription without requiring a pod restart. Added the `pubsub_reconnect_total` metric per D-14.

## Tasks Completed

| # | Name | Commit | Files |
|---|------|--------|-------|
| 1 | Add pubsub_reconnect_total metric and Subscriber reconnect/guard | 3733f8c | subscriber.go, subscriber_test.go, main.go |

## Decisions Made

- `resubscribe()` uses `context.Background()` — subscriptions outlive individual HTTP requests; lifecycle is governed by `stopChan`, not context cancellation
- New `viewerOnly map[string]bool` field tracks whether each subscription was created by `SubscribeViewerOnly` (main channel only) or `Subscribe` (main + updates), so `resubscribe()` recreates the correct channel set (AG-04)
- `NewGatewayMetricsForTest()` added to `shared/metrics` using `promauto.With(fresh registry)` — prevents duplicate metric registration panics when tests call metrics constructors multiple times in the same binary
- `metrics` parameter to `NewSubscriber` is nil-safe — `resubscribe()` guards with `if s.metrics != nil` so callers without metrics can pass nil

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Missing gatewayMetrics arg in NewStatusSubscriber call**
- **Found during:** Task 1 (build verification)
- **Issue:** `cmd/main.go` was calling `subscription.NewStatusSubscriber(redisClient, wsManager, log)` without the required `*metrics.GatewayMetrics` argument, which was added by a prior plan. The build was broken before this plan's changes.
- **Fix:** Added `gatewayMetrics` as the fourth argument to `NewStatusSubscriber` in main.go.
- **Files modified:** `services/api-gateway/cmd/main.go`
- **Commit:** 3733f8c

**2. [Rule 2 - Missing critical functionality] No per-test Prometheus registry**
- **Found during:** Task 1 (test execution)
- **Issue:** `status_subscriber_test.go` already used `metrics.NewGatewayMetrics()` and my new `subscriber_test.go` also called it — running both caused a "duplicate metrics collector registration" panic because `promauto` registers to the global registry.
- **Fix:** Added `NewGatewayMetricsForTest()` to `shared/metrics/gateway.go` using `promauto.With(prometheus.NewRegistry())` for test isolation. Updated both test files to use the new constructor.
- **Files modified:** `shared/metrics/gateway.go`, `subscriber_test.go`
- **Commit:** 3733f8c (shared/metrics committed by parallel agent 08-03)

## Acceptance Criteria Verification

- [x] `shared/metrics/gateway.go` contains `PubSubReconnectTotal` field
- [x] `shared/metrics/gateway.go` contains `pubsub_reconnect_total` metric name
- [x] `subscriber.go` contains `func (s *Subscriber) resubscribe(overlayID string)`
- [x] `subscriber.go` `listen()` contains `go s.resubscribe(overlayID)` in the `!ok` branch
- [x] `subscriber.go` `resubscribe` contains `PubSubReconnectTotal` increment
- [x] `subscriber.go` `Unsubscribe` contains `refCounts[overlayID] <= 0` guard
- [x] `subscriber.go` `UnsubscribeViewerOnly` contains same ref count guard
- [x] `subscriber.go` `resubscribe` checks `stopChan` before re-subscribing
- [x] `subscriber.go` `resubscribe` calls `s.wg.Add(1)` before spawning new listen goroutine
- [x] `subscriber_test.go` exists and contains `TestSubscriber*` tests
- [x] `go test ./subscription/... -timeout 30s` exits 0

## Known Stubs

None.

## Self-Check: PASSED

Files verified:
- `services/api-gateway/subscription/subscriber.go` — exists with resubscribe method
- `services/api-gateway/subscription/subscriber_test.go` — exists with 5 test functions
- Commits b41e55a (RED tests) and 3733f8c (GREEN implementation) verified in git log
