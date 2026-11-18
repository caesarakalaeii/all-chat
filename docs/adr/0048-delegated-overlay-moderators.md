# ADR-0048: Delegated Overlay Moderators

**Date**: 2026-08-06
**Status**: Accepted
**Revised**: 2026-11-18
**Deciders**: caesarakalaeii

## Context

ADR-0017's moderation write-path is owner-only (`VerifyOverlayOwnership`,
every call on the broadcaster's own token); streamers asked to hand the merged
moderation view to their volunteer moderators too. Core question: whose
credential performs the write? With the moderator's own, the platform
re-checks their role per call and our checks are advisory; with the owner's,
the platform validates nothing and All-Chat becomes the sole stale-able
authority. Further constraints: no privilege escalation, attribution to the
actor, minimal scopes on a volunteer's consent screen, premium keyed on the
overlay **owner**, revocation that works via the platform's own unmod.

## Decision

**A delegated moderator acts with their own platform credential, no fallback to
the owner's, ever.** Discord is the sole exception: it has no per-user
moderation API, so it runs a narrower authority model. Rejected: acting with
the streamer's token (scope inheritance, refresh-token races, broadcaster
attribution); a shared bot everywhere (destroys attribution, only Discord
supports it).

Two authority categories:

- **Platform-executed (Twitch, Kick, YouTube)** — the moderator's own token;
  the platform re-checks the role per call. Load-bearing pre-checks: the
  moderator must hold a credential with the required action scope; Kick's
  `broadcaster_user_id` must come from the owner-reach anchor (Kick's
  endpoints carry no moderator field, so it is the only id in the request; a
  401 surviving a successful refresh reads as not-a-moderator); YouTube's
  `liveChatId` must resolve from the stream-state cache and quota is reserved
  (ADR-0006). A cached "not a moderator" state is telemetry, never
  authorization.
- **Platform-attested (Discord)** — the shared bot performs the write and
  re-checks the moderator's live guild permissions, enforced in-process,
  fail-closed. With the `GUILD_MEMBERS` intent off (`RequiredIntents = 33281`)
  revocations are not pushed to us, so the **60-second cache TTL is a security
  bound**; All-Chat itself enforces role hierarchy for timeout/ban (not
  delete), with permissions as `botPermissions ∩ moderatorPermissions`. The
  three owner-side failures share `owner_channel_unverified`;
  `discord_link_required` is the moderator's code.
  `GET /guilds/{g}/members/{u}` works with the intent off (only the *list*
  endpoint is gated); a fixed defect: `@me` had broken `GuildBotPermissions` /
  `CheckBotPermissions` outright.

Per-platform capability: YouTube is **timeout + ban** (delete needs a Data
API message id the InnerTube listener does not produce; unban needs an
unpersisted ban resource id). A fixed prerequisite:
`KickOAuth.ExchangeCodeWithPKCE` dropped the `scope` field, leaving every
Kick grant with empty `granted_scopes`; a missing platform confirmation reads
as `missing_scope`, never a fabricated grant.

### Owner-reach anchor

A moderator acts only on a channel the **owner can prove they control**,
evaluated at grant activation and action time, fail-closed. It also applies
to owner actions (no-op on Twitch/Kick; closes YouTube's case where the
channel-agnostic `users` fallback would match any channel id).

- **Twitch**: credential row whose login equals the source's `channel_id`
  (`users` branch preferred, no scope predicate).
- **Kick**: `users.kick_id` or `kick_oauth_tokens.kick_user_id`;
  `kick_user_id IS NOT NULL` mandatory (migration 062).
- **YouTube**: `youtube_oauth_tokens` row for owner and exactly the source's
  `channel_id`; the `users` fallback forbidden.
- **Discord**: `discord_guilds` row (migration 035), plus a live
  owner-permission read on delegated actions only (the row suffices for owner
  actions: it is platform-attested via Manage Server at invite time).
- **TikTok**: n/a, no moderation API; absent from `PlatformActions`.
- **shared_overlay**: n/a, non-moderatable; the platform predicates are
  regression-tested.

### Storage, consent, grants, audit

- A moderator's credential lives only in a store keyed on the moderator's
  identity, never in a per-channel table a listener selects from
  (`twitch-eventsub-listener`, `kick-listener` select ingest credentials with
  no user scoping).
- **Consent deferred to first use**; the `/moderate` list is the only route to
  the overlay (`GET /api/v1/overlays` is owner-filtered). Twitch/Kick
  moderation scopes are role-based, so one consent per platform serves every
  delegating streamer. Moderator consent uses a third state kind
  `ActionModConsent` (the add-source state would 404 on the ownership check)
  and its own auth-URL builder, since `GetAuthURLWithScopes` would ask a
  volunteer for the streamer base-login set. Request the minimum for the
  action; never store a narrower grant than the user already had.
- **Premium gating inside `authorize()`** after role resolution, not
  `RequirePremium` middleware (pre-handler: no audit row, no caller-aware
  copy). `is_premium` is materialized via `shared/premium.Recompute`; a second
  gate key `delegated_moderation` allows independent rollback.
- **Grants per-overlay** (per-channel would go stale on every source edit),
  fail-closed narrowed by per-platform enablement (absent row = disabled,
  Discord off by default), the moderator's own credential, and the anchor.
  Dormancy-suspend after 90 days idle (owner re-activates); soft cap of 10 at
  invite time (409 on the 11th); no chat-send; explicit acceptance
  (`pending` until accepted).
- **Audit**: five distinguishable identities — actor, role, overlay owner,
  whose credential actually acted (the machine-checkable proof of
  no-fallback), platform actor id. `actor_role IS NULL` means legacy owner.
  Idempotency: keep the client `Idempotency-Key` (an escalation must not
  collapse into duplicate_ignored) plus a short-lived per-target marker for
  the duplicate reflect-back, computed in the handlers. No overlay-existence
  oracle; role probes counted (`allchat_moderation_unauthorized_denials_total`),
  not audited.

## Consequences

- On three of four platforms the authorization question is answered by the
  platform, free, per call; unmod-revocation works; actions land under the
  moderator's name; the streamer's credential is never on a delegated path.
- Discord has no platform backstop: a stale grant stays live until the
  60-second check fails, mitigated by per-guild opt-in (default off), the
  owner's `MANAGE_GUILD` proof, and the live check.
- Every moderator completes an OAuth consent per platform; admin impersonation
  of a delegated moderator would act with a third party's credential
  (extending ADR-0017's impersonation rule is an unresolved decision);
  rate buckets (Twitch per broadcaster, YouTube per project) are not relieved
  by spreading across moderator tokens; every proxied route needs an explicit
  api-gateway entry (no prefix forward for `/api/v1/auth/*`).

## Implementation

`migrations/080_delegated_moderators.sql` (plus audit columns, and the
`delegated_moderation` gate seed); `services/moderation-service/handler/grants.go`,
`repository/grants.go`, `tokens/` (owner anchors), `dispatch/`; backend
consent at `services/auth-service/oauth/state.go` and
`handlers/mod_consent.go`; moderator-credential refresh loop. Frontend:
`/moderate`, invite/revoke/per-platform toggles, `/upgrade`,
`OnboardingChecklist`.

Related: ADR-0017 (amended), 0012, 0016, 0008, 0019/0027, 0034, 0006, 0013.
