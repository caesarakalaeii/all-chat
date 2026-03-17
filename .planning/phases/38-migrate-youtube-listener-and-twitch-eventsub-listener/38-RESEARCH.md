# Phase 38: Migrate youtube-listener and twitch-eventsub-listener - Research

**Researched:** 2026-03-18
**Domain:** Go SDK migration — LeadershipListener (youtube-listener) + ListenerBase (twitch-eventsub-listener) + ChannelManager interface gap fixes
**Confidence:** HIGH

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**twitch-eventsub: Redis leader election coexistence**
- SDK `ListenerBase` handles coordinator/heartbeat/assignments boilerplate
- Custom Redis SetNX leader election (`leader:twitch-eventsub`) stays for subscription management — controls which pod registers/deletes EventSub webhooks on Twitch's API
- Two separate leadership concerns, both valid, both remain
- Redis lock TTL (10s) and renewal interval (5s) unchanged
- Leader goroutine structure unchanged: `channelManager.Start()` on acquire, `channelManager.Stop()` on loss
- Shutdown sequence unchanged: `releaseLeadership()` + `channelManager.Stop()` before HTTP server shutdown
- `getEnv` local function replaced by `listener.Env()` throughout cmd/main.go

**twitch-eventsub: ChannelManager interface gap fixes**
- `Start` signature changes: `Start(ctx context.Context, interval time.Duration)` → `Start(ctx context.Context) error` (SDK interface)
- `ChannelSyncInterval` (currently 30s constant) pre-configured at construction: `channels.NewManager(..., syncInterval time.Duration)` stores it as a field; `Start(ctx)` uses stored value
- `GetFilteredAssignmentCount() int` added: returns `len(assignedSourceIDs)`
- `GetActiveChannelCount() int` added: returns count of channels with active EventSub subscriptions
- `GetActiveChannels() []string` return type changed from `map[string]*Channel` to `[]string` of broadcaster IDs; old map-returning method renamed `GetActiveChannelMap()` if still needed internally, or deleted if no callers remain
- Compile-time assertion added to `channels/manager.go`: `var _ listener.ChannelManager = (*Manager)(nil)`

**youtube-listener: LeadershipListener migration**
- `NewLeadershipListenerFromEnv(base, "youtube", logger)` replaces manual `sourcemanager.NewSigningTokenSource` + `sourcemanager.NewClient` + `sourcemanager.NewLeadershipCoordinator` block
- `getEnvOrDefault` local function replaced by `listener.Env()` throughout cmd/main.go; `parseIntEnv` stays (wraps Env + Atoi, not a drop-in)
- `streams.NewManager` signature unchanged; main.go extracts `ll.LeadershipCoordinator()` and passes it through — exact Phase 37 youtube-innertube pattern
- Daily quota reset goroutine stays leadership-independent: all pods reset (idempotent via PostgreSQL); no leadership gate added
- `base.Start(ctx, nil)` not called — leadership-only service (established Phase 37 pattern)

**Plan structure**
- 3 plans: 38-01 (youtube-listener), 38-02 (eventsub ChannelManager gap fixes), 38-03 (eventsub ListenerBase wiring)
- youtube-listener migrated first (simpler, validates pattern before the more complex eventsub migration)
- Mixed-fleet monitoring period ends with successful Phase 38 deploy — no separate monitoring phase

**Test scope**
- youtube-listener: goleak smoke test in `cmd/main_sdk_test.go` — constructs `LeadershipListener` with `testutil.NewMockCoordinator()`, verifies `goleak.VerifyNone`. No quota tracker or stream manager in test. Phase 37 pattern.
- twitch-eventsub: goleak smoke test includes ChannelManager — `base.Start(ctx, channelManager)` + `base.Stop()` + `goleak.VerifyNone`. Uses real `channels.NewManager` (or minimal stub) with mock coordinator. Matches Phase 35 twitch-listener pattern.

### Claude's Discretion
- Whether `GetActiveChannelMap()` is kept or deleted (depends on whether any internal eventsub code calls the old map-returning method)
- Exact position of `"youtube"` platform string passed to `NewLeadershipListenerFromEnv`
- Package-level doc comment updates for both migrated `cmd/main.go` files

