# Domain Pitfalls

**Domain:** Extracting a shared Listener SDK from working Go microservices (v1.6)
**Researched:** 2026-03-17
**Confidence:** HIGH

---

## Context: What Makes This Migration Risky

Six listeners exist and work correctly in production: twitch-listener, kick-listener, youtube-listener, youtube-listener-innertube, tiktok-listener (Node.js — separate runtime), and discord-listener. The v1.6 goal is to extract `ListenerBase` and `ChannelManager` from the Go listeners into `/shared/listener` so future listeners are trivial to build.

The trap is not "will the SDK be correct" — it is "will migrating working listeners to the SDK break them while everything else runs." Each listener is independently deployed, independently tested, and carries live production traffic. A bug introduced during migration is a production incident, not a failing unit test.

The system already uses a multi-module monorepo with `replace` directives (no `go.work` file). The `shared` module is the single shared dependency. The new `listener` SDK will live inside it or alongside it. Both options have distinct pitfalls documented below.

---

## Critical Pitfalls

Mistakes that cause production incidents, data loss, or rewrites.

---

### Pitfall 1: Big-Bang Migration Breaks All Listeners Simultaneously

**What goes wrong:** All six listeners are migrated to the SDK in a single pull request or a single phase. The SDK has a subtle bug — wrong startup order, incorrect context propagation, a nil pointer on the optional `leaderCoord`. Six listeners break simultaneously. Redis Streams fill because no consumer is reading them. The message-processor falls behind. Overlays go dark.

**Why it happens:** It feels efficient to migrate everything at once once the SDK is "done." But the SDK has never run in production. It has no track record. The complexity surface is the product of (SDK bugs) × (migration bugs) × (6 listeners).

**Consequences:** Full listener fleet outage. No incremental rollback point. The only recovery is reverting the SDK entirely, which means reverting all six migrations in parallel.

**Prevention:**
- Migrate one listener first. Ship it to production. Let it run for 24 hours.
- Use the listener with the simplest startup sequence as the first migration target: twitch-listener has no leadership coordinator (confirmed: `var leaderCoord *sourcemanager.LeadershipCoordinator = nil`), making it the safest first subject.
- Only migrate the second listener after the first is verified stable in production.
- Keep the old `cmd/main.go` logic reachable via a feature flag or a parallel branch for at least one deployment cycle. "I can revert one listener in 5 minutes" is a different risk level than "I need to roll back the SDK."

**Detection:** Per-listener `messages_published_total` metric must not drop after migration. Alert on any drop > 10% for 5 minutes after a deploy. A silent zero is the worst failure mode.

**Phase:** This is the governing constraint for the entire migration roadmap. Single-listener-at-a-time is non-negotiable.

---

### Pitfall 2: ListenerBase Startup Sequence Hardcodes Order That Not All Listeners Share

**What goes wrong:** The SDK's `ListenerBase.Start()` wires this sequence (from the existing twitch-listener main.go): jitter → query assignments → start migration subscriber → start heartbeat → start assignment refresh loop. Kick-listener's main.go adds `leaderCoord` initialization and a reconnection goroutine between "start channel manager" and "start migration subscriber." Discord-listener uses a Gateway connection lock. YouTube-innertube uses stream discovery before any assignment query.

If `ListenerBase` hardcodes a single startup sequence, listeners that need a different order must either subvert the SDK (calling internal fields directly) or duplicate the sequence outside the SDK. Both are leaky abstractions.

**Why it happens:** Looking at two listeners (twitch, kick), their main.go files look 80% identical. The temptation is to extract that 80% and call it the canonical sequence. The 20% divergence is where the production bugs live.

**Consequences:** The SDK becomes a straightjacket. Listeners that don't fit the sequence grow workaround code in `cmd/main.go` that is harder to read than the original duplication. Future listeners are written with SDK + workarounds instead of clean SDK usage.

**Prevention:**
- Before writing a single line of SDK code, diff all six Go listener `cmd/main.go` files step by step. Document every divergence point.
- Design `ListenerBase` as a struct-of-hooks, not a call sequence. The caller provides hooks that the base invokes at the right points. Example: `OnAfterConnect func(ctx context.Context) error`, `OnChannelAdd func(channelID string) error`. The base provides the sequence skeleton; the hooks provide the variations.
- The `LeadershipListener` variant is the right architectural split already identified in PROJECT.md: base has no leader logic, `LeadershipListener` embeds base and adds leader hooks. Do not attempt a single struct that optionally does leadership via a nil-checked pointer.
- If a listener requires a step that does not fit any hook, that is a signal the abstraction is wrong — not a reason to expose internals.

