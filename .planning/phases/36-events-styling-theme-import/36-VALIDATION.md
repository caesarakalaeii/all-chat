---
phase: 36
slug: events-styling-theme-import
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-18
---

# Phase 36 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Vitest 2.x |
| **Config file** | `frontend/vitest.config.ts` |
| **Quick run command** | `cd frontend && npx vitest run --project unit` |
| **Full suite command** | `cd frontend && npx vitest run --project unit` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd frontend && npx vitest run --project unit`
- **After every plan wave:** Run `cd frontend && npx vitest run --project unit`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 36-01-01 | 01 | 1 | APPR-09 | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/EventsGroup.test.tsx` | ❌ W0 | ⬜ pending |
| 36-01-02 | 01 | 1 | APPR-09 | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/EventsGroup.test.tsx` | ❌ W0 | ⬜ pending |
| 36-01-03 | 01 | 1 | APPR-10 | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/EventsGroup.test.tsx` | ❌ W0 | ⬜ pending |
| 36-02-01 | 02 | 1 | APPR-09 | unit | `cd frontend && npx vitest run --project unit src/lib/utils/__tests__/visual-settings-to-css.test.ts` | ✅ (update) | ⬜ pending |
| 36-02-02 | 02 | 1 | VISM-02 | unit | `cd frontend && npx vitest run --project unit src/lib/utils/__tests__/theme-css-parser.test.ts` | ❌ W0 | ⬜ pending |
| 36-02-03 | 02 | 1 | VISM-02 | unit | `cd frontend && npx vitest run --project unit src/lib/utils/__tests__/theme-css-parser.test.ts` | ❌ W0 | ⬜ pending |
| 36-03-01 | 03 | 2 | VISM-02 | unit | `cd frontend && npx vitest run --project unit` | ❌ W0 | ⬜ pending |
| 36-03-02 | 03 | 2 | VISM-04 | manual | See manual verifications below | N/A | ⬜ pending |
| 36-03-03 | 03 | 2 | APPR-10 | unit | `cd frontend && npx vitest run --project unit` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `frontend/src/components/appearance/__tests__/EventsGroup.test.tsx` — stub tests for APPR-09, APPR-10 (show/hide toggles, size modifier sliders, onChange wiring)
- [ ] `frontend/src/lib/utils/__tests__/theme-css-parser.test.ts` — stub tests for VISM-02 (full parse, unknown properties ignored)
- [ ] Update `frontend/src/lib/utils/__tests__/visual-settings-to-css.test.ts` — add `membershipGiftSizeModifier` to Required fixture, update count assertion from 49 → 50

*All three files must exist before Wave 1 tasks begin.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| "Reset to theme defaults" restores parsed-theme appearance when theme was loaded | VISM-04 | Full page state transitions (parsedThemeSettings → visualSettings) not easily testable in unit scope | 1. Load overlay editor. 2. Apply a marketplace theme. 3. Change font size slider. 4. Click "Reset to theme defaults". 5. Verify all controls revert to theme values and preview updates. |
| "Reset to theme defaults" with no theme clears all visual overrides | VISM-04 | Same reason — requires live page interaction | 1. Load overlay editor (no theme applied). 2. Change several controls. 3. Click "Reset to theme defaults". 4. Verify controls clear and preview updates. |
| Live preview updates without save for all EventsGroup controls | APPR-10 | Requires running iframe preview | 1. Load editor with preview iframe visible. 2. Toggle each event type on/off. 3. Adjust each size modifier. 4. Verify preview updates immediately for each change. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
