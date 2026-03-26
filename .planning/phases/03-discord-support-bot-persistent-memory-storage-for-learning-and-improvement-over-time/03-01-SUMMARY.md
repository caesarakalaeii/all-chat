---
phase: 03-discord-support-bot-persistent-memory-storage-for-learning-and-improvement-over-time
plan: 01
subsystem: database
tags: [typescript, pg, postgresql, vitest, memory, repository-pattern]

# Dependency graph
requires:
  - phase: 02-support-bot-operational-awareness
    provides: support-bot service with types.ts and test infrastructure
provides:
  - MemoryType, StoredMemory, ParsedMemoryMarker, ParsedUpdateMemoryMarker TypeScript types
  - MemoryRepository class with retrieve/store/update/prune methods
  - normalizeTags and extractTagsFromQuestion helper functions
  - migrations/042_support_bot_memories.sql PostgreSQL schema
affects:
  - 03-02 (agent wiring: memory retrieval and storage hooks)
  - 03-03 (bot integration: memory-aware responses)

# Tech tracking
tech-stack:
  added: [pg@^8.20.0, @types/pg@^8.20.0]
  patterns: [repository-pattern, tdd-red-green, pg-pool-injection, try-catch-swallow]

key-files:
  created:
    - services/support-bot/src/memory/repository.ts
    - migrations/042_support_bot_memories.sql
    - services/support-bot/src/__tests__/memory.test.ts
  modified:
    - services/support-bot/src/types.ts
    - services/support-bot/package.json

key-decisions:
  - "pg installed as runtime dep (not devDep) — used at runtime for DB connection"
  - "statement_timeout removed from per-query config — not a valid QueryConfig field in @types/pg; pg pool-level config handles timeouts"
  - "pruneIfNeeded uses two queries (COUNT then conditional DELETE) — cleaner than subquery approach for testability with mocked pool"
  - "MemoryRepository constructor takes pg.Pool for dependency injection — enables vi.mock('pg') pattern in tests"

patterns-established:
  - "pg Pool injected via constructor for testability"
  - "All repository methods wrapped in try/catch, log warning, never throw — callers never crash on DB errors"
  - "normalizeTags applied on store — tags normalized at write time"
  - "Content truncated to 500 chars at write time in both store and update"

requirements-completed: [MEM-06, MEM-07, MEM-08]

# Metrics
duration: 5min
completed: 2026-03-26
---

# Phase 03 Plan 01: Memory Storage Foundation Summary

**PostgreSQL memory layer for support bot: MemoryRepository with tag-overlap retrieval, upsert-by-overlap store, staleness-ranked pruning, and full pg.Pool-mocked unit tests**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-03-26T15:32:31Z
- **Completed:** 2026-03-26T15:37:13Z
- **Tasks:** 1 (TDD: RED + GREEN)
- **Files modified:** 5

## Accomplishments
- Memory types (MemoryType, StoredMemory, ParsedMemoryMarker, ParsedUpdateMemoryMarker) added to types.ts
- MemoryRepository with retrieve (tag overlap + staleness ranking), store (insert or overlap-upsert), update, and prune (soft-cap 500)
- normalizeTags and extractTagsFromQuestion exported helpers for tag normalization and question-to-tag extraction
- PostgreSQL migration with bot_memories table, memory_type ENUM, GIN index on tags array, updated_at DESC index
- pg and @types/pg installed; 76 total tests pass (11 new memory tests)

## Task Commits

Each task was committed atomically:

1. **Task 1 RED: Failing tests for MemoryRepository** - `fb21930` (test)
2. **Task 1 GREEN: MemoryRepository implementation** - `e61e6f5` (feat)

**Plan metadata:** (this commit)

_Note: TDD tasks have two commits (test → feat)_

## Files Created/Modified
- `services/support-bot/src/types.ts` - Added MemoryType, StoredMemory, ParsedMemoryMarker, ParsedUpdateMemoryMarker
- `services/support-bot/src/memory/repository.ts` - MemoryRepository class + normalizeTags + extractTagsFromQuestion
- `services/support-bot/src/__tests__/memory.test.ts` - 11 unit tests with mocked pg.Pool
- `migrations/042_support_bot_memories.sql` - bot_memories table, memory_type ENUM, GIN + updated_at indexes
- `services/support-bot/package.json` - pg runtime dep, @types/pg dev dep added

## Decisions Made
- `statement_timeout` per-query config removed: `@types/pg` QueryConfig interface doesn't include it as a field. Pool-level timeout config handles this concern.
- `pruneIfNeeded` uses two separate queries (COUNT then conditional DELETE) rather than a single correlated subquery — makes mock assertions simpler and query logic clearer.
- MemoryRepository takes `pg.Pool` in constructor — enables clean `vi.mock('pg', ...)` pattern matching the existing agent.test.ts approach.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed test assertion for access_count bump query params**
- **Found during:** Task 1 GREEN (test run)
- **Issue:** Test checked `secondCall[1].toContain(42)` but `[ids]` is passed as `[[42]]` — the params array wraps the ids array
- **Fix:** Updated test to destructure the first element: `(secondCall[1] as number[][])[0]`
- **Files modified:** services/support-bot/src/__tests__/memory.test.ts
- **Verification:** All 76 tests pass
- **Committed in:** e61e6f5 (Task 1 GREEN commit)

**2. [Rule 1 - Bug] Removed invalid statement_timeout from pool.query call**
- **Found during:** Task 1 GREEN (TypeScript typecheck)
- **Issue:** `pool.query(sql, params, { statement_timeout: '2000' })` doesn't match any pg Pool.query overload
- **Fix:** Removed third argument; query issued with only text + values params
- **Files modified:** services/support-bot/src/memory/repository.ts
- **Verification:** `tsc --noEmit` shows no errors in repository.ts; tests pass
- **Committed in:** e61e6f5 (Task 1 GREEN commit)

---

**Total deviations:** 2 auto-fixed (2 bugs)
**Impact on plan:** Both auto-fixes necessary for correctness. No scope creep.

## Issues Encountered
None beyond the two auto-fixed TypeScript/assertion issues noted above.

## User Setup Required
Migration `042_support_bot_memories.sql` must be applied to the PostgreSQL database before the bot can store memories:
```bash
kubectl -n allchat exec -it <cnpg-pod> -- psql -U allchat allchat < migrations/042_support_bot_memories.sql
```

## Next Phase Readiness
- Memory foundation complete: types, repository, migration, and tests all green
- MemoryRepository can be instantiated with any `pg.Pool` instance
- Ready for Plan 02: wiring MemoryRepository into the agent query flow

---
*Phase: 03-discord-support-bot-persistent-memory-storage-for-learning-and-improvement-over-time*
*Completed: 2026-03-26*
