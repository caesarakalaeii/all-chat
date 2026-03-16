# Phase 31: Load Balancing - Research

**Researched:** 2026-03-16
**Domain:** Discord Gateway shard ownership, session resume, Kubernetes HPA with Prometheus custom metrics
**Confidence:** HIGH

## Summary

Phase 31 adds three production hardening capabilities to discord-listener: (1) shard ownership gating so only one pod holds the Gateway WebSocket at a time, (2) Gateway session RESUME so pod restarts avoid the cost of a full re-IDENTIFY, and (3) HPA configuration so the service auto-scales and new pods acquire ownership within the 60-second SLO.

The project already ships a complete leadership coordination stack: `shared/sourcemanager` provides `LeadershipCoordinator` + `Client` (HTTP to source-manager), and `services/source-manager/election` provides the Redis `SetNX` + Lua CAS backend. The kick-listener is the canonical integration example — it calls `leaderCoord.EnsureLeadership(ctx, streamID, lostCallback)` at startup and runs a heartbeat goroutine. Discord's Gateway RESUME protocol (op=6) is defined by Discord and requires `session_id`, `seq`, and `resume_gateway_url` — all three are already persisted to Redis by Phases 27-29 (`RedisKeySessionID`, `RedisKeyResumeURL`, `RedisKeySeq`). The main gaps for Phase 31 are: RESUME opcode handling in `GatewayClient.Connect()`, ownership gating in `cmd/main.go`, Prometheus metrics + `/metrics` endpoint in discord-listener, and Kubernetes manifests (Deployment, Service, HPA, ServiceMonitor).

**Primary recommendation:** Implement in three focused plans — (1) shard ownership + RESUME in gateway package, (2) metrics package + cmd/main.go wiring, (3) Kubernetes manifests.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| LOAD-01 | Gateway shard ownership coordinated via source-manager leader election — one pod holds each shard's connection | `shared/sourcemanager.LeadershipCoordinator` is the established mechanism; `discord:gateway:shard:0` is the stream ID; `cmd/main.go` has a TODO comment at the Gateway goroutine noting exactly this |
| LOAD-02 | discord-listener scales via HPA on Prometheus metrics (events/sec, active guilds) | `shared/metrics.ListenerMetrics` provides the standard metric types; HPA pattern established at `deployments/k8s/base/kick-listener/hpa.yaml`; `/metrics` + `promhttp.Handler()` is the project-wide pattern |
| LOAD-03 | Gateway session state (session_id + resume_gateway_url) persisted in Redis so pod restarts resume instead of re-IDENTIFY | Redis keys `RedisKeySessionID`, `RedisKeyResumeURL`, `RedisKeySeq` are written on READY; RESUME opcode (op=6) is not yet sent; `GatewayClient.Connect()` always sends IDENTIFY today |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/redis/go-redis/v9` | v9.18.0 | Redis SET/GET for session state + SetNX for shard lock | Already in discord-listener go.mod; used by SessionStore |
| `github.com/prometheus/client_golang` | latest in shared | Prometheus metrics exposition | Established project-wide; `shared/metrics` package |
| `github.com/caesar/all-chat/shared/sourcemanager` | local | Leadership claim/renew/release via source-manager API | Used by kick-listener, tiktok-listener; full heartbeat coordinator |
| `github.com/prometheus/client_golang/prometheus/promhttp` | latest | `promhttp.Handler()` for `/metrics` endpoint | Pattern from every listener service in project |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/prometheus/client_golang/prometheus/promauto` | latest | Auto-registering metrics | All project metric packages use promauto |
| `autoscaling/v2` HPA | Kubernetes 1.23+ | CPU/memory + custom metric autoscaling | All listener HPA manifests use v2 |
| `monitoring.coreos.com/v1` ServiceMonitor | kube-prometheus | Prometheus scrape config | ServiceMonitor pattern in deployments/k8s/monitoring |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| source-manager LeadershipCoordinator | Direct Redis SetNX in discord-listener | source-manager is the architectural choice; direct Redis avoids HTTP hop but breaks the coordination contract; source-manager is the right choice per project decisions |
| HPA on custom Prometheus metrics | HPA on CPU/memory only | CPU/memory is simpler but doesn't scale on Discord event volume; project uses CPU+memory HPA for other listeners (kick, twitch) which is acceptable for discord-listener at v1.5 scale |

