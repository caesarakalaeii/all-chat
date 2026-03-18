---
phase: 35
slug: appearance-controls-extended
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-18
---

# Phase 35 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Vitest (project `unit`) |
| **Config file** | `frontend/vitest.config.ts` |
| **Quick run command** | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/` |
| **Full suite command** | `cd frontend && npx vitest run --project unit` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/`
- **After every plan wave:** Run `cd frontend && npx vitest run --project unit`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** ~15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 35-01-01 | 01 | 0 | APPR-05 | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/VisibilityGroup.test.tsx` | Wave 0 | pending |
| 35-01-02 | 01 | 1 | APPR-05 | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/VisibilityGroup.test.tsx` | Wave 0 | pending |
| 35-01-03 | 01 | 1 | APPR-05 | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/VisibilityGroup.test.tsx` | Wave 0 | pending |
| 35-02-01 | 02 | 0 | APPR-06 | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/SizingGroup.test.tsx` | Wave 0 | pending |
| 35-02-02 | 02 | 1 | APPR-06 | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/SizingGroup.test.tsx` | Wave 0 | pending |
| 35-02-03 | 02 | 1 | APPR-06 | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/SizingGroup.test.tsx` | Wave 0 | pending |
| 35-03-01 | 03 | 0 | APPR-07 | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/PlatformColorsGroup.test.tsx` | Wave 0 | pending |
| 35-03-02 | 03 | 1 | APPR-07 | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/PlatformColorsGroup.test.tsx` | Wave 0 | pending |
| 35-03-03 | 03 | 1 | APPR-07 | unit | `cd frontend && npx vitest run --project unit src/components/appearance/__tests__/PlatformColorsGroup.test.tsx` | Wave 0 | pending |

*Status: pending · green · red · flaky*

---

## Wave 0 Requirements

- [ ] `frontend/src/components/appearance/__tests__/VisibilityGroup.test.tsx` — stubs for APPR-05
- [ ] `frontend/src/components/appearance/__tests__/SizingGroup.test.tsx` — stubs for APPR-06
- [ ] `frontend/src/components/appearance/__tests__/PlatformColorsGroup.test.tsx` — stubs for APPR-07

*Framework install: none needed — vitest + @testing-library/react already in place.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live preview updates when toggle/slider/color changes | APPR-05/06/07 | Requires running iframe with overlay CSS | Toggle a control in overlay editor, verify iframe CSS updates in real time |
| CollapsibleSection open/closed state persists on reload | APPR-05/06/07 | Requires browser localStorage | Open/close a group, reload page, verify state is preserved |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
