---
phase: 10
slug: message-pipeline-resilience-fix-silent-failure-modes-across
status: draft
nyquist_compliant: true
wave_0_complete: true
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
| 10-01-1a | 01 | 1 | F-01 | T-10-01 | Health probe methods do not block during SyncChannels | unit | `cd services/twitch-listener && go test ./channels/... -run "TestManager_AtomicProbe\|TestManager_SyncChannels_DoesNotBlockHealthProbe" -v` | ✅ | ⬜ pending |
| 10-01-1b-a | 01 | 1 | F-02 | — | Ring buffer overflow emits Error-level structured log | unit | `cd shared/listener && go test ./... -run TestRingBuffer_OverflowLog -v` | ❌ W0 | ⬜ pending |
| 10-01-1b-b | 01 | 1 | F-03 | — | status.Publisher retries on Redis failure | unit | `cd services/twitch-listener && go test ./status/... -run TestPublisher_RetryOnFailure -v` | ❌ W0 | ⬜ pending |
| 10-01-02 | 01 | 1 | Z-01 | T-10-03 | Zombie detector fires when received > 0 and published stalled N min | unit | `cd services/twitch-listener && go test ./zombie/... -run TestZombieDetector_DetectsStall -v` | ❌ W0 | ⬜ pending |
| 10-01-03 | 01 | 1 | Z-02 | T-10-03 | Zombie detector does not fire when both counters are zero | unit | `cd services/twitch-listener && go test ./zombie/... -run TestZombieDetector_NoFalsePositiveWhenOffline -v` | ❌ W0 | ⬜ pending |
| 10-01-04 | 01 | 1 | Z-01 | — | ADR-0011 documents zombie detector design | file-exists | `test -f docs/adr/0011-zombie-listener-detection.md && grep -q "0011" docs/adr/README.md` | N/A | ⬜ pending |
| 10-02-01 | 02 | 2 | F-08 | T-10-04 | Subscriber.resubscribe retries indefinitely until stopChan | unit | `cd services/api-gateway && go test ./subscription/... -run TestSubscriber_Resubscribe -v` | ❌ W0 | ⬜ pending |
| 10-02-02 | 02 | 2 | F-09 | T-10-05 | StatusSubscriber.reconnect retries indefinitely | unit | `cd services/api-gateway && go test ./subscription/... -run TestStatusSubscriber_ReconnectInfinite -v` | ❌ W0 | ⬜ pending |
| 10-02-03 | 02 | 2 | F-10 | T-10-06 | twitch-listener and api-gateway use shared Redis client | grep-regression | `! grep -n 'redis\.NewClient(&redis\.Options{' services/twitch-listener/cmd/main.go services/api-gateway/cmd/main.go` | N/A | ⬜ pending |
| 10-03-01 | 03 | 2 | F-04 | T-10-08 | XReadGroup error loop uses exponential backoff | unit | `cd services/message-processor && go test ./consumer/... -run TestConsumeLoop_ExponentialBackoff -v` | ❌ W0 | ⬜ pending |
| 10-03-02 | 03 | 2 | F-05 | T-10-09 | DLQ write failure emits structured error log | unit | `cd services/message-processor && go test ./consumer/... -run TestDLQ_WriteFailureLog -v` | ❌ W0 | ⬜ pending |
| 10-03-03 | 03 | 2 | F-06 | — | PEL idle threshold is documented (no code change) | N/A | N/A — documentation-only, no test required | N/A | ⬜ pending |
| 10-03-04 | 03 | 2 | F-07 | T-10-10 | Consumer group created with "0" not "$" | unit | `cd services/message-processor && go test ./consumer/... -run TestConsumerGroup_ReadsExisting -v` | ❌ W0 | ⬜ pending |
| 10-04-01 | 04 | 1 | F-11 | T-10-11 | RenewLeadership is atomic (Lua script) | unit | `cd services/source-manager && go test ./election/... -run TestRenewLeadership_Atomic -v` | ❌ W0 | ⬜ pending |
| 10-04-02 | 04 | 1 | F-12 | T-10-12 | RegisterPeer does not SCAN on every call | unit | `cd services/source-manager && go test ./election/... -run TestRegisterPeer_NoScan -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [x] `shared/listener/ring_buffer_test.go` — covers F-02 (ring buffer overflow log test)
- [x] `shared/listener/backoff_test.go` — covers shared backoff utility
- [x] `services/twitch-listener/status/publisher_test.go` — covers F-03
- [x] `services/twitch-listener/zombie/detector.go` + `zombie/detector_test.go` — covers Z-01, Z-02
- [x] `services/message-processor/consumer/stream_consumer_test.go` — covers F-04, F-05, F-07
- [x] `services/api-gateway/subscription/subscriber_test.go` — covers F-08
- [x] `services/api-gateway/subscription/status_subscriber_test.go` — covers F-09
- [x] `services/source-manager/election/leader_test.go` — covers F-11, F-12

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| F-10 shared Redis client adoption | F-10 | Grep-based regression check (no unit test for import choice) | `! grep -n 'redis\.NewClient(&redis\.Options{' services/twitch-listener/cmd/main.go services/api-gateway/cmd/main.go` — must return no matches |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 60s
- [x] F-06 is documentation-only — no phantom test requirement
- [x] F-02 test path matches plan: `shared/listener/ring_buffer_test.go`
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved
