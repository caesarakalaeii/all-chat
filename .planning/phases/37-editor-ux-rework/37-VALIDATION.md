---
phase: 37
slug: editor-ux-rework
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-19
---

# Phase 37 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Vitest (unit project) |
| **Config file** | `frontend/vitest.config.ts` |
| **Quick run command** | `cd /home/moersener/Hobby/worktree/all-chat/frontend && npx vitest run --project unit` |
| **Full suite command** | `cd /home/moersener/Hobby/worktree/all-chat/frontend && npx tsc --noEmit && npx vitest run --project unit` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd /home/moersener/Hobby/worktree/all-chat/frontend && npx vitest run --project unit`
- **After every plan wave:** Run `cd /home/moersener/Hobby/worktree/all-chat/frontend && npx tsc --noEmit && npx vitest run --project unit`
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** ~15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 37-01-01 | 01 | 1 | EDUX-02 | unit | `npx vitest run --project unit src/components/appearance/__tests__/CollapsibleSection.test.tsx` | ✅ (needs update) | ⬜ pending |
| 37-01-02 | 01 | 1 | EDUX-01 | unit | `npx vitest run --project unit src/components/theme-marketplace/__tests__/ThemeContent.test.tsx` | ❌ Wave 0 | ⬜ pending |
| 37-02-01 | 02 | 2 | EDUX-01, EDUX-02, EDUX-03 | unit | `npx vitest run --project unit` | ❌ Wave 0 | ⬜ pending |
| 37-02-02 | 02 | 2 | EDUX-04 | unit | `npx vitest run --project unit src/components/appearance/__tests__/` | ✅ (existing) | ⬜ pending |
| 37-03-01 | 03 | 3 | EDUX-01–04 | manual | Open overlay editor, verify section order, open/close, save/reload | N/A | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `src/components/appearance/__tests__/CollapsibleSection.test.tsx` — add assertion: `storageKey` prop causes writes to `editor-panel-sections-v1` (covers EDUX-02)
- [ ] `src/components/theme-marketplace/__tests__/ThemeContent.test.tsx` — new file: theme cards render inline, no modal overlay element present (covers EDUX-01)

*Existing AppearancePanel sub-group tests cover EDUX-04 with no changes needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Section open/close smooth animation | EDUX-02 | CSS transition not testable in Vitest/jsdom | Open overlay editor, expand/collapse each section, verify chevron rotation and height animation |
| Theme section starts expanded on first visit | EDUX-01 | Requires browser with clean localStorage | Clear localStorage, open overlay editor, verify Theme and Sources sections are expanded |
| Expert section collapsed by default | EDUX-03 | Requires browser interaction | Open overlay editor, verify CSS editor is not visible without expanding Expert section |
| Save/reload round-trip | EDUX-01–04 | Requires live API + browser | Configure settings in all sections, save, reload page, verify state restored |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