### Deferred Ideas (OUT OF SCOPE)
- twitch-eventsub: Replace Redis SetNX leader election with source-manager `LeadershipCoordinator` — would change the archetype to `LeadershipListener` and eliminate the `leaderState` struct entirely. Meaningful simplification but out of scope for this phase.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| MIGRATE-03 | youtube-listener `cmd/main.go` migrated to use `LeadershipListener` — quota tracker behavior unchanged; all existing tests pass | LeadershipListener.LeadershipCoordinator() accessor passes *sourcemanager.LeadershipCoordinator to streams.NewManager unchanged; goleak smoke test validates no goroutine leaks |
| MIGRATE-06 | twitch-eventsub-listener `cmd/main.go` migrated to use `ListenerBase` — stateless webhook receiver gains standardized heartbeat and health wiring | ChannelManager interface gap fixes enable compile-time assertion; base.Start(ctx, channelManager) replaces manual coordinator/jitter/assignments/heartbeat/migration blocks |
</phase_requirements>

---

## Summary

Phase 38 closes the v1.6 SDK migration window. Two services remain on hand-rolled coordinator wiring — youtube-listener (LeadershipListener archetype) and twitch-eventsub-listener (ListenerBase archetype with dual leadership concerns). Both services already have `replace ../../shared` directives in `go.mod`; no module graph changes are needed.

The youtube-listener migration is a near-identical replay of Phase 37's youtube-listener-innertube migration. The only difference is that youtube-listener passes `ll.LeadershipCoordinator()` to `streams.NewManager(...)` instead of a no-op stub, and `parseIntEnv` is preserved (it wraps `listener.Env` + `strconv.Atoi` — not a drop-in replacement for `listener.Env`). `getEnvOrDefault` is deleted; all other env lookups become `listener.Env(...)`.

The twitch-eventsub-listener migration is more involved because the existing `channels.Manager` does not satisfy `listener.ChannelManager`. Three new methods must be added, `Start` must be re-signed, and the constructor must absorb the `syncInterval` parameter before the SDK can wire it. The custom Redis SetNX leader election (`leader:twitch-eventsub`) is entirely separate from the coordinator heartbeat/assignment concern and intentionally coexists — both remain.

**Primary recommendation:** Plan 38-01 (youtube-listener) first to build confidence; plan 38-02 (eventsub ChannelManager gap fixes) second; plan 38-03 (eventsub ListenerBase wiring + smoke test) third.

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `shared/listener` | local replace | LeadershipListener, ListenerBase, ChannelManager interface, Env(), DefaultConfig() | Project SDK — the target of this entire migration |
| `shared/listener/testutil` | local replace | MockCoordinator for goleak smoke tests | Established in phases 35-37; avoids real HTTP in unit tests |
| `go.uber.org/goleak` | v1.3.0 | Goroutine-leak detection in smoke tests | Pinned as direct dep in all completed listener migrations |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `shared/sourcemanager` | local replace | `*LeadershipCoordinator` passed to `streams.NewManager` | Returned by `ll.LeadershipCoordinator()` — type unchanged from pre-migration |
| `shared/coordination` | local replace | `CoordinatorClient`, `MigrationEvent`, `NewCoordinatorClient` | Used in eventsub for SDK base construction via `NewListenerBaseFromEnv` |

**Installation (goleak — both services):**
```bash
cd services/youtube-listener
go mod edit -require go.uber.org/goleak@v1.3.0
go mod download go.uber.org/goleak@v1.3.0

cd services/twitch-eventsub-listener
go mod edit -require go.uber.org/goleak@v1.3.0
go mod download go.uber.org/goleak@v1.3.0
```

---

## Architecture Patterns

### Recommended File Layout (changes only)

```
services/youtube-listener/
├── cmd/main.go                  # Replace manual coordinator block; delete getEnvOrDefault
└── cmd/main_sdk_test.go         # NEW: goleak smoke test (LeadershipListener only)

services/twitch-eventsub-listener/
├── channels/manager.go          # Gap fixes: Start re-sign, 3 new methods, compile assertion
└── cmd/main.go                  # Replace coordinator/jitter/assignments/heartbeat/migration blocks
    cmd/main_sdk_test.go         # NEW: goleak smoke test (base.Start + real/stub ChannelManager)
```

### Pattern 1: LeadershipListener — leadership-only wiring (youtube-listener)

