# Phase 03: Discord Support Bot Persistent Memory — Research

**Researched:** 2026-03-26
**Domain:** Node.js/TypeScript, PostgreSQL (pg library), marker-protocol extension, Kubernetes init containers
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **Service boundary:** Extend existing `services/support-bot` — no new service created
- **Memory types:** three types only — `error_pattern`, `correction`, `codebase_insight`
- **Resolved Q&A pairs NOT stored** — only structured knowledge is stored
- **Memory creation:** auto-detected by Claude via `STORE_MEMORY:type|||tags|||content` marker — silent, no user notification
- **Memory scope:** global (not per-guild)
- **Any user can trigger** a correction memory
- **Codebase insights:** from both Claude's code discoveries and user-provided FYI statements
- **Replacement not accumulation:** newer contradicting memory replaces older — no history preservation
- **Retrieval:** keyword/tag matching, up to 10 memories per question, placed after system prompt and before conversation history
- **System prompt:** Claude instructed to reference memories naturally; placement order is `system prompt → "## Relevant memories:" → conversation history → question`
- **Storage:** PostgreSQL CNPG cluster, existing `allchat` database, plain SQL migration files, no ORM
- **Node.js pg client:** either `pg` or `postgres` library (Claude's discretion)
- **Decay:** time-based staleness score, older memories rank lower in retrieval
- **Soft cap:** ~500 memories; when full, stalest (oldest + least accessed) are archived/deleted
- **No management UI in v1** — direct PostgreSQL access for cleanup
- **UPDATE_MEMORY marker:** Claude can emit `UPDATE_MEMORY:id|||content` for targeted updates OR bot matches by similar tags and replaces
- **Marker format:** `STORE_MEMORY:type|||tags|||content` — same `|||` separator as PROPOSE_ISSUE and INFRA_VERDICT

### Claude's Discretion

- Exact PostgreSQL schema design (columns, indexes)
- Tag extraction and matching algorithm details
- Staleness scoring formula (age weight, access weight)
- How to detect contradicting/superseding memories
- Node.js PostgreSQL client library choice (pg vs postgres)
- Memory deduplication strategy
- Exact system prompt phrasing for memory instructions

### Deferred Ideas (OUT OF SCOPE)

- `/memories` slash command for in-Discord memory management
- Semantic search via embeddings
- Memory confidence scoring (verified vs unverified corrections)
- Per-guild memory scoping
- Memory export/backup tooling
- User-triggered explicit "remember this" / "forget that" commands
</user_constraints>

---

## Summary

Phase 3 extends the existing TypeScript support-bot service with a lightweight persistent memory layer backed by PostgreSQL. The bot already has a well-established marker protocol (INFRA_VERDICT, PROPOSE_ISSUE) that parses structured tokens from Claude's output — STORE_MEMORY follows the identical `|||`-separated pattern. The memory layer adds one new file (`src/memory/repository.ts`), a new migration file, and touches four existing files: `types.ts` (add interfaces), `agent.ts` (inject memories before call, parse STORE_MEMORY after), `bot.ts` (pass db to handleQuestion), and `index.ts` (DATABASE_URL validation + db init).

The `pg` library (`node-postgres`) is the recommended choice over the newer `postgres` library because `pg` is the battle-tested standard in the Node.js ecosystem (version 8.20.0 current), has `@types/pg` available for full TypeScript coverage, and all existing PostgreSQL patterns in this project (Go services) use connection-string-based setup that `pg`'s `Pool` mirrors directly. The `postgres` library has a different API that offers no advantage for this use case.

Staleness scoring can be computed entirely in SQL at query time using a simple formula weighting `updated_at` age and `access_count` — no background job needed. Tag matching uses a `tags TEXT[]` column with PostgreSQL array overlap operator (`&&`) to retrieve candidates, then ranks by staleness score. This stays fast well under the 500-memory cap.

**Primary recommendation:** Use `pg` with a `Pool`, a single `bot_memories` table with a `GIN` index on the `tags` array, and a computed staleness score (`EXTRACT(epoch FROM NOW() - updated_at) / 86400.0 - (access_count * 2.0)`) for ranking. Implement STORE_MEMORY parsing identically to INFRA_VERDICT parsing in agent.ts.

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `pg` | 8.20.0 | PostgreSQL client with connection pooling | Battle-tested, `@types/pg` support, Pool API matches project patterns |
| `@types/pg` | 8.20.0 | TypeScript types for `pg` | Required since `pg` has no bundled types |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `vitest` | already present (^3.0.0) | Unit tests for memory repository and marker parsing | All new logic must have unit tests matching existing agent.test.ts pattern |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `pg` + `@types/pg` | `postgres` (v3.4.8) | `postgres` has tagged template API, no `@types/` needed, but is a larger API surface change for a small addition; `pg` is more consistent with Go services' pgx connection-string approach |
| Plain SQL migration file | Flyway/node-pg-migrate | Over-engineering; project already uses plain numbered SQL files — consistent to add `042_support_bot_memories.sql` |
| Array overlap (`&&`) tag matching | Full-text search (`tsvector`) | FTS adds complexity; array overlap is sufficient for ≤500 memories and explicit tag lists |

**Installation:**
```bash
npm install pg
npm install --save-dev @types/pg
```

**Version verification:** Confirmed against npm registry on 2026-03-26.
- `pg`: 8.20.0 (published recently, stable)
- `@types/pg`: 8.20.0 (in sync)

---

## Architecture Patterns

### Recommended Project Structure Extension
```
services/support-bot/src/
├── memory/
│   └── repository.ts       # MemoryRepository class: retrieve, store, update, prune
├── claude/
│   └── agent.ts            # MODIFIED: inject memories before prompt, parse STORE_MEMORY after
├── bot.ts                  # MODIFIED: pass MemoryRepository to handleQuestion
├── index.ts                # MODIFIED: DATABASE_URL validation, pg Pool init, pass to startBot
└── types.ts                # MODIFIED: add MemoryEntry, StoredMemory, ParsedMemoryMarker

migrations/
└── 042_support_bot_memories.sql   # New migration file
```

### Pattern 1: pg Pool Initialization (matches project connection-string approach)
**What:** Create a `pg.Pool` from `DATABASE_URL`, ping on startup, fail-fast if unreachable.
**When to use:** At `index.ts` startup, passed into `MemoryRepository`.
**Example:**
```typescript
// Source: pg official docs + matches pgxpool.New pattern in overlay-manager/repository/overlay_repo.go
import pg from 'pg';

const pool = new pg.Pool({ connectionString: process.env['DATABASE_URL'] });
await pool.query('SELECT 1'); // ping — throw on failure
```

### Pattern 2: STORE_MEMORY Marker Parsing (verbatim from existing INFRA_VERDICT pattern)
**What:** Find marker in resultText, parse fields, strip from answer.
**When to use:** In `agent.ts` response parsing block, after INFRA_VERDICT and PROPOSE_ISSUE parsing.
**Example:**
```typescript
// Mirrors lines 105-138 of agent.ts exactly
const storeMarker = 'STORE_MEMORY:';
const storeIndex = cleanAnswer.indexOf(storeMarker);
let memoryMarker: ParsedMemoryMarker | null = null;
if (storeIndex !== -1) {
  const markerString = cleanAnswer.slice(storeIndex + storeMarker.length).split('\n')[0];
  const parts = markerString.split('|||');
  if (parts.length >= 3) {
    const type = parts[0].trim() as MemoryType;
    const tags = parts[1].trim().split(',').map(t => t.trim()).filter(Boolean);
    const content = parts.slice(2).join('|||').trim();
    memoryMarker = { type, tags, content };
  }
  cleanAnswer = cleanAnswer.slice(0, storeIndex).trimEnd();
}
```

### Pattern 3: Tag-Based Memory Retrieval with Staleness Ranking
**What:** Retrieve up to 10 memories whose tags overlap with question-derived tags, ranked by freshness + access frequency.
**When to use:** Before building `fullPrompt` in `queryCodebase()`.
**Example:**
```typescript
// Source: PostgreSQL array operators, pg library docs
const result = await pool.query<MemoryRow>(
  `SELECT id, type, tags, content, access_count, updated_at,
          EXTRACT(epoch FROM NOW() - updated_at) / 86400.0
          - (access_count * 2.0) AS staleness_score
   FROM bot_memories
   WHERE tags && $1            -- array overlap: any tag matches
   ORDER BY staleness_score ASC -- freshest / most accessed first
   LIMIT 10`,
  [tags]  // pass as string array: pg serialises to SQL array literal
);
```

### Pattern 4: Memory Upsert (replace-on-conflict by tag similarity)
**What:** When STORE_MEMORY fires, check for an existing memory of the same type with high tag overlap; replace content if found, insert if not.
**When to use:** In `MemoryRepository.storeMemory()`.
**Example:**
```typescript
// Check for existing memory with same type + at least 2 matching tags
const existing = await pool.query(
  `SELECT id FROM bot_memories
   WHERE type = $1
     AND cardinality(tags & $2) >= 2
   ORDER BY updated_at DESC LIMIT 1`,
  [type, tags]
);
if (existing.rows.length > 0) {
  await pool.query(
    `UPDATE bot_memories SET content = $1, tags = $2, updated_at = NOW()
     WHERE id = $3`,
    [content, tags, existing.rows[0].id]
  );
} else {
  await pool.query(
    `INSERT INTO bot_memories (type, tags, content)
     VALUES ($1, $2, $3)`,
    [type, tags, content]
  );
}
```

### Pattern 5: Memory Injection into Prompt
**What:** Format retrieved memories as a block and insert between system prompt and conversation history.
**When to use:** In `queryCodebase()` prompt construction.
**Example:**
```typescript
// Produces: systemPrompt + "\n\n## Relevant memories:\n...\n\n## Conversation so far:..."
let memoriesBlock = '';
if (memories.length > 0) {
  const lines = memories.map(m => `- [${m.type}] ${m.content}`).join('\n');
  memoriesBlock = `\n\n## Relevant memories:\n${lines}`;
}
if (conversationHistory && conversationHistory.length > 0) {
  fullPrompt = `${systemPrompt}${memoriesBlock}\n\n## Conversation so far:\n${conversationHistory.join('\n')}\n\n## New question:\n${question}`;
} else {
  fullPrompt = `${systemPrompt}${memoriesBlock}\n\n${question}`;
}
```

### Pattern 6: Soft-Cap Pruning
**What:** After every STORE_MEMORY, if count > 500, delete the single most stale memory.
**When to use:** Called at end of `storeMemory()`.
```sql
-- Delete one row only; avoids expensive full-table operations
DELETE FROM bot_memories
WHERE id = (
  SELECT id FROM bot_memories
  ORDER BY EXTRACT(epoch FROM NOW() - updated_at) / 86400.0 - (access_count * 2.0) DESC
  LIMIT 1
)
AND (SELECT COUNT(*) FROM bot_memories) > 500;
```

### Recommended PostgreSQL Schema
```sql
-- migrations/042_support_bot_memories.sql
CREATE TYPE memory_type AS ENUM ('error_pattern', 'correction', 'codebase_insight');