**Installation:** No new dependencies needed. Add `github.com/caesar/all-chat/shared/sourcemanager` and `github.com/caesar/all-chat/shared/metrics` as `replace` directives in discord-listener's go.mod (following kick-listener pattern), plus `github.com/prometheus/client_golang`.

## Architecture Patterns

### Recommended Project Structure
```
services/discord-listener/
├── cmd/main.go             # Wire ownership gate + metrics + /metrics endpoint
├── gateway/
│   ├── client.go           # Add RESUME opcode send + OpResumed handler
│   └── types.go            # Add OpResume = 6, OpResumed = 7, ResumeData type
├── metrics/
│   └── metrics.go          # New: discord-specific Prometheus metrics
└── deployments/k8s/base/discord-listener/
    ├── deployment.yaml     # New: standard service deployment
    ├── service.yaml        # New: ClusterIP service on port 8086
    ├── hpa.yaml            # New: HPA CPU 70% / memory 80%
    └── servicemonitor.yaml # New: Prometheus scrape config
```

### Pattern 1: Shard Ownership Gating (LeadershipCoordinator)
**What:** Before starting `gwClient.Connect()`, call `leaderCoord.EnsureLeadership(ctx, "shard:0", lostCallback)`. Block the reconnect loop until ownership is acquired. On leadership loss, close the current connection and re-enter the wait loop.
**When to use:** Every discord-listener pod startup. Gateway goroutine in `cmd/main.go`.
**Example (derived from kick-listener/cmd/main.go):**
```go
// In cmd/main.go Gateway goroutine:
for {
    select {
    case <-ctx.Done():
        return
    default:
    }

    // Gate on shard ownership
    acquired, err := leaderCoord.EnsureLeadership(ctx, "shard:0", func() {
        log.Warn("Lost gateway shard ownership, disconnecting")
        gwClient.Close()
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

    log.Info("Acquired shard ownership, connecting to Gateway")
    if err := gwClient.Connect(ctx); err != nil && ctx.Err() == nil {
        log.Warn("Gateway disconnected, reconnecting in 5s", zap.Error(err))
        select {
        case <-time.After(5 * time.Second):
        case <-ctx.Done():
            return
        }
    }
}
```

### Pattern 2: Gateway RESUME (Discord op=6)
**What:** On connect, check Redis for existing `session_id` and `seq`. If present, send op=6 RESUME instead of op=2 IDENTIFY. Discord replies with RESUMED dispatch (no new READY) or op=9 InvalidSession (fall back to IDENTIFY).
**When to use:** Every `Connect()` call after initial startup.
**Example:**
```go
// In gateway/client.go Connect(), after HELLO:
case OpHello:
    go c.heartbeatLoop(ctx, hello.HeartbeatInterval)

    // Attempt RESUME if session state exists
    sessionID, _ := c.store.Get(ctx, RedisKeySessionID)
    resumeURL, _ := c.store.Get(ctx, RedisKeyResumeURL)
    seqStr, _ := c.store.Get(ctx, RedisKeySeq)

    if sessionID != "" && resumeURL != "" {
        seq, _ := strconv.Atoi(seqStr)
        resumePayload := BuildResumePayload(c.token, sessionID, seq)
        if err := c.writeJSON(resumePayload); err != nil {
            return fmt.Errorf("failed to send RESUME: %w", err)
        }
        c.log.Info("Sent RESUME", zap.String("session_id", sessionID), zap.Int("seq", seq))
    } else {
        identify := BuildIdentifyPayload(c.token)
        if err := c.writeJSON(identify); err != nil {
            return fmt.Errorf("failed to send IDENTIFY: %w", err)
        }
    }
```

