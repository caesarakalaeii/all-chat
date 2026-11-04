# ADR-0008: Feature Gate Infrastructure

**Date**: 2026-03-29
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: Engineering team
**Amended by**: [ADR-0020](./0020-beta-tester-role.md) adds an `early_access`
dimension beside `is_premium` (beta testers and ambassadors only); this ADR folds
it into the body below.

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
- The cache graduated from `services/share-service/featuregates/` to `shared/featuregates/` once a second service (youtube-listener) needed it; overlay-manager, auth-service, engagement-service and moderation-service have since followed. It is still a plain shared package, not an interface layer (ADR-0004).

## Implementation

- **Migration**: `migrations/044_feature_gates.sql` creates the `feature_gates` table with columns `feature_key`, `is_premium`, `description` — the `sharing` seed row (`is_premium=TRUE`) was the only one at write time. The `early_access` column was added later by `migrations/065_beta_tester_role.sql`. Subsequent migrations (061 moderation, 076 engagement, 090 bubble_colors, 091-094 and 096 platform gates, 117 altstream, and others) seed their own rows; the altstream row (migration 117) is `is_premium=FALSE, early_access=TRUE` per ADR-0082.
- **Cache**: `shared/featuregates/cache.go` — `FeatureGateCache` struct with `IsPremium(key)`, `IsEarlyAccess(key)` (ADR-0020), `Start`, `reload` and `GetAll`. Unknown keys fail closed on both dimensions (premium- and early-access-required). The two dimensions compose: `early_access=TRUE` narrows a gate to beta testers and ambassadors (ADR-0020), it never widens access.
- **Constants**: the gate keys are typed Go constants in `shared/featuregates/cache.go` (`GateSharing = "sharing"`, `GateModeration = "moderation"`, `GatePlatformAltstream = "platform_altstream"`, `GateChatCommands = "chat_commands"`, and others); `PubSubChannel = "feature-gates:invalidate"`, `refreshInterval = 60s`.
- **Enforcement**: `shared/middleware` provides `RequirePremium` (`premium.go`) and, since ADR-0020, `RequireEarlyAccess` (`early_access.go`); services mount one or both per route.
- **Admin API**: `services/share-service/handlers/admin_featuregates.go` — `GET /api/v1/admin/feature-gates` and `PATCH /api/v1/admin/feature-gates/:key`, behind `AdminOnly` middleware, update a row and publish to the invalidate channel (`services/share-service/cmd/main.go:282`), so a gate flips with no deploy.
- **Tests**: `shared/featuregates/cache_test.go` plus per-gate suites (`alerts_gates_test.go`, `chat_commands_gate_test.go`, `custom_chat_bot_gate_test.go`, `ui_languages_gate_test.go`, `youtube_official_api_gate_test.go`) — 13 tests in `cache_test.go` covering known/unknown keys, both gate dimensions, Pub/Sub invalidation and periodic refresh.
- **Timeline**: Phase 07 Plan 01 — 2026-03-29

## Related Decisions

- ADR-0002: Redis Streams + Pub/Sub Hybrid — Pub/Sub already in use for lifecycle events
- ADR-0004: No Hexagonal Architecture — cache lives in a shared package, not behind interface layer
- ADR-0020: Beta tester role / early access rollout — the `early_access` column and `RequireEarlyAccess`
- Phase 07 CONTEXT.md: D-07 (DB source of truth), D-08 (in-memory per service), D-09 (60s TTL), D-10 (zero DB hits at request time)
