# Phase 37: Migrate youtube-innertube and discord-listener - Research

**Researched:** 2026-03-17
**Domain:** Go SDK migration — LeadershipListener archetype
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**SMClient exposure in LeadershipListener**
- Add `SMClient() *sourcemanager.Client` accessor to `shared/listener/leadership.go` — symmetric with `LeadershipCoordinator()`
- Returns nil when `SOURCE_MANAGER_SECRET` is absent (consistent with `LeadershipCoordinator()` nil-return behavior)
- `streams.NewManager` signature unchanged; `main.go` calls `ll.LeadershipCoordinator()` and `ll.SMClient()` and passes both through as before
- `streams.Manager` already nil-checks `smClient` before calling `ActivateSource`/`GetSources` — existing nil guards cover the absent-secret case

**youtube-innertube migration pattern**
- Replace manual `sourcemanager.NewSigningTokenSource` + `sourcemanager.NewClient` + `sourcemanager.NewLeadershipCoordinator` block with `listener.NewLeadershipListenerFromEnv(base, "youtube", logger)`
- `getEnv` local function deleted entirely; replaced by `listener.Env()` throughout `cmd/main.go` (same pattern as phase 35)
- All other main.go structure (HTTP server, metrics, deletion buffer, streamManager setup) unchanged

**Discord gateway goroutine**
- Remove `if leaderCoord != nil` nil-guard from `EnsureLeadership` call — always call `ll.LeadershipCoordinator().EnsureLeadership(...)` (nil-safe passthrough returns `acquired=true, err=nil` when coordinator is absent)
- Keep `if ll.LeadershipCoordinator() != nil` guard specifically for `metrics.SetShardOwnership` calls — avoids spurious shard ownership metrics when leadership is disabled
- Remove the manual `log.Warn("SOURCE_MANAGER_URL or SOURCE_MANAGER_SECRET not set...")` — SDK already logs `SOURCE_MANAGER_SECRET not set — leadership coordination disabled` in `NewLeadershipListenerFromEnv`
- `getEnv` local function replaced by `listener.Env()` in discord `cmd/main.go` as well

**Shutdown ordering**
- Both services keep fully custom shutdown sequences — `ShutdownCoordinator` does not apply (no ChannelManager)
- youtube-innertube: `streamManager.Shutdown(shutdownCtx)` → `deletionBuffer.Shutdown()` → `srv.Shutdown(shutdownCtx)` unchanged
- discord-listener: `signal.NotifyContext`-driven shutdown (ctx.Done()), `gwClient.Close()` + `relayMgr.Stop()` + `srv.Shutdown(shutdownCtx)` unchanged

**Test scope**
- Goleak smoke test in `cmd/main_sdk_test.go` for **both** services
- Test constructs `ListenerBase` with `testutil.NewMockCoordinator()`, wraps in `LeadershipListener` via `NewLeadershipListenerFromEnv`, calls `base.Start(ctx, nil)` (no ChannelManager), then `base.Stop()`, verifies `goleak.VerifyNone` passes
- SDK-only scope — no gateway client or stream manager in the test
- No additional tests beyond the smoke test per service

### Claude's Discretion
- Exact position of `"youtube"` / `"discord-listener"` platform string passed to `NewLeadershipListenerFromEnv`
- Whether `base.Start(ctx, nil)` is valid for leadership-only services or if a no-op ChannelManager stub is needed (Claude to verify)
- Package-level doc comment updates for migrated `main.go` files

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| MIGRATE-04 | youtube-listener-innertube `cmd/main.go` migrated to use `LeadershipListener` — no CoordinatorClient; SDK leadership wiring is the sole integration point | Existing manual leadership block (lines 142–156 of main.go) is a direct replacement target for `NewLeadershipListenerFromEnv`; `SMClient()` accessor must be added to `LeadershipListener` first |
| MIGRATE-05 | discord-listener `cmd/main.go` migrated to use `LeadershipListener` — shard ownership coordination via existing Redis lock pattern unchanged | Existing manual leadership block (lines 189–200 of main.go) plus gateway goroutine guard pattern (lines 235–264) are the replacement targets; nil-safe `EnsureLeadership` passthrough removes the outer nil-check |
</phase_requirements>