Discord RESUME payload (op=6):
```go
// In gateway/types.go
const OpResume  = 6
const OpResumed = 7  // Dispatch t="RESUMED"

type ResumeData struct {
    Token     string `json:"token"`
    SessionID string `json:"session_id"`
    Seq       int    `json:"seq"`
}

func BuildResumePayload(token, sessionID string, seq int) GatewayPayload {
    d, _ := json.Marshal(ResumeData{Token: token, SessionID: sessionID, Seq: seq})
    return GatewayPayload{Op: OpResume, D: json.RawMessage(d)}
}
```

On `OpInvalidSession` after a RESUME attempt, clear Redis session state and fall back to IDENTIFY:
```go
case OpInvalidSession:
    // Clear stale session state so next connect does IDENTIFY
    _ = c.store.Set(ctx, RedisKeySessionID, "")
    _ = c.store.Set(ctx, RedisKeyResumeURL, "")
    _ = c.store.Set(ctx, RedisKeySeq, "")
    c.log.Warn("Gateway invalidated session — will re-IDENTIFY on next connect")
    return fmt.Errorf("gateway invalid session")
```

### Pattern 3: Discord Metrics Package
**What:** Create `services/discord-listener/metrics/metrics.go` following the kick-listener pattern with `promauto` counters/gauges relevant to Discord.
**When to use:** Wire in `cmd/main.go`, increment from `GatewayClient`.
**Key metrics for HPA:**
```go
// Source: shared/metrics/listener.go pattern + kick-listener/metrics/metrics.go
var (
    // Tracks total gateway events — rate of this metric drives HPA
    gatewayEventsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "discord_listener_gateway_events_total",
        Help: "Total Gateway dispatch events received",
    }, []string{"type"})

    // Active guilds — useful for HPA threshold
    activeGuildsGauge = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "discord_listener_active_guilds",
        Help: "Number of guilds with at least one configured source",
    })

    shardOwnershipGauge = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "discord_listener_shard_ownership",
        Help: "1 if this pod holds shard ownership, 0 otherwise",
    })

    resumeAttemptsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "discord_listener_resume_attempts_total",
        Help: "Gateway RESUME attempts",
    }, []string{"result"}) // "success" | "fallback_identify"
)
```

### Pattern 4: Kubernetes HPA (CPU + Memory)
**What:** HPA targeting CPU 70% and memory 80%, consistent with kick-listener and twitch-listener HPAs.
**When to use:** Production deployment where pod count should scale under load.
**Note:** At v1.5 scale (single shard, far below 2500-guild limit), CPU/memory HPA is sufficient. The shard ownership lock ensures only one pod actually holds the Discord connection; additional pods stand by waiting for ownership. This is intentional — the "scale" here is for fault tolerance (quick failover) not true horizontal throughput scaling.

```yaml
# deployments/k8s/base/discord-listener/hpa.yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: discord-listener
  namespace: allchat
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: discord-listener
  minReplicas: 1
  maxReplicas: 3
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 30
      policies:
      - type: Pods
        value: 1
        periodSeconds: 30
    scaleDown:
      stabilizationWindowSeconds: 120
      policies:
      - type: Pods
        value: 1
        periodSeconds: 60
```

