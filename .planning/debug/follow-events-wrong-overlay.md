---
status: investigating
trigger: "Overlay fa92a81b shows follow events even though they are turned off. Follow events should only appear on overlay e0e469ce."
created: 2026-03-31T00:00:00Z
updated: 2026-03-31T00:00:00Z
---

## Current Focus

hypothesis: Follow events are broadcast to ALL overlays for a channel rather than only to overlays that have follow events enabled
test: Trace the path from EventSub listener → message processor → API gateway to find where overlay-level filtering should occur
expecting: Either missing filter logic or a bug in how overlay settings are checked
next_action: Examine overlay settings schema, EventSub listener publishing logic, and message processor routing

## Symptoms

expected: Follow events should only appear on overlay e0e469ce-b6f8-4df0-9527-027513027fd7 (where they are enabled). Overlay fa92a81b-7eec-4f45-b196-70ce5b9d6b8b should NOT show follow events (they are turned off).
actual: Overlay fa92a81b-7eec-4f45-b196-70ce5b9d6b8b is showing follow events despite them being disabled for that overlay.
errors: No specific error messages — the events just appear on the wrong overlay.
reproduction: Observed during a live stream. First time the user checked, so unclear if this ever worked correctly.
started: First noticed during the user's last stream. Never previously verified that event routing was correct.

## Eliminated

(none yet)

## Evidence

- timestamp: 2026-03-31T00:01:00Z
  checked: filter/event_filter.go mapEventTypeToColumn() for twitch "follow" event
  found: Maps to "enable_twitch_follows" column (line 81)
  implication: This column does not exist in the database

- timestamp: 2026-03-31T00:02:00Z
  checked: migrations/017_event_settings.sql
  found: Schema has enable_twitch_subs, enable_twitch_resubs, enable_twitch_gift_subs, enable_twitch_bits, enable_twitch_raids, enable_twitch_channel_points — NO enable_twitch_follows
  implication: SQL query SELECT enable_twitch_follows fails with "column does not exist" error

- timestamp: 2026-03-31T00:03:00Z
  checked: message-processor/cmd/main.go lines 293-302
  found: Error handling fails open: if IsEventEnabled returns error, enabled=true (allow event)
  implication: ALL overlays receive follow events regardless of their settings

- timestamp: 2026-03-31T00:04:00Z
  checked: overlay-manager/models/event_settings.go
  found: EnableTwitchFollows field is MISSING from the Go model
  implication: Confirms the column was never added anywhere in the stack

- timestamp: 2026-03-31T00:05:00Z
  checked: frontend/src/app/overlays/[id]/events/page.tsx
  found: No enable_twitch_follows in EventSettings interface and no UI toggle for Twitch follows
  implication: The full feature (DB column, model, filter mapping, UI) was never completed

## Resolution

root_cause: The enable_twitch_follows column was never added to the overlay_event_settings database table (missing from migration 017). The filter/event_filter.go references it, causing SQL errors. The fail-open error handling in main.go then allows follow events through to ALL overlays.
fix: Add migration 045_twitch_follows_event_setting.sql to add enable_twitch_follows column. Add field to EventSettings model. Update repository queries. Add UI toggle in frontend events page.
verification:
files_changed: []
