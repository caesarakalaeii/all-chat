# Phase 9: Core Ingestion PoC - Research

**Researched:** 2026-02-21
**Domain:** YouTube InnerTube (unofficial API) integration with Go, Redis Streams publishing, message normalization
**Confidence:** MEDIUM

## Summary

Phase 9 implements a YouTube chat listener using the InnerTube (unofficial) API to replace the official YouTube Data API v3, eliminating quota constraints while maintaining drop-in compatibility with the existing message-processor. The primary challenge is that **masterchat (the reference InnerTube library) is TypeScript/JavaScript**, while our service must be written in Go to match the existing architecture. No mature Go port of masterchat exists, so we must either: (1) build a minimal Go InnerTube client from scratch using observed network patterns, or (2) wrap the masterchat Node.js library via inter-process communication.

The research reveals that InnerTube is YouTube's internal API used by their web/mobile clients. It has no official documentation but is reverse-engineered by the community. The API uses continuation-based polling (similar to pagination), where each response includes a `continuation` token for the next request. Unlike the official API which costs 5 units per poll, InnerTube has no quota system, only rate limiting to prevent abuse.

**Primary recommendation:** Build a minimal Go InnerTube client focused solely on live chat polling for this PoC. Use the `abhinavxd/youtube-live-chat-downloader/v2` Go library as a reference implementation for continuation token handling and message parsing, while validating against masterchat's TypeScript codebase for message structure accuracy.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Polling & Reconnection Strategy
- Fixed 1-2 second polling interval (not adaptive)
- Reconnect to stream (new masterchat instance) only on fatal errors (network failure, 401, stream offline)
- Exponential backoff on rate limits and transient errors (2s → 4s → 8s → max 60s)
- Maximum backoff of 60 seconds, then maintain that interval

#### Error Handling & Resilience
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

#### Message Contract Strictness
- Target: exact byte-for-byte match with official youtube-listener RawChatMessage output
- Drop extra fields that InnerTube provides but official API doesn't (strict compatibility)
- Missing field handling depends on criticality:
  - Critical fields (user, message, timestamp): fail the message, log error
  - Optional fields (badges, thumbnails): use sensible defaults (empty array, null)
- Strict schema validation before publishing to Redis
  - Validate every field against official listener schema
  - Catch contract drift early in PoC phase

#### Health Checks & Startup
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
- Exact masterchat library version selection (or if we build our own Go client)
- Internal data structures for stream state tracking
- Logging format and structured logging details
- Prometheus metrics implementation (labels, naming)

### Deferred Ideas (OUT OF SCOPE)
None — discussion stayed within phase scope. All multi-stream features, HTTP control plane, and stream discovery belong in Phase 10.

</user_constraints>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.25+ | Service runtime | Matches existing service architecture (twitch-listener, youtube-listener) |
| github.com/redis/go-redis/v9 | v9.x | Redis Streams (XADD) | Standard Redis client across all services, battle-tested with existing youtube-listener |
| go.uber.org/zap | Latest | Structured logging | Project standard for all services, supports configurable log levels, high performance |
| github.com/gin-gonic/gin | Latest | HTTP server (health checks) | Standard HTTP framework for all listener services |
| github.com/google/uuid | Latest | Message ID generation | Used by existing youtube-listener for RawChatMessage.MessageID |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/cenkalti/backoff/v4 | v4.x | Exponential backoff | Required for transient error retry with jitter (prevent thundering herd) |
| github.com/abhinavxd/youtube-live-chat-downloader/v2 | v2.0.3 | Reference implementation | Study continuation token handling, not direct dependency |
| net/http | stdlib | HTTP client for InnerTube | Standard library sufficient for simple POST requests with JSON payloads |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Custom Go InnerTube client | masterchat Node.js wrapper via subprocess | Subprocess adds complexity, crash risk, resource overhead. Custom Go client simpler for PoC scope. |
| github.com/cenkalti/backoff/v4 | github.com/avast/retry-go | avast/retry-go offers more features (jitter by default) but backoff/v4 is simpler and sufficient |
| net/http stdlib | Third-party HTTP client | Stdlib is battle-tested, zero dependencies. InnerTube doesn't need advanced features (connection pooling sufficient). |

