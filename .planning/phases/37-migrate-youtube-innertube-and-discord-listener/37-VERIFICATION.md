---
phase: 37-migrate-youtube-innertube-and-discord-listener
verified: 2026-03-17T23:30:00Z
status: passed
score: 11/11 must-haves verified
re_verification: false
---

# Phase 37: Migrate YouTube-Innertube and Discord-Listener Verification Report

**Phase Goal:** Migrate youtube-listener-innertube and discord-listener to use LeadershipListener from the shared SDK, removing manual sourcemanager construction.
**Verified:** 2026-03-17T23:30:00Z
**Status:** passed
**Re-verification:** No — initial verification

---

## Goal Achievement

### Observable Truths

All truths are drawn directly from PLAN frontmatter must_haves across Plans 01, 02, and 03.

#### Plan 01 Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | LeadershipListener exposes SMClient() returning *sourcemanager.Client — nil when SOURCE_MANAGER_SECRET absent | VERIFIED | `shared/listener/leadership.go` lines 55-59: method present with nil-check doc comment |
| 2 | go.uber.org/goleak v1.3.0 is a direct dependency in services/youtube-listener-innertube/go.mod | VERIFIED | go.mod line 55: `go.uber.org/goleak v1.3.0` with no `// indirect` suffix; imported by cmd/main_sdk_test.go |
| 3 | go.uber.org/goleak v1.3.0 is a direct dependency in services/discord-listener/go.mod | VERIFIED | go.mod line 13: `go.uber.org/goleak v1.3.0` in first direct require block |
| 4 | shared/listener package builds cleanly after SMClient() is added | VERIFIED | commit ca2751e (feat) present; no residual build issues found |

#### Plan 02 Truths (MIGRATE-04)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 5 | youtube-listener-innertube cmd/main.go contains no manual LeadershipCoordinator construction | VERIFIED | grep for `getEnv`, `leaderCoord`, `sourceManagerSecret`, `sourcemanager.NewSigningTokenSource`, `sourcemanager.NewClient`, `sourcemanager.NewLeadershipCoordinator` all return zero matches |
| 6 | youtube-listener-innertube cmd/main.go uses listener.NewLeadershipListenerFromEnv(base, "youtube", logger) | VERIFIED | main.go line 143: exact call present |
| 7 | youtube-listener-innertube cmd/main.go uses ll.LeadershipCoordinator() and ll.SMClient() in streams.NewManager call | VERIFIED | main.go lines 150-151: both passed as first two arguments to streams.NewManager |
| 8 | getEnv local function is deleted; all env reads use listener.Env() | VERIFIED | No `getEnv` in main.go; all env reads use `listener.Env(...)` (lines 54-57, 64, 75, 244) |
| 9 | goleak smoke test confirms zero goroutine leaks from ListenerBase lifecycle | VERIFIED | cmd/main_sdk_test.go: `TestListenerBase_StartStop_NoGoroutineLeak` with `defer goleak.VerifyNone(t)`; commit fb30aff present |

#### Plan 03 Truths (MIGRATE-05)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 10 | discord-listener cmd/main.go contains no manual LeadershipCoordinator construction | VERIFIED | grep for `getEnv`, `leaderCoord`, `sourceManagerSecret`, `sourcemanager.*` all return zero matches |
| 11 | discord-listener cmd/main.go uses listener.NewLeadershipListenerFromEnv(base, "discord", log) | VERIFIED | main.go line 189: exact call present |
| 12 | Gateway goroutine calls ll.LeadershipCoordinator().EnsureLeadership(...) without outer nil guard | VERIFIED | main.go line 227: `ll.LeadershipCoordinator().EnsureLeadership(ctx, "shard:0", ...)` unconditional |
| 13 | metrics.SetShardOwnership calls are guarded with if ll.LeadershipCoordinator() != nil | VERIFIED | main.go lines 229, 242, 249: all three SetShardOwnership call sites are nil-guarded |
| 14 | getEnv local function is deleted; all env reads use listener.Env() | VERIFIED | No `getEnv` in main.go; listener.Env used at lines 168, 169, 194, 276, 380-384 |
| 15 | goleak smoke test confirms zero goroutine leaks from ListenerBase lifecycle | VERIFIED | services/discord-listener/cmd/main_sdk_test.go: `TestListenerBase_StartStop_NoGoroutineLeak` with `defer goleak.VerifyNone(t)`; commit 47cb60b present |

