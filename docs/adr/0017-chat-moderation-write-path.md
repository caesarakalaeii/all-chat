# ADR-0017: Chat Moderation Write-Path

**Date**: 2026-06-19
**Status**: Accepted; as amended by [ADR-0048](./0048-delegated-overlay-moderators.md)
**Revised**: 2026-11-18 (ADR-0048 amendment and the 2026-08-12 Kick/YouTube corrections folded in)
**Deciders**: caesarakalaeii

## Context

The platform is read-only end-to-end (ADR-0002). Streamers want to moderate chat from the dashboard (`/overlay/[id]/view`): delete a message, time out or ban a user. That is All-Chat's first authenticated write against platform APIs, so isolation, least privilege, cross-channel authorization safety, and per-action auditability are non-negotiable. Per-platform reality: Twitch is clean; Kick/Discord need new authenticated clients; YouTube lacks a usable message id for delete/unban; TikTok has no moderation API.

Alternatives rejected: extending auth-service (widens the identity boundary's blast radius); extending a listener (request/response handler inside a sharded ingest loop); command frames on the overlay WebSocket (that socket is an anonymous one-way broadcast for OBS).

## Decision

A dedicated `moderation-service` reached via the api-gateway proxy. Transport: `POST /api/v1/moderation/overlays/:id/{delete,timeout,ban,unban}` and `GET .../capabilities`; the service re-validates the JWT and re-checks authorization itself.

Sub-decisions:

- **Actor credential**: the acting token is the actor's own (never a fallback to the owner's); `broadcaster_id` is the overlay owner's platform id, `moderator_id` the actor's, resolved per `(platform, channel_id)` reusing the ADR-0016 credential selection. Discord is the exception: a shared-bot write where the bot is the actor, so All-Chat itself must validate a delegated moderator's live guild permissions, intersected with the bot's, with role hierarchy enforced (our pre-check is the authorization boundary there).
- **Authorization**: owner or active delegated moderator (ADR-0048), plus per-platform enablement, per-action grant, the owner-reach anchor, and a real non-`shared_overlay` source on the overlay (`VerifyOverlayOwnership`, `IsModeratableSource` — security-critical). Outside Discord the platform independently recognises the actor as a moderator.
- **Reflect-back**: on success the service publishes `message_deletion` to `chat:raw` (`single` for delete; `batch` + `ban_duration` for timeout; `batch` without it for ban). No new event types. A `single` carries the trusted internal `target_uuid` (skips the msgid-registry lookup; required for non-Twitch reflect-back since only twitch-listener populates the registry).
- **Least-privilege re-consent**: scopes come only from an explicit opt-in flow per platform/account, minimised to enabled actions and unioned with the existing grant (`HandleEnableModeration`). Login/add-source untouched; a `preservableScopes` set stops a plain login from clobbering an opt-in grant. This re-adds `youtube.force-ssl`, amending ADR-0012, for YouTube moderators only.
- **Impersonation**: allowed but always attributed; runs on the impersonated owner's token, audit row records the real admin (`impersonated_by`).
- **Rollout**: `moderation` feature gate (ADR-0008), seeded premium-only; entitlement checked inside `authorize()` keyed on the overlay owner (not caller-keyed middleware, so denials are audited and copy differs for owner vs moderator). `capabilities` returns `enabled`, fail-closed, so the dashboard hides controls.

Per-platform (all shipped): Twitch full; Kick full including single-message delete (`DELETE /public/v1/chat/{message_id}` behind `moderation:chat_message:manage`; the two Kick scopes are independent, per-scope action mapping); Discord full via the bot REST token gated by its guild permissions (delete needs Manage Messages, timeout Moderating Member, ban/unban Ban Members; a bot-permission 403 points at a re-invite, not re-consent); YouTube timeout and ban (`temporary` ban on `liveChatBans.insert`, quota-accounted per ADR-0006; delete and unban unavailable for want of a usable id, not a scope); TikTok reported `unsupported_platform`.

## Consequences

Reuses the reflect-back pipeline and ADR-0016 token model near-wholesale; the write capability is isolated with its own authz, audit, rate limits, and NetworkPolicy; non-moderating streamers never see a moderation consent screen; impersonated moderation is fully attributable. Costs: another service to operate, re-consent friction for opt-ins, YouTube on Google's re-verification timeline, and possible duplicate deletion events on EventSub-owned channels (our reflect-back plus Twitch's echo; safe via frontend idempotency and gateway replay dedup).

## Implementation

- Service: `services/moderation-service/` (cmd/main.go, handler, repository, audit, publisher, tokens, dispatch).
- Reflect-back: `publisher/deletion.go` to `chat:raw`, shape validated against the message-processor consumer.
- Authz: `repository/repository.go` (`VerifyOverlayOwnership`, `IsModeratableSource`, `ListModeratableSources`).
- Token resolution: `tokens/source.go` (users row preferred over `twitch_oauth_tokens`; shared `MultiKeyEncryptor`; refresh + re-encrypt write-back). No credential ⇒ `422`.
- Dispatch: `dispatch/multi.go` keyed by platform, dry-run fallback when no client is configured. Twitch/Kick pre-check `granted_scopes`, refresh proactively and once reactively on 401, map 403/expired-after-refresh to a re-consent prompt. `dispatch/youtube.go` resolves `liveChatId` from the youtube-listener's Redis stream-state cache (not-live ⇒ clean failure, no reflect-back) and reserves quota. `dispatch/discord.go` resolves the guild via the Redis-cached `DiscordGuildResolver`. Scope checks per platform in `handler/scopecheck.go`.
- Audit: `migrations/060_moderation_actions.sql` + `audit/store.go` (outcomes: success, dry_run, reauth_required, no_credential, denied, platform_error).
- Gateway: registry entry and protected routes in `api-gateway/models/service_config.go` + `cmd/main.go`.
- Gate: `featuregates.GateModeration`, `migrations/061_moderation_feature_gate.sql`; consent at `GET /api/v1/auth/{platform}/moderation/:overlay_id?actions=…` (Twitch `force_verify`, YouTube `prompt=consent` + `access_type=offline`, Kick PKCE with the verifier stashed in Redis for the add-source callback).
- Config env: `MODERATION_SERVICE_URL` (gateway); per platform Twitch/Kick/YouTube need `TOKEN_ENCRYPTION_KEY_V*` + their client id/secret (dry-run fallback when absent); Discord needs `DISCORD_BOT_TOKEN`.
- Phase 4 added linked-account support: migrations 062 (Kick id + scopes on `kick_oauth_tokens`, scopes on `youtube_oauth_tokens`), re-consent persisting to the per-link tables; Phase 5 completed Discord and added the elevated re-invite URL.

Related: ADR-0048 (amends: delegation, gate re-key), ADR-0012 (amended here), ADR-0002, ADR-0008, ADR-0016, ADR-0006, ADR-0013.
