# ADR-0008: Feature Gate Infrastructure

**Date**: 2026-03-29
**Status**: Accepted
**Revised**: 2026-11-18
**Deciders**: Engineering team
**Amended by**: [ADR-0020](./0020-beta-tester-role.md) added an `early_access` dimension beside `is_premium`; folded into the body below.

## Context

Experimental features need to ship as premium-only, be tested by the community, and be opened up at any time without a deploy. Initially `users.is_premium` was checked directly in middleware, so every toggle required a code change. The `sharing` feature was the first concrete case.

## Decision

A `feature_gates` Postgres table is the source of truth; each service boots an in-memory cache that reloads on a Redis Pub/Sub invalidation message and refreshes every 60s. Gate flips take effect within milliseconds, require no deploy, and cost no request-time DB hits. Up to 60s staleness if the invalidation message is missed; acceptable for feature toggles.

Alternatives rejected: environment variables (pod restart per change); LaunchDarkly-style per-user flags (external dependency, granular targeting unneeded for all-or-nothing gates); hardcoded constants (every graduation is a deploy).

Unknown gate keys fail closed on both dimensions, so unreviewed features are never exposed by accident.

## Implementation

- Migration `migrations/044_feature_gates.sql` creates the table (`feature_key`, `is_premium`, `description`); `migrations/065_beta_tester_role.sql` adds `early_access`. Later migrations seed their own rows (061 moderation, 076 engagement, 090 bubble_colors, 091-094 and 096 platform gates, 117 altstream with `is_premium=FALSE, early_access=TRUE` per ADR-0082).
- Cache: `shared/featuregates/cache.go` (graduated from `services/share-service/featuregates/` once a second service needed it) with `IsPremium(key)`, `IsEarlyAccess(key)`, `Start`, `GetAll`. `early_access=TRUE` narrows a gate to beta testers and ambassadors (ADR-0020), never widens access.
- Gate keys are typed constants in the same file (`GateSharing`, `GateModeration`, `GatePlatformAltstream`, `GateChatCommands`, ...); `PubSubChannel = "feature-gates:invalidate"`, `refreshInterval = 60s`.
- Enforcement: `shared/middleware/premium.go` (`RequirePremium`) and `shared/middleware/early_access.go` (`RequireEarlyAccess`), mounted per route.
- Admin API: `services/share-service/handlers/admin_featuregates.go`, `GET/PATCH /api/v1/admin/feature-gates[/:key]` behind `AdminOnly`, publishing to the invalidate channel on update.
- Tests: `shared/featuregates/cache_test.go` plus per-gate suites (`alerts_gates_test.go`, `chat_commands_gate_test.go`, `custom_chat_bot_gate_test.go`, `ui_languages_gate_test.go`, `youtube_official_api_gate_test.go`).

Related: ADR-0002 (Pub/Sub already in use for lifecycle events), ADR-0004 (plain shared package, not an interface layer), ADR-0020 (early access).
