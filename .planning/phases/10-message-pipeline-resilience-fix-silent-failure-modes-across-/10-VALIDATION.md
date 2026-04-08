---
phase: 10
slug: message-pipeline-resilience-fix-silent-failure-modes-across
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-08
---

# Phase 10 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` with `testify/assert`, `testify/require`, `goleak` |
| **Config file** | none — standard `go test ./...` per service |
| **Quick run command** | `go test ./... -count=1 -timeout 30s` (per service dir) |
| **Full suite command** | `make test` |
| **Estimated runtime** | ~60 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd services/{service} && go test ./... -count=1 -timeout 30s`
- **After every plan wave:** Run `make test`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 10-01-01 | 01 | 1 | F-01 | — | Health probe methods do not block during SyncChannels | unit | `go test ./channels/... -run TestManager_SyncChannels_DoesNotBlockHealthProbe -v` | ✅ | ⬜ pending |
| 10-01-02 | 01 | 1 | F-02 | — | Ring buffer overflow emits Error-level structured log | unit | `go test ./publisher/... -run TestRingBuffer_OverflowLog -v` | ❌ W0 | ⬜ pending |
| 10-01-03 | 01 | 1 | F-03 | — | status.Publisher retries on Redis failure | unit | `go test ./status/... -run TestPublisher_RetryOnFailure -v` | ❌ W0 | ⬜ pending |
| 10-02-01 | 02 | 1 | F-04 | — | XReadGroup error loop uses exponential backoff | unit | `go test ./consumer/... -run TestConsumeLoop_ExponentialBackoff -v` | ❌ W0 | ⬜ pending |
| 10-02-02 | 02 | 1 | F-05 | — | DLQ write failure emits structured error log | unit | `go test ./consumer/... -run TestDLQ_WriteFailureLog -v` | ❌ W0 | ⬜ pending |
| 10-02-03 | 02 | 1 | F-06 | — | PEL reclaim on startup with reduced idle threshold | unit | `go test ./consumer/... -run TestPEL_ReclaimOnStartup -v` | ❌ W0 | ⬜ pending |
| 10-02-04 | 02 | 1 | F-07 | — | Consumer group created with "0" not "$" | unit | `go test ./consumer/... -run TestConsumerGroup_ReadsExisting -v` | ❌ W0 | ⬜ pending |
| 10-03-01 | 03 | 1 | F-08 | — | Subscriber.resubscribe retries indefinitely until stopChan | unit | `go test ./subscription/... -run TestSubscriber_ResubscribeRetries -v` | ❌ W0 | ⬜ pending |
| 10-03-02 | 03 | 1 | F-09 | — | StatusSubscriber.reconnect retries indefinitely | unit | `go test ./subscription/... -run TestStatusSubscriber_ReconnectInfinite -v` | ❌ W0 | ⬜ pending |
| 10-03-03 | 03 | 1 | F-10 | — | twitch-listener and api-gateway use shared Redis client | integration | verify via build + Redis client options inspection | ❌ W0 | ⬜ pending |
| 10-04-01 | 04 | 1 | F-11 | — | RenewLeadership is atomic (Lua script) | unit | `go test ./election/... -run TestRenewLeadership_Atomic -v` | ❌ W0 | ⬜ pending |
| 10-04-02 | 04 | 1 | F-12 | — | RegisterPeer does not SCAN on every call | unit | `go test ./election/... -run TestRegisterPeer_NoScan -v` | ❌ W0 | ⬜ pending |
| 10-01-04 | 01 | 2 | Z-01 | — | Zombie detector fires when received > 0 and published stalled N min | unit | `go test ./zombie/... -run TestZombieDetector_DetectsStall -v` | ❌ W0 | ⬜ pending |
| 10-01-05 | 01 | 2 | Z-02 | — | Zombie detector does not fire when both counters are zero | unit | `go test ./zombie/... -run TestZombieDetector_NoFalsePositiveWhenOffline -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/twitch-listener/publisher/ring_buffer_overflow_log_test.go` — covers F-02
- [ ] `services/twitch-listener/status/publisher_retry_test.go` — covers F-03
- [ ] `services/twitch-listener/zombie/detector.go` + `zombie/detector_test.go` — covers Z-01, Z-02
- [ ] `services/message-processor/consumer/backoff_test.go` — covers F-04
- [ ] `services/api-gateway/subscription/subscriber_retry_test.go` — covers F-08
- [ ] `services/api-gateway/subscription/status_subscriber_retry_test.go` — covers F-09
- [ ] `services/source-manager/election/leader_atomic_test.go` — covers F-11, F-12
- [ ] `shared/listener/backoff.go` + `shared/listener/backoff_test.go` — shared utility

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| F-10 shared Redis client adoption | F-10 | Requires runtime Redis connection inspection | Verify `services/twitch-listener/cmd/main.go` and `services/api-gateway/cmd/main.go` call `shared/redis.NewClientWithTracing` instead of bare `redis.NewClient` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
