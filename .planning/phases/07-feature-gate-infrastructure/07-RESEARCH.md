# Phase 7: Feature Gate Infrastructure - Research

**Researched:** 2026-03-29
**Domain:** Feature flagging, Redis Pub/Sub invalidation, Go in-memory cache, Admin UI, PostgreSQL migrations
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** New `feature_gates` Postgres table (feature_key VARCHAR PK, is_premium BOOLEAN, description TEXT) as the source of truth
- **D-02:** Follows the existing `supported_platforms.is_enabled` pattern — database-driven, no hardcoded environment toggles
- **D-03:** Start minimal — only gate features that currently check `users.is_premium` (day-one: `sharing`)
- **D-04:** Future experimental features get a row added when they ship — the table grows organically
- **D-05:** Keep `supported_platforms` separate — it serves platform availability, not monetization
- **D-06:** Cosmetics keep existing per-item `is_premium` flag unchanged — already supports the desired model
- **D-07:** DB as source of truth + Redis Pub/Sub invalidation for instant propagation
- **D-08:** Each service holds an in-memory map of feature gates, refreshed via Redis Pub/Sub subscription
- **D-09:** Periodic TTL refresh (60s) as fallback for missed Pub/Sub messages
- **D-10:** Zero DB hits at request time after boot — all checks against in-memory map
- **D-11:** Rewrite `RequirePremium` middleware to: check in-memory gate → if feature is_premium=false, allow everyone → if is_premium=true, check `users.is_premium` as today
- **D-12:** Admin panel UI at `/admin/features` with toggle switches per feature
- **D-13:** Backed by API endpoint (PATCH /api/v1/admin/feature-gates/:key)
- **D-14:** Toggle publishes invalidation event to Redis Pub/Sub so all services pick up the change instantly
- **D-15:** Feature gate `is_premium=false` overrides user's `is_premium` check — feature is free for everyone
- **D-16:** Feature gate `is_premium=true` falls through to existing `users.is_premium` check — premium users only

### Claude's Discretion

None specified — all decisions are locked.

### Deferred Ideas (OUT OF SCOPE)

- Percentage-based rollout / gradual feature flagging
- Absorbing `supported_platforms.is_enabled` into `feature_gates`
- Deprecating or removing `users.is_premium` column
</user_constraints>

---

## Summary

Phase 7 adds a lightweight feature gate layer on top of the existing per-user `is_premium` check. The core pattern is: a `feature_gates` Postgres table stores whether each feature is premium-gated, each service boots an in-memory cache of that table, and a Redis Pub/Sub channel (`feature-gates:invalidate`) propagates changes from the admin toggle endpoint instantly.

The implementation touches four areas: (1) a new DB migration for `feature_gates`, (2) a reusable `FeatureGateCache` component living in `shared/` or inline in share-service (only share-service currently needs it), (3) a rewrite of `RequirePremium` middleware in share-service to consult the gate cache before falling through to the user-level check, (4) an admin PATCH endpoint and UI page modeled on the existing cosmetics admin patterns.

No new service is required. The feature gate infrastructure is self-contained. The ADR index currently runs 0001–0007; this phase warrants ADR-0008 documenting the "ship premium, graduate to free" lifecycle design (architecturally significant, multiple viable alternatives existed).

**Primary recommendation:** Implement the `FeatureGateCache` as a standalone struct in `services/share-service/featuregates/` (not `shared/`) since it is only consumed by share-service today. When a second service needs it, move to `shared/`. This avoids premature abstraction.

---

## Standard Stack

### Core (already in repo — no new installs needed)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/redis/go-redis/v9` | v9.18.0 | Redis Pub/Sub subscription + publish | Already in share-service go.mod |
| `github.com/jackc/pgx/v5` | v5.9.1 | DB load of `feature_gates` on boot | Already in share-service go.mod |
| `github.com/gin-gonic/gin` | v1.12.0 | PATCH admin endpoint | Already in share-service go.mod |
| `sync.RWMutex` | stdlib | Protect in-memory gate map | Established pattern (demand/subscriber.go) |
| `time.Ticker` | stdlib | 60s fallback refresh | Established pattern (websocket/manager.go) |
| React 19 / Next.js | project version | Admin page | Existing frontend stack |

**Installation:** No new dependencies required for either Go services or frontend.

---

## Architecture Patterns

### Recommended Structure (new files)

