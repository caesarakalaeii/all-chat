# Phase 10: Production Minimum - Research

**Researched:** 2026-02-21
**Domain:** YouTube InnerTube production lifecycle, dynamic stream management, source-manager integration, advanced event parsing
**Confidence:** HIGH

## Summary

Phase 10 transforms the YouTube InnerTube listener from a proof-of-concept (Phase 9) into a production-ready service with dynamic stream management and complete platform parity. This phase implements **drop-in replacement behavior** matching the official youtube-listener's integration with source-manager, lifecycle handling, and event parsing. The core challenge is implementing channel→video ID discovery without quota constraints while maintaining identical downstream behavior for zero-impact deployment.

The research reveals three critical areas: (1) **Source-Manager Integration** - InnerTube listener must use the same Redis-based leader election and active source registry pattern as the official listener, (2) **Stream Discovery** - channel→video ID resolution can use either HTML parsing or InnerTube browse endpoint, both quota-free approaches, and (3) **Advanced Event Parsing** - Super Chat, Super Sticker, memberships, milestones, and tickers require extracting additional metadata from InnerTube's rich response structure.

**Primary recommendation:** Match official youtube-listener's source-manager integration pattern exactly (no HTTP API), implement persistent channel→video mapping in Redis for durability, use exponential backoff with 1s→60s range for reconnection, and parse all event types (Super Chat, Super Sticker, memberships, milestones, tickers) in this phase for complete feature parity.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

#### Architectural Principle
- **CRITICAL: Match official YouTube API listener behavior in all aspects**
- Same source-manager integration pattern (no new HTTP API)
- Same Redis schema and publishing pattern
- Same lifecycle behaviors
- Same offline detection logic
- Drop-in replacement - zero changes to source-manager or message-processor

#### Stream Discovery
- **Discovery mechanism:** Claude's discretion (HTML parsing vs InnerTube browse endpoint)
- **Premiere filtering:** Check `isLive` metadata flag to distinguish live streams from premieres
- **No stream found:** Poll until stream starts (wait up to 15 minutes before timeout)
- **Timeout duration:** 15 minutes (handle scheduled streams that start soon)

#### Source-Manager Integration
- **Channel→Video discovery:** Async (start background goroutine when source-manager requests monitoring)
- **Discovery failure handling:** Give up and report error to source-manager after 15-minute timeout
- **State persistence:** Persist channel→video ID mapping to Redis (survives restarts, visible to other services)
- **Stream ended behavior:** Automatically discover next stream on that channel (seamless for 24/7 streamers)

#### Event Parsing
- **Implementation priority:** All events equally (Super Chat, Super Sticker, memberships, milestones, tickers) - complete feature in Phase 10
- **Redis format:** Match official listener's RawChatMessage schema and event metadata structure
- **Parse error handling:** Log and skip unparseable events (resilient to schema changes)
- **Testing strategy:** Both unit tests with golden fixtures AND live stream comparison validation

#### Lifecycle and Error Handling
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

### Deferred Ideas (OUT OF SCOPE)
- Deletion event detection (Phase 11/13)
- Advanced metrics and monitoring (Phase 12)
- Batch deletion detection (Phase 13)

