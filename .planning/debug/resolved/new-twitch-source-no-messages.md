---
status: resolved
trigger: "new Twitch source added to overlay ec9b0855-4592-490c-a2dc-b7f3fddeadb7, no messages after 1+ minute"
created: 2026-03-29T00:00:00Z
updated: 2026-03-29T00:00:00Z
---

## Current Focus
<!-- OVERWRITE on each update - reflects NOW -->

hypothesis: Pod vr86t has had a stuck IRC connection since ~10:31 (85+ minutes). The connectionWatchdog correctly detects the zombie and calls client.Disconnect() every 60s, but the disconnect does NOT cause the go-twitch-irc library's Connect() to return, so the reconnect goroutine remains permanently blocked. Ironmouse is assigned to this pod which cannot receive messages.
test: Confirm watchdog behavior vs go-twitch-irc library internals. Fix: reset lastActivityAt when forcing disconnect so the watchdog knows reconnect is in progress; plus ensure Disconnect() actually unblocks Connect(). Also fix underlying leadership to reassign ironmouse to the healthy pod.
expecting: After fix, zombie detection leads to actual reconnect and messages flow from ironmouse
next_action: Fix the connectionWatchdog — reset lastActivityAt when calling Disconnect() to prevent repeated useless calls; also investigate why Disconnect() doesn't unblock Connect() in the stuck state.

## Symptoms
<!-- Written during gathering, then IMMUTABLE -->

expected: After adding a new Twitch source to an overlay, messages from that Twitch channel should start appearing on the overlay within seconds
actual: No messages appear after over a minute
errors: Unknown
reproduction: Add a new Twitch source to overlay ec9b0855-4592-490c-a2dc-b7f3fddeadb7 via admin tooling
started: Just happened, new source addition (not existing source that stopped working)

## Eliminated
<!-- APPEND only - prevents re-investigating -->

- hypothesis: New source not picked up due to missing notification mechanism
  evidence: PostgreSQL NOTIFY trigger exists on overlay_chat_sources. twitch-listener has both pg LISTEN (instant) and 30s fallback. Confirmed in logs: ironmouse INSERT fired at 12:04:45, received by both pods, pod vr86t joined the channel at 12:04:45.
  timestamp: 2026-03-29T12:00:00Z

- hypothesis: New source not in DB or overlay
  evidence: DB confirms ironmouse was inserted at 12:04:45 for overlay ec9b0855-4592-490c-a2dc-b7f3fddeadb7, is_active=true
  timestamp: 2026-03-29T12:00:00Z

- hypothesis: Coverage verification prevents channel join
  evidence: assignedSourceIDs is nil in twitch-listener manager (passed as nil from main.go), so the entire coverage check block is skipped (assignedSourceIDs != nil is false). No filtering applied.
  timestamp: 2026-03-29T12:00:00Z

## Evidence
<!-- APPEND only - facts discovered -->

- timestamp: 2026-03-29T12:04:45Z
  checked: twitch-listener pod vr86t logs
  found: ironmouse received INSERT notification at 12:04:45, "Joined channel ironmouse" at 12:04:45.179
  implication: The notification mechanism and channel joining works correctly for new sources

- timestamp: 2026-03-29T10:37Z-ongoing
  checked: twitch-listener pod vr86t logs
  found: "IRC connection appears zombie" fires every 60 seconds since 10:37, idle_duration grows monotonically (from 363s to 6000+s). No "IRC connection lost", no "Creating new IRC client", no "Connected to Twitch IRC" after 10:31:20.
  implication: The connectionWatchdog calls Disconnect() every 60 seconds but it does NOT trigger the reconnect goroutine. The IRC connection is permanently dead on this pod. ironmouse was joined on this pod so no messages from ironmouse reach the overlay.

- timestamp: 2026-03-29T10:25-10:31Z
  checked: twitch-listener pod vr86t logs
  found: First zombie at 10:25:24 → "IRC connection lost" → "Creating new IRC client" → "Connected to Twitch IRC" at 10:25:30. Then library internally reconnects 8+ times (10:25:53, 10:26:29... 10:31:20). Each triggers onConnect→ClearActiveChannels→joinChannelsMultipleConnections.
  implication: The go-twitch-irc library has an INTERNAL reconnect loop inside Connect(). Multiple "Connected" events are from the library's own internal retries, not from our reconnect goroutine.

