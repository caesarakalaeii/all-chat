---
status: resolved
trigger: "Kick stream messages are showing on the overlay but the Discord relay has stopped delivering messages. This was working minutes ago."
created: 2026-03-26T11:30:00Z
updated: 2026-03-26T12:50:00Z
---

## Current Focus

hypothesis: CONFIRMED — Cross-platform migration flapping prevents kick messages from reaching Redis, so discord relay has nothing to forward
test: N/A — root cause confirmed via logs and code analysis
expecting: N/A
next_action: Deploy fixes and verify

## Symptoms

expected: Kick chat messages should flow through to both the overlay AND be relayed to Discord.
actual: Messages appear on the overlay (mock messages only) but Discord relay stopped. No real kick messages in chat:raw stream.
errors: source-manager "No pods available for platform: kick", kick-listener "context deadline exceeded" on /assignments
reproduction: Active Kick stream running with Discord relay configured.
started: Stopped minutes ago after source-manager pod churn.

## Eliminated

- Webhook URL invalid: No 403/404 logs in discord-listener
- Redis PubSub dead: PubSub channels active, subscriber counts correct
- Discord Gateway issues: Gateway connected (session invalidations recovered)
- Relay config missing: SyncRelayConfigs shows 1 active subscription

## Evidence

- timestamp: 2026-03-26T11:32:00Z
  checked: Kubernetes cluster connectivity
  found: All pods running, source-manager had a terminating pod replaced

- timestamp: 2026-03-26T11:33:00Z
  checked: discord-listener relay manager
  found: 1 active subscription, syncing every 30s, zero relay post activity

- timestamp: 2026-03-26T12:38:00Z
  checked: Redis chat:raw stream (XREVRANGE, 200 messages)
  found: Zero kick messages, only twitch. Kick-listener subscribed to 6 channels but no messages published.

- timestamp: 2026-03-26T12:38:00Z
  checked: source-manager logs
  found: "No pods available for platform: kick" — kick heartbeat stale. Migration events sent to wrong-platform pods (twitch, tiktok, youtube).

- timestamp: 2026-03-26T12:39:00Z
  checked: kick-listener logs
  found: Channels being unsubscribed via migration handoff, then re-subscribed on next sync. Flapping cycle every 30s. Assignment queries timing out (10s HTTP client timeout).

- timestamp: 2026-03-26T12:40:00Z
  checked: Redis shard:heartbeats sorted set
  found: kick-listener heartbeat 26s stale (score 1774522265, current time 1774522291). 15s timeout threshold exceeded.

- timestamp: 2026-03-26T12:41:00Z
  checked: coordinator.go triggerMigrationForFailedPods
  found: Used global c.assigner (all pods) instead of platform-specific assigners. Fixed in commit e4d0de0.

## Resolution

root_cause: |
  1. Heartbeat staleness: Shared 10s HTTP client timeout between heartbeat POST and slow /assignments query (scans 322+ Redis keys). Heartbeat arrives stale.
  2. Cross-platform migration: triggerMigrationForFailedPods used global assigner, migrating kick sources to twitch/tiktok/youtube pods.
  3. Channel flapping: Kick-listener unsubscribes on migration handoff, re-subscribes on next sync. Zero messages received during flapping.

fix: |
  1. Platform-aware migration (commit e4d0de0, pending deploy via Keel): triggerMigrationForFailedPods now uses groupPodsByPlatform() to build platform-specific assigners.
  2. Heartbeat timeout isolation (new fix): PublishHeartbeat uses dedicated 3s context timeout to fail fast instead of sharing 10s client timeout.

verification: |
  After deploy: source-manager should log "No healthy pods available for platform, skipping migration" instead of cross-platform migrations.
  Kick-listener heartbeat should stay fresh (< 15s in shard:heartbeats zset).
  Kick messages should appear in chat:raw stream and relay to Discord.

files_changed:
  - services/source-manager/coordination/coordinator.go (commit e4d0de0)
  - shared/coordination/client.go (heartbeat timeout fix)