**Detection:** Code review gate: no listener's `cmd/main.go` may reach into `ListenerBase` exported fields after `Start()` is called. If a listener needs to do that, the SDK is missing a hook.

**Phase:** Design-time concern. Must be resolved before SDK code is written, not after the first migration fails.

---

### Pitfall 3: ChannelManager Extraction Breaks Kick-Specific Chatroom ID Logic

**What goes wrong:** The Twitch ChannelManager key is the channel name (a string, e.g., `"shroud"`). The Kick ChannelManager has two keys: `channel_slug` (string) and `chatroom_id` (integer) — because Kick's Pusher events use chatroom ID, not slug. The `firstMessageChan` map is `map[string]chan struct{}` in twitch and `map[int]chan struct{}` in kick (confirmed from source).

If the shared `ChannelManager` uses `string` as the only key type, kick-listener must maintain a parallel `chatroomIndex` outside the shared manager to do integer lookups. The migration "succeeds" at compile time but the manager is actually two managers: one shared, one private.

**Why it happens:** Go generics are available but the existing codebase does not use them. The natural reflex is to use `string` as the universal key (it works for Twitch) and add a lookup table in kick-listener to bridge integer → string. This is the abstraction leak.

**Consequences:** `SignalFirstMessage` in kick-listener takes an int. The shared manager's `SignalFirstMessage` takes a string. The caller must do a string conversion that is fragile — a chatroom ID 12345 is not the same key space as a channel slug "shroud". If a channel has both a slug and a chatroom ID in the manager's keys, they are different entries. Assignment filtering breaks.

**Prevention:**
- Use a generic `ChannelManager[K comparable]` with `K = string` for Twitch/YouTube/Discord and `K = int` for Kick. This is the correct Go generics use case.
- Alternatively, define the manager as string-keyed and require Kick to always use `strconv.Itoa(chatroomID)` as the key. Document this convention explicitly. This is simpler but slightly more brittle.
- Do not accept the approach of "shared manager for common fields, kick adds its own map." That is duplication in disguise.

**Detection:** Unit test the shared `ChannelManager` with both `int` and `string` key types before any listener migration. If the test requires awkward conversions, the interface is wrong.

**Phase:** Phase 1 of SDK development. The key type question must be answered before any listener migration begins.

---

### Pitfall 4: Circular Dependency Between `/shared/listener` and Other Shared Packages

**What goes wrong:** The new `shared/listener` package depends on `shared/coordination`, `shared/sourcemanager`, `shared/metrics`, and `shared/tracing`. One of these packages — likely `shared/coordination` or `shared/metrics` — later needs a type from `shared/listener` (for example, a `ListenerType` enum used in metrics labels). Go does not allow circular imports. The build breaks. Resolving this mid-migration requires either introducing a third package or restructuring both packages.

**Why it happens:** `shared` is currently one Go module (`github.com/caesar/all-chat/shared`). All subpackages within it can import each other freely at first. But `shared/metrics` already defines `NewListenerMetrics("twitch", ...)` — it is listener-aware. If `shared/listener` needs to call `shared/metrics.NewListenerMetrics`, and `shared/metrics` later needs `shared/listener.ListenerType`, you have a cycle.

**Consequences:** Compile error blocking all builds. Resolution requires a non-trivial refactor during an active migration.

**Prevention:**
- Define a strict import hierarchy at the start: `shared/listener` may import `shared/{coordination,sourcemanager,metrics,tracing,logger}` but none of those packages may import `shared/listener`. Write this as a comment in `shared/listener/doc.go` on day one.
- Any type that both `shared/listener` and `shared/metrics` need (e.g., platform name constants) must live in a third package with no upstream dependencies: `shared/types` or `shared/listenertype`. Neither `shared/listener` nor `shared/metrics` owns it.
- Run `go build ./...` from the `shared` module root after every new file addition during SDK development. Catch cycles immediately rather than at integration.

**Detection:** `go build ./...` in CI for the shared module. Any import cycle is a build failure.

**Phase:** Structural concern for Phase 1 of SDK development. Diagram the import graph before writing the first SDK file.

