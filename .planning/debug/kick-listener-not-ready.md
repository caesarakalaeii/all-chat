---
status: awaiting_human_verify
trigger: "Kick listener pod is stuck as not ready in production Kubernetes — readiness probe is failing."
created: 2026-03-27T14:00:00Z
updated: 2026-03-27T15:20:00Z
---

## Current Focus

hypothesis: filteredAssignmentCount is set by syncChannels but never decremented by reconcileDemand. When demand filtering unsubscribes all channels, subscriptions=0 but filteredAssignmentCount=N, causing readiness probe Check 3 to always fail with "subscriptions connecting".
test: Read manager.go — confirmed filteredAssignmentCount has only one write site (line 386 in syncChannels). reconcileDemand deletes from m.subscriptions without touching filteredAssignmentCount.
expecting: Fix by updating filteredAssignmentCount inside reconcileDemand after removals, OR by adjusting the readiness probe to skip Check 3 when demand is zero (no active overlay clients).
next_action: implement fix — update filteredAssignmentCount in reconcileDemand to reflect actual demanded channel count

## Symptoms

expected: Kick listener pod should be 1/1 READY and processing kick chat messages
actual: Pod shows 0/1 READY, readiness probe returning HTTP 503 "subscriptions connecting"
errors: Readiness probe failed: HTTP probe failed with statuscode: 503 (x295 over 26m)
reproduction: Pod has been stuck in this state for 26+ minutes in production
started: Unknown — pod was recently (re)deployed
environment: Production Kubernetes, namespace allchat, context default

## Eliminated

- hypothesis: WebSocket not connected
  evidence: Logs show successful subscription_succeeded events from Pusher. wsClient.IsConnected() returns true. Check 1 passes.
  timestamp: 2026-03-27T15:00:00Z

- hypothesis: No assignments from coordinator
  evidence: Logs show assignment_count=61 retrieved from coordinator every 10s. Check 2 passes.
  timestamp: 2026-03-27T15:00:00Z

- hypothesis: Redis unhealthy
  evidence: Redis connections are successful at startup (no fatal errors). Check 4 would pass.
  timestamp: 2026-03-27T15:00:00Z

- hypothesis: Database connection issue
  evidence: repository.go successfully queries channels (count=6) on every sync cycle.
  timestamp: 2026-03-27T15:00:00Z

## Evidence

- timestamp: 2026-03-27T15:00:00Z
  checked: Pod events in kubectl describe
  found: 295 readiness probe failures over 26 minutes — all "HTTP probe failed with statuscode: 503"
  implication: Pod is running and serving HTTP, but consistently returning 503 on /health/ready

- timestamp: 2026-03-27T15:05:00Z
  checked: Pod logs (200 lines)
  found: Cyclic pattern repeating every ~30s: syncChannels subscribes to 4-6 channels → "active_subscriptions: 5" → ~1-2s later reconcileDemand fires and logs "Demand lost, unsubscribing channel" for ALL channels → subscriptions=0 → next syncChannels re-subscribes → repeat
  implication: Demand filtering is correctly driving subscriptions to zero (no active overlay clients), but this breaks the readiness probe

- timestamp: 2026-03-27T15:07:00Z
  checked: handlers/health.go ReadinessProbe (Check 3)
  found: Check 3 compares subscriptionCount (len(m.subscriptions)) < filteredAssignmentCount (m.filteredAssignmentCount). If true, returns 503 "subscriptions connecting".
  implication: After demand filtering removes all subscriptions, subscriptions=0 but filteredAssignmentCount=4-6 → Check 3 always fails

- timestamp: 2026-03-27T15:08:00Z
  checked: channels/manager.go — all write sites for filteredAssignmentCount
  found: filteredAssignmentCount is written ONLY at line 386 inside syncChannels. reconcileDemand (lines 1039-1083) deletes from m.subscriptions but never updates filteredAssignmentCount.
  implication: After demand-driven unsubscription, filteredAssignmentCount is stale (set from last sync), subscriptions is 0 → mismatch causes perpetual readiness failure

- timestamp: 2026-03-27T15:09:00Z
  checked: demand.go — what triggers UpdateDemandedSourceIDs
  found: The SDK's demand subscriber loop fires when any overlay client connects/disconnects. With zero active overlay clients, demanded map is empty → reconcileDemand unsubscribes everything. This is the intended phase-5 behavior ("demand-driven polling").
  implication: The readiness probe's Check 3 was designed before demand-driven behavior was added. It assumes subscriptions should always equal filteredAssignmentCount, but now subscriptions can legitimately be 0 when no overlays are viewing.

## Resolution

root_cause: The readiness probe Check 3 compares subscriptionCount < filteredAssignmentCount to detect "still subscribing" state. However, after demand-driven unsubscription (phase-5 feature), all channels are correctly removed from m.subscriptions, but filteredAssignmentCount is only updated in syncChannels and remains at the last non-zero value. This makes the check permanently fail: 0 < N.

fix: In reconcileDemand, after removing channels from m.subscriptions, update filteredAssignmentCount to reflect the current demanded channel count (the number of subscriptions that remain after demand filtering). This way the readiness probe sees the correct expected vs actual counts.

verification: go build ./... passes cleanly; TestManager tests pass; pre-existing repository_test.go failure is unrelated to this change (mock provides 4 columns but query was updated to return 5 columns)
files_changed: [services/kick-listener/channels/manager.go]
