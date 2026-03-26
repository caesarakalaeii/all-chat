---
phase: 02-support-bot-operational-awareness
plan: 01
subsystem: support-bot
tags: [typescript, mcp, grafana, kubectl, discord-bot, claude-subprocess]

# Dependency graph
requires:
  - phase: 01-discord-support-bot
    provides: queryCodebase function, agent.ts subprocess wrapper, types.ts QueryResult/BotConfig
provides:
  - InfraVerdict interface (types.ts) for infrastructure vs code classification
  - Extended QueryResult with infraVerdict field
  - Extended BotConfig with leadDeveloperDiscordId, grafanaUrl, grafanaServiceAccountToken
  - Grafana MCP config (grafana-caesar) conditionally built from env vars
  - kubectl tool access (Bash(kubectl:*)) in all queryCodebase calls
  - INFRA_VERDICT:type|||summary parsing and marker stripping from agent output
  - Leak prevention system prompt (NEVER include raw logs, secrets, hostnames)
  - 180s subprocess timeout (increased from 120s)
affects: [02-02, 02-03, bot.ts infraVerdict handling, lead developer pinging]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Conditional MCP config: grafana-caesar server built from GRAFANA_URL + GRAFANA_SERVICE_ACCOUNT_TOKEN env vars; omitted when either absent"
    - "Structured output marker protocol: INFRA_VERDICT:type|||summary appended by Claude subprocess, parsed and stripped by agent.ts"
    - "Layered allowedTools: base (Read,Glob,Grep,Bash(kubectl:*)) + conditional Grafana MCP tools"

key-files:
  created: []
  modified:
    - services/support-bot/src/types.ts
    - services/support-bot/src/claude/agent.ts
    - services/support-bot/src/index.ts
    - services/support-bot/src/__tests__/agent.test.ts

key-decisions:
  - "Read GRAFANA_URL and GRAFANA_SERVICE_ACCOUNT_TOKEN directly from process.env in agent.ts (not passed as function params) — avoids signature change, keeps callers clean"
  - "Bash(kubectl:*) always included in allowedTools (not conditional) — infra checks are always useful even without Grafana"
  - "INFRA_VERDICT stripped from answer before PROPOSE_ISSUE is parsed — correct ordering since INFRA_VERDICT appears at end of response"
  - "New BotConfig fields (leadDeveloperDiscordId, grafanaUrl, grafanaServiceAccountToken) default to empty string in index.ts — optional at runtime, validated per-call by env var presence"

patterns-established:
  - "Conditional MCP injection: check env vars → build JSON → spread into args array"
  - "Structured marker protocol: parse before stripping; strip by slicing to markerIndex and trimming"

requirements-completed: [OPS-01, OPS-02, OPS-03, OPS-04]

# Metrics
duration: 3min
completed: 2026-03-26
---

# Phase 02 Plan 01: Grafana MCP Integration and INFRA_VERDICT Parsing Summary

**Grafana MCP config (grafana-caesar), kubectl tool access, leak-prevention system prompt, and INFRA_VERDICT structured marker parsing added to Claude agent subprocess wrapper**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-03-26T08:55:00Z
- **Completed:** 2026-03-26T08:57:59Z
- **Tasks:** 1 (TDD: RED + GREEN commits)
- **Files modified:** 4

## Accomplishments

- Added `InfraVerdict` interface and extended `QueryResult` with `infraVerdict` field; extended `BotConfig` with Grafana and lead developer fields
- Agent subprocess now conditionally includes `--mcp-config` with `grafana-caesar` MCP server when `GRAFANA_URL` + `GRAFANA_SERVICE_ACCOUNT_TOKEN` are present; omits it when either is absent
- `Bash(kubectl:*)` always included in allowedTools; Grafana MCP tools added conditionally alongside it
- Leak prevention system prompt guardrails instruct Claude to never include raw log lines, stack traces, secret values, or internal hostnames
- `INFRA_VERDICT:type|||summary` marker parsed into `infraVerdict` field and stripped from returned answer text
- Subprocess timeout increased from 120s to 180s to accommodate Grafana/kubectl queries
- 18 unit tests written (TDD) covering all new behaviors; all pass

## Task Commits

Each task was committed atomically:

1. **RED: Failing tests for new behaviors** - `1fe03bd` (test)
2. **GREEN: Implementation** - `44a2164` (feat)

## Files Created/Modified

- `services/support-bot/src/types.ts` - Added `InfraVerdict` interface, `infraVerdict: InfraVerdict | null` to `QueryResult`, `leadDeveloperDiscordId/grafanaUrl/grafanaServiceAccountToken` to `BotConfig`
- `services/support-bot/src/claude/agent.ts` - Conditional MCP config, dynamic allowedTools, leak prevention system prompt, INFRA_VERDICT parsing and stripping, 180s timeout
- `services/support-bot/src/index.ts` - Populates new BotConfig fields from env vars (optional, default empty string)
- `services/support-bot/src/__tests__/agent.test.ts` - 18 unit tests for all new behaviors (TDD)

## Decisions Made

- Read Grafana credentials from `process.env` directly in `agent.ts` rather than threading them through function parameters — keeps the `queryCodebase` signature unchanged, callers unaffected
- `Bash(kubectl:*)` always included in base tools regardless of Grafana availability — kubectl is useful even without Grafana for pod status checks
- INFRA_VERDICT parsing and stripping happens before PROPOSE_ISSUE parsing since INFRA_VERDICT appears at the end of the response; both markers are then stripped sequentially

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

Pre-existing bot.test.ts failures (13 tests): `thread.send is not a function` in Discord.js mock setup. These failures existed before this plan was executed (confirmed via git stash verification). Documented in `deferred-items.md`. Not caused by this plan's changes.

## User Setup Required

None - no external service configuration required. Grafana integration is opt-in via environment variables.

## Next Phase Readiness

- `infraVerdict` field is now populated on every `QueryResult` — plan 02-02 can read it to decide whether to ping the lead developer
- `leadDeveloperDiscordId` is available in `BotConfig` for use in bot.ts lead pinging logic
- All acceptance criteria met; ready for plan 02-02 (lead developer pinging based on infraVerdict)

---
*Phase: 02-support-bot-operational-awareness*
*Completed: 2026-03-26*
