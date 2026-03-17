# Feature Landscape: Listener SDK

**Domain:** Shared Go SDK for all-chat listener microservices
**Researched:** 2026-03-17
**Confidence:** HIGH (grounded entirely in codebase analysis of existing listeners — twitch, kick, youtube, youtube-innertube, discord — and the shared/ packages already in /shared/)

---

## Research Notes

This research is based on direct inspection of:
- `services/twitch-listener/cmd/main.go` (330 lines)
- `services/kick-listener/cmd/main.go` (360 lines)
- `services/youtube-listener/cmd/main.go` (345 lines)
- `services/youtube-listener-innertube/cmd/main.go` (240 lines)
- `services/discord-listener/cmd/main.go` (305 lines)
- `shared/coordination/client.go` — CoordinatorClient with JWT refresh + heartbeat + assignment query
- `shared/sourcemanager/coordinator.go` — LeadershipCoordinator with EnsureLeadership + lease renewal

The goal is to define what a `ListenerBase` struct in `/shared/listener/` should provide, what it should leave to the concrete implementation, and where the two variants (channel-per-pod vs. leadership-per-stream) differ.

---

## Observed Duplication: What Every Listener Copies Today

Analyzing the five main.go files reveals the following blocks that are copy-pasted across all listeners, with only the service name and port changed:

| Duplicated block | Lines per listener | Listeners with it |
|---|---|---|
| Logger init (name + log level) | 4–6 | All 5 |
| Tracing init (OTEL_ENABLED check, Config, InitTracer, defer) | 15–18 | twitch, kick, youtube (3 of 5) |
| PostgreSQL connection (env vars, connString, NewPostgresPool, defer, Ping) | 12–15 | twitch, kick, youtube, discord (4 of 5) |
| Redis connection (env vars, NewClient, defer, Ping) | 10–12 | All 5 |
| Pod name from HOSTNAME env var (with fallback) | 5–6 | twitch, kick (2 of 5) |
| CoordinatorClient init (URL, JWT secret, NewCoordinatorClient) | 4–6 | twitch, kick (2 of 5) |
| JWT refresh start + defer stop | 2 | twitch, kick (2 of 5) |
| Startup jitter (rand.Intn(30) * time.Second sleep) | 5–6 | twitch, kick (2 of 5) |
| Assignment query (QueryAssignments, fatal on err) | 8–10 | twitch, kick (2 of 5) |
| Assignment filtering loop (assignedSourceIDs map) | 5–6 | twitch, kick (2 of 5) |
| ENABLE_COORDINATOR_FILTERING feature flag | 5–7 | twitch (1 of 5) |
| ShardMetrics init (NewShardMetrics) | 1–2 | twitch, kick (2 of 5) |
| Heartbeat goroutine (10s ticker, PublishHeartbeat) | 12–15 | twitch, kick (2 of 5) |
| Assignment refresh goroutine (60s ticker, QueryAssignments, UpdateAssignedSourceIDs) | 22–30 | twitch, kick (2 of 5) |
| MigrationSubscriber init + goroutine | 8–10 | twitch, kick (2 of 5) |
| LeadershipCoordinator init (SOURCE_MANAGER_SECRET check, NewSigningTokenSource, NewClient, NewLeadershipCoordinator) | 12–16 | kick, youtube, youtube-innertube, discord (4 of 5) |
| HTTP server setup (gin.New, Recovery, tracing middleware, port env var, http.Server, goroutine) | 18–22 | All 5 |
| Health route registration (live, ready, status) | 3–4 | All 5 |
| Prometheus /metrics route | 1 | All 5 |
| Signal handling + graceful shutdown (SIGINT/SIGTERM, srv.Shutdown) | 12–15 | All 5 |
| getEnvOrDefault helper | 5–6 | All 5 (named getEnv or getEnvOrDefault) |
| dbConnWrapper type (pgxpool.Pool → GetPool interface) | 6–8 | twitch, kick, youtube (3 of 5) |

**Conclusion:** Of the ~200–350 lines in each listener's main.go, roughly 100–150 lines are pure boilerplate. The concrete listener contributes only: platform-specific connection (IRC, WebSocket, HTTP polling, Gateway), service-specific manager setup, and any additional HTTP routes.

