# Phase 5: Sharding Infrastructure & Coordinator Service - Context

**Gathered:** 2026-02-19
**Status:** Ready for planning

<domain>
## Phase Boundary

Foundation for distributed channel assignment across listener pods. Coordinator service computes which pod handles which channels using bounded-load consistent hashing, stores assignments in Redis with O(1) channel lookup and O(log N) load queries, and prevents split-brain through Kubernetes Lease-based leader election with fencing tokens.

</domain>

<decisions>
## Implementation Decisions

### Hash key selection
- Use `source_id` (from database) as the primary hash key
- No additional context beyond source_id (not tenant_id or other fields)
- Hard-coded hash function (CRC32 or similar—simple, fast, sufficient)
- Orphaned assignment cleanup: defense in depth
  - Coordinator periodically scans and removes assignments for deleted sources
  - Pods also self-clean when connection attempts fail

### Load balancing strategy
- Bounded-load consistent hashing (not pure consistent hashing)
- Enforce 1.25x average load bound per pod (matches success criteria)
- Load measurement phasing:
  - Phase 5: Channel count only (each channel weighs the same)
  - Phase 7: Message-rate awareness added for hot channel rebalancing
- When all pods at capacity: HPA scales up (no assignment rejection or queueing)

### Failure recovery behavior
- Heartbeat timeout: 15 seconds (not 60s—too long for fast streams)
- No grace period after timeout (immediate redistribution for fast recovery)
- Leader election: Kubernetes Lease API with fencing tokens (prevents split-brain)
- Channel redistribution priority: High-traffic channels reconnect first (minimize impact on active streams)

### Assignment registry structure
- Assignment timestamp stored alongside pod_id (for debugging and audit logs)
- Global version counter for detecting stale reads (increments on every assignment change)
- Heartbeats stored in Redis (exact structure at Claude's discretion based on coordinator detection pattern)

### Claude's Discretion
- Exact Redis data structure for assignments (optimize for O(1) channel lookup + O(log N) load queries)
- Heartbeat storage implementation (TTL keys vs hash vs sorted set)
- Bounded-load bound configurability (hard-coded 1.25x vs env var)
- Orphaned assignment cleanup interval

</decisions>

<specifics>
## Specific Ideas

- "60 seconds of message loss in a fast-acting stream would be catastrophic"—keep heartbeat timeout tight (15s)
- "HPA should scale up" when all pods reach capacity—no rejection or queueing, trust HPA
- High-traffic channels prioritized during failure recovery to minimize user impact

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 05-sharding-infrastructure-coordinator-service*
*Context gathered: 2026-02-19*
