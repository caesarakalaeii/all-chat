# ADR-0058: An Owncast source is the instance URL, not a channel name

**Date**: 2026-09-08
**Status**: Accepted
**Deciders**: caesarakalaeii

## Context and Problem Statement

Every chat platform All-Chat supports addresses content by a *channel name* or
platform ID: a Twitch username, a YouTube `UC...` id, a Kick slug, a TikTok
username, a Discord channel id. `overlay_chat_sources.channel_identifier`
stores that name and the source picker asks for it.

[Owncast](https://owncast.online) does not have channels. It is a self-hosted
streaming server whose instances serve **one stream each**; there is no
instance-side channel to name. The only identity that picks out the content is
the instance's base URL (e.g. `https://watch.example.org`). Its chat is
read-only and open by design: an anonymous client `POST`s `/api/chat/register`
(unauthenticated), receives an `{id, accessToken, displayName}` triple, and
opens the websocket at `/ws?accessToken=...` to receive chat events.

A second, less obvious difference: an Owncast instance is somebody's own
server. It goes offline, upgrades, or changes version **independently of
All-Chat**, with no signal to us. A listener that treats an unreachable
instance as a fault will crash-loop or spam errors against a temporary
outage that is entirely correct to tolerate.

## Decision

**1. The instance URL is the channel.** `channel_identifier` stores the
instance base URL, normalized to `scheme://host` — lowercase host, no path, no
trailing slash, no fragment, no embedded credentials. `HandleAddSource`
rejects anything else with 400, so every row for `platform='owncast'` is a
connectable target by construction.

**2. Display name from the instance, best-effort.** On add, overlay-manager
fetches `<url>/api/config` and stores its `name` as `channel_name`. Any
failure — connection refused, non-200, unusable body — **falls back to the URL
hostname and does not fail the add**. An instance being down at add time is
the single most predictable way to lose a new user, and a hostname label is
trivially editable later; blocking the add for it buys nothing.

**3. Offline is a normal state.** `owncast-listener` holds one websocket
connection per instance URL and reconnects with capped exponential backoff
(2s doubling to a 5-minute cap). While the instance is unreachable it:
publishes `platform:status` `offline` with a `next_retry_at` (the ADR-0032
producer contract, so overlay clients self-heal on recovery), marks
`overlay_chat_sources.is_active = false`, and logs retries at Warn for the
first attempts and Debug thereafter — no Error spam, no crash-loop, no pod
restarts. The liveness probe never depends on per-instance connectivity;
readiness gates only on Redis.

**4. Registration per connection.** The `/api/chat/register` call is repeated
on every reconnect rather than cached, because the access token is
instance-session-scoped and an instance upgrade/restart invalidates it.
Re-registering is one unauthenticated POST against the instance and is
idempotent in effect.

**5. Chat only.** Only `type: "CHAT"` events are published to `chat:raw`.
Join/part, name changes, system messages, stream events and actions are
logged-and-dropped: the overlay shows chat, and every other event type is out
of scope for this platform's plan. `body` arrives as server-rendered,
sanitized HTML (markdown + emoji rendered in the Owncast server) and is passed
through as text; there is no client-side emote or badge extraction to do.
`displayColor` is an index into Owncast's palette, stored as `owncast:<n>`
rather than invented into a CSS color.

## Consequences

- The source picker for Owncast is a URL input, not a name lookup — the only
  platform where "add source" asks for a URL. Frontend handled separately.
- No OAuth, no `NAMEABLE_PLATFORMS` entry, no auth-service change: chat is
  read-only and unauthenticated by design.
- Deployment adds `owncast-listener` (port 8095) and migration 091
  (`supported_platforms` row + `platform_owncast` premium feature gate seed,
  ADR-0008). No Owncast-specific env vars are required anywhere.
- The per-connection manager diverges structurally from kick-listener's
  one-socket-many-subscriptions model. That is the honest shape for platforms
  where the unit of connection is the customer's own server, and the backoff
  logic is a pure function unit-tested at the cap.
- A URL concentration warning: many overlays pointing at the same instance
  URL share one connection (the manager keys connections by URL and fans
  messages out per overlay), so a busy instance loads its own server once,
  not once per overlay.

## Related

- ADR-0008 (feature gates) — the `platform_owncast` seed.
- ADR-0032 (source-liveness heartbeat and overlay self-heal) — the
  `platform:status` connected/offline contract this listener honors.
- Migration 091 (`091_owncast_platform.sql` / `_down.sql`).
