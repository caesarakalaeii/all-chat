# ADR-0041: Ambassador Role + Public Homepage Showcase

**Date**: 2026-07-21
**Status**: Accepted
**Revised**: 2026-11-03 (brought in line with the current implementation)
**Deciders**: caesarakalaeii

## Context

A small, curated set of streamers should be recognised as **ambassadors**: all premium features, the beta-tester/early-access capability (ADR-0020), and a showcase on the public marketing homepage as social proof. The first two map almost exactly onto the beta-tester pattern; the new part is the public showcase — no existing public endpoint exposes a streamer's avatar + display name, and featuring real people publicly raises a consent question.

## Decision

`users.is_ambassador` folded into the `is_premium` derivation and into the early-access check, plus a dedicated `ambassador_showcase` table (tagline, sort_order, featured_consent) driving a public `GET /api/v1/ambassadors`. Grants are admin actions; public display requires the streamer's own opt-in. Like ADR-0020, a generic `user_roles` table stays unjustified for a fixed bundle (premium + early-access); a fourth role would trigger reconsideration.

How it works:

- `shared/premium.Recompute` ORs `is_ambassador` into the premium input; `shared/middleware.RequireEarlyAccess` queries `(is_beta_tester OR is_ambassador)`. `Effective` is unchanged, so an admin force-deny still wins, and grants take effect immediately (DB-read, not JWT).
- `ambassador_showcase(user_id PK, tagline, sort_order, featured_consent)` keeps presentation off the hot `users` row: the entitlement is still one boolean column, the admin listing joins the card in, and `/auth/me` gains only `is_ambassador`.
- Public display is opt-in: the list returns a streamer only when `is_ambassador AND featured_consent` (and not banned). `featured_consent` defaults FALSE and is written only by the streamer via `PUT /api/v1/ambassadors/me/showcase`; the admin curates tagline/order but never consent.
- Not a purchasable premium feature: it carries no `feature_gates` entry, no `/upgrade` row and no onboarding-tour entry, because it is not something a user can unlock by paying.
- The channel link is derived, not stored: the list API ships `username` and `platform`, and the homepage card passes both through the `channelUrl` helper. Exact for Twitch/Kick (and most platforms); for YouTube an `@handle` link when a handle resolves and no link otherwise.
- Migration 078 is idempotent and writes no entitlement or consent values.

Alternatives considered: reusing `is_beta_tester` plus a showcase flag (conflates two distinct recognitions with tangled revoke semantics); ambassador as a JWT role (stale-until-relogin, inconsistent with the DB-backed gate precedent).

## Consequences

- Reuses the beta-tester/premium machinery; grants are immediate; real people are featured only with explicit consent; the homepage section self-hides when no ambassador has opted in.
- A third admin-granted role raises the pressure toward a generic role model — reconsidered if a fourth arrives.
- Entitlement state spans `users` (role) and `ambassador_showcase` (presentation/consent); two writers, but the public read is a single join and the recompute is convergent.
- The YouTube channel link is best-effort.

## Implementation

- Migration: `migrations/078_ambassador_role.sql` (+ `_down`).
- Shared: `shared/premium/recompute.go`, `shared/middleware/early_access.go`.
- share-service: `repository.AmbassadorRepository` (assign + recompute, showcase get/put, public list), `handlers.AmbassadorHandler`, routes — public `GET /api/v1/ambassadors`, self `GET|PUT /api/v1/ambassadors/me/showcase`, admin `POST /api/v1/admin/ambassadors/users/:id`.
- auth-service: `models.User.IsAmbassador` in user-load queries; admin user list joins the showcase.
- api-gateway: `/api/v1/ambassadors` and `/api/v1/admin/ambassadors` routed to share-service.
- Frontend: `User.is_ambassador`; the `FeaturedAmbassadors` homepage section; the admin grant/revoke + curated card; the `AmbassadorSettingsCard` opt-in switch in Settings.
