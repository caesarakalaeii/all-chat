# Project Research Summary

**Project:** All-Chat v1.6 — Listener SDK
**Domain:** Go shared library extraction from working Go microservices monorepo
**Researched:** 2026-03-17
**Confidence:** HIGH

## Executive Summary

The v1.6 milestone is a refactoring project: extract a shared Go SDK from five working listener microservices (twitch-listener, kick-listener, youtube-listener, youtube-listener-innertube, discord-listener) and migrate all of them to use it. Each listener's `cmd/main.go` currently contains 100–150 lines of identical startup wiring — logger init, Redis/PostgreSQL connection, coordinator client setup, startup jitter, assignment query, heartbeat loop, assignment refresh loop, migration subscriber, HTTP server, health routes, Prometheus metrics, and graceful shutdown. The SDK lives in the existing `shared` Go module at `shared/listener/`, making it automatically available to all listeners via their already-in-place `replace` directives. No new Go modules or `go.work` file is required.

The recommended approach is interface-first design with two concrete base types: `ListenerBase` for assignment-based listeners (Twitch, YouTube) and `LeadershipListener` (embeds `ListenerBase`) for per-stream leadership listeners (Kick, YouTube InnerTube, Discord). The `ChannelManager` interface extracts the Twitch/Kick channel management contract into the SDK layer, enabling `ListenerBase.Run` to wire migration events without importing platform-specific packages. The SDK does not touch the message publishing path — platform-specific publisher code stays in each service.

The governing risk is that all six listeners carry live production traffic and the SDK has no production track record. Every migration must be done one listener at a time with a 24-hour production soak before the next migration begins. Two pre-migration cleanup tasks are blockers that must be completed before SDK extraction starts: normalizing the source ID platform suffix inconsistency between Twitch and Kick, and canonicalizing the `HandleMigrationEvent` signature across both services. Attempting these as part of the SDK migration rather than before it creates compound failure modes that are harder to debug and revert. tiktok-listener is Node.js and is explicitly out of scope for the Go SDK.

## Key Findings

### Recommended Stack

The SDK requires no new dependencies. All required packages are already present in the shared module and pinned to the following versions across existing listener services: `gin-gonic/gin` v1.12.0, `redis/go-redis/v9` v9.18.0, `jackc/pgx/v5` v5.8.0, `go.uber.org/zap` v1.27.1, `prometheus/client_golang` v1.23.2, `go.opentelemetry.io/otel` v1.42.0. Go version is 1.25.6. `bwmarrin/discordgo` v0.28.1 is already in the discord-listener; verify `go get github.com/bwmarrin/discordgo@latest` before pinning.

**Core technologies:**
- `shared/coordination` (existing): CoordinatorClient with JWT refresh, heartbeat, assignment query — unchanged by SDK; gains explicit `serviceName` parameter on `NewCoordinatorClient`, removing hostname auto-detection
- `shared/sourcemanager` (existing): LeadershipCoordinator for per-stream ownership — unchanged by SDK; nil-safe methods already implemented
- `shared/listener` (new): four files — `base.go`, `leadership.go`, `channel_manager.go`, `shutdown.go` — all within the existing shared module
- `golang.org/x/time/rate` (existing transitive): rate limiting for Discord relay (5 msg/5s per channel) — already available
- `prometheus/client_golang` (existing): SDK owns metric registration to prevent duplicate-registration panics; tests use `prometheus.NewRegistry()` not the global registry

See `/home/moersener/Hobby/all-chat/.planning/research/STACK.md` for full dependency table and version requirements.

### Expected Features

Research is grounded entirely in direct codebase inspection of five listener `cmd/main.go` files and `shared/` packages. Features are not speculative.

**Must have (table stakes):**
- `ListenerBase` core lifecycle: logger, Redis, PostgreSQL, gin HTTP server, health routes, Prometheus metrics route, signal-based graceful shutdown — eliminates 80–100 lines from every listener
- `ChannelManagerBase` wiring: jitter, CoordinatorClient, JWT refresh, initial assignment query, heartbeat goroutine, assignment refresh goroutine, migration subscriber goroutine — eliminates 80–100 lines from Twitch and Kick
- `LeadershipCoordinator` construction helper: SOURCE_MANAGER_SECRET guard, signing token source, `NewClient`, nil-safe passthrough — eliminates 12–16 lines from Kick, YouTube InnerTube, and Discord
- `shared/listener.Env(key, default string) string` helper — eliminates 5-line copy-paste in every listener
- Migration of all 5 Go listeners to use the SDK (tiktok-listener is Node.js and is explicitly excluded)

