---
phase: 24
slug: component-library-setup-customization
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-10
---

# Phase 24 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Vitest 4.0.18 + `@storybook/addon-vitest` 10.2.17 |
| **Config file** | `frontend/vitest.config.ts` |
| **Quick run command** | `cd frontend && npx vitest run --project unit` |
| **Full suite command** | `cd frontend && npx vitest run` (unit + storybook browser tests) |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd frontend && npx tsc --noEmit`
- **After every plan wave:** Run `cd frontend && npx vitest run --project storybook`
- **Before `/gsd:verify-work`:** Full suite must be green + `grep -c '!important' frontend/src/styles/events.css` returns 0
- **Max feedback latency:** ~30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 24-xx-01 | Wave 0 | 0 | COMP-01..07 | Story stub creation | `cd frontend && npx tsc --noEmit` | ❌ W0 | ⬜ pending |
| COMP-09 check | 01 | 1 | COMP-09 | Shell check | `grep -c '!important' frontend/src/styles/events.css` | n/a | ⬜ pending |
| COMP-01 Card | 01 | 1 | COMP-01, COMP-02, COMP-04 | Storybook story | `cd frontend && npx vitest run --project storybook` | ❌ W0 | ⬜ pending |
| COMP-01 Input | 01 | 1 | COMP-01, COMP-02 | Storybook story | `cd frontend && npx vitest run --project storybook` | ❌ W0 | ⬜ pending |
| COMP-01 Badge | 01 | 1 | COMP-01, COMP-06 | Storybook story | `cd frontend && npx vitest run --project storybook` | ❌ W0 | ⬜ pending |
| COMP-05 Button gradient | 01 | 1 | COMP-05 | Storybook story | `cd frontend && npx vitest run --project storybook` | ❌ W0 | ⬜ pending |
| COMP-07 Skeleton | 01 | 1 | COMP-07 | Storybook story | `cd frontend && npx vitest run --project storybook` | ❌ W0 | ⬜ pending |
| COMP-01 Dialog | 02 | 2 | COMP-01, COMP-02 | Storybook story | `cd frontend && npx vitest run --project storybook` | ❌ W0 | ⬜ pending |
| COMP-01 Toast | 02 | 2 | COMP-01 | Storybook story | `cd frontend && npx vitest run --project storybook` | ❌ W0 | ⬜ pending |
| A11y gate | 02 | 2 | All | Storybook a11y error mode | `cd frontend && npx vitest run --project storybook` | W0: preview.ts change | ⬜ pending |
| COMP-08 bundle | 03 | 3 | COMP-08 | Manual | `cd frontend && npx next build 2>&1 \| grep "First Load"` | n/a | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `frontend/src/stories/Card.stories.tsx` — stubs for COMP-01, COMP-02, COMP-04
- [ ] `frontend/src/stories/Input.stories.tsx` — stubs for COMP-01, COMP-02
- [ ] `frontend/src/stories/Badge.stories.tsx` — stubs for COMP-01, COMP-06
- [ ] `frontend/src/stories/Dialog.stories.tsx` — stubs for COMP-01, COMP-02
- [ ] `frontend/src/stories/Toast.stories.tsx` — stubs for COMP-01
- [ ] `frontend/src/stories/Skeleton.stories.tsx` — stubs for COMP-07
- [ ] `.storybook/preview.ts` — add `import '../src/app/globals.css'` so CSS tokens render in stories
- [ ] `.storybook/preview.ts` — change `test: 'todo'` to `test: 'error'` as FINAL step after all stories pass

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Hover scale/shadow transitions visible | COMP-04 | Visual animation | Open Storybook, hover each component, confirm CSS transition plays |
| Gradient CTA variant visually correct | COMP-05 | Visual color accuracy | Confirm `#9146FF → #69C9D0` matches nav active underline gradient |
| Platform glow dot colors match platform identity | COMP-06 | Visual brand check | Inspect each platform badge in Storybook, confirm dot color matches platform brand |
| Toast stacking behavior (new pushes up) | COMP-01 | Runtime animation | Trigger 3 toasts rapidly, confirm stacking direction is correct |
| Bundle size delta < 100KB | COMP-08 | Build output | Run `npx next build`, compare First Load JS before/after phase |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
