# Quick Reference: Add New Platform

Goal: add support for a new streaming platform. Recent worked examples: `migrations/117_altstream_support.sql` + `services/altstream-listener/` (SSE), `migrations/091_owncast_platform.sql` + `services/owncast-listener/` (websocket), `migrations/039_discord_platform.sql` (relay).

## Architecture

Listener service -> Redis Stream `chat:raw` -> message-processor normalizer -> Redis Pub/Sub `overlay:{id}` -> api-gateway WebSocket. See [docs/architecture/01-DATA-FLOW.md](../architecture/01-DATA-FLOW.md).

Template choice by chat transport:

| Transport | Template | Pattern |
|---|---|---|
| IRC | `services/twitch-listener/` | IRC client with join/part |
| WebSocket | `services/kick-listener/` or `services/owncast-listener/` | WS client with subscriptions |
| HTTP/SSE polling | `services/altstream-listener/` or `services/youtube-listener-innertube/` | Polling client with leader election |

## 1. Listener service

Create `services/<platform>-listener/` following the standard layout (`cmd/main.go`, `handlers/`, channels + client packages, `Dockerfile`, `README.md`). Use the module path `github.com/caesar/all-chat/services/<platform>-listener` and a `replace` directive to `../../shared` (copy from an existing listener's `go.mod`). Copy the closest template, then:

- Set the service name in the logger and health handlers.
- Pick the next free port (check `deployments/k8s/base/` and `Makefile`).
- Client: connect with retries, subscribe to channels, reconnect on disconnect, graceful shutdown.
- `channels/repository.go`: query active sources for the platform from `overlay_chat_sources` (see `services/kick-listener/channels/repository.go` for the current query shape), join new channels, leave removed ones on a sync interval.
- `publisher/`: `XADD` to stream `chat:raw` with `platform`, `overlay_id`, `channel_id`, `channel_name`, `raw_message` (platform message as JSON), `timestamp`. Match what the template publishes exactly; the normalizer parses these fields.
- Health: `/health/live` (always 200) and `/health/ready` (checks DB/Redis).

## 2. Message-processor normalizer

- Add `services/message-processor/normalizer/<platform>_normalizer.go` with a `Parse<Platform>(rawMsg map[string]interface{})` producing `models.UnifiedMessage` (user, badges, emotes, text, timestamps, metadata). Follow the pattern of `goodgame_normalizer.go` or `discord_normalizer.go`.
- Add a `case` for `"<platform>"` in the router switch (`services/message-processor/router/`).

## 3. Migration

`migrations/0NN_<platform>_platform.sql` (next number; some numbers repeat, see `migrations/004_*`). Copy `091_owncast_platform.sql`:

- `INSERT INTO supported_platforms (platform, display_name, is_enabled, requires_oauth, config_schema) ... ON CONFLICT (platform) DO NOTHING`
- Seed the ADR-0008 feature gate: `INSERT INTO feature_gates (feature_key, is_early_access, is_premium, description) VALUES ('platform_<new>', ...)`; gate key `platform_<new>`. Beta platforms start premium (beta testers get premium), early-access ones with `is_early_access=TRUE`.
- Everything idempotent: services run the full migration set on pod restart, so `ON CONFLICT DO NOTHING` / `IF NOT EXISTS` everywhere. Add the matching `_down.sql`.

## 4. Deployment

Kustomize base under `deployments/k8s/base/<platform>-listener/` (Deployment, Service, HPA, kustomization): copy an existing listener, keep `/health/live` + `/health/ready` probes, pull DB credentials from the CNPG secret and platform tokens from encrypted-token env (`TOKEN_ENCRYPTION_KEY_V<n>`).

## 5. Frontend

Add the platform to `frontend/src/lib/platform-availability.ts` (included in `PLATFORMS`, plus any beta/early-access list it belongs in) so the source picker, onboarding and `/upgrade` pages show it. Beta platforms are listed on `beta.allch.at` only; early-access ones on both hosts but only for beta testers/ambassadors.

## 6. Docs

- `services/<platform>-listener/README.md` (env vars, message format, how to run locally).
- ADR describing the connection choice (see `docs/adr/README.md` index for recent examples), then CLAUDE.md's platform status list.

## 7. Testing

- Unit tests for the client and the normalizer (`go test ./... -v` in each module; `make test` runs them all).
- Local end-to-end: `make docker-up`, add the platform as a source on a test overlay (`make frontend-seed`, then the editor), and watch `redis-cli XREAD COUNT 10 STREAMS chat:raw` plus the overlay WebSocket.

## Common issues

- Messages reach the listener but not `chat:raw`: check redis connectivity, the stream key spelling, and the field names the normalizer expects.
- Channels never join: `overlay_chat_sources` query must match by `platform = '<new>'` with active overlays.
- Normalizer parse errors: log the raw message payload and diff against the parser.
