# Architecture Patterns: InnerTube YouTube Listener

**Domain:** Quota-Free YouTube Live Chat Listener
**Researched:** 2026-02-21
**Confidence:** MEDIUM

## Recommended Architecture

```
┌────────────────────────────────────────────────────────────────────────┐
│                     INNERTUBE-LISTENER (Node.js)                       │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────────┐ │
│  │                      HTTP API Layer (Express)                    │ │
│  │  POST /streams/monitor   DELETE /streams/:id   GET /status      │ │
│  └────────────┬─────────────────────────────────────────────────────┘ │
│               │                                                        │
│  ┌────────────▼─────────────────────────────────────────────────────┐ │
│  │                    Stream Manager (Orchestrator)                 │ │
│  │  - Track active streams (Map<videoId, StreamHandler>)           │ │
│  │  - Lifecycle: start(), stop(), reconnect()                       │ │
│  │  - Health monitoring (heartbeat, error tracking)                 │ │
│  └────────────┬─────────────────────────────────────────────────────┘ │
│               │                                                        │
│        ┌──────┴──────────┬──────────────┬──────────────┐             │
│        │                 │              │              │             │
│  ┌─────▼──────┐    ┌─────▼──────┐ ┌─────▼──────┐ ┌────▼─────┐      │
│  │ Stream     │    │ Stream     │ │ Stream     │ │   ...    │      │
│  │ Handler 1  │    │ Handler 2  │ │ Handler 3  │ │          │      │
│  │            │    │            │ │            │ │          │      │
│  │ videoId: A │    │ videoId: B │ │ videoId: C │ │          │      │
│  └─────┬──────┘    └─────┬──────┘ └─────┬──────┘ └────┬─────┘      │
│        │                 │              │              │             │
│  ┌─────▼─────────────────▼──────────────▼──────────────▼──────────┐ │
│  │              Masterchat Client Pool                             │ │
│  │  - Initialize: Masterchat.init(videoId)                         │ │
│  │  - Iterate: mc.iter().filter(action => action.type === ...)    │ │
│  │  - Error handling: mc.on("error", ...)                          │ │
│  └─────┬────────────────────────────────────────────────────────────┘ │
│        │                                                              │
│  ┌─────▼──────────────────────────────────────────────────────────┐ │
│  │                Message Normalizer                               │ │
│  │  - Map masterchat actions → RawChatMessage schema              │ │
│  │  - Generate UUIDs, extract tags, parse timestamps              │ │
│  └─────┬──────────────────────────────────────────────────────────┘ │
│        │                                                              │
│  ┌─────▼──────────────────────────────────────────────────────────┐ │
│  │              Redis Publisher (ioredis)                          │ │
│  │  - XADD to chat:raw stream                                      │ │
│  │  - Connection pooling, retry logic                             │ │
│  └────────────────────────────────────────────────────────────────┘ │
└────────────────────────────────┬───────────────────────────────────┘
                                 │
                                 │ Redis Streams
                                 │
                ┌────────────────▼────────────────┐
                │  Redis: chat:raw stream         │
                │  (same format as youtube-       │
                │   listener - drop-in)           │
                └────────────────┬────────────────┘
                                 │
                                 │
                ┌────────────────▼────────────────┐
                │  message-processor (Go)         │
                │  Consumes, normalizes, enriches │
                └─────────────────────────────────┘
```

### Component Boundaries

| Component | Responsibility | Communicates With |
|-----------|---------------|-------------------|
| **HTTP API Layer** | Receive control commands from Go services (start/stop monitoring) | Stream Manager |
| **Stream Manager** | Orchestrate multiple stream handlers, lifecycle management | Stream Handlers, HTTP API |
| **Stream Handler** | Manage single stream: masterchat connection, error recovery, state | Masterchat Client, Message Normalizer |
| **Masterchat Client** | Interact with YouTube InnerTube API, receive chat actions | Stream Handler (via async iterator) |
| **Message Normalizer** | Transform masterchat actions → RawChatMessage JSON | Redis Publisher |
| **Redis Publisher** | Publish messages to Redis Streams, handle connection failures | Redis |

