---
phase: 23
slug: design-token-system-foundation
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-10
---

# Phase 23 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | vitest (unit) + playwright (visual regression) |
| **Config file** | `frontend/vitest.config.ts` / `frontend/playwright.config.ts` |
| **Quick run command** | `cd frontend && npm run test` |
| **Full suite command** | `cd frontend && npm run test && npm run build` |
| **Estimated runtime** | ~30 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd frontend && npm run test`
- **After every plan wave:** Run `cd frontend && npm run test && npm run build`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 23-01-01 | 01 | 1 | FOUND-01/02 | build | `cd frontend && npm run build` | ✅ | ⬜ pending |
| 23-01-02 | 01 | 1 | FOUND-06 | build | `cd frontend && npm run build` | ✅ | ⬜ pending |
| 23-01-03 | 01 | 1 | FOUND-01/02 | build | `cd frontend && npm run build` | ✅ | ⬜ pending |
| 23-02-01 | 02 | 1 | FOUND-03 | unit | `cd frontend && npm run test -- --grep platform` | ❌ W0 | ⬜ pending |
| 23-02-02 | 02 | 1 | FOUND-05 | build | `cd frontend && npm run build` | ✅ | ⬜ pending |
| 23-03-01 | 03 | 2 | FOUND-04 | manual | File existence check | ✅ | ⬜ pending |
| 23-03-02 | 03 | 2 | FOUND-04 | manual | Visual review of events.css | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `frontend/src/lib/__tests__/platform-colors.test.ts` — unit tests for static PLATFORM_COLORS map (FOUND-03)

*Existing infrastructure covers all other phase requirements (build verification + manual review).*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Gradient visuals unchanged after `bg-linear-to-*` migration | FOUND-05 | Visual regression requires human judgement for subtle rendering differences | Open each migrated page in browser, compare gradient renders before/after |
| events.css `!important` removed with no visual regression in overlay preview | FOUND-04 | OBS overlay preview requires actual browser/OBS rendering | Load overlay preview page, confirm theme classes apply correctly without !important |
| oklch token values render correct colors in dark mode | FOUND-01 | oklch → sRGB conversion varies by browser/profile | Open app in Chrome, verify background (#07070a), surface (#0d0d12), platform colors match specs |
| Nav frosted glass renders correctly | FOUND-01 | backdrop-filter visual requires browser render | Verify blur(20px) + saturate(1.5) on nav element in Chrome DevTools |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