**Installation:**
```bash
go get github.com/redis/go-redis/v9
go get go.uber.org/zap
go get github.com/gin-gonic/gin
go get github.com/google/uuid
go get github.com/cenkalti/backoff/v4
```

## Architecture Patterns

### Recommended Project Structure
```
services/youtube-listener-innertube/
├── cmd/
│   └── main.go                    # Entry point (logger, Redis, HTTP server, graceful shutdown)
├── innertube/
│   ├── client.go                  # InnerTube HTTP client (POST to browse endpoint)
│   ├── parser.go                  # Parse InnerTube JSON responses → RawChatMessage
│   └── types.go                   # InnerTube-specific types (ChatAction, AuthorDetails, etc.)
├── poller/
│   ├── poller.go                  # Polling loop with continuation token management
│   ├── backoff.go                 # Exponential backoff state machine
│   └── state.go                   # Stream state tracking (active/failed/offline)
├── publisher/
│   └── redis_publisher.go         # Publish RawChatMessage to Redis Streams (chat:raw)
├── handlers/
│   └── health.go                  # /health/live and /health/ready endpoints
├── go.mod
├── Dockerfile
└── README.md
```

### Pattern 1: Continuation-Based Polling Loop

**What:** InnerTube uses continuation tokens (like pagination cursors) to fetch new messages. Each API response includes a `continuation` field for the next request.

**When to use:** Core polling mechanism for live chat.

**Example:**
```go
// Simplified from abhinavxd/youtube-live-chat-downloader/v2 reference
type Poller struct {
    client       *innertube.Client
    continuation string
    interval     time.Duration
    backoff      *Backoff
}

func (p *Poller) Poll(ctx context.Context) ([]*models.RawChatMessage, error) {
    // POST to InnerTube browse endpoint with continuation token
    resp, err := p.client.GetLiveChatReplay(ctx, p.continuation)
    if err != nil {
        return nil, fmt.Errorf("innertube request failed: %w", err)
    }

    // Parse actions into RawChatMessage format
    messages, err := p.client.ParseMessages(resp.Actions)
    if err != nil {
        return nil, fmt.Errorf("parse failed: %w", err)
    }

    // Update continuation for next poll
    p.continuation = resp.Continuation

    return messages, nil
}
```

