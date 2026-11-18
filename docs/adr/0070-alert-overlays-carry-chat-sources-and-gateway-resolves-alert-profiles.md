# ADR-0070: Alert Overlays Carry Chat Sources; the Gateway Resolves Alert Profiles

**Date**: 2026-10-16
**Status**: Accepted
**Revised**: 2026-11-15 (the `alerts_editor` gate is open to everyone; ADR-0071 has the tiering)
**Deciders**: All-Chat Platform Team

## Context

ADR-0068 routes events to alerts/goal/list overlays through their `overlay_chat_sources`
rows (UNION with accepted share_requests), but overlay-manager refused to attach a chat
source to a non-chat overlay, so no alert overlay ever had a source, the routing query
matched nothing, and production had zero `alert_events` rows; nothing subscribed
`overlay:{id}:alerts` either, and there was nowhere to store how an event type renders.
Constraints: one source model (source-manager demand and listener activation work only
through `overlay_chat_sources`); chat must not leak onto non-chat overlays (it fills the
replay buffer, credit-roll sessions and TTS per target); the OBS page renders an alert in
a single frame; fresh alerts overlays need no seeded rows.

## Considered options

- Alert overlays carry ordinary chat sources, with the message-processor routing chat only
  to `overlay_type = 'chat'` (chosen).
- A separate `overlay_alert_sources` table: duplicates the add-source UI, OAuth auto-add,
  source-manager demand and listener activation with a second path.
- Reference a chat overlay's sources: couples two overlays' lifecycles and forces
  sourceless streamers to create a chat overlay first.
- Profile resolution: gateway resolves per alert from Postgres (chosen; it already holds
  a DB pool, alerts are rare so one indexed lookup is cheap, and real and test alerts
  share one resolver) versus the alert-processor (test alerts skip it, and goal/list
  overlays would carry profile data they never use) versus the client (second fetch,
  stale profiles).

## Decision

- **Sources on every kind.** overlay-manager's add-source and OAuth auto-add accept chat
  sources on alerts/goal/list overlays (the `rejectNonChatOverlay` guard removed; the
  frontend add-source UI never branched on kind). Chat stays on chat overlays: both
  branches of the message-processor's `FindOverlaysForMessage` UNION filter
  `overlay_type = 'chat'`; the share fan-out sender side is unfiltered because what a
  recipient receives depends on the recipient's kind.
- **Conditional gateway subscription.** At connect the overlay socket reads
  `is_active, overlay_type` in one query; non-chat overlays also subscribe
  `overlay:{id}:alerts` (sibling of `:poll`/`:prediction`), added to a shared
  viewer-created subscription later if needed.
- **Alert frames.** Messages on `:alerts` go out as `WSMessage{type:"alert"}`: the alert
  envelope plus a resolved `profile` (saved or default, text template rendered
  server-side; `{username}`, `{amount}`, `{count}`, `{message}` substituted, missing
  fields keep their literal token). Disabled profiles drop the alert; unknown event types
  pass without a `profile` key; a failed lookup falls back to the defaults. Alert frames
  reach every non-engagement socket, never the chat replay buffer; clients dedup on
  `alert_id`. The envelope gained `message` (the event's chat text).
- **Profiles.** `alert_profiles` (migration 107), one row per (overlay, event type);
  defaults and the known event-type set are Go in `shared/alerts` using listener names
  (`gift_subscription`, `new_sponsor`, `member_milestone`, `subscriber`,
  `tiktok_subscription`, ...). overlay-manager serves `GET alerts/profiles` (all known
  types merged over the defaults, free) and `PUT alerts/profiles` (all-or-nothing bulk
  upsert; 400 for wrong kind, unknown or duplicated event type; gated by
  `RequirePremium("alerts_editor")`, open since 2026-11-15) and `POST alerts/test`
  (`{event_type}`, publishes a synthetic envelope with a fresh `alert_id` straight to
  `overlay:{id}:alerts`, free).

## Consequences

- Alert overlays use the source flow streamers already know (connecting one activates its
  sources like any overlay); one resolver serves real and test alerts; adding an event
  type or changing a default is a code change with no migration.
- A channel with both a chat and an alert overlay carries two source rows (listeners
  dedupe by channel, so this is cheap). One profile lookup per alert per gateway pod,
  acceptable at alert rates (cache if DB load shows it). Overlay kinds are read at
  connect, so a kind change takes effect on the next fresh subscription.

## Implementation

- overlay-manager `handlers/sources.go` (guard removed), `handlers/alerts.go`,
  `repository/alert_profile_repo.go`, routes in `cmd/main.go`.
- message-processor `router/overlay_router.go` (`findOverlaysForMessageSQL`).
- api-gateway `subscription/subscriber.go` (`channelsFor`), `subscription/repository.go`
  (`OverlayStatus`, `AlertProfile`), `cmd/alerts.go` (`buildAlertFrame`),
  `models/ws_message.go` (`alert`).
- `alert-processor/models/alert.go` (`message`); `shared/alerts/`;
  `shared/featuregates/cache.go` (`GateAlertsEditor`); `migrations/107_alert_profiles.sql`.
