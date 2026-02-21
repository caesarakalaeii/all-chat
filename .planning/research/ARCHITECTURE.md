# Architecture Research: InnerTube YouTube Listener Integration

**Domain:** InnerTube-based YouTube Live Chat Listener for All-Chat
**Researched:** 2026-02-21
**Confidence:** HIGH

## Integration Strategy

### Decision: Separate Service (Drop-In Replacement)

**Recommendation:** Deploy InnerTube YouTube listener as a **separate, standalone service** that is a drop-in replacement for the existing official API-based youtube-listener.

**Rationale:**
1. **Self-Hoster Choice**: Users can choose official API (with quota limits) OR InnerTube (quota-free but unofficial)
2. **Risk Isolation**: If YouTube breaks InnerTube API, official listener remains functional
3. **Zero Architecture Changes**: Both publish identical Redis Stream messages
4. **Deployment Flexibility**: Switch via image tag in Kubernetes deployment
5. **Testing Safety**: Run both in parallel during migration, compare outputs

**NOT Recommended:**
- Single service with mode switch (more complexity, larger binary, shared failure modes)
- Modifying existing youtube-listener (breaks single responsibility, complicates rollback)

---

## System Overview

### InnerTube Listener in Existing Architecture

```
┌──────────────────────────────────────────────────────────────────────────┐
│                          EXISTING ARCHITECTURE                            │
├──────────────────────────────────────────────────────────────────────────┤
│  Platform Listeners (Microservices)                                       │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │
│  │   Twitch     │  │   YouTube    │  │     Kick     │  │   TikTok     │ │
│  │  Listener    │  │   Listener   │  │   Listener   │  │   Listener   │ │
│  │  (IRC)       │  │ (Official API)│  │  (Pusher WS) │  │ (Unofficial) │ │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘ │
│         │                 │                 │                 │          │
│         └─────────────────┴─────────────────┴─────────────────┘          │
│                                  ↓                                        │
│         Redis Streams: chat:raw (RawChatMessage format)                   │
│                                  ↓                                        │
│  ┌──────────────────────────────────────────────────────────────────┐    │
│  │            Message Processor (Consumer Group)                     │    │
│  │  Normalize → Enrich (emotes) → Route (overlay mapping)           │    │
│  └──────────────────────────────┬───────────────────────────────────┘    │
│                                  ↓                                        │
│         Redis Pub/Sub: overlay:{overlay_id}                               │
│                                  ↓                                        │
│  ┌──────────────────────────────────────────────────────────────────┐    │
│  │              API Gateway → WebSocket → Frontend                   │    │
│  └──────────────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────────────────┐
│                      NEW: INNERTUBE INTEGRATION                           │
├──────────────────────────────────────────────────────────────────────────┤
│  Self-Hoster Chooses ONE of:                                              │
│                                                                            │
│  Option A: youtube-listener (Official API)                                │
│    ┌──────────────────────────────────────────────────────────┐          │
│    │  - HTTP polling (liveChatMessages.list)                   │          │
│    │  - OAuth 2.0 per user                                     │          │
│    │  - Quota tracking (1,009,000 units/day)                   │          │
│    │  - Leader election (per stream)                           │          │
│    │  - PostgreSQL quota state                                 │          │
│    └────────────────────────┬─────────────────────────────────┘          │
│                             ↓                                             │
│              Redis Streams: chat:raw (platform=youtube)                   │
│                                                                            │
│  Option B: youtube-innertube-listener (InnerTube API) ← NEW               │
│    ┌──────────────────────────────────────────────────────────┐          │
│    │  - InnerTube continuation token polling                   │          │
│    │  - NO OAuth (uses YouTube web context)                    │          │
│    │  - NO quota limits (unofficial API)                       │          │
│    │  - Leader election (per stream, same pattern)             │          │
│    │  - Rate limit awareness (avoid IP blocks)                 │          │
│    └────────────────────────┬─────────────────────────────────┘          │
│                             ↓                                             │
│              Redis Streams: chat:raw (platform=youtube)                   │
│                             ↓                                             │
│                      IDENTICAL OUTPUT                                     │
│  Both listeners produce same RawChatMessage schema                        │
│  Message Processor cannot distinguish between them                        │
└──────────────────────────────────────────────────────────────────────────┘
```