</user_constraints>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.25+ | Service runtime | Matches existing listener architecture (twitch-listener, youtube-listener, kick-listener) |
| github.com/redis/go-redis/v9 | v9.x | Redis operations (leader election, state persistence) | Standard Redis client across all services |
| go.uber.org/zap | Latest | Structured logging | Project standard for all services |
| github.com/gin-gonic/gin | Latest | HTTP server (health checks only) | Standard HTTP framework (NOT for stream control API) |
| github.com/google/uuid | Latest | Message ID generation | Used by existing youtube-listener for RawChatMessage.MessageID |
| shared/sourcemanager | Internal | Leader election, active source registry | Critical integration component used by all platform listeners |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/cenkalti/backoff/v4 | v4.x | Exponential backoff | Reconnection on transient errors (network, rate limit) |
| net/http | stdlib | HTTP client for InnerTube | Sufficient for POST requests to InnerTube browse endpoint |
| golang.org/x/net/html | Latest | HTML parsing | If using HTML parsing approach for stream discovery |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Source-manager integration | Custom HTTP API | HTTP API adds new attack surface and diverges from established pattern. Source-manager pattern proven across 4 platforms. |
| HTML parsing for discovery | InnerTube browse endpoint | Browse endpoint more stable but requires reverse engineering. HTML parsing simpler but may break on YouTube UI changes. Both quota-free. |
| Redis persistence | In-memory only | Redis persistence survives restarts and provides cross-service visibility. Memory-only loses state on pod restarts. |

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
│   └── main.go                    # Entry point, source-manager integration
├── innertube/
│   ├── client.go                  # InnerTube HTTP client (Phase 9)
│   ├── parser.go                  # Message parser with event types (Phase 9 + 10)
│   ├── types.go                   # InnerTube types (Phase 9)
│   └── discovery.go               # NEW: Channel→video ID discovery
├── poller/
│   ├── poller.go                  # Polling loop (Phase 9)
│   ├── backoff.go                 # Exponential backoff (Phase 9)
│   └── state.go                   # Stream state (Phase 9)
├── streams/
│   ├── manager.go                 # NEW: Stream manager (source-manager integration)
│   ├── repository.go              # NEW: Redis persistence (channel→video mappings)
│   └── lifecycle.go               # NEW: Offline detection, graceful shutdown
├── publisher/
│   └── redis_publisher.go         # Redis Streams publisher (Phase 9)
├── handlers/
│   └── health.go                  # Health checks (Phase 9)
├── go.mod
├── Dockerfile
└── README.md
```

### Pattern 1: Source-Manager Integration (Leader Election + Active Source Registry)

**What:** Integration with source-manager for distributed leadership coordination and active source tracking, matching official youtube-listener pattern exactly.

**When to use:** Core service initialization - required for production deployment with multiple replicas.

**Example:**
```go
// From services/youtube-listener/streams/manager.go pattern
type Manager struct {
    leader           *sourcemanager.LeadershipCoordinator
    repository       *Repository
    redisClient      *redis.Client

    mu               sync.RWMutex
    activeStreams    map[string]*Stream  // videoID -> stream
    pollers          map[string]*Poller  // videoID -> poller

    connectedOverlays map[string]time.Time // overlay_id -> connection_time
    channelConnectedOverlays map[string]map[string]struct{} // channel_id -> overlay_ids
}

func (m *Manager) Start(ctx context.Context) error {
    // Start PostgreSQL LISTEN for source changes
    go m.listenForSourceChanges(ctx)

    // Periodic sync from source-manager (every 30s)
    go m.periodicSync(ctx)

    return nil
}

// Called by source-manager when overlay connects
func (m *Manager) OnOverlayConnected(overlayID string, sources []Source) {
    for _, source := range sources {
        if source.Platform == "youtube" {
            // Background goroutine: discover video ID from channel ID
            go m.discoverAndStartStream(source.ChannelID, overlayID)
        }
    }
}

// Called by source-manager when overlay disconnects
func (m *Manager) OnOverlayDisconnected(overlayID string) {
    // Check if channel still has other connected overlays
    // If not, stop polling (after debounce delay)
    m.handleDisconnect(overlayID)
}
```

**Source:** Existing pattern from [services/youtube-listener/streams/manager.go](file:///home/moersener/Hobby/all-chat/services/youtube-listener/streams/manager.go) and [services/source-manager/README.md](file:///home/moersener/Hobby/all-chat/services/source-manager/README.md)

### Pattern 2: Channel→Video Discovery with Redis Persistence

**What:** Asynchronously discover live stream video ID from channel ID, persist mapping to Redis for durability and cross-service visibility.

**When to use:** When source-manager notifies about new active channel.

**Example:**
```go
// Two approaches: HTML parsing or InnerTube browse endpoint

// Approach A: HTML Parsing (simpler, may break on YouTube UI changes)
func (d *Discovery) DiscoverVideoIDFromHTML(ctx context.Context, channelID string) (string, error) {
    url := fmt.Sprintf("https://www.youtube.com/channel/%s/live", channelID)
    resp, err := http.Get(url)
    if err != nil {
        return "", fmt.Errorf("fetch channel page: %w", err)
    }
    defer resp.Body.Close()

    // Parse HTML for video ID
    // YouTube embeds video ID in canonical link: <link rel="canonical" href="https://www.youtube.com/watch?v=VIDEO_ID">
    doc, _ := html.Parse(resp.Body)
    videoID := extractVideoIDFromDOM(doc)

    if videoID == "" {
        return "", fmt.Errorf("no live stream found")
    }

    // Check if premiere (not live)
    if isPremiere(videoID) {
        return "", fmt.Errorf("found premiere, not live stream")
    }

    return videoID, nil
}

