---
phase: 15
slug: share-acceptance
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-09
---

# Phase 15 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (backend), vitest (frontend) |
| **Config file** | go.mod (backend), vitest.config.ts (frontend) |
| **Quick run command** | `make test-quick` |
| **Full suite command** | `make test` |
| **Estimated runtime** | ~8 seconds |

---

## Sampling Rate

- **After every task commit:** Run `make test-quick`
- **After every plan wave:** Run `make test`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 8 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 15-01-01 | 01 | 0 | SHARE-04 | unit | `go test ./services/share-service/acceptance/...` | ❌ W0 | ⬜ pending |
| 15-01-02 | 01 | 0 | SHARE-04 | unit | `go test ./services/share-service/cycles/...` | ❌ W0 | ⬜ pending |
| 15-01-03 | 01 | 1 | SHARE-04 | unit | `go test ./services/share-service/acceptance/...` | ❌ W0 | ⬜ pending |
| 15-01-04 | 01 | 1 | SHARE-04 | unit | `go test ./services/share-service/cycles/...` | ❌ W0 | ⬜ pending |
| 15-02-01 | 02 | 0 | SHARE-04 | unit | `npm test -- AcceptModal.test.tsx` | ❌ W0 | ⬜ pending |
| 15-02-02 | 02 | 1 | SHARE-04 | unit | `npm test -- AcceptModal.test.tsx` | ❌ W0 | ⬜ pending |
| 15-02-03 | 02 | 1 | SHARE-05 | unit | `npm test -- AddSourceModal.test.tsx` | ❌ W0 | ⬜ pending |
| 15-03-01 | 03 | 0 | SHARE-08 | unit | `go test ./services/message-processor/dedup/...` | ✅ | ⬜ pending |
| 15-03-02 | 03 | 1 | SHARE-08 | unit | `go test ./services/message-processor/dedup/...` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/share-service/acceptance/acceptance_test.go` — stubs for acceptance endpoint tests (SHARE-04)
- [ ] `services/share-service/cycles/cycles_test.go` — stubs for cycle detection tests (SHARE-04)
- [ ] `frontend/src/app/dashboard/shares/components/__tests__/AcceptModal.test.tsx` — stubs for AcceptModal tests (SHARE-04)
- [ ] `frontend/src/app/dashboard/shares/components/__tests__/AddSourceModal.test.tsx` — stubs for AddSourceModal tests (SHARE-05)
- [ ] `frontend/vitest.config.ts` — if not exists (vitest framework setup)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Acceptance modal visual styling | SHARE-04 | Visual verification | Open acceptance modal, verify spacing/shadows/animations match design system |
| Add-source modal appears after acceptance | SHARE-05 | Timing/UX verification | Accept share, verify second modal opens immediately with correct overlay data |
| WebSocket notification delivery to sender | SHARE-05 | Network/realtime verification | Accept share while sender online, verify sender sees add-source modal instantly |
| Cycle error message clarity | SHARE-04 | UX verification | Attempt circular share, verify error message explains problem clearly |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 8s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