**What:** Replace manual `sourcemanager.*` construction block with `NewLeadershipListenerFromEnv`; extract coordinator via accessor and pass to domain package unchanged.

**When to use:** Service uses `*sourcemanager.LeadershipCoordinator` for ownership-based stream activation, not for channel assignment filtering. `base.Start` is NOT called.

**Example (adapted from Phase 37 youtube-innertube):**
```go
// Source: services/youtube-listener-innertube/cmd/main.go (Phase 37)

// Build base (config only — no coordinator client needed for leadership-only)
cfg := listener.DefaultConfig()
base := listener.NewListenerBase(cfg, &noopCoordinatorClient{}, redisClient, podID, log)
// NOTE: for youtube-listener, the base is passed to NewLeadershipListenerFromEnv differently
// because there IS a real coordinator client — see NewListenerBaseFromEnv pattern below.

ll, err := listener.NewLeadershipListenerFromEnv(base, "youtube", log)
if err != nil {
    log.Fatal("Failed to initialize LeadershipListener", zap.Error(err))
}

// Pass coordinator to domain package — type is unchanged from pre-migration
streamManager := streams.NewManager(..., ll.LeadershipCoordinator(), ...)

// Do NOT call ll.Start(ctx, nil) — leadership-only service
```

**Key insight for youtube-listener:** The pre-migration `leaderCoord` variable is assigned from `sourcemanager.NewLeadershipCoordinator(...)`. After migration it becomes `ll.LeadershipCoordinator()`. The type (`*sourcemanager.LeadershipCoordinator`) and every downstream call site in `streams/manager.go` are unchanged.

### Pattern 2: ListenerBase wiring — assignment-based coordinator (twitch-eventsub)

**What:** Replace manual coordinator construction + jitter + assignments block + heartbeat goroutine + assignment-refresh goroutine + migration subscriber goroutine with `NewListenerBaseFromEnv` + `base.Start(ctx, channelManager)`.

**When to use:** Service is not leadership-only — it uses the coordinator for assignment filtering and health signals. The custom Redis SetNX leader election is a separate concern and is NOT replaced.

**Example (adapted from Phase 35 twitch-listener):**
```go
// Source: services/twitch-listener/cmd/main.go (Phase 35)
cfg := listener.DefaultConfig()
base, err := listener.NewListenerBaseFromEnv(cfg, redisClient, podName, log)
if err != nil {
    log.Fatal("Failed to initialize ListenerBase", zap.Error(err))
}

// channelManager already satisfies listener.ChannelManager after gap fixes
if err := base.Start(ctx, channelManager); err != nil {
    log.Fatal("Failed to start ListenerBase", zap.Error(err))
}
defer base.Stop()
```

### Pattern 3: ChannelManager interface gap fixes

**What:** Bring `channels.Manager` into compliance with `listener.ChannelManager` before SDK wiring.

**Current state of `channels.Manager` vs SDK interface:**

| Method | SDK interface | Current eventsub Manager | Gap |
|--------|--------------|--------------------------|-----|
| `Start(ctx context.Context) error` | required | `Start(ctx, interval time.Duration)` — wrong signature, no error return | Fix signature; store interval as field |
| `Stop()` | required | present | none |
| `HandleMigrationEvent(*coordination.MigrationEvent) error` | required | present | none |
| `UpdateAssignedSourceIDs(map[string]bool)` | required | present | none |
| `GetFilteredAssignmentCount() int` | required | missing | Add: return `len(assignedSourceIDs)` |
| `GetActiveChannelCount() int` | required | missing | Add: return `len(m.channels)` |
| `GetActiveChannels() []string` | required (`[]string`) | returns `map[string]*Channel` | Change return type; rename old method if needed |

**Constructor change:**
```go
// Before
func NewManager(db, logger, resolver, cipher) *Manager

// After
func NewManager(db, logger, resolver, cipher, syncInterval time.Duration) *Manager
// syncInterval stored as m.syncInterval; Start(ctx) uses m.syncInterval instead of parameter
```

**Compile-time assertion (add to channels/manager.go):**
```go
var _ listener.ChannelManager = (*Manager)(nil)
```

### Pattern 4: goleak smoke test (both services)

**What:** Minimal `cmd/main_sdk_test.go` that constructs the SDK object, calls Start, cancels context, calls Stop, and asserts no leaked goroutines.

