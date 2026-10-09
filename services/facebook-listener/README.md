# Facebook Listener

Ingests Facebook Live video comments for a streamer's Page and publishes them
to Redis Streams (`chat:raw`), exactly like the other listeners. Ingest is
**HTTP polling** of the Graph API — Facebook has no public realtime comment
API (ADR-0060). The moderation write path (delete comment, ban/unban page
user) lives in moderation-service; this service is read-only.

**Port**: 8099
**Status**: dev-mode ready — live verification pending (needs an operator
Meta app, business verification and App Review; see below)

---

## Architecture

```
overlay_chat_sources (platform = 'facebook', channel_id = Page ID)
  ↓ periodic sync (30s)
Manager (per-Page pollers)
  ↓ GET /{page-id}/live_videos        → resolve the LIVE_NOW broadcast
  ↓ GET /{live-video-id}/comments     → order=reverse_chronological, since=<newest created_time>
  ↓ publish each new comment
Redis Streams (chat:raw) → message-processor (facebook normalizer) → overlays
```

- **No quota subsystem** (deliberate, ADR-0060): Meta's page-token rate limit
  is `4800 × engaged users` per rolling 24 h. A hobby deployment cannot
  approach it. The listener reads `X-Business-Use-Case-Usage` and warns at
  90%.
- **No leader election / HPA**: the deployment pins one replica. The poll is
  cheap and the `since` cursor is idempotent, so a second replica would only
  double the call count.
- **No OAuth in this service**: it reads the non-expiring **Page access
  token** stored by auth-service in `facebook_oauth_tokens` (migration 094).
  Page tokens from a long-lived user token do not expire (ADR-0060), so there
  is no refresh path; an invalidated token surfaces as a Graph 401 and the
  streamer reconnects from the dashboard.

## Environment Variables

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8099` | HTTP (health/metrics) |
| `LOG_LEVEL` | `info` | zap level |
| `DATABASE_HOST/PORT/NAME/USER/PASSWORD` | — | source registry + page tokens |
| `REDIS_HOST/PORT/PASSWORD` | — | `chat:raw` publisher |
| `TOKEN_ENCRYPTION_KEY_V1` | — | decrypts `facebook_oauth_tokens` (plaintext v0 rows work without it) |
| `FACEBOOK_POLLING_INTERVAL_MS` | `10000` | comments poll interval per live Page |
| `FACEBOOK_SOURCE_SYNC_SECONDS` | `30` | source re-sync cadence |
| `FACEBOOK_GRAPH_URL` / `FACEBOOK_GRAPH_VERSION` | graph.facebook.com / v26.0 | override seams (tests/proxies) |

`FACEBOOK_APP_ID` / `FACEBOOK_APP_SECRET` are **not** needed here — they
belong to auth-service. The listener only consumes stored page tokens.

## Endpoints

| Path | Purpose |
|---|---|
| `GET /health/live` | always 200 |
| `GET /health/ready` | 200 when Redis reachable |
| `GET /status` | active poller count + Redis health |
| `GET /metrics` | Prometheus |

## Protocol sources

The request/response shapes are pinned against Meta's Graph API v26.0
references (fetched and quoted while implementing):

- Live video discovery: https://developers.facebook.com/docs/graph-api/reference/page/live_videos/ — paged edge; `status` is a LiveVideo node field (`LIVE_NOW`, `SCHEDULED_*`, `UNPUBLISHED`), filtered client-side.
- Comments edge: https://developers.facebook.com/docs/graph-api/reference/live-video/comments/ — `order` (`reverse_chronological` recommended for live), `since` datetime lower bound, cursor `paging.cursors.after`.
- Comment node (`id`, `message`, `created_time`, `from{id,name}`, `parent{id}`): https://developers.facebook.com/docs/graph-api/reference/comment/
- Token chain / non-expiry: https://developers.facebook.com/docs/facebook-login/guides/access-tokens/get-long-lived — *"Long-lived Page access token do not have an expiration date."*
- Rate limiting (BUC `4800 × engaged users`, headers): https://developers.facebook.com/docs/graph-api/overview/rate-limiting
- Permissions: https://developers.facebook.com/docs/permissions (`pages_read_engagement`, `pages_manage_engagement`)

Community reference implementations used as cross-checks: AxelChat's
Facebook adapter (github.com/axelnetwork/axelchat) and Social Stream Ninja's
comment polling.

## Operator setup (day 1)

1. **Create the Meta app**: developers.facebook.com → Create App → type
   *Business*. Add the **Facebook Login** product. Under Facebook Login →
   Settings, add `<FRONTEND_URL>/api/v1/auth/facebook/callback` as a valid
   OAuth redirect URI.
2. **Configure the backend**: set `FACEBOOK_APP_ID` and `FACEBOOK_APP_SECRET`
   (auth-service reads them). Dev mode works immediately against Pages owned
   by the app's admins/developers/testers.
3. **Business verification — start early.** Advanced Access on
   `pages_read_engagement` and `pages_manage_engagement` requires a verified
   business, and verification + review commonly takes ~20 days. Begin before
   you need production traffic.
4. **App Review for both permissions.** Screencast: the Facebook Login
   consent on All-Chat; a live Page's comments appearing on an overlay; a
   comment deleted and a user blocked from the moderation panel.
5. After approval, any streamer can connect: dashboard → overlay settings →
   add source → Facebook → consent → All-Chat stores the Page token
   (encrypted) and adds the source.

## Building and testing

```bash
cd services/facebook-listener
go build ./...
go vet ./...
go test ./...
gofmt -l .   # expect no output
```

Live end-to-end verification requires a Meta app with a test Page going live;
unit tests cover the client against a fake Graph server
(`client/client_test.go`), and the normalizer fixtures live in
message-processor (`normalizer/facebook_normalizer_test.go`).

---

See [ADR-0060](../../docs/adr/0060-facebook-graph-api-and-moderation.md) for
the polling-vs-webhooks decision, the token model and the rate-limit analysis.