```
services/share-service/
├── featuregates/
│   ├── cache.go            # FeatureGateCache struct: in-memory map + Pub/Sub + TTL refresh
│   └── cache_test.go       # Unit tests with mock redis
├── middleware/
│   └── premium.go          # REWRITE: gate-aware RequirePremium
└── handlers/
    └── admin_featuregates.go  # PATCH /api/v1/admin/feature-gates/:key

migrations/
└── 044_feature_gates.sql   # CREATE TABLE feature_gates + seed sharing row

services/api-gateway/cmd/main.go
│   # ADD: PATCH /admin/feature-gates/:key route proxy to share-service

frontend/src/app/admin/features/
└── page.tsx                # Admin toggle page

frontend/src/components/AdminNav.tsx
│   # ADD: { href: '/admin/features', label: 'Features' } to ADMIN_LINKS
```

### Pattern 1: FeatureGateCache (in-memory map with Pub/Sub + TTL)

**What:** A struct holding `map[string]bool` (feature_key → is_premium), protected by `sync.RWMutex`. Loads from DB on `Start()`, refreshes every 60s via ticker, and subscribes to `feature-gates:invalidate` Pub/Sub to reload immediately on change.

**When to use:** Any service that needs zero-latency feature gate checks at request time.

**Example — struct layout (modeled on demand/subscriber.go):**

```go
// services/share-service/featuregates/cache.go
package featuregates

import (
    "context"
    "sync"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/redis/go-redis/v9"
    "go.uber.org/zap"
)

const PubSubChannel = "feature-gates:invalidate"
const refreshInterval = 60 * time.Second

type FeatureGateCache struct {
    db     *pgxpool.Pool
    redis  *redis.Client
    logger *zap.Logger

    mu    sync.RWMutex
    gates map[string]bool // feature_key -> is_premium
}

func NewFeatureGateCache(db *pgxpool.Pool, rc *redis.Client, logger *zap.Logger) *FeatureGateCache {
    return &FeatureGateCache{
        db:    db,
        redis: rc,
        logger: logger,
        gates: make(map[string]bool),
    }
}

// IsPremium returns true if the feature is premium-gated.
// Returns true (safe default) if the feature key is unknown (gate not yet seeded).
func (c *FeatureGateCache) IsPremium(key string) bool {
    c.mu.RLock()
    defer c.mu.RUnlock()
    val, ok := c.gates[key]
    if !ok {
        return true // unknown gate → default to premium-gated (safe)
    }
    return val
}
```

**Safe default on unknown key:** If a service boots before migration 044 runs (during rolling deploy), the gate is unknown. Returning `true` preserves existing behaviour — the user-level `is_premium` check still fires. This is the correct fail-safe.

### Pattern 2: Rewritten RequirePremium Middleware

**What:** Adds a gate-key parameter so the same middleware factory can be reused when additional features are added.

**Example:**

```go
// services/share-service/middleware/premium.go
func RequirePremium(db *pgxpool.Pool, gates *featuregates.FeatureGateCache, featureKey string, logger *zap.Logger) gin.HandlerFunc {
    return func(c *gin.Context) {
        // D-15/D-16: check gate first
        if !gates.IsPremium(featureKey) {
            // Feature is free for everyone — skip user check
            c.Next()
            return
        }
        // Gate is premium-gated → check user.is_premium as before
        userID := c.GetString("user_id")
        // ... existing DB query logic unchanged ...
    }
}
```

**Callsite update in cmd/main.go:**

```go
premiumRoutes.Use(localMiddleware.RequirePremium(dbPool, gateCache, "sharing", log))
```

### Pattern 3: PATCH Admin Endpoint

**What:** Single handler in share-service that updates `feature_gates.is_premium` and publishes to Redis Pub/Sub. Modeled on `admin_cosmetics.go` pattern (direct DB, admin role check already enforced by `AdminOnly()` shared middleware).

**Example:**

```go
// PATCH /api/v1/admin/feature-gates/:key
type updateFeatureGateRequest struct {
    IsPremium bool `json:"is_premium" binding:"required"`
}

func (h *AdminFeatureGatesHandler) UpdateGate(c *gin.Context) {
    key := c.Param("key")
    var req updateFeatureGateRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    // Update DB
    // Publish invalidation
    h.redis.Publish(c, featuregates.PubSubChannel, key)
    c.JSON(200, ...)
}
```

