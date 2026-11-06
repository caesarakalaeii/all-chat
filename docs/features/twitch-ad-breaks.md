# Twitch Ad Break Notices

## Overview

When a Twitch ad break starts, the monitor view's **Activity & Events** panel
(`/overlay/{id}/view`) shows a row for it, so a streamer can pause a game before viewers
miss a boss fight or a cutscene. Both kinds of break are reported: the ones Ads Manager
schedules and the ones the broadcaster or an editor run by hand.

**Platform**: Twitch only (`channel.ad_break.begin` v1).

**Audience**: the overlay owner, in the authenticated monitor. The row never reaches an
OBS browser source, a viewer participate tab or the public overlay, and it raises no
on-stream alert.

**Opt-in**: yes. It needs the broadcaster's `channel:read:ads` scope, which is not a
login scope.

**Persistence**: none. Live-only, gone on reload.

**Feature gate**: `twitch_ad_breaks` (ADR-0008), seeded free (`is_premium=FALSE`,
migration 127). auth-service checks it on the consent, so narrowing it stops new grants
and leaves existing ones working.

## What a streamer sees

An `Ad Break` row with its duration and how it started:

| Break     | Row                                           |
| --------- | --------------------------------------------- |
| Scheduled | `Ad Break  1m 30s  Automatic ad break`        |
| Manual    | `Ad Break  1m  Ad break started by someeditor` |

The row's time is the start of the break. When the monitor's activity sound is on, it
plays for this row like for any other activity.

`started_at` is when Twitch began the break. Twitch documents a possible delay between
that moment and viewers actually seeing ads, so the row usually lands a little before the
audience is in the ad.

### Turning it on

While the overlay has a Twitch source, the viewer is its owner and
`ad_breaks_granted` on `GET /overlays/:id/capabilities` is not `true`, the monitor
shows a banner with a **Show ad breaks** button. It redirects to Twitch consent via
`GET /api/v1/auth/twitch/moderation/{overlay_id}?actions=ads`
(`moderationApi.getTwitchAdBreaksConsentUrl`). The request asks for `channel:read:ads`
unioned with the scopes already granted for Twitch, so the new token is a superset of the
stored grant. `channel:read:ads` is in `preservableScopes`, so a later plain login cannot
drop it.

The grant is owner-only: `ads` is not in `delegatableActions`, so a delegated moderator's
consent can never request it, and moderators do not see the banner.

## End-to-end trace

1. **Twitch → webhook.** `routeEvent` in
   `services/twitch-eventsub-listener/webhooks/handler.go` dispatches
   `channel.ad_break.begin` to `handleAdBreakBegin`.
2. **Handler → wire format.** `buildAdBreak` produces a
   `RawChatMessage{EventType: "ad_break"}` on the lowercased broadcaster login.
   `eventsub.AdBreakBeginEvent` accepts `duration_seconds` and `is_automatic` as numbers
   or quoted strings: Twitch's reference table types them integer/boolean, its own example
   payload quotes them.
3. **`chat:raw` → message-processor.** `mapEventTypeToColumn` answers
   `columnAlwaysEnabled` (the consent is the opt-in, so there is no per-overlay toggle).
   `TwitchNormalizer.NormalizeEvent` sets the value pill (`formatAdBreakDuration`:
   `30s`, `1m 30s`, `3m`); `classifier.ClassifyEvent` returns tier `low`, duration 0.
   alert-processor never routes it: `ad_break` is not on its allowlist.
4. **api-gateway.** `isOwnerOnlyFrame` (`services/api-gateway/cmd/main.go`) marks
   `ad_break` owner-only, so `Pool.BroadcastFiltered` skips anonymous and viewer
   sockets, and `shouldBufferForReplay` keeps it out of the replay buffer. Nothing in the
   frame is confidential; the reasons are that an OBS source would render it on stream,
   and that a replayed ad break is one that already ended.
5. **Frontend.** A normal feed item: `partitionItems` puts it in `events`,
   `ActivityPanel` renders it with `CompactEvent`.

## `EventData` keys

| Key                | Type    | Always present?                                                                   |
| ------------------ | ------- | --------------------------------------------------------------------------------- |
| `duration_seconds` | int     | Yes                                                                               |
| `is_automatic`     | bool    | Yes. `true` for an Ads Manager break                                              |
| `started_at`       | string  | Yes, RFC 3339. Falls back to receipt time if Twitch sends none                    |
| `requester_login`  | string  | Manual breaks only. Twitch fills the requester on scheduled breaks too; dropped there so the broadcaster is not shown as having started it |

## Troubleshooting

### The streamer enabled it and sees nothing

Same limitation as the moderation log (ADR-0030): event subscriptions are created when a
channel is first tracked, so a grant made later takes effect on the next channel
(re)sync, which is a leader change, a pod restart, or the channel being removed and
re-added to the overlay. The last one is the one a streamer can do.

Until then the listener logs, at Info, `Ad break events require re-authentication with
channel:read:ads scope` with the `broadcaster_id`. A `Failed to subscribe to ad breaks`
warning is a real failure instead.

### The banner keeps coming back

`ad_breaks_granted` is false when the credential lookup failed or when the owner's Twitch
credential is not the broadcaster of any Twitch source on the overlay. Re-consenting is
harmless.

## References

- [Twitch: channel.ad_break.begin event reference](https://dev.twitch.tv/docs/eventsub/eventsub-reference/#channel-ad-break-begin-event)
- [ADR-0054: Twitch moderation and AutoMod event feed](../adr/0054-twitch-moderation-and-automod-event-feed.md) — the owner-only delivery this reuses
- [ADR-0030: Twitch native engagement mirroring](../adr/0030-twitch-native-engagement-mirroring.md) — the resync limitation
