# moderation-service

All-Chat's **chat-moderation write-path** — the first service that performs authenticated
**write** actions against streaming-platform APIs on a streamer's behalf (delete a message,
time out, ban, unban). See **[ADR-0017](../../docs/adr/0017-chat-moderation-write-path.md)**.

Everything else in All-Chat is read-only ingest (ADR-0002). This service is reached through the
api-gateway proxy by the authenticated dashboard at `/overlay/[id]/view`, authorizes the request
(owner-only), performs the platform action, audits it, and publishes a `message_deletion` event onto
`chat:raw` so the moderated message disappears from every overlay and the dashboard live — reusing the
existing read pipeline (no new event types).

## Endpoints

All require a user JWT (forwarded by the gateway; re-validated here). Mounted under `/api/v1/moderation`:

| Method | Path | Body |
|---|---|---|
| `GET`  | `/overlays/:id/capabilities` | — → `{ is_owner, enabled, sources: [{platform, channel_id, channel_name, moderatable, reason?, actions[]}] }` |
| `POST` | `/overlays/:id/delete`  | `{ platform, channel_id, native_message_id, target_uuid }` |
| `POST` | `/overlays/:id/timeout` | `{ platform, channel_id, target_user_id, target_username, duration_seconds, reason? }` |
| `POST` | `/overlays/:id/ban`     | `{ platform, channel_id, target_user_id, target_username, reason? }` |
| `POST` | `/overlays/:id/unban`   | `{ platform, channel_id, target_user_id }` |

Send a unique `Idempotency-Key` header on each POST (double-click/retry dedup). Per-user rate limiting
applies (`MODERATION_RATE_PER_MIN`, default 30/min).

## Authorization (owner-only, ADR-0017)

1. The JWT user must own the overlay (`VerifyOverlayOwnership`).
2. The `(platform, channel_id)` must be a real source on that overlay and **not** `shared_overlay`.
3. The action uses the **broadcaster's own** stored OAuth token (`broadcaster_id == moderator_id`).

Admin **impersonation** is allowed but always attributed: the action runs as the impersonated owner
(their token), while the `moderation_actions` audit row records the real admin in `impersonated_by`.

## Per-platform support

| Platform | Actions | Status |
|---|---|---|
| Twitch | delete, timeout, ban, unban | **live** — real Helix calls via the broadcaster's own token (scope-gated) |
| Kick | timeout, ban, unban | **live** — real Kick API via the broadcaster's own token (`moderation:ban` scope-gated); no single-message delete |
| Discord | delete | **live** — bot REST `DELETE /channels/{c}/messages/{m}`; needs the bot in the guild with **Manage Messages** |
| YouTube | ban | **live** — `liveChatBans.insert` via the broadcaster's own token (`force-ssl` scope-gated); ban-only (unban needs the ban resource id, deferred); liveChatId from the listener's Redis cache; quota-accounted (ADR-0006) |
| TikTok | — | unsupported (no moderation API) — reported `unsupported_platform` |

Dispatch is a per-platform router (`dispatch.Multi`); a platform whose credentials aren't configured for
the deployment falls back to **dry-run** (reflect-back only, no platform call) so nothing breaks.

Moderation OAuth scopes are **opt-in and least-privilege**: requested only via a dedicated re-consent
flow (`GET /api/v1/auth/{twitch,kick,youtube}/moderation/:overlay_id?actions=…` on the auth-service), per
platform/account, minimised to the enabled actions and unioned with the existing grant so the new token
is always a superset. Twitch splits delete (`moderator:manage:chat_messages`) from ban/timeout/unban
(`moderator:manage:banned_users`); Kick gates ban/timeout/unban behind one scope (`moderation:ban`);
YouTube re-adds `youtube.force-ssl` (dropped at login per ADR-0012) for ban.
`granted_scopes` is the source of truth; the capabilities endpoint reports `missing_scope` until they
are granted, and the dispatcher pre-checks scopes before any platform call. **Discord is the
exception**: its authority is a shared bot token (a service credential), not a per-user OAuth grant, so
it has no re-consent — capability is `delete` whenever the bot is configured, and the bot's
`MANAGE_MESSAGES` permission is enforced by Discord at call time (a 403 fails the action, never a false
reflect-back).

## Rollout (feature gate, ADR-0008)

The write path is gated on the `moderation` feature gate, seeded `is_premium=TRUE` (migration 061) so
it ships to a premium cohort first. The `delete/timeout/ban/unban` routes enforce it with
`RequirePremium` (403 outside the cohort); `capabilities` stays ungated but returns `enabled`
(gate ∧ `users.is_premium`, fail-closed) so the dashboard hides controls for non-cohort owners.
Graduate to everyone by flipping the gate to `is_premium=FALSE` via the feature-gate admin endpoint —
no redeploy. Locally, non-premium users can either set `users.is_premium=true` or flip the gate to test.