- timestamp: 2026-03-29T10:31:20Z
  checked: twitch-listener pod vr86t logs
  found: Last "Connected to Twitch IRC" is 10:31:20. After this, no more connection events. At 10:37, zombie fires but no reconnect follows.
  implication: The library got stuck in a state where Connect() is blocking but Disconnect() doesn't unblock it. The library may be blocked on TCP connect (makeConnection → dialer.Dial) with connActive=false, causing Disconnect() to return ErrConnectionIsNotOpen (which we log as "Error forcing disconnect") — but that log doesn't appear either. OR Disconnect() closes userDisconnect but wg.Wait() blocks because startWriter/startReader goroutines are stuck.

- timestamp: 2026-03-29T12:04:45Z
  checked: twitch-listener pod s9dvw (healthy pod) logs
  found: Received ironmouse notification but did NOT join it (total_active went from 73 to 73, joined:74 - 74 channels were in toJoin but only 73 became active). Pod vr86t won leadership for ironmouse.
  implication: Because vr86t holds the leadership lock for ironmouse, s9dvw (healthy pod) cannot take over. The leadership lock has a 10-second TTL and must be renewed every 5 seconds. If vr86t's IRC is dead but its process is alive (leadership heartbeat still works), it keeps renewing the lock forever despite being unable to receive messages.

## Resolution
<!-- OVERWRITE as understanding evolves -->

root_cause: |
  When a new Twitch source is added, the notification mechanism and channel joining work correctly.
  Pod vr86t joined ironmouse at 12:04:45 within milliseconds of the DB INSERT notification.

  HOWEVER: pod vr86t's IRC connection has been zombie since 10:31 (~85 minutes). The
  connectionWatchdog correctly detects this and calls client.Disconnect() every 60 seconds, but
  the go-twitch-irc library's internal Connect() loop is stuck (either blocked on TCP dial with
  connActive=false, or stuck in wg.Wait() with a blocked startWriter goroutine). Disconnect() either
  returns ErrConnectionIsNotOpen (because connActive=false) OR closes userDisconnect but the
  goroutine never unblocks because wg.Wait() is stuck. Either way, our reconnect goroutine never
  runs and the pod remains in a permanently broken state.

  The liveness probe was returning 200 unconditionally, so Kubernetes never restarted the pod.
  Meanwhile, vr86t holds the leadership lock for ironmouse (renewed every 5 seconds via HTTP to
  source-manager), preventing the healthy pod (s9dvw) from claiming it.

  Root cause chain: zombie IRC → liveness probe always 200 → Kubernetes doesn't restart →
  leadership lock held by broken pod → healthy pod can't take ironmouse → no messages on overlay.

fix: |
  1. Added OnPingSent callback registration in irc.ConnectionManager.setupClientCallbacks() so
     that every PING the go-twitch-irc library sends (every ~IdlePingInterval seconds) updates
     lastActivityAt. Previously only chat messages (PRIVMSG etc.) updated this timestamp.

  2. Added IsStale() method to irc.ConnectionManager — returns true when lastActivityAt is older
     than staleLivenessThreshold (10 minutes) AND lastActivityAt is not zero (was ever connected).

  3. Modified handlers.LivenessProbe to return HTTP 503 when IsStale() is true. Kubernetes will
     restart the pod after 3 consecutive 503 responses (30 seconds), releasing leadership locks
     so the healthy pod immediately claims all channels including ironmouse.

  4. Extracted callback setup into setupClientCallbacks() helper — used by both NewConnectionManager
     and the reconnect loop, ensuring OnPingSent is registered on every new client.

  5. Introduced ircConnectionHealth interface in handlers package for testability.

verification: Manual — confirm pod vr86t gets restarted by Kubernetes after deploying. Then verify ironmouse channel is active on the restarted (or other) pod and messages flow to the overlay.

files_changed:
  - services/twitch-listener/irc/connection.go
  - services/twitch-listener/handlers/health.go
  - services/twitch-listener/irc/connection_stale_test.go
  - services/twitch-listener/handlers/health_test.go
