---
phase: 34
slug: sdk-package-definition
status: draft
nyquist_compliant: false
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
| 34-01-01 | 01 | 0 | SDK-07 | unit | `cd shared && go test ./listener/... -run TestEnv -race` | ❌ W0 | ⬜ pending |
| 34-01-02 | 01 | 0 | SDK-05 | unit | `cd shared && go test ./listener/... -run TestListenerConfig -race` | ❌ W0 | ⬜ pending |
| 34-01-03 | 01 | 0 | SDK-03 | compile | `cd services/twitch-listener && go build ./...` | ❌ W0 | ⬜ pending |
| 34-01-04 | 01 | 0 | SDK-08 | unit | `cd shared && go test ./coordination/... -run TestJWTRefresh -race` | ✅ (needs update) | ⬜ pending |
| 34-02-01 | 02 | 1 | SDK-01 | unit | `cd shared && go test ./listener/... -run TestListenerBase_StartStop -race` | ❌ W0 | ⬜ pending |
| 34-02-02 | 02 | 1 | SDK-01 | unit | `cd shared && go test ./listener/... -run TestListenerBase_NoJitter -race` | ❌ W0 | ⬜ pending |
| 34-02-03 | 02 | 1 | SDK-02 | unit | `cd shared && go test ./listener/... -run TestLeadershipListener_NilSafe -race` | ❌ W0 | ⬜ pending |
| 34-02-04 | 02 | 1 | SDK-04 | unit | `cd shared && go test ./listener/... -run TestShutdownCoordinator -race` | ❌ W0 | ⬜ pending |
| 34-03-01 | 03 | 2 | SDK-06 | unit | `cd shared && go test ./listener/... -run TestListenerBase_FilteringDisabled -race` | ❌ W0 | ⬜ pending |
| 34-04-01 | 04 | 3 | VERIFY-01 | build smoke | `make build-all` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `shared/listener/` directory with 4 files: `base.go`, `leadership.go`, `channel_manager.go`, `shutdown.go`
- [ ] `shared/listener/config.go` or config embedded in `base.go` — `ListenerConfig` struct + `Env()` helper
- [ ] `shared/listener/testutil/mock_coordinator.go` — behavioral mock with call count tracking
- [ ] `shared/listener/base_test.go` — goroutine start/stop + jitter stubs for SDK-01, SDK-05
- [ ] `shared/listener/leadership_test.go` — nil-safe passthrough stub for SDK-02
- [ ] `shared/listener/env_test.go` — `Env()` helper stub for SDK-07
- [ ] `shared/go.mod` — add `go.uber.org/goleak v1.3.0` test dependency (`cd shared && go get go.uber.org/goleak@v1.3.0`)
- [ ] `Makefile` — add `build-all` target covering all Go listener modules (covers VERIFY-01)

*(Existing `shared/coordination/client_jwt_test.go` needs serviceName arg update — not a new file.)*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Compile-time assertion in kick-listener satisfies ChannelManager interface | SDK-03 | Requires kick `Start()` signature change; assertion is `var _ listener.ChannelManager = (*channels.Manager)(nil)` | Run `cd services/kick-listener && go build ./...` — must exit 0 |
| Compile-time assertion in twitch-listener satisfies ChannelManager interface | SDK-03 | Same — assertion in `services/twitch-listener/channels/manager.go` | Run `cd services/twitch-listener && go build ./...` — must exit 0 |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
