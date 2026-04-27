---
phase: 03-discord-support-bot-persistent-memory-storage-for-learning-and-improvement-over-time
verified: 2026-03-26T16:07:11Z
status: passed
score: 16/16 must-haves verified
re_verification: false
---

# Phase 3: Support Bot Persistent Memory Verification Report

**Phase Goal:** Add persistent memory storage to the support bot so it learns from past interactions — stores common error patterns, user corrections, and codebase insights in PostgreSQL, retrieves relevant memories via tag matching, and injects them into the Claude prompt. Memory creation is auto-detected via STORE_MEMORY marker protocol.
**Verified:** 2026-03-26T16:07:11Z
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|---------|
| 1 | MemoryRepository can store a memory with type, tags, and content | VERIFIED | `storeMemory` in `repository.ts` lines 117-148; INSERT/upsert with type, tags, content params |
| 2 | MemoryRepository can retrieve memories by tag overlap, ranked by staleness | VERIFIED | `retrieveMemories` in `repository.ts` lines 80-115; uses `tags && $1` overlap, `STALENESS_FORMULA` ORDER BY |
| 3 | MemoryRepository handles database errors gracefully without throwing | VERIFIED | try/catch wraps all methods; `console.warn` on error, never throws; 3 tests confirm this |
| 4 | Tag normalisation trims whitespace and lowercases | VERIFIED | `normalizeTags` in `repository.ts` line 38-40; test at `memory.test.ts` line 21-28 |
| 5 | Soft cap at 500 memories prunes stalest entry on overflow | VERIFIED | `pruneIfNeeded` in `repository.ts` lines 162-178; COUNT query then conditional DELETE; test confirms trigger at 501 |
| 6 | STORE_MEMORY marker is parsed from Claude response and stripped from answer | VERIFIED | `agent.ts` lines 156-172; test at `agent.test.ts` lines 369-395 |
| 7 | UPDATE_MEMORY marker is parsed from Claude response and stripped from answer | VERIFIED | `agent.ts` lines 174-189; test at `agent.test.ts` lines 397-421 |
| 8 | Retrieved memories are injected between system prompt and conversation history | VERIFIED | `agent.ts` lines 52-63; `## Relevant memories:` block inserted; test confirms position relative to conversation history |
| 9 | Empty memories block is not injected when no memories retrieved | VERIFIED | `agent.ts` line 53: `if (memories && memories.length > 0)` guard; two tests confirm empty array and undefined both skip injection |
| 10 | DATABASE_URL missing causes process.exit(1) | VERIFIED | `index.ts` lines 23, 26-30; `DATABASE_URL` in required map; test at `index.test.ts` line 165 confirms exit(1) |
| 11 | pg Pool is initialized at startup and passed to MemoryRepository | VERIFIED | `index.ts` lines 63-72: `new pg.Pool`, `await pool.query('SELECT 1')` ping, `new MemoryRepository(pool)` |
| 12 | pg Pool is drained on graceful shutdown | VERIFIED | `index.ts` lines 47-52: `shutdown` calls `await pool?.end()`; test at `index.test.ts` line 231 confirms |
| 13 | Memory storage errors do not block the Discord answer | VERIFIED | `bot.ts` lines 39-46: `storeMemory`/`updateMemory` called after `result.answer` is already captured; errors swallowed inside MemoryRepository |
| 14 | Support bot pod has DATABASE_URL env var pointing to CNPG cluster | VERIFIED | `support-bot-deployment.yaml` lines 133-147: DATABASE_URL constructed via K8s variable substitution |
| 15 | Migration SQL runs before main container starts | VERIFIED | `support-bot-deployment.yaml` lines 25-49: `run-migrations` is the FIRST init container; uses postgres:16-alpine to execute idempotent DDL |
| 16 | Bot can connect to PostgreSQL and store/retrieve memories in production | VERIFIED (human gate passed) | Plan 03-03 Task 2 was a `checkpoint:human-verify` gate that was approved by user on 2026-03-26 |