**Note on `binding:"required"` with bool:** `binding:"required"` on a `bool` field will reject `false` as "missing" in older gin versions. Use `binding:"-"` and validate manually, or use `*bool` pointer type. This is a known gin gotcha.

### Pattern 4: Admin UI Toggle Page

**What:** React page at `/admin/features` following the `admin/cosmetics` component structure (`AdminNav`, `apiClient`, `Card`, `Button`, `toastManager`). Fetches gate list via `GET /api/v1/admin/feature-gates`, toggles via `PATCH /api/v1/admin/feature-gates/:key`.

**UI state shape:**

```typescript
interface FeatureGate {
  feature_key: string
  is_premium: boolean
  description: string
}
```

Toggle switches: use a `<input type="checkbox">` or inline toggle styled consistently with existing admin pages. The cosmetics page does not have toggles in-place, but the users page has the pattern of optimistic UI updates + refetch.

### Pattern 5: Redis Pub/Sub Invalidation Flow

**What:** When admin POSTes a toggle, share-service publishes `feature-gates:invalidate` with the `feature_key` as payload. The `FeatureGateCache` subscriber receives this and calls `reload()` from DB.

**Established reference:** `services/share-service/jobs/lifecycle_subscriber.go` — exact same pattern: `redis.Subscribe(ctx, channel)` → `pubsub.Channel()` → goroutine `for { select { case msg: ... } }`.

**Channel name:** `feature-gates:invalidate` — namespaced to avoid collision with `overlay:*`, `lifecycle:*`, `source:demand`.

### Pattern 6: GET Endpoint for Gate List

**What:** `GET /api/v1/admin/feature-gates` returns all rows from `feature_gates`. Needed by the admin page on load.

```go
// SELECT feature_key, is_premium, description FROM feature_gates ORDER BY feature_key
```

### Anti-Patterns to Avoid

- **Checking gate per-request from DB:** Zero DB hits at request time is D-10. Gate is always read from in-memory map.
- **Single global gate variable:** Use a struct with a method so tests can inject a mock or a pre-populated cache.
- **Putting `FeatureGateCache` in `shared/` now:** Only share-service uses it today. Premature move adds shared module coupling cost. Move when a second service needs it.
- **Using `binding:"required"` on `bool` fields:** Gin rejects `false` as missing. Use `*bool` pointer or manual validation.
- **Hardcoded gate keys as magic strings in middleware call sites:** Define `const GateSharing = "sharing"` in the `featuregates` package to make keys discoverable via grep.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Redis Pub/Sub subscription | Custom goroutine select loop | `redis.Client.Subscribe()` + `.Channel()` | Already established in lifecycle_subscriber.go |
| Thread-safe in-memory map | Custom locking | `sync.RWMutex` + plain `map` | Established pattern (demand/subscriber.go) |
| Periodic refresh | Custom timer abstraction | `time.NewTicker(60 * time.Second)` | Established pattern (websocket/manager.go) |
| Admin role enforcement | Per-handler role check | `middleware.AdminOnly()` from `shared/middleware/auth.go` | Used by all admin routes in share-service already |

---

## Common Pitfalls

### Pitfall 1: `binding:"required"` rejects `false` for bool fields in Gin
**What goes wrong:** `PATCH /api/v1/admin/feature-gates/:key` with body `{"is_premium": false}` returns 400 "Field validation for 'IsPremium' failed on the 'required' tag".
**Why it happens:** Gin's validator treats the zero value of `bool` (which is `false`) as "not provided" for `required`.
**How to avoid:** Use `*bool` pointer type in the request struct. `nil` = not provided, non-nil = value given.
**Warning signs:** Any admin toggle that sets a boolean to `false` silently fails.

### Pitfall 2: Unknown gate key defaults matter on rolling deploy
**What goes wrong:** If migration 044 hasn't run yet on the DB but the updated share-service pod starts, `IsPremium("sharing")` returns the zero value (depending on implementation). If zero = `false`, the sharing feature becomes free for everyone during the deploy window.
**Why it happens:** Code ahead of migration in rolling deploy.
**How to avoid:** Default `IsPremium()` to `true` for unknown keys (safe: preserves premium-only behaviour). Explicitly test this branch.
**Warning signs:** Unauthenticated users suddenly able to create share requests.

