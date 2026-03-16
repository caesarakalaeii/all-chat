---
phase: 31
slug: all-chat-platform-badges
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-16
---

# Phase 31 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go test (stdlib) + miniredis for enricher; Vitest for frontend |
| **Config file** | None (go test ./...) / vitest.config.ts |
| **Quick run command** | `cd services/message-processor && go test ./enricher/... -v` |
| **Full suite command** | `make test` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd services/message-processor && go test ./enricher/... -v`
- **After every plan wave:** Run `make test`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 31-01-01 | 01 | 1 | BADGE-04 | manual | `psql -c "SELECT * FROM badge_definitions"` | ❌ W0 | ⬜ pending |
| 31-01-02 | 01 | 1 | BADGE-01, BADGE-02 | unit | `go test ./enricher/... -run TestEnrich_AdminBadge` | ❌ W0 | ⬜ pending |
| 31-01-03 | 01 | 1 | BADGE-01, BADGE-02 | unit | `go test ./enricher/... -run TestEnrich_PremiumBadge` | ❌ W0 | ⬜ pending |
| 31-02-01 | 02 | 2 | BADGE-03 | unit | `npx vitest run src/lib/badgeOrder` | ✅ extend | ⬜ pending |
| 31-02-02 | 02 | 2 | BADGE-01, BADGE-03 | manual | Overlay visual — allchat badge renders before platform badges | N/A | ⬜ pending |
| 31-02-03 | 02 | 2 | BADGE-02, BADGE-03 | manual | Overlay visual — premium badge renders before platform badges | N/A | ⬜ pending |
| 31-03-01 | 03 | 2 | BADGE-01, BADGE-02 | manual | Extension visual — allchat/premium badges visible in extension | N/A | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/message-processor/enricher/viewer_badge_enricher_test.go` — add `TestEnrich_AdminBadge` and `TestEnrich_PremiumBadge` test functions (file exists; extend fakeViewerDB to 8-value queryFn)
- [ ] `frontend/src/lib/badgeOrder.test.ts` — create with allchat/premium sort order test cases (file may not exist; create with new cases verifying `allchat` sorts before `moderator` and `premium` sorts before `moderator`)

*Existing test infrastructure covers the enricher pattern — only new test functions needed, not new files for the enricher.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `badge_definitions` table seeded with 2 rows | BADGE-04 | DDL migration — no runtime assertion | `psql -U allchat allchat -c "SELECT * FROM badge_definitions;"` — expect 2 rows: allchat, premium |
| All-Chat badge renders in overlay (before position) | BADGE-01, BADGE-03 | Visual rendering with component | Open overlay with admin viewer chat message; verify allchat badge appears before platform badges |
| Premium badge renders in overlay | BADGE-02, BADGE-03 | Visual rendering with component | Open overlay with premium viewer; verify purple gem badge appears |
| Admin grant/revoke via admin users page | BADGE-01 | UI interaction | Toggle is_admin on admin/users page; verify badge appears/disappears on next message |
| Premium grant/revoke via admin users page | BADGE-02 | UI interaction | Toggle is_premium on admin/users page; verify badge appears/disappears after cache TTL |
| Extension renders all-chat/premium badges | BADGE-01, BADGE-02 | Extension visual — not in test suite | Load extension with admin/premium viewer; verify badges visible (not dropped) |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
