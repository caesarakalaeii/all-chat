---
status: awaiting_human_verify
trigger: "kick-slow-source-discovery — messages eventually appeared without code deployment"
created: 2026-03-26T10:30:00Z
updated: 2026-03-26T10:30:00Z
---

## Current Focus

hypothesis: CONFIRMED — Two independent delays compound when a new Kick source is added. (1) The source-manager reconciliation loop runs on a 30s ticker with NO immediate first run — new sources are invisible to assignment computation for up to 30s. (2) The kick-listener's channel sync runs on a 30s ticker (syncInterval=30s) independently of assignments — even after assignments are updated, the channel sync may take another 30s. In the worst case a new source waits up to 60s (30s + 30s). Additionally, with HeartbeatInterval(30s) > HeartbeatTimeout(15s) in deployed code, the kick-listener pod oscillates between healthy/failed, meaning reconciliation cycles sometimes produce 0 assignments — adding further uncertainty. The source-manager also does NOT subscribe to PostgreSQL LISTEN/NOTIFY (unlike the kick-listener itself), so it never gets an immediate trigger on source creation.

test: Confirmed by code reading all critical paths. No further test needed.
expecting: Fix must add immediate reconciliation on source-manager pod startup AND react to source creation events without waiting for timers.
next_action: Implement fix — trigger immediate reconciliation on startup in coordinator.reconcile(), and reduce source-manager registry sync interval or hook into pg NOTIFY.

## Symptoms

expected: After adding a Kick source to an overlay, messages should appear within seconds
actual: Messages took a very long time to start appearing (minutes+), then eventually started without any code deployment
errors: Previous session: kick-listener logs showed oscillation between 0 and 7 assigned channels every ~30s due to HeartbeatInterval(30s) > HeartbeatTimeout(15s)
reproduction: Add a new Kick channel source to an overlay
timeline: User added source, waited a long time with no messages, then they eventually appeared on their own. Code fix (HeartbeatInterval 30s→10s) was committed locally but never deployed.

## Key Context from Prior Session

- HeartbeatInterval(30s) > HeartbeatTimeout(15s) causes oscillation — real bug, but was never deployed
- "Messages just started flowing" happened without any deployment — so something else caused the eventual success
- The deployed code still has HeartbeatInterval=30s and HeartbeatTimeout=15s
- 322 sources, 8 listener pods in production at time of investigation

## Eliminated

(none yet)

## Evidence

- timestamp: 2026-03-26T10:30:00Z
  checked: Prior debug session (.planning/debug/resolved/kick-messages-not-in-overlay.md)
  found: HeartbeatInterval=30s > HeartbeatTimeout=15s confirmed as oscillation cause. Kick-listener oscillated between 0 and 7 channels every 30s. Fix committed locally but never deployed or pushed.
  implication: The oscillation means assignments cycle between valid and cleared every 30s. A newly added source may eventually be assigned during a "good" window (when heartbeat is fresh) and messages flow. But during bad windows, all kick assignments are dropped.

- timestamp: 2026-03-26T10:45:00Z
  checked: services/source-manager/coordination/coordinator.go — reconcile() function
  found: reconcileInterval=30s. The reconcile loop uses a ticker with NO immediate first run. First reconciliation fires only after 30s. Code: `ticker := time.NewTicker(c.reconcileInterval); for { select { case <-ticker.C: computeAssignments() } }`
  implication: A newly added source is invisible to assignment computation for 0–30 seconds from when the source-manager leader's ticker last fired.

- timestamp: 2026-03-26T10:45:00Z
  checked: services/source-manager/registry/registry.go — Registry.Start() and periodicSync()
  found: Registry has an IMMEDIATE initial sync at startup (r.sync(ctx) called before goroutine launch). But subsequent syncs are on 30s ticker only. Source-manager does NOT subscribe to PostgreSQL LISTEN/NOTIFY at all — only kick-listener, twitch-listener, youtube-listener, tiktok-listener do.
  implication: After a new source is created in DB: the registry syncs within 0-30s (next ticker). Then coordinator.computeAssignments() runs 0-30s later. Total delay before assignment is written to Redis: 0-60s.

