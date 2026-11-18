# Frontend Quick Start

Minimal backend for fast frontend iteration: `make frontend-dev` starts
PostgreSQL, Redis, API Gateway, Overlay Manager, Message Processor and Share
Service, but no auth service, emote service, platform listeners, token
refresh service or source manager.

```bash
make frontend-dev     # start services (about 30 s)
make frontend-seed    # create test overlay and chat sources
make frontend-verify  # check everything is up
```

Then, in two terminals:

```bash
make frontend-messages    # mock chat messages every 3 s
cd frontend && npm install && npm run dev
```

Visit http://localhost:3000. All dev defaults work without secrets. This setup
has no authentication, no emotes and no real platform connections; it is for
frontend development only. For full testing use `deployments/docker-compose.yml`.

`scripts/quick-start-frontend.sh` does the same in one command.

## What gets seeded

- User `teststreamer` (`00000000-0000-0000-0000-000000000001`)
- Overlay `00000000-0000-0000-0000-000000000002`
- Twitch channel `teststreamer` and YouTube channel `UCtest12345` sources
- WebSocket: `ws://localhost:8080/ws/overlay/00000000-0000-0000-0000-000000000002`

## Common commands

```bash
docker-compose -f docker-compose.frontend.yml logs -f [service]
make frontend-down    # stop, keep data
make frontend-reset   # stop and reset
```

Full cleanup: `docker-compose -f docker-compose.frontend.yml down -v`
(add `--rmi all` to free images). Services restart automatically on crash
(`restart: unless-stopped`) and use separate volumes, so they can run next to
the main compose stack.

## Development loop

Next.js hot-reloads, so the running backend needs no restart. Restart services
only when changing API Gateway code, the database schema or message-processor
logic.

## Customizing messages

```bash
MESSAGE_INTERVAL=1 ./scripts/generate-test-messages.sh     # 1 msg/s
MESSAGE_COUNT=100 MESSAGE_INTERVAL=0.1 ./scripts/generate-test-messages.sh
TEST_OVERLAY_ID=<id> ./scripts/generate-test-messages.sh
```

Edit `scripts/generate-test-messages.sh` to change texts, usernames or
platforms. Sending a single message directly:

```bash
curl -X POST http://localhost:8087/internal/mock-messages \
  -H "Content-Type: application/json" \
  -H "X-Internal-Token: dev-frontend-key" \
  -d '{"overlay_id": "00000000-0000-0000-0000-000000000002",
       "platform": "twitch", "username": "APIUser", "text": "Hello!"}'
```

## API endpoints

- Overlay Manager (`:8082`) mounts routes at the root path, no `/api/overlays`
  prefix. Most endpoints need a JWT; easiest is through the gateway
  (`/api/v1/overlays/...` on :8080). Public read-only:
  `GET /public/:id/config`.
- API Gateway (`:8080`): WebSocket at `/ws/overlay/:id`, health at `/health/live`.

WebSocket test: `node scripts/test-websocket.js` (after `cd scripts && npm install`),
or `wscat -c ws://localhost:8080/ws/overlay/00000000-0000-0000-0000-000000000002`.

## Debugging

```bash
curl http://localhost:8080/health/live   # API Gateway
curl http://localhost:8082/health/live   # Overlay Manager
curl http://localhost:8087/health/live   # Message Processor
redis-cli SUBSCRIBE "overlay:00000000-0000-0000-0000-000000000002"
redis-cli XREAD COUNT 10 STREAMS chat:raw 0-0
PGPASSWORD=dev_password_123 psql -h localhost -U allchat -d allchat \
  -c 'SELECT id, name, is_active FROM overlays;
      SELECT platform, channel_name, is_active FROM overlay_chat_sources;'
```

Reconnection test: `docker-compose -f docker-compose.frontend.yml restart api-gateway` mid-stream.

## Services and ports

| Service | Port |
|---|---|
| PostgreSQL | 5432 |
| Redis | 6379 |
| API Gateway | 8080 |
| Overlay Manager | 8082 |
| Message Processor | 8087 |
| Share Service | 8090 |

## Troubleshooting

- **Services won't start**: check ports with `lsof -i :5432,:6379,:8080,:8082,:8087`;
  stop the full stack (`cd deployments && docker-compose down`) if it holds them.
- **Postgres not ready**: `docker-compose -f docker-compose.frontend.yml logs postgres`.
- **No messages appearing**: check the message-processor health endpoint and the
  Redis subscription above; re-seed with `make frontend-seed`.