CREATE TABLE bot_memories (
  id           SERIAL PRIMARY KEY,
  type         memory_type NOT NULL,
  tags         TEXT[]      NOT NULL DEFAULT '{}',
  content      TEXT        NOT NULL,
  access_count INTEGER     NOT NULL DEFAULT 0,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bot_memories_tags ON bot_memories USING GIN (tags);
CREATE INDEX idx_bot_memories_updated_at ON bot_memories (updated_at DESC);
```

**Key design choices:**
- `SERIAL` not UUID — memories are internal, no external references need stable IDs
- `GIN` index on `tags` — required for efficient `&&` array overlap queries
- `access_count` — bumped on every retrieval (`UPDATE bot_memories SET access_count = access_count + 1 WHERE id = ANY($1)`)
- `updated_at` — bumped on content update; used for staleness ranking
- `memory_type` enum — enforces the three valid types at database level

### Anti-Patterns to Avoid
- **Returning all memories and filtering in JS:** Always use the SQL `&&` operator for tag filtering; JS-side filtering would load all 500 rows on every question.
- **Calling `pool.end()` inside MemoryRepository methods:** The Pool is long-lived; only close on graceful shutdown in `index.ts`.
- **Parsing tags with `split(',')` on raw Claude output without trimming:** Claude may emit `"kick-listener, auth-service"` with spaces — always `.map(t => t.trim()).filter(Boolean)`.
- **Using `client.query()` without releasing:** Use `pool.query()` for auto-release, or always `client.release()` in a finally block.
- **Catching memory errors and throwing:** Memory is best-effort — wrap `storeMemory()` and `retrieveMemories()` in try/catch and log errors; never let a memory failure block the answer.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| PostgreSQL connection pooling | Custom connection manager | `pg.Pool` | Handles reconnection, idle timeout, max connections automatically |
| Array overlap querying | Deserialise tags to JS, filter | PostgreSQL `&&` operator + GIN index | SQL-side filtering with index is O(log n); JS-side is O(n) |
| Tag normalisation | Complex NLP pipeline | `.toLowerCase().trim()` + Claude-generated tags | Claude already outputs clean comma-separated tags; normalise only case/whitespace |
| Staleness scoring background job | Cron job / worker | Computed column in SELECT at query time | No persistence needed; score is always current at query time |

**Key insight:** The database does the heavy lifting for tag matching and ranking. The TypeScript layer is intentionally thin — parse markers, call repository methods, inject text into prompts.

---

## Common Pitfalls

### Pitfall 1: STORE_MEMORY Marker Appearing Before INFRA_VERDICT
**What goes wrong:** If STORE_MEMORY is parsed before INFRA_VERDICT, the slice indices are wrong and INFRA_VERDICT content bleeds into the memory content.
**Why it happens:** Marker parsing order matters — each parse slices `cleanAnswer` progressively.
**How to avoid:** Parse in order: 1) INFRA_VERDICT (strips tail), 2) PROPOSE_ISSUE (strips from what remains), 3) STORE_MEMORY (strips from what remains). The system prompt must instruct Claude to emit markers in this order.
**Warning signs:** Memory content contains `INFRA_VERDICT:` text.

### Pitfall 2: Memory Retrieval Blocking the Answer
**What goes wrong:** A slow or failed database query delays the Discord response.
**Why it happens:** `retrieveMemories()` is on the critical path before the Claude subprocess call.
**How to avoid:** Set a short query timeout (e.g., 2000ms) on the memory retrieval query. Wrap in try/catch — if retrieval fails, proceed with empty memories (log warning). Never propagate the error to the user.

### Pitfall 3: pg Array Parameter Serialisation
**What goes wrong:** Passing a JS string array to a `TEXT[]` parameter fails or produces wrong SQL if not using the `pg` array format.
**Why it happens:** `pg` requires the parameter to be a JS array — it serialises to `ARRAY['tag1','tag2']`. Passing a comma-string breaks the query.
**How to avoid:** Always pass `tags` as `string[]` (JS array), never as a comma-separated string. `pg` handles the serialisation automatically.

### Pitfall 4: UPDATE_MEMORY ID Reference
**What goes wrong:** Claude emits `UPDATE_MEMORY:id|||content` but the bot doesn't know what IDs exist.
**Why it happens:** Claude can only see the memory content injected into the prompt — it doesn't see database IDs.
**How to avoid:** When injecting memories into the prompt, include the memory ID: `- [${m.type}] (id:${m.id}) ${m.content}`. This lets Claude emit a valid UPDATE_MEMORY marker. Alternatively, the bot can skip UPDATE_MEMORY and rely solely on tag-based deduplication in storeMemory().
**Recommendation:** Include IDs in the injected memory block so UPDATE_MEMORY is usable.

### Pitfall 5: Database Connection Failing Silently in Kubernetes
**What goes wrong:** The bot pod starts, DATABASE_URL points to the CNPG cluster, but the database isn't reachable yet.
**Why it happens:** Pod start race condition if init container only copies migration files but doesn't run them.
**How to avoid:** The init container runs migrations AND verifies connectivity (the migration runner exits non-zero on failure, blocking the main container). Validate `DATABASE_URL` in `validateEnv()` and test the connection with `pool.query('SELECT 1')` during startup before calling `startBot()`.

### Pitfall 6: Memory Content Size
**What goes wrong:** Claude emits very long memory content (paragraph-length) that balloons the prompt injection.
**Why it happens:** No length constraint in the STORE_MEMORY instruction.
**How to avoid:** Add to the system prompt: "Memory content must be concise — one to two sentences maximum." Truncate in the repository at 500 characters as a safety net.

---

## Code Examples

Verified patterns from official sources:

### pg Pool Setup (official pg docs)
```typescript
// Source: https://node-postgres.com/features/pooling
import pg from 'pg';

const pool = new pg.Pool({
  connectionString: process.env['DATABASE_URL'],
  max: 5,             // small pool — low concurrency for a bot
  idleTimeoutMillis: 30_000,
  connectionTimeoutMillis: 2_000,
});
```

### pg Array Parameter (official pg docs — arrays)
```typescript
// Source: https://node-postgres.com/features/queries
// pg serialises JS string[] to PostgreSQL TEXT[] automatically
const result = await pool.query(
  'SELECT * FROM bot_memories WHERE tags && $1',
  [['kick-listener', 'auth-service']]  // note: array inside array for parameterised query
);
```

### Graceful Shutdown with Pool
```typescript
// In index.ts shutdown function — mirrors existing client.destroy() pattern
export async function shutdown(client?: { destroy: () => void }, pool?: pg.Pool): Promise<void> {
  console.log('Shutting down support-bot...');
  client?.destroy();
  await pool?.end();  // drain pending queries before exit
  process.exit(0);
}
```

### Memory System Prompt Addition
```typescript
// Add to systemPrompt array in agent.ts
'',
'You have access to a memory bank of past observations about this codebase. When the section "## Relevant memories:" appears below, treat these as verified past knowledge — weave relevant memories naturally into your answer.',
'When you observe something memory-worthy (a correction from a user, a recurring error pattern, or a non-obvious codebase insight), append to your response:',
'STORE_MEMORY:type|||tag1,tag2,tag3|||one or two sentence description',
'where type is one of: error_pattern, correction, codebase_insight',
'Tags should be service names (e.g. kick-listener), error types (e.g. OOMKill), or concepts (e.g. quota).',
'Memory content must be concise — one to two sentences maximum.',
'If you need to update an existing memory you can see above, append: UPDATE_MEMORY:id|||updated content',
'Emit STORE_MEMORY or UPDATE_MEMORY at most once per response, after INFRA_VERDICT and PROPOSE_ISSUE.',
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `pg` v7 callbacks | `pg` v8 Promises / async-await | 2018 | No callback hell; direct `await pool.query()` |
| Separate `pg-pool` package | `pg.Pool` built into `pg` | v6+ | One import for pool |
| `node-postgres` npm name | `pg` npm name | always | Install as `pg`, import as `pg` |

**Deprecated/outdated:**
- `pg-native`: native C binding for pg — unnecessary complexity, not needed for this use case
- Callback-style `pool.query(sql, params, callback)` — use `await pool.query(sql, params)` instead

---

## Open Questions

1. **Migration file numbering**
   - What we know: Current highest migration is `041_visual_settings.sql`
   - What's unclear: Whether `042` is already claimed by another in-progress feature
   - Recommendation: Use `042_support_bot_memories.sql`; verify with `ls migrations/` before committing

2. **DATABASE_URL secret in Kubernetes**
   - What we know: CNPG cluster exists in `allchat` namespace; other services use `DATABASE_URL`; caesar-deployment directory currently has no YAML files (Phase 1 and 2 deployments may exist in cluster but not in repo)
   - What's unclear: Whether the support-bot deployment YAML exists in the cluster already or needs to be created; which Kubernetes secret holds the CNPG connection string
   - Recommendation: Check cluster with `kubectl -n allchat get secret` before writing deployment YAML; reuse the existing CNPG secret format

3. **Tag extraction from incoming questions**
   - What we know: We need tags to query the memory bank before calling Claude
   - What's unclear: How to extract tags from the raw user question without calling Claude first (chicken-and-egg)
   - Recommendation: Use a simple heuristic — extract known service names (`twitch-listener`, `kick-listener`, etc.) and error keywords from the question text with a static lookup list. This is fast, zero-latency, and sufficient for ≤500 memories.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Vitest 3.x |
| Config file | `services/support-bot/vitest.config.ts` |
| Quick run command | `npm test` (in `services/support-bot/`) |
| Full suite command | `npm test` (in `services/support-bot/`) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| MEM-01 | STORE_MEMORY marker parsed and stripped from answer | unit | `npm test` in `services/support-bot/` | ❌ Wave 0 — add to `agent.test.ts` |
| MEM-02 | UPDATE_MEMORY marker parsed and stripped from answer | unit | `npm test` in `services/support-bot/` | ❌ Wave 0 — add to `agent.test.ts` |
| MEM-03 | Retrieved memories injected between system prompt and conversation | unit | `npm test` in `services/support-bot/` | ❌ Wave 0 — add to `agent.test.ts` |
| MEM-04 | Empty memories block not injected when no memories retrieved | unit | `npm test` in `services/support-bot/` | ❌ Wave 0 — add to `agent.test.ts` |
| MEM-05 | DATABASE_URL missing causes process.exit(1) | unit | `npm test` in `services/support-bot/` | ❌ Wave 0 — add to `index.test.ts` |
| MEM-06 | storeMemory handles pg error without throwing to caller | unit | `npm test` in `services/support-bot/` | ❌ Wave 0 — add `memory.test.ts` |
| MEM-07 | retrieveMemories handles pg error and returns empty array | unit | `npm test` in `services/support-bot/` | ❌ Wave 0 — add `memory.test.ts` |
| MEM-08 | Tag normalisation trims whitespace and lowercases | unit | `npm test` in `services/support-bot/` | ❌ Wave 0 — add `memory.test.ts` |

**Note:** All memory repository tests should mock `pg.Pool` using `vi.mock('pg')` — same pattern as existing `vi.mock('execa')` in `agent.test.ts`. No real database connection needed for unit tests.

### Sampling Rate
- **Per task commit:** `npm test` (in `services/support-bot/`)
- **Per wave merge:** `npm test`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `services/support-bot/src/__tests__/memory.test.ts` — covers MEM-06, MEM-07, MEM-08 (new file)
- [ ] Add MEM-01 through MEM-05 test cases to existing `agent.test.ts` and `index.test.ts`

---

## Sources

### Primary (HIGH confidence)
- Direct code inspection of `services/support-bot/src/claude/agent.ts` lines 105-141 — existing marker parsing pattern
- Direct code inspection of `services/support-bot/src/types.ts` — interface conventions
- Direct code inspection of `services/support-bot/src/index.ts` — env validation pattern
- Direct code inspection of `services/support-bot/src/bot.ts` — handleQuestion orchestrator
- Direct code inspection of `services/support-bot/vitest.config.ts` and `__tests__/agent.test.ts` — test patterns
- `npm view pg version` confirmed 8.20.0 on 2026-03-26
- `npm view @types/pg version` confirmed 8.20.0 on 2026-03-26
- PostgreSQL docs: `&&` array overlap operator, GIN indexes, ENUM types

### Secondary (MEDIUM confidence)
- node-postgres official docs (https://node-postgres.com) — Pool API, array parameter handling
- PostgreSQL 16 docs — GIN index for array columns

### Tertiary (LOW confidence)
- None

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — versions verified against npm registry; `pg` is the clear choice
- Architecture: HIGH — derived directly from existing marker-parsing code in the repo; no guesswork
- Pitfalls: HIGH — derived from actual code structure and known PostgreSQL array/pool behaviour
- Schema design: HIGH — standard PostgreSQL patterns for this use case

**Research date:** 2026-03-26
**Valid until:** 2026-04-26 (stable libraries; 30-day horizon)