### Pitfall 3: Pub/Sub message missed at startup
**What goes wrong:** Service boots, subscribes to `feature-gates:invalidate`, but misses a toggle that happened during the subscription gap (between DB load and subscribe completing).
**Why it happens:** There is a window between `loadFromDB()` and `subscribe()` where a publish can be missed.
**How to avoid:** In `Start()`: (1) load from DB, (2) subscribe, (3) do NOT reload from DB after subscribe — the 60s TTL ticker will catch any gap. Alternatively, reload once immediately after subscribe confirm. The 60s TTL (D-09) is the explicit fallback for exactly this case.
**Warning signs:** Gate state is stale for up to 60s after an admin toggle (acceptable per D-09).

### Pitfall 4: Admin endpoint mounted at wrong service
**What goes wrong:** `PATCH /api/v1/admin/feature-gates/:key` implemented in auth-service instead of share-service, causing the share-service's in-memory cache to never hear its own publish.
**Why it happens:** Auth-service already owns many admin routes. Tempting to add here.
**How to avoid:** Feature gates are owned by share-service (the only current consumer). The handler must live in share-service so it can access the cache and publish directly. Auth-service has no Redis publish-to-invalidate logic.
**Warning signs:** Admin toggle succeeds (200 OK) but share-service behaviour doesn't change.

### Pitfall 5: API Gateway proxy route missing
**What goes wrong:** Frontend calls `PATCH /api/v1/admin/feature-gates/:key` — gateway returns 404 because no proxy rule exists.
**Why it happens:** New admin endpoint not added to `services/api-gateway/cmd/main.go`.
**How to avoid:** Add three routes to api-gateway: `GET /admin/feature-gates`, `PATCH /admin/feature-gates/:key`, and optionally `GET /admin/feature-gates/:key`. Follow existing pattern at line ~165 of api-gateway cmd/main.go.
**Warning signs:** Frontend gets 404 from the gateway.

### Pitfall 6: AdminNav missing Features link
**What goes wrong:** Admin navigates to `/admin/features` manually but there's no nav link; the page is inaccessible from the admin dashboard.
**Why it happens:** `ADMIN_LINKS` array in `AdminNav.tsx` not updated.
**How to avoid:** Add `{ href: '/admin/features', label: 'Features' }` to `ADMIN_LINKS`. The admin layout is shared — one edit covers all admin pages.

---

## Code Examples

### Migration 044: feature_gates table

```sql
-- migrations/044_feature_gates.sql
-- Migration: 044
-- Description: Create feature_gates table for capability-level premium toggling

CREATE TABLE IF NOT EXISTS feature_gates (
    feature_key VARCHAR(100) PRIMARY KEY,
    is_premium  BOOLEAN NOT NULL DEFAULT TRUE,
    description TEXT NOT NULL DEFAULT '',
    updated_at  TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Trigger for updated_at (matches existing pattern from migration 001)
CREATE TRIGGER update_feature_gates_updated_at
    BEFORE UPDATE ON feature_gates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Seed day-one gate: sharing feature
INSERT INTO feature_gates (feature_key, is_premium, description)
VALUES ('sharing', TRUE, 'Overlay share requests — allows users to create and accept chat overlay shares')
ON CONFLICT (feature_key) DO NOTHING;

COMMENT ON TABLE feature_gates IS 'Capability-level premium feature flags. is_premium=false means feature is free for all users.';
COMMENT ON COLUMN feature_gates.feature_key IS 'Unique feature identifier, used as the gate key in middleware calls';
COMMENT ON COLUMN feature_gates.is_premium IS 'When true, user must have is_premium=true to access. When false, all authenticated users may access.';
```

**Down migration (044_feature_gates_down.sql):**

```sql
DROP TABLE IF EXISTS feature_gates;
```

### FeatureGateCache.Start() flow

