# ADR-0017: Chat Moderation Write-Path

**Date**: 2026-06-19
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

All-Chat is read-only end-to-end (ADR-0002: listeners → `chat:raw` → message-processor → Pub/Sub → gateway WebSocket → frontend). Streamers have repeatedly asked to **moderate chat directly from the dashboard** (`/overlay/[id]/view`) — delete a message, time out or ban a user — instead of switching to each platform's native tools. This requires All-Chat's first authenticated **write** action against platform APIs, on a streamer's behalf, which is a new and security-sensitive capability.

## Decision Drivers

- **Reuse the existing reflect-back pipeline.** Platform-originated deletions already travel as `RawChatMessage{EventType:"message_deletion"}` on `chat:raw` and render on overlays + the dashboard (`overlayViewModel.ts`). A moderation action should produce the *same* event, not a new mechanism.
- **Least privilege (non-negotiable).** A streamer who only *displays* chat must never be asked for moderation scopes. Scopes must be opt-in, per-platform, and minimised to the enabled actions.
- **Owner-only authorization.** A powerful write path must not let user A moderate user B's channel; shared overlays must not grant moderation of the original streamer's channel.
- **Accountability.** Every action — including those performed by an admin under impersonation — must be auditable.
- **Per-platform reality.** Twitch is clean; Kick/Discord need new authenticated clients; YouTube is blocked on a Google OAuth re-verification; TikTok has no moderation API at all.

## Considered Options

1. **New `moderation-service` + HTTP via the gateway proxy** — a dedicated Standard-Go-Layout service (ADR-0001/0004) that receives commands through the existing api-gateway proxy/registry and JWT middleware.
   - ✅ Pros: clean isolation of a high-blast-radius capability; own deploy cadence, metrics, rate limits; matches the dominant mutation pattern (gateway proxy + per-service JWT re-validation); reuses the reflect-back pipeline wholesale.
   - ❌ Cons: a new service to operate (deployment, secrets, NetworkPolicy).

2. **Extend `auth-service`** — add moderation handlers next to the existing `chat_send.go` streamer-token write path.
   - ✅ Pros: token-refresh providers already wired there.
   - ❌ Cons: widens the blast radius of the identity boundary; conflicts with ADR-0012's scope minimisation; auth-service is already large.

3. **Extend a listener (e.g. `twitch-eventsub-listener`)** — it already holds Helix clients and broadcaster tokens.
   - ✅ Pros: Helix client + tokens in memory.
   - ❌ Cons: listeners are sharded/leader-elected ingest loops (ADR-0007/0015) — a synchronous request/response handler is an impedance mismatch; Twitch-only, no home for Kick/Discord/YouTube.

4. **WebSocket command frames on the overlay socket** — send commands upstream on the existing overlay WS.
   - ✅ Pros: a socket already exists.
   - ❌ Cons: that socket is a one-way broadcast (server→client) and must stay anonymous for OBS; adding bidirectional command semantics is large net-new surface with its own auth/correlation.

## Decision Outcome

**Chosen**: Option 1 — a new `moderation-service` reached via HTTP through the api-gateway proxy.

**Rationale**: It isolates the first write path behind its own boundary, matches the existing proxy/JWT pattern, and reuses the entire reflect-back pipeline (no new event types). The other options either spread powerful writes into ill-fitting services or bolt command semantics onto a broadcast socket.

