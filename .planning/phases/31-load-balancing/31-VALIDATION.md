---
phase: 31
slug: load-balancing
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-16
---

# Phase 31 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — existing Go test infrastructure |
| **Quick run command** | `cd services/discord-listener && go test ./...` |
| **Full suite command** | `cd services/discord-listener && go test ./... -race` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd services/discord-listener && go test ./...`
- **After every plan wave:** Run `cd services/discord-listener && go test ./... -race`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 31-01-01 | 01 | 1 | LOAD-03 | unit | `cd services/discord-listener && go test ./gateway/...` | ❌ W0 | ⬜ pending |
| 31-01-02 | 01 | 1 | LOAD-03 | unit | `cd services/discord-listener && go test ./gateway/...` | ❌ W0 | ⬜ pending |
| 31-01-03 | 01 | 1 | LOAD-03 | unit | `cd services/discord-listener && go test ./gateway/...` | ❌ W0 | ⬜ pending |
| 31-02-01 | 02 | 2 | LOAD-01 | integration | `cd services/discord-listener && go test ./...` | ❌ W0 | ⬜ pending |
| 31-02-02 | 02 | 2 | LOAD-02 | unit | `cd services/discord-listener && go test ./...` | ❌ W0 | ⬜ pending |
| 31-03-01 | 03 | 1 | LOAD-02 | manual | kubectl apply --dry-run=client | ❌ W0 | ⬜ pending |
| 31-03-02 | 03 | 1 | LOAD-02 | manual | kubectl get hpa | N/A | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/discord-listener/gateway/resume_test.go` — stubs for LOAD-03 RESUME opcode tests (TestResumeWhenSessionExists, TestIdentifyWhenNoSession, TestInvalidSessionFalseClears, TestInvalidSessionTruePreserves, TestReconnectPreservesSession)

*metrics/metrics_test.go is created inline by Plan 31-02 Task 1 (tdd=true) — no separate Wave 0 stub needed.*

*Existing test infrastructure covers framework; Wave 0 adds the new gateway test file for new behavior.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Two pods — only one holds Gateway connection | LOAD-01 | Requires live k8s cluster + Discord token | Deploy 2 replicas, check logs that only 1 pod shows IDENTIFY/RESUME sent |
| Session RESUME after pod restart | LOAD-03 | Requires live Discord Gateway + Redis | Restart pod, verify logs show RESUME (op=6) not IDENTIFY (op=2) |
| HPA scales and new pod acquires shard within 60s | LOAD-02 | Requires HPA + load generation | Simulate load, watch HPA scale up, verify pod acquires shard within 60s |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
