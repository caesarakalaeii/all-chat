---
phase: 35
slug: migrate-twitch-listener
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-17
---

# Phase 35 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test |
| **Config file** | none — existing go.mod |
| **Quick run command** | `cd services/twitch-listener && go test ./...` |
| **Full suite command** | `cd services/twitch-listener && go test ./... && cd shared/listener && go test ./...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd services/twitch-listener && go test ./...`
- **After every plan wave:** Run `cd services/twitch-listener && go test ./... && cd shared/listener && go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 35-01-01 | 01 | 1 | MIGRATE-01 | build | `cd services/twitch-listener && go build ./cmd/...` | ✅ | ⬜ pending |
| 35-01-02 | 01 | 1 | MIGRATE-01 | unit | `cd services/twitch-listener && go test ./channels/...` | ✅ | ⬜ pending |
| 35-01-03 | 01 | 1 | MIGRATE-01 | build | `cd services/twitch-listener && go vet ./...` | ✅ | ⬜ pending |
| 35-02-01 | 02 | 2 | MIGRATE-01 | unit | `cd services/twitch-listener && go test ./cmd/... -run TestSDKWiring` | ❌ W0 | ⬜ pending |
| 35-02-02 | 02 | 2 | VERIFY-02 | unit | `cd services/twitch-listener && go test ./cmd/... -run TestGoroutineLeak` | ❌ W0 | ⬜ pending |
| 35-02-03 | 02 | 2 | VERIFY-02 | unit | `cd services/twitch-listener && go test ./...` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/twitch-listener/cmd/main_sdk_test.go` — smoke test stubs for MIGRATE-01 + VERIFY-02
- [ ] Add `go.uber.org/goleak` to `services/twitch-listener/go.mod` as test dependency

*Wave 0 is small: one test file + one dependency.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `messages_published_total` no >10% drop for 5min post-deploy | VERIFY-02 | Requires live Twitch IRC traffic + production Prometheus | Deploy, watch dashboard for 5 min, confirm metric stability |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
