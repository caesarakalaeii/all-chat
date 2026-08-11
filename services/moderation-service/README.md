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
| `GET`  | `/overlays/:id/capabilities` | — → `{ role, is_owner, enabled, can_moderate, delegated_actions?, sources: [{platform, channel_id, channel_name, moderatable, reason?, actions[]}] }` |
| `POST` | `/overlays/:id/delete`  | `{ platform, channel_id, native_message_id, target_uuid }` |
| `POST` | `/overlays/:id/timeout` | `{ platform, channel_id, target_user_id, target_username, duration_seconds, reason? }` |
| `POST` | `/overlays/:id/ban`     | `{ platform, channel_id, target_user_id, target_username, reason? }` |
| `POST` | `/overlays/:id/unban`   | `{ platform, channel_id, target_user_id }` |

Send a unique `Idempotency-Key` header on each POST (double-click/retry dedup). Per-user rate limiting
applies (`MODERATION_RATE_PER_MIN`, default 30/min).

`capabilities` is role-aware (ADR-0048): `role` is `owner` / `moderator` / `none`, and `can_moderate` is
the single flag the UI switches controls on. An owner's per-source answer comes from the scopes on their
broadcaster credential; a **moderator's** comes from what the streamer delegated intersected with the
scopes on the moderator's *own* credential, so `reason` gains moderator-only values that differ in
who can clear them: `not_delegated` (only the streamer can), `needs_consent` (only the moderator can —
this is the "Connect to moderate" state), `needs_discord_link` (the moderator — they must link their
Discord account, since the shared bot acts and All-Chat checks *their* server permissions),
`owner_channel_unverified` (only the streamer — on Discord, they have not linked their own account) and
`bot_missing_permission` (only the streamer — the bot needs re-inviting with moderation permissions).
`can_send` is never true for a
moderator: chat send is a distinct, higher-trust capability and stays owner-only in v1.

The Discord answer deliberately makes **no live permission read**: it checks the two account links and
the bot's cached guild permissions, and leaves the moderator's own standing to the action path. That
mirrors Twitch, where capabilities checks the scope and Helix decides whether they moderate the channel.
`delegated_actions` echoes the grant's action set so a `needs_consent` source can request scopes for
exactly what was delegated rather than guessing high.

A caller with **no role** and an overlay that **does not exist** produce a byte-identical body, so this
endpoint is not an overlay-existence oracle either.

### Delegation grant lifecycle (ADR-0048)

Owner-only. A delegated moderator gets the same 403 a stranger does — managing who may moderate is an
ownership power, not a moderation power.

| Method | Path | Body |
|---|---|---|
| `GET`    | `/overlays/:id/moderators` | — → `{ moderators: [{id, status, display_name?, invitee_label?, actions[], platforms: [{platform, enabled, verification}], invite_expires_at?, …}], cap, used }` |
| `POST`   | `/overlays/:id/moderators` | `{ actions?, platforms?, invitee_label?, expected_platform?, expected_platform_user_id? }` → **201** `{ grant_id, invite_token, expires_at, … }` |
| `PATCH`  | `/overlays/:id/moderators/:grant_id` | `{ actions?, platforms? }` — `platforms` is a `{platform: bool}` map; only the platforms it names change |
| `DELETE` | `/overlays/:id/moderators/:grant_id` | — revoke one |
| `DELETE` | `/overlays/:id/moderators` | — the kill switch: revoke every grant on the overlay, unredeemed invites included |

`invite_token` is a 256-bit secret returned **exactly once**; only its SHA-256 is stored, so a lost
invite is re-minted rather than re-displayed. It expires after 7 days, is single-use, and dies the
moment the grant is revoked.

