# Instagram Listener

Ingests Instagram Live broadcast comments for a streamer's Instagram
professional account and publishes them to Redis Streams (`chat:raw`), exactly
like the other listeners. Ingest is **HTTP polling** of the Instagram Graph
API's `live_comments` edge — the realtime alternative is webhooks, which need a
public endpoint All-Chat's polling model deliberately avoids (ADR-0062). This
service is read-only: there is **no moderation write path for Instagram** (out
of plan scope), and no emotes or quota subsystem either.

**Port**: 8100
**Status**: dev-mode ready — live verification pending (needs an operator Meta
app with the Instagram product, business verification and App Review; see
below)

---

## Architecture

```
overlay_chat_sources (platform = 'instagram', channel_id = IG user ID)
  ↓ periodic sync (30s)
Manager (per-IG-user pollers)
  ↓ GET /{ig-user-id}/live_media     → resolve the currently-broadcast live video
  ↓ GET /{ig-user-id}/live_comments  → fields id,text,timestamp,username,user; comment_id=<last-seen> cursor
  ↓ publish each new comment
Redis Streams (chat:raw) → message-processor (instagram normalizer) → overlays
```

- **Cursor**: the poll sends the last-seen comment id as the `comment_id`
  query parameter, and the edge returns the comments made *after* it — an
  id cursor, not a `since` datetime (IG comments cannot be filtered by
  timestamp). The cursor is persisted per source in Redis under
  `instagram:source:state:` so a poller restart does not replay the whole live
  backlog more than once.
- **Not-live is a normal state**: `GET /{ig-user-id}/live_media` returns only
  media being broadcast at request time, so an empty set IS the offline
  signal. The listener publishes a status event, clears the cursor and keeps
  ticking at the regular poll interval — no retry-hammering.
- **No quota subsystem** (deliberate, ADR-0062): Instagram's rate limits are
  BUC-based like Facebook's, and a hobby deployment cannot approach them. The
  listener reads `X-Business-Use-Case-Usage` / `X-App-Usage` and warns at 90%.
- **No leader election / HPA**: the deployment pins one replica. The poll is
  cheap and the comment_id cursor tolerates replays, so a second replica
  would only double the call count.
- **No OAuth in this service**: it reads the token stored by auth-service in
  `instagram_oauth_tokens` (migration 096) — normally the non-expiring Page
  token for the Page linked to the streamer's IG professional account. The
  table keeps `expires_at` for the 60-day long-lived user-token variant; an
  expired row is treated as a missing credential, the source is deactivated
  and the streamer re-auths from the dashboard.

## Environment Variables

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8100` | HTTP (health/metrics) |
| `LOG_LEVEL` | `info` | zap level |
| `DATABASE_HOST/PORT/NAME/USER/PASSWORD` | — | source registry + IG tokens |
| `REDIS_HOST/PORT/PASSWORD` | — | `chat:raw` publisher |
| `TOKEN_ENCRYPTION_KEY_V1` | — | decrypts `instagram_oauth_tokens` (plaintext v0 rows work without it) |
| `INSTAGRAM_POLLING_INTERVAL_MS` | `10000` | live-comments poll interval per IG user |
| `INSTAGRAM_SOURCE_SYNC_SECONDS` | `30` | source re-sync cadence |
| `INSTAGRAM_GRAPH_URL` / `INSTAGRAM_GRAPH_VERSION` | graph.facebook.com / v26.0 | override seams (tests/proxies) |

`INSTAGRAM_APP_ID` / `INSTAGRAM_APP_SECRET` are **not** needed here — they
belong to auth-service. The listener only consumes stored tokens.

## Endpoints

| Path | Purpose |
|---|---|
| `GET /health/live` | always 200 |
| `GET /health/ready` | 200 when Redis reachable |
| `GET /status` | active poller count + Redis health |
| `GET /metrics` | Prometheus |

## Protocol sources

The request/response shapes are pinned against Meta's Instagram Graph API
v26.0 references (fetched and quoted while implementing):

- Live comments (the ingest edge; `fields=id,text,timestamp,username,user`,
  pagination via the `comment_id` param returning comments after that id):
  https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/live-comments/
- Live media discovery ("Only live video IG Media being broadcast at the time
  of the request will be returned"):
  https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/reference/ig-user/live_media/
- IG Comment node (`from{id,username}`, `parent_id`, `media{id}`; live comments
  readable only while the broadcast runs):
  https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/reference/ig-comment/
- Comments edge reference ("Comments cannot be filtered by timestamp" — why
  the cursor is an id, not a datetime):
  https://developers.facebook.com/docs/instagram-platform/instagram-graph-api/reference/ig-media/comments
- Permissions (`instagram_basic`, `instagram_manage_comments`):
  https://developers.facebook.com/docs/permissions
- Rate limiting (BUC headers):
  https://developers.facebook.com/docs/graph-api/overview/rate-limiting
- Comment Moderation guide (the permission/token matrix this service reads
  from): https://developers.facebook.com/docs/instagram-platform/comment-moderation

## Operator setup (day 1)

1. **Add the Instagram product to the existing Meta app** (the one from the
   Facebook phase, ADR-0060). Same app, so no second business verification.
2. **Configure the backend**: set `INSTAGRAM_APP_ID` and
   `INSTAGRAM_APP_SECRET` (auth-service reads them). Dev mode works
   immediately for app admins/developers/testers who hold an Instagram
   professional account linked to a Page they manage.
3. **Streamer prerequisites**: an Instagram professional (business or
   creator) account linked to a Facebook Page. Personal IG accounts cannot be
   connected.
4. **Fold the permissions into the next App Review cycle.** Requesting
   Advanced Access for `instagram_basic` and `instagram_manage_comments` in a
   separate submission would mean a second ~20-day review; the plan batches
   them with Facebook's. Screencast: the Facebook Login consent on All-Chat
   and live IG comments appearing on an overlay.
5. After approval, any streamer can connect: dashboard → overlay settings →
   add source → Instagram → consent → All-Chat stores the token (encrypted)
   and adds the source.

## Troubleshooting

- **Source shows offline / token errors**: the IG token expired (the 60-day
  variant) or was revoked. Re-auth from the dashboard — there is no background
  refresh; the row is replaced wholesale on re-consent.
- **Nothing published but poller is running**: the account is probably not
  live. `live_media` returns only currently-broadcast media; comments are only
  readable while the broadcast runs. Check `GET /status` for the poller count
  and start a live broadcast.
- **403 on source add**: the `platform_instagram` feature gate is closed for
  your account (ADR-0008 rollout pattern). An admin flips it via the
  feature-gates admin endpoint — no redeploy.
- **Gate closed but sources exist**: sources created while the gate was open
  keep working; the gate only blocks new adds.

## Building and testing

```bash
cd services/instagram-listener
go build ./...
go vet ./...
go test ./...
gofmt -l .   # expect no output
```

Live end-to-end verification requires a Meta app with an Instagram
professional account going live; unit tests cover the client against a fake
Graph server (`client/client_test.go`), and the normalizer fixtures live in
message-processor (`normalizer/instagram_normalizer_test.go`).

---

See [ADR-0062](../../docs/adr/0062-instagram-live-comments-polling.md) for
the polling-vs-webhooks decision, the token model and what is deliberately out
of scope.
