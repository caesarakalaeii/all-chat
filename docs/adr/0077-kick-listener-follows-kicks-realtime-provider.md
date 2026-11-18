# ADR-0077: Kick Listener Follows Kick's Realtime Provider (Centrifugo or Pusher)

**Date**: 2026-10-18
**Status**: Accepted
**Deciders**: All-Chat Platform Team

## Context

kick-listener read Kick chat over Pusher with a hardcoded app key until 2026-10-17, when
Kick revoked it (every connect failed with Pusher `4001 App key not in this cluster` and
Kick chat went dark for about 32.5 h before anyone noticed; the listener now looks the
key up before each connect). The lookup revealed the bigger change: kick.com's web client
asks Kick which realtime provider to use. `POST https://web.kick.com/api/v1/realtime/
connection` with the accepted providers answers one connection, either Pusher
(`{app_key, cluster}`) or Centrifugo (`{url}`). When the client accepts both, Kick answers
Centrifugo for every channel sampled (18 of 18); it hands Pusher only to clients that
refuse Centrifugo, so Pusher may stop being offered at any time. Measured against Kick's
Centrifugo (server 6.9.7 PRO): a guest connection token comes from
`/v1/realtime/auth/connection` (a 45-minute JWT for `guest:<client_id>`, no login); the
protocol is Centrifugo's JSON over websocket (connect reply carries `expires`, `ttl`
2699 s, `ping` 25 s, `{}` ping answered with `{}`, several newline-separated replies per
frame); channels keep their Pusher names `chatrooms.{id}.v2` and are public; publication
data is `{"event","data"}` with the same event names and JSON-string payload, so parsing is
unchanged; one connection holds at most 128 subscriptions (error `106` from the 129th) and
we watch about 600 chatrooms; the global server URL delivers every chatroom, including
ones whose per-channel descriptor named the other region.

Constraints: Kick decides the transport and can change it without notice, so following
its choice is what keeps working; no new single point of failure (if Kick's API is
unreachable the listener starts as it does today, and a way back must need no rebuild
since kick-listener has no beta deployment).

## Considered options

- Ask Kick at startup and implement both providers (chosen).
- Stay on Pusher by never accepting Centrifugo: works only as long as Kick keeps
  offering Pusher to refusing clients, exactly the path it is leaving.
- Centrifugo only: discards a today-working transport and gives no fallback.
- Connect each chatroom to its per-channel regional descriptor: more connections and API
  calls for no measured gain (the global URL delivers everything).

## Decision

At startup the listener posts to the realtime connection endpoint with the providers in
`KICK_REALTIME_PROVIDERS` (default `centrifugo,pusher`, what kick.com itself accepts) and
builds the client for the provider Kick returns; a failed lookup starts the Pusher client
with the configured key as before. Setting `KICK_REALTIME_PROVIDERS=pusher` forces Pusher
without a rebuild. The Centrifugo client (`services/kick-listener/websocket/
centrifugo.go`) pools connections at 100 chatrooms each, below Kick's 128 limit; each
connection fetches its own guest token, refreshes before expiry, answers pings and
reconnects with backoff, resubscribing its own chatrooms; emptied connections close
except the first. Both clients satisfy one `websocket.Realtime` interface, share event
decoding and drive the same `kick_listener_socket_state` gauge, so the
`KickListenerDisconnected` alert covers either transport. The provider is chosen once per
process; when Kick stops offering it, connects fail, the liveness probe restarts the pod
after five silent minutes, and the new process asks again.

## Consequences

- Kick chat keeps working on whichever transport Kick prefers; switching back to Pusher
  is a manifest change, and the identical payload means nothing downstream changes.
- Two transports to maintain until Kick drops Pusher, at which point the Pusher client
  and `KICK_REALTIME_PROVIDERS` go too. Everything rests on undocumented Kick endpoints
  and limits; when they change, the caesar-deployment alerts are what tell us. The
  Pusher-only private-channel auth (`/broadcasting/auth`) has no Centrifugo equivalent;
  it already failed 401 for every channel in production, and chatrooms subscribe publicly
  on both.