**Should have (differentiators that elevate the SDK from useful to essential):**
- `PlatformListener` interface (`Connect`, `Disconnect`, `IsConnected`) — single method set called at startup and shutdown
- `HealthChecker` interface — decouples health route registration from HTTP boilerplate; base registers routes, concrete listener implements checks
- `OnShutdown` hook — lets each listener stop its platform connection without touching signal handling code
- Configurable heartbeat interval (`LISTENER_HEARTBEAT_INTERVAL`, default 10s) — replaces hardcoded constant
- Configurable assignment refresh interval (`LISTENER_ASSIGNMENT_REFRESH_INTERVAL`, default 60s)
- Configurable startup jitter max (`LISTENER_STARTUP_JITTER_MAX=0` disables jitter in tests)
- `DisableCoordinatorFiltering` bool in `ListenerConfig` — preserves the operational rollback mechanism currently in twitch-listener
- CI `make build-all` target (or `go.work`) for monorepo-wide compile verification

**Defer (v1.7+):**
- `HealthChecker` interface full rollout — concrete health handlers can remain per-service for v1.6 until migration proves the API shape
- Generic `ChannelManager[K comparable]` — string-keyed with documented `strconv.Itoa(chatroomID)` convention is sufficient for v1.6; generics add complexity before the interface is proven
- `go.work` at repo root — a `make build-all` Makefile target achieves the same CI coverage without the `go mod tidy` side-effect risks

See `/home/moersener/Hobby/all-chat/.planning/research/FEATURES.md` for the full feature dependency graph and MVP recommendation.

### Architecture Approach

The SDK lives entirely in `shared/listener/` as a new package within the existing `github.com/caesar/all-chat/shared` Go module. Four new files are added; no existing shared packages are structurally modified. The strict import hierarchy — `shared/listener` may import `shared/{coordination,sourcemanager,metrics,tracing,logger}` but none of those packages may import `shared/listener` — must be documented in `shared/listener/doc.go` on day one. Any type needed by both layers (platform name constants) lives in a new `shared/types` package with no upstream dependencies. The SDK owns the lifecycle only; it never calls `Publish` into the `chat:raw` stream.

**Major components:**
1. `shared/listener/base.go` — `Config`, `ListenerBase` struct: `Start()` (jitter + assignment query), `Run()` (heartbeat, assignment refresh, migration subscriber goroutines), `Stop()`, `GetFilteredAssignedSourceIDs()`
2. `shared/listener/leadership.go` — `LeadershipConfig`, `LeadershipListener` struct (embeds `ListenerBase`): `NewLeadershipListener()` constructs `LeadershipCoordinator` + `Client` or returns nil if `SOURCE_MANAGER_SECRET` absent
3. `shared/listener/channel_manager.go` — `ChannelManager` interface (`Start`, `Stop`, `HandleMigrationEvent`, `UpdateAssignedSourceIDs`, `GetFilteredAssignmentCount`); `PlatformConnector` interface (`Disconnect() error`)
4. `shared/listener/shutdown.go` — `ShutdownCoordinator.Wait()`: ordered shutdown — channel manager stop (parallel with base.Stop()) → platform disconnect → HTTP server drain with 10s timeout

**Unchanged components:** `shared/coordination/`, `shared/sourcemanager/`, `shared/metrics/`, all service `channels/repository.go`, `handlers/`, `publisher/`, platform connection packages (IRC, WebSocket, InnerTube), Redis keys (`chat:raw`), `RawChatMessage` schema, Kubernetes manifests.

**One modification to existing shared code:** `shared/coordination/client.go` — add explicit `serviceName string` parameter to `NewCoordinatorClient`, remove hostname prefix auto-detection. This is done in Phase 2 before any listener migration.

See `/home/moersener/Hobby/all-chat/.planning/research/ARCHITECTURE.md` for full struct signatures, data flow diagrams, component boundaries, and the five-phase build order with per-phase file change lists.