**youtube-listener variant (LeadershipListener — no ChannelManager):**
```go
// Source: services/discord-listener/cmd/main_sdk_test.go (Phase 37 pattern)
func TestListenerBase_StartStop_NoGoroutineLeak(t *testing.T) {
    defer goleak.VerifyNone(t)
    mock := &testutil.MockCoordinator{}
    cfg := listener.ListenerConfig{
        HeartbeatInterval:         20 * time.Millisecond,
        AssignmentRefreshInterval: 20 * time.Millisecond,
        StartupJitterMax:          0,
    }
    base := listener.NewListenerBase(cfg, mock, nil, "test-pod", nil)
    // youtube-listener: do NOT call base.Start — leadership-only, no ChannelManager
    _ = base // LeadershipListener wraps base; test just constructs and GCs it
}
```

**NOTE:** For youtube-listener the smoke test validates that `NewLeadershipListenerFromEnv` construction itself does not leak. Because `base.Start` is never called in production (leadership-only), the test does not call it either. If the planner wants to match the exact discord/innertube pattern, the test constructs `ListenerBase` + calls `base.Start(ctx, mockMgr)` + `base.Stop()` to exercise the goroutine lifecycle even though production main doesn't call Start. CONTEXT.md says "No quota tracker or stream manager in test" — use `mockChannelManagerForTest` inline stub if Start is exercised.

**twitch-eventsub variant (full base.Start with ChannelManager):**
```go
// Pattern matches Phase 35 twitch-listener
func TestListenerBase_StartStop_NoGoroutineLeak(t *testing.T) {
    defer goleak.VerifyNone(t)
    mock := &testutil.MockCoordinator{}
    cfg := listener.ListenerConfig{
        HeartbeatInterval:         20 * time.Millisecond,
        AssignmentRefreshInterval: 20 * time.Millisecond,
        StartupJitterMax:          0,
    }
    base := listener.NewListenerBase(cfg, mock, nil, "test-pod", nil)
    mgr := &mockChannelManagerForTest{} // inline stub
    ctx, cancel := context.WithCancel(context.Background())
    if err := base.Start(ctx, mgr); err != nil {
        t.Fatal(err)
    }
    cancel()
    base.Stop()
}
```

### Anti-Patterns to Avoid

- **Calling `base.Start(ctx, nil)` for leadership-only services:** `Start` calls `nil.UpdateAssignedSourceIDs` → panic. Leadership-only services must NOT call `Start`. (Established Phase 37 pattern.)
- **Passing `syncInterval` to `channelManager.Start(ctx)` after gap fix:** After the fix, `Start` takes only `ctx`. The interval is stored in the constructor.
- **Deleting `parseIntEnv` in youtube-listener:** Unlike `getEnvOrDefault`, `parseIntEnv` wraps `listener.Env` + `strconv.Atoi` — it is not a drop-in for `listener.Env`. Keep it.
- **Replacing the Redis SetNX leader election in eventsub:** That concern controls EventSub subscription ownership on Twitch's API side. It is separate from the coordinator concern. Both remain.
- **Calling `coordClient.StartJWTRefresh` / `StopJWTRefresh` manually after SDK wiring:** `base.Start` owns `StartJWTRefresh`; `base.Stop` owns `StopJWTRefresh`. Duplicate calls must be deleted from eventsub `cmd/main.go`.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| JWT refresh goroutine | custom `StartJWTRefresh` / `StopJWTRefresh` calls in main | `base.Start` / `base.Stop` | SDK owns this lifecycle; duplicating causes double-start |
| Startup jitter | `time.Sleep(rand.Intn(30))` in main | `ListenerConfig.StartupJitterMax` | SDK handles it; env var `LISTENER_STARTUP_JITTER_MAX=0` disables in tests |
| Assignment refresh goroutine | ticker + `QueryAssignments` loop in main | `base.Start` background loop | SDK handles refresh + backoff; manual loop is dead code after migration |
| Heartbeat goroutine | ticker + `PublishHeartbeat` loop in main | `base.Start` background loop | Same as above |
| Migration subscriber goroutine | `coordination.NewMigrationSubscriber` + goroutine in main | `base.Start` background loop | SDK auto-reconnects; manual subscriber has no retry |
| Coordinator client construction | `coordination.NewCoordinatorClient(...)` in main | `NewListenerBaseFromEnv` | SDK reads env vars; no duplicate wiring needed |

