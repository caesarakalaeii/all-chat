# Phase 3: Discord Support Bot Persistent Memory - Context

**Gathered:** 2026-03-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Add persistent memory storage to the existing support-bot service (`services/support-bot`) so it learns from past interactions and improves over time. The bot stores three types of knowledge — common error patterns, user corrections, and codebase insights — in PostgreSQL, retrieves relevant memories via keyword/tag matching, and injects them into the Claude prompt. Memory creation is auto-detected by Claude using a `STORE_MEMORY:` marker protocol. No new services are created; this extends the existing support-bot.

</domain>

<decisions>
## Implementation Decisions

### What to remember
- Three memory types: **common error patterns** (recurring infra issues), **user corrections** (when someone corrects the bot), **codebase insights** (non-obvious discoveries about the code)
- Resolved Q&A pairs are NOT stored — keeps the memory pool lean and focused on knowledge, not conversations
- Memory creation is **auto-detected** by Claude — it recognizes memory-worthy moments (corrections, recurring errors, code discoveries) and emits a `STORE_MEMORY:` marker, similar to the existing `PROPOSE_ISSUE:` protocol
- Memory creation is **silent** — no notification to the user when something is memorized
- Memories are **global** (not per-guild) — the bot serves one project (All-Chat), so all memories are universally relevant
- **Any user** can trigger a correction memory — not restricted to lead dev
- Codebase insights come from **both** Claude's own code discoveries and user-provided context (e.g., "FYI auth-service was rewritten last week")
- Memories are **replaced** when newer information contradicts them — no history preservation, just the latest accurate version

### Memory retrieval
- **Keyword/tag matching** — each memory is tagged with service names, error types, and concepts; match by overlapping tags with the incoming question
- **Up to 10** memories injected per question
- Memories placed **after the system prompt**, before conversation history: `system prompt → "## Relevant memories:" → conversation history → question`
- Claude is **instructed to reference memories** when relevant — system prompt tells Claude it has access to past observations and should weave them in naturally

### Storage backend
- **PostgreSQL** — use the existing CNPG cluster, same `allchat` database with a dedicated schema or prefixed tables
- **SQL migration files + init container** — plain SQL files in the service repo, run at startup; consistent with project's plain-SQL approach; no ORM
- Support-bot gets a new database connection (pgx pattern exists in Go services; Node.js uses `pg` or `postgres` library)

### Memory lifecycle
- **Time-based decay** — memories get a staleness score that increases over time; older memories rank lower in retrieval
- **Soft limit of ~500 memories** — when full, the stalest (oldest + least accessed) memories are archived/deleted
- **No management UI** in v1 — manage memories directly in PostgreSQL if needed
- Claude generates its own tags via the `STORE_MEMORY:type|||tags|||content` marker protocol — tags are service names, error types, concepts

### Claude marker protocol
- `STORE_MEMORY:type|||tags|||content` — type is `error_pattern`, `correction`, or `codebase_insight`; tags are comma-separated keywords; content is the memory text
- Parsed and stripped from the response before sending to Discord (same pattern as INFRA_VERDICT and PROPOSE_ISSUE)
- When updating an existing memory, Claude can emit `UPDATE_MEMORY:id|||content` or the bot can match by similar tags and replace

### Claude's Discretion
- Exact PostgreSQL schema design (columns, indexes)
- Tag extraction and matching algorithm details
- Staleness scoring formula (age weight, access weight)
- How to detect contradicting/superseding memories
- Node.js PostgreSQL client library choice (pg vs postgres)
- Memory deduplication strategy
- Exact system prompt phrasing for memory instructions

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Support bot service (existing code to extend)
- `services/support-bot/src/claude/agent.ts` — Claude subprocess wrapper; must be extended with memory retrieval (before call) and memory storage (after parsing response)
- `services/support-bot/src/bot.ts` — Discord event handlers; handleQuestion orchestrator needs memory retrieval injected
- `services/support-bot/src/types.ts` — TypeScript interfaces; new memory types needed
- `services/support-bot/src/index.ts` — Entry point; new DATABASE_URL env var validation

### Existing marker protocol patterns
- `services/support-bot/src/claude/agent.ts:105-141` — INFRA_VERDICT and PROPOSE_ISSUE marker parsing; STORE_MEMORY follows same pattern

### Kubernetes deployment
- `../caesar-deployment/apps/workloads/all-chat/support-bot-deployment.yaml` — Add DATABASE_URL env var
- `../caesar-deployment/apps/workloads/all-chat/support-bot-rbac.yaml` — May need read access to database secrets

### Database patterns
- `services/overlay-manager/repository/` — Go PostgreSQL repository pattern (reference for schema design approach)
- `migrations/` — Existing migration directory structure in the project

### Phase 1 & 2 context
- `.planning/phases/01-discord-support-bot-that-answers-user-questions-with-codebase-awareness-proposes-code-changes-without-making-them-and-uses-claude-code-login-to-avoid-additional-charges/01-CONTEXT.md` — Bot architecture decisions
- `.planning/phases/02-support-bot-operational-awareness-grafana-logs-and-k8s-cluster-state-access-with-leak-prevention-infrastructure-error-detection-and-lead-developer-pinging/02-CONTEXT.md` — Operational awareness decisions

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `agent.ts` marker parsing pattern (lines 105-141): INFRA_VERDICT and PROPOSE_ISSUE parsing with index-based extraction and stripping — STORE_MEMORY follows identical pattern
- `bot.ts` handleQuestion orchestrator: central place to inject memory retrieval before queryCodebase and memory storage after
- `types.ts` interface pattern: small focused interfaces for each marker type (IssueProposal, InfraVerdict) — add MemoryEntry

### Established Patterns
- All config from `process.env`, validated at startup with `process.exit(1)` on missing required vars
- `execa` subprocess for Claude with `--allowedTools` restriction
- Marker protocol: Claude emits `MARKER:field1|||field2|||field3`, bot parses and strips before sending response
- Per-channel queuing in bot.ts ensures sequential processing — memory storage won't race

### Integration Points
- `agent.ts` `queryCodebase()`: inject retrieved memories into `fullPrompt` between system prompt and conversation history
- `agent.ts` response parsing: add STORE_MEMORY marker parsing after INFRA_VERDICT and PROPOSE_ISSUE
- `index.ts`: add DATABASE_URL to required env vars, initialize pg client, pass to bot
- Kubernetes deployment: new DATABASE_URL env var referencing existing CNPG allchat cluster secret

</code_context>

<specifics>
## Specific Ideas

- STORE_MEMORY marker follows the exact same `MARKER:field|||field|||field` protocol as PROPOSE_ISSUE and INFRA_VERDICT — consistent and predictable
- Time-based decay means memories from 6 months ago rank lower than memories from last week, even if both match by tags
- Soft cap at ~500 keeps PostgreSQL queries fast and prevents the memory pool from becoming a dumping ground
- No management UI — if memories need cleanup, query PostgreSQL directly; a /memories command can be added in a future phase

</specifics>

<deferred>
## Deferred Ideas

- `/memories` slash command for in-Discord memory management — future phase
- Semantic search via embeddings — could replace keyword matching if it proves insufficient
- Memory confidence scoring (verified vs unverified corrections) — decided against for v1
- Per-guild memory scoping — not needed while bot serves only All-Chat
- Memory export/backup tooling — manage via PostgreSQL directly for now
- User-triggered explicit "remember this" / "forget that" commands — auto-detect only for v1

</deferred>

---

*Phase: 03-discord-support-bot-persistent-memory-storage-for-learning-and-improvement-over-time*
*Context gathered: 2026-03-26*