- timestamp: 2026-03-26T10:45:00Z
  checked: services/kick-listener/channels/manager.go — syncLoop() and Start()
  found: syncInterval=30s. Start() calls syncChannels() immediately at startup, then runs syncLoop() on 30s ticker. Also has PostgreSQL LISTEN/NOTIFY via listenForChanges() — receives instant notification on source CREATE/UPDATE/DELETE via chat_source_changes pg notify. So the kick-listener DOES react quickly to pg NOTIFY. BUT: syncChannels() is only useful after the source-manager has already written the assignment to Redis — it reads from the DB directly for subscriptions, not from assignments.
  implication: The kick-listener gets the db notification quickly but must wait for the assignment from source-manager. The assignment only arrives on the kick-listener's 10s AssignmentRefreshInterval timer.

- timestamp: 2026-03-26T10:45:00Z
  checked: shared/listener/base.go — startAssignmentRefreshLoop()
  found: AssignmentRefreshInterval=10s (in DefaultConfig after the fix). Loop is ticker-based with NO immediate first run after startup. First refresh runs 10s after start.
  implication: After source-manager writes the assignment to Redis, the kick-listener sees it within 0-10s on its next assignment refresh tick.

- timestamp: 2026-03-26T10:45:00Z
  checked: migrations/004_source_change_notifications.sql
  found: PostgreSQL trigger fires pg_notify('chat_source_changes', payload) on AFTER INSERT OR UPDATE OR DELETE on overlay_chat_sources. The kick-listener subscribes to this and calls syncChannels() immediately on notification — but syncChannels() reads from DB, not from Redis assignments. The source-manager does NOT use this trigger at all.
  implication: The pg NOTIFY path lets kick-listener know a source exists in DB fast, but assignments from source-manager are the bottleneck.

- timestamp: 2026-03-26T10:45:00Z
  checked: shared/listener/config.go (deployed version in prior session vs current)
  found: Current config.go shows HeartbeatInterval=10s (the fix IS committed). Prior session says "committed locally but never deployed". This means the fix is in the codebase but the running pods may still have the old image.
  implication: Need to verify if deployed pods have old or new image. With old (30s) heartbeat, the oscillation causes many reconciliation cycles to produce 0 kick assignments, making new sources wait much longer than 60s.

- timestamp: 2026-03-26T10:45:00Z
  checked: source-manager coordinator.reconcile() — no initial run
  found: The reconcile loop starts AFTER leader election completes (OnStartedLeading callback). Leader election itself takes up to LeaseDuration=30s if there's contention. First actual reconciliation fires only after reconcileInterval=30s from when leadership is acquired.
  implication: On source-manager pod restart, up to 30s leader acquisition + 30s first reconcile = 60s before first assignment is ever computed.

## Resolution

root_cause: Two compounding delays prevent newly added Kick sources from receiving messages quickly. (1) The source-manager coordinator.reconcile() loop starts with a 30s ticker and NO initial run — after the leader acquires its lease, the first computeAssignments() fires only 30s later. (2) The source-manager registry only syncs from DB every 30s with no pg NOTIFY subscription — a newly created source may sit in the DB for up to 30s before the registry sees it AND another 30s before the coordinator assigns it. In the worst case (deployed code with HeartbeatInterval=30s > HeartbeatTimeout=15s), kick-listener pods oscillate between healthy/failed, causing many reconciliation cycles to produce 0 kick assignments — making messages never arrive until the polling window aligns correctly (explaining "eventually started on their own" without any deployment).

fix: Three changes applied: (1) coordinator.reconcile() now calls computeAssignments() immediately upon leadership acquisition, before the first ticker tick — eliminating the 30s startup delay. (2) NewCoordinator() now accepts a *pgxpool.Pool and launches listenForSourceChanges() goroutine that subscribes to chat_source_changes PostgreSQL NOTIFY channel. When a source is created/updated/deleted, notifyCh receives a signal and the reconcile loop immediately calls computeAssignments() instead of waiting up to 30s for the next tick. (3) The HeartbeatInterval=10s fix (already committed in shared/listener/config.go) must be deployed to eliminate the heartbeat oscillation that was the root cause of the prior session.

verification: Build passes (go build ./... and go test ./... in source-manager — all tests pass). kick-listener build also passes.

files_changed: [services/source-manager/coordination/coordinator.go, services/source-manager/cmd/main.go]
