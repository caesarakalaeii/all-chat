---
phase: 02-support-bot-operational-awareness
plan: 02
subsystem: discord-bot
tags: [discord, typescript, vitest, bot, mention, env-validation]

# Dependency graph
requires:
  - phase: 02-01
    provides: "InfraVerdict type in QueryResult, BotConfig with leadDeveloperDiscordId/grafanaUrl/grafanaServiceAccountToken fields"
provides:
  - "Lead dev @mention injected into bot responses on infra verdict or issue creation"
  - "Fail-fast env var validation for LEAD_DEVELOPER_DISCORD_ID, GRAFANA_URL, GRAFANA_SERVICE_ACCOUNT_TOKEN"
affects: ["02-03", "deploy", "kubernetes-secrets"]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "shouldPingLeadDev computed from infraVerdict + issueProposal before prepending mention"
    - "enqueue returns Promise<void> for testable async Discord event handlers"

key-files:
  created: []
  modified:
    - services/support-bot/src/bot.ts
    - services/support-bot/src/index.ts
    - services/support-bot/src/__tests__/bot.test.ts
    - services/support-bot/src/__tests__/index.test.ts

key-decisions:
  - "shouldPingLeadDev is a derived boolean (infraVerdict.type==='infrastructure' OR issueProposal!==null) — evaluated after issue creation so one mention covers both conditions"
  - "enqueue changed to return Promise<void> instead of void — Discord.js ignores return value, but tests can await the queued task for synchronous assertion"
  - "LEAD_DEVELOPER_DISCORD_ID, GRAFANA_URL, GRAFANA_SERVICE_ACCOUNT_TOKEN moved from optional fallback to required entries in the required Record — consistent fail-fast pattern with all other env vars"

patterns-established:
  - "Test thread mocks must include send: vi.fn() — bot always calls thread.send after startThread"
  - "buildMessage helper defined at module scope — shared across MessageCreate, Lead developer @mention, and Response formatting describe blocks"

requirements-completed: [OPS-05, OPS-06]

# Metrics
duration: 15min
completed: 2026-03-26
---

# Phase 02 Plan 02: Lead Developer @Mention and Env Var Validation Summary

**Lead dev Discord @mention injected when infra issue detected or GitHub issue created; LEAD_DEVELOPER_DISCORD_ID, GRAFANA_URL, GRAFANA_SERVICE_ACCOUNT_TOKEN now required at startup**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-03-26T10:07:00Z
- **Completed:** 2026-03-26T10:16:00Z
- **Tasks:** 1
- **Files modified:** 4

## Accomplishments
- `handleQuestion` in bot.ts now prepends `<@leadDeveloperDiscordId>` to answers when infraVerdict.type is 'infrastructure' or issueProposal is not null
- @mention appears exactly once even when both conditions are true simultaneously
- `validateEnv` in index.ts now requires LEAD_DEVELOPER_DISCORD_ID, GRAFANA_URL, and GRAFANA_SERVICE_ACCOUNT_TOKEN — exits with 1 if any are missing
- All 60 tests pass (up from 39 passing before this plan)

## Task Commits

Each task was committed atomically:

1. **Task 1: Add lead dev @mention logic to bot.ts and new env var validation to index.ts** - `10407b2` (feat)

**Plan metadata:** (pending)

_Note: TDD approach — tests and implementation combined in single commit since baseline tests were pre-existing failures requiring coordinated fix_

## Files Created/Modified
- `services/support-bot/src/bot.ts` - Added shouldPingLeadDev logic in handleQuestion; enqueue now returns Promise<void>
- `services/support-bot/src/index.ts` - Three new required env vars replacing optional fallbacks
- `services/support-bot/src/__tests__/bot.test.ts` - Added 'Lead developer @mention' describe block (5 tests); fixed thread mocks; moved buildMessage to module scope; updated all mocks to include infraVerdict: null
- `services/support-bot/src/__tests__/index.test.ts` - Added 3 new env var tests; updated BotConfig assertions to include new fields; added new env vars to beforeEach

## Decisions Made
- `shouldPingLeadDev` is evaluated after issue creation so that both the issue URL and the @mention appear in the same message with a single mention
- `enqueue` returns `Promise<void>` — Discord.js ignores event handler return values so this has no production impact, but enables `await handler!(msg)` in tests
- All three new env vars are moved to the required Record (fail-fast) rather than kept as optional — bot cannot function without lead dev ID or Grafana credentials

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed 13 pre-existing test failures from broken async event handler contract and missing thread mock methods**
- **Found during:** Task 1 (bot.test.ts review before writing new tests)
- **Issue:** (a) `enqueue` returned void so `await handler!(msg)` resolved before queryCodebase was called; (b) `startThread` mocks resolved to `{}` without a `send` method, causing `TypeError: thread.send is not a function`; (c) Response formatting tests checked `channel.send` but bot sends to `thread.send` for non-thread messages; (d) `buildMessage` was scoped inside `MessageCreate handler` describe block but needed by `Response formatting` tests
- **Fix:** Made `enqueue` return the task promise; fixed all thread mocks to include `send: vi.fn()`; updated Response formatting tests to capture `threadSend` from the mock; moved `buildMessage` to module scope; updated all `mockResolvedValue` calls to include `infraVerdict: null`
- **Files modified:** `services/support-bot/src/bot.ts`, `services/support-bot/src/__tests__/bot.test.ts`
- **Verification:** 60/60 tests pass
- **Committed in:** 10407b2 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - bug)
**Impact on plan:** Pre-existing test infrastructure bugs fixed as part of writing new tests. No scope creep beyond plan requirements.

## Issues Encountered
None — all issues were pre-existing and resolved inline under deviation Rule 1.

## User Setup Required
The Kubernetes secret for the support-bot deployment needs three new values:
- `LEAD_DEVELOPER_DISCORD_ID` — Discord snowflake ID of the lead developer to ping
- `GRAFANA_URL` — Base URL of the Grafana instance
- `GRAFANA_SERVICE_ACCOUNT_TOKEN` — Service account token with read access

These are validated at startup; the bot will exit(1) if any are missing.

## Self-Check: PASSED

All created/modified files verified present. Commit 10407b2 confirmed in git log.

## Next Phase Readiness
- Bot now correctly @mentions lead developer on infra verdicts and issue creation
- All env vars validated at startup for fail-fast behavior
- Plan 02-03 (kubectl + Dockerfile RBAC) is already complete — phase 02 is fully done

---
*Phase: 02-support-bot-operational-awareness*
*Completed: 2026-03-26*