// Approach B: InnerTube Browse Endpoint (more stable, requires API knowledge)
func (d *Discovery) DiscoverVideoIDFromInnerTube(ctx context.Context, channelID string) (string, error) {
    // POST to https://www.youtube.com/youtubei/v1/browse?key={apiKey}
    payload := map[string]interface{}{
        "browseId": channelID,
        "params":   "EgdzdHJlYW1z8gYECgJ6AA%3D%3D", // "Live" tab filter (base64 encoded)
        "context": map[string]interface{}{
            "client": map[string]interface{}{
                "clientName":    "WEB",
                "clientVersion": "2.20250101.00.00",
            },
        },
    }

    resp, err := d.client.PostJSON(ctx, "browse", payload)
    if err != nil {
        return "", fmt.Errorf("browse request: %w", err)
    }

    // Extract video ID from response (nested in tabs[].tabRenderer.content.richGridRenderer.contents[])
    videoID := extractVideoIDFromBrowseResponse(resp)

    if videoID == "" {
        return "", fmt.Errorf("no live stream found")
    }

    return videoID, nil
}

// Persistence layer
func (r *Repository) SetChannelVideoMapping(ctx context.Context, channelID, videoID string) error {
    key := fmt.Sprintf("innertube:channel_video:%s", channelID)
    return r.redisClient.Set(ctx, key, videoID, 24*time.Hour).Err()
}

func (r *Repository) GetChannelVideoMapping(ctx context.Context, channelID string) (string, error) {
    key := fmt.Sprintf("innertube:channel_video:%s", channelID)
    return r.redisClient.Get(ctx, key).Result()
}
```

**Sources:**
- HTML parsing approach: Common pattern for YouTube scrapers, documented in various community projects
- InnerTube browse endpoint: Observed in [YouTube.js/src/Innertube.ts](https://github.com/LuanRT/YouTube.js/blob/main/src/Innertube.ts) and [innertube Python client](https://github.com/tombulled/innertube)

### Pattern 3: Advanced Event Parsing (Super Chat, Memberships, Milestones, Tickers)

**What:** Parse InnerTube event types beyond regular chat messages, extracting rich metadata for EventData field.

**When to use:** In message parser when detecting non-text-message renderers.

**Example:**
```go
// Super Chat parsing (already started in Phase 9, complete in Phase 10)
func parseSuperChat(renderer *LiveChatPaidMessageRenderer, channelID string) (*RawChatMessage, error) {
    msg := &RawChatMessage{
        MessageID: uuid.New().String(),
        Platform:  "youtube",
        ChannelID: channelID,
        UserID:    renderer.AuthorExternalChannelID,
        Username:  renderer.AuthorName.SimpleText,
        Text:      extractMessageText(renderer.Message),
        Timestamp: parseTimestampUsec(renderer.TimestampUsec),
        EventType: "super_chat",
        EventData: map[string]interface{}{
            "amount":       renderer.PurchaseAmountText.SimpleText,  // "$5.00"
            "color":        extractHeaderBackgroundColor(renderer),   // "#1e88e5"
            "amount_micros": renderer.AmountMicros,                   // 5000000 (for sorting)
        },
        Tags: make(map[string]string),
    }
    return msg, nil
}

// Super Sticker parsing
func parseSuperSticker(renderer *LiveChatPaidStickerRenderer, channelID string) (*RawChatMessage, error) {
    stickerURL := ""
    if len(renderer.Sticker.Thumbnails.Thumbnails) > 0 {
        stickerURL = renderer.Sticker.Thumbnails.Thumbnails[0].URL
    }

    msg := &RawChatMessage{
        MessageID: uuid.New().String(),
        Platform:  "youtube",
        ChannelID: channelID,
        UserID:    renderer.AuthorExternalChannelID,
        Username:  renderer.AuthorName.SimpleText,
        Text:      "", // No text for stickers
        Timestamp: parseTimestampUsec(renderer.TimestampUsec),
        EventType: "super_sticker",
        EventData: map[string]interface{}{
            "amount":       renderer.PurchaseAmountText.SimpleText,
            "sticker_url":  stickerURL,
            "amount_micros": renderer.AmountMicros,
        },
        Tags: make(map[string]string),
    }
    return msg, nil
}

// Membership welcome message
func parseMembershipWelcome(renderer *LiveChatMembershipItemRenderer, channelID string) (*RawChatMessage, error) {
    msg := &RawChatMessage{
        MessageID: uuid.New().String(),
        Platform:  "youtube",
        ChannelID: channelID,
        UserID:    renderer.AuthorExternalChannelID,
        Username:  renderer.AuthorName.SimpleText,
        Text:      extractMessageText(renderer.HeaderSubtext), // "Welcome to membership!"
        Timestamp: parseTimestampUsec(renderer.TimestampUsec),
        EventType: "member_joined",
        EventData: map[string]interface{}{
            "level_name": extractMembershipLevel(renderer), // "Member", "Tier 2", etc.
        },
        Tags: make(map[string]string),
    }
    return msg, nil
}