### Anti-Patterns to Avoid
- **Connecting before acquiring ownership:** The current `cmd/main.go` reconnect loop has a TODO note — connecting before `EnsureLeadership` means two pods will both connect during HPA scale-up, violating LOAD-01.
- **Sending IDENTIFY when RESUME is possible:** Every full IDENTIFY counts against Discord's identify rate limit (1 per 5 seconds for small bots) and causes a full session replay. Always attempt RESUME first.
- **Clearing session state on every disconnect:** Op=7 (Reconnect) and transient errors should preserve session state. Only clear on op=9 (InvalidSession) with `d: false` (the `d` field being `false` means do not attempt RESUME).
- **Not wiring the `/metrics` endpoint:** The HPA requires Prometheus scraping. Without `promhttp.Handler()` on `/metrics`, custom metric HPAs can never trigger.
- **Using leaderCoord with empty streamID:** Stream ID for Discord Gateway shard 0 should be `"shard:0"` — consistent with the Redis key schema `discord:gateway:shard:0:*`.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Redis leadership lock with heartbeat | Custom SetNX loop in discord-listener | `shared/sourcemanager.LeadershipCoordinator` | Handles heartbeat, TTL renewal, consecutive failure threshold, lostCallback, Prometheus metrics — 300 LOC already tested |
| HTTP client to source-manager | Custom HTTP calls | `shared/sourcemanager.Client` | JWT signing, retry, error decode all handled |
| Prometheus metric registration | Manual `prometheus.NewXxx` + `prometheus.MustRegister` | `promauto.NewXxx` | Auto-registers on package init; project-wide convention |
| Gateway session RESUME logic | Custom protocol buffer management | Discord spec op=6 with existing `SessionStore` interface | Redis keys already written by Phase 27 `HandleReady` |

**Key insight:** All infrastructure for this phase exists. The work is wiring existing pieces (`LeadershipCoordinator`, `SessionStore`, `promhttp`) into discord-listener, not building new infrastructure.

## Common Pitfalls

### Pitfall 1: InvalidSession `d` Field Semantics
**What goes wrong:** Discord's op=9 InvalidSession includes a boolean `d` field. `d: true` means the session can be resumed after a short delay; `d: false` means it cannot and requires a fresh IDENTIFY.
**Why it happens:** The current `GatewayClient` ignores the `d` field — it just returns an error. Phase 31 RESUME logic must parse `d` to decide whether to clear session state.
**How to avoid:** Add `ResumeableField bool` to InvalidSession parsing. If `d: false`, delete the three Redis session keys. If `d: true`, the session may be resumable after 1-5 seconds.
**Warning signs:** Pod restarting repeatedly with "gateway invalid session" after RESUME attempts.

### Pitfall 2: Reconnect Opcode (op=7) Must Preserve Session State
**What goes wrong:** Discord sends op=7 (Reconnect) to ask the client to immediately close and reconnect using the existing session. If the handler clears session state, the reconnect will IDENTIFY instead of RESUME.
**Why it happens:** The current handler `return fmt.Errorf("gateway reconnect requested")` is correct — it returns an error which triggers the reconnect loop — but the session state in Redis is not cleared (correct). This is fine as-is; just don't add session-clearing logic to the op=7 handler.
**How to avoid:** Document the invariant: op=7 preserves Redis state, op=9 clears it (conditionally).

### Pitfall 3: Leadership Heartbeat TTL vs. Gateway Reconnect Window
**What goes wrong:** If the leadership TTL (10s default) expires during a Gateway reconnect (5s backoff), the standby pod may acquire ownership and start its own IDENTIFY before the primary recovers.
**Why it happens:** The reconnect loop in `cmd/main.go` has `time.After(5s)` backoff. The `LeadershipCoordinator` heartbeat interval is 5s and TTL is 10s. A pod that fails its heartbeat twice in a row (10s) loses ownership.
**How to avoid:** The existing `consecutiveFailures` threshold (2 failures = 10s grace period) in `shared/sourcemanager/coordinator.go` is already correct. The 5s reconnect backoff + 5s heartbeat interval means a crashed pod will lose ownership in ~10-15s, well within the 60s SLO from LOAD-03 success criterion 3.
**Warning signs:** Two pods logging "Acquired shard ownership" within a short window.

