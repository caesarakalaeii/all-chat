---
phase: 10
plan: "02"
status: complete
started: 2026-04-08T21:30:00+02:00
completed: 2026-04-08T21:50:00+02:00
---

## Summary

Hardened api-gateway Pub/Sub reconnection and standardized Redis client usage across api-gateway and twitch-listener.

## What Was Built

### Task 1: Subscriber.resubscribe infinite retry (F-08)
Replaced single-attempt resubscribe with infinite retry loop using `listener.JitteredBackoff`. Added stopChan check before `wg.Add(1)` to prevent WaitGroup leak (Pitfall 3). Tests verify retry-on-failure and transient-failure recovery.

### Task 2: StatusSubscriber.reconnect infinite retry (F-09)
Replaced 3-attempt cap with infinite retry using `listener.JitteredBackoff`. Added `defer s.wg.Done()` pattern and stopChan guard before `s.wg.Add(1)` in the listen() caller (Pitfall 4). Tests verify retry, stop-signal exit, and transient-failure recovery.

### Task 3: Shared Redis client standardization (F-10)
Both api-gateway and twitch-listener now use `sharedredis.NewClientWithTracing` instead of bare `redis.NewClient`. Pool tuning: MaxRetries=3, PoolSize=50, MinIdleConns=10, DialTimeout=5s, ReadTimeout=3s, WriteTimeout=3s, PoolTimeout=4s.

## Key Files

### key-files.modified
- `services/api-gateway/subscription/subscriber.go` — infinite retry resubscribe
- `services/api-gateway/subscription/status_subscriber.go` — infinite retry reconnect
- `services/api-gateway/cmd/main.go` — shared Redis client
- `services/twitch-listener/cmd/main.go` — shared Redis client

### key-files.created
- `services/api-gateway/subscription/subscriber_test.go` — 2 new retry tests
- `services/api-gateway/subscription/status_subscriber_test.go` — 3 new retry tests

## Commits
- `d38b65b` feat(10-02): infinite retry with jittered backoff in Subscriber.resubscribe (F-08)
- `186534d` feat(10-02): infinite retry with jittered backoff in StatusSubscriber.reconnect (F-09)
- `ffaf82a` feat(10-02): replace bare redis.NewClient with shared Redis client (F-10)

## Self-Check: PASSED
- [x] Subscriber.resubscribe retries indefinitely with JitteredBackoff
- [x] StatusSubscriber.reconnect retries indefinitely with JitteredBackoff
- [x] Both reconnect paths tracked in WaitGroup for clean shutdown
- [x] Both services use shared Redis client with pool tuning
- [x] All tests pass (15/15 subscription tests)
- [x] Grep regression: no bare redis.NewClient in either cmd/main.go
