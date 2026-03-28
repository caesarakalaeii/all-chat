---
status: resolved
trigger: "production-leadership-tiktok-broken: Multiple production issues observed in Grafana after recent deploy: (1) Twitch leadership election not working — both pods show 144 sources each instead of splitting, (2) TikTok stuck usernames count very high, (3) TikTok backoff intervals very high suggesting on-demand polling is not functional."
created: 2026-03-28T00:00:00Z
updated: 2026-03-28T21:30:00Z
---

## Current Focus

hypothesis: CONFIRMED (new bug) — The readiness probe at health.go:102 requires activeChannelCount >= filteredAssignmentCount (144). After deploying the EnsureLeadership fix, pod dwlkn (started first, old code) holds all 144 Redis leadership locks. Pod kjvxp (new code) correctly tries EnsureLeadership per channel, loses all elections, and ends up with 0 active channels. filteredAssignmentCount=144, activeChannelCount=0 → readiness returns 503 forever. The pod will NEVER become ready under these conditions.
test: Add initialSyncComplete bool flag to Manager, set it after first SyncChannels, expose via IsInitialSyncComplete(). Update readiness probe: when leadership is enabled, require initialSyncComplete instead of activeChannelCount >= filteredAssignmentCount.
expecting: After fix, second pod becomes ready after initial sync (even with 0 channels), rolling deploy completes, old pod terminates, new pod claims its half.
next_action: Implement fix in channels/manager.go (add flag) and handlers/health.go (update readiness logic)

## Symptoms

expected: Twitch listener pods should split sources via leadership election (one leader, one follower with different source counts). TikTok should have low stuck username counts and reasonable backoff intervals with functional on-demand polling.
actual: Both twitch listener pods show 144 sources each (no leadership splitting). TikTok has very high stuck username count and high backoff intervals.
errors: No specific error messages — observed via Grafana dashboards; TikTok: "Failed to publish heartbeat: 404" every 10s
reproduction: Check Grafana dashboards for twitch-listener and tiktok-listener metrics
started: Just noticed now, after a recent deploy. Was working before.

## Eliminated

- hypothesis: Twitch 144 sources per pod is "correct behavior" (DisableDemandFiltering=true means always-connected to all)
  evidence: User confirmed this is WRONG. Twitch SHOULD split channels between pods via leadership election. DisableDemandFiltering only means no demand-based connect/disconnect — leadership election via EnsureLeadership still handles which pod owns which channel.
  timestamp: 2026-03-28T22:00:00Z

- hypothesis: source-manager not publishing demand correctly
  evidence: source:demand channel is active; demand updates publish correctly for overlays with connected clients (Twitch/YouTube/Kick overlays all work)
  timestamp: 2026-03-28T21:20:00Z

- hypothesis: database missing TikTok sources
  evidence: 53 TikTok sources exist in overlay_chat_sources; but none of the 9 currently-connected overlays have TikTok sources → demand_count=0 is CORRECT behavior
  timestamp: 2026-03-28T21:22:00Z

## Evidence

- timestamp: 2026-03-28T22:00:00Z
  checked: services/twitch-listener/channels/manager.go SyncChannels (lines 347-382) and joinChannelsMultipleConnections (lines 627-658)
  found: SyncChannels calls joinChannelsMultipleConnections when len(toJoin)>=100; joinChannelsMultipleConnections loops through channels calling m.joinParter.Join(ch) directly with NO EnsureLeadership call; the else branch (< 100 channels) DOES call EnsureLeadership per channel
  implication: On startup with 144 channels both pods always take the multi-connection path and join all 144 without any leadership election. Subsequent syncs have toJoin=0 so EnsureLeadership is never called. Leadership is completely bypassed.

