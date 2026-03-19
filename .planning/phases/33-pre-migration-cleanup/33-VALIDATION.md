---
phase: 33
slug: pre-migration-cleanup
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-17
---

# Phase 33 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go standard `testing` + `testify/assert` + `testify/require` |
| **Config file** | none — standard `go test ./...` per service module |
| **Quick run command** | `cd services/kick-listener && go test ./channels/... -count=1 -v` |
| **Full suite command** | `cd services/twitch-listener && go test ./... && cd ../kick-listener && go test ./... && cd ../../shared/coordination && go test ./...` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd services/kick-listener && go test ./channels/... -count=1` and `cd services/twitch-listener && go test ./channels/... -count=1`
- **After every plan wave:** Run `cd services/twitch-listener && go test ./... && cd ../kick-listener && go test ./... && cd ../../shared/coordination && go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** ~10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 33-01-01 | 01 | 1 | PREP-01 | unit | `cd services/kick-listener && go test ./channels/... -run TestManager_SourceIDNormalization -v` | ❌ W0 | ⬜ pending |
| 33-01-02 | 01 | 1 | PREP-01 | compile | `cd services/kick-listener && go build ./...` | ✅ | ⬜ pending |
| 33-01-03 | 01 | 1 | PREP-01 | unit | `cd services/twitch-listener && go test ./channels/... -v` | ✅ | ⬜ pending |
| 33-02-01 | 02 | 1 | PREP-02 | compile | `cd services/twitch-listener && go build ./... && cd ../kick-listener && go build ./...` | ✅ after change | ⬜ pending |
| 33-02-02 | 02 | 1 | PREP-02 | unit | `cd shared/coordination && go test ./... -run TestMigrationSubscriber -v` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/kick-listener/channels/manager_test.go` — create with `TestManager_SourceIDNormalization` stub for PREP-01
- [ ] `shared/coordination/migration_subscriber_test.go` — create with `TestMigrationSubscriber_ErrorHandling` stub for PREP-02

*Wave 0 must create these files before the code changes that satisfy PREP-01 and PREP-02.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Coordinator returns `{uuid}:platform` format in production | PREP-01 | Requires live coordinator connection | Check kick-listener logs after deploy: assigned channel IDs should be bare UUIDs, not `{uuid}:kick` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