// Membership milestone (recurring membership anniversary)
func parseMembershipMilestone(renderer *LiveChatMembershipItemRenderer, channelID string) (*RawChatMessage, error) {
    // Extract months from header subtext (e.g., "Member for 6 months")
    months := extractMilestoneMonths(renderer.HeaderSubtext)

    msg := &RawChatMessage{
        MessageID: uuid.New().String(),
        Platform:  "youtube",
        ChannelID: channelID,
        UserID:    renderer.AuthorExternalChannelID,
        Username:  renderer.AuthorName.SimpleText,
        Text:      extractMessageText(renderer.HeaderSubtext),
        Timestamp: parseTimestampUsec(renderer.TimestampUsec),
        EventType: "member_milestone",
        EventData: map[string]interface{}{
            "months":     months,
            "level_name": extractMembershipLevel(renderer),
        },
        Tags: make(map[string]string),
    }
    return msg, nil
}

// Ticker events (pinned/highlighted messages from Super Chats)
func parseTickerEvent(action *AddLiveChatTickerItemAction, channelID string) (*RawChatMessage, error) {
    // Tickers are visual pinned displays at the top of chat
    // They reference Super Chats or Super Stickers
    // Extract the underlying message and mark as "pinned"

    if action.Item.LiveChatPaidMessageRenderer != nil {
        msg, err := parseSuperChat(action.Item.LiveChatPaidMessageRenderer, channelID)
        if err != nil {
            return nil, err
        }

        // Add ticker metadata
        msg.EventData["pinned"] = true
        msg.EventData["ticker_duration_sec"] = action.DurationSec

        return msg, nil
    }

    return nil, fmt.Errorf("ticker item without supported renderer")
}
```

**Source:** InnerTube response structures documented in existing [services/youtube-listener-innertube/innertube/types.go](file:///home/moersener/Hobby/all-chat/services/youtube-listener-innertube/innertube/types.go) and observed in live stream captures

### Pattern 4: Offline Detection and Auto-Resume

**What:** Detect when stream goes offline, stop polling, but automatically discover next stream when channel goes live again.

**When to use:** Continuous operation for 24/7 streamers.

**Example:**
```go
type Poller struct {
    client       *innertube.Client
    channelID    string
    videoID      string
    manager      *Manager

    stopChan     chan struct{}
}

func (p *Poller) pollLoop(ctx context.Context) {
    for {
        select {
        case <-p.stopChan:
            return
        default:
        }

        messages, err := p.client.GetLiveChatReplay(ctx, p.continuation)

        if err != nil {
            // Classify error
            if innertube.IsFatalError(err) {
                // Stream offline or invalid
                p.handleStreamOffline()
                return
            }

            // Transient error - backoff and retry
            p.backoff.Wait(ctx, err)
            continue
        }

        // Reset backoff on success
        p.backoff.Reset()

        // Publish messages
        for _, msg := range messages {
            p.publisher.Publish(ctx, msg)
        }

        // Update continuation
        p.continuation = messages.Continuation
    }
}

func (p *Poller) handleStreamOffline() {
    p.logger.Info("Stream offline detected",
        zap.String("channel_id", p.channelID),
        zap.String("video_id", p.videoID),
    )

    // Clear Redis mapping (force rediscovery)
    p.manager.repository.DeleteChannelVideoMapping(context.Background(), p.channelID)

    // Notify manager to start discovery loop for this channel
    // Manager will poll for new stream every 1-5 minutes with exponential backoff
    p.manager.StartDiscoveryLoop(p.channelID)
}
```

**Source:** Conceptual pattern inspired by official youtube-listener's circuit breaker and backoff store

### Pattern 5: Graceful Shutdown with 25-Second Timeout

**What:** Clean shutdown sequence: stop polling → flush Redis → clear state → notify source-manager → exit. Force exit if cleanup exceeds 25 seconds.

**When to use:** SIGTERM signal handling (Kubernetes pod termination).

**Example:**
```go
func (m *Manager) Shutdown(ctx context.Context) error {
    // Create timeout context (25 seconds as per user constraint)
    shutdownCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
    defer cancel()

    m.logger.Info("Starting graceful shutdown")

    // 1. Stop accepting new streams
    close(m.stopChan)

    // 2. Stop all active pollers
    m.mu.Lock()
    pollers := make([]*Poller, 0, len(m.pollers))
    for _, poller := range m.pollers {
        pollers = append(pollers, poller)
    }
    m.mu.Unlock()

    var wg sync.WaitGroup
    for _, poller := range pollers {
        wg.Add(1)
        go func(p *Poller) {
            defer wg.Done()
            p.Stop()
        }(poller)
    }

    // Wait for pollers to stop (with timeout)
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        m.logger.Info("All pollers stopped")
    case <-shutdownCtx.Done():
        m.logger.Error("Shutdown timeout - force stopping pollers")
        return fmt.Errorf("shutdown timeout")
    }

    // 3. Flush Redis buffers
    if err := m.publisher.Flush(shutdownCtx); err != nil {
        m.logger.Error("Failed to flush Redis buffers", zap.Error(err))
    }

    // 4. Clear state from Redis
    for channelID := range m.channelConnectedOverlays {
        m.repository.DeleteChannelVideoMapping(shutdownCtx, channelID)
    }

    // 5. Release leadership locks (notify source-manager)
    for videoID := range m.activeStreams {
        m.leader.ReleaseLeadership(shutdownCtx, videoID)
    }

    m.logger.Info("Graceful shutdown complete")
    return nil
}