### Critical Pitfalls

1. **Big-bang migration breaks the entire listener fleet** — one listener at a time, non-negotiable. Twitch is the safest first target (no leadership coordinator). Monitor `messages_published_total` metric; any drop >10% sustained for 5 minutes is a rollback trigger. The SDK has no production track record; treat first deployment as an experiment.

2. **Hardcoded startup sequence straightjackets listeners with divergent ordering** — before writing any SDK code, diff all six Go listener `cmd/main.go` files step by step and document every divergence point. Design `ListenerBase` as a struct-of-hooks, not a fixed call sequence. Kick has a reconnect goroutine between "start channel manager" and "start migration subscriber"; YouTube InnerTube has stream discovery before assignment query. These differences break a fixed sequence.

3. **Source ID platform suffix inconsistency blocks correct SDK extraction** — Twitch strips `:twitch` suffix from coordinator assignment IDs; Kick does not. This divergence must be normalized before SDK extraction begins. Do not encode it as a `StripPlatformSuffix bool` flag in `ListenerConfig` — that makes an undocumented inconsistency permanent.

4. **Circular dependency between `shared/listener` and `shared/metrics`** — `shared/metrics` is already listener-aware (`NewListenerMetrics("twitch", ...)`). If `shared/listener` imports `shared/metrics` and `shared/metrics` later needs a type from `shared/listener`, the build breaks with a circular import. Define the import hierarchy and create `shared/types` for cross-cutting constants before writing the first SDK file.

5. **SDK behavioral changes after partial migration create a mixed fleet** — once `ListenerBase` is deployed by the first migrated listener, its timing semantics (jitter duration, heartbeat interval, assignment refresh interval) are frozen for the rest of the migration window. All tunable values must be injected via `ListenerConfig` struct fields, not hardcoded, so per-listener overrides are possible without changing the SDK.

See `/home/moersener/Hobby/all-chat/.planning/research/PITFALLS.md` for all 15 pitfalls with phase-specific warnings, detection metrics, and mitigation strategies.

## Implications for Roadmap

Research produced a clear dependency-driven build order: pre-migration cleanup must precede SDK definition (the inconsistencies cannot be fixed inside the SDK without encoding them), SDK must be tested before the first migration (no production deployment of untested shared lifecycle code), and migrations must proceed one listener at a time from simplest to most complex.

### Phase 1: Pre-Migration Cleanup
**Rationale:** Two behavioral inconsistencies between existing listeners must be resolved before SDK extraction begins. Attempting to paper over them inside the SDK permanently encodes undocumented divergences into the public interface. Both fixes are small, low-risk, and independently deployable.
**Delivers:** Normalized source ID handling deployed to all listeners (either coordinator normalizes its response or all listeners strip the suffix uniformly — one behavior, no branching in the SDK); canonical `HandleMigrationEvent(event *coordination.MigrationEvent) error` signature deployed to both Twitch and Kick channel managers
**Addresses:** Pitfall 9 (source ID suffix inconsistency), Pitfall 11 (HandleMigrationEvent signature mismatch)
**Avoids:** Encoding undocumented inconsistencies into the SDK's permanent public interface

### Phase 2: SDK Package Definition (no listener changes)
**Rationale:** The SDK must exist and be fully tested before any listener is migrated. `goleak.VerifyNone(t)` tests must verify goroutine lifecycle before the SDK touches production traffic. This phase is purely additive — four new files in `shared/listener/`, one signature change in `shared/coordination/client.go`, no existing package structural changes.
**Delivers:** `shared/listener/{base.go, leadership.go, channel_manager.go, shutdown.go}`; unit tests with mock coordinator verifying start/stop goroutine lifecycle; `go build ./...` passing in `shared/`; `shared/coordination/client.go` updated with explicit `serviceName` parameter
**Uses:** All existing shared dependencies — no new `go.mod` entries required
**Implements:** All four SDK components (ListenerBase, LeadershipListener, ChannelManager interface, ShutdownCoordinator)
**Avoids:** Pitfall 4 (circular imports — import hierarchy established here), Pitfall 8 (SDK unit tested before migration), Pitfall 14 (Prometheus registry strategy decided here)

