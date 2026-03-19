---
phase: 36
slug: migrate-kick-listener
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-17
---

# Phase 36 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify (indirect via shared) + goleak v1.3.0 |
| **Config file** | none — standard `go test` |
| **Quick run command** | `cd services/kick-listener && go build ./... && go test ./... -count=1` |
| **Full suite command** | `cd services/kick-listener && go test ./... -race -count=1` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd services/kick-listener && go build ./... && go test ./... -count=1`
- **After every plan wave:** Run `cd services/kick-listener && go test ./... -race -count=1`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** ~15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 36-01-01 | 01 | 0 | MIGRATE-02 | build assertion | `go build ./...` in kick-listener | ❌ W0 | ⬜ pending |
| 36-01-02 | 01 | 0 | MIGRATE-02 | unit | `go test ./channels/... -count=1` | ❌ W0 | ⬜ pending |
| 36-02-01 | 02 | 1 | MIGRATE-02 | build + goleak smoke | `go build ./cmd/... && go test ./cmd/... -count=1` | ❌ W0 | ⬜ pending |
| 36-02-02 | 02 | 1 | MIGRATE-02 | unit (existing) | `go test ./channels/... -count=1` | ✅ existing | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/kick-listener/channels/manager.go` — add `var _ listener.ChannelManager = (*Manager)(nil)` compile-time assertion
- [ ] `services/kick-listener/go.mod` — add `go.uber.org/goleak@v1.3.0` as direct dep (`go get go.uber.org/goleak@v1.3.0`)
- [ ] `services/kick-listener/cmd/main_sdk_test.go` — goroutine leak smoke test covering MIGRATE-02

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Both SDK archetypes exercised against live Kick traffic with no `messages_published_total` regression | MIGRATE-02 | Requires live Kick stream + production coordinator | Deploy to staging, start kick-listener, observe `messages_published_total` metric over 5 minutes; verify messages flow |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
