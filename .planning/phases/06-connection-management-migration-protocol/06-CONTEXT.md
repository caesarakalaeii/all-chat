# Phase 6: Connection Management & Migration Protocol - Context

**Gathered:** 2026-02-19
**Status:** Ready for planning

<domain>
## Phase Boundary

This phase integrates Twitch, Kick, and TikTok listener services with the Phase 5 coordinator service, enabling distributed channel assignment across pods. Each listener queries the coordinator on startup to connect ONLY to assigned channels (not all channels). Implements graceful zero-loss channel migration protocol when pods scale or rebalance, using overlap handoff to ensure no messages are dropped during transitions.

</domain>

<decisions>
## Implementation Decisions

### Startup Integration
- **Coordinator availability:** Block indefinitely until coordinator responds (pod not ready until assignments received)
- **Connection strategy:** Connect immediately to all assigned channels (no batching delays)
- **Ready status:** Pod reports ready AFTER successfully connecting to all assigned channels
- **Note:** Previous pod readiness issue (1/5 ready) was transient - current state shows 5/5 healthy pods with aggressive HPA scaling

### Migration Notification & Timing
- **Notification mechanism:** Hybrid Redis Pub/Sub approach
  - Coordinator publishes migration event to Redis Pub/Sub channel (5-20ms latency)
  - All listener pods subscribe to `migration:events` channel
  - Faster than polling (15s avg), safer than direct HTTP push
- **Migration event structure:** Full context for debugging and metrics
  - Required fields: `channel_id`, `platform`, `from_pod`, `to_pod`, `migration_id`, `timestamp`, `reason`
- **Overlap protocol:** New pod waits for first message OR 30s timeout (whichever comes first)
  - New pod connects and subscribes to channel
  - Waits up to 30 seconds for first message (proof of successful connection)
  - Publishes confirmation to Redis when connected
- **Disconnect trigger:** Old pod disconnects immediately after seeing new pod's confirmation
  - No additional grace period - immediate handoff after confirmation
- **Duplicate handling:** Downstream deduplication
  - Both pods publish to Redis Streams during overlap period
  - Message Processor deduplicates based on platform message IDs
  - Keeps listener implementation simple

### Platform-Specific Implementation
- **Architecture:** Platform-specific implementations (no forced abstraction layer)
  - Twitch listener implements coordinator integration for IRC
  - Kick listener implements coordinator integration for Pusher WebSocket
  - TikTok listener implements coordinator integration for tiktok-live-connector
  - Respects platform quirks, accepts some code duplication
- **Connection state migration:** Minimal state - just channel assignment list
  - "State" = which channels to connect to (from coordinator)
  - New pod creates fresh connections (new IRC socket, new Pusher subscription)
  - No transfer of connection handles or platform-specific state

### Twitch IRC Specifics
- **Multiple connections:** Claude decides based on channel count
  - <100 channels: Single IRC connection with JOIN commands
  - ≥100 channels: Multiple IRC connections (Requirement TWITCH-03)
  - Balances simplicity vs Twitch's per-connection limits

### Failure Handling & Recovery
- **New pod connection failure:** Quick retry + coordinator fallback
  - New pod attempts connection 3 times with 1-second delays (3s total)
  - If all fail: publish "migration failed" event with error details
  - Old pod continues running (zero message loss)
  - Coordinator immediately reassigns to different pod
  - Optional: Coordinator can try multiple pods in parallel (first success wins)
- **Old pod doesn't disconnect:** Coordinator intervention
  - If old pod doesn't send "disconnected" confirmation within 60s timeout
  - Coordinator marks pod as dead, removes from assignments
  - Coordinator can trigger pod restart via Kubernetes if needed
  - New pod becomes source of truth
- **Coordinator crashes mid-migration:** Idempotent recovery
  - New coordinator leader reads migration events from Redis Streams
  - Checks current pod states via heartbeat data
  - Resumes or completes any in-progress migrations
  - Aligns with Phase 5 Kubernetes Lease-based leader election

### Claude's Discretion
- Observability event design (lifecycle events, retries, failures)
- Exact batching strategy if platform rate limits require it
- Kubernetes pod restart mechanics (delete pod vs update deployment)
- Migration retry backoff tuning if 1-second intervals prove suboptimal

</decisions>

<specifics>
## Specific Ideas

- **Fast handover priority:** User constraint: "very fast handovers and as little missed messages as possible"
  - Drove choice of Redis Pub/Sub (5-20ms) over polling (15s avg)
  - Quick retry pattern (3s) vs long backoff (30s+)
- **Aggressive HPA:** Current Twitch deployment scales aggressively (5 pods at 100% memory target)
  - Migration protocol must handle rapid scale-up/down
- **Redis infrastructure reuse:** All pods already have Redis connections for publishing messages
  - Natural fit for Pub/Sub migration notifications

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 06-connection-management-migration-protocol*
*Context gathered: 2026-02-19*