## Summary

Phase 37 migrates two leadership-only listeners — `youtube-listener-innertube` and `discord-listener` — to use `LeadershipListener` from the shared SDK. Neither service uses `ChannelManager`, assignment loops, or `CoordinatorClient`. The only SDK integration point is `LeadershipListener`, which handles `LeadershipCoordinator` and `sourcemanager.Client` construction from environment variables.

Both services already have `replace ../../shared` directives in their `go.mod` files. The migration in each service is a targeted replacement of a small manual leadership block in `cmd/main.go`, replacement of the local `getEnv` function with `listener.Env()`, and addition of a goleak smoke test.

The one pre-migration SDK change required is adding `SMClient() *sourcemanager.Client` to `shared/listener/leadership.go` so that `youtube-listener-innertube/streams.NewManager` can receive the `*sourcemanager.Client` extracted from the `LeadershipListener` — symmetric with the existing `LeadershipCoordinator()` accessor.

**Primary recommendation:** Three-plan structure: (1) add `SMClient()` accessor to SDK and add goleak to both go.mod files, (2) migrate youtube-innertube, (3) migrate discord-listener. Each plan independently buildable and testable.

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `shared/listener` | local replace | `LeadershipListener`, `ListenerBase`, `Env()`, `testutil.MockCoordinator` | Phase 34 SDK; used by Twitch (Phase 35) and Kick (Phase 36) |
| `shared/sourcemanager` | local replace | `LeadershipCoordinator`, `Client` | Already referenced in both services; SDK wraps construction |
| `go.uber.org/goleak` | v1.3.0 | Goroutine leak detection in smoke tests | Same version as kick-listener (Phase 36 precedent) |

### No New Dependencies
Both services already import `shared` via replace directive. No module-level changes beyond adding `go.uber.org/goleak` to each service's `go.mod`.

**Installation (both services):**
```bash
cd services/youtube-listener-innertube && go get go.uber.org/goleak@v1.3.0
cd services/discord-listener && go get go.uber.org/goleak@v1.3.0
```

## Architecture Patterns

### Established Migration Pattern (from Phases 35 and 36)

The pattern is identical to kick-listener (Phase 36), which is also a `LeadershipListener` archetype:

1. Add goleak to go.mod as a direct dependency
2. Create `cmd/main_sdk_test.go` with `TestListenerBase_StartStop_NoGoroutineLeak`
3. Replace manual leadership block in `cmd/main.go` with `NewLeadershipListenerFromEnv`
4. Replace local `getEnv` with `listener.Env()`
5. Wire `ll.LeadershipCoordinator()` and `ll.SMClient()` to domain constructors
6. Verify with `go test ./... -race -count=1` and `make build-all`

### SDK Constructor Call Chain

```
// Plan 01 — add to shared/listener/leadership.go
func (l *LeadershipListener) SMClient() *sourcemanager.Client {
    return l.smClient
}

// Plan 02 — youtube-innertube cmd/main.go
base := listener.NewListenerBase(listener.ListenerConfig{}, nil, nil, "", logger)
ll, err := listener.NewLeadershipListenerFromEnv(base, "youtube", logger)
// ...
streamManager := streams.NewManager(ll.LeadershipCoordinator(), ll.SMClient(), ...)

// Plan 03 — discord-listener cmd/main.go
base := listener.NewListenerBase(listener.ListenerConfig{}, nil, nil, "", logger)
ll, err := listener.NewLeadershipListenerFromEnv(base, "discord-listener", log)
// In gateway goroutine:
acquired, err := ll.LeadershipCoordinator().EnsureLeadership(ctx, "shard:0", func() { ... })
if ll.LeadershipCoordinator() != nil { metrics.SetShardOwnership(1) }
```

### ListenerBase for Leadership-Only Services

For leadership-only services, `ListenerBase` is constructed but never `Start`-ed in production — it is only the carrier for `LeadershipListener` construction. The SDK's `NewLeadershipListenerFromEnv` embeds `ListenerBase` but the leadership-only services do not call `base.Start()` because they have no `ChannelManager`.

