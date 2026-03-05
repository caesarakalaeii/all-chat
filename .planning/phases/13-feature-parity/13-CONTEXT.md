# Phase 13: Feature Parity - Context

**Gathered:** 2026-03-05
**Status:** Ready for planning

<domain>
## Phase Boundary

Add deletion event detection (batch vs single) and advanced metrics to the InnerTube YouTube listener. Enhances the service's ability to detect when multiple messages are deleted together (bans/timeouts) and provides richer observability through metrics.

Scope:
- Batch deletion detection (5+ deletions in 100ms = batch event)
- Deletion buffering to handle race conditions
- Enhanced metrics (message rate gauge, error breakdown)
- All within the existing InnerTube listener service

Out of scope: Moderation UI, analytics dashboards, cross-service deletion coordination.

</domain>

<decisions>
## Implementation Decisions

### Metrics Structure
- **Granularity:** Per-channel for message rate gauge (not per-stream or global)
- **Error metrics:** Single counter with label `innertube_errors{type="parse|network|rate_limit"}` (standard Prometheus pattern)
- **Additional labels:** Include stream status (live/offline/reconnecting) for correlation
- **Rate calculation:** 1-minute rolling average (not instantaneous or 5-minute)

### Buffer Behavior
- **Buffer window:** 500ms wait before emitting deletion events
- **Maximum size:** 1000 events per buffer
- **Overflow strategy:** Drop oldest (FIFO) when buffer full
- **Buffer scope:** Per-channel (one buffer per channel_id, not global or per-stream)

### Batch Detection Logic
- **Threshold:** Configurable via environment variable (default: 5 deletions / 100ms)
- **Batch metadata:** Include deletion_count + reason field (ban|timeout|mod) when detectable
- **Emission policy:** Emit BOTH batch events (when threshold reached) AND single deletion events (below threshold)

### Event Emission Timing
- **Batch emission:** Immediately after detection window closes (100ms)
- **Emission order:** Arrival order from InnerTube (not sorted by message timestamp)
- **Failure handling:** Drop and log if Redis emission fails (eventual consistency, no retry)

### Claude's Discretion
- Resolving timing interaction between immediate batch emission (100ms) and 500ms buffer window
- Edge case handling for deletions that span multiple detection windows (e.g., 3 at t=0, 2 at t=150ms)
- Exact implementation of reason detection (ban vs timeout vs mod action)
- Buffer memory management and cleanup strategies

</decisions>

<specifics>
## Specific Ideas

- Follow existing InnerTube listener patterns for event emission (Redis Streams, RawChatMessage contract)
- Metrics should align with existing innertube_* namespace from Phase 12
- Buffer prevents race condition where deletion arrives before original message reaches message-processor
- Batch detection synthesizes single event from multiple individual deletions (reduces noise for moderation events)

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 13-feature-parity*
*Context gathered: 2026-03-05*
