# ADR-0082: Altstream chat via the official public SSE chat API

**Date**: 2026-10-31
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

Altstream (https://altstream.online) is a young live streaming platform that
streamers have started multistreaming to next to Twitch. In October 2026 six
streamers asked on Bluesky for All-Chat to merge Altstream chat into their
overlay. It is the first platform request since the expansion plan that comes
from existing users rather than from a market survey.

Altstream is small, community-funded, queer-friendly and explicitly against
generative AI. Those values fit All-Chat, and supporting the platform is a
choice we want to make, not only a technical one. This is the same input that
ADR-0061 weighed the other way: Rumble support was withdrawn partly because of
Rumble's political affiliations, which we do not want to promote. Here the
platform's values are a reason to add support, not a cost to accept.

Facts that shape the design (verified live on 2026-10-31):

- **There is an official, documented, unauthenticated developer API**
  (https://altstream.online/documentation, "External Developer API" tab).
  No API key, no OAuth, CORS `*`. REST endpoints are limited to 60 requests per
  minute per IP.
  - Live status: `GET /api/v1/channels/<username>`.
  - Recent chat (last 24 h, oldest first): `GET /api/chat?channel=<username>&limit=N`.
  - Channel emotes: `GET /api/v1/channels/<username>/emotes`, with
    site-relative image URLs.
  - Live chat: Server-Sent Events, `GET /api/chat/stream?channel=<username>&lastId=<id>`.
    Each frame is `data: [ <event>, ... ]` (a JSON array, usually one event),
    with a `: ping` comment about every 25 s.
- **The channel key is the Altstream username**, case-insensitive. There is no
  numeric user id; `authorUsername` is the stable identity of a chatter.
- **The SSE events are richer than the REST ones**: they carry
  `authorUsername`, `authorImage`, `authorPronouns`, `roles` and `badges`,
  which the REST recent-chat response lacks. Event ids are numeric strings and
  are not monotonic.
- **Event types** seen live: `message`, `highlight` (a paid highlighted
  message), `delete` (one message, by `targetId`), `clear` (wipe the chat),
  `redemption`, `raid`, `system` and `shoutout`. The docs also list `tip` and
  `subscription`.
- **Emotes** appear in the text as `:name:` tokens. A chatter may use the
  emotes of any channel they follow, so a token can belong to a channel other
  than the one being read, and there is no anonymous global emote lookup.

## Considered Options

1. **The official SSE chat stream plus REST** for status and emotes.
2. **Polling the REST recent-chat endpoint.** Every poll of every channel
   counts against the 60 requests per minute per IP that the whole cluster
   shares, so it caps the number of channels at a low number and adds seconds
   of latency. The REST events also lack the chatter's username, avatar,
   pronouns and badges.
3. **An internal or scraped surface** (the website's own client bundle, the
   rendered chat DOM), the route Picarto (ADR-0059) and Rumble (ADR-0061)
   needed. Unnecessary when a documented API exists, and it would break with
   any site deploy.
4. **Not supporting Altstream.** Rejected: the demand is real, the values fit,
   and option 1 carries none of the API risk that sank Rumble.

## Decision Outcome

**Chosen: option 1.** A new `altstream-listener` service (port 8101) reads
each watched channel's chat over the official SSE stream and uses the
documented REST endpoints for live status and emote catalogs. It publishes to
`chat:raw` like every other listener, and a message-processor normalizer maps
the events into the unified format.

**Scope of v1:**

- `message` events, and `highlight` events rendered as a chat message (text
  from `data.text`, marked highlighted with the reward name).
- Native deletions: `delete` becomes a single deletion of `targetId`, `clear`
  becomes a chat clear, published the same way as the other listeners'
  deletions. Altstream's message id is its native id, so the message-processor
  registers it in the msgid registry itself (`selfRegisteredPlatforms`) and a
  native delete resolves to the rendered message. The buffered-deletion drain
  now runs after the message is published, so a delete that overtakes its
  message still removes it.
- Every other event type (`redemption`, `raid`, `system`, `shoutout`, `tip`,
  `subscription`) is dropped and counted in a metric, labelled by type.
- **Read-only.** No message sending and no moderation write path: Altstream has
  no app login flow to act on a streamer's behalf.
- **Emotes** are resolved from one shared name-to-URL map built from the emote
  catalogs of every channel the listener currently watches. A `:name:` token
  that is not in the map stays plain text.

**Rollout, as early access (ADR-0020):**

- Migration 117 seeds the `supported_platforms` row (`requires_oauth = false`)
  and the `platform_altstream` feature gate with `early_access = TRUE` and
  `is_premium = FALSE` (ADR-0008), so only beta testers and ambassadors can add
  an Altstream source.
- The frontend offers Altstream on every host, but on allch.at only to
  early-access users (`EARLY_ACCESS_PLATFORMS` in
  `frontend/src/lib/platform-availability.ts`).
- The whole feature ships on `main`, and the listener image runs from `:main`.
- Release to everyone by removing Altstream from `EARLY_ACCESS_PLATFORMS`, then
  turning `early_access` off on the gate in `/admin/features`.

### The `lastId` gotcha

The `lastId` query parameter on the SSE stream is **mandatory and
undocumented**. Without it, or with it empty, the stream sends only `: ping`
comments forever and looks healthy while delivering nothing. The website's own
client always sends it, which is how this was found.

- With a `lastId` that matches the newest message, the stream sends only new
  messages.
- With a `lastId` that does not match (an old id, or a sentinel such as
  `none`), it first sends the newest one to three existing messages, then live
  ones. It does **not** replay the whole gap after an old id.

The listener therefore always sends a non-empty `lastId`, resumes with the last
id it saw, drops events it has already seen, and on a fresh connect drops the
catch-up events older than the connection itself.

## Consequences

**Good:**

- A documented, unauthenticated API: no credentials to store, rotate or leak,
  and no auth-service change.
- The same no-OAuth, channel-name-keyed SSE shape the Rumble listener proved
  (ADR-0061), without its ToS grey zone.
- Streamers who multistream to Altstream get their chat into the same overlay,
  and All-Chat supports a platform whose values it shares.

**Risks, accepted knowingly:**

- **The platform is young and fragile.** Altstream is about three months old,
  run by one developer, and had frequent outages in October 2026. Expect the
  listener to spend time reconnecting through outages that are not ours to
  fix.
- **The API is new** (its first third-party wrapper calls it "brand new") and
  the documentation already lags the server (`lastId`), so it may change shape
  while it settles. The parser is defensive (unknown types dropped, not
  fatal), and the early-access rollout limits who is affected.
- **One SSE connection per watched channel**, all leaving from the cluster's
  egress IP. Twelve parallel connections from one IP worked in testing; there
  is no documented limit. If the number of watched Altstream channels grows
  large, ask Altstream's developer about a limit or a multiplexed stream.
- **No full gap replay.** After a disconnect only the newest one to three
  messages come back; anything older sent during the gap is lost.
- **Cross-channel emotes cannot always be resolved.** A token from a channel
  the listener does not watch renders as plain `:name:` text.
- **No alerts.** Raids, redemptions, tips and subscriptions are dropped; the
  alert-processor does not handle Altstream. Adding them is a separate
  decision.
- The REST limit (60 requests per minute per IP) is shared by every replica
  behind the same egress IP, so status and emote catalog refreshes must stay
  well below it.

## Related

- ADR-0008: feature gates (`platform_altstream`).
- ADR-0020: early access for beta testers and ambassadors.
- ADR-0059: Picarto, the rollout this mirrors.
- ADR-0061: Rumble, withdrawn; the closest technical precedent and the
  opposite values outcome.
- `services/altstream-listener/README.md`
