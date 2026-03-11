---
phase: 17
slug: message-routing
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-10
---

# Phase 17 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing + testify v1.11.1 |
| **Config file** | none — standard `go test ./...` |
| **Quick run command** | `cd services/message-processor && go test ./router/... -v` |
| **Full suite command** | `cd services/message-processor && go test ./... && cd ../overlay-manager && go test ./...` |
| **Estimated runtime** | ~10 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd services/message-processor && go test ./router/... -v`
- **After every plan wave:** Run `cd services/message-processor && go test ./... && cd ../overlay-manager && go test ./...`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 17-01-01 | 01 | 1 | SOURCE-04 | unit | `cd services/message-processor && go test ./router/... -run TestFindOverlaysForMessage -v` | ❌ W0 | ⬜ pending |
| 17-01-02 | 01 | 1 | SOURCE-04 | unit | `cd services/overlay-manager && go test ./handlers/... -run TestHandleAddSource_SharedOverlay -v` | ❌ W0 | ⬜ pending |
| 17-01-03 | 01 | 1 | SOURCE-05 | manual | `make frontend-messages` + observe in browser | N/A | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/message-processor/router/overlay_router_test.go` — covers SOURCE-04 (router UNION logic: direct only, shared only, both, no match, revoked share excluded)
- [ ] Extend `services/overlay-manager/handlers/sources_shared_overlay_test.go` — add `TestHandleAddSource_SharedOverlay_IsActiveTrue` asserting `source.IsActive == true` after creation

*No new framework installs needed — testify and go test already available in both modules.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Recipient overlay receives message on its own `overlay:{id}` pub/sub channel with recipient's display settings applied | SOURCE-05 | Requires live WebSocket connection and browser rendering to verify isolation | Start services with `make frontend-dev`, seed data with `make frontend-seed`, run `make frontend-messages`, open recipient overlay in browser and verify messages appear with recipient CSS |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
