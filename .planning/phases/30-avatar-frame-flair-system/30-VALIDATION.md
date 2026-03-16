---
phase: 30
slug: avatar-frame-flair-system
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-16
---

# Phase 30 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` + `net/http/httptest` (backend); Vitest (frontend) |
| **Config file** | `vitest.config.ts` in `frontend/` root |
| **Quick run command** | `cd services/auth-service && go test ./handlers/... -count=1` |
| **Full suite command (backend)** | `cd services/auth-service && go test ./... && cd ../message-processor && go test ./...` |
| **Full suite command (frontend)** | `cd frontend && npm run test` |
| **Estimated runtime** | ~10 seconds (backend), ~20 seconds (frontend) |

---

## Sampling Rate

- **After every task commit:** Run `cd services/auth-service && go test ./handlers/... -count=1`
- **After every plan wave:** Run full backend + frontend suite
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** ~10 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 30-01-01 | 01 | 1 | PREM-03, PREM-04 | unit (Go) | `go test ./handlers/... -run TestPatchCosmetics` | ⬜ W0 extend | ⬜ pending |
| 30-01-02 | 01 | 1 | PREM-05 | unit (Go) | `go test ./handlers/... -run TestAdminCosmetics` | ❌ W0 | ⬜ pending |
| 30-02-01 | 02 | 1 | PREM-03, PREM-04 | unit (Go) | `go test ./handlers/... -run TestPatchCosmetics` | ⬜ W0 extend | ⬜ pending |
| 30-03-01 | 03 | 2 | WEB-03, WEB-04 | unit (Vitest) | `cd frontend && npx vitest run --reporter=verbose` | ❌ W0 | ⬜ pending |
| 30-04-01 | 04 | 2 | PREM-03, PREM-04 | unit (Go) | `go test ./... -run TestEnricher` | ⬜ W0 extend | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/auth-service/handlers/admin_cosmetics_test.go` — stubs for PREM-05 (list/create/delete catalog items via mock DB interface)
- [ ] `services/auth-service/handlers/viewer_cosmetics_test.go` — extend existing mock to accept two new `*uuid.UUID` parameters (covers PREM-03, PREM-04 regression)
- [ ] `services/message-processor/enricher/viewer_badge_enricher_test.go` — extend existing enricher test fixture for `avatar_frame_url` / `avatar_flair_url` injection

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Frame renders at 1.4× size without overflow clipping in chat overlay | PREM-03 | Visual CSS layout, not reliably unit-testable | Start overlay, send messages with frames enabled; verify frame is not cropped at any edge |
| Flair renders at bottom-right 0.4× size in overlay and extension | PREM-04 | Visual CSS layout + separate extension repo | Verify in overlay page and browser extension simultaneously |
| Non-premium user sees locked items with lock icon, None remains selectable | WEB-03, WEB-04 | Requires premium gate state; no automated flag toggle | Log in as non-premium viewer, open /settings/viewer; premium catalog items should be greyed out |
| Live preview updates on catalog item click before Save | WEB-03, WEB-04 | UI state interaction | Click catalog items and verify UserAvatar updates immediately without PATCH |
| Admin URL preview shows 64×64 thumbnail on blur | PREM-05 | Visual browser behavior | In /admin/cosmetics, enter a valid image URL and tab out; thumbnail should appear |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 10s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