**Score:** 16/16 truths verified

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `services/support-bot/src/types.ts` | MemoryType, StoredMemory, ParsedMemoryMarker, ParsedUpdateMemoryMarker interfaces | VERIFIED | All 4 types exported; `QueryResult` extended with `memoryMarker` and `updateMemoryMarker`; `BotConfig` has `databaseUrl` |
| `services/support-bot/src/memory/repository.ts` | MemoryRepository class with retrieve, store, update, prune methods | VERIFIED | 180 lines; exports `MemoryRepository`, `normalizeTags`, `extractTagsFromQuestion`; all methods substantive |
| `services/support-bot/src/__tests__/memory.test.ts` | Unit tests for MemoryRepository with mocked pg.Pool | VERIFIED | 11 tests across 4 describe blocks; `vi.mock('pg', ...)` at top; `describe('MemoryRepository'` present |
| `migrations/042_support_bot_memories.sql` | PostgreSQL schema for bot_memories table | VERIFIED | Contains `CREATE TYPE memory_type AS ENUM`, `CREATE TABLE bot_memories`, GIN index, updated_at index |
| `services/support-bot/src/claude/agent.ts` | Memory injection into prompt, STORE_MEMORY and UPDATE_MEMORY parsing | VERIFIED | `memories?: StoredMemory[]` param; `## Relevant memories:` injection; both markers parsed and stripped |
| `services/support-bot/src/bot.ts` | MemoryRepository wiring into handleQuestion | VERIFIED | `handleQuestion` accepts `memoryRepo`; calls `extractTagsFromQuestion`, `retrieveMemories`, `storeMemory`, `updateMemory` |
| `services/support-bot/src/index.ts` | DATABASE_URL validation, pg.Pool init, pool.end() on shutdown | VERIFIED | All three present; connection ping with SELECT 1; SIGINT/SIGTERM handlers drain pool |
| `../caesar-deployment/apps/workloads/all-chat/support-bot-deployment.yaml` | DATABASE_URL env var, migration init container | VERIFIED | `run-migrations` first init container; DATABASE_URL via K8s variable substitution; allchat-secrets/database-password referenced |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `memory/repository.ts` | `types.ts` | `import MemoryType, StoredMemory` | WIRED | Line 2: `import type { StoredMemory, ParsedMemoryMarker, MemoryType } from '../types.js'` |
| `bot.ts` | `claude/agent.ts` | `queryCodebase` call with memories parameter | WIRED | Line 35: `queryCodebase(question, repoPaths, history, memories)` — fourth param is the memories array |
| `bot.ts` | `memory/repository.ts` | MemoryRepository.retrieveMemories and storeMemory calls | WIRED | Lines 32-45: `extractTagsFromQuestion`, `retrieveMemories`, `storeMemory`, `updateMemory` all called |
| `index.ts` | `bot.ts` | startBot receives memoryRepo | WIRED | Line 75: `discordClient = await startBot(config, memoryRepo)` |
| `support-bot-deployment.yaml` | `index.ts` | DATABASE_URL env var consumed by pg.Pool | WIRED | Deployment line 146-147 defines DATABASE_URL; index.ts line 23 reads `process.env['DATABASE_URL']` |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|---------|
| MEM-01 | 03-02-PLAN.md | STORE_MEMORY marker parsed and stripped from answer | SATISFIED | `agent.ts` lines 156-172; `agent.test.ts` lines 369-395 (parses and strips) |
| MEM-02 | 03-02-PLAN.md | UPDATE_MEMORY marker parsed and stripped from answer | SATISFIED | `agent.ts` lines 174-189; `agent.test.ts` lines 397-421 (parses and strips) |
| MEM-03 | 03-02-PLAN.md | Retrieved memories injected between system prompt and conversation history | SATISFIED | `agent.ts` lines 52-63; `agent.test.ts` line 360-365 tests relative position |
| MEM-04 | 03-02-PLAN.md | Empty memories block not injected when no memories retrieved | SATISFIED | `agent.ts` line 53 guard; `agent.test.ts` lines 306-327 (empty array and undefined) |
| MEM-05 | 03-02-PLAN.md | DATABASE_URL missing causes process.exit(1) | SATISFIED | `index.ts` lines 23-30; `index.test.ts` line 165 test |
| MEM-06 | 03-01-PLAN.md | storeMemory handles pg error without throwing to caller | SATISFIED | `repository.ts` lines 145-147; `memory.test.ts` line 178-187 test |
| MEM-07 | 03-01-PLAN.md | retrieveMemories handles pg error and returns empty array | SATISFIED | `repository.ts` lines 111-114; `memory.test.ts` line 96-102 test |
| MEM-08 | 03-01-PLAN.md | Tag normalisation trims whitespace and lowercases | SATISFIED | `repository.ts` lines 38-40; `memory.test.ts` lines 21-28 test |
| MEM-09 | 03-03-PLAN.md | K8s deployment has DATABASE_URL and migration init container | SATISFIED | `support-bot-deployment.yaml` lines 25-49 (run-migrations), 146-147 (DATABASE_URL) |

**All 9 requirements accounted for. No orphaned requirements.**

---

### Anti-Patterns Found

No blockers found.

| File | Pattern | Severity | Notes |
|------|---------|----------|-------|
| None | — | — | No TODO/FIXME/placeholder comments, no empty implementations, no stub returns found in modified files |

---

### Human Verification Required

The following cannot be verified purely from static analysis:

**1. End-to-end memory round-trip in production**

Test: Ask the bot a question. Then correct it ("actually, kick-listener uses Pusher WebSocket, not plain WebSocket"). Ask a follow-up related to kick-listener. Verify the correction appears in the Claude prompt as a memory entry.
Expected: Claude acknowledges the past correction naturally in subsequent answers.
Why human: Requires a live Discord session, real Claude subprocess execution, and a live PostgreSQL connection in the cluster.

**2. Memory injection position with real multi-turn conversation**

Test: Have a multi-turn exchange in a bot thread with several messages. Verify memories appear after the system prompt and before the thread history, not at the end.
Expected: Claude receives `system_prompt → memories → conversation_history → new_question` in correct order.
Why human: Prompt assembly order is proven by unit tests, but the actual Claude response quality and coherence with injected memories requires a live run.

---

### Gaps Summary

No gaps. All 16 observable truths verified, all 8 artifacts pass existence, substance, and wiring checks, all 5 key links wired, all 9 requirements satisfied, all unit tests pass (92/92).

The only unverified item is the production end-to-end behaviour (whether Claude actually uses memories coherently in responses), which requires a live Discord session and is appropriately flagged for human verification. This is not a blocker for phase completion since the human gate in Plan 03-03 Task 2 was already passed by the user on 2026-03-26.

---

_Verified: 2026-03-26T16:07:11Z_
_Verifier: Claude (gsd-verifier)_