---

## Table Stakes

Features the SDK must provide. Missing any of these means the SDK does not reduce real duplication.

| Feature | Why Expected | Complexity | Depends On |
|---------|--------------|------------|------------|
| Logger construction from env (LOG_LEVEL → NewLogger with service name) | Every listener starts with this; identical across all 5 | Low | `shared/logger` |
| Redis connection setup (env vars, NewClient, Ping, defer Close) | All 5 listeners connect the same way | Low | `github.com/redis/go-redis/v9` |
| PostgreSQL connection setup (env vars, NewPostgresPool, defer Close) | 4 of 5 listeners connect the same way | Low | `shared/database` |
| HTTP server construction (gin.New, Recovery, port env var, http.Server with timeouts) | All 5 listeners build the same server structure | Low | `github.com/gin-gonic/gin` |
| Health route registration (/health/live, /health/ready, /status) | All 5 listeners register the same routes | Low | `HealthChecker` interface |
| Prometheus /metrics route registration | All 5 listeners register this | Low | `promhttp.Handler()` |
| Signal handling + graceful shutdown (SIGINT/SIGTERM → srv.Shutdown with 10s timeout) | All 5 listeners do this identically | Low | stdlib `os/signal` |
| OTEL tracing init (OTEL_ENABLED flag, Config, InitTracer, defer shutdown) | 3 of 5 listeners have this; the other 2 omit it inconsistently | Low | `shared/tracing` |
| getEnvOrDefault helper (exported as `Env(key, default string) string`) | All 5 listeners copy-paste this function | Low | none |
| Pod name from HOSTNAME with fallback (exported as `PodName(fallback string) string`) | Needed for coordinator and shard metrics | Low | none |
| Run lifecycle (blocks until signal, then calls Shutdown hook) | All 5 listeners have this; differs only in what gets stopped | Low | none |

---

## Table Stakes — ChannelManager Variant (assignment-based listeners)

For listeners using the coordinator (twitch, kick): these features belong in `ListenerBase` or a `ChannelManagerBase`.

| Feature | Why Expected | Complexity | Depends On |
|---------|--------------|------------|------------|
| CoordinatorClient init (URL, JWT secret, NewCoordinatorClient) | twitch and kick both init this identically | Low | `shared/coordination` |
| JWT refresh start + defer stop | Prevents token expiration; both listeners start this immediately after coord init | Low | CoordinatorClient |
| Startup jitter (0–30s random sleep) | Both listeners apply this before QueryAssignments to prevent thundering herd | Low | none |
| Initial assignment query (QueryAssignments, fatal on err) | Both listeners block on this before starting | Low | CoordinatorClient |
| Assignment ID map construction (assigned SourceID → bool) | Both listeners build this map identically | Low | QueryAssignments result |
| Heartbeat goroutine (10s ticker, PublishHeartbeat) | Both listeners start this goroutine identically | Low | CoordinatorClient |
| Assignment refresh goroutine (60s ticker, QueryAssignments, UpdateAssignedSourceIDs) | Both listeners copy-paste this loop with minor key-parsing variations | Medium | CoordinatorClient + ChannelManager interface |
| MigrationSubscriber init + goroutine (HandleMigrationEvent callback) | Both listeners do this identically | Low | `shared/coordination.MigrationSubscriber` |
| ShardMetrics pod channel count recording (after channel manager start) | Both listeners record filteredCount after manager Start | Low | `shared/metrics.ShardMetrics` |

---

## Table Stakes — LeadershipListener Variant (stream-ownership listeners)

For listeners using per-stream leadership (kick, youtube, youtube-innertube, discord): these belong in `LeadershipListener` or are wired by `ListenerBase` when a `LeadershipCoordinator` is provided.

| Feature | Why Expected | Complexity | Depends On |
|---------|--------------|------------|------------|
| LeadershipCoordinator init (SOURCE_MANAGER_SECRET guard, NewSigningTokenSource, NewClient, NewLeadershipCoordinator) | 4 of 5 listeners construct this identically | Low | `shared/sourcemanager` |
| SOURCE_MANAGER_SECRET absent → warn + nil coordinator (still boots) | All 4 leadership listeners handle the nil case; absence means "no coordination, run unconditionally" | Low | none |
| Nil coordinator passthrough (listener works without coordination) | LeadershipCoordinator.EnsureLeadership already handles nil receiver — SDK must not require non-nil | Low | Existing nil guard in LeadershipCoordinator |

