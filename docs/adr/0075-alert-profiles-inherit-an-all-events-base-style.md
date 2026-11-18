# ADR-0075: Alert Profiles Inherit an All-Events Base Style

**Date**: 2026-10-17
**Status**: Accepted
**Deciders**: All-Chat Platform Team
**Amends**: ADR-0070 (profile resolution at the gateway)

## Context

ADR-0070 resolves an alerts overlay's profile per event type as "saved row, else built-in
default", and the defaults carry no media. Two problems on beta: one GIF and sound on
every alert meant up to 20 saves (the maintainer's "usability nightmare"); and test
alerts resolved gateway-side from saved profiles, so a GIF picked but not saved never
showed (prod received four test alerts with no `PUT /alerts/profiles` and every test
showed the text-only default). Constraints: one action sets one GIF everywhere; per-event
control stays; no migration (`alert_profiles.event_type` is Go-validated); a test alert
shows what the editor shows.

## Considered options

- A stored base profile with event type `all`, layered under each event type at the
  gateway (chosen).
- An "apply to all" button copying media into every row: no backend change, but every
  later change to the shared GIF means copying again and each copy overwrites per-event
  tweaks.
- Send the unsaved draft with the test alert: fixes the test only and adds an
  envelope field only tests would use.

## Decision

`shared/alerts.AllEvents = "all"` is storable (`IsKnownEventType`) but is no event: not in
`EventTypes()`, and `TestAlert` rejects it. Default enabled, 5 s, volume 100, no template,
no media. `alerts.ApplyBase(p, saved, base)` resolves one event type: without a saved row,
the built-in default plus the base's image, sound, duration, volume and custom code; with
a saved row, its own values, with only empty image/sound falling back to the base, and
custom code moving as a unit (own code, else the base's; mixing HTML from one profile with
JS from another would break both); `enabled` = base AND event. The gateway
(`buildAlertFrame`) looks up the event row and the `all` row and calls `ApplyBase`; a
failed lookup still falls back to defaults, logged. overlay-manager's GET/PUT
`/alerts/profiles` return the `all` profile first, then every event type, plus `saved`
(the types with a stored row). The editor opens on "All events (default style)": no text
field (text is per event), the base's timing shown and saved on first edit so it does not
jump, empty media fields reading "Empty: All events style". "Send test alert" lives in
the editor: it tests the event type being edited (a subscription when "All events" is
selected); with unsaved changes for a premium user it becomes "Save and send test alert"
and saves first, a failed save sends no test. `power_up` (Twitch) and `gift_received`
(YouTube) join `eventDefs`.

## Consequences

- One GIF for every alert is one field and one save, at the cost of two profile lookups
  per alert at the gateway (alerts are rare next to chat).
- An older frontend ignores the `all` row and `saved`; an older gateway ignores a saved
  `all` row and alerts look as before, so either side deploys first.
- A saved event type cannot be reset to full inheritance from the editor; clearing media
  brings back the base's media but its own timing stays. A reset that deletes the row
  (`DELETE /alerts/profiles/:event_type`) is the ask if streamers want it.
