---
phase: 8
slug: message-pipeline-resilience-fix-silent-failure-modes-across-twitch-message-pipeline
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-29
---

# Phase 8 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify v1.11.1 + miniredis v2.37.0 |
| **Config file** | Existing per-service go.mod (miniredis already in message-processor) |
| **Quick run command** | `cd services/message-processor && go test ./consumer/... -v -timeout 30s` |
| **Full suite command** | `cd services/message-processor && go test ./... -timeout 120s && cd ../../services/api-gateway && go test ./subscription/... -timeout 120s && cd ../../shared/listener && go test ./... -timeout 60s` |
| **Estimated runtime** | ~60 seconds |

---

## Sampling Rate

- **After every task commit:** Run quick suite in modified service
- **After every plan wave:** Run full suite across all modified services
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 08-01-01 | 01 | 1 | D-08/MP-01 | unit | `go test ./consumer/... -run TestConsumerName` | ❌ W0 | ⬜ pending |
| 08-01-02 | 01 | 1 | D-01/MP-02 | unit (miniredis) | `go test ./consumer/... -run TestPELDrain` | ❌ W0 | ⬜ pending |
| 08-01-03 | 01 | 1 | D-04/MP-04 | unit (miniredis) | `go test ./consumer/... -run TestPublishRetry` | ❌ W0 | ⬜ pending |
| 08-01-04 | 01 | 1 | D-05/DQ-01 | unit (miniredis) | `go test ./consumer/... -run TestDLQWrite` | ❌ W0 | ⬜ pending |
| 08-01-05 | 01 | 1 | D-09/DQ-02 | unit (miniredis) | `go test ./consumer/... -run TestDLQTrim` | ❌ W0 | ⬜ pending |
| 08-02-01 | 02 | 1 | D-06/AG-01 | unit (miniredis) | `go test ./subscription/... -run TestResubscribe` | ❌ W0 | ⬜ pending |
| 08-03-01 | 03 | 1 | D-06/SS-01 | unit | `go test ./subscription/... -run TestStatusSubscriberNilChannel` | ❌ W0 | ⬜ pending |
| 08-04-01 | 04 | 1 | D-07/LI-01 | unit | `go test ./... -run TestRingBuffer` | ❌ W0 | ⬜ pending |
| 08-04-02 | 04 | 1 | D-07/LI-01 | unit | `go test ./... -run TestRingBufferFull` | ❌ W0 | ⬜ pending |
| 08-05-01 | 05 | 2 | D-14 | unit | `go test ./consumer/... -run TestMetrics` | ❌ W0 | ⬜ pending |
| 08-manual-01 | 01 | - | D-08 | manual | hostname differs per K8s pod | N/A | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/message-processor/consumer/stream_consumer_test.go` — stubs for MP-01, MP-02, MP-04, D-05, D-08, D-09
- [ ] `services/api-gateway/subscription/subscriber_test.go` — stubs for AG-01 reconnect
- [ ] `services/api-gateway/subscription/status_subscriber_test.go` — stubs for SS-01, SS-02
- [ ] `shared/listener/ring_buffer_test.go` — stubs for LI-01, ring buffer capacity, drop semantics

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Consumer name unique per replica | D-08 | Requires multiple K8s pods | Deploy 3 replicas, `XINFO GROUPS chat:raw` → verify 3 distinct consumer names matching pod hostnames |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