// Force exit if cleanup can't complete in 25s
func main() {
    manager := NewManager(...)

    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

    <-sigChan

    // Attempt graceful shutdown
    if err := manager.Shutdown(context.Background()); err != nil {
        logger.Error("Graceful shutdown failed, forcing exit", zap.Error(err))
        os.Exit(1)
    }

    os.Exit(0)
}
```

**Source:** Standard pattern from existing listeners, timeout constraint from user decision

### Anti-Patterns to Avoid

- **Custom HTTP API for stream control**: Official listener uses source-manager integration, InnerTube listener MUST match
- **In-memory channel→video mapping**: Use Redis for durability and cross-service visibility
- **Synchronous stream discovery**: Discovery MUST be async (background goroutine) to avoid blocking source-manager
- **Ignoring premieres**: Must filter out premieres (check `isLive` flag) to avoid polling non-live streams
- **Immediate discovery retry on failure**: Use 15-minute timeout with exponential backoff (30s → 1m → 2m → 5m)

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| **Leader election** | Custom Redis locking | shared/sourcemanager.LeadershipCoordinator | Battle-tested, handles TTL renewal, prevents split-brain |
| **Exponential backoff** | Manual sleep calculations | github.com/cenkalti/backoff/v4 | Handles jitter, max interval, reset logic |
| **HTML parsing** | String manipulation, regex | golang.org/x/net/html | Robust DOM traversal, handles malformed HTML |
| **Redis connection pooling** | Custom connection manager | github.com/redis/go-redis/v9 built-in pool | Automatic pooling, health checks, reconnection |
| **Graceful shutdown coordination** | Manual channel/waitgroup orchestration | sync.WaitGroup + context.WithTimeout | Standard pattern, timeout enforcement |

**Key insight:** Source-manager integration is the most critical "don't hand-roll" - reusing the established pattern ensures zero-risk deployment and consistent behavior across all platforms.

## Common Pitfalls

### Pitfall 1: Synchronous Stream Discovery Blocking Source-Manager

**What goes wrong:** Stream discovery (channel→video ID) takes 1-5 seconds. If done synchronously when source-manager notifies about new overlay connection, it blocks the notification handler and delays other channels.

**Why it happens:** Discovery requires HTTP request to YouTube (HTML parsing or browse endpoint) which can be slow or fail, causing cascading delays.

**How to avoid:**
- Start discovery in background goroutine immediately when notified
- Return success to source-manager before discovery completes
- Manager tracks "discovering" state separate from "polling" state
- 15-minute timeout prevents indefinite discovery loops

**Warning signs:** Slow overlay connection times, source-manager timeout errors, multiple channels failing to start simultaneously. Monitor discovery duration metrics.

### Pitfall 2: Premiere False Positives (Starting Pollers for Non-Live Streams)

**What goes wrong:** YouTube channels often have scheduled "premiere" events that appear as "live" tabs but aren't actually live streams. Starting a poller for a premiere wastes resources and fails immediately.

**Why it happens:** Both live streams and premieres show up on channel live pages. Only the `isLive` metadata flag distinguishes them.

**How to avoid:**
- When discovering video ID, always check `isLive` field in metadata
- HTML parsing: Extract from `<meta property="og:video:type" content="live">` or similar
- InnerTube browse: Check `videoDetails.isLiveContent` and `videoDetails.isLive` both true
- Reject video IDs where `isLive=false` and retry discovery after backoff

**Warning signs:** Immediate "stream ended" errors after starting poller, recurring discovery failures for same channel. Add metric tracking premiere rejections.

### Pitfall 3: Not Persisting Channel→Video Mapping (State Loss on Restart)

**What goes wrong:** Service restart loses all channel→video mappings. On restart, must rediscover all active streams (expensive, slow). 24/7 streamers experience polling interruption.

**Why it happens:** In-memory maps don't survive pod restarts. Kubernetes HPA scale-down/up cycles cause frequent restarts.

**How to avoid:**
- Persist channel→video mappings to Redis immediately after discovery
- On service startup, load existing mappings from Redis
- Set 24-hour TTL on mappings (auto-expire if channel offline)
- Cross-service visibility: Other pods can see mappings (prevents duplicate discovery)

**Warning signs:** Slow startup times (rediscovering many channels), duplicate discovery across pods, temporary polling gaps on restarts. Monitor discovery-on-startup count.

### Pitfall 4: Missing Graceful Shutdown Causing Message Loss

**What goes wrong:** SIGTERM received (pod termination), service exits immediately, in-flight messages never reach Redis, Redis buffers not flushed.

**Why it happens:** Default SIGTERM handling calls os.Exit() without cleanup. Kubernetes waits 30 seconds before SIGKILL, but service doesn't use that grace period.

**How to avoid:**
- Implement signal handler for SIGTERM/SIGINT
- Stop accepting new connections/streams immediately
- Allow in-flight pollers to finish current poll cycle
- Flush Redis publisher buffer (ensure XADD completes)
- Release leadership locks (notify source-manager)
- Complete all cleanup within 25 seconds (leave 5-second buffer before Kubernetes 30s timeout)

**Warning signs:** Message gaps during rolling updates, Redis "connection reset" errors in logs, leadership locks not released. Monitor shutdown duration metrics.

### Pitfall 5: Failing to Auto-Resume After Stream End

**What goes wrong:** Stream ends (24/7 streamer restarts for technical reasons), poller stops, but never starts again even when channel goes live again.

**Why it happens:** Offline detection stops poller but doesn't start discovery loop. Channel remains in "offline" state indefinitely.

**How to avoid:**
- When stream offline detected, immediately start background discovery loop for that channel
- Discovery loop polls for new stream with exponential backoff (1m → 2m → 5m → 10m)
- Continue discovery as long as overlay remains connected to that channel
- Store discovery backoff state in Redis (survives restarts)

**Warning signs:** Channels stuck in "offline" state after temporary stream interruptions, manual restarts needed to resume polling. Monitor discovery loop count per channel.

## Code Examples

### Stream Discovery with Premiere Filtering

```go
// HTML Parsing Approach
func (d *Discovery) DiscoverLiveStream(ctx context.Context, channelID string) (string, error) {
    url := fmt.Sprintf("https://www.youtube.com/channel/%s/live", channelID)

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return "", fmt.Errorf("create request: %w", err)
    }

    resp, err := d.httpClient.Do(req)
    if err != nil {
        return "", fmt.Errorf("fetch channel page: %w", err)
    }
    defer resp.Body.Close()

    // Parse HTML
    doc, err := html.Parse(resp.Body)
    if err != nil {
        return "", fmt.Errorf("parse HTML: %w", err)
    }

    // Extract canonical video URL
    videoID := extractCanonicalVideoID(doc)
    if videoID == "" {
        return "", fmt.Errorf("no live stream found")
    }

    // Check if actually live (not premiere)
    isLive := checkIsLiveMeta(doc)
    if !isLive {
        return "", fmt.Errorf("found premiere, not live stream")
    }

    d.logger.Info("Discovered live stream",
        zap.String("channel_id", channelID),
        zap.String("video_id", videoID),
    )

    return videoID, nil
}

