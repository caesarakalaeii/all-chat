---
phase: 18
slug: revocation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-10
---

# Phase 18 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` + `testify/assert` + `testify/require` (backend); Jest (frontend) |
| **Config file** | none — `go test ./...` per service |
| **Quick run command** | `cd services/share-service && go test ./handlers/... -v -run TestRevoke` |
| **Full suite command** | `cd services/share-service && go test ./... && cd ../../frontend && npm test -- --passWithNoTests` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd services/share-service && go test ./handlers/... -v -run TestRevoke`
- **After every plan wave:** Run `cd services/share-service && go test ./... && cd ../../frontend && npm test -- --passWithNoTests`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 18-01-01 | 01 | 0 | SHARE-06, SHARE-07 | unit stub | `go test ./handlers/... -v -run TestRevoke` | ❌ W0 | ⬜ pending |
| 18-01-02 | 01 | 1 | SHARE-06, SHARE-07 | unit | `go test ./handlers/... -v -run TestRevokeShareRequest` | ❌ W0 | ⬜ pending |
| 18-01-03 | 01 | 1 | SHARE-06 | unit | `go test ./handlers/... -v -run TestRevokeShareRequest_AuthCheck` | ❌ W0 | ⬜ pending |
| 18-01-04 | 01 | 1 | SHARE-06 | unit | `go test ./handlers/... -v -run TestRevokeShareRequest_StatusCheck` | ❌ W0 | ⬜ pending |
| 18-01-05 | 01 | 1 | SHARE-07 | unit | `go test ./handlers/... -v -run TestRevokeShareRequest_SourceDeactivation` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/share-service/handlers/shares_revoke_test.go` — stubs for SHARE-06 auth check, status check, source deactivation (mock repo assertions)

*Frontend tests: RevocationConfirmModal test file added with modal component in Wave 1.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Transaction atomicity (share + sources update together) | SHARE-06, SHARE-07 | No DB in unit tests; requires integration DB | Run migration, call POST /shares/:id/revoke, verify both share_requests.status='revoked' AND overlay_chat_sources.is_active=false in DB |
| WebSocket share_revoked notification delivered | SHARE-06 | Requires live WS connection + api-gateway running | Open overlay in browser, revoke from dashboard, verify toast appears |
| Revoked source greyed out in overlay editor | SHARE-07 | Visual rendering check | Revoke a share, open overlay editor, verify shared_overlay row shows 50% opacity + red ✗ badge |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