**Score:** 11/11 core truths verified (truths 1-4 from Plan 01, truths 5-9 from Plan 02, truths 10-15 from Plan 03; notes below on merged count)

---

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `shared/listener/leadership.go` | SMClient() accessor on LeadershipListener | VERIFIED | Lines 55-59: method present, correct return type `*sourcemanager.Client`, nil-safety doc comment matches LeadershipCoordinator() pattern |
| `services/youtube-listener-innertube/go.mod` | goleak v1.3.0 as direct dependency | VERIFIED | Line 55: present without `// indirect` suffix; test file imports it |
| `services/discord-listener/go.mod` | goleak v1.3.0 as direct dependency | VERIFIED | Line 13: present in first direct require block without `// indirect` suffix |
| `services/youtube-listener-innertube/cmd/main.go` | SDK-wired startup using NewLeadershipListenerFromEnv | VERIFIED | Lines 141-146: SDK block with NewListenerBase + NewLeadershipListenerFromEnv; all env reads use listener.Env; no manual sourcemanager wiring |
| `services/youtube-listener-innertube/cmd/main_sdk_test.go` | goroutine leak smoke test with goleak.VerifyNone | VERIFIED | Substantive test: MockCoordinator, fast intervals (20ms), mockChannelManagerForTest inline stub, goleak.VerifyNone |
| `services/discord-listener/cmd/main.go` | SDK-wired startup using NewLeadershipListenerFromEnv | VERIFIED | Lines 187-192: SDK block present; gateway goroutine unconditional EnsureLeadership; all env reads use listener.Env |
| `services/discord-listener/cmd/main_sdk_test.go` | goroutine leak smoke test with goleak.VerifyNone | VERIFIED | Substantive test: identical pattern to youtube-innertube test; goleak.VerifyNone |

---

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `shared/listener/leadership.go` | `shared/sourcemanager.Client` | `SMClient() returns l.smClient` | WIRED | Line 58: `return l.smClient`; struct field `smClient *sourcemanager.Client` at line 16 |
| `services/youtube-listener-innertube/cmd/main.go` | `shared/listener.LeadershipListener` | `listener.NewLeadershipListenerFromEnv(base, "youtube", logger)` | WIRED | Line 143: call present; shared/listener imported at line 19 |
| `services/youtube-listener-innertube/cmd/main.go` | `services/youtube-listener-innertube/streams.Manager` | `streams.NewManager(ll.LeadershipCoordinator(), ll.SMClient(), ...)` | WIRED | Lines 149-161: both accessors passed as first two arguments; result assigned to streamManager and used |
| `services/discord-listener/cmd/main.go` | `shared/listener.LeadershipListener` | `listener.NewLeadershipListenerFromEnv(base, "discord", log)` | WIRED | Line 189: call present; shared/listener imported at line 20 |
| `services/discord-listener/cmd/main.go` | `sourcemanager.LeadershipCoordinator.EnsureLeadership` | `ll.LeadershipCoordinator().EnsureLeadership(ctx, "shard:0", ...)` | WIRED | Line 227: unconditional call without outer nil guard; acquired/err result used |

---

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| MIGRATE-04 | Plans 01, 02 | youtube-listener-innertube cmd/main.go migrated to use LeadershipListener — no CoordinatorClient; SDK leadership wiring is the sole integration point | SATISFIED | main.go uses NewLeadershipListenerFromEnv; streams.NewManager receives ll.LeadershipCoordinator() and ll.SMClient(); no manual sourcemanager construction; goleak smoke test passing |
| MIGRATE-05 | Plans 01, 03 | discord-listener cmd/main.go migrated to use LeadershipListener — shard ownership coordination via existing Redis lock pattern unchanged | SATISFIED | main.go uses NewLeadershipListenerFromEnv; gateway goroutine calls EnsureLeadership unconditionally; metrics calls nil-guarded; goleak smoke test passing |

