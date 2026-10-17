# ADR-0077: Kick Listener Follows Kick's Realtime Provider (Centrifugo or Pusher)

**Date**: 2026-10-18
**Status**: Accepted
**Deciders**: All-Chat Platform Team

## Context and Problem Statement

kick-listener reads Kick chat over Pusher, with an app key that was hardcoded
until 2026-10-17. That day Kick revoked the key, and every connect failed with
Pusher `4001 App key not in this cluster`. Kick chat was dark for about 32.5h
before anyone noticed. The listener now looks the key up before each connect
(all-chat #1072).

That lookup showed something bigger. kick.com's web client no longer starts
from a fixed Pusher key. It asks Kick which realtime provider to use:

- `POST https://web.kick.com/api/v1/realtime/connection` with
  `{"client":{"id","type":"web"},"capabilities":{"accepted_providers":[...]}}`.
  It answers with one connection, either
  `{"provider":"pusher","credentials":{"app_key","cluster"}}` or
  `{"provider":"centrifugo","credentials":{"url"}}`.
- When the client accepts both, Kick answers Centrifugo for every channel we
  sampled (18 of 18 on 2026-10-18). It hands out Pusher only to a client that
  does not accept Centrifugo. Kick is moving chat to Centrifugo, and Pusher may
  stop being offered at any time.

Measured against Kick's Centrifugo on 2026-10-18 (server `6.9.7 PRO`):

- A guest connection token comes from
  `POST https://web.kick.com/api/v1/realtime/auth/connection` `{"client_id"}`.
  It is a JWT for `guest:<client_id>`, valid 45 minutes, and needs no login.
- The protocol is Centrifugo's JSON protocol over websocket. The connect reply
  carries `expires`, `ttl` (2699s) and `ping` (25s). The server sends `{}` as a
  ping, which the client answers with `{}`. One frame can hold several
  newline-separated replies.
- Chat channels have the same names as on Pusher, `chatrooms.{id}.v2`, and are
  public: they subscribe without a channel token.
- A publication's data is `{"event","data"}`, with the same event names and
  the same JSON-string payload as Pusher, so the chat parsing is unchanged.
- **One connection may hold at most 128 subscriptions** (error
  `106 limit exceeded` from the 129th on). We watch about 600 chatrooms.
- Kick's per-channel descriptor (`/v1/realtime/channels/{id}/chat/connection`)
  points at regional servers (us-east-1, us-west-2), but a connection to the
  global server's URL receives every chatroom we tried, including ones whose
  descriptor named the other region.

## Decision Drivers

- Kick decides the transport and can change it without notice. Following its
  choice is the only thing that keeps working.
- No new single point of failure: if Kick's API is unreachable, the listener
  must still come up the way it does today.
- A way back that needs no rebuild, because kick-listener has no beta
  deployment.

## Considered Options

1. **Ask Kick at startup which provider to use, and implement both:
   the existing Pusher client and a new Centrifugo client (chosen).**
2. Stay on Pusher by never accepting Centrifugo. Works only for as long as
   Kick keeps offering Pusher to clients that refuse Centrifugo, which is
   exactly what Kick is moving away from.
3. Switch to Centrifugo only. Throws away a transport that works today, and
   leaves no fallback when Kick's API or Centrifugo misbehaves.
4. Use the per-channel descriptor and connect each chatroom to its region.
   More connections and more API calls for no measured gain: the global URL
   delivers every chatroom.

## Decision Outcome

Option 1.

- At startup the listener posts to the realtime connection endpoint with the
  providers in `KICK_REALTIME_PROVIDERS` (default `centrifugo,pusher`, what
  kick.com itself accepts), and builds the client for the provider Kick
  returns. If the lookup fails, it starts the Pusher client with the
  configured key, as before this ADR.
- Setting `KICK_REALTIME_PROVIDERS=pusher` in the deployment forces Pusher
  without a rebuild.
- The Centrifugo client (`services/kick-listener/websocket/centrifugo.go`)
  keeps a pool of connections with at most 100 chatrooms each, below Kick's
  128 limit. Each connection fetches its own guest token, refreshes it before
  it expires, answers pings, and reconnects with backoff, resubscribing its
  own chatrooms. Connections that end up empty are closed, except the first.
- Both clients satisfy one `websocket.Realtime` interface, share the event
  decoding, and drive the same `kick_listener_socket_state` gauge, so the
  `KickListenerDisconnected` alert covers either transport.
- The provider is chosen once per process. When Kick stops offering it,
  connects fail, the liveness probe restarts the pod after five silent
  minutes, and the new process asks Kick again.

### Consequences

- Good: Kick chat keeps working on whichever transport Kick prefers, and
  switching back to Pusher is a manifest change.
- Good: the chat payload is the same on both transports, so nothing
  downstream of the listener changes.
- Bad: two transports to maintain until Kick drops Pusher. The Pusher client
  can then be deleted together with `KICK_REALTIME_PROVIDERS`.
- Bad: everything here rests on undocumented Kick endpoints and limits. When
  they change, the alerts from caesar-deployment #153 are what will tell us.
- Neutral: the Pusher-only private-channel auth (`/broadcasting/auth` with a
  socket ID) has no Centrifugo equivalent here. It already failed for every
  channel in production (401), and chatrooms subscribe publicly on both
  transports.