---

## Differentiators

Features that elevate the SDK from "useful shared code" to "new listeners are trivial to build."

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| `PlatformListener` interface (Connect, Disconnect, IsConnected) | Single method set that ListenerBase calls at startup and shutdown; concrete listeners implement only this | Low | Interface definition; no runtime overhead |
| `HealthChecker` interface (IsLive, IsReady, Status) passed into health handler | Decouples health handler construction from the HTTP boilerplate; ListenerBase owns the handler, concrete listener provides the checks | Low | Replaces per-listener `handlers.NewHealthHandler(...)` |
| `OnShutdown` hook (called after signal, before HTTP server shutdown) | Lets each listener stop its platform connection without touching signal handling code | Low | Ordered shutdown: listener.Disconnect → HTTP shutdown |
| Assignment key normalizer as injectable function (strips platform suffix like `:twitch` from SourceID) | twitch-listener has a platform-suffix stripping loop in its refresh goroutine that kick-listener lacks — normalizer resolves the inconsistency | Low | Fixes existing divergence |
| `ENABLE_COORDINATOR_FILTERING` feature flag centralized in base | Currently only twitch-listener has this flag; all assignment-based listeners should have it | Low | Env flag read once in base |
| Configurable heartbeat interval (default 10s, env `LISTENER_HEARTBEAT_INTERVAL`) | Makes testing faster without sleep hacks | Low | Replaces hardcoded `10 * time.Second` |
| Configurable assignment refresh interval (default 60s, env `LISTENER_ASSIGNMENT_REFRESH_INTERVAL`) | Allows tuning per environment | Low | Replaces hardcoded `60 * time.Second` |
| Configurable startup jitter max (default 30s, env `LISTENER_STARTUP_JITTER_MAX`) | Allows disabling jitter in tests (`LISTENER_STARTUP_JITTER_MAX=0`) without code changes | Low | Replaces hardcoded `rand.Intn(30)` |
| `ListenerMetrics` construction wired into base (platform label injected) | twitch and youtube init `metrics.NewListenerMetrics` with the platform name — base can own this given the service name | Low | `shared/metrics` |

---

## Anti-Features

Features to explicitly not build in this SDK.

| Anti-Feature | Why Avoid | What to Do Instead |
|--------------|-----------|-------------------|
| Automatic platform-specific client construction inside ListenerBase | ListenerBase cannot know about IRC, WebSocket, InnerTube HTTP, or Discord Gateway — coupling to platform specifics defeats the purpose | Concrete listener constructs its own client, passes it to the manager |
| Message publishing logic inside ListenerBase | Each listener has a different publisher (stream publisher, innertube publisher) with different signatures | Concrete listener owns publisher; base only owns lifecycle |
| Health handler implementation inside ListenerBase | Health checks are deeply platform-specific (IRC connection state, WebSocket connected, Redis ping, quota tracker state) | `HealthChecker` interface: base registers routes, concrete listener implements the interface |
| Automatic database migration execution at startup | Not every listener needs DB (innertube does not); running migrations from multiple pods simultaneously is unsafe | Migrations stay in a dedicated migration job/init container |
| Context propagation within base (passing ctx through interface methods) | Context ownership is subtle; base already owns the root context from signal handling — listener interface methods receive ctx from base | Pass ctx into Connect(ctx), Disconnect() does not need it; manager methods receive ctx from their own goroutines |
| Retry logic for QueryAssignments inside base | The current pattern fatals on error (correct: if the coordinator is unreachable at startup, there is nothing to do) | Keep fatal-on-error behavior; retry belongs in the coordinator's HTTP client if at all |
| Embedded config struct with all possible fields | A config struct with 30 fields for every listener produces noise for listeners that use 5 of them | Functional options pattern (WithHeartbeatInterval, WithJitterMax, WithAssignmentRefreshInterval) or a minimal required Config + optional setters |
| SDK version of dbConnWrapper (pgxpool.Pool → GetPool() interface{}) | This wrapper only exists because some channel manager constructors take an interface instead of a concrete type — the real fix is to accept `*pgxpool.Pool` directly | Fix the channel manager constructor signatures during migration; do not bless this wrapper into the SDK |
| Logging the same startup messages from inside base | Base logging should be minimal (one "starting {service}" line); verbose per-step logging belongs in the concrete service | Base logs at debug level for lifecycle events |