### Phase 3: Migrate twitch-listener
**Rationale:** Twitch is the safest first migration candidate — no leadership coordinator, pure assignment-based model. Acts as the first real-world production validation of the SDK's lifecycle, assignment query, heartbeat, and shutdown sequence. Must be deployed and soaked for 24 hours before any other migration proceeds.
**Delivers:** twitch-listener `cmd/main.go` reduced by ~120 lines; compile-time `ChannelManager` interface assertion added (`var _ listener.ChannelManager = (*channels.Manager)(nil)`); SDK confirmed working in production with `messages_published_total` metric baseline established
**Avoids:** Pitfall 1 (single listener at a time), Pitfall 7 (composition vs. embed decision validated against real usage), Pitfall 14 (Prometheus duplicate registration)

### Phase 4: Migrate kick-listener
**Rationale:** Kick uses both the assignment-based coordinator (same as Twitch, validating `ListenerBase`) and the leadership coordinator (adding `LeadershipListener` validation). The Kick chatroom ID key type (int vs. string) must be resolved here — use string-keyed manager with `strconv.Itoa(chatroomID)` as the documented convention. Both SDK archetypes are confirmed in production after this phase.
**Delivers:** kick-listener `cmd/main.go` using both `ListenerBase` and `LeadershipListener`; `ChannelManager` interface assertion added; both SDK archetypes (assignment + leadership) confirmed in production; `DBConnInterface` defined once in SDK with `GetPool() *pgxpool.Pool` (stronger type than current `interface{}`)
**Addresses:** Pitfall 3 (ChannelManager key type), Pitfall 10 (DBConnInterface deduplication)
**Avoids:** Pitfall 5 (all listener modules built in CI during this phase to catch replace directive version skew)

### Phase 5: Migrate youtube-listener-innertube and discord-listener
**Rationale:** Both are leadership-only listeners with no assignment coordinator. Their migrations are independent of each other and can run in parallel branches. This validates `LeadershipListener` as a standalone archetype without the full assignment machinery. Keeping the mixed-fleet window short reduces operational risk from Pitfall 6.
**Delivers:** youtube-listener-innertube and discord-listener `cmd/main.go` using `LeadershipListener`; manual `sourcemanager.NewLeadershipCoordinator` wiring eliminated from both services
**Avoids:** Pitfall 6 (mixed-fleet window shortened by migrating two services together)

### Phase 6: Migrate youtube-listener
**Rationale:** YouTube uses the assignment-based coordinator (same archetype as Twitch) but has additional quota tracking complexity that makes debugging harder. Migrated last so the SDK's assignment machinery is well-proven before touching the service with the most fragile health state. Completing this phase closes the migration window.
**Delivers:** All 5 Go listeners running on the SDK; migration window closed; mixed-fleet monitoring alert cleared; Grafana deployment timestamp dashboard showing all services on SDK-backed code
**Avoids:** Pitfall 6 (mixed-fleet period ends here)

### Phase Ordering Rationale

- Pre-migration cleanup (Phase 1) must precede SDK definition (Phase 2) to avoid encoding inconsistencies into the public API
- SDK definition (Phase 2) must precede all migrations (Phases 3–6) — SDK must be tested before production exposure
- Twitch first (Phase 3) because it is the simplest archetype; validates `ListenerBase` in isolation before adding leadership complexity
- Kick second (Phase 4) because it exercises both archetypes; validates `LeadershipListener` after `ListenerBase` is proven stable
- YouTube InnerTube and Discord together (Phase 5) because they share the leadership-only archetype and their migrations are independent
- YouTube last (Phase 6) because quota complexity makes debugging harder; all SDK primitives proven by then
- tiktok-listener explicitly excluded — Node.js service cannot use the Go SDK

### Research Flags

Phases with well-documented patterns (additional research not needed):
- **Phase 2 (SDK definition):** All APIs verified from direct codebase inspection — HIGH confidence. Standard Go struct/interface/goroutine patterns. No external API unknowns.
- **Phase 3 (Twitch migration):** Twitch-listener is the most thoroughly understood service. The startup sequence is fully traced in the research files. No new external dependencies.
- **Phase 4 (Kick migration):** Chatroom ID key type resolution is a design decision, not a research question. String-keyed with `strconv.Itoa` is the recommended answer; proceed without further research.