---

### Pitfall 5: `replace` Directive Version Skew During Migration

**What goes wrong:** Each listener module uses `replace github.com/caesar/all-chat/shared => ../../shared`. This means all listeners always use the local `shared` code. The problem arises when the new `shared/listener` package is added to `shared/go.mod` but some listeners have stale `go.sum` entries or their `go.mod` does not yet reference the new package. A listener that was not yet migrated compiles against the new shared module with new exported types. If the new types introduce backward-incompatible changes to existing shared packages (e.g., `shared/coordination.CoordinatorClient` gains a required parameter), unmigrated listeners break at compile time even though they have not touched the SDK.

**Why it happens:** The `replace` directive makes all listeners always reflect the latest `shared` state. This is convenient for development but means any breaking change to `shared` immediately breaks all consumers.

**Consequences:** Attempting to migrate listener A while listeners B-F are unmigrated fails if the SDK changes any existing shared interface. The migration must be done atomically (violating Pitfall 1) or the shared interfaces must be strictly backward-compatible during the migration window.

**Prevention:**
- Strict rule: during the v1.6 migration window, **no existing shared package API may be changed**. The SDK adds new packages and new types only. Existing `shared/coordination`, `shared/sourcemanager`, `shared/metrics` public interfaces are frozen.
- New SDK code lives in `shared/listener` — a completely new package. Existing packages are not modified.
- If a change to an existing shared package is truly required, it must be done in a separate commit before migration begins, must be backward-compatible (add new function alongside old), and all listeners must still compile against it without changes.
- After each listener migration, run `go build ./...` for all listener modules in CI to verify no unintended breakage.

**Detection:** CI must build every listener module, not just the one being migrated. A "migrated listener CI" that only checks the migrated service is insufficient.

**Phase:** Applies to every phase of migration. The "freeze existing shared APIs" rule is in effect from the start of SDK development until all migrations are complete.

---

### Pitfall 6: SDK Changes After Partial Migration Require Coordinated Deploys

**What goes wrong:** Listener A is migrated and deployed. Listener B is being migrated. A bug is found in `ListenerBase.Start()` — the heartbeat goroutine leaks when `ctx` is cancelled before the goroutine reads from it. The fix changes `ListenerBase`'s exported interface (adds a `context.Context` parameter to `Start()`). Listener A must be redeployed to pick up the fix. Listener B's in-progress migration must be rebased. All four unmigrated listeners are not affected because they do not use the SDK — but when they are migrated later, they will immediately get the fixed interface.

This is manageable. What is not manageable: a fix that changes the behavior (not the interface) of `ListenerBase.Start()` — for example, changes the startup jitter timing. Listener A picks up the fix on next deploy. Listeners B-F run the old behavior until migrated. The fleet now has mixed startup behavior, which can cause thundering herd from the subset running old code if the fix was changing jitter semantics.

**Why it happens:** Partial migration means the fleet is in a mixed state. SDK behavior changes affect deployed listeners immediately (via next deploy) but unmigrated listeners are immune.

**Consequences:** Operational complexity during the migration window. Difficult to reason about fleet behavior when some listeners use the SDK and some do not.

**Prevention:**
- Minimize the SDK's stable surface area. The SDK should have zero behavior changes after it is first deployed by listener A. If a bug requires a behavioral change, evaluate whether to roll back listener A's migration temporarily rather than operating a mixed fleet with different timing behavior.
- Document explicitly: "once ListenerBase.Start() is deployed, its timing semantics (jitter duration, heartbeat interval, assignment refresh interval) are frozen for the duration of the migration." These values should be injected via a `ListenerConfig` struct, not hardcoded, so per-listener overrides are possible without changing the SDK.
- Keep the migration window short. The longer the fleet is in mixed state, the higher the operational risk.

**Detection:** Grafana dashboard showing deployment timestamps per listener. Alert if any listener has been using old (non-SDK) code for more than 2 weeks after SDK first deployment.

**Phase:** Operational constraint for the migration phases. Addressed in rollout strategy, not SDK design.

---

## Moderate Pitfalls

---

### Pitfall 7: Embed vs. Interface — Choosing the Wrong Go Pattern for ListenerBase