---

## Feature Dependencies

```
ListenerBase (core lifecycle)
    requires──> shared/logger (NewLogger)
    requires──> shared/tracing (InitTracer, optional via OTEL_ENABLED)
    requires──> github.com/gin-gonic/gin (HTTP server)
    requires──> github.com/prometheus/client_golang/prometheus/promhttp (/metrics)
    produces──> Running HTTP server with /health/live, /health/ready, /metrics
    produces──> Signal-based graceful shutdown

ListenerBase (channel-manager variant / assignment-based)
    requires──> ListenerBase (core lifecycle)
    requires──> shared/coordination.CoordinatorClient
    requires──> shared/coordination.MigrationSubscriber
    requires──> shared/metrics.ShardMetrics
    requires──> ChannelManager interface (UpdateAssignedSourceIDs, HandleMigrationEvent, GetFilteredAssignmentCount)
    produces──> Assignment query at startup
    produces──> Heartbeat goroutine (10s)
    produces──> Assignment refresh goroutine (60s)
    produces──> Migration event subscriber goroutine

ListenerBase (leadership variant / stream-ownership)
    requires──> ListenerBase (core lifecycle)
    requires──> shared/sourcemanager.LeadershipCoordinator (may be nil)
    produces──> leaderCoord available to concrete listener's stream manager

HealthChecker interface
    required by──> ListenerBase (routes /health/live and /health/ready to it)
    implemented by──> each concrete listener's health handler

PlatformListener interface
    required by──> ListenerBase (called at Connect and shutdown)
    implemented by──> each concrete listener's connection manager

Database + Redis construction
    produced by──> ListenerBase
    consumed by──> concrete listener via base.DB() and base.Redis() accessors
    note: innertube listener does not use DB — accessor returns nil, concrete listener must not call it
```

---

## Startup Sequence (SDK-defined order)

The current listeners each hard-code this sequence. The SDK should codify it as the canonical order:

```
1. Logger construction (from LOG_LEVEL, service name)
2. Tracing init (if OTEL_ENABLED=true)
3. Infrastructure connections (Redis, then PostgreSQL if needed)
   → fatal if Redis unavailable
   → fatal if PostgreSQL unavailable and listener requires it
4. Metrics init (ListenerMetrics + ShardMetrics if channel-manager variant)
5. Pod name resolution (HOSTNAME env var with fallback)
6. CoordinatorClient init + JWT refresh start    (channel-manager variant only)
7. LeadershipCoordinator init                    (leadership variant only)
8. Startup jitter sleep (0 to LISTENER_STARTUP_JITTER_MAX seconds)
9. Initial assignment query → build assignedSourceIDs map  (channel-manager variant)
10. Concrete listener: platform connection setup (IRC, WebSocket, etc.)
11. Concrete listener: manager start (channel sync, stream discovery)
12. Channel count metric recording               (channel-manager variant)
13. MigrationSubscriber goroutine start          (channel-manager variant)
14. Heartbeat goroutine start                    (channel-manager variant)
15. Assignment refresh goroutine start           (channel-manager variant)
16. HTTP server start (health + metrics routes, then concrete listener's additional routes)
17. Block on SIGINT/SIGTERM
18. Shutdown: OnShutdown hook (concrete listener stops platform connection)
19. Shutdown: HTTP server shutdown (10s timeout)
20. Shutdown: defer cleanup (DB close, Redis close, tracer flush)
```

Steps 1–9 and 13–20 are identical across listeners and belong entirely in the SDK. Steps 10–12 are listener-specific. Step 16 is split: base registers /health and /metrics, concrete listener adds platform-specific routes before `ListenAndServe`.

---

## Graceful Shutdown Contract