```go
func (c *FeatureGateCache) Start(ctx context.Context) error {
    // 1. Load from DB (before subscribe to avoid empty-cache window)
    if err := c.reload(ctx); err != nil {
        return fmt.Errorf("initial feature gate load: %w", err)
    }

    // 2. Subscribe to invalidation channel
    pubsub := c.redis.Subscribe(ctx, PubSubChannel)
    if _, err := pubsub.Receive(ctx); err != nil {
        return fmt.Errorf("subscribe to feature-gates:invalidate: %w", err)
    }

    // 3. Background: handle invalidations + periodic refresh
    go c.run(ctx, pubsub)
    return nil
}

func (c *FeatureGateCache) run(ctx context.Context, pubsub *redis.PubSub) {
    defer pubsub.Close()
    ticker := time.NewTicker(refreshInterval)
    defer ticker.Stop()
    ch := pubsub.Channel()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            if err := c.reload(ctx); err != nil {
                c.logger.Warn("Periodic feature gate refresh failed", zap.Error(err))
            }
        case msg, ok := <-ch:
            if !ok {
                c.logger.Warn("feature-gates:invalidate channel closed")
                return
            }
            c.logger.Info("Feature gate invalidated, reloading", zap.String("key", msg.Payload))
            if err := c.reload(ctx); err != nil {
                c.logger.Error("Feature gate reload after invalidation failed", zap.Error(err))
            }
        }
    }
}

func (c *FeatureGateCache) reload(ctx context.Context) error {
    rows, err := c.db.Query(ctx, "SELECT feature_key, is_premium FROM feature_gates")
    if err != nil {
        return err
    }
    defer rows.Close()
    gates := make(map[string]bool)
    for rows.Next() {
        var key string
        var isPremium bool
        if err := rows.Scan(&key, &isPremium); err != nil {
            return err
        }
        gates[key] = isPremium
    }
    c.mu.Lock()
    c.gates = gates
    c.mu.Unlock()
    c.logger.Debug("Feature gates reloaded", zap.Int("count", len(gates)))
    return rows.Err()
}
```

### Admin handler publish pattern

```go
// After successful DB update:
if err := h.redis.Publish(c.Request.Context(), featuregates.PubSubChannel, key).Err(); err != nil {
    h.logger.Warn("Failed to publish feature gate invalidation",
        zap.String("key", key),
        zap.Error(err),
    )
    // Do NOT return error — DB is already updated, cache will refresh via TTL
}
```

### share-service cmd/main.go wiring

