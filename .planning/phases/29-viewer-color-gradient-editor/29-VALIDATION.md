---
phase: 29
slug: viewer-color-gradient-editor
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-15
---

# Phase 29 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (backend), vitest / jest (frontend) |
| **Config file** | `services/api-gateway/go.mod`, `frontend/package.json` |
| **Quick run command** | `cd services/api-gateway && go test ./handlers/... -run TestCosmetics` |
| **Full suite command** | `cd services/api-gateway && go test ./... && cd ../../frontend && npm test -- --run` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd services/api-gateway && go test ./handlers/... -run TestCosmetics`
- **After every plan wave:** Run `cd services/api-gateway && go test ./... && cd ../../frontend && npm test -- --run`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 29-01-01 | 01 | 1 | VID-01, VID-02 | unit | `cd services/api-gateway && go test ./handlers/... -run TestCosmetics` | ❌ W0 | ⬜ pending |
| 29-01-02 | 01 | 1 | VID-01, VID-02 | unit | `cd services/api-gateway && go test ./... -run TestPatchCosmetics` | ❌ W0 | ⬜ pending |
| 29-01-03 | 01 | 1 | PREM-01 | unit | `cd services/api-gateway && go test ./... -run TestJWTClaims` | ❌ W0 | ⬜ pending |
| 29-02-01 | 02 | 2 | WEB-01, WEB-02 | unit | `cd frontend && npm test -- --run src/app/settings/viewer` | ❌ W0 | ⬜ pending |
| 29-02-02 | 02 | 2 | PREM-01, PREM-02 | unit | `cd frontend && npm test -- --run src/app/settings/viewer` | ❌ W0 | ⬜ pending |
| 29-02-03 | 02 | 2 | WEB-05 | manual | Browser: verify gradient editor hidden for non-premium | N/A | ⬜ pending |
| 29-03-01 | 03 | 3 | VID-02 | manual | Browser: verify overlay renders gradient username | N/A | ⬜ pending |
| 29-03-02 | 03 | 3 | VID-02 | manual | Browser: verify extension ChatContainer renders gradient | N/A | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/api-gateway/handlers/cosmetics_test.go` — stubs for gradient PATCH and mutual-exclusion (VID-01, VID-02)
- [ ] `services/api-gateway/handlers/jwt_test.go` — stubs for `IsPremium` JWT claim (PREM-01)
- [ ] `frontend/src/app/settings/viewer/__tests__/page.test.tsx` — stubs for settings page render, color picker, gradient editor visibility (WEB-01, WEB-02, WEB-05)

*Framework already installed; Wave 0 adds test file stubs only.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Overlay renders gradient username text | VID-02 | Requires live overlay page with real JWT and cosmetics data | 1. Set gradient via settings UI, 2. Open overlay URL, 3. Send chat message as that viewer, 4. Verify name appears as gradient |
| Extension ChatContainer renders gradient | VID-02 | Requires browser extension environment | 1. Set gradient via settings UI, 2. Load extension popup, 3. Verify chat name renders with gradient |
| Non-premium users cannot access gradient editor | WEB-05, PREM-01 | Requires JWT with `is_premium: false` | 1. Log in as non-premium user, 2. Navigate to `/settings/viewer`, 3. Verify gradient tab/section is not shown |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
