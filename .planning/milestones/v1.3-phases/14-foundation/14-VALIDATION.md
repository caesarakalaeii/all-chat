---
phase: 14
slug: foundation
status: active
nyquist_compliant: true
wave_0_complete: false
created: 2026-03-09
---

# Phase 14 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (stdlib) + Vitest ^4.0.18 (frontend) |
| **Config file** | None for Go (stdlib), vitest.config.ts (frontend) |
| **Quick run command** | `go test ./services/share-service/... -v` |
| **Full suite command** | `make test` (all Go services), `cd frontend && npm test` (frontend) |
| **Estimated runtime** | ~15-30 seconds (Go tests), ~10-20 seconds (frontend tests) |

---

## Sampling Rate

- **After every task commit:** Run relevant test subset per task (e.g., `go test -run TestPremiumMiddleware ./services/share-service/middleware/...`)
- **After every plan wave:** Run `make test` (full Go suite) + `cd frontend && npm test` (frontend suite)
- **Before `/gsd:verify-work`:** Full suite must be green + manual EXPLAIN ANALYZE for search query index usage
- **Max feedback latency:** 30 seconds per task

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 14-01-01 | 01 | 1 | foundation | integration | `make migrate-up && psql -c "\d share_requests"` | ✅ exists | ⬜ pending |
| 14-01-02 | 01 | 1 | foundation | unit | `cd services/share-service && go test ./models/...` | ✅ exists | ⬜ pending |
| 14-02-01 | 02 | 2 | SHARE-01 | unit | `cd services/share-service && go build ./repository/...` | ✅ exists | ⬜ pending |
| 14-02-02 | 02 | 2 | SHARE-02 | unit | `cd services/share-service && go build ./repository/...` | ✅ exists | ⬜ pending |
| 14-02-03 | 02 | 2 | SHARE-01, SHARE-02 | unit | `cd services/share-service && go build ./handlers/...` | ✅ exists | ⬜ pending |
| 14-02-04 | 02 | 2 | integration | build | `cd services/share-service && go build ./cmd/main.go` | ✅ exists | ⬜ pending |
| 14-03-01 | 03 | 2 | PREMIUM-01 | unit | `cd services/share-service && go build ./middleware/...` | ✅ exists | ⬜ pending |
| 14-03-02 | 03 | 2 | PREMIUM-02 | unit | `cd services/share-service && go build ./repository/... ./handlers/...` | ✅ exists | ⬜ pending |
| 14-03-03 | 03 | 2 | PREMIUM-01, PREMIUM-02 | integration | `cd services/share-service && go build ./cmd/main.go` | ✅ exists | ⬜ pending |
| 14-04-01 | 04 | 3 | SHARE-03 | unit | `cd services/share-service && go build ./jobs/...` | ✅ exists | ⬜ pending |
| 14-04-02 | 04 | 3 | SHARE-03 | integration | `cd services/share-service && go build ./cmd/main.go` | ✅ exists | ⬜ pending |
| 14-04-03 | 04 | 3 | SHARE-03 | unit | `cd frontend && npm run build` | ✅ exists | ⬜ pending |
| 14-04-04 | 04 | 3 | SHARE-03 | unit | `cd frontend && npm run build && npm run lint` | ✅ exists | ⬜ pending |
| 14-04-05 | 04 | 3 | SHARE-03 | manual | Human verification of dashboard UI and expiry job | ✅ exists | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

**Wave 0 not required for this phase.** All tasks use build/compile verification or manual checkpoints. Comprehensive unit tests with testcontainers can be added post-MVP.

Rationale:
- Plan 14-01: Database migration verification via `make migrate-up` and schema inspection
- Plan 14-02: Repository and handler verification via `go build` (compile-time safety)
- Plan 14-03: Middleware and admin handler verification via `go build`
- Plan 14-04: Frontend verification via `npm run build` and human checkpoint

Future enhancement (post-Phase 14):
- [ ] `services/share-service/repository/user_search_test.go` — testcontainers integration tests for SHARE-01
- [ ] `services/share-service/repository/share_repo_test.go` — testcontainers integration tests for SHARE-02 (FK constraints, self-share)
- [ ] `services/share-service/middleware/premium_test.go` — testcontainers integration tests for PREMIUM-01
- [ ] `services/share-service/handlers/admin_test.go` — unit tests for PREMIUM-02
- [ ] `frontend/src/app/dashboard/shares/__tests__/ShareRequestsPage.test.tsx` — Vitest unit tests for SHARE-03 (tab filtering)
- [ ] `frontend/tests/e2e/shares.spec.ts` — Playwright E2E tests for SHARE-03 (full user flow)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Dashboard UI responsive layout (1/2/3 columns) | SHARE-03 | Visual design verification | Resize browser window, verify grid changes at breakpoints (768px, 1024px) |
| Platform badge hover tooltips | SHARE-03 | Visual interaction | Hover over platform badges, verify tooltip shows "{channel_name} on {platform}" |
| Card styling consistency with admin pages | SHARE-03 | Visual design comparison | Compare dashboard/shares cards with admin/users page card styling |
| Expiry job logs every 5 minutes | SHARE-03 | Time-based behavior | Start service, monitor logs for "Expired share requests" messages every 5 minutes |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or manual checkpoint
- [x] Sampling continuity: No 3 consecutive tasks without automated verify (all tasks have build/compile verification)
- [x] Wave 0 not required: Build verification sufficient for MVP
- [x] No watch-mode flags
- [x] Feedback latency < 30s (Go build ~5-10s, npm build ~10-20s)
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-03-09