Absent `actions` grants the safe default (`delete`, `timeout`); an explicitly empty list is a 400 rather
than being widened. Absent `platforms` enables nothing — absence *is* disablement, which is what keeps
Discord (the one platform with no external authority behind it) off until the streamer opts in.
`expected_platform` pre-binds an invite to one account and is accepted for Twitch only, because that is
the only platform where acceptance can actually verify the redeeming account; storing an unverifiable
binding would look like a constraint while protecting nobody.

The 10-moderator cap is enforced at invite time (409 `moderator_cap_reached`), never retroactively, so
lowering it can't cut off a working mod team mid-stream. Admins bypass it. The count is taken under a row
lock on the overlay, so two tabs cannot both see nine and both insert.

Redemption is keyed on the secret, not on any role:

| Method | Path | Body |
|---|---|---|
| `POST` | `/invites/preview` | `{ token }` → `{ overlay_name, owner_display_name, actions[], platforms[], expires_at, expected_account? }` |
| `POST` | `/invites/accept`  | `{ token }` → `{ grant_id, overlay_id, overlay_name, … }` |

Once accepted, the moderator finds the channel through their own listing:

| Method | Path | Body |
|---|---|---|
| `GET` | `/delegations` | — → `{ delegations: [{grant_id, overlay_id, overlay_name, owner_display_name, status, actions[], platforms[], available, accepted_at?, last_action_at?}] }` |

This is **not** a convenience: `GET /api/v1/overlays` is owner-filtered and there is no shared-with-me
listing, so without it an accepted grant is unreachable. It is keyed on the caller alone, carries no user
ids (the moderator learns who delegated to them, not who else moderates there), and is never gated — a
moderator must be able to see a delegation even when the streamer's plan has lapsed, which surfaces as
`available: false` with the streamer's plan named as the cause. `suspended` grants are listed rather than
hidden: a channel that silently vanished would be indistinguishable from a revocation.

The secret travels in the **body**, never in a path — a URL would put a live moderation credential into
every access log, proxy log and `Referer` header along the way. `preview` deliberately omits
`overlay_id`: an overlay UUID already grants chat *read* to whoever holds it, so it is disclosed on
acceptance rather than to everyone who merely opens the link. Accepting grants nothing by itself; each
platform's consent is deferred to the first time the moderator actually uses it.

Only invite **creation** is gated. Listing, narrowing and revoking always work, including after the
feature is rolled back — otherwise a streamer could be left holding moderators they cannot remove.

## Authorization (owner or delegated moderator — ADR-0017, amended by ADR-0048)

1. `ResolveOverlayAccess` resolves, in one round trip, the caller's role (`owner` / `moderator` /
   `none`) plus the **overlay owner's** identity and entitlement. A caller with no role is refused —
   with the same status *and* body an unknown overlay gets, so the endpoints are not an
   overlay-existence oracle. Grants are read live on every action, never cached, so a revocation
   takes effect within one request.
2. **Entitlement is keyed on the overlay OWNER**, not the caller: a premium streamer's moderators
   moderate for free, and a moderator never sees an upgrade prompt for a plan that is not theirs to
   buy. Enforced inside `authorize()` rather than in `RequirePremium` middleware, which keys on the
   caller — so the denial is audited like every other denial and the copy can differ by role.
3. A delegated moderator may only perform the actions the grant lists, **and** only on the platforms
   whose leg the grant enables. Two separate grants of authority: the action names are shared across
   platforms, so an action-only check would let a Twitch-only moderator act on every source the overlay
   carries. An absent leg row is a disabled leg, so this fails closed. An owner may perform anything the
   platform supports and is narrowed by neither.
4. The `(platform, channel_id)` must be a real source on that overlay and **not** `shared_overlay`.
   Under owner-only authorization that exclusion was true by construction; under role-based
   authorization this predicate carries it alone, so it has its own regression test.
5. The action uses the **acting human's own** stored OAuth token — the owner's when the owner acts,
   the moderator's when a delegated moderator acts, never a fallback between them. Discord is the
   exception: the shared bot is always the actor, so All-Chat must verify the actor's own guild
   permissions itself.