---

## Data Flow

### 1. Stream Start Flow

```
overlay-manager (Go)
  │
  │ POST /streams/monitor
  │ Body: { "channelId": "UCxxx", "videoId": "abc123", "overlayId": "uuid" }
  │
  ▼
HTTP API Layer (Express)
  │
  │ Validate request, check auth (SERVICE_SECRET)
  │
  ▼
Stream Manager
  │
  │ Create StreamHandler(videoId)
  │ Add to active streams Map
  │
  ▼
Stream Handler
  │
  │ Initialize: mc = await Masterchat.init(videoId)
  │
  ▼
Masterchat Client
  │
  │ Fetch metadata, start polling InnerTube
  │ Emit async iterator: for await (action of mc.iter())
  │
  ▼
Stream Handler
  │
  │ Filter actions: addChatItemAction, addSuperChatItemAction, etc.
  │
  ▼
Message Normalizer
  │
  │ Map masterchat fields → RawChatMessage
  │ Generate UUID, format timestamp, extract tags
  │
  ▼
Redis Publisher
  │
  │ XADD chat:raw * "data" "{...json...}"
  │
  ▼
Redis Streams (chat:raw)
```

### 2. Stream Stop Flow

```
overlay-manager (Go)
  │
  │ DELETE /streams/:videoId
  │
  ▼
HTTP API Layer (Express)
  │
  │ Validate request
  │
  ▼
Stream Manager
  │
  │ streamHandlers.get(videoId).stop()
  │ streamHandlers.delete(videoId)
  │
  ▼
Stream Handler
  │
  │ masterchat.stop() [if method exists]
  │ Clear async iterator
  │ Emit "stopped" event
  │
  ▼
(Connection closed, resources freed)
```

### 3. Error Recovery Flow

```
Masterchat Client
  │
  │ InnerTube API error (network, rate limit, stream ended)
  │ Emit "error" event
  │
  ▼
Stream Handler
  │
  │ Classify error:
  │  - Temporary (network) → retry with backoff
  │  - Permanent (stream ended) → notify overlay-manager, stop handler
  │  - Rate limit → exponential backoff, retry
  │
  ▼
(if retry)
Stream Handler
  │
  │ Wait backoff duration (1s, 2s, 4s, 8s, 16s max)
  │ Reinitialize: mc = await Masterchat.init(videoId)
  │
  ▼
Masterchat Client (restarted)
```

---

## Patterns to Follow

### Pattern 1: Async Iterator for Continuous Polling

**What:** masterchat provides async iterator API for consuming live chat actions.

**When:** Initializing stream handler to receive messages continuously.

**Example:**
```typescript
const mc = await Masterchat.init(videoId);
const actions = mc.iter().filter(action =>
  action.type === "addChatItemAction"
);

for await (const action of actions) {
  const message = normalizeMessage(action);
  await redisPublisher.publish(message);
}
```

**Why:** Async iterators handle backpressure naturally (pause iteration if Redis slow), cleaner than event emitters for sequential processing.

### Pattern 2: Stream Handler State Machine

**What:** Each stream handler tracks lifecycle state (INITIALIZING → RUNNING → STOPPING → STOPPED → ERROR).

**When:** Managing stream lifecycle, determining recovery strategy.

**States:**
- **INITIALIZING**: Calling `Masterchat.init(videoId)`
- **RUNNING**: Iterator consuming actions, publishing to Redis
- **STOPPING**: Received stop command, cleaning up resources
- **STOPPED**: Cleanly terminated
- **ERROR**: Unrecoverable error (e.g., stream ended, permanent API failure)

**Example:**
```typescript
class StreamHandler {
  state: 'INITIALIZING' | 'RUNNING' | 'STOPPING' | 'STOPPED' | 'ERROR';

  async start() {
    this.state = 'INITIALIZING';
    try {
      this.mc = await Masterchat.init(this.videoId);
      this.state = 'RUNNING';
      await this.consumeActions();
    } catch (err) {
      this.state = 'ERROR';
      throw err;
    }
  }

  async stop() {
    if (this.state === 'RUNNING') {
      this.state = 'STOPPING';
      // Clean up resources
      this.state = 'STOPPED';
    }
  }
}
```

