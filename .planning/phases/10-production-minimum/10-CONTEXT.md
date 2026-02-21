# Phase 10: Production Minimum - Context

**Gathered:** 2026-02-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Building production-ready InnerTube YouTube listener with dynamic stream management and lifecycle behaviors. **Drop-in replacement for official YouTube API listener** - must integrate with source-manager using the same pattern as existing platform listeners.

Core capabilities:
- Discover live streams from channel IDs (filter premieres)
- Integrate with source-manager for monitor/unmonitor commands
- Parse advanced event types (Super Chat, Super Sticker, memberships, milestones, tickers)
- Production lifecycle: reconnection, offline detection, graceful shutdown

</domain>

<decisions>
## Implementation Decisions

### Architectural Principle

**CRITICAL: Match official YouTube API listener behavior in all aspects.**
- Same source-manager integration pattern (no new HTTP API)
- Same Redis schema and publishing pattern
- Same lifecycle behaviors
- Same offline detection logic
- Drop-in replacement - zero changes to source-manager or message-processor

### Stream Discovery

- **Discovery mechanism:** Claude's discretion (HTML parsing vs InnerTube browse endpoint)
- **Premiere filtering:** Check `isLive` metadata flag to distinguish live streams from premieres
- **No stream found:** Poll until stream starts (wait up to 15 minutes before timeout)
- **Timeout duration:** 15 minutes (handle scheduled streams that start soon)

### Source-Manager Integration

- **Channel→Video discovery:** Async (start background goroutine when source-manager requests monitoring)
- **Discovery failure handling:** Give up and report error to source-manager after 15-minute timeout
- **State persistence:** Persist channel→video ID mapping to Redis (survives restarts, visible to other services)
- **Stream ended behavior:** Automatically discover next stream on that channel (seamless for 24/7 streamers)

### Event Parsing

- **Implementation priority:** All events equally (Super Chat, Super Sticker, memberships, milestones, tickers) - complete feature in Phase 10
- **Redis format:** Match official listener's RawChatMessage schema and event metadata structure
- **Parse error handling:** Log and skip unparseable events (resilient to schema changes)
- **Testing strategy:** Both unit tests with golden fixtures AND live stream comparison validation

### Lifecycle and Error Handling

- **Offline detection:** Match official listener's detection logic exactly
- **Reconnection strategy:** Exponential backoff (start 1s, double each retry up to max ~60s)
- **Graceful shutdown sequence:**
  1. Stop active polling
  2. Flush Redis buffers
  3. Clear state from Redis
  4. Notify source-manager
  5. Complete within 25-second timeout
- **Cleanup timeout handling:** Force exit immediately if cleanup can't complete in 25s

### Claude's Discretion

- Exact stream discovery mechanism (HTML vs InnerTube browse)
- Exponential backoff parameters (initial delay, max delay, multiplier)
- Redis key naming for channel→video mappings
- Logging verbosity and error message formatting
- Internal state management structures

</decisions>

<specifics>
## Specific Ideas

- "Should work exactly like the official YouTube listener from source-manager's perspective"
- 15-minute timeout for stream discovery handles scheduled streams (not just instant failures)
- Persist to Redis for durability and cross-service visibility
- Auto-resume on stream end for 24/7 channels (no manual intervention needed)

</specifics>

<deferred>
## Deferred Ideas

None - discussion stayed within phase scope.

Phase 10 establishes production minimum. Advanced features deferred to later phases:
- Deletion event detection (Phase 11/13)
- Advanced metrics and monitoring (Phase 12)
- Batch deletion detection (Phase 13)

</deferred>

---

*Phase: 10-production-minimum*
*Context gathered: 2026-02-21*