---

## Component Responsibilities

### New Service: youtube-innertube-listener

| Component | Responsibility | Implementation |
|-----------|----------------|----------------|
| **InnerTube Client** | HTTP client for YouTube's private API | Go HTTP client with custom headers mimicking browser |
| **Continuation Manager** | Obtain and refresh continuation tokens | Extract from initial page load, poll with continuation |
| **Chat Poller** | Poll InnerTube endpoint for chat messages | HTTP POST to `/youtubei/v1/live_chat/get_live_chat` |
| **Message Parser** | Parse InnerTube JSON response to RawChatMessage | Extract actions, map to unified schema |
| **Rate Limiter** | Prevent IP blocks from aggressive polling | Configurable polling interval (default 1000-2000ms) |
| **Leader Election Client** | Claim/renew leadership for streams | HTTP client to source-manager (same as official listener) |
| **Redis Publisher** | Publish to chat:raw stream | Same XAdd pattern as all other listeners |
| **Health Checks** | Kubernetes liveness/readiness | Same /health/live, /health/ready endpoints |

### Reused Components (No Changes)

| Component | Used By | Notes |
|-----------|---------|-------|
| **Source Manager** | Both listeners | Same leader election API, stream ID format |
| **Message Processor** | Both listeners | Cannot distinguish source (same platform=youtube) |
| **API Gateway** | Both listeners | No changes, subscribes to same Pub/Sub channels |
| **Overlay Manager** | Both listeners | No changes, manages source configuration |
| **PostgreSQL** | Both listeners | Same overlay_chat_sources table schema |
| **Redis** | Both listeners | Same Streams (chat:raw) and leader locks |

---

## Recommended Project Structure

### New Service Directory

```
services/
├── youtube-listener/                # EXISTING (Official API)
│   ├── cmd/main.go
│   ├── oauth/                       # OAuth token management
│   ├── polling/                     # Official API polling
│   ├── quota/                       # Quota tracking
│   └── ...
│
├── youtube-innertube-listener/      # NEW (InnerTube API)
│   ├── cmd/
│   │   └── main.go                  # Entry point (logger, Redis, HTTP server)
│   ├── innertube/
│   │   ├── client.go                # HTTP client with browser headers
│   │   ├── continuation.go          # Continuation token extraction/refresh
│   │   ├── models.go                # InnerTube API response structs
│   │   └── parser.go                # Parse actions → RawChatMessage
│   ├── polling/
│   │   ├── poller.go                # Main polling loop per stream
│   │   ├── manager.go               # Manages pollers (start/stop)
│   │   └── rate_limiter.go          # Prevent aggressive polling
│   ├── discovery/
│   │   ├── stream_finder.go         # Find live streams for channels
│   │   └── initial_data.go          # Extract chat context from page
│   ├── leadership/
│   │   └── client.go                # Source Manager HTTP client
│   ├── publisher/
│   │   └── redis.go                 # Publish to chat:raw stream
│   ├── handlers/
│   │   └── health.go                # /health/live, /health/ready
│   ├── models/
│   │   └── message.go               # RawChatMessage (same as official)
│   ├── go.mod
│   ├── go.sum
│   ├── Dockerfile                   # Multi-stage build (same pattern)
│   └── README.md
│
└── message-processor/               # UNCHANGED (works with both)
    └── ...
```

### Structure Rationale