### Pattern 3: Exponential Backoff with Jitter

**What:** Retry failed InnerTube requests with exponentially increasing delays plus random jitter.

**When:** Temporary network errors, rate limiting.

**Example:**
```typescript
async function retryWithBackoff<T>(
  fn: () => Promise<T>,
  maxRetries = 5
): Promise<T> {
  for (let i = 0; i < maxRetries; i++) {
    try {
      return await fn();
    } catch (err) {
      if (i === maxRetries - 1) throw err;

      const baseDelay = Math.min(1000 * Math.pow(2, i), 16000); // Cap at 16s
      const jitter = Math.random() * 1000; // 0-1s jitter
      await sleep(baseDelay + jitter);
    }
  }
}
```

**Why:** Prevents thundering herd (jitter staggers retries), caps delay at reasonable max (16s), exponential growth reduces API load.

### Pattern 4: Graceful Shutdown with Drain

**What:** On SIGTERM, stop accepting new streams, finish processing in-flight messages, close connections.

**When:** Kubernetes pod termination, HPA scale-down.

**Example:**
```typescript
let isShuttingDown = false;

process.on('SIGTERM', async () => {
  console.log('SIGTERM received, starting graceful shutdown');
  isShuttingDown = true;

  // Stop accepting new streams
  server.close();

  // Wait for all stream handlers to finish current messages
  await Promise.all(
    Array.from(streamHandlers.values()).map(h => h.stop())
  );

  // Close Redis connection
  await redis.quit();

  console.log('Graceful shutdown complete');
  process.exit(0);
});
```

**Why:** Prevents message loss (finish in-flight), clean resource cleanup (no leaked connections), respects K8s termination grace period (default 30s).

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Shared Masterchat Client Across Streams

**What:** Using single masterchat instance for multiple video IDs.

**Why bad:** masterchat.init(videoId) is per-stream. Cannot multiplex. Attempting to share will mix chat messages from different streams.

**Instead:** One `Masterchat` instance per video ID (StreamHandler pattern above).

### Anti-Pattern 2: Synchronous Message Processing

**What:** Processing each masterchat action synchronously (await Redis publish inside iterator loop).

**Why bad:** Slow Redis writes block entire iterator, reduces throughput, increases latency.

**Instead:** Buffer messages in-memory, batch publish to Redis (e.g., every 100ms or 50 messages). Trade: complexity for throughput.

**Caveat:** For MVP, synchronous is acceptable (simplicity > performance). Optimize only if Redis becomes bottleneck.

### Anti-Pattern 3: No Error Classification

**What:** Treating all masterchat errors the same (always retry).

**Why bad:** Permanent errors (stream ended, chat disabled) cause infinite retry loops, waste resources.

**Instead:** Classify errors:
- **Retry**: Network errors, temporary InnerTube failures
- **Stop**: Stream ended ("unavailable"), chat disabled ("disabled"), members-only ("membersOnly")
- **Alert**: Unexpected errors (log + notify)

### Anti-Pattern 4: Hardcoded Video IDs in Config

**What:** Storing video IDs in environment variables or config files.

**Why bad:** Video IDs change every time channel goes live. Requires manual update. Defeats purpose of dynamic system.

**Instead:** Accept video IDs via HTTP API (overlay-manager provides dynamically).

---

## Scalability Considerations

| Concern | At 10 streams | At 100 streams | At 1000 streams |
|---------|---------------|----------------|-----------------|
| **Memory** | ~50MB (Node.js base + 10 masterchat connections) | ~200MB (streaming overhead scales linearly) | ~2GB (consider multi-instance, HPA) |
| **CPU** | Negligible (I/O bound, async iterator idle between messages) | <10% single core | ~50% single core (message normalization + Redis publishes) |
| **Network** | ~10 KB/s (10 streams * 1 KB/s avg chat rate) | ~100 KB/s | ~1 MB/s (still low, typical bandwidth) |
| **Redis load** | ~100 ops/sec (10 streams * 10 messages/sec) | ~1000 ops/sec | ~10K ops/sec (Redis handles 100K+, not bottleneck) |
| **Architecture change** | Single instance sufficient | Single instance sufficient, enable HPA (2-3 replicas for HA) | Multi-instance required (HPA 3-10 replicas), consider per-stream leader election (source-manager integration) |