### Pitfall 4: go.mod Replace Directives for Shared Packages
**What goes wrong:** discord-listener's `go.mod` does not currently reference `shared/sourcemanager` or `shared/metrics`. Simply adding imports without adding `replace` directives and `require` entries will fail `go build`.
**Why it happens:** All services use local `replace` directives (e.g., `replace github.com/caesar/all-chat/shared => ../../shared`). Check kick-listener's `go.mod` for the exact pattern.
**How to avoid:** Add the replace directive and run `go mod tidy` before writing implementation code.
**Warning signs:** `go: module not found` or `cannot find module providing package` build errors.

### Pitfall 5: Kubernetes Deployment Missing Service for HPA
**What goes wrong:** HPA requires a Service manifest for scraping. Without a Service, the HPA can't discover pods.
**Why it happens:** The Kubernetes manifests for discord-listener don't exist yet.
**How to avoid:** Create deployment + service + hpa in one plan; add to kustomization.yaml.

## Code Examples

Verified patterns from official sources:

### Discord Gateway RESUME (op=6) — Discord Developer Docs
```go
// Source: Discord Gateway documentation — Resuming a connection
// The d field of a RESUME payload:
type ResumeData struct {
    Token     string `json:"token"`
    SessionID string `json:"session_id"`
    Seq       int    `json:"seq"`
}
// Op=6: Client sends RESUME
// Op=7 (Reconnect): Server asks client to reconnect — preserve session state
// Op=9 (InvalidSession): d=false → IDENTIFY; d=true → wait 1-5s then RESUME
```

### LeadershipCoordinator Integration (from kick-listener)
```go
// Source: services/kick-listener/cmd/main.go lines 153-165
tokenSource := sourcemanager.NewSigningTokenSource("discord-listener", sourceManagerSecret, 15*time.Minute)
smClient, err := sourcemanager.NewClient(sourceManagerURL, tokenSource)
if err != nil {
    log.Fatal("Failed to initialize Source Manager client", zap.Error(err))
}
leaderCoord = sourcemanager.NewLeadershipCoordinator("discord", smClient, 5*time.Second, log)
```

### Prometheus /metrics endpoint (from kick-listener)
```go
// Source: services/kick-listener/cmd/main.go
router.GET("/metrics", gin.WrapH(promhttp.Handler()))
```

### go.mod replacement pattern (reference kick-listener go.mod for exact version)
```
require (
    github.com/caesar/all-chat/shared v0.0.0
    github.com/prometheus/client_golang vX.Y.Z
)
replace github.com/caesar/all-chat/shared => ../../shared
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Each pod connects independently (Phase 27-30 behavior) | One pod holds shard ownership via leadership lock | Phase 31 | Prevents double-connect; enables safe multi-pod deployment |
| IDENTIFY on every connect | RESUME if session state exists | Phase 31 | Avoids session replay cost; faster reconnect after pod restart |
| No Prometheus metrics | `discord_listener_gateway_events_total`, `discord_listener_active_guilds`, `discord_listener_shard_ownership` | Phase 31 | Enables observability and HPA threshold decisions |

**Deprecated/outdated:**
- Direct connection without ownership check (the TODO comment in `cmd/main.go` line 165): replaced by `leaderCoord.EnsureLeadership()` gate.

## Open Questions

1. **InvalidSession `d: true` handling**
   - What we know: Discord says wait 1-5s then attempt RESUME again
   - What's unclear: Whether to implement the retry within `Connect()` or let the outer reconnect loop handle it (simpler)
   - Recommendation: Let the outer reconnect loop handle it — just return an error without clearing session state; the 5s backoff covers the wait window

2. **source-manager availability**
   - What we know: source-manager is a dependency for leadership; if it's down, `EnsureLeadership` returns error
   - What's unclear: Current behavior on source-manager downtime — kick-listener logs WARN and skips leadership, which means the pod connects anyway (degraded but functional)
   - Recommendation: Follow the same pattern as kick-listener: if `SOURCE_MANAGER_SECRET` is empty or source-manager is unreachable, log WARN and connect without ownership gating (graceful degradation)

3. **Active guild count metric**
   - What we know: LOAD-02 mentions "active guild count" as a scaling threshold
   - What's unclear: No in-memory guild set exists; counting active guilds requires a Redis SCAN on `discord:channels:*` keys
   - Recommendation: Implement a periodic goroutine (every 30s) that counts distinct guild IDs from registered channel sources; this is a background gauge update, not per-message

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) + testify v1.11.1 |
| Config file | none — `go test ./...` |
| Quick run command | `cd services/discord-listener && go test ./gateway/... -run TestResume -v` |
| Full suite command | `cd services/discord-listener && go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| LOAD-01 | Pod without lock does not call Connect | unit | `go test ./gateway/... -run TestShardOwnership` | No — Wave 0 |
| LOAD-01 | Pod acquires lock within 60s after prior pod terminates | integration | Manual / Kubernetes test | Manual only — live K8s needed |
| LOAD-02 | /metrics endpoint returns discord_listener_gateway_events_total | unit | `go test ./metrics/... -run TestMetricsEndpoint` | No — Wave 0 |
| LOAD-03 | RESUME opcode sent when session_id present in Redis | unit | `go test ./gateway/... -run TestResumeOnReconnect` | No — Wave 0 |
| LOAD-03 | Session cleared when op=9 InvalidSession d=false | unit | `go test ./gateway/... -run TestInvalidSessionClearsState` | No — Wave 0 |
| LOAD-03 | Session preserved when op=7 Reconnect | unit | `go test ./gateway/... -run TestReconnectPreservesSession` | No — Wave 0 |

