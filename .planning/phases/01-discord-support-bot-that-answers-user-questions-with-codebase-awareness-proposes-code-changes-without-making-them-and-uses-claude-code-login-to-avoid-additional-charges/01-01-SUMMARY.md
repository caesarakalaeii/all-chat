---
phase: 01-discord-support-bot
plan: 01
subsystem: bot
tags: [discord, typescript, execa, octokit, vitest, claude-code]

# Dependency graph
requires: []
provides:
  - "services/support-bot npm project with ESM TypeScript config"
  - "src/types.ts: IssueProposal, QueryResult, BotConfig interfaces"
  - "src/claude/agent.ts: queryCodebase() spawning claude -p via execa with Read,Glob,Grep only"
  - "src/github/issues.ts: createIssue() via Octokit with bot-proposed label"
  - "src/index.ts: validateEnv() and shutdown() with SIGINT/SIGTERM handlers"
  - "src/bot.ts: placeholder startBot() for Plan 02"
affects: [02-discord-bot-integration]

# Tech tracking
tech-stack:
  added: [discord.js@14, execa@9, "@octokit/rest@22", typescript@5, tsx@4, vitest@3]
  patterns:
    - "execa subprocess for claude -p with --allowedTools restricting to read-only tools"
    - "PROPOSE_ISSUE:repo|||title|||body delimiter for issue proposals in LLM output"
    - "validateEnv() returns typed BotConfig; process.exit(1) on missing vars"
    - "TDD with vitest vi.mock for execa and Octokit"

key-files:
  created:
    - services/support-bot/package.json
    - services/support-bot/tsconfig.json
    - services/support-bot/vitest.config.ts
    - services/support-bot/src/types.ts
    - services/support-bot/src/claude/agent.ts
    - services/support-bot/src/github/issues.ts
    - services/support-bot/src/index.ts
    - services/support-bot/src/bot.ts
    - services/support-bot/src/__tests__/agent.test.ts
    - services/support-bot/src/__tests__/github.test.ts
    - services/support-bot/src/__tests__/index.test.ts
  modified: []

key-decisions:
  - "Use execa subprocess (claude -p) not @anthropic-ai/claude-agent-sdk — reuses user's Claude.ai subscription via CLAUDE_CODE_OAUTH_TOKEN"
  - "allowedTools restricted to Read,Glob,Grep only — bot can never write/edit code"
  - "PROPOSE_ISSUE:repo|||title|||body protocol for structured issue creation from LLM output"
  - "BotConfig uses claudeOAuthToken field (not anthropicApiKey) per auth decision"
  - "isMain guard in index.ts prevents auto-execution during vitest test runs"

patterns-established:
  - "queryCodebase: system prompt + optional conversation history + question as single -p string"
  - "claude subprocess output parsed as JSON: parsed.result contains the response text"
  - "vitest vi.mock at top of file, vi.mocked() for typed mock access"

requirements-completed: [BOT-03, BOT-04, BOT-05, BOT-06]

# Metrics
duration: 3min
completed: 2026-03-24
---

# Phase 01 Plan 01: Scaffold Support-Bot Core Modules Summary

**TypeScript ESM service scaffold with execa-based claude -p subprocess wrapper (read-only tools), Octokit issue creation, and env-validated entry point — 18 unit tests all passing**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-24T11:11:25Z
- **Completed:** 2026-03-24T11:15:23Z
- **Tasks:** 2
- **Files modified:** 11

## Accomplishments

- ESM TypeScript npm project scaffolded with discord.js, execa, @octokit/rest, vitest (no claude-agent-sdk)
- queryCodebase() spawns `claude -p` subprocess via execa with --allowedTools Read,Glob,Grep and parses PROPOSE_ISSUE: protocol
- createIssue() wraps Octokit to create GitHub issues with bot-proposed + needs-review labels
- validateEnv() checks 5 required env vars (including CLAUDE_CODE_OAUTH_TOKEN), exits on missing, returns typed BotConfig
- SIGINT/SIGTERM graceful shutdown handlers registered; placeholder bot.ts ready for Plan 02

## Task Commits

Each task was committed atomically:

1. **Task 1: Scaffold project and implement core modules (types, agent, issues)** - `0dac415` (feat)
2. **Task 2: Entry point with env validation and graceful shutdown** - `d1496e3` (feat)

_Note: TDD tasks confirmed RED then GREEN before each commit._

## Files Created/Modified

- `services/support-bot/package.json` - ESM project with execa, @octokit/rest, discord.js, vitest
- `services/support-bot/tsconfig.json` - ES2022/NodeNext TypeScript config
- `services/support-bot/vitest.config.ts` - Vitest with globals and node environment
- `services/support-bot/src/types.ts` - IssueProposal, QueryResult, BotConfig interfaces
- `services/support-bot/src/claude/agent.ts` - queryCodebase() execa subprocess with PROPOSE_ISSUE parsing
- `services/support-bot/src/github/issues.ts` - createOctokitClient() and createIssue() wrappers
- `services/support-bot/src/index.ts` - validateEnv(), shutdown(), SIGINT/SIGTERM handlers
- `services/support-bot/src/bot.ts` - Placeholder startBot() for Plan 02
- `services/support-bot/src/__tests__/agent.test.ts` - 7 tests for queryCodebase
- `services/support-bot/src/__tests__/github.test.ts` - 3 tests for createIssue
- `services/support-bot/src/__tests__/index.test.ts` - 8 tests for validateEnv and shutdown

## Decisions Made

- Used `execa` subprocess (not @anthropic-ai/claude-agent-sdk) per user decision to reuse Claude.ai subscription via CLAUDE_CODE_OAUTH_TOKEN — avoids API billing
- isMain guard added to index.ts to prevent auto-execution when imported during vitest runs

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

**External services require manual configuration before Plan 02 runs:**

- `CLAUDE_CODE_OAUTH_TOKEN` — run `claude setup-token` locally to generate token
- `DISCORD_BOT_TOKEN` — discord.com/developers/applications -> New Application -> Bot -> Reset Token
- `GITHUB_TOKEN` — GitHub Settings -> Developer settings -> Fine-grained tokens (Issues: Write)

## Next Phase Readiness

- All core non-Discord modules implemented and tested
- Plan 02 can focus purely on Discord integration (slash commands, @mention handling, thread context)
- bot.ts placeholder ready for Plan 02 to replace with full Discord implementation

---
*Phase: 01-discord-support-bot*
*Completed: 2026-03-24*
