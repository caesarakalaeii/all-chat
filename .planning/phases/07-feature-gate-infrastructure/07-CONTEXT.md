# Phase 7: Feature Gate Infrastructure - Context

**Gathered:** 2026-03-29
**Status:** Ready for planning

<domain>
## Phase Boundary

Add capability-level premium toggling so experimental features can ship as premium, be tested by the community, and flipped to free at any time without code changes. This is the infrastructure for a "ship premium, graduate to free" lifecycle.

Cosmetics item-level gating (`cosmetic_*.is_premium`) is OUT OF SCOPE — it already works correctly for the per-item premium/free distinction (event flairs free, custom flairs premium).

</domain>

<decisions>
## Implementation Decisions

### Gate Model
- **D-01:** New `feature_gates` Postgres table (feature_key VARCHAR PK, is_premium BOOLEAN, description TEXT) as the source of truth
- **D-02:** Follows the existing `supported_platforms.is_enabled` pattern — database-driven, no hardcoded environment toggles

### Granularity Scope
- **D-03:** Start minimal — only gate features that currently check `users.is_premium` (day-one: `sharing`)
- **D-04:** Future experimental features get a row added when they ship — the table grows organically
- **D-05:** Keep `supported_platforms` separate — it serves platform availability, not monetization
- **D-06:** Cosmetics keep existing per-item `is_premium` flag unchanged — already supports the desired model (event/collab flairs free, premium flairs paid)

### Enforcement Pattern
- **D-07:** DB as source of truth + Redis Pub/Sub invalidation for instant propagation
- **D-08:** Each service holds an in-memory map of feature gates, refreshed via Redis Pub/Sub subscription
- **D-09:** Periodic TTL refresh (60s) as fallback for missed Pub/Sub messages
- **D-10:** Zero DB hits at request time after boot — all checks against in-memory map
- **D-11:** Rewrite `RequirePremium` middleware to: check in-memory gate → if feature is_premium=false, allow everyone → if is_premium=true, check `users.is_premium` as today

### Admin Experience
- **D-12:** Admin panel UI at `/admin/features` with toggle switches per feature
- **D-13:** Backed by API endpoint (PATCH /api/v1/admin/feature-gates/:key)
- **D-14:** Toggle publishes invalidation event to Redis Pub/Sub so all services pick up the change instantly

### Precedence Rules
- **D-15:** Feature gate `is_premium=false` overrides user's `is_premium` check — feature is free for everyone
- **D-16:** Feature gate `is_premium=true` falls through to existing `users.is_premium` check — premium users only

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing Premium Infrastructure
- `services/share-service/middleware/premium.go` — Current RequirePremium middleware to be rewritten
- `services/share-service/repository/premium_repo.go` — IsPremium/UpdateUserPremium methods
- `services/share-service/handlers/routes.go` — Route registration showing which endpoints are premium-gated
- `migrations/030_share_requests.sql` — users.is_premium column definition

### Existing Feature Toggle Patterns
- `migrations/001_initial_schema.sql` — supported_platforms.is_enabled pattern (reference for DB-driven gating)

### Cosmetics (reference only — not modified)
- `services/auth-service/handlers/admin_cosmetics.go` — Per-item is_premium flag management
- `migrations/037_cosmetics_catalog.sql` — cosmetic_frames/flairs is_premium columns

### Admin Panel
- `frontend/src/app/admin/users/page.tsx` — Existing premium toggle UI (reference for admin patterns)
- `frontend/src/app/admin/cosmetics/page.tsx` — Existing cosmetics admin page

### Architecture
- `docs/adr/README.md` — ADR index (new ADR may be needed for feature gate design)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `RequirePremium` middleware pattern in share-service — rewrite target, not reusable as-is
- Admin panel pages for users and cosmetics — UI patterns to follow for the new `/admin/features` page
- `PremiumBadge` component — could be repurposed for feature gate status indicators
- Redis Pub/Sub already used for overlay message delivery — same infrastructure for gate invalidation

### Established Patterns
- Database-driven feature flags via `supported_platforms.is_enabled` — proven pattern
- Gin middleware for auth/premium checks — extend this for gate-aware middleware
- Admin API pattern: POST/PATCH endpoints with auth middleware

### Integration Points
- Share service: RequirePremium middleware needs gate-awareness
- Auth service (cosmetics): No changes needed — per-item gating already works
- Redis: New Pub/Sub channel for feature gate invalidation
- Frontend admin: New `/admin/features` route and page
- Database: New migration for `feature_gates` table

</code_context>

<specifics>
## Specific Ideas

- **Lifecycle model**: "Ship premium → community tests → flip to free when ready" — this is the core product workflow driving the design
- **Cosmetics distinction**: Per-item gating for cosmetics enables cooperation/event flairs to be free while custom/premium flairs stay paid — this is already working and should not be changed
- **Scale priority**: User explicitly chose Redis cache + Pub/Sub invalidation over simpler approaches because scale is a priority

</specifics>

<deferred>
## Deferred Ideas

- Percentage-based rollout / gradual feature flagging (noted as a downside — not needed now)
- Absorbing `supported_platforms.is_enabled` into `feature_gates` (explicitly rejected in favor of keeping them separate)
- Deprecating or removing `users.is_premium` column (still needed as the user-level premium check when feature gate is active)

</deferred>

---

*Phase: 07-feature-gate-infrastructure*
*Context gathered: 2026-03-29*