- timestamp: 2026-03-28T22:49:00Z
  checked: pod kjvxp logs and describe output
  found: total_active:0 joined:144 on every sync cycle. Readiness probe returns 503. Pod events show "Readiness probe failed: HTTP probe failed with statuscode: 503". filteredAssignmentCount=144, activeChannelCount=0 → health.go line 102: activeChannelCount < filteredAssignmentCount → not_ready. Ready pod dwlkn shows total_active:144 joined:0 — it holds all 144 Redis locks. Pod kjvxp correctly loses all leadership elections → 0 active channels → will NEVER pass readiness probe under old logic.
  implication: The fix in 701d0cd (EnsureLeadership in joinChannelsMultipleConnections) works correctly, but exposed a broken readiness probe assumption: it requires a pod to own ALL channels, which is impossible when a peer holds all locks during rolling deploy.

- timestamp: 2026-03-28T22:01:00Z
  checked: kubectl logs twitch-listener startup
  found: "Creating multiple IRC connections" channel_count:144, followed by "Channel sync completed" total_active:144 on BOTH pods with joined:144. No "SOURCE_MANAGER_SECRET not set" message → leadership IS configured and the coordinator IS instantiated, but its EnsureLeadership is simply never called by joinChannelsMultipleConnections.
  implication: Fix is to call EnsureLeadership inside joinChannelsMultipleConnections, skipping channels where another pod holds the lock.

- timestamp: 2026-03-28T21:00:00Z
  checked: git log --oneline -20
  found: feat(06-02) migrated twitch-listener to LeadershipListener; feat(06-03) removed coordinator infrastructure from source-manager
  implication: Phase 6 migration is the direct cause

- timestamp: 2026-03-28T21:05:00Z
  checked: kubectl logs twitch-listener pod
  found: "Creating multiple IRC connections" channel_count:144 client_count:2; every sync shows total_active:144
  implication: Both pods are joining all 144 channels — no splitting

- timestamp: 2026-03-28T21:08:00Z
  checked: channels/manager.go SyncChannels (lines 347-382)
  found: if len(toJoin) >= 100 → calls joinChannelsMultipleConnections which NEVER calls EnsureLeadership; only the else-branch (toJoin < 100) calls EnsureLeadership per channel
  implication: On first startup with 144 channels, ALL channels are joined without leadership check. After that, toJoin=0 so EnsureLeadership is never called again

- timestamp: 2026-03-28T21:10:00Z
  checked: channels/manager.go joinChannelsMultipleConnections (lines 627-658)
  found: Loops through channels, calls m.joinParter.Join(ch) directly; no m.leader.EnsureLeadership call anywhere in this function
  implication: Leadership election completely bypassed for the multi-connection join path

- timestamp: 2026-03-28T21:15:00Z
  checked: tiktok-listener startup logs
  found: coordinator_url: http://source-manager.allchat.svc.cluster.local:8088; heartbeat 404 every 10s; BUT /assignments returns 19 assignments (old source-manager pod still serving port 8088); demand_count: 0 consistently
  implication: TikTok is running OLD coordinator code path (SERVICE_JWT_SECRET set); gets 19 assignments; but demand is 0 because no TikTok overlays connected

- timestamp: 2026-03-28T21:16:00Z
  checked: kubectl get pods -l app=source-manager
  found: source-manager-bdfc86cf7-78hfs 24h old (running old coordinator code, port 8088); bgnn5 and hxdhx 70s old (running new code, port 8083, no coordinator)
  implication: Half-migrated cluster state: old pod still serving /assignments to TikTok

- timestamp: 2026-03-28T21:18:00Z
  checked: source:demand Redis pub/sub payload
  found: demand messages contain twitch/youtube/kick/discord sources but ZERO tiktok sources
  implication: TikTok demand is zero because none of the 9 connected overlays (overlay:connected:*) have TikTok sources — this is correct behavior when no TikTok overlay is being watched

