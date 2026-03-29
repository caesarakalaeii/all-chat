# ADR-0008: Feature Gate Infrastructure

**Date**: 2026-03-29
Status: Accepted
**Deciders**: Engineering team

---

## Context and Problem Statement

All-Chat needs the ability to ship experimental features as premium-only, let the community test them, and flip them to free at any time without code deploys. Currently `users.is_premium` is checked directly in middleware — adding a new gated feature requires code changes and a deploy. The `sharing` feature was the first case where this limitation became concrete: we wanted to launch it as premium-only and later open it to all users based on adoption.

## Decision Drivers

- Zero-downtime toggles: flipping a gate must not require a pod restart
- No external dependencies: LaunchDarkly and similar tools add cost and operational complexity
- Per-service caching: DB hits at request time would add latency on every authenticated request
- Safe defaults: unknown gate keys must deny access, not grant it
- Discoverable: gate key strings must be typed constants, not magic strings scattered in code

## Considered Options

1. **Environment variable toggles**
   - Pros: Simple to implement, no DB schema changes
   - Cons: Requires pod restart to change; ConfigMap rollout is not instantaneous; cannot toggle individual features independently
   - **Rejected**: Requires deploy for every flag change

2. **Per-user feature flags (LaunchDarkly style)**
   - Pros: Granular targeting (A/B, percentage rollout, user cohorts)
   - Cons: External dependency, licensing cost, over-engineering for capability-level toggles
   - **Rejected**: Overkill; adds external dependency; all-or-nothing capability gates don't need per-user targeting

3. **Hardcoded constants in code**
   - Pros: Zero infra, fastest reads
   - Cons: Requires code change and deploy for every feature graduation
   - **Rejected**: Every toggle is a deploy; defeats the purpose of soft feature management

4. **`feature_gates` Postgres table + in-memory cache per service (chosen)**
   - Pros: DB is source of truth; zero request-time DB hits; Redis Pub/Sub provides near-instant propagation; 60s TTL fallback for network partitions; no external dependencies
   - Cons: Each service must boot a FeatureGateCache; brief staleness window (~60s worst case during partition)
   - **Chosen**: Balances simplicity, correctness, and operational safety

## Decision Outcome

**Chosen**: Option 4 — `feature_gates` Postgres table with in-memory cache per service.

**Rationale**: The system already uses Redis Pub/Sub for lifecycle events (see `lifecycle_subscriber.go`). Adding a `feature_gates` table extends the existing pattern at near-zero marginal complexity. The cache model is identical to the source-manager demand subscriber pattern already in production. No external dependencies are introduced.

## Consequences

### Positive

- Feature lifecycle becomes "ship premium → graduate to free" with zero-downtime toggles
- Adding a new gated feature requires only a migration `INSERT` and a constant in Go code — no middleware change
- Unknown gate keys default to premium (safe), preventing accidental exposure of unreviewed features
- Each toggle fires a Redis Pub/Sub message → all service instances reload within milliseconds

### Negative

- Each service that gates features must boot a `FeatureGateCache` instance
- Up to 60s staleness if the Pub/Sub invalidation message is missed (acceptable for feature toggles)
- Only `share-service` consumes the cache today — cache lives in `share-service/featuregates/`, not `shared/`, to avoid premature abstraction. Move to `shared/` when a second service needs it.

## Implementation

- **Migration**: `migrations/044_feature_gates.sql` — creates `feature_gates` table with `sharing` seed row (`is_premium=TRUE`)
- **Cache**: `services/share-service/featuregates/cache.go` — `FeatureGateCache` struct with `IsPremium`, `Start`, `reload`, `GetAll`
- **Constants**: `GateSharing = "sharing"`, `PubSubChannel = "feature-gates:invalidate"`, `refreshInterval = 60s`
- **Tests**: `services/share-service/featuregates/cache_test.go` — 9 tests covering known/unknown keys, Pub/Sub invalidation, periodic refresh
- **Timeline**: Phase 07 Plan 01 — 2026-03-29

## Related Decisions

- ADR-0002: Redis Streams + Pub/Sub Hybrid — Pub/Sub already in use for lifecycle events
- ADR-0004: No Hexagonal Architecture — cache lives in service package, not behind interface layer
- Phase 07 CONTEXT.md: D-07 (DB source of truth), D-08 (in-memory per service), D-09 (60s TTL), D-10 (zero DB hits at request time)
