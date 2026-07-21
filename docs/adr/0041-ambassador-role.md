# ADR-0041: Ambassador Role + Public Homepage Showcase

**Date**: 2026-07-21
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

We want to recognise a small, curated set of streamers as **ambassadors**. An ambassador should:

1. receive all **premium** features,
2. receive the **beta-tester / early-access** capability, and
3. be **showcased on the public marketing homepage** as social proof.

Entitlements today are three independent mechanisms (ADR-0018 premium, ADR-0020 beta-tester, admin role in the JWT), and there is no unified RBAC. The first two requirements map almost exactly onto the beta-tester role (ADR-0020). The genuinely new part is the public showcase: there is no existing public endpoint that exposes a streamer's avatar + display name, and featuring real people publicly raises a consent question.

## Decision Drivers

- **Reuse the proven patterns.** ADR-0020's admin-granted boolean folded into `Recompute`, and ADR-0008's feature-gate enforcement, are the right shapes. Do not invent a new authorization subsystem for a third role.
- **Fresh enforcement.** Granting the role from the admin dashboard must take effect immediately — no "log out and back in" (rules out a JWT role).
- **Consent before public exposure.** Being featured on public marketing is outward-facing; the streamer, not only an admin, must opt in.
- **Separate identity from presentation.** The entitlement is one boolean; the marketing-card metadata (tagline, order, consent) is presentation and should not bloat the hot `users` read path.
- **No entitlement-writing data migration.** The migration runner re-runs every file on every pod start; grants must be admin actions, never a blanket `UPDATE`.

## Considered Options

1. **`users.is_ambassador` boolean folded into `Recompute` + early-access, plus a separate `ambassador_showcase` table for the public card.** (Mirrors beta-tester for entitlement; isolates presentation.)
2. **Reuse `is_beta_tester` and add only a showcase flag.** Conflates two distinct recognitions (a beta tester is not necessarily a public ambassador, and vice-versa) and gives them tangled revoke semantics.
3. **A generalized `user_roles` table** (the "third role" ADR-0020 said to reconsider at). Still over-engineered: ambassador is a fixed bundle (premium + early-access), not an arbitrary permission set, and the showcase data is presentation, not permissions.
4. **Ambassador as a JWT role** (like admin). Stale-until-relogin, inconsistent with the DB-backed `RequirePremium`/`RequireEarlyAccess` precedent.

## Decision Outcome

**Chosen**: **Option 1** — `users.is_ambassador` folded into the `is_premium` derivation and into the early-access check, plus a dedicated `ambassador_showcase` table (tagline + sort_order + featured_consent) driving a public `GET /api/v1/ambassadors`. Grants are an admin action; public display requires the streamer's own opt-in.

**Rationale**: it reuses both established patterns (driver 1), enforces freshly on grant because the flag is read from the DB, not the JWT (driver 2), gates public exposure on streamer consent (driver 3), and keeps the entitlement a single boolean while presentation lives in its own table (driver 4). A generic role table (Option 3) remains unjustified for a fixed bundle; ADR-0020's "revisit at a third role" note is thus explicitly considered and declined here.

### Key sub-decisions

- **An ambassador is premium and early-access, by derivation.** `shared/premium.Recompute`'s premium half becomes `Effective(override, hasActiveSub OR is_beta_tester OR is_ambassador)`, and `shared/middleware.RequireEarlyAccess` queries `(is_beta_tester OR is_ambassador)`. `Effective` is unchanged; an admin force-deny (override FALSE) still wins over everything.
- **Presentation is a separate table.** `ambassador_showcase(user_id PK, tagline, sort_order, featured_consent)`. The entitlement column on `users` stays a single boolean, so the ~9 `users` SELECT/scan sites in auth-service gain exactly one field. The admin listing surfaces the card via a `LEFT JOIN`; `/auth/me` gains only `is_ambassador` (the showcase fields are `omitempty` and unpopulated there).
- **Public display is opt-in.** Assigning the role grants premium + early access **immediately**, but `GET /api/v1/ambassadors` returns a streamer only when `is_ambassador AND featured_consent` (and not banned). `featured_consent` defaults FALSE and is written **only** by the streamer via `PUT /api/v1/ambassadors/me/showcase`; the admin curates `tagline`/`sort_order` but never the consent.
- **Not a purchasable premium feature.** Ambassador is an admin-granted recognition role, exactly like beta-tester (ADR-0020). It is therefore **not** a `feature_gates` entry and is **not** listed in the `/upgrade` page or the onboarding "Optional: go further" premium funnel — those advertise features a user can unlock by paying, which this is not. The public showcase is marketing, not a gated capability.
- **Channel link is derived, not stored.** The showcase card links to the streamer's channel via the existing `channelUrl(platform, username)` helper using their `auth_provider` + `username`. This is exact for Twitch/Kick; for YouTube it is a best-effort `@handle` link (documented limitation — a stored custom URL can be added later if needed).
- **Migration 078 (new; edits no prior migration).** Idempotent `ADD COLUMN IF NOT EXISTS users.is_ambassador` (+ partial index) and `CREATE TABLE IF NOT EXISTS ambassador_showcase` (+ index). It writes **no** entitlement or consent values, so a re-run can never clobber a live grant or flip a streamer's consent — guarded by `services/auth-service/repository/migrations_rerun_test.go`.

## Consequences

### Positive

- Reuses the beta-tester/premium machinery; no new authorization subsystem.
- Grants take effect immediately (DB-backed, not JWT).
- Real people are only featured publicly with their explicit consent.
- The homepage section self-hides when no ambassador has opted in, so it never shows an empty shell.

### Negative

- A third admin-granted role increases the pressure toward a generic role model; if a **fourth** arrives, reconsider a `user_roles` table (carrying ADR-0020's note forward).
- Entitlement state now spans `users` (role) and `ambassador_showcase` (presentation/consent); two writers, but the public read is a single join and the recompute is convergent.
- The YouTube channel link is best-effort until a stored custom URL is added.

## Implementation

- **Migration**: `migrations/078_ambassador_role.sql` (+ `_down`).
- **Shared**: `shared/premium/recompute.go` — `Recompute` ORs `is_ambassador` into the premium input (`Effective` unchanged). `shared/middleware/early_access.go` — querier reads `(is_beta_tester OR is_ambassador)`.
- **share-service**: `repository.AmbassadorRepository` (SetUserAmbassador + recompute, GetShowcase, SetFeaturedConsent, ListPublic); `handlers.AmbassadorHandler` (admin assign, self get/put consent, public list); routes — public `GET /api/v1/ambassadors`, self `GET|PUT /api/v1/ambassadors/me/showcase`, admin `POST /api/v1/admin/ambassadors/users/:id`.
- **auth-service**: `models.User.IsAmbassador` + `is_ambassador` in the user-load queries/scan; admin user-list `LEFT JOIN ambassador_showcase` surfaces `tagline`/`sort_order`.
- **api-gateway**: `service_config.go` prefixes `/api/v1/ambassadors` and `/api/v1/admin/ambassadors` → share-service; public/protected/admin routes registered.
- **Frontend**: `User.is_ambassador` type; `FeaturedAmbassadors` homepage section; admin users page grant/revoke + curated tagline/order + AMBASSADOR badge; `AmbassadorSettingsCard` opt-in switch in Settings.

(ADR numbering is shared with caesar-deployment; 0038 is allocated there, so this is 0041.)
