---
phase: 34
slug: sdk-package-definition
status: draft
nyquist_compliant: true
wave_0_complete: false
created: 2026-03-17
---

# Phase 34 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (stdlib) + testify v1.11.1 + goleak v1.3.0 (new — Wave 0 installs) |
| **Config file** | none — `go test` invoked per module |
| **Quick run command** | `cd shared && go test ./listener/... -v -race` |
| **Full suite command** | `cd shared && go test ./... -race && make build-all` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd shared && go test ./listener/... -race`
- **After every plan wave:** Run `cd shared && go test ./... -race && make build-all`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 34-01-01 | 01 | 1 | SDK-08 | unit | `cd shared && go test ./coordination/... -run TestJWT -race` | ✅ (needs update) | ⬜ pending |
| 34-01-02 | 01 | 1 | SDK-03 | compile | `cd shared && go build ./listener/... && cd services/kick-listener && go build ./... && cd services/twitch-listener && go build ./...` | ❌ new | ⬜ pending |
| 34-01-03 | 01 | 1 | SDK-03 | compile | `cd services/twitch-listener && go build ./... && cd services/kick-listener && go build ./... && cd services/twitch-eventsub-listener && go build ./...` | ❌ new | ⬜ pending |
| 34-02-01 | 02 | 2 | SDK-07 | grep | `grep "go.uber.org/goleak" shared/go.mod` | ❌ new | ⬜ pending |
| 34-02-02 | 02 | 2 | SDK-01/SDK-04/SDK-05/SDK-07 | compile+unit | `cd shared && go build ./listener/... && go test ./listener/... -run TestEnv -race` | ❌ new | ⬜ pending |
| 34-02-03 | 02 | 2 | VERIFY-01 | build smoke | `make build-all` | ❌ new | ⬜ pending |
| 34-03-01 | 03 | 3 | SDK-01/SDK-05 | unit | `cd shared && go test ./listener/... -run TestListenerBase -race` | ❌ new | ⬜ pending |
| 34-03-02 | 03 | 3 | SDK-04 | unit | `cd shared && go test ./listener/... -run TestShutdownCoordinator -race` | ❌ new | ⬜ pending |
| 34-03-03 | 03 | 3 | SDK-02 | unit | `cd shared && go test ./listener/... -run TestLeadershipListener -race` | ❌ new | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `shared/listener/` directory with 4 files: `base.go`, `leadership.go`, `channel_manager.go`, `shutdown.go`
- [ ] `shared/listener/config.go` or config embedded in `base.go` — `ListenerConfig` struct + `Env()` helper
- [ ] `shared/listener/testutil/mock_coordinator.go` — behavioral mock with call count tracking
- [ ] `shared/listener/env_test.go` — `Env()` helper tests for wave 2 behavioral Nyquist check (created in Plan 34-02 Task 2)
- [ ] `shared/listener/base_test.go` — goroutine start/stop + jitter tests for SDK-01, SDK-05 (created in Plan 34-03 Task 1)
- [ ] `shared/listener/leadership_test.go` — nil-safe passthrough test for SDK-02 (created in Plan 34-03 Task 3)
- [ ] `shared/listener/shutdown_test.go` — ShutdownCoordinator tests for SDK-04 (created in Plan 34-03 Task 2)
- [ ] `shared/go.mod` — add `go.uber.org/goleak v1.3.0` test dependency (`cd shared && go get go.uber.org/goleak@v1.3.0`)
- [ ] `Makefile` — add `build-all` target covering all Go listener modules (covers VERIFY-01)

*(Existing `shared/coordination/client_jwt_test.go` needs serviceName arg update — not a new file.)*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Compile-time assertion in kick-listener satisfies ChannelManager interface | SDK-03 / VERIFY-02 | Deferred to Phase 35 per REQUIREMENTS.md scope boundary — assertions added per-listener during migration | Phase 35 plan will add `var _ listener.ChannelManager = (*channels.Manager)(nil)` to each listener's channels/manager.go |
| Compile-time assertion in twitch-listener satisfies ChannelManager interface | SDK-03 / VERIFY-02 | Same — deferred to Phase 35 | Phase 35 plan will add assertion to twitch-listener channels/manager.go |

---

## Nyquist Compliance

Each plan wave has at least 2 behavioral `go test` runs:

| Wave | Behavioral Tests |
|------|-----------------|
| Wave 1 (Plan 01) | `go test ./coordination/... -run TestJWT` (existing test updated) |
| Wave 2 (Plan 02) | `go test ./listener/... -run TestEnv` (3 Env tests in env_test.go) |
| Wave 3 (Plan 03) | `go test ./listener/... -run TestListenerBase` (5 tests) + `TestShutdownCoordinator` (3 tests) + `TestLeadershipListener` (2 tests) |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
