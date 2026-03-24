---
phase: 01-discord-support-bot
plan: 02
subsystem: support-bot
tags: [discord, typescript, bot, slash-commands, tdd]
dependency_graph:
  requires: [01-01]
  provides: [discord-bot-layer, slash-command-registration]
  affects: [services/support-bot]
tech_stack:
  added: []
  patterns: [TDD red-green, Discord.js event handlers, EmbedBuilder response formatting]
key_files:
  created:
    - services/support-bot/src/bot.ts
    - services/support-bot/src/commands/support.ts
    - services/support-bot/src/commands/deploy.ts
    - services/support-bot/src/__tests__/bot.test.ts
  modified:
    - services/support-bot/src/index.ts
key_decisions:
  - "Mention regex uses \\d+ (numeric snowflake IDs only) — Discord IDs are always numeric, tests updated to use numeric mock IDs"
  - "sendResponse is a standalone async function taking a channel — enables clean separation from editReply (slash command) path"
  - "editReply for slash commands handles all three size cases independently — cannot reuse sendResponse since editReply has different API than channel.send"
  - "InteractionCreate handler calls fetchReply before editReply to get the message reference for thread creation"
metrics:
  duration: 248s
  completed: "2026-03-24"
  tasks_completed: 2
  files_changed: 5
---

# Phase 01 Plan 02: Discord Bot Layer Summary

**One-liner:** Discord event handlers with @mention/slash-command routing, thread conversation context, embed/chunked response formatting, and guild-or-global slash command registration.

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| RED | Failing bot tests | 6de3797 | src/__tests__/bot.test.ts |
| 1 | Discord bot event handlers and handleQuestion orchestrator | 55de027 | src/bot.ts, src/index.ts, src/commands/support.ts |
| 2 | Slash command deployment script | 3036266 | src/commands/deploy.ts |

## What Was Built

### bot.ts
- `startBot(config)` creates a Discord Client with `Guilds`, `GuildMessages`, `MessageContent` intents
- `Events.MessageCreate` handler: ignores bots, strips mention tags (`/<@!?\d+>/g`), detects bot-owned threads via `ownerId === client.user.id`, collects last 20 non-bot thread messages as history, calls `handleQuestion`, sends response, creates a follow-up thread (non-thread channels only)
- `Events.InteractionCreate` handler: `/support` slash command with `deferReply()` before Claude call (prevents 3s Discord timeout), `editReply()` after, creates thread on reply message
- `handleQuestion`: calls `queryCodebase`, then `createIssue` if `issueProposal` present, appends GitHub URL to answer
- `sendResponse`: <= 2000 chars as plain text, 2001-4096 as embed, > 4096 split into 2000-char sequential chunks

### commands/support.ts
- `SlashCommandBuilder` with name `support` and required string option `question`

### commands/deploy.ts
- Standalone registration script — guild-scoped (instant, dev) if `DISCORD_GUILD_ID` set, global (1hr propagation, prod) otherwise
- `npm run deploy-commands` script already in package.json from Plan 01

### index.ts
- Now stores `Client` from `startBot` in `discordClient` (typed as `Client` not `{ destroy }`) for proper typed shutdown

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Mention regex mismatch with test mock IDs**
- **Found during:** Task 1 GREEN phase (tests failing after implementation)
- **Issue:** Test mocks used `'bot-user-id'` (with hyphens) as Discord client user ID; the correct regex `/<@!?\d+>/g` only strips numeric snowflake IDs — `bot-user-id` would not be stripped
- **Fix:** Updated test mock to use `'123456789'` (numeric snowflake-style) and all message content references to `<@123456789>` — regex in bot.ts is correct per Discord spec
- **Files modified:** src/__tests__/bot.test.ts
- **Commit:** 55de027 (bundled with implementation)

## Verification Results

- `cd services/support-bot && npm test` — 41/41 tests pass (4 test files)
- `grep "Events.MessageCreate"` — present in bot.ts
- `grep "Events.InteractionCreate"` — present in bot.ts
- `grep "deferReply"` — present in bot.ts
- `grep "queryCodebase"` — present in bot.ts
- `grep "createIssue"` — present in bot.ts
- `npx tsx --check src/commands/deploy.ts` — exits 0

## Self-Check: PASSED

Files exist:
- services/support-bot/src/bot.ts: FOUND
- services/support-bot/src/commands/support.ts: FOUND
- services/support-bot/src/commands/deploy.ts: FOUND
- services/support-bot/src/__tests__/bot.test.ts: FOUND

Commits exist:
- 6de3797: FOUND (RED phase tests)
- 55de027: FOUND (Task 1 implementation)
- 3036266: FOUND (Task 2 deploy.ts)
