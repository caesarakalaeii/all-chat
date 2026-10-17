# ADR-0075: Alert Profiles Inherit an All-Events Base Style

**Date**: 2026-10-17
**Status**: Accepted
**Deciders**: All-Chat Platform Team
**Amends**: ADR-0070 (profile resolution at the gateway)

## Context and Problem Statement

ADR-0070 gives an alerts overlay one profile per event type, resolved by the
gateway as "saved row, else built-in default". The built-in defaults carry no
media. Two problems showed up on beta:

1. **One GIF meant up to 20 saves.** A streamer who wants the same GIF and
   sound on every alert had to pick each event type in the editor and set
   the media again. The maintainer called it "a usability nightmare".
2. **Test alerts ignored the form.** The editor and the "Send test alert"
   panel were separate, with separate event-type pickers. A test alert is
   resolved at the gateway from the *saved* profiles, so a GIF that was
   picked but not saved never appeared. On prod an alerts overlay received
   four test alerts and no `PUT /alerts/profiles` at all, and every test
   showed the text-only default. That is why the report was "alerts don't
   show the GIFs".

## Decision Drivers

- Setting one GIF for every alert must take one action.
- Per-event control (a different raid GIF, a longer bits alert) must stay.
- No migration: `alert_profiles.event_type` is validated in Go, not by a
  CHECK constraint (migration 107), so a new stored type needs no schema
  change.
- A test alert must show what the streamer sees in the editor.

## Considered Options

1. **A stored base profile with event type `all`, layered under each event
   type at the gateway (chosen).**
2. An "apply to all event types" button that copies the media into every
   row. No backend change, but a later change to the shared GIF means
   copying again, and every copy overwrites per-event tweaks.
3. Send the unsaved draft along with the test alert. That fixes the test
   but not the twenty saves, and it adds a profile field to the alert
   envelope that only test alerts would ever use.

## Decision Outcome

Option 1, plus folding the test button into the editor.

- `shared/alerts.AllEvents = "all"` is storable (`IsKnownEventType`) but is
  no event: it is not in `EventTypes()` and `TestAlert` rejects it. Its
  default is enabled, 5 s, volume 100, with no template and no media.
- `alerts.ApplyBase(p, saved, base)` resolves one event type:
  - Event type **without** a saved row: the built-in default (its own text)
    plus the base's image, sound, duration, volume and custom code.
  - Event type **with** a saved row: its own values. Only an empty image or
    sound falls back to the base. Custom code moves as a unit: the event's
    own code if it has any, else the base's. HTML from one profile mixed
    with JS from another would break both.
  - `enabled` = base AND event. Switching the base off silences every alert.
- The api-gateway (`buildAlertFrame`) looks up the event's row and the `all`
  row, then calls `ApplyBase`. A failed lookup still falls back to the
  defaults and is logged (ADR-0070).
- overlay-manager's `GET`/`PUT /alerts/profiles` return the `all` profile
  first, then every event type, plus `saved`: the event types that have a
  stored row.
- The editor opens on "All events (default style)". That view has no text
  field, since text is always per event. For an event type still on the
  base, the editor shows the base's timing and says so. Its first edit saves
  that timing, so the timing does not jump. Its empty media fields read
  "Empty: All events style".
- "Send test alert" moves into the editor. It tests the event type being
  edited (a subscription when "All events" is selected). For premium users
  with unsaved changes it becomes "Save and send test alert" and saves
  first; a failed save sends no test.
- `power_up` (Twitch) and `gift_received` (YouTube) were published by the
  listeners but had no profile, so they never showed media. They join
  `eventDefs`.

### Consequences

- One GIF for every alert is one field and one save.
- Two profile lookups per alert at the gateway instead of one. Alerts are
  rare compared with chat, so this does not matter.
- An older frontend ignores the `all` row and `saved`. An older gateway
  ignores a saved `all` row, so its alerts look as they did before. Either
  side can be deployed first.
- A saved event type cannot be reset to "inherit everything" from the
  editor; clearing its media fields brings back the base's media, but its
  own timing stays. If streamers ask for it, add a reset that deletes the
  row (`DELETE /alerts/profiles/:event_type`).
