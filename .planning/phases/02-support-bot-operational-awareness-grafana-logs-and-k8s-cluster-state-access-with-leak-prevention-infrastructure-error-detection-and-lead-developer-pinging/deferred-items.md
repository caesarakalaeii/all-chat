# Deferred Items — Phase 02

## Pre-existing bot.test.ts failures (out of scope for 02-01)

**Found during:** Task 1 — confirming test baseline
**Status:** Pre-existing before plan 02-01 changes were applied
**Scope:** Out of scope — bot.test.ts was not modified in this plan

### Failing tests (13 total)

All failures are in `src/__tests__/bot.test.ts` and fail with:
`TypeError: thread.send is not a function` or `TypeError: channel.send is not a function`

The mock setup for `startThread()` returns an object without a `send` method, causing all interaction and message handler tests that send messages to threads or channels to fail.

Root cause: The mock for `message.startThread()` and `interaction.fetchReply().startThread()` does not include a `send` function in the returned mock object.

**These failures existed before this plan was executed** (confirmed via git stash verification).

**Recommendation:** Fix bot.test.ts mock setup in a future plan or quick task to add `send: vi.fn()` to thread mock objects.