**What goes wrong:** `ListenerBase` is implemented as a struct that listeners embed: `type TwitchListener struct { listener.ListenerBase; ... }`. The embedded base has a `Start(ctx context.Context) error` method. The twitch-listener's `Start` needs to do extra steps after `ListenerBase.Start()`. It does: `func (t *TwitchListener) Start(ctx context.Context) error { t.ListenerBase.Start(ctx); t.doTwitchStuff() }`. This works.

Now consider: a test creates a `TwitchListener` and calls `listener.StartAll(ctx, []listener.Startable{tl, kl})` where `Startable` is an interface with `Start(ctx) error`. The embedded `ListenerBase.Start` is promoted — unless `TwitchListener` defines its own `Start`, in which case the embedded method is shadowed. If `TwitchListener.Start` calls `t.ListenerBase.Start(ctx)` and `ListenerBase.Start` launches goroutines that hold a reference to `ListenerBase` internals, and `TwitchListener` is garbage collected while those goroutines run — the embed creates subtle lifetime dependencies.

More practically: if `ListenerBase` defines a `Stop()` method via embed and the listener also defines `Stop()`, Go's method promotion rules mean the listener's `Stop()` shadows the base's `Stop()`. If the test calls `Stop()` on the `listener.Startable` interface, which `Stop()` runs? The listener's, which may forget to call `t.ListenerBase.Stop()`. Silent goroutine leak.

**Prevention:**
- Prefer composition over embedding for `ListenerBase`. The listener holds a `base *listener.Base` field (unexported) and explicitly calls `base.Start(ctx)` and `base.Stop()` in its own lifecycle methods. No method promotion ambiguity.
- The public interface for "anything that can be started/stopped" is an interface, not a base struct: `type Listener interface { Start(ctx context.Context) error; Stop() }`. Implementations satisfy it explicitly.
- If embedding is used, document exactly which methods are promoted and which are shadowed, with a compile-time check: `var _ listener.Listener = (*TwitchListener)(nil)`.

**Detection:** `go vet ./...` catches some method shadowing issues but not all. Static analysis (e.g., `staticcheck`) catches promoted method shadowing in more cases.

**Phase:** SDK design phase. Decide embed vs. composition before writing any listener implementation.

---

### Pitfall 8: Tests That Depend on `cmd/main.go` Logic Cannot Test the SDK

**What goes wrong:** The existing channel manager tests (e.g., `twitch-listener/channels/manager_test.go`) mock `JoinParterInterface` and test the manager in isolation. They do not test the startup sequence in `cmd/main.go` — the jitter, assignment query, migration subscriber launch, heartbeat goroutine, assignment refresh loop. After migration to the SDK, this startup sequence lives in `ListenerBase.Start()`. If `ListenerBase` has a bug in how it wires these goroutines, the existing tests will not catch it. The tests still pass. The bug only manifests in a deployed pod.

**Why it happens:** The startup sequence in `cmd/main.go` was previously untestable imperative code. It was acceptable when it was ~150 lines of straightforward Go. When it becomes `ListenerBase.Start()`, it is shared code that must be tested.

**Consequences:** SDK bugs in goroutine wiring, context propagation, or shutdown ordering are invisible to the test suite. They surface in production as subtle issues: goroutine leaks detected by `go/pprof`, double-heartbeat under load, assignment refresh running after shutdown.

**Prevention:**
- Write unit tests for `ListenerBase.Start()` before any listener migration. Mock all external dependencies: coordinator client (returns assignments), migration subscriber (no-op), heartbeat publisher (counter). Verify: goroutines start, goroutines stop when `ctx` is cancelled, no goroutine leaks (use `goleak` in tests).
- Write a test that calls `ListenerBase.Start()` and then cancels the context before the startup jitter completes. Verify the method returns promptly and no goroutines are left running.
- The SDK tests must achieve higher coverage than the original `cmd/main.go` code, because the SDK serves more consumers.
- After migrating a listener, run the listener's existing channel manager tests with the SDK-backed implementation. If any test needs to be modified to accommodate the SDK, that is a signal the SDK changed behavior.

**Detection:** `goleak.VerifyNone(t)` in every SDK test. If goroutine count after test teardown is non-zero, the test fails.

**Phase:** Phase 1 of SDK development. SDK tests must exist before the first listener migration begins.

---

### Pitfall 9: Assignment Refresh Strips Platform Suffix in Twitch but Not in Kick