Admin **impersonation** is allowed but always attributed: the action runs as the impersonated owner
(their token), while the `moderation_actions` audit row records the real admin in `impersonated_by`.

## Per-platform support

| Platform | Actions | Status |
|---|---|---|
| Twitch | delete, timeout, ban, unban | **live** — real Helix calls via the broadcaster's own token (scope-gated) |
| Kick | timeout, ban, unban | **live** — real Kick API via the broadcaster's own token (`moderation:ban` scope-gated); no single-message delete |
| Discord | delete, timeout, ban, unban | **live** — bot REST (delete `messages`, member `communication_disabled_until`/`bans`); each action gated by the bot's effective guild permission |
| YouTube | ban | **live** — `liveChatBans.insert` via the broadcaster's own token (`force-ssl` scope-gated); ban-only (unban needs the ban resource id, deferred); liveChatId from the listener's Redis cache; quota-accounted (ADR-0006) |
| TikTok | — | unsupported (no moderation API) — reported `unsupported_platform` |

The broadcaster credential is resolved for both primary-login and **linked** (non-primary-login) accounts:
Twitch/Kick/YouTube each UNION the users row with their per-link token table (`*_oauth_tokens`, which
carry `granted_scopes` + the broadcaster id needed by moderation, migration 062 for Kick/YouTube).

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
the "opt-in" is the bot **re-invite** (`GET /api/v1/auth/discord/connect?moderation=true` returns the
elevated invite URL). Capability is computed from the bot's effective guild permissions (delete⇐
`MANAGE_MESSAGES`, timeout⇐`MODERATE_MEMBERS`, ban/unban⇐`BAN_MEMBERS`), so a bot invited without the
moderation permissions reports no actions and the dashboard shows the re-invite CTA. A dispatch 403
(e.g. a channel-level permission overwrite) fails the action — never a false reflect-back. A delegated
moderator additionally needs a Discord **account link** (`GET /api/v1/auth/discord/identity/connect`),
which grants All-Chat no scopes and stores no token: it only records which Discord account they are, so
their own server permissions can be checked. See "The Discord leg" below.

## Delegated writes (ADR-0048, Twitch + Discord)

An owner action and a delegated action differ in exactly one place — which credential performs the
call and against whose channel — and the dispatcher makes that difference explicit rather than
inferring it from the caller's id:

| | Owner | Delegated moderator |
|---|---|---|
| Token | the owner's broadcaster credential (`users` / `twitch_oauth_tokens`) | the moderator's own credential (`mod_oauth_credentials`) |
| `broadcaster_id` | their own, from that credential | the **owner-reach anchor** |
| `moderator_id` | the same id — a streamer is their own moderator | the moderator's `platform_user_id` |
| Scope pre-check | the owner's `granted_scopes` | the **moderator's** `granted_scopes` |

There is **no fallback** between them. A moderator who has not consented gets `422 connect_required`
— consent is deferred to first use, so that is the expected first click on a fresh grant — and never
a call made with the streamer's token.

The **owner-reach anchor** (`tokens.TwitchSource.OwnerTwitchAnchor`) is what stops delegation from
exceeding what the owner could do themselves: it resolves the numeric broadcaster id from a
credential row whose login equals the source's `channel_id`. It applies **no scope predicate and
reads no token** — requiring the owner to hold a moderation scope would deny delegation to exactly
the streamer who delegates *because* they do not moderate themselves. When it cannot be proven the
action is refused with `owner_channel_unverified`, and the copy names the streamer: only they can
fix it.

Twitch itself is the authority on whether the moderator may act. Helix re-checks on every call that
`moderator_id` is the token's own user and that they moderate `broadcaster_id`; a 403 surfaces as a
re-consent prompt rather than anything All-Chat cached.

