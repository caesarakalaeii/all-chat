# engagement-service

Cross-platform **polls, predictions, and a per-overlay viewer points economy** (issue #523).

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

## Data model (migrations 068–076)

`viewer_points` (materialized balance, `CHECK balance>=0`), `points_transactions` (append-only ledger, `UNIQUE dedup_key`), `points_earn_config` (per-overlay multipliers + `points_name`), `polls`/`poll_options`/`poll_votes`, `predictions`/`prediction_outcomes`/`prediction_entries`. Partial unique indexes enforce one live All-Chat poll/prediction per overlay; primary keys enforce one vote/wager per viewer. Mirror-idempotency (`(overlay_id, source, external_id)`) and chat replay-dedup (`(round, source_message_id)`) uniques are created **per-overlay / per-round directly in 069/070** so the runner replaying every migration on each pod start never rebuilds a global unique over legit multi-overlay data (P0-1); 071/072 only drop the retired global names, and 074 adds `predictions.sweep_canceled`.

## Integrity highlights

- Payout = stake back + proportional split of the losers' pool, integer remainder to the largest stake; conserves points exactly (`math/big`, unit-tested). No winners → refund all.
- Wager debits under `SELECT ... FOR UPDATE` + `balance >= amt` guard; lock/resolve are guarded state transitions (first-commit-wins); auto-lock is a restart-safe periodic sweep.
- All ledger moves idempotent across retries and multi-replica Pub/Sub fan-out via `dedup_key`.

## Configuration

`PORT` (8093), `DATABASE_*`, `REDIS_*`, `JWT_SECRET`/`JWT_SECRET_V1`, `ENGAGEMENT_RATE_PER_MIN` (120), `ENGAGEMENT_IDEMPOTENCY_TTL_SECONDS` (60), `ENGAGEMENT_SWEEP_INTERVAL_SECONDS` (10). No platform API credentials — this service never calls platform APIs.

## Testing

- `go test ./...` — payout conservation/rounding, command grammar, earn rules.
- End-to-end (Postgres + Redis + binary): apply migrations 068–074, seed an overlay/source/poll, `XADD engagement:commands` a `!vote`, assert `poll_votes`; `PUBLISH engagement:events` a bits event, assert `viewer_points`. See the plan for the full recipe.
- Integration (`go test -tags=integration ./repository/...`, needs Postgres with 068–074): B1 IDOR, H1 cross-economy wager, P0-1 migration re-run over multi-overlay data, P2-1 round-independent wager dedup, P2-2 active-flag reconcile set, P2-4 sweep-cancel override.