**Source:** Conceptual pattern from [abhinavxd/youtube-live-chat-downloader](https://github.com/abhinavxd/youtube-live-chat-downloader) architecture

### Pattern 2: Transient vs Fatal Error Classification

**What:** Classify errors to decide whether to retry or stop monitoring.

**When to use:** All InnerTube API call error handling.

**Example:**
```go
func classifyError(err error) ErrorType {
    // HTTP status codes from InnerTube response
    if statusErr, ok := err.(*HTTPStatusError); ok {
        switch statusErr.StatusCode {
        case 401: // Unauthorized
            return FatalError // Stop monitoring, stream requires auth
        case 404: // Stream not found
            return FatalError // Stop monitoring, stream ended or invalid ID
        case 429: // Rate limited
            return TransientError // Retry with backoff
        case 500, 502, 503, 504: // Server errors
            return TransientError // Retry with backoff
        default:
            return TransientError // Assume transient for unknown codes
        }
    }

    // Network errors (connection refused, timeout, etc.)
    if isNetworkError(err) {
        return TransientError // Retry with backoff
    }

    return UnknownError // Log and treat as transient
}
```

### Pattern 3: Strict Message Validation Before Publishing

**What:** Validate RawChatMessage against official youtube-listener schema to catch InnerTube API changes early.

**When to use:** After parsing InnerTube response, before publishing to Redis.

**Example:**
```go
func ValidateRawMessage(msg *models.RawChatMessage) error {
    // Critical fields (must exist and be non-empty)
    if msg.MessageID == "" {
        return fmt.Errorf("critical field missing: message_id")
    }
    if msg.Platform != "youtube" {
        return fmt.Errorf("invalid platform: %s (expected youtube)", msg.Platform)
    }
    if msg.UserID == "" {
        return fmt.Errorf("critical field missing: user_id")
    }
    if msg.Username == "" {
        return fmt.Errorf("critical field missing: username")
    }
    if msg.Text == "" && msg.EventType == "" {
        return fmt.Errorf("message has no text and no event_type")
    }
    if msg.Timestamp.IsZero() {
        return fmt.Errorf("critical field missing: timestamp")
    }

    // Optional fields (use defaults if missing)
    if msg.Tags == nil {
        msg.Tags = make(map[string]string)
    }

    // Validate schema matches official youtube-listener
    // (compare to services/youtube-listener/models/raw_message.go)

    return nil
}
```

### Pattern 4: Configurable Log Levels with Environment Variables

**What:** Support debug mode (log everything) and default mode (log errors only after retries exhausted).

**When to use:** Service initialization, all logging calls.

**Example:**
```go
// From shared/logger/logger.go pattern
func NewLogger(serviceName, level string) *zap.Logger {
    config := zap.NewProductionConfig()
    config.Level = zap.NewAtomicLevelAt(parseLevel(level))
    // ... rest of config
}

// Usage in poller with configurable verbosity
if p.logLevel == "debug" {
    p.logger.Debug("Polling InnerTube",
        zap.String("continuation", p.continuation),
        zap.Int("attempt", attempt),
    )
}

// Error logging after retries exhausted
if err := p.Poll(ctx); err != nil {
    if isRetryExhausted(err) {
        p.logger.Error("Polling failed after retries",
            zap.Error(err),
            zap.Int("attempts", maxAttempts),
        )
    }
}
```

**Source:** Existing pattern from [shared/logger/logger.go](/home/moersener/Hobby/all-chat/shared/logger/logger.go)

### Pattern 5: Redis Streams Publishing with go-redis/v9

**What:** Publish RawChatMessage to Redis Streams using XAdd with MAXLEN for memory management.

**When to use:** After successful InnerTube poll and message validation.

**Example:**
```go
// From services/twitch-listener/publisher/stream_publisher.go
func (p *StreamPublisher) Publish(ctx context.Context, msg *models.RawChatMessage) error {
    jsonBytes, err := msg.ToJSON()
    if err != nil {
        return fmt.Errorf("failed to marshal message: %w", err)
    }

    values := map[string]interface{}{
        "message_id": msg.MessageID,
        "platform":   msg.Platform,
        "channel_id": msg.ChannelID,
        "user_id":    msg.UserID,
        "username":   msg.Username,
        "text":       msg.Text,
        "timestamp":  msg.Timestamp.Format(time.RFC3339Nano),
        "data":       string(jsonBytes), // Full JSON for message-processor
    }

    args := &redis.XAddArgs{
        Stream: "chat:raw",
        MaxLen: 1000000, // 1 million messages sliding window
        Approx: true,    // ~MAXLEN for performance
        Values: values,
    }

    streamID, err := p.client.XAdd(ctx, args).Result()
    if err != nil {
        return fmt.Errorf("redis XAdd failed: %w", err)
    }

    p.logger.Debug("Published to Redis Streams",
        zap.String("stream_id", streamID),
        zap.String("message_id", msg.MessageID),
    )

    return nil
}
```

**Source:** Exact pattern from [services/twitch-listener/publisher/stream_publisher.go](/home/moersener/Hobby/all-chat/services/twitch-listener/publisher/stream_publisher.go)

### Pattern 6: Graceful Shutdown with Signal Handling

**What:** Handle SIGINT/SIGTERM to stop polling gracefully and close connections.

**When to use:** Main function.

**Example:**
```go
// From services/twitch-listener/cmd/main.go pattern
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

go func() {
    // Start poller
    poller.Start(ctx)
}()

<-quit // Block until signal received

log.Info("Shutting down service...")

// Stop poller (close connections, finish in-flight requests)
poller.Stop()

// Shutdown HTTP server with timeout
shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
if err := srv.Shutdown(shutdownCtx); err != nil {
    log.Error("HTTP server forced shutdown", zap.Error(err))
}
```

**Source:** Standard pattern from [Graceful Shutdown in Golang (Gin): A Complete Guide](https://medium.com/@kittipat_1413/graceful-shutdown-in-golang-gin-a-complete-guide-130e3f075415)

### Anti-Patterns to Avoid

- **Don't use adaptive polling intervals:** User decision requires fixed 1-2s interval. InnerTube responses may suggest `pollingIntervalMillis` but we must ignore it for PoC phase.
- **Don't crash on non-fatal errors:** Service must stay alive and mark stream as failed. This prepares for Phase 10 multi-stream support where one bad stream shouldn't kill the entire service.
- **Don't log everything in production:** Use configurable log levels. Debug logs add significant overhead and should only be enabled during troubleshooting.
- **Don't skip schema validation:** Byte-for-byte contract match is critical. InnerTube API changes frequently (no official support), and early detection prevents downstream breakage in message-processor.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Exponential backoff | Custom sleep loop with manual state | github.com/cenkalti/backoff/v4 | Handles edge cases (jitter, max elapsed time, reset), battle-tested across Go ecosystem |
| UUID generation | String concatenation with timestamps | github.com/google/uuid | Correct UUID v4 implementation, collision-resistant, matches existing youtube-listener pattern |
| JSON parsing | Manual string manipulation | encoding/json + struct tags | Type-safe, handles edge cases (escaping, nested objects), standard library |
| HTTP retry logic | Manual for-loop with error checks | Backoff wrapper around http.Client | Prevents thundering herd with jitter, respects context cancellation |
| Redis connection pooling | Manual connection management | go-redis/v9 built-in pooling | Handles reconnection, health checks, concurrent access correctly |

**Key insight:** InnerTube is reverse-engineered and undocumented. Hand-rolling parsers for InnerTube JSON responses is acceptable (no official library exists), but use standard libraries for everything else (backoff, HTTP, Redis, logging). The unofficial nature increases risk of API changes, making robust error handling and validation critical.

## Common Pitfalls

### Pitfall 1: Continuation Token Expiration

**What goes wrong:** Continuation tokens expire after ~60 seconds of inactivity. If polling stops (due to backoff or crash recovery), the token becomes invalid and InnerTube returns 400 Bad Request.

**Why it happens:** InnerTube treats continuation tokens as temporary session identifiers. Long backoff periods (e.g., 60s max) can cause token expiration.

**How to avoid:** On continuation token errors (400 with "continuation not found"), treat as fatal error and stop monitoring. Don't attempt to fetch a new token (requires stream ID lookup, out of scope for Phase 9). In Phase 10, the control plane will handle stream re-initialization.

**Warning signs:** Seeing 400 errors after successful polling, especially after long backoff periods. Log continuation token age to detect this early.

### Pitfall 2: InnerTube Rate Limiting Detection

**What goes wrong:** InnerTube has no documented rate limits. Polling too fast can trigger IP-based rate limiting (429 or connection resets), but the exact threshold is unknown.

**Why it happens:** YouTube protects their infrastructure from abuse. Rate limits are dynamic and may vary based on IP reputation, time of day, etc.

**How to avoid:**
- Respect user-specified 1-2s polling interval (don't go faster)
- On 429 errors, exponential backoff to 60s max
- Add jitter to prevent synchronized polling across replicas
- Monitor HTTP error rates; sustained 429s indicate rate limiting

**Warning signs:** Sudden spike in 429 errors, connection timeouts, or "service temporarily unavailable" errors after extended polling. Consider adding per-stream rate limit tracking in Phase 10.

### Pitfall 3: InnerTube Message Structure Changes

**What goes wrong:** InnerTube is an internal API with no compatibility guarantees. Field names, nesting, or types can change without notice, breaking message parsing.

**Why it happens:** YouTube updates their web/mobile clients frequently. InnerTube schema follows client needs, not external consumers.

**How to avoid:**
- Strict schema validation before Redis publish (fail fast on unexpected structure)
- Version detection: check for `serviceTrackingParams` or similar metadata fields that indicate API version
- Defensive parsing: use optional fields with default values for non-critical data
- Automated testing: compare parsed messages against official youtube-listener output in CI

**Warning signs:** Sudden drop in message parsing success rate, schema validation failures, missing fields that previously existed. Set up alerting on parse error metrics.

### Pitfall 4: Redis Connection Failures After Startup

**What goes wrong:** Redis becomes unavailable after service starts (network partition, Redis restart). Service continues running but fails to publish messages, causing data loss.

**Why it happens:** User decision: "keep service running, fail readiness probe" instead of crashing. Redis errors are logged but not fatal.

**How to avoid:**
- Implement Redis health check in `/health/ready` endpoint
- Retry Redis publish with exponential backoff (separate from InnerTube backoff)
- Circuit breaker: after N consecutive Redis failures, stop polling until Redis recovers
- Kubernetes will mark pod unready and stop traffic, allowing manual intervention

**Warning signs:** Rising Redis error metrics, messages logged as "failed to publish" but polling continues. Monitor Redis connection state separately from InnerTube polling state.

### Pitfall 5: Byte-for-Byte Schema Mismatch

**What goes wrong:** InnerTube provides extra metadata fields (e.g., `contextMenuAccessibility`, `trackingParams`) not present in official YouTube API. Including these breaks contract with message-processor.

**Why it happens:** InnerTube is designed for YouTube's own clients, which need UI rendering data. Official API only returns data relevant to third-party developers.

**How to avoid:**
- Explicit field mapping: only copy fields that exist in official youtube-listener RawChatMessage
- Unit tests: compare InnerTube-parsed message JSON with official listener output
- Validation step: reject messages with unexpected fields (strict mode for PoC)
- Documentation: maintain mapping table of InnerTube → RawChatMessage field names

**Warning signs:** Message-processor logs warnings about unknown fields, emote enrichment failures due to unexpected data structure. Add schema validation tests in CI/CD pipeline.

## Code Examples

Verified patterns from research and existing codebase:

### InnerTube HTTP Client (POST Request)

```go
// Conceptual example based on abhinavxd/youtube-live-chat-downloader/v2 architecture
type Client struct {
    httpClient *http.Client
    apiKey     string // InnerTube API key (extracted from initial HTML)
    logger     *zap.Logger
}

func (c *Client) GetLiveChatReplay(ctx context.Context, continuation string) (*LiveChatResponse, error) {
    // InnerTube browse endpoint
    url := "https://www.youtube.com/youtubei/v1/live_chat/get_live_chat_replay?key=" + c.apiKey

    // Request payload with continuation token
    payload := map[string]interface{}{
        "continuation": continuation,
        "context": map[string]interface{}{
            "client": map[string]interface{}{
                "clientName":    "WEB",
                "clientVersion": "2.20250101.00.00", // Update regularly
            },
        },
    }

    jsonPayload, err := json.Marshal(payload)
    if err != nil {
        return nil, fmt.Errorf("marshal payload: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonPayload))
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("http request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != 200 {
        return nil, &HTTPStatusError{StatusCode: resp.StatusCode}
    }

    var chatResp LiveChatResponse
    if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
        return nil, fmt.Errorf("decode response: %w", err)
    }

    return &chatResp, nil
}
```

**Source:** Conceptual pattern from [abhinavxd/youtube-live-chat-downloader](https://github.com/abhinavxd/youtube-live-chat-downloader) request structure

### Exponential Backoff with Jitter

```go
// Using cenkalti/backoff/v4
import "github.com/cenkalti/backoff/v4"

type Backoff struct {
    policy *backoff.ExponentialBackOff
    logger *zap.Logger
}

func NewBackoff() *Backoff {
    policy := backoff.NewExponentialBackOff()
    policy.InitialInterval = 2 * time.Second  // Start at 2s per user decision
    policy.Multiplier = 2.0                    // Double each time (2s → 4s → 8s)
    policy.MaxInterval = 60 * time.Second      // Cap at 60s per user decision
    policy.MaxElapsedTime = 0                  // Never stop retrying (infinite)

    return &Backoff{
        policy: policy,
        logger: logger,
    }
}

func (b *Backoff) Wait(ctx context.Context, err error) error {
    if !isTransientError(err) {
        // Fatal error, don't wait
        return err
    }

    duration := b.policy.NextBackOff()
    if duration == backoff.Stop {
        return fmt.Errorf("backoff exhausted")
    }

    b.logger.Warn("Transient error, backing off",
        zap.Error(err),
        zap.Duration("wait", duration),
    )

    select {
    case <-time.After(duration):
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

func (b *Backoff) Reset() {
    b.policy.Reset() // Reset to 2s after successful operation
}
```

**Source:** Pattern from [cenkalti/backoff/v4 documentation](https://pkg.go.dev/github.com/cenkalti/backoff/v4) and user-specified backoff parameters

### Health Check Endpoints (Gin)

```go
// From services/twitch-listener/handlers/health.go pattern
func SetupHealthRoutes(router *gin.Engine, redisClient *redis.Client, innertubeClient *innertube.Client) {
    router.GET("/health/live", func(c *gin.Context) {
        // Liveness: always return 200 unless deadlock detected
        // (Kubernetes will restart pod on deadlock)
        c.JSON(http.StatusOK, gin.H{"status": "alive"})
    })

    router.GET("/health/ready", func(c *gin.Context) {
        ctx := c.Request.Context()

        // Check Redis connection
        if err := redisClient.Ping(ctx).Err(); err != nil {
            c.JSON(http.StatusServiceUnavailable, gin.H{
                "status": "not_ready",
                "error":  "redis connection failed",
                "detail": err.Error(),
            })
            return
        }

        // Check InnerTube client initialized (basic health, not full poll)
        if !innertubeClient.IsInitialized() {
            c.JSON(http.StatusServiceUnavailable, gin.H{
                "status": "not_ready",
                "error":  "innertube client not initialized",
            })
            return
        }

        c.JSON(http.StatusOK, gin.H{"status": "ready"})
    })
}
```

**Source:** Existing pattern from [services/message-processor/cmd/main.go](/home/moersener/Hobby/all-chat/services/message-processor/cmd/main.go) lines 531-555

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Official YouTube Data API v3 | InnerTube (unofficial) API | Phase 9 (2026-02) | Eliminates 10,000 units/day quota constraint, requires custom Go client instead of google.golang.org/api/youtube/v3 |
| Adaptive polling (pollingIntervalMillis) | Fixed 1-2s interval | Phase 9 (user decision) | Simpler implementation for PoC, predictable load, may need adjustment in Phase 10 based on rate limiting |
| Crash on Redis failure | Fail readiness probe, stay alive | Phase 9 (user decision) | Prepares for multi-stream support (Phase 10), more resilient than restart churn, requires circuit breaker logic |
| Masterchat TypeScript library | Custom Go InnerTube client | Phase 9 (no mature Go port exists) | Increased implementation complexity, need to reverse-engineer InnerTube protocol, higher maintenance risk |

**Deprecated/outdated:**
- **github.com/iopred/ytlivechatapi**: Archived in 2015, replaced by official Google API. Not suitable for InnerTube (uses old protocol).
- **cenkalti/backoff v3**: Use v4 for better Go module support and context cancellation.
- **masterchat v1.1.0 (npm)**: Last updated June 2022. Check for active forks (e.g., @stu43005/masterchat v1.5.0 updated 10 months ago) for recent InnerTube changes.

## Open Questions

### 1. **InnerTube API Key Extraction**
   - **What we know:** InnerTube requires an API key in the query string (`?key=...`). This key is embedded in YouTube's HTML page source and changes periodically.
   - **What's unclear:** How often does the API key rotate? Can we cache it? Do we extract it once at startup or per-stream?
   - **Recommendation:** Extract API key from initial stream HTML page (parse `<script>` tags for `"INNERTUBE_API_KEY"`). Cache for service lifetime. Monitor for 401 errors indicating key expiration. If key expires, restart service (Kubernetes will handle pod recycling). Document extraction logic clearly for Phase 10.

### 2. **Continuation Token Format Changes**
   - **What we know:** Continuation tokens are opaque strings (base64-encoded protocol buffers). Format may change with InnerTube updates.
   - **What's unclear:** Will token format changes break our polling loop? Do we need version detection?
   - **Recommendation:** Treat continuation tokens as opaque strings (no parsing/validation). If token becomes invalid (400 error), log token length and first 20 chars (obfuscated) for debugging. Add metric: `continuation_token_errors_total`. Accept that InnerTube changes may require code updates.

### 3. **Message Deduplication at InnerTube Level**
   - **What we know:** Official YouTube API can return duplicate messages. Message-processor has deduplication logic (1-hour TTL in Redis registry).
   - **What's unclear:** Does InnerTube also return duplicates? Should we deduplicate before publishing to Redis?
   - **Recommendation:** Don't deduplicate in youtube-listener-innertube. Rely on message-processor's existing deduplication (maintains parity with official listener). If duplicates become a problem, add InnerTube-specific deduplication in Phase 10 with separate Redis key namespace.

### 4. **Polling Interval Lower Bound**
   - **What we know:** User specified 1-2s interval. InnerTube rate limits are undocumented.
   - **What's unclear:** Is 1s too fast? Will it trigger rate limiting? Should we start at 2s and only go to 1s after testing?
   - **Recommendation:** Start with 2s interval for PoC. Monitor 429 error rates. If no rate limiting observed after 24h of testing, consider 1s. Add environment variable `INNERTUBE_POLL_INTERVAL_MS` (default 2000) for easy tuning without code changes.

### 5. **Stream Offline Detection**
   - **What we know:** When stream ends, InnerTube returns specific error. User decision: stop monitoring immediately.
   - **What's unclear:** What exact error response indicates "stream offline"? Is it 404, or a specific JSON field?
   - **Recommendation:** Study masterchat TypeScript code for offline detection logic. Look for fields like `continuations` array empty, or `isLive: false` in response. Document exact detection criteria. Add metric: `streams_ended_total`.

## Sources

### Primary (HIGH confidence)

- **Official Go Libraries:**
  - [github.com/redis/go-redis/v9](https://pkg.go.dev/github.com/redis/go-redis/v9) - Redis Streams XADD documentation
  - [go.uber.org/zap](https://pkg.go.dev/go.uber.org/zap) - Structured logging with configurable levels
  - [github.com/cenkalti/backoff/v4](https://pkg.go.dev/github.com/cenkalti/backoff/v4) - Exponential backoff algorithms

- **Existing Codebase:**
  - [services/youtube-listener/models/raw_message.go](/home/moersener/Hobby/all-chat/services/youtube-listener/models/raw_message.go) - RawChatMessage schema (lines 8-24)
  - [services/twitch-listener/publisher/stream_publisher.go](/home/moersener/Hobby/all-chat/services/twitch-listener/publisher/stream_publisher.go) - Redis Streams publishing pattern (lines 36-82)
  - [services/message-processor/cmd/main.go](/home/moersener/Hobby/all-chat/services/message-processor/cmd/main.go) - Health check pattern (lines 531-555)
  - [shared/logger/logger.go](/home/moersener/Hobby/all-chat/shared/logger/logger.go) - Configurable log levels (lines 11-44)

- **InnerTube Reference Implementations:**
  - [GitHub - abhinavxd/youtube-live-chat-downloader](https://github.com/abhinavxd/youtube-live-chat-downloader) - Go InnerTube client for live chat (v2.0.3, updated March 2022)
  - [GitHub - sigvt/masterchat](https://github.com/sigvt/masterchat) - TypeScript InnerTube library, 20+ action types, continuation handling (v1.1.0, June 2022)
  - [GitHub - HolodexNet/masterchat](https://github.com/HolodexNet/masterchat) - Active fork of masterchat with recent maintenance

### Secondary (MEDIUM confidence)

- **Go Error Handling Patterns:**
  - [Implementing Exponential Backoff in Go](https://medium.com/@barikhan.hillol/implementing-exponential-backoff-in-go-for-effective-retry-strategies-32b2c94cb52d) - Retry strategies with backoff
  - [Graceful Shutdown in Golang (Gin): A Complete Guide](https://medium.com/@kittipat_1413/graceful-shutdown-in-golang-gin-a-complete-guide-130e3f075415) - Signal handling, timeout management
  - [How to Implement Graceful Shutdown in Go for Kubernetes](https://oneuptime.com/blog/post/2026-01-07-go-graceful-shutdown-kubernetes/view) - Kubernetes-specific shutdown patterns (2026 guide)

- **Zap Logger Documentation:**
  - [Zap: The High-Performance, Structured Logging Framework for Go](https://www.dash0.com/faq/zap-the-high-performance-structured-logging-framework-for-go-in-2025) - 2025 performance benchmarks
  - [Mastering Production-Grade Logging in Go: Complete 2025 Guide to Uber Zap](https://medium.com/@mamidipaka2003/mastering-production-grade-logging-in-go-golang-the-complete-2025-guide-to-uber-zap-94622c874f1b) - Best practices for production

- **InnerTube API Context:**
  - [Extract YouTube Transcripts Using Innertube API (2025 JavaScript Guide)](https://medium.com/@aqib-2/extract-youtube-transcripts-using-innertube-api-2025-javascript-guide-dc417b762f49) - InnerTube authentication and request structure
  - [Is the YouTube API Free? Costs, Limits, and What You Actually Get](https://www.getphyllo.com/post/is-the-youtube-api-free-costs-limits-iv) - Official API quota context (why InnerTube is needed)

### Tertiary (LOW confidence - marked for validation)

- **Go InnerTube Libraries:**
  - [GitHub - wslyyy/youtube-go](https://github.com/wslyyy/youtube-go) - Go InnerTube client (6 commits, OAuth not implemented, no live chat support documented)
  - [GitHub - Agash/YTLiveChat](https://github.com/Agash/YTLiveChat) - .NET InnerTube library (v4.0.0, Feb 2026, shows InnerTube protocol but wrong language)

- **Validation needed:** Exact InnerTube endpoint URLs, API key rotation frequency, continuation token TTL, rate limit thresholds, stream offline detection JSON structure.

## Metadata

**Confidence breakdown:**
- **Standard stack: HIGH** - Redis, Zap, Gin are verified from existing codebase. Backoff library is industry standard.
- **Architecture: MEDIUM** - Patterns derived from existing services (twitch-listener, youtube-listener) and Go InnerTube reference (abhinavxd). InnerTube-specific details need validation against live API.
- **Pitfalls: MEDIUM** - Common InnerTube issues documented in masterchat GitHub issues and YouTube scraping community forums. Exact rate limits and token expiration values need empirical testing.

**Research date:** 2026-02-21
**Valid until:** 2026-03-21 (30 days for stable stack, 7 days for InnerTube-specific details - API changes frequently)

**Critical unknowns requiring validation:**
1. InnerTube API key extraction and rotation frequency
2. Exact continuation token TTL and expiration behavior
3. Rate limiting thresholds (requests/second per IP)
4. Stream offline detection JSON response structure
5. InnerTube message schema stability (field names, nesting, types)

**Recommended next steps for planner:**
1. Create minimal Go InnerTube client using abhinavxd as reference
2. Implement strict message validation matching official youtube-listener schema
3. Add comprehensive logging for InnerTube responses (debug mode)
4. Create integration test comparing InnerTube vs official API message output
5. Document all InnerTube-specific assumptions for Phase 10 validation
