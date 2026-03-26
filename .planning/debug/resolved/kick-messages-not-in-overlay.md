---
status: resolved
trigger: "kick-messages-not-in-overlay"
created: 2026-03-26T00:00:00Z
updated: 2026-03-26T10:15:00Z
---

## Current Focus

hypothesis: CONFIRMED — HeartbeatInterval (30s) > HeartbeatTimeout (15s) causes the kick-listener pod to appear as "failed" during every source-manager reconciliation cycle that runs when the heartbeat is between 15-30s old. The source-manager then removes all kick assignments, causing the kick-listener to unsubscribe from all Kick channels and stop processing messages.
test: Observation: kick-listener oscillates between 0 and 7 assigned channels every ~30 seconds. Source-manager logs show "No pods available for platform kick" every reconciliation cycle.
expecting: Fix: reduce HeartbeatInterval to < HeartbeatTimeout (from 30s to 10s) so pods are never considered stale between heartbeats.
next_action: Update HeartbeatInterval in shared/listener/config.go

## Symptoms

expected: Kick stream chat messages should appear in the overlay after adding a Kick source
actual: No messages from Kick appear in the overlay despite the stream being very active
errors: kick-listener logs: "Filtered channels by coordinator assignments: total=7, assigned=0" repeatedly
reproduction: Add a Kick channel as a source to an overlay, messages don't appear
started: Unknown — possibly always broken with current config mismatch

## Eliminated

- hypothesis: Kick WebSocket connection not working / Pusher subscription failing
  evidence: Logs show kick-listener successfully subscribes to all channels when assignments > 0
  timestamp: 2026-03-26T10:15:00Z

- hypothesis: Message-processor not processing Kick messages
  evidence: Never reached — messages aren't being published to Redis Streams due to no subscriptions
  timestamp: 2026-03-26T10:15:00Z

- hypothesis: Chatroom ID not available
  evidence: kick-listener successfully fetches chatroom IDs via Kick API when it gets assignments
  timestamp: 2026-03-26T10:15:00Z

## Evidence

- timestamp: 2026-03-26T10:10:00Z
  checked: kick-listener logs (pod kick-listener-7b9c45b99c-gv8q9)
  found: Oscillates every 30s between assigned_channels=0 and assigned_channels=7. When 0, active_subscriptions=0, no messages published.
  implication: Assignments are being cleared periodically by source-manager

- timestamp: 2026-03-26T10:10:10Z
  checked: source-manager old pod (qh89d) logs
  found: "Computing assignments: source_count=322, pod_count=8, failed_pods=2", followed by "No pods available for platform kick" for all kick sources. Assignment computation complete: skipped_no_pods=64
  implication: Source-manager is not seeing kick-listener as a healthy pod, clears all kick assignments

- timestamp: 2026-03-26T10:10:10Z
  checked: HeartbeatTimeout vs HeartbeatInterval values
  found: HeartbeatTimeout=15s (heartbeat.go), HeartbeatInterval=30s (config.go DefaultConfig). 30s > 15s means every pod appears "failed" for up to 15 seconds in every 30-second cycle.
  implication: This is a fundamental configuration mismatch. Any pod with a heartbeat interval > HeartbeatTimeout will intermittently appear as failed.

- timestamp: 2026-03-26T10:15:00Z
  checked: Redis heartbeats ZRANGE shard:heartbeats
  found: kick-listener heartbeat updates roughly every 20-30s. Cutoff is 15s ago. When the heartbeat is older than 15s but within 30s, the pod is falsely detected as failed.
  implication: Need to lower HeartbeatInterval to < HeartbeatTimeout (e.g. 10s)

## Resolution

root_cause: HeartbeatInterval (30s) exceeds HeartbeatTimeout (15s) in the source-manager. When the source-manager's reconciliation cycle runs at the wrong moment within the 30s heartbeat cycle, the kick-listener pod appears as "failed". The source-manager then computes assignments with no kick pods available, writes nothing to the Redis registry for kick sources, causing the kick-listener to read 0 assignments on its next refresh and unsubscribe from all Kick channels.

fix: Reduced HeartbeatInterval from 30s to 10s in shared/listener/config.go DefaultConfig(). This ensures listener pods publish heartbeats at 10s intervals, which is always fresher than the source-manager's 15s HeartbeatTimeout. Tests pass (shared/listener package: ok 0.612s).
verification: Tests pass. User confirmed "messages just started flowing" in production after pod restart with updated HeartbeatInterval.
files_changed: [shared/listener/config.go]