## Phasing

- **Phase 0: dry-run.** Authz + audit + capabilities + reflect-back live; no platform calls. Proved the UX.
- **Phase 1 (done): Twitch real.** `tokens/` resolves + refreshes the broadcaster token; `dispatch/`
  pre-checks scopes, calls Helix (401→refresh→retry, 403→re-consent), and emits the reflect-back on
  success. The opt-in re-consent flow lets streamers grant the minimal scopes. Dry-run is retained for
  platforms without a client yet. **Scope-gated: inert until a streamer opts in.** Also **cohort-gated**
  behind the `moderation` feature gate (see Rollout above).
- **Phase 2 (done): Kick + Discord.** Kick mirrors the Twitch pattern (broadcaster's own token,
  `moderation:ban` scope, opt-in re-consent; ban/timeout/unban — Kick has no single-message delete).
  Discord is delete-only via a shared bot token (no per-user OAuth). To make deletions reflect on the
  OBS overlay for non-Twitch platforms (whose listeners don't populate the msgid registry), the delete
  command now threads the internal `target_uuid` through the `message_deletion` event and the
  message-processor trusts it, skipping the Twitch-only registry lookup.
- **Phase 3 (done): YouTube** (ban-only). `tokens/youtube.go` resolves the YT-login broadcaster's
  credential; `clients/youtube.go` calls `liveChatBans.insert` and resolves the liveChatId from the
  youtube-listener's Redis stream-state cache (no quota-costly `search.list`); `quota/` reserves the
  50-unit ban against the shared `youtube_quota_usage` counter (ADR-0006, reserve→confirm/rollback). The
  opt-in re-consent re-adds `youtube.force-ssl`. Unban is deferred (the YouTube API deletes a ban by its
  resource id, which All-Chat does not persist).

## Layout

```
cmd/main.go        # wiring: db, redis, JWT keychain, cipher, per-platform token sources, dispatch router, rate-limit, routes
handler/           # HTTP handlers + idempotency middleware (owner-only authz, dispatch, capabilities); scope-check router
clients/           # per-platform moderation API clients (twitch, kick, discord, youtube + liveChat resolver; httptest-mocked)
tokens/            # broadcaster-token resolve/refresh (ADR-0016 selection) + granted-scope checkers (twitch, kick, youtube)
dispatch/          # per-platform routers + orchestration: multi.go routes by platform; scope pre-check, refresh, 401-retry, 403->re-consent
quota/             # YouTube ban quota reservation over the shared SQL functions (ADR-0006)
publisher/         # reflect-back: RawChatMessage{message_deletion} -> chat:raw (carries target_uuid)
repository/        # owner/source authorization queries (mirrors api-gateway/subscription)
audit/             # moderation_actions audit log (migration 060)
models/            # request DTOs + capability model + platform support matrix + per-platform scope mapping
```

## Configuration

`PORT` (8092), `DATABASE_*`, `REDIS_*`, `JWT_SECRET_V*`, `MODERATION_RATE_PER_MIN`,
`MODERATION_IDEMPOTENCY_TTL_SECONDS`. Per-platform credentials are independent — each platform falls
back to dry-run if its credentials are absent:

- **Twitch** (delete/timeout/ban/unban): `TOKEN_ENCRYPTION_KEY_V*` (decrypt broadcaster tokens) **and**
  `TWITCH_CLIENT_ID` / `TWITCH_CLIENT_SECRET` (Client-Id header + refresh grant).
- **Kick** (timeout/ban/unban): `TOKEN_ENCRYPTION_KEY_V*` **and** `KICK_CLIENT_ID` / `KICK_CLIENT_SECRET`
  (refresh grant).
- **Discord** (delete): `DISCORD_BOT_TOKEN` (the same bot token discord-listener uses; the bot must be
  in the guild with the **Manage Messages** permission). No cipher needed — Discord uses no per-user OAuth.
- **YouTube** (ban): `TOKEN_ENCRYPTION_KEY_V*` **and** `YOUTUBE_CLIENT_ID` / `YOUTUBE_CLIENT_SECRET`
  (Google refresh grant). Optional `YOUTUBE_QUOTA_LIMIT_DAILY` (default 1009000, match the listener).
  Uses the shared Redis (liveChatId cache) and DB (quota functions, migration 008) already configured.

## Tests

`go test ./...` — unit (publisher, handler, clients, idempotency via miniredis) + integration
(repository, audit via testcontainers Postgres; requires Docker).
