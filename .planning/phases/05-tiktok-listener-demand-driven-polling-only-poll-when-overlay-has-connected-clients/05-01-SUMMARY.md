---
phase: 05-tiktok-listener-demand-driven-polling-only-poll-when-overlay-has-connected-clients
plan: 01
subsystem: source-manager
tags: [demand-signal, redis-pubsub, source-manager, tiktok, phase-5]
dependency_graph:
  requires:
    - api-gateway overlay:connections Pub/Sub channel
    - api-gateway overlay:connected:{id} Redis keys
    - overlay_chat_sources and overlays database tables
  provides:
    - source:demand Redis Pub/Sub channel with DemandUpdate snapshots
    - GET /demand HTTP endpoint for fallback polling
  affects:
    - services/source-manager (new demand/ package)
    - services/source-manager/registry/repository.go (new method)
    - services/source-manager/cmd/main.go (wiring)
tech_stack:
  added: []
  patterns:
    - Redis Pub/Sub subscriber with exponential backoff retry loop
    - Startup hydration from Redis key scan before subscribing to live events
    - In-memory demand map protected by sync.RWMutex
    - Test doubles via interface injection (sourceRepository interface)
    - miniredis for Redis mocking in unit tests
key_files:
  created:
    - services/source-manager/demand/subscriber.go
    - services/source-manager/demand/handler.go
    - services/source-manager/demand/subscriber_test.go
  modified:
    - services/source-manager/registry/repository.go
    - services/source-manager/cmd/main.go
decisions:
  - sourceRepository interface defined in demand package — allows mock injection in tests without importing registry package
  - HandleConnectionEventForTest and HydrateForTest test helpers on OverlayDemandSubscriber — expose private methods for unit testing without making the main API surface public
  - hydrate() called before subscribeLoop() in Start() — prevents empty DemandUpdate snapshot on source-manager restart (Pitfall 3)
  - GetDemandedSources() returns make([]DemandSource, 0) not nil — ensures JSON marshals as [] not null when no sources are demanded
metrics:
  duration: 226s
  completed_date: "2026-03-27"
  tasks_completed: 2
  files_changed: 5
---

# Phase 05 Plan 01: Demand Signal Infrastructure in Source-Manager Summary

**One-liner:** OverlayDemandSubscriber that hydrates from Redis keys on startup, subscribes to overlay:connections Pub/Sub, maintains per-overlay demand map, and publishes full DemandUpdate snapshots to source:demand channel.

## What Was Built

Source-manager now acts as the sole authority for "which sources have demand":

1. **demand/subscriber.go** — `OverlayDemandSubscriber` that:
   - Scans `overlay:connected:*` keys on startup and hydrates demand from DB before subscribing
   - Subscribes to `overlay:connections` Pub/Sub channel for live connect/disconnect events
   - On connect: queries `GetSourcesForOverlays` and adds to demand map
   - On disconnect: removes overlay from demand map
   - After each change: publishes `DemandUpdate` snapshot to `source:demand` channel
   - Retry loop with exponential backoff (1s to 30s) for resilience

2. **demand/handler.go** — `DemandHandler` providing `GET /demand[?platform=X]` for fallback polling by listeners

3. **registry/repository.go** — `GetSourcesForOverlays(ctx, overlayIDs)` query with active/non-banned filter

4. **cmd/main.go** — wired demand subscriber into startup sequence and HTTP routes

## Tests

6 tests in `demand/subscriber_test.go`, all passing:
- `TestGetSourcesForOverlays` — mock repository interface contract
- `TestOverlayDemandSubscriber_Connected` — connect event adds sources
- `TestOverlayDemandSubscriber_Disconnected` — disconnect event removes sources
- `TestStartupHydration` — hydrates from overlay:connected:* keys
- `TestDemandHandler_GetDemand` — filters by platform
- `TestEmptyDemand` — returns non-nil empty slice

## Commits

- `d9effed` — test(05-01): add failing tests for OverlayDemandSubscriber and DemandHandler (RED)
- `df35412` — feat(05-01): add OverlayDemandSubscriber, DemandHandler, GetSourcesForOverlays (GREEN)
- `5d6831d` — feat(05-01): wire demand subscriber and GET /demand endpoint into source-manager main.go

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED

Files created:
- services/source-manager/demand/subscriber.go: FOUND
- services/source-manager/demand/handler.go: FOUND
- services/source-manager/demand/subscriber_test.go: FOUND

Commits verified:
- d9effed: FOUND
- df35412: FOUND
- 5d6831d: FOUND

Acceptance criteria verified:
- `type OverlayDemandSubscriber struct`: present in subscriber.go
- `func (s *OverlayDemandSubscriber) Start`: present in subscriber.go
- `Publish(ctx, "source:demand"`: present in subscriber.go
- `Keys(ctx, "overlay:connected:*"`: present in subscriber.go
- `Subscribe(ctx, "overlay:connections"`: present in subscriber.go
- `type DemandUpdate struct`: present in subscriber.go
- `type DemandSource struct`: present in subscriber.go
- `func (h *DemandHandler) GetDemand`: present in handler.go
- `func (r *Repository) GetSourcesForOverlays`: present in registry/repository.go
- `func TestOverlayDemandSubscriber`: present in subscriber_test.go
- `go test ./demand/... -v -count=1`: exits 0
- `go build ./...`: exits 0