- timestamp: 2026-03-28T21:22:00Z
  checked: tiktok-listener index.ts lines 276-316 (coordinator startup path)
  found: if SERVICE_JWT_SECRET → queries old coordinator, populates assignedSourceIDs, subscribes migration events; assignedSourceIDs filter in DemandSubscriber filters out sources not in assigned set; but since demand_count=0 anyway, this doesn't affect current behavior
  implication: TikTok's stuck/backoff metrics are from old coordinator-based migration events and backoff state that accumulated before this deploy; the coordinator is removed from source-manager so heartbeat 404s permanently

## Resolution

root_cause:
  BUG 1 (Twitch 144 sources per pod): joinChannelsMultipleConnections (called when len(toJoin) >= 100) never called EnsureLeadership. The else branch (< 100 channels) correctly calls EnsureLeadership per channel, but on startup with 144 channels both pods always take the multi-connection path and join all 144 channels without any leadership election. After the first sync toJoin=0, so EnsureLeadership is never called again. Net effect: both pods hold all 144 channels, full channel duplication.

  BUG 2 (TikTok stuck usernames/backoff — ROOT CAUSE): In caesar-deployment/apps/workloads/all-chat/tiktok-listener-deployment.yaml, COORDINATOR_URL, SERVICE_JWT_SECRET, and HEARTBEAT_INTERVAL_MS env vars were never removed when the coordinator was removed from source-manager in feat(06-03). With SERVICE_JWT_SECRET set, tiktok-listener enables the OLD coordinator startup path (line 276 in index.ts: if SERVICE_JWT_SECRET). This path: (1) publishes heartbeat to /heartbeat which returns 404 on all source-manager pods — causing error spam every 10s, (2) waits 35-50 seconds before querying /assignments — drastically slowing startup, (3) subscribed to migration:events channel from old coordinator — processing stale migration events, (4) set assignedSourceIDs filter — could incorrectly filter demand updates. Old source-manager pod (78hfs, running old code from before feat(06-03) removal) served /assignments requests from TikTok, returning 19 stale assignments. Stuck/backoff metrics accumulated from TikTok continuously trying to connect streams from stale assignments.

fix:
  BUG 1: Added EnsureLeadership call per channel inside joinChannelsMultipleConnections, identical to the existing logic in the else branch. Channels where another pod holds the lock are skipped. Added regression test TestManager_JoinChannelsMultipleConnections_RespectsLeadership that simulates two concurrent managers with a shared leadership backend and verifies each channel is joined by exactly one pod.
  BUG 2: Removed COORDINATOR_URL, SERVICE_JWT_SECRET, and HEARTBEAT_INTERVAL_MS from caesar-deployment tiktok-listener deployment. ArgoCD synced and rolled out new pods.

verification:
  BUG 1: All twitch-listener tests pass including new regression test. Regression test verifies that with 110 channels and two concurrent managers, no channel is joined twice and both pods together cover all channels.
  BUG 2: New tiktok-listener pods show coordination_enabled:false, no heartbeat 404 errors, immediate startup (no 35-50s jitter wait), demand subscriber active. All 3 pods running replicaset 7bb464d5cd. Backoff state cleared on restart.
  BUG 3 (readiness probe): After deploying BUG 1 fix, rolling deploy stalled. New fix (commit 9d469bb) deployed. Both pods now show total_active:72 (exactly half of 144). Both pods 1/1 Ready. Leadership splitting confirmed working in production.

files_changed:
  - services/twitch-listener/channels/manager.go (added EnsureLeadership to joinChannelsMultipleConnections; added initialSyncDone flag; added IsInitialSyncComplete() and IsLeadershipEnabled())
  - services/twitch-listener/channels/manager_test.go (added regression tests for initial sync flag and leadership enabled flag)
  - services/twitch-listener/handlers/health.go (updated readiness probe check 4 to use initialSyncDone when leadership is enabled)
  - caesar-deployment/apps/workloads/all-chat/tiktok-listener-deployment.yaml (bug 2, already fixed)
