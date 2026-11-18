# Architecture Overview

All-Chat aggregates chat messages from multiple live streaming platforms
(Twitch, YouTube, Kick, TikTok, Discord, plus beta platforms) and displays them
on streaming overlays. Standard microservices in Go (two services in
TypeScript/Node: tiktok-listener, discord-bot) around PostgreSQL (CloudNativePG)
and Redis 7 (Streams for durability + Pub/Sub for fan-out).

```
Platforms -> listeners (one service each) -> Redis Stream chat:raw
  -> message-processor (normalize + emote enrich) -> Redis Pub/Sub overlay:*
  -> api-gateway WebSocket -> overlay frontend (Next.js)
```

Explicit non-goals: not a chat bot (no outbound messages to platforms), not a
chat archive (messages are transient, stream capped at 100k entries), single
region.

## Services

Full list with code paths lives in `services/`; each has its own README. The
load-bearing ones for the message path:

- **Listeners** (`services/*-listener`): one per platform. Publish raw messages
  to `chat:raw`. Gate new platforms behind the `platform_<name>` feature gate
  (ADR-0008).
- **message-processor**: consumes `chat:raw` in group `message-processor`,
  normalizes per platform, enriches emotes/badges, routes to overlays, publishes
  to `overlay:*` channels and `chatcmd:jobs` for chat commands.
- **alert-processor**: separate consumer group `alert-processors` on the same
  stream; feeds alerts/goal/list overlays and leaderboard scoring.
- **api-gateway**: WebSocket hub at `/ws/overlay/:id` + viewer route, HTTP
  reverse proxy to the platform services.
- **emote-service**: aggregates 7TV/BTTV/FFZ, HTTP API + Redis cache.
- **source-manager**: active source registry, demand snapshots, leader
  election (YouTube listener).
- **overlay-manager / auth-service**: CRUD and OAuth/JWT for streamers.
- **engagement-service**: polls, predictions, viewer points, chat commands.
- **moderation-service**: cross-platform delete/timeout/ban write path.
- **payment-service**: Patreon entitlements (streamer + viewer premium).
- **token-refresh-service**: background OAuth refresh, 1 replica.
- **share-service**: share links, admin feature gates, ambassador showcase.
- **support-bot**: Discord support/admin agent (Go).
- **discord-bot**: community ops bot (TypeScript).
- **tiktok-signer / tiktok-relay**: webcast signing and US-region lookup for
  tiktok-listener.
- **youtube-quota-monitor**: reads shared quota table, publishes quota alerts.

## Technology

| Area | Choice |
|---|---|
| Backend | Go 1.26+, Gin, pgx/v5, go-redis/v9, Zap |
| Frontend | React 19+, Next.js 16+ App Router, TypeScript, Tailwind |
| Deploy | Kubernetes, Keel-driven image deploys, CNPG operator |

## Key decisions

Why each was chosen lives in the ADR, not here:

- Standard Go layout, no hexagonal layer (ADR-0001, ADR-0004).
- Redis Streams + Pub/Sub hybrid for the message path (ADR-0002).
- CloudNativePG for PostgreSQL (ADR-0003).
- React + Next.js frontend (ADR-0005).
- YouTube quota reserve-confirm-rollback (ADR-0006).
- Leadership rebalancing for auto-scaled listeners (ADR-0007).
- Feature gates (`feature_gates` table) for premium/early access, replacing
  deploys with a config change (ADR-0008).

## Full ADR index

`docs/adr/README.md`.
