---
phase: 38
slug: migrate-youtube-listener-and-twitch-eventsub-listener
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-18
---

# Phase 38 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` + `goleak` v1.3.0 |
| **Config file** | none — standard `go test ./cmd/...` |
| **Quick run command** | `go test ./cmd/... -v` (in each service dir) |
| **Full suite command** | `go test ./... -v` (in each service dir) |
| **Estimated runtime** | ~10 seconds per service |

---

## Sampling Rate

- **After every task commit:** Run `go build ./...` in the changed service directory
- **After every plan wave:** Run `go test ./... -v` in the changed service directory
- **Before `/gsd:verify-work`:** Full suite must be green (`make build-all` from repo root)
- **Max feedback latency:** ~10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 38-01-W0 | 01 | 0 | MIGRATE-03 | unit/smoke | `go test ./cmd/... -run TestListenerBase_StartStop_NoGoroutineLeak` | ❌ W0 | ⬜ pending |
| 38-01-01 | 01 | 1 | MIGRATE-03 | compile | `go build ./...` (youtube-listener dir) | ✅ | ⬜ pending |
| 38-01-02 | 01 | 1 | MIGRATE-03 | smoke | `go test ./cmd/... -run TestListenerBase_StartStop_NoGoroutineLeak` | ❌ W0 | ⬜ pending |
| 38-02-W0 | 02 | 0 | MIGRATE-06 | compile | `go build ./...` (eventsub dir) | ✅ | ⬜ pending |
| 38-02-01 | 02 | 1 | MIGRATE-06 | compile | `go build ./...` (eventsub dir) | ✅ | ⬜ pending |
| 38-02-02 | 02 | 1 | MIGRATE-06 | compile | `go build ./...` (eventsub dir) | ✅ | ⬜ pending |
| 38-03-W0 | 03 | 0 | MIGRATE-06 | unit/smoke | `go test ./cmd/... -run TestListenerBase_StartStop_NoGoroutineLeak` | ❌ W0 | ⬜ pending |
| 38-03-01 | 03 | 1 | MIGRATE-06 | compile | `go build ./...` (eventsub dir) | ✅ | ⬜ pending |
| 38-03-02 | 03 | 1 | MIGRATE-06 | smoke | `go test ./cmd/... -run TestListenerBase_StartStop_NoGoroutineLeak` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/youtube-listener/cmd/main_sdk_test.go` — goroutine leak smoke test for MIGRATE-03
- [ ] `services/twitch-eventsub-listener/cmd/main_sdk_test.go` — goroutine leak smoke test for MIGRATE-06
- [ ] `go.uber.org/goleak v1.3.0` added as direct dep in `services/youtube-listener/go.mod`
- [ ] `go.uber.org/goleak v1.3.0` added as direct dep in `services/twitch-eventsub-listener/go.mod`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Mixed-fleet monitoring — all 6 services running SDK-backed simultaneously | MIGRATE-03 + MIGRATE-06 | Requires live Kubernetes cluster observation | Verify no active mixed-fleet monitoring alerts after deploying both services |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