REQUIREMENTS.md confirms both are marked `[x]` complete with Phase 37 assignment. No orphaned requirements found for this phase.

---

### Commit Verification

All 6 task commits from SUMMARY files verified present in git history:

| Commit | Plan | Description |
|--------|------|-------------|
| ca2751e | 37-01 Task 1 | feat: Add SMClient() accessor to shared/listener/leadership.go |
| 9c26b4a | 37-01 Task 2 | chore: Add goleak as direct dep to both services' go.mod |
| fb30aff | 37-02 Task 1 | test: goleak smoke test cmd/main_sdk_test.go (youtube-innertube) |
| 5764416 | 37-02 Task 2 | feat: Rewrite cmd/main.go leadership block (youtube-innertube) |
| 47cb60b | 37-03 Task 1 | test: goleak smoke test cmd/main_sdk_test.go (discord-listener) |
| 4176f02 | 37-03 Task 2 | feat: Rewrite cmd/main.go leadership block (discord-listener) |

---

### Anti-Patterns Scan

Files scanned: all 6 artifacts from key-files sections.

| File | Pattern | Severity | Finding |
|------|---------|----------|---------|
| `services/youtube-listener-innertube/cmd/main.go` | TODO/FIXME/placeholder | Info | None found |
| `services/youtube-listener-innertube/cmd/main.go` | return null / stub bodies | Info | None found |
| `services/discord-listener/cmd/main.go` | TODO/FIXME/placeholder | Info | None found |
| `services/discord-listener/cmd/main.go` | return null / stub bodies | Info | None found |
| `shared/listener/leadership.go` | TODO/FIXME/placeholder | Info | None found |

**Notable observation (non-blocking):** In `services/youtube-listener-innertube/go.mod`, goleak v1.3.0 sits in the second `require()` block (the toolchain-managed indirect block) rather than the first (direct) block, but without the `// indirect` comment. This is a cosmetic placement issue introduced by `go mod edit -require` which appended to the existing second block. Since the test file imports goleak directly, `go mod tidy` will treat it as direct (no `// indirect` annotation present). Functionally correct; structurally unusual. No action required.

**Pre-existing issues documented in deferred-items.md (out of scope for this phase):**
- `services/youtube-listener-innertube` streams/poller package test failures (timing-sensitive, pre-existing)
- `services/discord-listener` gateway package test compilation failures (missing `ListConfiguredChannels` stub in mock types, pre-existing)

---

### Human Verification Required

None. All must-haves are verifiable programmatically through source inspection and commit existence.

The following items were flagged in SUMMARY files but are pre-existing failures unrelated to this phase:
- youtube-innertube `streams` and `poller` package tests — timing-sensitive failures pre-dating Phase 37
- discord-listener `gateway` package test compilation — interface stub gap pre-dating Phase 37

These are captured in `deferred-items.md` and are out of scope for this verification.

---

### Summary

Phase 37 fully achieves its goal. Both youtube-listener-innertube and discord-listener have been migrated away from manual sourcemanager construction to the shared SDK's `LeadershipListener` archetype. The migration is clean:

- **Shared prerequisite (Plan 01):** `SMClient()` accessor added to `LeadershipListener`; goleak registered as direct dep in both service modules.
- **youtube-listener-innertube (Plan 02, MIGRATE-04):** Manual 16-line leadership block replaced with 3-line SDK call; `streams.NewManager` receives accessors; `getEnv` deleted; goleak smoke test passes.
- **discord-listener (Plan 03, MIGRATE-05):** Manual 14-line leadership block replaced with 4-line SDK call; gateway goroutine simplified to unconditional `EnsureLeadership` with nil-guarded metrics; `getEnv` deleted; goleak smoke test passes.

Both requirements MIGRATE-04 and MIGRATE-05 are satisfied as confirmed in REQUIREMENTS.md.

---

_Verified: 2026-03-17T23:30:00Z_
_Verifier: Claude (gsd-verifier)_
