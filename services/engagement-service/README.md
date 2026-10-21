# engagement-service

Cross-platform **polls, predictions, and a per-overlay viewer points economy** (issue #523), and **streamer-defined chat commands and keyword triggers** (ADR-0079).

Participation is **universal and install-free**: viewers vote and wager by typing in native platform chat (`!vote 2`, a bare `2`, `!predict 1 500`) on Twitch/YouTube/Kick/TikTok. An authenticated web page and the browser extension are richer *enhancements*, not gates. Twitch viewers additionally get their native poll/prediction UI mirrored (state only — those use Twitch Channel Points, not All-Chat points).

See **ADR-0028** (chat-command write-path) and **ADR-0029** (points economy + prediction payout model) for the why.

## Architecture

```
                         ┌ message-processor hot path: prefix pre-check + EXISTS engagement:active:{p}:{ch}
listeners → chat:raw → mp ┤   → XADD engagement:commands  (durable; votes/wagers)
                         └   → PUBLISH engagement:events  (best-effort; subs/bits/donations/gifts → points)
                                    │                 │
        command_consumer (XREADGROUP)        earn_consumer (SUBSCRIBE)
                                    │                 │
                              engagement-service (this service)
                                    │
   PUBLISH overlay:{id}:poll / :prediction → api-gateway → OBS overlay + web + viewers (broadcast, aggregate only)
   HTTP GET /viewers/me/engagement → private balance / my-vote / my-wager (pull-first; broadcast WS can't carry per-viewer data)
```

- **Commands** ride a **durable** Redis stream (a dropped vote is worse than a dropped earn). **Earning** rides best-effort Pub/Sub (a missed earn is an unpaid point, never a corrupted balance). Both are idempotent via the ledger `dedup_key`.
- **Identity** is resolved from `(platform, platform_user_id)` via the existing `viewer_platform_identities` + a race-safe `GetOrCreateViewerByPlatform` (no login needed for chat voters).

## HTTP API (`/api/v1/engagement`, proxied by api-gateway)

| Method + Path | Auth | Purpose |
|---|---|---|
| `GET /overlays/:id/active-poll` | public | aggregate poll snapshot (OBS / web render) |
| `GET /overlays/:id/active-prediction` | public | aggregate prediction snapshot |
| `POST /overlays/:id/polls` | owner · **premium** | create poll (`question`, 2–5 `options`, `allow_change?`, `duration_seconds?`) |
| `POST /overlays/:id/polls/:pollId/close` | owner | close poll |
| `POST /overlays/:id/predictions` | owner · **premium** | create prediction (`title`, 2–10 `outcomes`, `auto_lock_seconds?`) |
| `POST /overlays/:id/predictions/:pid/lock` | owner | stop accepting wagers |
| `POST /overlays/:id/predictions/:pid/resolve` | owner | settle + pay out (`winning_outcome_id`) |
| `POST /overlays/:id/predictions/:pid/cancel` | owner | void + refund all stakes |
| `GET/PUT /overlays/:id/points/config` | owner | read/update earn rules + points name |
| `POST /overlays/:id/polls/:pollId/vote` | viewer | web click-to-vote (`option_idx`) |
| `POST /overlays/:id/predictions/:pid/wager` | viewer | web wager (`outcome_idx`, `amount`) |
| `GET /viewers/me/points?overlay_id=` | viewer | balance + points name |
| `GET /viewers/me/engagement?overlay_id=` | viewer | balance + my current vote + my current wager |
| `POST /viewers/me/heartbeat` | viewer | watch-time points (deduped per minute-bucket) |

Owner routes verify overlay ownership; the shared JWT middleware accepts either a user or viewer token and the handlers enforce which each route needs.

**Premium gate (ADR-0008).** *Starting* a round — `POST …/polls` and `POST …/predictions` — is gated behind `shared/middleware.RequirePremium("engagement")`: opening a round posts the question + participate link to chat (`announce_on_start`) and thus consumes the streamer's send quota, so it is a premium capability. A non-premium owner gets `403`. Managing an already-open round (close/lock/resolve/cancel), the earn config, viewer participation, and points earning are **not** gated. The gate is seeded premium in migration 076; flip `feature_gates.is_premium=false` for `engagement` via the admin endpoint to graduate it to all users with no redeploy.

**Personal access tokens (ADR-0051).** The owner routes also accept a non-browser client presenting `Authorization: Bearer allchat_pat_<secret>`, resolved against `api_tokens` by `shared/middleware` — this service calls `middleware.SetAPITokenResolver` at startup because api-gateway forwards the client's `Authorization` header verbatim and every backend re-validates independently, so without it every PAT would be rejected here as a malformed JWT. Such a token must additionally carry the `engagement:write` scope on every owner write (`RequireAPITokenScope`), which sits **beside** the premium gate and never in place of it: a correctly scoped token belonging to a non-premium owner is still `403`'d by `RequirePremium` when starting a round, exactly as that owner's browser session is. A PAT is authentication, never an authorization bypass.

## Data model (migrations 068–076)

`viewer_points` (materialized balance, `CHECK balance>=0`), `points_transactions` (append-only ledger, `UNIQUE dedup_key`), `points_earn_config` (per-overlay multipliers + `points_name`), `polls`/`poll_options`/`poll_votes`, `predictions`/`prediction_outcomes`/`prediction_entries`. Partial unique indexes enforce one live All-Chat poll/prediction per overlay; primary keys enforce one vote/wager per viewer. Mirror-idempotency (`(overlay_id, source, external_id)`) and chat replay-dedup (`(round, source_message_id)`) uniques are created **per-overlay / per-round directly in 069/070** so the runner replaying every migration on each pod start never rebuilds a global unique over legit multi-overlay data (P0-1); 071/072 only drop the retired global names, and 074 adds `predictions.sweep_canceled`.

## Integrity highlights

- Payout = stake back + proportional split of the losers' pool, integer remainder to the largest stake; conserves points exactly (`math/big`, unit-tested). No winners → refund all.
- Wager debits under `SELECT ... FOR UPDATE` + `balance >= amt` guard; lock/resolve are guarded state transitions (first-commit-wins); auto-lock is a restart-safe periodic sweep.
- All ledger moves idempotent across retries and multi-replica Pub/Sub fan-out via `dedup_key`.

## Chat commands (ADR-0079)

A streamer's commands (`chat_commands`, migration 113) fire on the chat sources of their overlays. A **command** fires when a message's first word equals its trigger (`!discord`), a **keyword** when the trigger appears anywhere ("clip that"). Matching ignores case; a command beats a keyword; at most one fires per message.

```
engagement-service ──(SET chatcmd:channels, every 30s + on write)──► message-processor
mp forwarder: message on a listed channel ──XADD chatcmd:jobs (roles, display name)──►
chatcmd consumer (NOACK, at most once): match → min_role → echo guard → per-message dedup → cooldown
   ├─ create_clip → auth-service POST /internal/twitch/clip  (Twitch, clips:edit)
   └─ response    → render template → auth-service POST /internal/chat/announce (origin platform)
```

- **Who can use it.** `min_role`: everyone < subscriber < vip < moderator < broadcaster. Roles come from Twitch/Kick badges and YouTube owner/moderator/member flags; TikTok and Discord chatters count as everyone.
- **Where it replies.** In the chat the message came from, from the streamer's own account, and only when that channel is the streamer's own (Twitch login, `youtube_oauth_tokens`, Kick login or `kick_oauth_tokens`). TikTok, Discord and the beta platforms cannot be replied to; clips still fire from them.
- **YouTube replies are premium.** Each spends YouTube Data API quota, so a reply in YouTube chat is sent only when the owner has `users.is_premium`.
- **Loop guard.** Each sent reply is remembered for 60s (`chatcmd:echo:*`); a message with that text cannot fire a command, so a reply containing its own keyword does not answer itself.
- **Templates** (`chatcmd/`): `{user}` `{touser}` `{query}` `{1}`…`{9}` `{count}` `{random 1 6}` `{pick a | b}` `{time Europe/Berlin}` `{platform}` `{channel}` `{clip}` and `{js …}`. JavaScript runs in QuickJS (`modernc.org/quickjs`) with no host functions, 100 ms for all blocks of a reply, 16 MB memory, 256 KB stack; chat input reaches it as data, never as code. A failing block renders `[js error]`.

| Method + Path | Auth | Purpose |
|---|---|---|
| `GET /chat-commands` | streamer | the streamer's commands + `status` (premium, Twitch send/clip grants, `consent_overlay_id` for the grant flow) |
| `POST /chat-commands` | streamer · `chat_commands` gate | create (`kind`, `trigger`, `response`, `create_clip`, `min_role`, `cooldown_seconds`, `enabled`); max 100 per streamer |
| `PUT /chat-commands/:cid` | streamer · `chat_commands` gate | replace a command |
| `DELETE /chat-commands/:cid` | streamer | delete (never gated) |
| `POST /chat-commands/preview` | streamer | render a `response` against sample `input` without sending; returns `output` + `errors` |

Writes take the `engagement:write` PAT scope; paired devices are refused (the routes have no overlay to bind to). The `chat_commands` gate is seeded **open**: the only paid part is the YouTube reply check above.

## Configuration

`PORT` (8093), `DATABASE_*`, `REDIS_*`, `JWT_SECRET`/`JWT_SECRET_V1`, `ENGAGEMENT_RATE_PER_MIN` (120), `ENGAGEMENT_IDEMPOTENCY_TTL_SECONDS` (60), `ENGAGEMENT_SWEEP_INTERVAL_SECONDS` (10), `AUTH_SERVICE_URL` + `SERVICE_JWT_SECRET_V1` (announcements, chat command replies and clips; without them those are disabled). No platform API credentials: this service never calls a platform API itself, every post or clip goes through auth-service's internal routes.

## Testing

- `go test ./...` — payout conservation/rounding, command grammar, earn rules.
- End-to-end (Postgres + Redis + binary): apply migrations 068–074, seed an overlay/source/poll, `XADD engagement:commands` a `!vote`, assert `poll_votes`; `PUBLISH engagement:events` a bits event, assert `viewer_points`. See the plan for the full recipe.
- Integration (`go test -tags=integration ./repository/...`, needs Postgres with 068–074 and 113): B1 IDOR, H1 cross-economy wager, P0-1 migration re-run over multi-overlay data, P2-1 round-independent wager dedup, P2-2 active-flag reconcile set, P2-4 sweep-cancel override, chat command CRUD and channel ownership.
- `go test ./chatcmd/ ./consumer/ ./handler/` covers the template engine and JS sandbox limits, matching and roles, and the chat command consumer (roles, cooldown, dedup, echo guard, YouTube premium, clips) against miniredis.
