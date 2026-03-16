---
phase: 32
slug: integration-wiring-fixes
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-16
---

# Phase 32 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (message-processor) + Vitest (frontend) |
| **Config file** | `services/message-processor/go.mod` / `frontend/vitest.config.ts` |
| **Quick run command** | `cd services/message-processor && go test ./enricher/... -v` and `cd frontend && npx vitest run src/app/overlay/__tests__/` |
| **Full suite command** | `cd services/message-processor && go test ./...` and `cd frontend && npx vitest run` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./enricher/... -v` and `npx vitest run src/app/overlay/__tests__/`
- **After every plan wave:** Run `go test ./... && cd frontend && npx vitest run`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 32-01-01 | 01 | 1 | BADGE-02 | unit | `go test ./enricher/... -run TestEnrich_PremiumBadge -v` | ✅ | ⬜ pending |
| 32-02-01 | 02 | 1 | PREM-02 | unit | `npx vitest run src/app/overlay/__tests__/ws-message-parse.test.ts` | ❌ W0 | ⬜ pending |
| 32-02-02 | 02 | 1 | PREM-02 | unit | `npx vitest run src/app/overlay/__tests__/gradient-render.test.tsx` | ✅ | ⬜ pending |
| 32-03-01 | 03 | 1 | PREM-03, PREM-04 | manual | `curl http://localhost:8080/api/v1/auth/viewer/catalog/frames` | manual | ⬜ pending |
| 32-03-02 | 03 | 1 | PREM-05, WEB-03, WEB-04 | manual | `curl -H "Authorization: Bearer ..." http://localhost:8080/api/v1/admin/cosmetics/frames` | manual | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `frontend/src/app/overlay/__tests__/ws-message-parse.test.ts` — covers PREM-02: ws.onmessage parse guard converts JSON string to `NameGradient` object before `buildGradientCSS` is called

*All Go tests for BADGE-02 already exist in `viewer_badge_enricher_test.go`. Existing `TestEnrich_PremiumBadge` covers the premium badge injection path. No new Go test files needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Catalog routes return 200 | PREM-03, PREM-04, WEB-03, WEB-04 | Integration test requires running gateway + auth-service | `curl http://localhost:8080/api/v1/auth/viewer/catalog/frames` — expect 200 |
| Admin cosmetics routes return 200 | PREM-05 | Requires admin JWT + running services | `curl -H "Authorization: Bearer <admin-token>" http://localhost:8080/api/v1/admin/cosmetics/frames` — expect 200 |
| Premium badge appears in overlay | BADGE-02 | Requires live viewer session with premium flag | Create premium viewer, send message, verify badge in overlay UI |
| Gradient renders without TypeError | PREM-02 | Requires live WebSocket message flow | Load overlay, check browser console for no TypeError on gradient messages |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