func extractCanonicalVideoID(doc *html.Node) string {
    // Find: <link rel="canonical" href="https://www.youtube.com/watch?v=VIDEO_ID">
    var videoID string
    var f func(*html.Node)
    f = func(n *html.Node) {
        if n.Type == html.ElementNode && n.Data == "link" {
            var isCanonical bool
            var href string
            for _, attr := range n.Attr {
                if attr.Key == "rel" && attr.Val == "canonical" {
                    isCanonical = true
                }
                if attr.Key == "href" {
                    href = attr.Val
                }
            }
            if isCanonical && strings.Contains(href, "watch?v=") {
                videoID = strings.TrimPrefix(strings.Split(href, "watch?v=")[1], "")
                return
            }
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            f(c)
        }
    }
    f(doc)
    return videoID
}

func checkIsLiveMeta(doc *html.Node) bool {
    // Find: <meta property="og:video:type" content="live">
    var isLive bool
    var f func(*html.Node)
    f = func(n *html.Node) {
        if n.Type == html.ElementNode && n.Data == "meta" {
            var property, content string
            for _, attr := range n.Attr {
                if attr.Key == "property" {
                    property = attr.Val
                }
                if attr.Key == "content" {
                    content = attr.Val
                }
            }
            if property == "og:video:type" && content == "live" {
                isLive = true
                return
            }
        }
        for c := n.FirstChild; c != nil; c = c.NextSibling {
            f(c)
        }
    }
    f(doc)
    return isLive
}
```

**Source:** Common YouTube scraping pattern, canonical link extraction standard across web scrapers

### Source-Manager Integration with Async Discovery

```go
type Manager struct {
    leader           *sourcemanager.LeadershipCoordinator
    repository       *Repository
    discovery        *Discovery
    redisClient      *redis.Client
    logger           *zap.Logger

    mu               sync.RWMutex
    activeStreams    map[string]*Stream  // videoID -> stream
    pollers          map[string]*Poller  // videoID -> poller
    discovering      map[string]*DiscoveryState // channelID -> discovery state
}