**Critical discovery:** `base.Start(ctx, nil)` will **panic** in production and in tests. `base.Start()` calls `mgr.UpdateAssignedSourceIDs(...)`, `mgr.Start(ctx)`, `mgr.HandleMigrationEvent(...)` (via migration subscriber) — all unconditional, no nil checks. Do NOT pass nil for `mgr`.

**Smoke test resolution:** The smoke test in CONTEXT.md says `base.Start(ctx, nil)`. This MUST be corrected to use a `mockChannelManagerForTest` stub (same no-op struct as in kick-listener's `cmd/main_sdk_test.go`). The kick-listener smoke test (established Phase 36 pattern) uses `base.Start(ctx, mgr)` with a stub, not nil. Follow that pattern.

### Anti-Patterns to Avoid

- **Passing nil ChannelManager to base.Start():** Panics at `mgr.UpdateAssignedSourceIDs`. Use `mockChannelManagerForTest` in tests.
- **Keeping the local `getEnv` function:** It duplicates `listener.Env()`. Delete it entirely.
- **Using `ShutdownCoordinator`:** Not applicable — neither service has a `ChannelManager`. Both keep their custom shutdown sequences.
- **Double-calling `StartJWTRefresh`:** If `base.Start()` were called, it would call `StartJWTRefresh` internally. For leadership-only services not calling `base.Start()`, JWT refresh is not used — `ListenerBase` is purely a container here.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| LeadershipCoordinator construction | Manual `tokenSource` + `NewClient` + `NewLeadershipCoordinator` block | `listener.NewLeadershipListenerFromEnv` | SDK handles nil-safe absent-secret case and logs uniformly |
| Env var with default | Local `getEnv(key, fallback)` | `listener.Env(key, fallback)` | Identical signature, eliminates copy-paste |
| `sourcemanager.Client` exposure | New accessor on per-service type | `ll.SMClient()` on `LeadershipListener` | Symmetric with `LeadershipCoordinator()`, one place to change |

## Common Pitfalls

### Pitfall 1: Platform String for youtube-innertube
**What goes wrong:** Passing `"youtube"` vs `"youtube-innertube"` vs `"youtube-listener"` affects the signing token source's `platform` claim in the JWT and how source-manager identifies the caller.
**Why it happens:** The existing manual code uses `"youtube"` as the platform string passed to `NewLeadershipCoordinator` (line 155 of current main.go).
**How to avoid:** Pass `"youtube"` to `NewLeadershipListenerFromEnv` — matching existing behavior. Note: `NewLeadershipListenerFromEnv` internally uses `platform+"-listener"` as the token source service name (e.g., `"youtube-listener"`), which matches the pattern already in production for kick-listener (`"kick-listener"`). The existing innertube code passes `"youtube"` as platform and `"youtube"` as the service token prefix — verify that `platform+"-listener"` = `"youtube-listener"` is consistent with what source-manager expects.

### Pitfall 2: Platform String for discord-listener
**What goes wrong:** The existing discord code uses `"discord-listener"` as the signing token source service name (line 191) and `"discord"` as the platform name for `NewLeadershipCoordinator` (line 196).
**Why it happens:** The SDK's `NewLeadershipListenerFromEnv` uses `platform+"-listener"` for the token source. If `platform = "discord"`, the token source service name becomes `"discord-listener"` — matching the existing behavior exactly.
**How to avoid:** Pass `"discord"` as the platform string to `NewLeadershipListenerFromEnv`. This produces token source service name `"discord-listener"` and coordinator platform `"discord"`, identical to the current manual wiring.

### Pitfall 3: Gateway goroutine nil-guard removal
**What goes wrong:** The existing gateway goroutine has `if leaderCoord != nil { ... EnsureLeadership ... }` — the entire leadership gate is inside a nil check. After migration, `ll.LeadershipCoordinator()` is always callable (nil-safe passthrough), but the outer structure must change.
**Why it happens:** The current code skips leadership entirely when `leaderCoord == nil`. After migration, the SDK's nil-safe `EnsureLeadership` returns `acquired=true, err=nil` when coordinator is nil — the gate is always "passed" when coordination is disabled.
**How to avoid:** Remove the outer `if leaderCoord != nil` from the goroutine. Keep the `if ll.LeadershipCoordinator() != nil` guard only around `metrics.SetShardOwnership` calls, per the locked decision in CONTEXT.md.

### Pitfall 4: SMClient() accessor must exist before youtube-innertube migration
**What goes wrong:** `streams.NewManager` takes `*sourcemanager.Client` as second arg. Without `ll.SMClient()`, there is no way to extract it from the `LeadershipListener`.
**Why it happens:** Phase 37 adds this accessor — it does not yet exist in `shared/listener/leadership.go`.
**How to avoid:** Add `SMClient()` accessor in Plan 01 (SDK change) before Plan 02 (youtube-innertube migration). Both plans can land in the same wave only if Plan 01 completes first.

### Pitfall 5: `sourcemanager` import removal from discord main.go
**What goes wrong:** After removing the manual leadership block, `sourcemanager` may appear to be unused. However, `sourcemanager.LeadershipCoordinator` may still be referenced in type assertions or variable declarations within the gateway goroutine depending on how the migration is structured.
**Why it happens:** After migration, `leaderCoord` variable is gone; `ll.LeadershipCoordinator()` returns `*sourcemanager.LeadershipCoordinator` but via the SDK import, not a direct `sourcemanager` package reference in main.go.
**How to avoid:** After rewriting main.go, check whether any direct reference to `sourcemanager.*` remains. If not, remove the `sourcemanager` import. The compiler will flag unused imports.

## Code Examples

### SMClient() accessor to add to leadership.go
```go
// Source: shared/listener/leadership.go (to be added in Plan 01)
// SMClient returns the source manager client.
// May be nil when SOURCE_MANAGER_SECRET was absent.
func (l *LeadershipListener) SMClient() *sourcemanager.Client {
    return l.smClient
}
```

### youtube-innertube leadership replacement (in cmd/main.go)
```go
// Source: pattern from shared/listener/leadership.go + streams/manager.go

// BEFORE (lines 142-156 of current main.go):
// sourceManagerURL := getEnv("SOURCE_MANAGER_URL", "http://source-manager:8088")
// sourceManagerSecret := getEnv("SOURCE_MANAGER_SECRET", "dev-service-secret")
// var leaderCoord *sourcemanager.LeadershipCoordinator
// var smClient *sourcemanager.Client
// if sourceManagerSecret == "" { ... } else { tokenSource := ...; smClient, _ = ...; leaderCoord = ... }

// AFTER:
base := listener.NewListenerBase(listener.ListenerConfig{}, nil, nil, "", logger)
ll, err := listener.NewLeadershipListenerFromEnv(base, "youtube", logger)
if err != nil {
    logger.Fatal("Failed to initialize leadership listener", zap.Error(err))
}

// Then pass to streams.NewManager:
streamManager := streams.NewManager(
    ll.LeadershipCoordinator(),
    ll.SMClient(),
    repository,
    // ... remaining args unchanged
)
```

### discord gateway goroutine after migration
```go
// Source: services/discord-listener/cmd/main.go (gateway goroutine, to be rewritten)

go func() {
    for {
        select {
        case <-ctx.Done():
            return
        default:
        }

        // EnsureLeadership is nil-safe: returns acquired=true, err=nil when coordinator absent
        acquired, err := ll.LeadershipCoordinator().EnsureLeadership(ctx, "shard:0", func() {
            log.Warn("Lost gateway shard ownership — disconnecting")
            if ll.LeadershipCoordinator() != nil {
                metrics.SetShardOwnership(0)
            }
        })
        if err != nil || !acquired {
            log.Info("Waiting for shard ownership...")
            select {
            case <-time.After(5 * time.Second):
            case <-ctx.Done():
                return
            }
            continue
        }
        if ll.LeadershipCoordinator() != nil {
            metrics.SetShardOwnership(1)
        }

        log.Info("Starting Gateway connection")
        if err := gwClient.Connect(ctx); err != nil && ctx.Err() == nil {
            log.Warn("Gateway disconnected, reconnecting in 5s", zap.Error(err))
            if ll.LeadershipCoordinator() != nil {
                metrics.SetShardOwnership(0)
            }
            select {
            case <-time.After(5 * time.Second):
            case <-ctx.Done():
                return
            }
        }
    }
}()
```

### Smoke test pattern (both services)
```go
// Source: services/kick-listener/cmd/main_sdk_test.go (Phase 36 established pattern)
// Apply identically to both youtube-innertube and discord-listener

package main

import (
    "context"
    "testing"
    "time"

    "github.com/caesar/all-chat/shared/coordination"
    "github.com/caesar/all-chat/shared/listener"
    "github.com/caesar/all-chat/shared/listener/testutil"
    "go.uber.org/goleak"
)

type mockChannelManagerForTest struct{}

func (m *mockChannelManagerForTest) Start(_ context.Context) error                            { return nil }
func (m *mockChannelManagerForTest) Stop()                                                     {}
func (m *mockChannelManagerForTest) HandleMigrationEvent(_ *coordination.MigrationEvent) error { return nil }
func (m *mockChannelManagerForTest) UpdateAssignedSourceIDs(_ map[string]bool)                {}
func (m *mockChannelManagerForTest) GetFilteredAssignmentCount() int                          { return 0 }
func (m *mockChannelManagerForTest) GetActiveChannels() []string                              { return nil }
func (m *mockChannelManagerForTest) GetActiveChannelCount() int                               { return 0 }

func TestListenerBase_StartStop_NoGoroutineLeak(t *testing.T) {
    defer goleak.VerifyNone(t)

    mock := &testutil.MockCoordinator{}
    cfg := listener.ListenerConfig{
        HeartbeatInterval:         20 * time.Millisecond,
        AssignmentRefreshInterval: 20 * time.Millisecond,
        StartupJitterMax:          0,
    }
    base := listener.NewListenerBase(cfg, mock, nil, "test-pod", nil)
    mgr := &mockChannelManagerForTest{}

    ctx, cancel := context.WithCancel(context.Background())
    if err := base.Start(ctx, mgr); err != nil {
        t.Fatal(err)
    }
    cancel()
    base.Stop()
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual `tokenSource` + `NewClient` + `NewLeadershipCoordinator` in each service | `listener.NewLeadershipListenerFromEnv(base, platform, logger)` | Phase 34 (SDK) + Phase 36 (kick) | Eliminates ~15 lines of boilerplate per service |
| Local `getEnv(key, fallback)` copied to each service | `listener.Env(key, fallback)` | Phase 34 (SDK) + Phase 35 (twitch) | Single source of truth |
| Per-service `if leaderCoord != nil` outer nil guard | `ll.LeadershipCoordinator()` always callable (nil-safe passthrough) | Phase 34 (SDK) | Removes branching in gateway goroutine |

## Open Questions

1. **`base.Start(ctx, nil)` in CONTEXT.md smoke test description**
   - What we know: `base.Start()` calls `mgr.UpdateAssignedSourceIDs(assignedIDs)` unconditionally on line 76 of base.go — passing nil panics.
   - What's unclear: CONTEXT.md says "calls `base.Start(ctx, nil)`" for the smoke test.
   - Recommendation: The CONTEXT.md description is incorrect; this is listed under Claude's Discretion ("whether `base.Start(ctx, nil)` is valid..."). Use `mockChannelManagerForTest` stub exactly as in kick-listener's Phase 36 smoke test. The stub is 7 no-op methods, identical across all services.

2. **youtube-innertube ListenerBase construction — what coordinator client to pass**
   - What we know: youtube-innertube is leadership-only — no `CoordinatorClient` for heartbeat/assignment loops. `base.Start()` is never called in production for this service.
   - What's unclear: If `base.Start()` is never called in production, what coordinator client (if any) should be passed to `NewListenerBase`?
   - Recommendation: Pass `nil` for the coordinator client — `ListenerBase.Start()` is never called for leadership-only services in production, so the coordinator client is never exercised. The smoke test uses `testutil.MockCoordinator` to exercise the `Start/Stop` cycle safely.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` package (stdlib) + goleak v1.3.0 |
| Config file | none — standard `go test` |
| Quick run command | `go test ./cmd/... -count=1 -run TestListenerBase_StartStop_NoGoroutineLeak` |
| Full suite command | `go test ./... -race -count=1` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| MIGRATE-04 | youtube-innertube ListenerBase lifecycle has zero goroutine leaks | smoke/unit | `cd services/youtube-listener-innertube && go test ./cmd/... -count=1 -run TestListenerBase_StartStop_NoGoroutineLeak` | ❌ Wave 0 |
| MIGRATE-04 | youtube-innertube compiles and all existing tests pass | build/unit | `cd services/youtube-listener-innertube && go build ./cmd/... && go test ./... -count=1` | ✅ (existing tests) |
| MIGRATE-05 | discord-listener ListenerBase lifecycle has zero goroutine leaks | smoke/unit | `cd services/discord-listener && go test ./cmd/... -count=1 -run TestListenerBase_StartStop_NoGoroutineLeak` | ❌ Wave 0 |
| MIGRATE-05 | discord-listener compiles and all existing tests pass | build/unit | `cd services/discord-listener && go build ./cmd/... && go test ./... -count=1` | ✅ (existing tests) |
| Both | Cross-module build clean | build | `make build-all` | ✅ (Makefile target exists) |

### Sampling Rate
- **Per task commit:** `go test ./cmd/... -count=1 -run TestListenerBase_StartStop_NoGoroutineLeak` (per-service)
- **Per wave merge:** `go test ./... -race -count=1` (per-service) + `make build-all`
- **Phase gate:** Full suite green in both services before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `services/youtube-listener-innertube/cmd/main_sdk_test.go` — covers MIGRATE-04 goroutine leak smoke test; requires `go.uber.org/goleak@v1.3.0` in go.mod
- [ ] `services/discord-listener/cmd/main_sdk_test.go` — covers MIGRATE-05 goroutine leak smoke test; requires `go.uber.org/goleak@v1.3.0` in go.mod
- [ ] goleak install (both): `go get go.uber.org/goleak@v1.3.0` in each service directory

## Sources

### Primary (HIGH confidence)
- Direct code inspection: `shared/listener/leadership.go` — `LeadershipListener` struct, `NewLeadershipListenerFromEnv`, `LeadershipCoordinator()` accessor; confirmed `smClient` field exists but no `SMClient()` accessor yet
- Direct code inspection: `shared/listener/base.go` — `ListenerBase.Start()` method; confirmed `mgr` is called unconditionally (nil-safe passthrough for nil ChannelManager does NOT exist)
- Direct code inspection: `services/youtube-listener-innertube/cmd/main.go` — manual leadership block at lines 142–156; `getEnv` function at line 244
- Direct code inspection: `services/discord-listener/cmd/main.go` — manual leadership block at lines 189–200; gateway goroutine guard at lines 235–264; `getEnv` at line 307
- Direct code inspection: `services/youtube-listener-innertube/streams/manager.go` — `NewManager` signature; nil guards on `m.leader` and `m.smClient` throughout
- Direct code inspection: `services/kick-listener/cmd/main_sdk_test.go` — canonical smoke test pattern for LeadershipListener archetype (Phase 36)
- Direct code inspection: `shared/listener/testutil/mock_coordinator.go` — `MockCoordinator` struct and interface compliance
- Direct code inspection: both `go.mod` files — `replace ../../shared` directive present; goleak not yet in either

### Secondary (MEDIUM confidence)
- `.planning/phases/36-migrate-kick-listener/36-02-PLAN.md` — established execution pattern for LeadershipListener migration (identical archetype)
- `.planning/STATE.md` — accumulated decisions relevant to SDK migration phases

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries verified via direct code inspection
- Architecture: HIGH — migration pattern established in Phases 35 and 36; both target services fully read
- Pitfalls: HIGH — nil-safety of `base.Start(ctx, nil)` verified by reading base.go line 76; platform string behavior verified from current manual wiring in both services

**Research date:** 2026-03-17
**Valid until:** 2026-04-17 (stable SDK, no external dependencies added)
