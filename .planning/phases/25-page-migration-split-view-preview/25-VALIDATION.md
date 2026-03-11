---
phase: 25
slug: page-migration-split-view-preview
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-11
---

# Phase 25 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Vitest 4.0.18 + `@storybook/addon-vitest` 10.2.17 + `@storybook/addon-a11y` 10.2.17 |
| **Config file** | `frontend/.storybook/main.ts` + `frontend/.storybook/vitest.setup.ts` |
| **Quick run command** | `cd frontend && npm run storybook` |
| **Full suite command** | `cd frontend && npm test` |
| **Estimated runtime** | ~30 seconds (unit tests); Storybook interactive |

---

## Sampling Rate

- **After every task commit:** Visual inspection in browser (`make frontend-dev`) — check the specific page changed
- **After every plan wave:** Run Storybook and verify a11y panel shows no errors in 'error' mode
- **Before `/gsd:verify-work`:** Full Storybook suite green + manual breakpoint check at 375px/768px/1920px
- **Max feedback latency:** 30 seconds (unit tests); immediate (browser visual check)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 25-01-01 | 01 | 0 | PAGE-01, PAGE-08 | Storybook story | `npm run storybook` | ❌ W0 | ⬜ pending |
| 25-01-02 | 01 | 0 | PAGE-02, PAGE-09, PAGE-10 | Storybook story | `npm run storybook` | ❌ W0 | ⬜ pending |
| 25-01-03 | 01 | 0 | PAGE-03, FEAT-01 | Storybook story | `npm run storybook` | ❌ W0 | ⬜ pending |
| 25-01-04 | 01 | 0 | PAGE-05, PAGE-08 | Storybook story | `npm run storybook` | ❌ W0 | ⬜ pending |
| 25-01-05 | 01 | 0 | PAGE-06 | Storybook story | `npm run storybook` | ❌ W0 | ⬜ pending |
| 25-02-01 | 02 | 1 | PAGE-01 | Storybook visual + a11y | `npm run storybook` | ❌ W0 | ⬜ pending |
| 25-02-02 | 02 | 1 | PAGE-02, PAGE-09, PAGE-10 | Storybook visual + a11y | `npm run storybook` | ❌ W0 | ⬜ pending |
| 25-03-01 | 03 | 2 | PAGE-03, FEAT-01, FEAT-02, FEAT-03, FEAT-04 | Storybook + unit test | `npm test` + `npm run storybook` | ❌ W0 | ⬜ pending |
| 25-04-01 | 04 | 3 | PAGE-05, PAGE-06, PAGE-08 | Storybook a11y | `npm run storybook` | ❌ W0 | ⬜ pending |
| 25-05-01 | 05 | 4 | PAGE-07, PAGE-08 | Manual browser resize + Storybook | Manual + `npm run storybook` | N/A | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `frontend/src/components/AppNav.tsx` — shared nav component (prerequisite for all page stories)
- [ ] `frontend/src/stories/LandingPage.stories.tsx` — covers PAGE-01, PAGE-08 (a11y for login buttons)
- [ ] `frontend/src/stories/Dashboard.stories.tsx` — covers PAGE-02, PAGE-08, PAGE-09, PAGE-10
- [ ] `frontend/src/stories/OverlayEditor.stories.tsx` — covers PAGE-03, FEAT-01, FEAT-02
- [ ] `frontend/src/stories/Settings.stories.tsx` — covers PAGE-05, PAGE-08
- [ ] `frontend/src/stories/AdminLayout.stories.tsx` — covers PAGE-06

*All 6 story files are new — existing Storybook infrastructure covers all tooling needs.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Responsive layout at 375px/768px/1920px | PAGE-07 | CSS layout changes can't be fully automated | Open browser DevTools → set viewport to each size, verify grid/stack/side-by-side behavior |
| Split-view mobile vertical stack | FEAT-03 | Requires real viewport resize | Set browser width < 768px, confirm config on top, preview below |
| Preview iframe preserves overlay CSS | FEAT-04 | Visual marketplace theme check | Open overlay editor, verify themes render correctly in iframe split-view |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