type DiscoveryState struct {
    ChannelID     string
    OverlayID     string
    StartedAt     time.Time
    Attempts      int
    NextAttemptAt time.Time
    CancelFunc    context.CancelFunc
}

func (m *Manager) OnOverlayConnected(overlayID string, sources []Source) {
    for _, source := range sources {
        if source.Platform == "youtube" {
            // Start async discovery (non-blocking)
            m.startAsyncDiscovery(source.ChannelID, overlayID)
        }
    }
}

func (m *Manager) startAsyncDiscovery(channelID, overlayID string) {
    m.mu.Lock()
    defer m.mu.Unlock()

    // Check if already discovering
    if _, exists := m.discovering[channelID]; exists {
        m.logger.Info("Discovery already in progress",
            zap.String("channel_id", channelID),
        )
        return
    }

    // Check Redis cache first
    if videoID, err := m.repository.GetChannelVideoMapping(context.Background(), channelID); err == nil && videoID != "" {
        m.logger.Info("Using cached video ID",
            zap.String("channel_id", channelID),
            zap.String("video_id", videoID),
        )

        // Start poller immediately with cached video ID
        go m.startPoller(channelID, videoID, overlayID)
        return
    }

    // Start background discovery goroutine
    ctx, cancel := context.WithCancel(context.Background())
    state := &DiscoveryState{
        ChannelID:  channelID,
        OverlayID:  overlayID,
        StartedAt:  time.Now(),
        Attempts:   0,
        CancelFunc: cancel,
    }
    m.discovering[channelID] = state

    go m.discoveryLoop(ctx, state)
}

func (m *Manager) discoveryLoop(ctx context.Context, state *DiscoveryState) {
    defer func() {
        m.mu.Lock()
        delete(m.discovering, state.ChannelID)
        m.mu.Unlock()
    }()

    backoff := []time.Duration{30 * time.Second, 1 * time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute}
    maxAttempts := len(backoff)
    timeout := 15 * time.Minute

    for {
        // Check timeout
        if time.Since(state.StartedAt) > timeout {
            m.logger.Error("Discovery timeout",
                zap.String("channel_id", state.ChannelID),
                zap.Duration("elapsed", time.Since(state.StartedAt)),
            )
            return
        }

        // Attempt discovery
        videoID, err := m.discovery.DiscoverLiveStream(ctx, state.ChannelID)
        state.Attempts++

        if err == nil {
            // Success - persist and start poller
            m.repository.SetChannelVideoMapping(ctx, state.ChannelID, videoID)
            m.startPoller(state.ChannelID, videoID, state.OverlayID)

            m.logger.Info("Discovery successful",
                zap.String("channel_id", state.ChannelID),
                zap.String("video_id", videoID),
                zap.Int("attempts", state.Attempts),
            )
            return
        }

        // Failure - check if should retry
        if state.Attempts >= maxAttempts {
            m.logger.Error("Discovery failed after max attempts",
                zap.String("channel_id", state.ChannelID),
                zap.Int("attempts", state.Attempts),
                zap.Error(err),
            )
            return
        }

        // Exponential backoff
        backoffDuration := backoff[state.Attempts-1]
        m.logger.Warn("Discovery failed, retrying",
            zap.String("channel_id", state.ChannelID),
            zap.Int("attempts", state.Attempts),
            zap.Duration("backoff", backoffDuration),
            zap.Error(err),
        )

        select {
        case <-time.After(backoffDuration):
            // Continue to next attempt
        case <-ctx.Done():
            return
        }
    }
}

