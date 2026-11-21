# ADR-0094: Kick events come from the realtime feed and from Kick's official webhooks

**Date**: 2026-11-20
**Status**: Accepted
**Revised**: 2026-11-21
**Deciders**: caesarakalaeii

## Context

Alerts overlays covered Twitch, YouTube and TikTok events but nothing from Kick, and the
public pages had to say so. Kick offers two sources for events:

- **The realtime feed** kick.com's own web client uses (ADR-0077). kick-listener already
  holds it for chat. Kick does not document it and can change it without notice. A
  capture on 2026-11-20 showed what it carries: subscriptions and hosts on
  `chatrooms.<id>.v2`, gifted subscriptions only on the `chatroom_<id>` alias, KICKs gifts
  on `channel_<channel id>`, and no follower name on any public channel.
- **The official webhook API** (`docs.kick.com/events`): documented, signed deliveries for
  follows, subscriptions, gifted subs and KICKs gifts. An app subscribes per broadcaster,
  with a user access token (scope `events:subscribe`) or with an app access token plus the
  broadcaster's user id.

Constraints: events should work for every Kick source, including channels added by name
whose streamer never linked a Kick account; streamers should not have to re-consent; and
kick-listener has no beta deployment, so the change must be safe to ship to production.

## Considered options

- **Realtime feed only.** No follows at all, and KICKs gifts need a third channel per
  source plus a channel id the listener does not store.
- **Official webhooks only, with user tokens.** Stable, but only for linked accounts that
  re-consent to a new scope; sources added by name get nothing.
- **Both, with app-token webhook subscriptions (chosen).** The feed supplies what it
  already carries on the chatroom channels; webhooks supply the two events it lacks.

## Decision

kick-listener publishes five event types to `chat:raw`:

| Event type | Source | `event_data` |
| --- | --- | --- |
| `kick_subscription` | feed, `App\Events\SubscriptionEvent` | `months` |
| `kick_gift_subscription` | feed, `GiftedSubscriptionsEvent` on `chatroom_<id>` | `gift_count` |
| `kick_host` | feed, `App\Events\StreamHostEvent` | `viewers` |
| `kick_follow` | webhook `channel.followed` | none |
| `kick_kicks` | webhook `kicks.gifted` | `kicks`, `gift_name` |

**Feed** (`websocket/events.go`). The Centrifugo client subscribes both chatroom aliases,
so a pooled connection holds 50 chatrooms. The second alias repeats chat under the same
event name with another payload; chat and deletions are taken from the first alias only.
The Pusher fallback reads the first alias only and so delivers no gifted subs.

**Webhooks** (`kickevents/`). When `KICK_CLIENT_ID` and `KICK_CLIENT_SECRET` are set,
kick-listener serves `POST /webhooks/kick` and runs a reconciler:

- Every ten minutes one pod (a Redis lock) lists the app's subscriptions with an app
  access token, requests `channel.followed` and `kicks.gifted` for each Kick source of an
  active overlay, and deletes subscriptions for channels that are no longer sources. Kick
  drops a subscription whose deliveries fail for a day; the next pass brings it back.
  Kick rate-limits these requests without publishing a limit (a burst of about five a
  second was refused with `429`), so they go out one a second, at most 400 a pass, and a
  pass stops at the first `429`.
- A delivery is accepted only if Kick signed it: RSA PKCS#1 v1.5 over SHA-256 of
  `<message id>.<timestamp>.<body>`, checked against the key from
  `GET /public/v1/public-key` (cached, refetched once on a failed check because Kick may
  rotate it), with a timestamp within ten minutes. Message ids are remembered in Redis for
  an hour, so a retried or replayed delivery is published once.
- Every verified delivery is answered `200`, also for event types the listener does not
  use, so that Kick never counts one as failed.
- The overlays for a delivery come from the database (cached for a minute), not from the
  channel leader's memory, because the gateway may hand a delivery to any pod.

Subscriptions and gifted subs are deliberately not subscribed as webhooks: the feed already
delivers them and a second source would double every alert.

The webhook URL (`https://allch.at/webhooks/kick`) is set in the Kick developer dashboard
of the app; the gateway routes that path to kick-listener (caesar-deployment
`ingress/allchat-route.yaml`).

The message-processor and the alert-processor normalize all five with
`KickNormalizer.NormalizeEvent`. Chat overlays have per-overlay toggles
(`enable_kick_subs`, `enable_kick_gifts`, `enable_kick_hosts`, `enable_kick_follows`);
alerts overlays have a profile per event (`shared/alerts`).

## Consequences

- Kick alerts work for every Kick source without a linked account or a new consent.
- Three of the five events depend on an undocumented feed. If Kick renames an event or
  moves it to another alias, those alerts stop silently while chat keeps working.
- Follows and KICKs gifts depend on Kick accepting app-token subscriptions for any
  broadcaster, as its API reference states (confirmed on 2026-11-20 with a
  `channel.followed` subscription for a channel that never linked an account). If Kick
  refuses one, the reconciler logs
  `Kick event subscription refused` with Kick's reason and that channel gets no follow or
  KICKs alerts; the fallback would be user-token subscriptions for linked accounts.
- Anonymous KICKs gifts carry no sender and are not published.
- A subscription limit of 10,000 per event type per app applies.