```go
// After Redis client creation:
gateCache := featuregates.NewFeatureGateCache(dbPool, redisClient, log)
if err := gateCache.Start(context.Background()); err != nil {
    log.Fatal("Failed to start feature gate cache", zap.Error(err))
}

// Admin handler:
adminFGHandler := handlers.NewAdminFeatureGatesHandler(dbPool, redisClient, log)

// Routes:
adminRoutes.GET("/feature-gates", adminFGHandler.ListGates)
adminRoutes.PATCH("/feature-gates/:key", adminFGHandler.UpdateGate)

// Premium middleware (updated call):
premiumRoutes.Use(localMiddleware.RequirePremium(dbPool, gateCache, featuregates.GateSharing, log))
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Direct DB query on every request in RequirePremium | In-memory gate cache + fallback TTL | Phase 7 | Zero DB hits at request time |
| Hardcoded "sharing is always premium" | Database-driven gate, togglable at runtime | Phase 7 | Feature can be promoted to free without deploys |

**Deprecated/outdated:**
- `RequirePremium(db, logger)` signature — Phase 7 rewrites to `RequirePremium(db, gates, featureKey, logger)`. The old direct-DB-per-request approach was explicitly noted as "MVP simplicity" in the existing middleware comment.

---

## Open Questions

1. **Context lifetime for `gateCache.Start()`**
   - What we know: share-service uses `context.Background()` for long-running jobs (see `expiryJob.Start(context.Background())`)
   - What's unclear: Should the cache use a cancellable context tied to OS signal for clean shutdown?
   - Recommendation: Use a context derived from a root cancel that fires in the shutdown sequence, consistent with how other background goroutines are stopped. The existing expiryJob does not do this, so `context.Background()` is acceptable for consistency.

2. **GET endpoint location: share-service or auth-service?**
   - What we know: All other admin GET endpoints are in auth-service (users, overlays, sources, viewers). Feature gates are different — they are owned by share-service (the writer).
   - What's unclear: Should `GET /admin/feature-gates` proxy to share-service or auth-service?
   - Recommendation: Keep read and write in share-service for ownership clarity. The API gateway proxy comment block documents which service each route hits — update that comment.

3. **ADR-0008 scope**
   - What we know: CLAUDE.md says architectural changes need ADRs. The "ship premium, graduate to free" lifecycle model is architecturally significant (affects future feature design, has viable alternatives like per-user feature flags, env vars, etc.).
   - Recommendation: Write ADR-0008 as part of this phase. Key alternatives to document: (a) env-var feature flags, (b) per-user feature flags (LaunchDarkly style), (c) hardcoded toggles. Decision: DB + cache chosen for runtime toggleability without deploys.

---

## Environment Availability

Step 2.6: SKIPPED — this phase adds code and configuration to existing services. No new external dependencies. Redis and PostgreSQL are already running and verified by prior phases.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go `testing` package + `github.com/stretchr/testify` v1.11.1 |
| Config file | none (standard `go test ./...`) |
| Quick run command | `cd services/share-service && go test ./featuregates/... ./middleware/... ./handlers/... -v -count=1` |
| Full suite command | `cd services/share-service && go test ./... -v -count=1` |

### Phase Requirements → Test Map

| Behavior | Test Type | Automated Command | Notes |
|----------|-----------|-------------------|-------|
| `IsPremium("sharing")` returns `true` when gate is seeded as premium | unit | `go test ./featuregates/... -run TestIsPremium` | Cache pre-populated |
| `IsPremium("sharing")` returns `false` after gate flipped to free | unit | `go test ./featuregates/... -run TestIsPremiumFree` | Cache pre-populated |
| `IsPremium("unknown-key")` returns `true` (safe default) | unit | `go test ./featuregates/... -run TestIsPremiumUnknownKey` | Safe default |
| RequirePremium allows non-premium user when gate is free | unit | `go test ./middleware/... -run TestRequirePremiumGateFree` | Mock gate |
| RequirePremium blocks non-premium user when gate is premium | unit | `go test ./middleware/... -run TestRequirePremiumGatePremium` | Mock gate |
| RequirePremium allows premium user when gate is premium | unit | `go test ./middleware/... -run TestRequirePremiumUserPremium` | Mock gate |
| `reload()` correctly maps DB rows to in-memory map | unit | `go test ./featuregates/... -run TestReload` | Mock pgxpool |
| Pub/Sub message triggers reload | unit | `go test ./featuregates/... -run TestInvalidationTrigger` | miniredis or mock |
| PATCH handler updates DB and publishes invalidation | unit | `go test ./handlers/... -run TestUpdateGate` | Mock redis publish |
| GET handler returns all gates | unit | `go test ./handlers/... -run TestListGates` | Mock DB |

### Wave 0 Gaps

- [ ] `services/share-service/featuregates/cache_test.go` — covers IsPremium, unknown-key default, reload, invalidation trigger
- [ ] `services/share-service/middleware/premium_test.go` — covers gate-free, gate-premium + user checks
- [ ] `services/share-service/handlers/admin_featuregates_test.go` — covers PATCH and GET handlers

*(No new framework install needed — testify already in go.mod)*

---

## Sources

### Primary (HIGH confidence)

- Direct code reading: `services/share-service/middleware/premium.go` — current RequirePremium (rewrite target)
- Direct code reading: `services/share-service/jobs/lifecycle_subscriber.go` — canonical Redis Pub/Sub subscription pattern in share-service
- Direct code reading: `services/api-gateway/subscription/subscriber.go` — canonical Redis Pub/Sub pattern (overlay channel)
- Direct code reading: `services/source-manager/demand/subscriber.go` — canonical `sync.RWMutex` + in-memory map pattern
- Direct code reading: `services/api-gateway/websocket/manager.go` — canonical `time.NewTicker` TTL refresh pattern
- Direct code reading: `migrations/001_initial_schema.sql` — `supported_platforms.is_enabled` (reference for DB-driven gates)
- Direct code reading: `migrations/030_share_requests.sql` — `users.is_premium` column origin
- Direct code reading: `services/share-service/cmd/main.go` — wiring pattern for middleware and route registration
- Direct code reading: `frontend/src/app/admin/users/page.tsx` — admin toggle UI pattern (Dialog + toastManager + refetch)
- Direct code reading: `frontend/src/components/AdminNav.tsx` — how to add new admin link
- Direct code reading: `services/api-gateway/cmd/main.go` — admin proxy route pattern
- Direct code reading: `shared/middleware/auth.go` — `AdminOnly()` middleware (used for admin route protection)

### Secondary (MEDIUM confidence)

- Gin bool binding gotcha: `binding:"required"` fails on `false` — well-documented community pattern, verified by code inspection of existing handlers using `*bool` pointers in similar contexts

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries already in use, no new dependencies
- Architecture: HIGH — all patterns traced to existing, working code in the repo
- Pitfalls: HIGH — all traced to concrete code paths or known Gin behaviour
- Migration: HIGH — follows exact format of migrations/001 and migrations/030

**Research date:** 2026-03-29
**Valid until:** 2026-04-29 (stable stack)