**Key insight:** For twitch-eventsub, lines 563–608 (coordinator construction + jitter + assignments) and lines 840–910 (migration subscriber + heartbeat + assignment refresh goroutines) in `cmd/main.go` are all dead code after SDK adoption. They must be deleted, not left as commented-out blocks.

---

## Common Pitfalls

### Pitfall 1: `parseIntEnv` incorrectly deleted in youtube-listener
**What goes wrong:** If `parseIntEnv` is deleted along with `getEnvOrDefault`, callers like `parseIntEnv("QUOTA_LIMIT_DAILY", 1009000)` break at compile time.
**Why it happens:** Both look like env helpers; easy to sweep both out.
**How to avoid:** Keep `parseIntEnv` — it is called 4 times in `cmd/main.go` with integer parsing logic that `listener.Env` does not provide.
**Warning signs:** Compile error: `undefined: parseIntEnv`.

### Pitfall 2: `channels.Manager.Start` still accepts `interval time.Duration` after gap fix
**What goes wrong:** The SDK calls `mgr.Start(ctx)` with one argument. If the old two-argument signature persists, compilation fails.
**Why it happens:** The gap fix changes the method signature — callers in `cmd/main.go` (the leader election goroutine) must also be updated from `channelManager.Start(ctx, ChannelSyncInterval)` to `channelManager.Start(ctx)`.
**How to avoid:** Update both the method definition AND all callers simultaneously.
**Warning signs:** `too many arguments in call to channelManager.Start`.

### Pitfall 3: Duplicate goroutines after partial SDK wiring in eventsub
**What goes wrong:** If the heartbeat or assignment-refresh goroutines are left in `cmd/main.go` alongside `base.Start`, the service runs double instances of each loop.
**Why it happens:** Incremental migration leaves existing code in place.
**How to avoid:** Plan 38-03 must explicitly delete the three goroutine blocks (lines 455–509 in current `cmd/main.go`) as part of the wiring step.
**Warning signs:** goleak test shows extra goroutines; duplicate heartbeat log lines in production.

### Pitfall 4: goleak fails because `base.Start` is called in test for youtube-listener
**What goes wrong:** If the smoke test calls `base.Start(ctx, mgr)` without a nil-safe ChannelManager and then does not call `base.Stop()`, goroutines leak.
**Why it happens:** Forgetting `base.Stop()` or cancelling context without draining `wg`.
**How to avoid:** Always: `ctx, cancel := context.WithCancel(...); base.Start(ctx, mgr); cancel(); base.Stop()`.
**Warning signs:** `goleak.VerifyNone` reports leaked goroutines.

### Pitfall 5: `GetActiveChannels()` callers not updated after return-type change
**What goes wrong:** If `cmd/main.go` or HTTP handlers call `channelManager.GetActiveChannels()` and expect `map[string]*Channel`, they break.
**Why it happens:** The return type change from map to `[]string` is a breaking change for all callers.
**How to avoid:** Grep all callers of `GetActiveChannels()` in the eventsub service before renaming. If the HTTP `/status` handler uses it, update accordingly. If needed, add `GetActiveChannelMap()` for internal use.

---

## Code Examples

### youtube-listener: SDK wiring in cmd/main.go

```go
// Source: analogous to services/youtube-listener-innertube/cmd/main.go (Phase 37 pattern)

import "github.com/caesar/all-chat/shared/listener"

// Replace this block (lines ~208-220 in current main.go):
//   tokenSource := sourcemanager.NewSigningTokenSource(...)
//   smClient, err := sourcemanager.NewClient(...)
//   leaderCoord = sourcemanager.NewLeadershipCoordinator(...)
// With:

podName := listener.Env("HOSTNAME", "youtube-listener-unknown")
cfg := listener.DefaultConfig()
base := listener.NewListenerBase(cfg, /* coordinator client */ nil, redisClient, podName, log)
// NOTE: for youtube-listener we need a real coordinator client in the base when SOURCE_MANAGER_SECRET
// is set. Use NewListenerBaseFromEnv instead:
ll, err := listener.NewLeadershipListenerFromEnv(base, "youtube", log)
if err != nil {
    log.Fatal("Failed to initialize LeadershipListener", zap.Error(err))
}

// streams.NewManager receives the same type as before:
streamManager := streams.NewManager(
    streamRepo, oauthManager, messageHandler, dbConnWrapper,
    ll.LeadershipCoordinator(), // replaces leaderCoord
    quotaTracker, perChannelQuotaTracker, redisClient, ytMetrics, statusPublisher, log,
)
```