**What goes wrong:** In twitch-listener's assignment refresh loop, source IDs are stripped of their platform suffix: `"abc123:twitch"` → `"abc123"` (see `cmd/main.go` lines 264-269). Kick-listener's assignment refresh does not strip the suffix (lines 260-262). After migration to the SDK's shared assignment refresh logic, this divergence must be preserved — or one listener gets incorrect source ID filtering.

**Why it happens:** This is an undocumented behavioral difference between two otherwise-identical code blocks. It exists because Twitch source IDs include a platform suffix in the coordinator response but the twitch-listener's internal channel matching does not expect the suffix. It was added as a quick fix and not documented.

**Consequences:** If the SDK's assignment refresh strips the suffix universally, kick-listener breaks (it doesn't expect the strip). If it doesn't strip, twitch-listener breaks. If the SDK accepts a `StripPlatformSuffix bool` config field, future SDK users will not know whether to set it.

**Prevention:**
- Before extracting the assignment refresh loop, document the exact source ID format each listener receives from the coordinator and the format each listener's `UpdateAssignedSourceIDs` expects.
- The correct fix is upstream: normalize the coordinator's response to not include the platform suffix in the first place, or always include it and always strip it. Pick one and make it consistent across all listeners before extracting to the SDK.
- Do not add a `StripPlatformSuffix bool` field to `ListenerConfig`. That is encoding an undocumented inconsistency into the SDK's public interface.

**Detection:** Integration test: call `coordClient.QueryAssignments()` in both the old twitch-listener and the SDK-backed twitch-listener. Assert the resulting `assignedSourceIDs` map is identical.

**Phase:** Pre-migration cleanup task. Must be resolved before SDK extraction begins.

---

### Pitfall 10: `DBConnInterface` Defined Redundantly in Each Listener — Conflict on Extraction

**What goes wrong:** Both `twitch-listener/channels` and `kick-listener/channels` define a local `DBConnInterface` with method `GetPool() interface{}`. Both use a local `dbConnWrapper` struct in `cmd/main.go`. If the shared `ChannelManager` defines its own `DBConnInterface`, there are now three definitions of the same interface. The listener-local ones are not removed. Code that passes a `*dbConnWrapper` to the local `channels.NewManager` compiles, but if it is passed to the shared `channels.NewManager`, there may be a type mismatch at the interface boundary (Go structural typing means it still compiles, but the `interface{}` return from `GetPool()` hides the concrete type, making it impossible to use without a type assertion).

**Why it happens:** The interface was defined locally in each package because Go interfaces are lightweight. When extracting to shared code, the natural move is to define it in the SDK. But the old local definitions remain, and `cmd/main.go` still uses `dbConnWrapper` from the service package — which satisfies the local interface but may not satisfy the shared interface if signatures diverge.

**Consequences:** Compile errors after migration, or worse, silent behavior where both interfaces exist and different parts of the code use different definitions.

**Prevention:**
- Define `DBConnInterface` once: in the shared `ChannelManager` package. Remove the local definitions as part of migration. Do not leave them as aliases.
- The `GetPool() interface{}` return type is already a code smell — it forces callers to do type assertions. During extraction, change this to `GetPool() *pgxpool.Pool` for a stronger contract. If this is a breaking change for any consumer, address it explicitly.

**Phase:** During ChannelManager extraction (Phase 2 of SDK). Not a blocker for ListenerBase extraction.

---

### Pitfall 11: Shared `ChannelManager` with Two Different `HandleMigrationEvent` Signatures

