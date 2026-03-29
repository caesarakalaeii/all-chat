# Phase 7: Feature Gate Infrastructure - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-29
**Phase:** 07-feature-gate-infrastructure
**Areas discussed:** Gate Model, Granularity Scope, Enforcement Pattern, Admin Experience

---

## Gate Model

| Option | Description | Selected |
|--------|-------------|----------|
| Database table (`feature_gates`) | New Postgres table, services query at request time with caching. Follows existing `supported_platforms.is_enabled` pattern. | ✓ |
| Environment variables per service | Each service reads feature flags from env vars. Requires redeployment to change. | |
| Redis key-value | Feature flags stored in Redis. Instant propagation but volatile. | |

**User's choice:** Database table (`feature_gates`)
**Notes:** User agreed this fits existing patterns best — already using Postgres as source of truth for `supported_platforms`.

---

## Granularity Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Flat feature keys only | One row per feature, all independent | |
| Flat keys + absorb `supported_platforms` | Consolidate all toggles into one table | |
| Start minimal — only `is_premium`-gated features | Just `sharing` and `cosmetics` for now. Keep `supported_platforms` separate. | ✓ |

**User's choice:** Start minimal
**Notes:** User later clarified that cosmetics should keep per-item gating (`cosmetic_*.is_premium`) rather than a feature-level gate. The `cosmetics` entry in `feature_gates` is not needed — per-item flags already handle the desired model (event/collab flairs free, custom flairs premium). Day-one entries: `sharing` only + future experimental features as they ship.

---

## Enforcement Pattern

| Option | Description | Selected |
|--------|-------------|----------|
| Shared middleware (gate-aware) | Rewrite `RequirePremium` to check gate table, then user premium. Simple, low-friction. | |
| Central API call to gate service | New service/endpoint that answers "can user X access feature Y?" | |
| DB + Redis cache with Pub/Sub invalidation | In-memory map per service, Redis Pub/Sub for instant propagation, periodic TTL refresh fallback. Zero DB hits at request time. | ✓ |

**User's choice:** DB + Redis cache with Pub/Sub invalidation
**Notes:** User stated "scale is the name of the game" — chose the most scalable option. Combines DB source of truth with zero request-time latency via in-memory maps.

---

## Admin Experience

| Option | Description | Selected |
|--------|-------------|----------|
| Admin panel UI | New `/admin/features` page with toggle switches. Visual, immediate. | ✓ |
| API-only | PATCH endpoint, no UI. Developer-operated. | |
| Both (API first, UI follows) | Build API, then add UI later. | |

**User's choice:** Admin panel UI
**Notes:** Follows existing admin panel patterns (users, cosmetics pages).

---

## Claude's Discretion

None — all areas discussed and decided by user.

## Deferred Ideas

- Percentage-based / gradual rollout (noted as a downside, not needed now)
- Absorbing `supported_platforms.is_enabled` into feature_gates (explicitly rejected)
