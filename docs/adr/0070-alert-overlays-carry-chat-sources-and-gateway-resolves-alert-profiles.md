# ADR-0070: Alert Overlays Carry Chat Sources; the Gateway Resolves Alert Profiles

**Date**: 2026-10-16
**Status**: Accepted
**Deciders**: All-Chat Platform Team

## Context and Problem Statement

ADR-0068 routes platform events to alerts, goal and list overlays through the
`overlay_chat_sources` rows of those overlays (UNION with accepted
`share_requests`). But the overlay-manager refused to attach a chat source to
any overlay that was not a chat overlay. Its reason was that "their pipelines have
nothing that reads a chat source". So no alert overlay could ever have a source,
the alert-processor's routing query never matched, and production recorded zero
`alert_events` rows.

The second half of the delivery path was also missing: nothing subscribed
`overlay:{id}:alerts`, and an alert overlay had nowhere to store how an event
type is presented (issue #952).

## Decision Drivers

- One source model. A streamer connects Twitch/YouTube/TikTok the same way for
  every overlay kind, and source-manager demand and listener activation work only
  through `overlay_chat_sources`.
- Chat must not leak onto overlays that do not render chat. Chat delivery fills
  the replay buffer, credit-roll sessions and TTS for each target overlay.
- The OBS overlay should render an alert from a single frame, with no second fetch.
- Defaults must cost nothing: a fresh alerts overlay needs no seeded rows.

## Considered Options

1. **Alert overlays carry ordinary chat sources; the message-processor routes
   chat only to `overlay_type = 'chat'` (chosen).**
2. **A separate `overlay_alert_sources` table.** This duplicates the source model:
   the add-source UI, OAuth auto-add, source-manager demand and listener
   activation would all need a second path.
3. **Alert overlays reference a chat overlay's sources.** This couples two overlays'
   lifecycles, and a streamer with only an alert box would have to create a chat
   overlay first.

For profile resolution:

a. **The gateway resolves the profile per alert from Postgres (chosen).** The
   gateway already holds a DB pool for overlay checks. Alerts are rare, so one
   indexed point lookup per alert costs very little. Both real and test alerts go
   through one place.
b. The alert-processor resolves it. Then test alerts, which skip the processor,
   would need their own resolver, and goal/list overlays would carry profile data
   they never use.
c. The client fetches profiles. That means a second fetch and stale profiles
   until the next fetch.

## Decision Outcome

- **Sources on every kind.** overlay-manager's add-source and OAuth auto-add
  accept chat sources on alerts/goal/list overlays (the `rejectNonChatOverlay`
  guard is removed). The editor's add-source UI never branched on kind, so the
  frontend needs no change.
- **Chat stays on chat overlays.** Both branches of the message-processor's
  `FindOverlaysForMessage` UNION filter `o.overlay_type = 'chat'`. The sender side
  of the share fan-out is not filtered, because what a recipient receives depends
  on the recipient's kind.
- **Conditional gateway subscription.** At connect, the overlay socket reads
  `is_active, overlay_type` in one query. A non-chat overlay's Pub/Sub
  subscription also includes `overlay:{id}:alerts`, as a sibling of
  `:poll`/`:prediction`. If a viewer-only socket created the shared
  subscription, the alerts channel is added to it later.
- **Alert frames.** Messages on `:alerts` are sent as
  `WSMessage{type:"alert"}`. The payload is the alert envelope unchanged, plus
  `profile`: the saved or default profile for the event type, with the text
  template rendered server-side. `{username}`, `{amount}`, `{count}` and
  `{message}` are substituted, and a placeholder whose field is missing keeps its
  literal token. A disabled profile drops the alert at the gateway. Event types
  without a profile are forwarded with no `profile` key. A failed profile lookup
  falls back to the default rather than losing the alert. Alert frames reach
  every non-engagement socket, are never written to the chat replay buffer, and
  clients dedup on `alert_id`.
- **The alert envelope gains `message`**, the event's chat text, so `{message}`
  has something to substitute.
- **Profiles.** The `alert_profiles` table (migration 107) has one row per
  (overlay, event type). Defaults and the known event-type set are Go code in
  `shared/alerts`, which both overlay-manager and the gateway import. The set uses
  the raw `event_type` names the listeners publish (`gift_subscription`,
  `new_sponsor`, `member_milestone`, `subscriber`, `tiktok_subscription`, ...).
- **Endpoints** (overlay-manager, behind the gateway at `/api/v1/overlays/:id/...`):
  `GET alerts/profiles` returns every known type, merged over the defaults and
  free. `PUT alerts/profiles` is an all-or-nothing bulk upsert. It returns 400
  unless `overlay_type = 'alerts'` or when an event type is unknown or duplicated.
  It is gated by `RequirePremium("alerts_editor")`, a gate seeded
  `is_premium=TRUE` by migration 107. `POST alerts/test` takes `{event_type}`,
  publishes a synthetic envelope with a fresh `alert_id` straight to
  `overlay:{id}:alerts`, and is free.

## Consequences

### Positive

- Alert overlays work with the source flow streamers already know. Connecting an
  alert overlay activates its sources like any other overlay.
- One resolver serves real and test alerts, and the frame is complete for the
  renderer.
- Adding an event type or changing a default is a code change, with no migration.

### Negative

- A streamer running a chat overlay and an alert overlay on the same channel has
  two source rows for that channel. This is cheap, because listeners dedupe by
  channel.
- Each gateway pod subscribed to an alert overlay does one profile lookup per
  alert. This is acceptable at alert rates. Cache it if it ever shows up in DB
  load.
- Overlay kinds are read at connect. A kind change only takes effect on the next
  fresh subscription.

## Implementation

- `services/overlay-manager/handlers/sources.go` (guard removed),
  `handlers/alerts.go`, `repository/alert_profile_repo.go`, routes in `cmd/main.go`.
- `services/message-processor/router/overlay_router.go` (`findOverlaysForMessageSQL`).
- `services/api-gateway/subscription/subscriber.go` (`channelsFor`, alerts flag),
  `subscription/repository.go` (`OverlayStatus`, `AlertProfile`),
  `cmd/alerts.go` (`buildAlertFrame`), `models/ws_message.go` (`alert`).
- `services/alert-processor/models/alert.go` (`message`).
- `shared/alerts/`, `shared/featuregates/cache.go` (`GateAlertsEditor`).
- `migrations/107_alert_profiles.sql` (+ down).

## Related Decisions

- ADR-0068 (event alerts platform): the routing and delivery channel this completes.
- ADR-0008 (feature gates): the `alerts_editor` gate.
- ADR-0002 (Redis Streams + Pub/Sub): the per-overlay channel convention.
