---
phase: 03-discord-support-bot-persistent-memory-storage-for-learning-and-improvement-over-time
plan: 02
subsystem: support-bot
tags: [typescript, discord, postgresql, pg, memory, claude]

requires:
  - phase: 03-01
    provides: MemoryRepository with retrieveMemories/storeMemory/updateMemory, StoredMemory/ParsedMemoryMarker/ParsedUpdateMemoryMarker types

provides:
  - queryCodebase with memories?: StoredMemory[] parameter and memory injection into prompt
  - STORE_MEMORY and UPDATE_MEMORY marker parsing and stripping from Claude responses
  - QueryResult extended with memoryMarker and updateMemoryMarker fields
  - handleQuestion wired to retrieve memories before Claude call and store/update after
  - DATABASE_URL required env var with fail-fast validation
  - pg.Pool initialized at startup with connection ping and graceful shutdown drain

affects:
  - 03-03 (validation strategy phase — full memory loop is now wired end-to-end)

tech-stack:
  added: []
  patterns:
    - TDD (red-green) for memory injection and marker parsing in agent.ts
    - Memory injection block positioned after system prompt, before conversation history
    - Marker parsing order: INFRA_VERDICT -> PROPOSE_ISSUE -> STORE_MEMORY -> UPDATE_MEMORY
    - Memory errors silently swallowed in MemoryRepository — never block Discord answer

key-files:
  created: []
  modified:
    - services/support-bot/src/claude/agent.ts
    - services/support-bot/src/bot.ts
    - services/support-bot/src/index.ts
    - services/support-bot/src/types.ts
    - services/support-bot/src/__tests__/agent.test.ts
    - services/support-bot/src/__tests__/bot.test.ts
    - services/support-bot/src/__tests__/index.test.ts

key-decisions:
  - "STORE_MEMORY and UPDATE_MEMORY parsed after INFRA_VERDICT and PROPOSE_ISSUE — tail markers strip cleanly in sequence"
  - "System prompt instruction text uses 'Relevant memories' (no ## prefix) to avoid colliding with injected block assertion in tests"
  - "memoryRepo.retrieveMemories/storeMemory/updateMemory errors are already swallowed inside MemoryRepository — no additional try/catch needed in bot.ts"
  - "bot.test.ts mocks MemoryRepository module entirely — preserves isolation without pg dependency in unit tests"

patterns-established:
  - "Memory injection: memories?.length > 0 guard ensures empty/undefined memories never inject the block"
  - "Marker parsing order: INFRA_VERDICT (strips tail) -> PROPOSE_ISSUE -> STORE_MEMORY -> UPDATE_MEMORY each strip from cleanAnswer"

requirements-completed: [MEM-01, MEM-02, MEM-03, MEM-04, MEM-05]

duration: 8min
completed: 2026-03-26
---

# Phase 03 Plan 02: Memory Integration into Support Bot Summary

**Memory loop fully wired: queryCodebase injects retrieved memories into Claude prompt, parses STORE_MEMORY/UPDATE_MEMORY markers from responses, and bot.ts orchestrates retrieval + storage with pg.Pool lifecycle managed in index.ts**

## Performance

- **Duration:** 8 min
- **Started:** 2026-03-26T16:39:54Z
- **Completed:** 2026-03-26T16:47:30Z
- **Tasks:** 2
- **Files modified:** 7

## Accomplishments
- queryCodebase now accepts memories parameter and injects `## Relevant memories:` block between system prompt and conversation history when non-empty
- STORE_MEMORY:type|||tags|||content and UPDATE_MEMORY:id|||content markers parsed from Claude responses and stripped from answer text
- QueryResult extended with memoryMarker and updateMemoryMarker fields
- bot.ts handleQuestion calls extractTagsFromQuestion + retrieveMemories before Claude, then storeMemory/updateMemory after
- DATABASE_URL added to required env vars with process.exit(1) on missing
- pg.Pool initialized at startup with SELECT 1 ping for fail-fast behavior, drained on SIGINT/SIGTERM
- 92 tests pass (12 new agent tests, 2 new index tests, updated bot tests for new signatures)

## Task Commits

1. **Task 1: Extend agent.ts with memory injection and STORE_MEMORY/UPDATE_MEMORY parsing** - `a810d99` (feat, TDD)
2. **Task 2: Wire MemoryRepository into bot.ts and index.ts** - `7d41b36` (feat)

## Files Created/Modified
- `services/support-bot/src/claude/agent.ts` - Added memories param, prompt injection, STORE_MEMORY/UPDATE_MEMORY parsing
- `services/support-bot/src/types.ts` - Extended QueryResult with memoryMarker/updateMemoryMarker, added databaseUrl to BotConfig
- `services/support-bot/src/bot.ts` - Wired MemoryRepository into handleQuestion and startBot signature
- `services/support-bot/src/index.ts` - DATABASE_URL validation, pg.Pool init/ping/shutdown drain
- `services/support-bot/src/__tests__/agent.test.ts` - 12 new tests for memory injection and marker parsing
- `services/support-bot/src/__tests__/bot.test.ts` - Updated for new startBot/queryCodebase signatures, mocked MemoryRepository
- `services/support-bot/src/__tests__/index.test.ts` - Added DATABASE_URL and pool.end() shutdown tests

## Decisions Made
- System prompt instruction text says "Relevant memories" (no ## prefix) to prevent the literal injection anchor string from appearing in the system prompt and breaking the "no memories injected" assertions
- MemoryRepository module fully mocked in bot.test.ts — isolates unit tests from pg without adding real DB calls
- Marker parsing order is deterministic: INFRA_VERDICT strips from tail of raw result, then PROPOSE_ISSUE/STORE_MEMORY/UPDATE_MEMORY each strip from cleanAnswer left-to-right

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] System prompt instruction text adjusted to avoid test collision**
- **Found during:** Task 1 (TDD GREEN phase)
- **Issue:** System prompt contained literal `## Relevant memories:` in instruction text, causing the "no memories injected" tests to fail since the string appeared in the prompt even without memories
- **Fix:** Changed instruction text from `"## Relevant memories:"` to `"Relevant memories"` — Claude still understands the context section header
- **Files modified:** services/support-bot/src/claude/agent.ts
- **Verification:** All 90 agent tests pass including both empty/undefined memories assertions
- **Committed in:** a810d99 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - Bug)
**Impact on plan:** Minor text change in instruction wording, no functional change to memory injection behavior.

## Issues Encountered
None beyond the system prompt text collision fixed as deviation above.

## Next Phase Readiness
- Full memory flow is complete: question -> tags -> retrieve -> inject into prompt -> Claude -> parse markers -> store/update
- DATABASE_URL and pg.Pool lifecycle are production-ready
- Phase 03-03 validation strategy can proceed

---
*Phase: 03-discord-support-bot-persistent-memory-storage-for-learning-and-improvement-over-time*
*Completed: 2026-03-26*