Every write records five identities in `moderation_actions`: who acted, in what role, for whom,
**whose credential acted** (`credential_user_id` — the machine-checkable proof no fallback
happened), and the platform id sent as the moderator. Denials carry them too, because "this
moderator keeps getting refused" is a signal that is invisible if a denial cannot be told apart
from an owner's.

### The Discord leg

Discord has no per-user moderation API. The shared bot performs every write, so unlike Twitch there is
no token to hand over and **nothing external re-checks the moderator** — All-Chat's own decision *is* the
authorization. `dispatch/discord.go` therefore checks, failing closed on any read error:

1. the **owner-reach anchor** — a `discord_guilds` row for (owner, guild), on owner and delegated
   actions alike, plus (delegated only) a live read showing the owner still holds
   `owner ∥ ADMINISTRATOR ∥ MANAGE_GUILD`;
2. the **moderator's Discord identity** (`discord_identities`, migration 083), without which their
   permissions cannot be read at all;
3. the **`bot ∩ moderator` action intersection** — the bot bounds what is possible, the moderator what
   is permitted, so nobody does through the bot what Discord would refuse them directly;
4. **role hierarchy** for timeout and ban only, because Discord hierarchy-gates the *actor* and the
   actor is the bot, which typically outranks everyone.

The member-standing cache TTL (60s, `clients/discord.go`) is a **security bound, not a tuning knob**:
the `GUILD_MEMBERS` intent is off, so Discord cannot push us a revocation, and that TTL is exactly how
long a moderator whose roles were just stripped keeps acting.

## Rollout (feature gate, ADR-0008)

Delegation has its **own** gate key, `delegated_moderation` (seeded `is_premium=TRUE`, migration 080), so
it can be rolled back without disabling owner moderation. It requires both keys: closing `moderation`
necessarily closes delegation, since there would be nothing left to delegate. Both are evaluated against
the **overlay owner**.