Key sub-decisions:
- **Transport**: `POST /api/v1/moderation/overlays/:id/{delete,timeout,ban,unban}` and `GET .../capabilities`, proxied to the service (which re-validates the JWT and re-checks authorization — the gateway proxies blindly).
- **Moderator identity**: the broadcaster's **own** stored OAuth token (`broadcaster_id == moderator_id`), resolved per `(platform, channel_id)` (reusing the ADR-0016 credential selection). All-Chat can only moderate channels whose owner authorized it.
- **Authorization (owner-only)**: `VerifyOverlayOwnership` + the `(platform, channel_id)` must be a real source on the overlay and **not** `shared_overlay`.
- **Reflect-back**: on success, publish a `message_deletion` to `chat:raw` (`single` for delete; `batch` + `ban_duration` for timeout; `batch` without it for ban). The existing consumer/normalizer/gateway/frontend path applies it. No new event types. A `single` deletion carries the internal `target_uuid` (the dashboard already knows the id of the message it moderated); the message-processor consumer trusts it and skips the msgid-registry reverse-lookup. This is additive — native platform deletions (which carry only the platform message id) still use the registry — and is **required for non-Twitch reflect-back**, since only twitch-listener populates that registry.
- **Least-privilege re-consent**: moderation scopes are requested **only** via an explicit opt-in flow, per platform/account, minimised to the enabled actions (Twitch: delete ⇒ `moderator:manage:chat_messages`; timeout/ban ⇒ `moderator:manage:banned_users`). Login and add-source flows are untouched. `granted_scopes` is the source of truth.
- **Impersonation**: moderation under admin impersonation is **allowed but always attributed** — the action runs as the impersonated owner (their token) while the audit row records the real admin (`impersonated_by`).
- **Per-platform**: Twitch full; Kick timeout/ban (new OAuth-2.1 client); Discord delete/timeout/ban/unban (shared bot REST token, gated by the bot's guild permissions); YouTube ban-only and feature-gated (re-adds `youtube.force-ssl`); TikTok unsupported (no API — reported `unsupported_platform`, never a fake button). Moderation resolves the broadcaster credential for both primary-login AND linked (non-primary) accounts on every OAuth platform.
- **Rollout control (feature gate, ADR-0008)**: the whole write path is gated on the `moderation` feature gate, seeded premium-only so it ships to a small cohort first and graduates to all users by flipping one row (no redeploy). The four action endpoints enforce it with `middleware.RequirePremium` (403 outside the cohort); the `capabilities` endpoint stays ungated but reports `enabled` so the dashboard hides controls for non-cohort owners rather than offering actions that would 403. Because the dispatcher path is reflect-back-only until a streamer holds real mod scopes, the gate is the primary guard that keeps the (live) dry-run from reaching a broad audience.

This **amends ADR-0012**: `youtube.force-ssl` is re-added, but only for streamers who opt into YouTube moderation — not on login or for viewers.

## Consequences

### Positive
- Reuses the entire reflect-back pipeline and the ADR-0016 token model; near-zero frontend rendering changes.
- A high-blast-radius capability is isolated with its own authz, audit log, rate limits, metrics, and (planned) NetworkPolicy.
- Least-privilege, opt-in scopes mean non-moderating streamers never see a moderation consent screen.
- Impersonated moderation is fully attributable, closing the per-action gap left by `impersonation_audit_log` (which logs only session start).

### Negative
- A new service to deploy, secret-manage, and network-policy.
- Re-consent friction for streamers who opt in (bounded to the platforms/actions they enable).
- YouTube ships on Google's timeline (force-ssl re-verification is external, multi-week).
- Possible duplicate deletion events on EventSub-owned Twitch channels (our emit + Twitch's echo) — safe via frontend idempotency + gateway replay dedup.

## Implementation

- **Service**: `services/moderation-service/` — `cmd/main.go`, `handler/`, `repository/`, `audit/`, `publisher/`, `models/`, plus Phase 1 `clients/` (Helix), `tokens/` (broadcaster-token resolve/refresh + scope check), `dispatch/` (platform call orchestration).
- **Reflect-back**: `publisher/deletion.go` emits to `chat:raw` (shape mirrors `twitch-eventsub-listener/webhooks/handler.go`; validated against `message-processor` consumer + `NormalizeDeletion`).
- **Authz**: `repository/repository.go` (`VerifyOverlayOwnership`, `IsModeratableSource`, `ListModeratableSources` — excludes `shared_overlay`).
- **Token resolution (Phase 1)**: `tokens/source.go` resolves the requesting user's OWN broadcaster credential per `(user, channel)` — `users` row preferred over `twitch_oauth_tokens` (ADR-0016) — decrypts via the shared `MultiKeyEncryptor`, and refreshes via the id.twitch.tv refresh grant (re-encrypt + write-back). `broadcaster_id` comes from `users.twitch_id` / `twitch_oauth_tokens.twitch_user_id` (no Helix login→id lookup). No moderator credential ⇒ `422`.
- **Dispatch**: a per-platform router (`dispatch/multi.go`) keyed by platform; the handler stays platform-agnostic and a platform with no configured client falls back to dry-run (reflect-back only). `dispatch/twitch.go` and `dispatch/kick.go` share the shape: pre-check `granted_scopes` (fail fast, no API call), refresh proactively near expiry and reactively once on `401`, call the platform API, and map `403`/expired-after-refresh to a re-consent prompt. `dispatch/youtube.go` adds two YouTube-specific steps before the call: resolve the live broadcast's `liveChatId` from the youtube-listener's Redis stream-state cache (no quota-costly `search.list`; not-live ⇒ a clean failure, no reflect-back) and reserve quota (ADR-0006), confirmed on success / rolled back on failure. `dispatch/discord.go` (shared bot token; delete + member ban/timeout/unban): no credential resolution or OAuth scope pre-check; member ops are guild-scoped, so it resolves the guild from the channel id (Redis-cached `DiscordGuildResolver`, keeping the handler platform-agnostic). A bot-permission `403` fails the dispatch (no false reflect-back) and points the owner at the bot **re-invite**, not an OAuth re-consent. The capability scope-check is likewise a per-platform router (`handler/scopecheck.go`): Twitch/Kick/YouTube resolve the broadcaster credential's scopes; Discord computes the bot's effective guild permissions (cached) and maps them to actions (delete⇐MANAGE_MESSAGES, timeout⇐MODERATE_MEMBERS, ban/unban⇐BAN_MEMBERS) — so a bot invited without the elevated permissions reports no actions and the dashboard shows the re-invite CTA.
- **Audit**: migration `migrations/060_moderation_actions.sql` + `audit/store.go` (outcomes incl. `success`, `dry_run`, `reauth_required`, `no_credential`, `denied`, `platform_error`).
- **Impersonation provenance**: `shared/middleware/auth.go` now sets `impersonated_by` / `impersonated_user` in the gin context (additive).
- **Gateway**: registry entry + protected routes in `services/api-gateway/models/service_config.go` and `services/api-gateway/cmd/main.go`.
- **Feature gate (ADR-0008)**: `featuregates.GateModeration` (`"moderation"`), seeded `is_premium=TRUE` by `migrations/061_moderation_feature_gate.sql`. `cmd/main.go` boots a `FeatureGateCache`, applies `middleware.RequirePremium` to the `delete/timeout/ban/unban` routes, and injects the cohort check into the handler (`Handler.SetFeatureGate`) so `GET .../capabilities` returns `enabled` (gate ∧ `users.is_premium`, fail-closed on error; `repository.IsUserPremium`). The frontend hides controls + the re-consent banner and shows an upgrade notice when `enabled` is false.
- **Opt-in re-consent**: `GET /api/v1/auth/{twitch,kick,youtube}/moderation/:overlay_id?actions=…` (`auth-service` `HandleEnableModeration`, one handler branching per provider) requests the minimal scopes for the chosen actions **unioned with the existing grant**, so the issued token is always a superset. Twitch uses `oauth.GetAuthURLWithScopes` (`force_verify=true`); YouTube uses `oauth.GetAuthURLWithScopes` (`prompt=consent` + `access_type=offline` to re-prompt for force-ssl and reissue a refresh token); Kick uses `oauth.GetAuthURLWithScopesPKCE` (Kick is OAuth-2.1 + PKCE — the consent URL also stashes a code verifier in Redis under the same key the add-source callback reads). The scope-downgrade guards (`platform_auth_v2.go`) are generalised to a `preservableScopes` set (chat + both Twitch moderation scopes + Kick `moderation:ban` + YouTube `force-ssl`) so a plain login or narrower link never clobbers a stored moderation grant. Reuses the add-source state + callback unchanged (incl. the linked-token path); token-refresh-service never touches `granted_scopes`, so a periodic refresh preserves the grant.
- **Configuration**: `MODERATION_SERVICE_URL` (gateway); `JWT_SECRET_V*`, `DATABASE_*`, `REDIS_*` (service). Per-platform, each falling back to dry-run if absent: Twitch needs `TOKEN_ENCRYPTION_KEY_V*` + `TWITCH_CLIENT_ID`/`TWITCH_CLIENT_SECRET`; Kick needs `TOKEN_ENCRYPTION_KEY_V*` + `KICK_CLIENT_ID`/`KICK_CLIENT_SECRET`; YouTube needs `TOKEN_ENCRYPTION_KEY_V*` + `YOUTUBE_CLIENT_ID`/`YOUTUBE_CLIENT_SECRET` (+ optional `YOUTUBE_QUOTA_LIMIT_DAILY`); Discord needs `DISCORD_BOT_TOKEN` (no cipher — no per-user OAuth).
- **Timeline**: Phase 0 shipped the service, authz, audit, reflect-back, and capabilities as **dry-run**. **Phase 1 (done): Twitch** client + token resolution + dispatcher + opt-in re-consent + the `moderation` feature gate — dry-run dropped for Twitch (scope- *and* cohort-gated). **Phase 2 (done): Kick** (broadcaster's own token, `moderation:ban` scope, opt-in PKCE re-consent; timeout/ban/unban — no single-message delete) **and Discord** (delete-only via the shared bot token, no per-user OAuth; needs the bot in-guild with Manage Messages). Reflect-back was extended to carry `target_uuid` so deletions render on overlays for these platforms (their listeners don't populate the msgid registry). **Phase 3 (done): YouTube** ban-only — `liveChatBans.insert` as the broadcaster (force-ssl re-consent), liveChatId from the listener's Redis cache, quota-accounted (ADR-0006). Unban is deferred (the API deletes a ban by its resource id, which is not persisted). TikTok stays unsupported throughout. **Phase 4 (done): linked accounts** — the broadcaster credential is resolved for non-primary-login accounts too: `kick_oauth_tokens`/`youtube_oauth_tokens` gained the columns moderation needs (migration 062: Kick numeric id + `granted_scopes`; YouTube `granted_scopes`), the re-consent callback persists the grant to the per-link tables, and `HandleEnableModeration` reads platform-scoped existing scopes (fixing a latent cross-platform scope-injection bug in the consent URL for linked accounts). **Phase 5 (done): Discord full moderation** — timeout/ban/unban added to the bot client; the dispatcher resolves the guild from the channel; capability computes the bot's effective guild permissions; and an opt-in **re-invite** URL (`GetModerationAuthURL`, elevated guild permissions) upgrades existing bots in place.

## Related Decisions

- [ADR-0001](./0001-standard-go-layout.md), [ADR-0004](./0004-no-hexagonal-architecture.md) — service layout / no ports-adapters.
- [ADR-0002](./0002-redis-streams-pubsub.md) — the read pipeline the reflect-back reuses.
- [ADR-0012](./0012-oauth-scope-minimisation.md) — **amended** here (force-ssl re-add for opt-in YouTube moderators only).
- [ADR-0013](./0013-overlay-observability-view.md) — the `/overlay/[id]/view` dashboard that hosts the moderation UI.
- [ADR-0008](./0008-feature-gate-infrastructure.md) — the feature-gate infrastructure used to roll the write-path out to a cohort.
- [ADR-0016](./0016-linked-twitch-credentials.md) — the broadcaster-token selection reused to resolve the moderator token.
- [ADR-0006](./0006-youtube-quota-tracking.md) — quota accounting that will gate YouTube ban actions.