**What goes wrong:** In both twitch-listener and kick-listener, `channelMgr.HandleMigrationEvent` is passed to `coordination.NewMigrationSubscriber`. The method signature is defined locally in each manager. If the signature differs between listeners (e.g., Kick's manager passes additional context about chatroom IDs), the shared `MigrationSubscriber` cannot accept a generic `HandleMigrationEvent` function.

Examining the codebase: the migration subscriber in `shared/coordination/migration_subscriber.go` calls a callback. If that callback's signature is `func(event MigrationEvent) error`, and kick-listener's current manager defines it as `func(event coordination.MigrationEvent)` (no error return), the migration to shared code requires a signature change — which breaks the existing manager test for kick.

**Prevention:**
- Before extracting the migration callback, determine the canonical signature: does it return an error or not? Make it return an error. This is strictly better and enables the subscriber to log callback errors.
- Update both listeners' `HandleMigrationEvent` to match the canonical signature before SDK extraction.
- The update to `HandleMigrationEvent` is a preparation task that should be done and deployed independently, before any SDK extraction. It is low-risk (error return is ignored or logged).

**Phase:** Pre-migration cleanup. Done before SDK extraction starts.

---

## Minor Pitfalls

---

### Pitfall 12: TikTok Listener Is Node.js — Cannot Use the Go SDK

**What goes wrong:** tiktok-listener is a Node.js/TypeScript service (confirmed by `tsconfig.json` and `node_modules` in the services directory). PROJECT.md lists it as one of the six listeners to migrate to the SDK. It cannot import Go packages.

If the roadmap defines "migrate all 6 listeners" as a success criterion and tiktok-listener is included in that count, the milestone can never be fully completed as specified.

**Prevention:**
- Redefine "migrate all 6 listeners" to explicitly exclude tiktok-listener, or plan a parallel effort to rewrite it in Go first.
- The SDK extraction provides value to the 5 Go listeners regardless of tiktok-listener. Document this explicitly so tiktok-listener's status does not block the milestone.

**Phase:** Milestone scoping clarification before roadmap is written.

---

### Pitfall 13: `go.work` Absent — Missing Module Graph Visibility During Development

**What goes wrong:** The monorepo uses `replace` directives and has no `go.work` file. During SDK development, if you `cd /shared && go test ./listener/...`, the test sees the local `shared/listener` code. But if you `cd /services/twitch-listener && go test ./...`, the test also sees local shared code (via replace directive). However, running `go build ./...` at the repo root fails because there is no root module.

The practical consequence: there is no single command to verify all modules compile against the new SDK. You must run `go build ./...` in each service directory separately. This is error-prone during migration.

**Prevention:**
- Create a `go.work` file at the repo root that includes all modules. This is not a breaking change and does not affect deployed behavior (go.work is a local development tool). The `replace` directives remain functional.
- With a `go.work` file, `go build ./...` from the repo root verifies all modules compile against the current SDK state in a single command.
- Alternatively, add a Makefile target `build-all` that runs `go build ./...` in each service directory. This requires no structural change.

**Detection:** CI explicitly tests all service modules, not just the module being changed.

**Phase:** Development tooling setup, ideally before any SDK code is written.

---

### Pitfall 14: `ListenerMetrics` Prometheus Labels Registered Twice

**What goes wrong:** After migration, both the old listener startup code path (if partially removed) and `ListenerBase.Start()` call `metrics.NewListenerMetrics("twitch", "twitch-listener")`. Prometheus panics on duplicate metric registration if the same label values are registered twice in the same process. The panic crashes the pod on startup.

**Why it happens:** Partial migrations where some initialization code is moved to the SDK but the original code is not fully removed. Also triggered if the listener unit tests register metrics during test setup and the SDK also registers them.

**Prevention:**
- Prometheus metric registration should use `MustRegister` only once per process. If the SDK registers metrics, the listener must not register them separately. Use `promauto` (registers on declaration) or register in an explicit `init()` guarded by a `sync.Once`.
- Test isolation: use `prometheus.NewRegistry()` in tests rather than the default global registry. Pass it via the `ListenerConfig`. This prevents test-to-test pollution and avoids the double-registration panic in test binaries.

**Phase:** During migration of each listener. Verify by running the new `cmd/main.go` in a test binary with `go test -run TestMain` before full deployment.

---

### Pitfall 15: Feature Flag `ENABLE_COORDINATOR_FILTERING` Is Twitch-Specific and Not in the SDK

**What goes wrong:** twitch-listener has `ENABLE_COORDINATOR_FILTERING` env var (lines 155-159 of `cmd/main.go`) that disables coordinator filtering entirely as an instant rollback. kick-listener does not have this flag. If the SDK manages the assignment filtering logic, this flag either becomes SDK-level (configurable by all listeners) or disappears. If it disappears, the operational rollback mechanism for twitch-listener disappears with it.

**Prevention:**
- Preserve the flag in the SDK's `ListenerConfig` as `DisableCoordinatorFiltering bool`. Document it as an emergency rollback mechanism, not a configuration option.
- Ensure the flag is evaluated in `ListenerBase` at the point where `assignedSourceIDs` is applied to filtering — not buried in the channel manager.

**Phase:** Minor concern during SDK design. Must be noted in the SDK config struct's documentation.

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| SDK design | Startup sequence as fixed order instead of hooks | Document all 6 listener divergences first; design struct-of-hooks before writing code |
| SDK design | Circular import between `shared/listener` and `shared/metrics` | Define import hierarchy on day one; platform constants in separate `shared/types` package |
| Pre-migration cleanup | Platform suffix stripping inconsistency (twitch strips, kick does not) | Normalize source ID format in coordinator response; single behavior for all listeners |
| Pre-migration cleanup | `HandleMigrationEvent` signature mismatch | Canonicalize to `func(event) error`; deploy to both listeners before SDK extraction |
| ChannelManager extraction | Generic key type (string vs. int) for Kick chatroom IDs | Use `ChannelManager[K comparable]` with generics, or string-keyed with explicit conversion documented |
| ChannelManager extraction | `DBConnInterface` defined in 3 places post-extraction | Define once in SDK; remove listener-local definitions as part of migration commit |
| ListenerBase extraction | Embed vs. composition method shadowing | Prefer composition; compile-time interface check `var _ Listener = (*TwitchListener)(nil)` |
| First listener migration | No SDK tests exist before migration | `goleak`-verified SDK tests required before touching any listener |
| First listener migration | Prometheus duplicate metric registration | Use `prometheus.NewRegistry()` in tests; SDK owns metric registration, listener does not |
| Each listener migration | Big-bang migration across all listeners | One listener at a time; 24-hour production soak before next migration |
| Mixed-fleet period | SDK behavioral change affects deployed listeners, unmigrated listeners immune | Freeze SDK timing semantics after first deployment; inject all tunable values via `ListenerConfig` |
| Mixed-fleet period | No single build command for all modules | Add `go.work` or `make build-all`; CI must build all modules on every PR |
| Milestone scoping | TikTok listener is Node.js and cannot use Go SDK | Exclude from migration scope explicitly; document why |
| Post-migration | `ENABLE_COORDINATOR_FILTERING` rollback mechanism disappears | Preserve as `DisableCoordinatorFiltering` in `ListenerConfig` |

---

## Sources

**Confidence assessment:**

- Migration pitfalls: HIGH — derived from direct inspection of `services/twitch-listener/cmd/main.go`, `services/kick-listener/cmd/main.go`, `services/twitch-listener/channels/manager.go`, `services/kick-listener/channels/manager.go`. All specific line references in pitfalls above verified against current source.
- Go-specific pitfalls (embed/interface, circular deps, replace directives): HIGH — these are properties of the Go module system and language spec. Not time-sensitive.
- Operational pitfalls (coordinated deploys, mixed fleet): HIGH — logical derivation from the multi-module monorepo structure and replace directive model confirmed in `services/twitch-listener/go.mod` line 87-89.
- TikTok listener runtime: HIGH — confirmed by presence of `tsconfig.json` and `node_modules/` in `services/tiktok-listener/`.
- go.work absence: HIGH — confirmed no `go.work` file exists at repo root via glob search.

**Codebase references:**
- `/home/moersener/Hobby/all-chat/services/twitch-listener/cmd/main.go` — startup sequence, jitter, platform suffix stripping, ENABLE_COORDINATOR_FILTERING flag
- `/home/moersener/Hobby/all-chat/services/kick-listener/cmd/main.go` — startup sequence divergence, leaderCoord initialization, reconnect goroutine
- `/home/moersener/Hobby/all-chat/services/twitch-listener/channels/manager.go` — firstMessageChan type (`map[string]chan struct{}`), DBConnInterface
- `/home/moersener/Hobby/all-chat/services/kick-listener/channels/manager.go` — firstMessageChan type (`map[int]chan struct{}`), DBConnInterface, dual-key structure
- `/home/moersener/Hobby/all-chat/services/twitch-listener/go.mod` — replace directive for shared
- `/home/moersener/Hobby/all-chat/services/discord-listener/go.mod` — discord-listener does not import coordination or sourcemanager packages (confirmed: no coordinator references)
- `/home/moersener/Hobby/all-chat/shared/go.mod` — shared module dependency list
- `/home/moersener/Hobby/all-chat/.planning/PROJECT.md` — v1.6 milestone requirements, constraints, known technical debt

---

*Pitfalls research for: All-Chat v1.6 Listener SDK*
*Researched: 2026-03-17*
*Confidence: HIGH*