The write path is gated on the `moderation` feature gate, seeded `is_premium=TRUE` (migration 061) so
it ships to a premium cohort first. The `delete/timeout/ban/unban` routes enforce it inside
`authorize()`, keyed on the **overlay owner** (ADR-0048 — `RequirePremium` middleware keys on the
caller, which delegation inverts); `capabilities` stays ungated but returns `enabled`
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
  Discord shipped delete-only via a shared bot token (no per-user OAuth). To make deletions reflect on the
  OBS overlay for non-Twitch platforms (whose listeners don't populate the msgid registry), the delete
  command now threads the internal `target_uuid` through the `message_deletion` event and the
  message-processor trusts it, skipping the Twitch-only registry lookup.
- **Phase 3 (done): YouTube** (ban-only). `tokens/youtube.go` resolves the YT-login broadcaster's
  credential; `clients/youtube.go` calls `liveChatBans.insert` and resolves the liveChatId from the
  youtube-listener's Redis stream-state cache (no quota-costly `search.list`); `quota/` reserves the
  50-unit ban against the shared `youtube_quota_usage` counter (ADR-0006, reserve→confirm/rollback). The
  opt-in re-consent re-adds `youtube.force-ssl`. Unban is deferred (the YouTube API deletes a ban by its
  resource id, which All-Chat does not persist).
- **Phase 4 (done): linked accounts.** `tokens/{kick,youtube}.go` now resolve the broadcaster credential
  for non-primary-login accounts by UNIONing the users row with the per-link `*_oauth_tokens` table
  (migration 062 added the Kick numeric id + `granted_scopes` and the YouTube `granted_scopes`). The
  auth-service re-consent callback persists the grant to those tables, and `GetPlatformGrantedScopes`
  reads platform-scoped existing scopes for the consent URL (no cross-platform scope injection).
- **Phase 5 (done): Discord full moderation.** `clients/discord.go` added timeout/ban/unban + guild
  resolution (`GET /channels/{id}`, Redis-cached) + effective-permission computation; `dispatch/discord.go`
  resolves the guild from the channel and performs member ops; `handler.DiscordScopeChecker` reports the
  actions the bot's guild permissions allow; the auth-service `GetModerationAuthURL` (elevated invite) +
  `HandleConnect?moderation=true` give an opt-in re-invite that upgrades existing bots in place.
- **Phase 6 (in progress): delegated moderators** (ADR-0048). Role resolution and owner-keyed
  entitlement; the grant lifecycle; the owner's Moderators panel; the moderator's `/delegations`
  listing and role-aware `capabilities`; per-platform leg enforcement; the **Twitch leg** — a
  delegated moderator's Helix write now runs on their own credential (see Delegated writes below) —
  and the **Discord leg**, which works differently on purpose (see below).
  **Kick and YouTube legs are not built**: a delegated action on those refuses with
  `delegation_unsupported` rather than falling through to whatever credential the dispatcher would
  otherwise reach for.

## Layout

```
cmd/main.go        # wiring: db, redis, JWT keychain, cipher, per-platform token sources, dispatch router, rate-limit, routes
handler/           # HTTP handlers + idempotency middleware (role authz, dispatch, capabilities); grant lifecycle; scope-check router
clients/           # per-platform moderation API clients (twitch, kick, discord, youtube + liveChat resolver; httptest-mocked)
invites/           # invite secrets: crypto/rand mint + SHA-256 digest (the plaintext is never stored)
tokens/            # broadcaster-token resolve/refresh (ADR-0016 selection) + granted-scope checkers (twitch, kick, youtube)
dispatch/          # per-platform routers + orchestration: multi.go routes by platform; scope pre-check, refresh, 401-retry, 403->re-consent
quota/             # YouTube ban quota reservation over the shared SQL functions (ADR-0006)
publisher/         # reflect-back: RawChatMessage{message_deletion} -> chat:raw (carries target_uuid)
repository/        # role/source authorization queries + delegation grant lifecycle (migration 080)
audit/             # moderation_actions audit log (migration 060)
models/            # request DTOs + capability model + platform support matrix + per-platform scope mapping + grant DTOs
```

## Configuration

`PORT` (8092), `DATABASE_*`, `REDIS_*`, `JWT_SECRET_V*`, `MODERATION_RATE_PER_MIN`,
`MODERATION_IDEMPOTENCY_TTL_SECONDS`. Per-platform credentials are independent — each platform falls
back to dry-run if its credentials are absent:

- **Twitch** (delete/timeout/ban/unban): `TOKEN_ENCRYPTION_KEY_V*` (decrypt broadcaster tokens) **and**
  `TWITCH_CLIENT_ID` / `TWITCH_CLIENT_SECRET` (Client-Id header + refresh grant).
- **Kick** (timeout/ban/unban): `TOKEN_ENCRYPTION_KEY_V*` **and** `KICK_CLIENT_ID` / `KICK_CLIENT_SECRET`
  (refresh grant).
- **Discord** (delete/timeout/ban/unban): `DISCORD_BOT_TOKEN` (the same bot token discord-listener uses).
  No cipher needed — Discord uses no per-user OAuth. Each action is gated by the bot's effective guild
  permission (MANAGE_MESSAGES / MODERATE_MEMBERS / BAN_MEMBERS); owners grant these via the opt-in bot
  re-invite (`/api/v1/auth/discord/connect?moderation=true`).
- **YouTube** (ban): `TOKEN_ENCRYPTION_KEY_V*` **and** `YOUTUBE_CLIENT_ID` / `YOUTUBE_CLIENT_SECRET`
  (Google refresh grant). Optional `YOUTUBE_QUOTA_LIMIT_DAILY` (default 1009000, match the listener).
  Uses the shared Redis (liveChatId cache) and DB (quota functions, migration 008) already configured.

## Tests

`go test ./...` — unit (publisher, handler, clients, idempotency via miniredis) + integration
(repository, audit via testcontainers Postgres; requires Docker).
