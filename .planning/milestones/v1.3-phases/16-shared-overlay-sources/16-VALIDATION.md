---
phase: 16
slug: shared-overlay-sources
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-10
---

# Phase 16 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (backend), vitest (frontend) |
| **Config file** | frontend/vitest.config.ts |
| **Quick run command** | `cd services/share-service && go test ./... && cd ../../frontend && npm test -- --run` |
| **Full suite command** | `cd services/share-service && go test ./... && cd services/api-gateway && go test ./... && cd ../../frontend && npm test -- --run` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run quick run command
- **After every plan wave:** Run full suite command
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 16-00-T1 | 00 | 0 | SOURCE-01, SOURCE-03 | unit stub | `cd services/overlay-manager && go test ./handlers/... -run TestHandleAddSource_SharedOverlay` | ❌ W0 | ⬜ pending |
| 16-00-T1b | 00 | 0 | SOURCE-02 | compile-time stub | `cd services/share-service && go build ./...` (fails until Plan 02 adds GetAcceptedShares) | ❌ W0 | ⬜ pending |
| 16-00-T2 | 00 | 0 | SOURCE-03 | unit stub | `cd frontend && npm test -- --run src/app/dashboard/shares/components/AddSourceModal.test.tsx` | ✅ | ⬜ pending |
| 16-01-T1 | 01 | 1 | SOURCE-01 | migration file | `ls migrations/032_shared_overlay_platform.sql && grep 'shared_overlay' ... && grep 'recipient_overlay_id' ...` | ❌ W0 | ⬜ pending |
| 16-01-T2 | 01 | 1 | SOURCE-01 | unit | `cd services/overlay-manager && go test ./models/... -v -run TestChatSource` | ✅ (file exists, needs new cases) | ⬜ pending |
| 16-02-T1 | 02 | 1 | SOURCE-02 | unit | `cd services/share-service && go test ./handlers/... -v -run TestGetAcceptedShares` | ❌ W0 | ⬜ pending |
| 16-02-T2 | 02 | 1 | SOURCE-02 | build | `cd services/api-gateway && go build ./...` | ✅ | ⬜ pending |
| 16-02-T3 | 02 | 1 | SOURCE-02 | build | `cd frontend && npm run build` | ✅ | ⬜ pending |
| 16-03-T1 | 03 | 2 | SOURCE-01, SOURCE-03 | unit | `cd services/overlay-manager && go test ./handlers/... -v -run TestHandleAddSource_SharedOverlay` | ❌ W0 | ⬜ pending |
| 16-03-T2 | 03 | 2 | SOURCE-02, SOURCE-03 | unit + build | `cd frontend && npm test -- --run src/app/dashboard/shares/components/AddSourceModal.test.tsx && npm run build` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/overlay-manager/handlers/sources_shared_overlay_test.go` — failing test for shared_overlay 403 response (no accepted share) and compile-time checks
- [ ] `services/share-service/handlers/shares_accepted_test.go` — compile-time check (`(*ShareHandler)(nil).GetAcceptedShares`) that fails until Plan 02 adds the method
- [ ] `frontend/src/app/dashboard/shares/components/AddSourceModal.test.tsx` — new test case asserting `overlaysApi.addSource` is called (currently fails because handler uses console.log)

Note: `services/api-gateway/handlers/shares_proxy_test.go` is NOT required as a Wave 0 stub. The api-gateway route registration (Plan 02 Task 2) is verified by `go build ./... && grep 'shares/accepted' services/api-gateway/cmd/main.go`. Build-only verification is sufficient for route registration because: (1) Gin panics at startup if routes conflict, so build + startup validates routing, and (2) the routes are pure proxy registrations with no logic to unit-test. Adding a test stub for a `proxyHandler.ForwardRequest` call would test the test framework, not business logic.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| "Shared Overlays" appears in add-source UI alongside Twitch/YouTube | SOURCE-02 | Visual rendering check | Navigate to overlay settings → Add Source → verify shared overlay option present |
| End-to-end add shared overlay as source | SOURCE-03 | Requires running services + DB | Accept share → click "Add Source" → verify source persists in overlay config |
| Shared overlay source persists after page reload | SOURCE-03 | Requires browser + DB state | Add source → reload page → verify source still listed |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
