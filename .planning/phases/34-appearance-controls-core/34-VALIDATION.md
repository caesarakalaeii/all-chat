---
phase: 34
slug: appearance-controls-core
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-18
---

# Phase 34 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Vitest 4.x |
| **Config file** | `frontend/vitest.config.ts` |
| **Quick run command** | `cd frontend && npx vitest run --project unit src/components/appearance` |
| **Full suite command** | `cd frontend && npx vitest run --project unit` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd frontend && npx vitest run --project unit src/lib/utils/__tests__/visual-settings-to-css.test.ts`
- **After every plan wave:** Run `cd frontend && npx vitest run --project unit`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** ~15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 34-01-01 | 01 | 0 | APPR-01 | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/CollapsibleSection.test.tsx` | ❌ W0 | ⬜ pending |
| 34-01-02 | 01 | 0 | APPR-01 | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/TypographyGroup.test.tsx` | ❌ W0 | ⬜ pending |
| 34-01-03 | 01 | 1 | APPR-01 | unit | `cd frontend && npx vitest run --project unit src/lib/utils/__tests__/visual-settings-to-css.test.ts` | ✅ (extend) | ⬜ pending |
| 34-02-01 | 02 | 0 | APPR-02 | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/ColorsGroup.test.tsx` | ❌ W0 | ⬜ pending |
| 34-02-02 | 02 | 2 | APPR-02 | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/ColorsGroup.test.tsx` | ❌ W0 | ⬜ pending |
| 34-03-01 | 03 | 0 | APPR-03, APPR-04, APPR-08 | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/BackgroundGroup.test.tsx` | ❌ W0 | ⬜ pending |
| 34-03-02 | 03 | 3 | APPR-03, APPR-04, APPR-08 | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/BackgroundGroup.test.tsx` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `frontend/src/components/appearance/__tests__/CollapsibleSection.test.tsx` — stubs for CollapsibleSection localStorage persistence and open/close behavior
- [ ] `frontend/src/components/appearance/__tests__/TypographyGroup.test.tsx` — stubs for APPR-01 (font family picker, weight select, sliders)
- [ ] `frontend/src/components/appearance/__tests__/ColorsGroup.test.tsx` — stubs for APPR-02 (color pickers for message/username/timestamp text)
- [ ] `frontend/src/components/appearance/__tests__/BackgroundGroup.test.tsx` — stubs for APPR-03, APPR-04, APPR-08 (overlay bg, bubble bg, border, padding, blur)
- [ ] Extend `frontend/src/lib/utils/__tests__/visual-settings-to-css.test.ts` — add cases for `usernameFontFamily`, `timestampFontFamily`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live preview iframe updates in real-time when sliders/pickers change | APPR-01–04, APPR-08 | Requires real browser + iframe postMessage | Open overlay editor, adjust any control, verify preview updates within 100ms |
| Font family Combobox shows live preview of selected font | APPR-01 | Visual rendering test | Open font picker, select each font, confirm preview renders in that font |
| Color pickers open/close correctly via @base-ui/react Popover | APPR-02 | Interactive UI behavior | Click each color swatch, verify picker opens; click outside, verify it closes |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
