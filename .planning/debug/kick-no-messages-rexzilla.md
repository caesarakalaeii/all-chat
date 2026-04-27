---
status: investigating
trigger: "kick-no-messages-rexzilla — Kick listener not delivering messages for overlay dfa101f4-ae79-4f41-b549-044b731621c5 with rexzilla as kick source"
created: 2026-03-31T00:00:00Z
updated: 2026-03-31T00:00:00Z
---

## Current Focus

hypothesis: Unknown — gathering initial evidence from live system
test: kubectl pod status, logs, source-manager state, Redis streams
expecting: identify where in the pipeline messages are being lost or never generated
next_action: check pod readiness, then kick-listener logs for rexzilla subscription

## Symptoms

expected: After adding rexzilla as a kick chat source to overlay dfa101f4-ae79-4f41-b549-044b731621c5, kick messages should appear in the overlay within seconds, and the platform indicator should show connected.
actual: After 5 minutes, zero kick messages appear. Platform status indicator shows no connection for kick.
errors: Unknown — checking logs now
reproduction: Add rexzilla as a kick source to overlay dfa101f4 via admin panel, wait 5 minutes, no messages arrive.
started: Just happened during stability testing. Twitch and YouTube are working fine on the same overlay.

## Eliminated

(none yet)

## Evidence

(gathering)

## Resolution

root_cause:
fix:
verification:
files_changed: []