### eventsub: ChannelManager gap fix — Start re-sign

```go
// Source: services/twitch-eventsub-listener/channels/manager.go

// Add field to Manager struct:
syncInterval time.Duration

// Update NewManager:
func NewManager(db *pgxpool.Pool, logger *zap.Logger, resolver UserIDResolver,
    cipher *encryption.AESEncryptor, syncInterval time.Duration) *Manager {
    return &Manager{
        // ... existing fields ...
        syncInterval: syncInterval,
        stopChan:     make(chan struct{}),
    }
}

// Update Start signature:
func (m *Manager) Start(ctx context.Context) error {
    m.logger.Info("Starting channel manager", zap.Duration("sync_interval", m.syncInterval))
    if err := m.SyncChannels(ctx); err != nil {
        m.logger.Error("Initial channel sync failed", zap.Error(err))
    }
    m.wg.Add(1)
    go func() {
        defer m.wg.Done()
        ticker := time.NewTicker(m.syncInterval)
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done():
                return
            case <-m.stopChan:
                return
            case <-ticker.C:
                if err := m.SyncChannels(ctx); err != nil {
                    m.logger.Error("Channel sync failed", zap.Error(err))
                }
            }
        }
    }()
    return nil
}
```

### eventsub: New methods

```go
// GetFilteredAssignmentCount returns the number of assigned source IDs.
func (m *Manager) GetFilteredAssignmentCount() int {
    m.assignmentMu.RLock()
    defer m.assignmentMu.RUnlock()
    return len(m.assignedSourceIDs)
}

// GetActiveChannelCount returns the number of channels with active EventSub subscriptions.
func (m *Manager) GetActiveChannelCount() int {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return len(m.channels)
}

// GetActiveChannels returns broadcaster IDs with active EventSub subscriptions.
func (m *Manager) GetActiveChannels() []string {
    m.mu.RLock()
    defer m.mu.RUnlock()
    ids := make([]string, 0, len(m.channels))
    for id := range m.channels {
        ids = append(ids, id)
    }
    return ids
}

// GetActiveChannelMap returns the full channel map (internal use or renamed callers).
func (m *Manager) GetActiveChannelMap() map[string]*Channel {
    m.mu.RLock()
    defer m.mu.RUnlock()
    channels := make(map[string]*Channel, len(m.channels))
    for k, v := range m.channels {
        channels[k] = v
    }
    return channels
}

// Compile-time assertion
var _ listener.ChannelManager = (*Manager)(nil)
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Manual `sourcemanager.*` construction in main | `NewLeadershipListenerFromEnv` | Phase 37 (youtube-innertube, discord) | 15-line block → 3 lines |
| Manual heartbeat + assignment-refresh + migration goroutines | `base.Start(ctx, mgr)` | Phase 35 (twitch), Phase 36 (kick) | 70+ lines → 1 call |
| `getEnvOrDefault` / `getEnv` local helpers | `listener.Env(key, default)` | Phase 35 onward | Consistent; one SDK function |

**Deprecated (will be removed in this phase):**
- `getEnv` in twitch-eventsub `cmd/main.go` — replaced by `listener.Env`
- `getEnvOrDefault` in youtube-listener `cmd/main.go` — replaced by `listener.Env`
- `coordClient.StartJWTRefresh(ctx)` / `defer coordClient.StopJWTRefresh()` in eventsub `cmd/main.go` — owned by SDK
- Three manual goroutine blocks in eventsub `cmd/main.go` (lines 455–509 approx.) — replaced by `base.Start`
- Manual `sourcemanager.*` construction block in youtube-listener `cmd/main.go` (lines 208–220 approx.) — replaced by `NewLeadershipListenerFromEnv`

---

## Open Questions

1. **Does any eventsub internal code call `GetActiveChannels()` expecting a map?**
   - What we know: `GetActiveChannels` returning `map[string]*Channel` has no callers in `cmd/main.go` (grep confirmed zero hits in cmd/).
   - What's unclear: Whether any HTTP handler or test file in the eventsub service calls it.
   - Recommendation: Grep `GetActiveChannels` across all eventsub files before the gap-fix plan executes. If there are internal callers expecting the map, keep `GetActiveChannelMap()` alongside the new `[]string` method. If none, `GetActiveChannelMap()` can be omitted entirely. (This is explicitly Claude's discretion per CONTEXT.md.)

2. **Does twitch-eventsub smoke test use real `channels.NewManager` or inline stub?**
   - What we know: CONTEXT.md says "Uses real `channels.NewManager` (or minimal stub) with mock coordinator." The discord/innertube patterns use an inline `mockChannelManagerForTest` stub that avoids DB/Redis.
   - What's unclear: Whether using real `channels.NewManager` is achievable without a DB (it requires `*pgxpool.Pool`).
   - Recommendation: Use the inline stub pattern — consistent with all completed migrations (phases 35, 36, 37). The compile-time assertion in `channels/manager.go` already validates the interface; the smoke test does not need to exercise the real manager.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` + `goleak` v1.3.0 |