### Sampling Rate
- **Per task commit:** `cd services/discord-listener && go test ./gateway/... ./metrics/...`
- **Per wave merge:** `cd services/discord-listener && go test ./...`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `services/discord-listener/gateway/resume_test.go` — covers LOAD-03 RESUME + InvalidSession + Reconnect opcode behavior
- [ ] `services/discord-listener/metrics/metrics_test.go` — covers LOAD-02 metric registration
- [ ] No framework install needed — existing `go test` infrastructure covers all phase requirements

## Sources

### Primary (HIGH confidence)
- `services/discord-listener/gateway/client.go` + `types.go` — existing READY/session persistence code; confirmed RESUME not yet implemented
- `services/discord-listener/cmd/main.go` lines 163-183 — TODO comment explicitly names the Phase 31 ownership gate pattern and the Redis lock key
- `shared/sourcemanager/coordinator.go` — `LeadershipCoordinator` full implementation; heartbeat, lostCallback, Prometheus metrics
- `shared/sourcemanager/client.go` — `ClaimLeadership`, `RenewLeadership`, `ReleaseLeadership` HTTP methods
- `services/kick-listener/cmd/main.go` lines 153-165 — canonical `LeadershipCoordinator` integration pattern
- `services/kick-listener/metrics/metrics.go` — canonical `promauto` metrics package pattern
- `deployments/k8s/base/kick-listener/hpa.yaml` — HPA `autoscaling/v2` template with behavior section
- Discord Gateway documentation — op=6 RESUME, op=7 Reconnect, op=9 InvalidSession semantics (HIGH — well-established protocol)

### Secondary (MEDIUM confidence)
- `shared/metrics/listener.go` — `ListenerMetrics` struct with standard metric names; discord-listener metrics should follow the same naming convention
- `services/source-manager/election/leader.go` — source-manager backend for leadership (confirms 10s TTL, 5s heartbeat interval)
- `deployments/k8s/monitoring/servicemonitor-innertube.yaml` — ServiceMonitor template for Prometheus operator

### Tertiary (LOW confidence)
- None

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries are already in the project; no new external dependencies
- Architecture: HIGH — kick-listener is a directly applicable reference implementation; Discord RESUME protocol is stable
- Pitfalls: HIGH — derived from existing code analysis + Discord protocol documentation

**Research date:** 2026-03-16
**Valid until:** 2026-04-16 (stable domain; Discord Gateway protocol and shared packages are stable)