Phases needing careful integration testing (not external research, but implementation verification):
- **Phase 5 (YouTube InnerTube + Discord):** Discord `LeadershipListener` usage confirmed from source but the shard ownership model has not been exercised via the SDK. Integration test shard acquisition and release with the SDK-backed implementation before deploying.
- **Phase 6 (YouTube quota-based):** YouTube's quota tracker state interacts with the assignment loop timing. Run the quota tracker's existing tests against the SDK-backed assignment implementation before deploying to confirm no behavioral change.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | All dependency versions verified from actual `go.mod` files. discordgo version is MEDIUM — run `go get` to confirm before pinning. No new dependencies required. |
| Features | HIGH | Grounded entirely in direct inspection of five listener `cmd/main.go` files and `shared/` packages. Duplication count and line estimates are from actual file analysis. |
| Architecture | HIGH | Package structure derived from existing module topology verified via `go.mod` replace directives and source file inspection. Interface method sets verified against existing implementations. |
| Pitfalls | HIGH | All critical pitfalls verified against specific source files and line numbers. Migration risks are logical derivations from the multi-module monorepo structure. Go-specific pitfalls (embed/interface, circular imports, replace directives) are properties of the language spec and module system. |

**Overall confidence:** HIGH

### Gaps to Address

- **Source ID normalization strategy:** Pitfall 9 identifies that Twitch strips platform suffixes from assignment IDs and Kick does not. The correct fix (normalize coordinator response vs. normalize all listeners) depends on whether the coordinator can be changed without affecting non-listener consumers. Validate in Phase 1 by inspecting the coordinator's `/assignments` endpoint response format and all known consumers before choosing the normalization approach.

- **`HandleMigrationEvent` error return impact:** The canonicalization to `func(event) error` is a signature change to deployed code. Confirm that `shared/coordination/migration_subscriber.go` will log or ignore the returned error appropriately before deploying the canonical signature. Resolve in Phase 1.

- **ChannelManager test coverage for Kick's int key:** The decision to use string-keyed manager with `strconv.Itoa` for Kick must be validated by running existing Kick channel manager tests with the string-keyed interface. If tests require material rewriting, reconsider the generic `ChannelManager[K comparable]` approach. Validate at the start of Phase 4.

- **discordgo version:** Run `go get github.com/bwmarrin/discordgo@latest` before creating `discord-listener/go.mod`. Expected v0.28.1 or higher. Not a blocker — confirm at the start of Phase 2.

## Sources

### Primary (HIGH confidence — direct codebase inspection)
- `/services/twitch-listener/cmd/main.go` — full startup sequence, assignment loop, jitter, ENABLE_COORDINATOR_FILTERING flag
- `/services/kick-listener/cmd/main.go` — dual archetype (assignment + leadership), reconnect goroutine, divergence from Twitch
- `/services/youtube-listener/cmd/main.go` — leadership-only, quota complexity
- `/services/youtube-listener-innertube/cmd/main.go` — leadership-only, no PostgreSQL
- `/services/discord-listener/cmd/main.go` — leadership-only, no CoordinatorClient
- `/services/twitch-listener/channels/manager.go` — ChannelManager method set, `map[string]chan struct{}`
- `/services/kick-listener/channels/manager.go` — `map[int]chan struct{}`, DBConnInterface, dual-key structure
- `/shared/coordination/client.go` — CoordinatorClient API, hostname auto-detection (targeted for removal)
- `/shared/sourcemanager/coordinator.go` — LeadershipCoordinator nil-safe methods
- `/shared/go.mod`, `/services/*/go.mod` — dependency versions and replace directives
- `/services/discord-listener/go.mod` — confirmed no coordination/sourcemanager imports

### Secondary (MEDIUM confidence — training knowledge, stable APIs)
- Discord API v10 reference — Gateway v10 stable since February 2022; `MESSAGE_CONTENT` privileged intent enforced since April 2022
- `bwmarrin/discordgo` v0.28.1 — latest as of August 2025 training cutoff; verify before pinning

---
*Research completed: 2026-03-17*
*Ready for roadmap: yes*
