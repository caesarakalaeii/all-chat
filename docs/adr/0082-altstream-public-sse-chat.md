# ADR-0082: Altstream chat via the official public SSE chat API

**Date**: 2026-10-31
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context

Altstream (https://altstream.online) is a young live streaming platform streamers have
started multistreaming to next to Twitch; six asked on Bluesky in October 2026 for its
chat in their overlay, the first platform request from existing users rather than a
market survey. Altstream is small, community-funded, queer-friendly and explicitly
against generative AI; its values fit All-Chat, the same weight ADR-0061 gave Rumble the
other way.

Verified live on 2026-10-31: an official, documented, unauthenticated developer API
(https://altstream.online/documentation), no key or OAuth, CORS `*`, REST limited to 60
req/min/IP: `GET /api/v1/channels/<username>` (status), `GET /api/chat?channel=...&limit=N`
(24 h recent chat), `GET /api/v1/channels/<username>/emotes`, and SSE chat at
`GET /api/chat/stream?channel=<username>&lastId=<id>` (frames `data: [...]`, `: ping` every
25 s). The channel key is the username, case-insensitive. SSE events carry more than REST
(avatar, pronouns, roles, badges); ids are numeric strings, not monotonic. Event types:
`message`, `highlight`, `delete`, `clear`, `redemption`, `raid`, `system`, `shoutout`
(docs also list `tip`, `subscription`). Emotes appear as `:name:` tokens and a chatter may
use any followed channel's emotes, so a token can belong to an unwatched channel.

## Considered options

- Poll the REST recent-chat endpoint: every poll of every channel counts against the
  60/min shared limit, caps channel count at a low number, adds latency, and REST events
  lack username, avatar, pronouns and badges.
- An internal or scraped surface (the route Picarto, ADR-0059, and Rumble, ADR-0061,
  needed): unnecessary when a documented API exists, and it would break with any deploy.
- Not support Altstream: rejected: the demand is real, the values fit, and the official
  API carries none of the risk that sank Rumble.

## Decision

Chosen: the official SSE stream plus REST. A new `altstream-listener` (port 8101) reads
each watched channel's chat over SSE and uses REST for live status and emote catalogs,
publishing to `chat:raw` like every listener with a message-processor normalizer. Scope
of v1: `message` events; `highlight` rendered as a highlighted chat message; native
`delete` (single message by `targetId`) and `clear`, with Altstream's message id
self-registered in the msgid registry so native deletes resolve to the rendered message
(the buffered-deletion drain runs after the message is published); every other event type
dropped and counted in a labeled metric. Read-only: no send or moderation path (Altstream
has no app login flow to act for a streamer). Emotes resolve from one shared name-to-URL
map of every watched channel's catalog; an unknown token stays plain text.

Rollout as early access (ADR-0020): migration 117 seeds `supported_platforms`
(`requires_oauth = false`) and the `platform_altstream` gate with `early_access = TRUE`,
`is_premium = FALSE`; the frontend offers Altstream on every host but on allch.at only to
early-access users (`EARLY_ACCESS_PLATFORMS` in `frontend/src/lib/platform-availability.ts`);
the listener runs from `:main`. Release by removing Altstream from
`EARLY_ACCESS_PLATFORMS` and turning `early_access` off in `/admin/features`.

**The `lastId` gotcha.** The SSE `lastId` parameter is mandatory and undocumented:
without it, or empty, the stream sends only `: ping` comments forever and looks healthy
while delivering nothing (the website's own client always sends it). A matching `lastId`
yields only new messages; a stale one yields the newest one to three existing messages
then live ones, never a full gap replay. The listener therefore always sends a non-empty
`lastId`, resumes with the last id it saw, drops already-seen events, and on a fresh
connect drops catch-up events older than the connection.

## Consequences

- No credentials to store, rotate or leak, and no auth-service change; the no-OAuth
  channel-name SSE shape Rumble proved without its ToS grey zone.
- Accepted risks: a three-month-old, one-developer platform with frequent October 2026
  outages (listener spends time reconnecting); a new API whose docs already lag the
  server; one SSE connection per watched channel from the cluster egress IP (twelve
  parallel worked in testing, no documented limit; ask the developer about one if growth
  makes it needed); no full gap replay after a disconnect; unwatched channels' emotes stay
  `:name:` text; no alerts (raids, redemptions, tips, subscriptions dropped); and the
  60/min REST limit shared by every replica behind one egress IP, so status and emote
  refreshes must stay well below it.
