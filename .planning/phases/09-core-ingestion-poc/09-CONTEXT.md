# Phase 9: Core Ingestion PoC - Context

**Gathered:** 2026-02-21
**Status:** Ready for planning

<domain>
## Phase Boundary

Build a YouTube chat listener using InnerTube (unofficial) API that publishes to Redis Streams (`chat:raw`), maintaining drop-in compatibility with the existing message-processor. This PoC validates InnerTube viability by establishing basic message flow from InnerTube to Redis with exact contract match to official youtube-listener.

</domain>

<decisions>
## Implementation Decisions

### Polling & Reconnection Strategy
- Fixed 1-2 second polling interval (not adaptive)
- Reconnect to stream (new masterchat instance) only on fatal errors (network failure, 401, stream offline)
- Exponential backoff on rate limits and transient errors (2s → 4s → 8s → max 60s)
- Maximum backoff of 60 seconds, then maintain that interval

### Error Handling & Resilience
- Fatal vs transient error classification
  - Fatal: 401 unauthorized, invalid stream, stream offline → stop monitoring
  - Transient: network errors, timeouts, rate limits → retry with backoff
- Configurable log levels via environment variables
  - Debug mode: log everything (useful for PoC debugging)
  - Default mode: log errors only after retries exhausted
- When stream goes offline: stop monitoring immediately, clean up resources
- On fatal errors: stay alive and mark stream as failed (don't crash the service)
  - Prepares for Phase 10 multi-stream support
  - More resilient than crash-and-restart

### Message Contract Strictness
- Target: exact byte-for-byte match with official youtube-listener RawChatMessage output
- Drop extra fields that InnerTube provides but official API doesn't (strict compatibility)
- Missing field handling depends on criticality:
  - Critical fields (user, message, timestamp): fail the message, log error
  - Optional fields (badges, thumbnails): use sensible defaults (empty array, null)
- Strict schema validation before publishing to Redis
  - Validate every field against official listener schema
  - Catch contract drift early in PoC phase

### Health Checks & Startup
- `/health/ready` returns 200 when: Redis connected AND masterchat library initialized
  - Ready even if no stream actively monitored yet
- `/health/live` behavior:
  - Returns 200 normally
  - Returns 500 on deadlock or panic detection (not "always 200")
- Startup grace period: handled by Kubernetes initialDelaySeconds (no internal grace logic)
- Redis unavailable after startup: fail readiness probe but keep service running
  - Kubernetes stops traffic (pod becomes unready)
  - Service retries Redis connection automatically
  - Avoids unnecessary restart churn

### Claude's Discretion
- Exact masterchat library version selection
- Internal data structures for stream state tracking
- Logging format and structured logging details
- Prometheus metrics implementation (labels, naming)

</decisions>

<specifics>
## Specific Ideas

- "This is a PoC - we need to validate InnerTube works before committing to it fully"
- "Byte-for-byte match is critical because message-processor must work with zero changes"
- "Configurable log levels from day 1 - we'll need to debug this in production"
- "Don't crash on Redis failures - we learned that lesson in Phase 6 with connection management"

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. All multi-stream features, HTTP control plane, and stream discovery belong in Phase 10.

</deferred>

---

*Phase: 09-core-ingestion-poc*
*Context gathered: 2026-02-21*
