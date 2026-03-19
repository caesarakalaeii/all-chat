---
phase: 37
slug: migrate-youtube-innertube-and-discord-listener
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-17
---

# Phase 37 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` package (stdlib) + goleak v1.3.0 |
| **Config file** | none — standard `go test` |
| **Quick run command** | `go test ./cmd/... -count=1 -run TestListenerBase_StartStop_NoGoroutineLeak` (per-service) |
| **Full suite command** | `go test ./... -race -count=1` (per-service) + `make build-all` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./cmd/... -count=1 -run TestListenerBase_StartStop_NoGoroutineLeak` (in the affected service directory)
- **After every plan wave:** Run `go test ./... -race -count=1` (per-service) + `make build-all`
- **Before `/gsd:verify-work`:** Full suite must be green in both services
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 37-01-01 | 01 | 1 | MIGRATE-04, MIGRATE-05 | build | `cd shared && go build ./listener/... && go vet ./listener/...` | ✅ | ⬜ pending |
| 37-01-02 | 01 | 1 | MIGRATE-04 | build/unit | `cd services/youtube-listener-innertube && go get go.uber.org/goleak@v1.3.0 && go build ./...` | ✅ | ⬜ pending |
| 37-01-03 | 01 | 1 | MIGRATE-05 | build/unit | `cd services/discord-listener && go get go.uber.org/goleak@v1.3.0 && go build ./...` | ✅ | ⬜ pending |
| 37-02-01 | 02 | 1 | MIGRATE-04 | smoke/unit | `cd services/youtube-listener-innertube && go test ./cmd/... -count=1 -run TestListenerBase_StartStop_NoGoroutineLeak` | ❌ W0 | ⬜ pending |
| 37-02-02 | 02 | 1 | MIGRATE-04 | build/unit | `cd services/youtube-listener-innertube && go build ./cmd/... && go test ./... -count=1` | ✅ | ⬜ pending |
| 37-03-01 | 03 | 2 | MIGRATE-05 | smoke/unit | `cd services/discord-listener && go test ./cmd/... -count=1 -run TestListenerBase_StartStop_NoGoroutineLeak` | ❌ W0 | ⬜ pending |
| 37-03-02 | 03 | 2 | MIGRATE-05 | build/unit | `cd services/discord-listener && go build ./cmd/... && go test ./... -count=1` | ✅ | ⬜ pending |
| 37-final | — | — | Both | build | `make build-all` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/youtube-listener-innertube/cmd/main_sdk_test.go` — goroutine leak smoke test for MIGRATE-04; requires `go.uber.org/goleak@v1.3.0` in go.mod
- [ ] `services/discord-listener/cmd/main_sdk_test.go` — goroutine leak smoke test for MIGRATE-05; requires `go.uber.org/goleak@v1.3.0` in go.mod
- [ ] goleak install: `go get go.uber.org/goleak@v1.3.0` in each service directory (both services)

*Both smoke test files are created as part of Plan 01 (SDK + test scaffolding wave) — goleak is added to go.mod in that same plan.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| InnerTube message rate stable after migration | MIGRATE-04 | Requires live YouTube InnerTube session; no mock available | Deploy to staging, confirm messages appear in overlay at expected rate for 5+ minutes |
| Discord relay continues functioning (loop-safety filter intact) | MIGRATE-05 | Requires live Discord Gateway connection; race condition cannot be unit-tested | Deploy to staging, send test messages in Discord channel, verify relay to overlay; confirm no duplicate messages |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
