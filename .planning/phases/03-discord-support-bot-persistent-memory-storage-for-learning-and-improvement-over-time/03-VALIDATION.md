---
phase: 03
slug: discord-support-bot-persistent-memory-storage-for-learning-and-improvement-over-time
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-03-26
---

# Phase 03 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Vitest 3.x |
| **Config file** | `services/support-bot/vitest.config.ts` |
| **Quick run command** | `npm test` (in `services/support-bot/`) |
| **Full suite command** | `npm test` (in `services/support-bot/`) |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `npm test` (in `services/support-bot/`)
- **After every plan wave:** Run `npm test` (in `services/support-bot/`)
- **Before `/gsd:verify-work`:** Full suite must be green
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| MEM-01 | TBD | TBD | STORE_MEMORY marker parsed and stripped | unit | `npm test` in `services/support-bot/` | ❌ W0 | ⬜ pending |
| MEM-02 | TBD | TBD | UPDATE_MEMORY marker parsed and stripped | unit | `npm test` in `services/support-bot/` | ❌ W0 | ⬜ pending |
| MEM-03 | TBD | TBD | Retrieved memories injected between system prompt and conversation | unit | `npm test` in `services/support-bot/` | ❌ W0 | ⬜ pending |
| MEM-04 | TBD | TBD | Empty memories block not injected when no matches | unit | `npm test` in `services/support-bot/` | ❌ W0 | ⬜ pending |
| MEM-05 | TBD | TBD | DATABASE_URL missing causes process.exit(1) | unit | `npm test` in `services/support-bot/` | ❌ W0 | ⬜ pending |
| MEM-06 | TBD | TBD | storeMemory handles pg error without throwing | unit | `npm test` in `services/support-bot/` | ❌ W0 | ⬜ pending |
| MEM-07 | TBD | TBD | retrieveMemories handles pg error → empty array | unit | `npm test` in `services/support-bot/` | ❌ W0 | ⬜ pending |
| MEM-08 | TBD | TBD | Tag normalisation trims whitespace and lowercases | unit | `npm test` in `services/support-bot/` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `services/support-bot/src/__tests__/memory.test.ts` — stubs for MEM-06, MEM-07, MEM-08 (new file)
- [ ] Add MEM-01 through MEM-05 test cases to existing `agent.test.ts` and `index.test.ts`

*All memory repository tests mock `pg.Pool` using `vi.mock('pg')` — same pattern as existing `vi.mock('execa')` in `agent.test.ts`.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Memory injection improves answer quality | End-to-end | Requires Claude judgment | Ask a question the bot previously answered incorrectly, verify memory context is referenced |
| STORE_MEMORY triggered appropriately by Claude | End-to-end | Claude's discretion when to memorize | Correct the bot, verify a memory is stored in PostgreSQL |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