**Bottleneck Analysis:**
- **1-100 streams**: I/O bound (waiting on InnerTube responses). Single instance handles comfortably.
- **100-500 streams**: CPU bound (JSON parsing, normalization). HPA scales horizontally.
- **500+ streams**: Need distributed coordination (prevent duplicate chat ingestion across replicas). Integrate with source-manager leader election (reuse existing pattern from youtube-listener).

---

## Integration with All-Chat Infrastructure

### Source-Manager Integration (Phase 2)

**Leverage existing leader election:**
- Source-manager already implements per-source leader election via Redis locks
- Each stream (video ID) gets leader lock: `source:youtube:innertube:{videoId}:leader`
- Only leader replica polls that stream's chat
- If leader fails, another replica acquires lock and resumes

**Integration points:**
1. StreamHandler checks source-manager before starting: `GET /sources/youtube/{videoId}/leader`
2. If not leader, wait and retry
3. If leader, start masterchat polling
4. Periodically renew leadership (heartbeat pattern)

**Why:** Prevents duplicate message publishing when multiple innertube-listener replicas run (K8s HPA).

### Redis Streams Contract

**CRITICAL:** Output must match official youtube-listener format exactly.

**Verification:**
1. Compare RawChatMessage schema (youtube-listener/models/raw_message.go)
2. Test message-processor can consume InnerTube messages without code changes
3. Validate timestamp format (ISO 8601 UTC), tag keys ("is_verified" not "isVerified")

**Testing strategy:**
- Integration test: innertube-listener publishes → message-processor consumes → validate UnifiedChatMessage output
- Compare side-by-side with official listener output (same stream, both listeners running)

### Docker Compose Networking

**Service definition:**
```yaml
innertube-listener:
  build: ./services/innertube-listener
  image: allchat-innertube-listener:latest
  environment:
    REDIS_HOST: redis
    REDIS_PORT: 6379
    SOURCE_MANAGER_URL: http://source-manager:8088
    SERVICE_SECRET: ${SERVICE_SECRET}
    PORT: 8087
  ports:
    - "8087:8087"  # Expose for Go services
  networks:
    - allchat-network
  depends_on:
    - redis
    - source-manager
  healthcheck:
    test: ["CMD", "wget", "-q", "-O", "/dev/null", "http://localhost:8087/health/live"]
    interval: 10s
    timeout: 5s
    retries: 3
```

**Go service calls InnerTube listener:**
```go
// overlay-manager
type InnerTubeClient struct {
    BaseURL string // "http://innertube-listener:8087"
    Secret  string
}

func (c *InnerTubeClient) MonitorStream(channelID, videoID, overlayID string) error {
    req := MonitorRequest{
        ChannelID: channelID,
        VideoID:   videoID,
        OverlayID: overlayID,
    }
    // POST /streams/monitor with Authorization: Bearer {SERVICE_SECRET}
}
```

---

## Sources

- [masterchat async iterator](https://github.com/sigvt/masterchat/blob/master/MANUAL.md) - `mc.iter()` documented
- [Node.js graceful shutdown](https://oneuptime.com/blog/post/2026-01-23-go-graceful-shutdown/view) - SIGTERM handling patterns (Go examples, but Node.js analogous)
- [Docker Compose service-to-service](https://forums.docker.com/t/cross-container-communication-via-http-post-request/54605) - Service name as hostname
- [All-Chat source-manager](file:///home/moersener/Hobby/all-chat/services/source-manager/README.md) - Leader election via Redis locks (inferred from architecture, need to verify)
- [Official youtube-listener README](file:///home/moersener/Hobby/all-chat/services/youtube-listener/README.md) - Per-stream leader election pattern