- **innertube/**: Core InnerTube API integration (client, continuation, parsing)
- **polling/**: Same responsibility as official listener's polling module
- **discovery/**: Replaces OAuth-based stream discovery with page scraping
- **leadership/**: Identical pattern to official listener (reuse interface if extracted to shared/)
- **publisher/**: Identical Redis Streams publishing logic (reuse if extracted to shared/)
- **handlers/**: Same health check endpoints for Kubernetes
- **models/**: RawChatMessage schema MUST match official listener exactly

---

## Architectural Patterns

### Pattern 1: Drop-In Replacement

**What:** Two services with identical external contracts (input: database sources, output: Redis Stream messages)

**When to use:** When migrating from risky/expensive service to alternative with same functionality

**Trade-offs:**
- **Pros:** Zero architecture changes, easy rollback, run both in parallel for validation
- **Cons:** Code duplication (polling logic, message parsing, health checks)

**Example:**
```go
// BOTH services publish identical messages to Redis Streams
type RawChatMessage struct {
    MessageID   string            `json:"message_id"`
    Platform    string            `json:"platform"` // Always "youtube"
    ChannelID   string            `json:"channel_id"`
    StreamID    string            `json:"stream_id"`
    UserID      string            `json:"user_id"`
    Username    string            `json:"username"`
    Text        string            `json:"text"`
    Timestamp   time.Time         `json:"timestamp"`
    Tags        map[string]string `json:"tags"`
}

// Official Listener: Parses from YouTube Data API response
func (p *OfficialPoller) parseMessage(apiMsg *youtube.LiveChatMessage) RawChatMessage {
    return RawChatMessage{
        Platform:  "youtube",
        ChannelID: apiMsg.AuthorDetails.ChannelID,
        // ... extract from official API fields
    }
}

// InnerTube Listener: Parses from InnerTube API response
func (p *InnerTubePoller) parseMessage(action *AddChatItemAction) RawChatMessage {
    return RawChatMessage{
        Platform:  "youtube",
        ChannelID: action.Item.LiveChatTextMessageRenderer.AuthorExternalChannelID,
        // ... extract from InnerTube API fields
    }
}

// Message Processor: Receives identical messages from EITHER listener
func (mp *MessageProcessor) consumeRawMessages(ctx context.Context) {
    streams, _ := mp.redis.XReadGroup(ctx, &redis.XReadGroupArgs{
        Group:    "message-processors",
        Consumer: "consumer-1",
        Streams:  []string{"chat:raw", ">"},
    }).Result()
    // Cannot distinguish which listener published the message
}
```

### Pattern 2: Continuation-Based Polling

**What:** Use continuation tokens from InnerTube API to paginate through live chat messages

**When to use:** YouTube InnerTube API requires continuation tokens for chat message pagination

**Trade-offs:**
- **Pros:** Stateless polling (continuation token contains all state), lower overhead than OAuth
- **Cons:** Continuation tokens can expire, initial token acquisition requires page scraping

**Example:**
```go
// innertube/continuation.go
type ContinuationManager struct {
    httpClient *http.Client
}

// Extract initial continuation token from live stream page
func (cm *ContinuationManager) GetInitialContinuation(videoID string) (string, error) {
    url := fmt.Sprintf("https://www.youtube.com/live_chat?v=%s", videoID)
    resp, err := cm.httpClient.Get(url)
    // Parse HTML → extract ytInitialData → liveChatRenderer.continuations[0].token
    return extractContinuationFromHTML(resp.Body)
}

// Poll with continuation token
func (cm *ContinuationManager) PollChat(continuation string) (*ChatResponse, string, error) {
    payload := map[string]interface{}{
        "context": cm.buildContext(),
        "continuation": continuation,
    }

    resp, err := cm.httpClient.Post(
        "https://www.youtube.com/youtubei/v1/live_chat/get_live_chat",
        "application/json",
        jsonPayload(payload),
    )

    chatResp := parseResponse(resp.Body)
    nextContinuation := extractNextContinuation(chatResp)
    return chatResp, nextContinuation, nil
}
```

### Pattern 3: Leader Election (Reused from Official Listener)

**What:** Use Source Manager to coordinate which pod polls which stream (prevent duplicate API calls)

**When to use:** Multiple replicas of youtube-innertube-listener running (HPA scales 1-5 pods)

**Trade-offs:**
- **Pros:** Same pattern as official listener, proven in production, prevents duplicate polling
- **Cons:** Adds HTTP overhead for claim/renew, requires Source Manager availability

**Example:**
```go
// leadership/client.go (IDENTICAL to official listener)
type SourceManagerClient struct {
    baseURL    string
    secret     string
    httpClient *http.Client
}

func (c *SourceManagerClient) ClaimLeadership(streamID, consumerID string, ttl int) (bool, error) {
    payload := map[string]interface{}{
        "stream_id":   streamID,
        "consumer_id": consumerID,
        "ttl_seconds": ttl,
    }

    req, _ := http.NewRequest("POST", c.baseURL+"/leadership/claim", jsonPayload(payload))
    req.Header.Set("Authorization", "Bearer "+c.secret)

    resp, err := c.httpClient.Do(req)
    return resp.StatusCode == 200, err
}

// polling/poller.go (uses leadership client)
func (p *Poller) Start(ctx context.Context, streamID string) {
    // Claim leadership before starting
    claimed, _ := p.leaderClient.ClaimLeadership(streamID, p.podName, 60)
    if !claimed {
        return // Another pod is leader, enter standby
    }

    // Renew leadership every 30s
    go p.renewLeadership(ctx, streamID)

    // Poll chat messages
    continuation := p.getContinuation(streamID)
    for {
        messages, nextContinuation, _ := p.innertubeClient.PollChat(continuation)
        p.publishMessages(messages)
        continuation = nextContinuation
        time.Sleep(p.pollingInterval)
    }
}
```

---

## Data Flow

### InnerTube Listener Message Flow

```
1. Overlay Manager: User adds YouTube source
       ↓
   PostgreSQL: INSERT INTO overlay_chat_sources (platform=youtube, channel_id=UCxxxxx)
       ↓
   PostgreSQL NOTIFY: source_changes channel
       ↓
2. YouTube InnerTube Listener: Receives notification (via PostgreSQL LISTEN)
       ↓
   Discovery: Find live stream for channel UCxxxxx
       ↓
   InnerTube Client: GET https://www.youtube.com/channel/UCxxxxx/live
       ↓
   Parser: Extract video ID from page
       ↓
3. Leadership Client: Claim leadership for stream (video ID)
       ↓
   Source Manager: SET leader:stream:{video_id} = pod-abc123 EX 60
       ↓ (if claimed)
4. Continuation Manager: Get initial continuation token
       ↓
   InnerTube Client: GET https://www.youtube.com/live_chat?v={video_id}
       ↓
   Parser: Extract ytInitialData → continuations[0].token
       ↓
5. Poller: Start polling loop
       ↓
   InnerTube Client: POST /youtubei/v1/live_chat/get_live_chat
       ↓ (every 1-2 seconds)
   Response: { actions: [addChatItemAction, ...], continuations: [...] }
       ↓
6. Parser: Extract messages from actions
       ↓
   RawChatMessage: {platform: "youtube", channel_id: "UCxxxxx", ...}
       ↓
7. Redis Publisher: XADD chat:raw {message JSON}
       ↓
   ════════════════════════════════════════════════════════════
   IDENTICAL TO OFFICIAL LISTENER FROM THIS POINT FORWARD
   ════════════════════════════════════════════════════════════
       ↓
8. Message Processor: XREADGROUP chat:raw
       ↓
   Normalize → Enrich (emotes) → Route (overlay mapping)
       ↓
9. Redis Pub/Sub: PUBLISH overlay:{overlay_id} {enriched message}
       ↓
10. API Gateway: SUBSCRIBE overlay:{overlay_id}
       ↓
    WebSocket: Push to frontend overlays
```

### State Management

**Stream State (Redis):**
```
leader:stream:{video_id}              → "youtube-innertube-pod-abc123" (TTL: 60s)
continuation:{video_id}                → "continuation_token_xyz" (TTL: 300s, optional cache)
rate_limit:global                      → Request count in sliding window
```

**Persistent State (PostgreSQL):**
```
overlay_chat_sources
├── platform: "youtube"
├── channel_id: "UCxxxxx"
├── config: {"polling_interval_ms": 1500}  # InnerTube-specific config
└── is_active: true
```

**No Quota State (vs Official Listener):**
- InnerTube listener does NOT need youtube_quota_usage table
- InnerTube listener does NOT need reserve-confirm-rollback pattern
- InnerTube listener does NOT track API quota (no official quota applies)

---

## Integration Points

### Identical to Official Listener

| Integration | Pattern | Notes |
|-------------|---------|-------|
| **Redis Streams** | XADD chat:raw | Identical message schema (RawChatMessage) |
| **Source Manager** | HTTP API (claim/renew/release) | Same leadership endpoints, stream ID format |
| **PostgreSQL** | Read overlay_chat_sources | Same schema, platform="youtube" |
| **Message Processor** | Consumer of chat:raw | Cannot distinguish InnerTube vs official |
| **Health Checks** | /health/live, /health/ready | Same endpoints for Kubernetes |
| **Metrics** | /metrics (Prometheus) | Same metrics format (counter, histogram, gauge) |

### Different from Official Listener

| Aspect | Official Listener | InnerTube Listener |
|--------|-------------------|---------------------|
| **Authentication** | OAuth 2.0 (per user) | None (uses browser context) |
| **API Endpoint** | YouTube Data API v3 | InnerTube private API |
| **Quota Tracking** | PostgreSQL youtube_quota_usage | Not needed (no quota limits) |
| **Initial Discovery** | search.list API call (100 units) | Page scraping (0 quota) |
| **Polling Mechanism** | liveChatMessages.list (5 units) | Continuation token (0 quota) |
| **Rate Limiting** | API quota state machine | IP-based rate limiting (avoid blocks) |
| **Config Schema** | YOUTUBE_CLIENT_ID, YOUTUBE_CLIENT_SECRET | INNERTUBE_POLLING_INTERVAL_MS |

---

## Deployment Strategy

### Kubernetes Deployment: Mutual Exclusivity

**Self-hosters deploy ONE of these:**

#### Option A: Official API Listener (deployments/k8s/youtube-listener/deployment.yaml)
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: youtube-listener
  namespace: allchat
spec:
  replicas: 3
  selector:
    matchLabels:
      app: youtube-listener
      variant: official  # NEW label
  template:
    metadata:
      labels:
        app: youtube-listener
        variant: official
    spec:
      containers:
      - name: youtube-listener
        image: allchat/youtube-listener:v1.2.0
        env:
        - name: YOUTUBE_CLIENT_ID
          valueFrom:
            secretKeyRef:
              name: youtube-oauth
              key: client_id
        - name: YOUTUBE_CLIENT_SECRET
          valueFrom:
            secretKeyRef:
              name: youtube-oauth
              key: client_secret
        # ... rest of config
```

#### Option B: InnerTube Listener (deployments/k8s/youtube-innertube-listener/deployment.yaml)
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: youtube-innertube-listener
  namespace: allchat
spec:
  replicas: 3
  selector:
    matchLabels:
      app: youtube-listener          # SAME app label
      variant: innertube              # Different variant
  template:
    metadata:
      labels:
        app: youtube-listener
        variant: innertube
    spec:
      containers:
      - name: youtube-innertube-listener
        image: allchat/youtube-innertube-listener:v1.0.0
        env:
        - name: INNERTUBE_POLLING_INTERVAL_MS
          value: "1500"
        - name: INNERTUBE_USER_AGENT
          value: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)..."
        # NO OAuth secrets needed
        # ... rest of config
```

**Deployment Instructions for Self-Hosters:**
```bash
# Choose ONE deployment method:

# Option A: Official API (requires OAuth setup)
kubectl apply -f deployments/k8s/youtube-listener/

# Option B: InnerTube API (no OAuth, quota-free)
kubectl apply -f deployments/k8s/youtube-innertube-listener/

# DO NOT deploy both simultaneously (will cause duplicate messages)
```

### Docker Image Build

**Separate Images:**
```dockerfile
# services/youtube-innertube-listener/Dockerfile
FROM golang:1.25-alpine AS builder

WORKDIR /build
COPY shared /build/shared
COPY services/youtube-innertube-listener /build/services/youtube-innertube-listener

WORKDIR /build/services/youtube-innertube-listener
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o youtube-innertube-listener ./cmd

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /build/services/youtube-innertube-listener/youtube-innertube-listener .
EXPOSE 8089  # Different port from official (8086)
CMD ["./youtube-innertube-listener"]
```

**Build Commands:**
```bash
# Build official listener
docker build -t allchat/youtube-listener:v1.2.0 -f services/youtube-listener/Dockerfile .

# Build InnerTube listener
docker build -t allchat/youtube-innertube-listener:v1.0.0 -f services/youtube-innertube-listener/Dockerfile .
```

### Load Balancing Considerations

**No Special Load Balancing Needed:**
- InnerTube listener uses same leader election pattern as official listener
- Source Manager already handles load distribution across replicas
- HPA scales based on CPU/memory (same as all other listeners)

**Leader Election Prevents Duplicate Polling:**
```
┌────────────────────────────────────────────────────────┐
│  InnerTube Listener Replicas (HPA: 1-5 pods)          │
├────────────────────────────────────────────────────────┤
│                                                         │
│  Pod 1: Leader for stream A, stream B                  │
│         → Polls InnerTube API for A and B              │
│                                                         │
│  Pod 2: Leader for stream C                            │
│         → Polls InnerTube API for C                    │
│                                                         │
│  Pod 3: Standby (no leadership)                        │
│         → Monitors for leadership opportunities        │
│                                                         │
│  (If Pod 1 crashes, Pod 3 claims leadership for A+B)   │
└────────────────────────────────────────────────────────┘
                        ↓
        Source Manager (Redis-based locks)
                        ↓
          leader:stream:video_abc → "pod-1"
          leader:stream:video_xyz → "pod-1"
          leader:stream:video_123 → "pod-2"
```

---

## Scaling Considerations

### Per-Replica Capacity

| Metric | Official Listener | InnerTube Listener |
|--------|-------------------|---------------------|
| **Streams/Pod** | ~50 concurrent streams | ~50-100 concurrent streams |
| **CPU** | 200-500m (API parsing overhead) | 100-300m (simpler parsing) |
| **Memory** | 256Mi-512Mi (OAuth tokens, quota state) | 128Mi-256Mi (no quota state) |
| **Bottleneck** | API quota (1.009M units/day) | Rate limiting (avoid IP blocks) |

### Scaling Priorities

1. **First bottleneck: Rate limiting (InnerTube)**
   - **Symptom:** Polling fails with 429 Too Many Requests or IP blocks
   - **Fix:** Increase polling interval from 1000ms to 1500-2000ms
   - **Tuning:** Configure INNERTUBE_POLLING_INTERVAL_MS per deployment

2. **Second bottleneck: CPU (message parsing)**
   - **Symptom:** CPU >80% on listener pods
   - **Fix:** Scale replicas (HPA increases from 3 → 5 → 10 pods)
   - **Note:** Leader election distributes streams across replicas automatically

3. **Third bottleneck: Redis Streams throughput**
   - **Symptom:** chat:raw stream lag increases
   - **Fix:** Scale Message Processor replicas (consumer group shares load)

---

## Anti-Patterns

### Anti-Pattern 1: Combining Official and InnerTube in Single Service

**What people might do:** Add InnerTube client to existing youtube-listener with a config flag

**Why it's wrong:**
- Larger binary (both OAuth and InnerTube dependencies)
- More complex failure modes (OAuth token refresh can break InnerTube polling)
- Harder to test (need to mock both APIs)
- Violates single responsibility principle

**Do this instead:** Separate services that share common interfaces (leadership, publisher)

### Anti-Pattern 2: Running Both Listeners Simultaneously

**What people might do:** Deploy both official and InnerTube listeners at same time

**Why it's wrong:**
- Duplicate messages published to Redis Streams (same platform=youtube)
- Message Processor sees 2x messages for same chat
- Users see duplicate messages in overlays
- Wastes resources (both listeners polling same streams)

**Do this instead:** Deploy ONE listener per environment, use feature flag at deployment level

### Anti-Pattern 3: Aggressive Polling to "Reduce Latency"

**What people might do:** Set INNERTUBE_POLLING_INTERVAL_MS=100 (10 polls/second)

**Why it's wrong:**
- YouTube will rate limit or block IP address
- InnerTube API is not designed for sub-second polling
- No latency benefit (chat messages batched by YouTube)
- Wastes CPU and network bandwidth

**Do this instead:** Use 1000-2000ms polling interval (same latency as official API polling)

### Anti-Pattern 4: Ignoring Continuation Token Expiry

**What people might do:** Cache continuation token indefinitely, reuse across restarts

**Why it's wrong:**
- Continuation tokens expire after 5-10 minutes of inactivity
- Stale tokens return 400 Bad Request (requires re-fetching initial token)
- Stream state changes (stream ends/restarts) invalidate tokens

**Do this instead:** Always re-fetch initial continuation on stream start, handle 400 errors by re-initializing

---

## Suggested Build Order

### Phase 1: Core InnerTube Client (Foundation)
1. **innertube/client.go**: HTTP client with browser headers
2. **innertube/models.go**: Response structs (actions, continuations)
3. **innertube/continuation.go**: Extract initial continuation from page
4. **innertube/parser.go**: Parse addChatItemAction → RawChatMessage
5. **Tests**: Mock InnerTube API responses, verify parsing

**Milestone:** Can fetch live chat messages for a hardcoded video ID

### Phase 2: Polling & Publishing (Integration)
6. **polling/poller.go**: Main polling loop (continuation → poll → parse → repeat)
7. **publisher/redis.go**: Publish RawChatMessage to chat:raw stream
8. **polling/rate_limiter.go**: Sliding window rate limiting
9. **Tests**: Verify Redis Stream publishing, rate limiting

**Milestone:** Publishes messages to Redis Streams, Message Processor consumes them

### Phase 3: Leadership & Discovery (Scalability)
10. **leadership/client.go**: Source Manager HTTP client (claim/renew/release)
11. **discovery/stream_finder.go**: Find live streams for channels
12. **polling/manager.go**: Start/stop pollers based on active sources
13. **Tests**: Leader election, stream discovery

**Milestone:** Multiple replicas coordinate via Source Manager, no duplicate polling

### Phase 4: Production Readiness (Observability)
14. **handlers/health.go**: /health/live, /health/ready endpoints
15. **cmd/main.go**: Service initialization, graceful shutdown
16. **Dockerfile**: Multi-stage build, optimized image
17. **deployments/k8s/youtube-innertube-listener/**: Kubernetes manifests
18. **README.md**: Documentation (setup, deployment, troubleshooting)

**Milestone:** Production deployment, health checks pass, metrics exported

### Phase 5: Validation & Migration (Safety)
19. **Compare outputs**: Run both listeners in parallel, compare Redis Stream messages
20. **Load testing**: Verify rate limiting prevents IP blocks
21. **Failover testing**: Kill leader pod, verify standby takes over
22. **Documentation**: Self-hoster migration guide (official → InnerTube)

**Milestone:** Proven drop-in replacement, ready for self-hosters

---

## New vs Modified Components

### New Components (youtube-innertube-listener service)

| Component | Purpose | Files |
|-----------|---------|-------|
| **InnerTube Client** | HTTP client for private API | innertube/client.go, innertube/models.go |
| **Continuation Manager** | Token extraction and refresh | innertube/continuation.go |
| **InnerTube Parser** | Parse actions to RawChatMessage | innertube/parser.go |
| **Rate Limiter** | Prevent IP blocks | polling/rate_limiter.go |
| **Stream Finder** | Discover live streams | discovery/stream_finder.go |
| **Poller** | Main polling loop | polling/poller.go |
| **Poller Manager** | Start/stop pollers | polling/manager.go |
| **Leadership Client** | Source Manager integration | leadership/client.go |
| **Redis Publisher** | Publish to chat:raw | publisher/redis.go |
| **Health Handlers** | Kubernetes probes | handlers/health.go |
| **Main** | Service entry point | cmd/main.go |
| **Dockerfile** | Container image | Dockerfile |
| **K8s Manifests** | Deployment specs | deployments/k8s/youtube-innertube-listener/ |

### Modified Components (Minimal Changes)

| Component | Change | Reason |
|-----------|--------|--------|
| **None** | No changes to existing services | Drop-in replacement design |

### Shared Components (Could Extract to shared/)

| Component | Current Location | Shared Usage |
|-----------|------------------|--------------|
| **Leadership Client** | youtube-listener/leadership/ | Both listeners use Source Manager |
| **Redis Publisher** | youtube-listener/publisher/ | All listeners publish to chat:raw |
| **RawChatMessage Model** | youtube-listener/models/ | All listeners use same schema |

**Recommendation for v2.0:** Extract leadership and publisher to shared/ module to avoid duplication.

---

## Deployment Workflow Comparison

### Official Listener Deployment

```bash
# 1. Create OAuth credentials in Google Cloud Console
# 2. Create Kubernetes secret
kubectl create secret generic youtube-oauth \
  --from-literal=client_id=xxx.apps.googleusercontent.com \
  --from-literal=client_secret=GOCSPX-xxx \
  -n allchat

# 3. Deploy official listener
kubectl apply -f deployments/k8s/youtube-listener/

# 4. Users authenticate via OAuth flow
# 5. Monitor quota usage
kubectl exec -n allchat youtube-listener-abc123 -- \
  wget -qO- http://localhost:8086/quota/status
```

### InnerTube Listener Deployment

```bash
# 1. NO OAuth setup needed (skip Google Cloud Console)

# 2. Deploy InnerTube listener
kubectl apply -f deployments/k8s/youtube-innertube-listener/

# 3. NO user authentication needed (uses browser context)

# 4. Monitor rate limiting (not quota)
kubectl exec -n allchat youtube-innertube-listener-abc123 -- \
  wget -qO- http://localhost:8089/status
```

**Simpler for self-hosters:** No OAuth credentials, no quota tracking, no Google Cloud Console setup.

---

## Sources

**InnerTube API Research:**
- [YouTube.js - JavaScript client for InnerTube](https://github.com/LuanRT/YouTube.js)
- [innertube (Python) - Private InnerTube API client](https://github.com/tombulled/innertube)
- [YTLiveChat (.NET) - InnerTube API for live chat](https://github.com/Agash/YTLiveChat)
- [InnerTube unofficial documentation](https://github.com/davidzeng0/innertube)

**Quota Comparison:**
- [YouTube API Quota: 10,000 Units/Day Breakdown](https://www.contentstats.io/blog/youtube-api-quota-tracking)
- [Is the YouTube API Free? Costs, Limits](https://www.getphyllo.com/post/is-the-youtube-api-free-costs-limits-iv)

**All-Chat Architecture:**
- services/youtube-listener/README.md (official listener implementation)
- docs/architecture/01-DATA-FLOW.md (message flow architecture)
- services/message-processor/README.md (consumer of chat:raw)
- services/source-manager/README.md (leader election coordination)
- docs/architecture/02-DEPLOYMENT.md (Kubernetes deployment patterns)

---

*Architecture research for: InnerTube YouTube Listener Integration*
*Researched: 2026-02-21*
*Confidence: HIGH (verified existing architecture, WebSearch for InnerTube patterns)*