| Config file | none — standard `go test ./cmd/...` |
| Quick run command | `go test ./cmd/... -v` (in each service dir) |
| Full suite command | `go test ./... -v` (in each service dir) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| MIGRATE-03 | LeadershipListener construction + no goroutine leak | unit / smoke | `go test ./cmd/... -run TestListenerBase_StartStop_NoGoroutineLeak` | ❌ Wave 0 |
| MIGRATE-06 | ListenerBase Start+Stop with ChannelManager + no goroutine leak | unit / smoke | `go test ./cmd/... -run TestListenerBase_StartStop_NoGoroutineLeak` | ❌ Wave 0 |
| MIGRATE-06 | ChannelManager interface compliance | compile-time | `go build ./...` | ❌ Wave 0 (assertion added in plan 38-02) |

### Sampling Rate
- **Per task commit:** `go build ./...` in the changed service directory
- **Per wave merge:** `go test ./... -v` in the changed service directory
- **Phase gate:** `make build-all` from repo root — verifies no replace-directive drift across all services

### Wave 0 Gaps
- [ ] `services/youtube-listener/cmd/main_sdk_test.go` — covers MIGRATE-03 smoke test
- [ ] `services/twitch-eventsub-listener/cmd/main_sdk_test.go` — covers MIGRATE-06 smoke test
- [ ] `go.uber.org/goleak v1.3.0` as direct dep in `services/youtube-listener/go.mod`
- [ ] `go.uber.org/goleak v1.3.0` as direct dep in `services/twitch-eventsub-listener/go.mod`

---

## Sources

### Primary (HIGH confidence)
- `shared/listener/base.go` — ListenerBase.Start signature, goroutine lifecycle, coordinatorClient interface
- `shared/listener/leadership.go` — NewLeadershipListenerFromEnv, LeadershipCoordinator() accessor
- `shared/listener/channel_manager.go` — exact 7-method ChannelManager interface
- `shared/listener/config.go` — DefaultConfig(), Env() helper
- `shared/listener/testutil/mock_coordinator.go` — MockCoordinator fields and method signatures
- `services/youtube-listener-innertube/cmd/main.go` — Phase 37 reference implementation (LeadershipListener pattern)
- `services/youtube-listener-innertube/cmd/main_sdk_test.go` — exact goleak smoke test pattern
- `services/discord-listener/cmd/main_sdk_test.go` — identical smoke test pattern confirmation
- `services/youtube-listener/cmd/main.go` — exact lines to be replaced (lines 208–220, 348–366)
- `services/twitch-eventsub-listener/cmd/main.go` — exact lines to be replaced (approx 157–509)
- `services/twitch-eventsub-listener/channels/manager.go` — current interface state; all gap items confirmed

### Secondary (MEDIUM confidence)
- `.planning/STATE.md` accumulated decisions — Phase 35/36/37 migration patterns
- `.planning/phases/38-migrate-youtube-listener-and-twitch-eventsub-listener/38-CONTEXT.md` — all locked decisions

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries are local replace-directive deps already in go.mod
- Architecture: HIGH — all patterns verified by reading actual completed migration files
- Pitfalls: HIGH — derived from reading actual code; confirmed by interface diff

**Research date:** 2026-03-18
**Valid until:** Indefinite — all sources are local codebase files, not external APIs
