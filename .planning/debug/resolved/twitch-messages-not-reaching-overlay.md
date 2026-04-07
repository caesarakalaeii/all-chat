---
status: awaiting_human_verify
trigger: "URGENT — Twitch messages are not appearing on any of caesarlps overlays. Re-added the source, still nothing."
created: 2026-04-07T17:45:00Z
updated: 2026-04-07T18:10:00Z
---

## Current Focus

hypothesis: CONFIRMED AND FIXED — SyncChannels held m.mu (write-lock) during the entire rate-limited IRC join phase, blocking readiness probe HTTP handlers, causing Kubernetes to TCP-timeout and kill pods in a cycle. CaesarLP messages stopped because pods were constantly restarting and losing IRC channel memberships.
test: Deployed fix (commit 5041775). New pods 7dd49ccbb5-* show 503 (not TCP timeout) for readiness probes during startup, survive startup, and caesarlp messages are flowing in chat:raw.
expecting: Pods remain stable across deployments and caesarlp messages reach all overlays.
next_action: Human verification — confirm Twitch messages appear on overlays in real-time.

## Symptoms

expected: Twitch chat messages should appear on caesarlps streaming overlays in real-time
actual: No Twitch messages appearing on ANY overlay. Re-adding the chat source does not fix it.
errors: No visible errors in logs or browser console; pods showed "context deadline exceeded" for readiness probes
reproduction: Affects all overlays for user caesarlps. Messages simply don't appear.
started: Was working, then suddenly stopped. No known deploy or config change.

## Eliminated

- hypothesis: caesarlp channel not in database
  evidence: DB shows 4 active twitch sources for channel_id=caesarlp across 4 overlays
  timestamp: 2026-04-07T17:48:00Z

- hypothesis: caesarlp leadership key missing
  evidence: Redis shows leader:twitch:CaesarLP exists with active TTL (being renewed)
  timestamp: 2026-04-07T17:49:00Z

- hypothesis: message-processor routing failure
  evidence: chat:raw stream receiving caesarlp messages once pods stabilized; overlay pub/sub channels active
  timestamp: 2026-04-07T17:52:00Z

- hypothesis: IRC library not joining caesarlp due to case mismatch
  evidence: go-twitch-irc library lowercases all channels in createJoinMessages; joins #caesarlp correctly
  timestamp: 2026-04-07T17:53:00Z

## Evidence

- timestamp: 2026-04-07T17:47:00Z
  checked: Kubernetes pod status
  found: twitch-listener pods cycling — svf7b just started (39s ago), wnm96 and bh9c4 being killed
  implication: pods not surviving startup

- timestamp: 2026-04-07T17:48:00Z
  checked: overlay_chat_sources table
  found: CaesarLP channel_id=caesarlp is_active=t across 4 overlay_ids
  implication: DB config is correct

- timestamp: 2026-04-07T17:49:30Z
  checked: chat:raw Redis stream for caesarlp messages
  found: last caesarlp twitch message was April 5 22:11 UTC (44+ hours ago)
  implication: something broke around April 5-6

- timestamp: 2026-04-07T17:50:30Z
  checked: Kubernetes events for twitch-listener
  found: pods killed with "context deadline exceeded" (TCP timeout) on readiness probe — NOT 503 HTTP
  implication: HTTP server was not responding during probe; server was blocked

- timestamp: 2026-04-07T17:51:00Z
  checked: SyncChannels in channels/manager.go lines 334+
  found: m.mu.Lock() held from line 334 through the entire joinChannelsMultipleConnections call including rate limiter waits
  implication: write-lock held for 40+ seconds for 80 channels at 20 joins/10s

- timestamp: 2026-04-07T17:52:00Z
  checked: readiness probe handlers GetActiveChannelCount / IsInitialSyncComplete
  found: both call m.mu.RLock() which is blocked waiting for the 40s write-lock
  implication: Gin handler goroutines stall → HTTP server stops responding → TCP timeout → Kubernetes kills pod

- timestamp: 2026-04-07T17:53:00Z
  checked: chat:raw stream after pod restarts
  found: caesarlp twitch messages resumed at 17:50:39 (pod svf7b survived long enough)
  implication: messages flow when a pod survives; the cycle was the root cause

- timestamp: 2026-04-07T18:08:00Z
  checked: chat:raw stream after deploying fix (commit 5041775)
  found: caesarlp messages flowing continuously at 18:06 and 18:08 UTC
  implication: fix is working; IRC channel maintained across pod startup

## Resolution

root_cause: SyncChannels in channels/manager.go held m.mu (write-lock) for the entire duration of the rate-limited IRC join loop. At 80 channels × 20 joins/10s, the lock was held for ~40 seconds. The readiness probe Gin handlers call m.mu.RLock() which blocked for those 40 seconds, causing the Kubernetes HTTP probe to TCP-timeout. Kubernetes treated TCP-timeouts as unhealthy and cycled the pods, preventing any pod from staying alive long enough to build a stable IRC session. caesarlps channel memberships were lost on every restart.
fix: Restructured SyncChannels to hold m.mu only for brief, non-blocking operations (computing toJoin/toPart snapshots, setting filteredAssignmentCount, marking initialSyncDone). All IRC operations (Depart, Join, rate-limiter waits, leadership election) now happen outside the mutex. joinChannel and joinChannelsMultipleConnectionsUnlocked each acquire the lock only for the single map write (activeChans[ch]=true), and the new partChannel acquires its own lock internally. Adds TestManager_SyncChannels_DoesNotBlockHealthProbe regression test.
verification: go build + go test pass. CI built image (run 24096574104). Rollout succeeded. New pods show 503 (proper HTTP) instead of TCP timeout during startup. caesarlp Twitch messages confirmed flowing at 18:06-18:08 UTC after deployment.
files_changed:
  - services/twitch-listener/channels/manager.go
  - services/twitch-listener/channels/manager_test.go