func (m *Manager) startPoller(channelID, videoID, overlayID string) {
    // Claim leadership for this stream
    claimed, err := m.leader.ClaimLeadership(context.Background(), videoID, 60*time.Second)
    if err != nil || !claimed {
        m.logger.Info("Leadership not claimed, another replica polling",
            zap.String("video_id", videoID),
        )
        return
    }

    // Create and start poller
    poller := NewPoller(videoID, channelID, m.publisher, m.logger)

    m.mu.Lock()
    m.pollers[videoID] = poller
    m.mu.Unlock()

    go poller.Start(context.Background())
}
```

**Source:** Architectural pattern inspired by official youtube-listener's stream detection with backoff store

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| HTTP API for stream control | Source-manager integration | Always (established pattern) | Consistent behavior across all platforms |
| Synchronous discovery | Async background discovery | Phase 10 design decision | Non-blocking, better UX for multi-channel overlays |
| In-memory stream state | Redis-persisted state | Phase 10 requirement | Survives restarts, cross-pod visibility |
| Poll-on-demand only | Auto-resume after stream end | Phase 10 feature | 24/7 streamers work seamlessly |
| Basic chat messages only | All event types (Super Chat, etc.) | Phase 10 scope | Feature parity with official listener |

**Deprecated/outdated:**
- **Quota tracking database**: InnerTube has no quotas - remove youtube_quota_usage table, tracker, all quota logic
- **OAuth token storage**: InnerTube works unauthenticated - remove youtube_oauth_tokens table, refresh logic
- **Adaptive polling slowdown**: No quota pressure - use fixed 1-2s polling interval

## Open Questions

1. **Stream discovery approach: HTML parsing vs InnerTube browse endpoint**
   - What we know: Both are quota-free, both can extract video ID and check isLive flag
   - What's unclear: Which is more stable long-term? HTML may break on UI changes, browse endpoint may change schema
   - Recommendation: Start with HTML parsing (simpler), prepare InnerTube browse as fallback. Monitor parse failures.

2. **Exact event metadata structure for Super Chat color tiers**
   - What we know: InnerTube provides headerBackgroundColor field, official listener may not expose this
   - What's unclear: Should InnerTube listener expose richer metadata (color tiers) or match official listener exactly?
   - Recommendation: Phase 10 extracts all available metadata, Phase 11 contract testing validates against official output

3. **Discovery timeout vs scheduled stream detection**
   - What we know: 15-minute timeout covers "stream starting soon" scenarios
   - What's unclear: Should we distinguish between "channel offline" vs "stream scheduled in 1 hour"?
   - Recommendation: Phase 10 uses simple 15-minute timeout. Phase 12+ can add scheduled stream detection if users request it.

## Sources

### Primary (HIGH confidence)
- [services/youtube-listener/streams/manager.go](file:///home/moersener/Hobby/all-chat/services/youtube-listener/streams/manager.go) - Source-manager integration pattern
- [services/youtube-listener/streams/poller.go](file:///home/moersener/Hobby/all-chat/services/youtube-listener/streams/poller.go) - Polling lifecycle, offline detection
- [services/youtube-listener/models/raw_message.go](file:///home/moersener/Hobby/all-chat/services/youtube-listener/models/raw_message.go) - RawChatMessage schema (EventType, EventData)
- [services/source-manager/README.md](file:///home/moersener/Hobby/all-chat/services/source-manager/README.md) - Leader election, active source registry
- [services/youtube-listener-innertube/innertube/types.go](file:///home/moersener/Hobby/all-chat/services/youtube-listener-innertube/innertube/types.go) - InnerTube event types

### Secondary (MEDIUM confidence)
- [YouTube.js/src/Innertube.ts](https://github.com/LuanRT/YouTube.js/blob/main/src/Innertube.ts) - InnerTube browse endpoint patterns
- [innertube Python client](https://github.com/tombulled/innertube) - Browse endpoint usage examples
- [Phase 9 RESEARCH.md](file:///home/moersener/Hobby/all-chat/.planning/phases/09-core-ingestion-poc/09-RESEARCH.md) - InnerTube client patterns, backoff strategies

### Tertiary (LOW confidence)
- Community YouTube scraper projects - HTML parsing patterns (canonical link, og:video:type meta tags)
- Web search results on InnerTube browse endpoint (requires verification with live testing)

## Metadata

**Confidence breakdown:**
- Source-manager integration: HIGH - Documented in existing codebase, battle-tested across 4 platforms
- Stream discovery: MEDIUM - Two viable approaches (HTML parsing, InnerTube browse), both require testing
- Event parsing: HIGH - InnerTube types already defined in Phase 9, official listener schema documented
- Lifecycle patterns: HIGH - Existing official listener provides proven patterns for offline detection, graceful shutdown
- Redis persistence: HIGH - Standard go-redis/v9 usage, matches existing patterns

**Research date:** 2026-02-21
**Valid until:** 2026-03-21 (30 days for architectural patterns - relatively stable)