The SDK defines shutdown order to prevent log noise from goroutines writing to closed connections:

```
SIGINT/SIGTERM received
    │
    ├─ Cancel root context (stops all goroutines watching ctx.Done())
    │   This stops: heartbeat, assignment refresh, migration subscriber,
    │               leadership lease renewals, stream managers
    │
    ├─ Call concrete listener's OnShutdown()
    │   Concrete listener: channelMgr.Stop() → ircConn.Disconnect() (twitch pattern)
    │   Concrete listener: channelMgr.Stop() → wsClient.Disconnect() (kick pattern)
    │   Concrete listener: streamManager.Stop() → quotaTracker.Stop() (youtube pattern)
    │   Concrete listener: gwClient.Close() → relayMgr.Stop() (discord pattern)
    │
    └─ srv.Shutdown(ctx with 10s timeout)
```

The SDK does NOT dictate what OnShutdown does internally — only that it is called before the HTTP server drain.

---

## Configuration Injection Approach

Based on observed listener patterns, a functional options approach fits best:

```go
// Required configuration (always needed)
type Config struct {
    ServiceName string        // e.g. "twitch-listener"
    Platform    string        // e.g. "twitch" (for metrics labels)
    PodName     string        // os.Getenv("HOSTNAME") with fallback
}

// Options for channel-manager variant
WithCoordinator(url, jwtSecret string)
WithHeartbeatInterval(d time.Duration)
WithAssignmentRefreshInterval(d time.Duration)
WithJitterMax(d time.Duration)

// Options for leadership variant
WithLeadershipCoordinator(coord *sourcemanager.LeadershipCoordinator)

// Options for infrastructure
WithDatabase(pool *pgxpool.Pool)   // if concrete listener needs DB
WithTracingEnabled(endpoint string)
WithPort(port string)
```

This avoids both the "giant config struct" anti-pattern and the "magic reflection on env vars" approach. The concrete listener's main.go calls `listener.New(cfg, options...)`, connects its platform, then calls `base.Run(ctx, platformListener)`.

---

## MVP Recommendation

Prioritize (deliver in one milestone):

1. `ListenerBase` with core lifecycle (logger, redis, gin server, health routes, metrics route, signal shutdown) — eliminates 80–100 lines from every listener
2. `ChannelManagerBase` wiring (jitter, coord client, JWT refresh, assignment query, heartbeat, assignment refresh, migration subscriber) — eliminates 80–100 lines from twitch and kick
3. `LeadershipCoordinator` construction helper (SOURCE_MANAGER_SECRET guard, signing token source, NewClient) — eliminates 12–16 lines from kick, youtube, youtube-innertube, discord
4. `getEnvOrDefault` → `shared/listener.Env(key, default)` — eliminate 5-line copy in every listener
5. Migrate all 6 listeners to use the SDK (each migration validates the API)

Defer:
- `HealthChecker` interface (LOW value before migration proves the API; concrete health handlers can remain in each service for v1.6)
- Configurable intervals via env vars (LOW urgency; hardcoded defaults are fine once centralized)
- `ENABLE_COORDINATOR_FILTERING` flag in base (only twitch uses it; generalizing mid-migration adds risk)

---

## Sources

**Codebase analysis (HIGH confidence — direct file inspection):**
- `/services/twitch-listener/cmd/main.go` — assignment-based listener, 330 lines, no leadership
- `/services/kick-listener/cmd/main.go` — both assignment-based and leadership, 360 lines
- `/services/youtube-listener/cmd/main.go` — leadership-only, 345 lines, no coordinator
- `/services/youtube-listener-innertube/cmd/main.go` — leadership-only, 240 lines, no PostgreSQL
- `/services/discord-listener/cmd/main.go` — leadership-only, 305 lines, no coordinator
- `/shared/coordination/client.go` — CoordinatorClient, JWT refresh, QueryAssignments, PublishHeartbeat
- `/shared/sourcemanager/coordinator.go` — LeadershipCoordinator, EnsureLeadership, nil-safe receiver

---
*Feature research for: All-Chat Listener SDK (v1.6)*
*Researched: 2026-03-17*
*Confidence: HIGH — grounded entirely in direct codebase analysis, no external sources needed*
