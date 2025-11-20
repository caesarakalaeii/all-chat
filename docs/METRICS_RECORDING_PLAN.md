# Metrics Recording Implementation Plan

**Goal**: Wire up actual metric recording calls in all service components

**Approach**: Service by service, component by component

---

## 1. Twitch Listener (Port 8085)

### File: `services/twitch-listener/irc/client.go`

**Metrics to Record**:
- Connection status changes
- Connection attempts (success/failure)
- Connection duration tracking
- Messages received by type
- IRC commands sent
- Errors by category

**Recording Points**:

```go
// In Connect() method
- RecordConnectionAttempt("twitch", "twitch-listener", "attempting")
- On success: RecordConnectionAttempt("twitch", "twitch-listener", "success")
- On success: RecordConnection("twitch", "twitch-listener", "irc", true)
- On failure: RecordConnectionAttempt("twitch", "twitch-listener", "failed")
- On failure: RecordConnection("twitch", "twitch-listener", "irc", false)

// In Disconnect() method
- RecordConnection("twitch", "twitch-listener", "irc", false)
- ConnectionDuration.Observe() with time since connection

// In message handler (onPrivateMessage, onUserNotice, etc.)
- RecordMessage("twitch", "twitch-listener", channelID, messageType)

// In error handlers
- RecordError("twitch", "twitch-listener", errorCategory, severity)
```

### File: `services/twitch-listener/publisher/redis.go`

**Metrics to Record**:
- Messages published to Redis (success/failure)
- Message publish latency

**Recording Points**:

```go
// In Publish() method
- start := time.Now()
- On success: RecordPublish("twitch", "twitch-listener", "success")
- On failure: RecordPublish("twitch", "twitch-listener", "failed")
- MessageLatency.WithLabelValues("twitch", "twitch-listener").Observe(time.Since(start).Seconds())
```

### File: `services/twitch-listener/channels/manager.go`

**Metrics to Record**:
- Active sources count
- Source lifecycle events (added, removed)
- Rate limit delays

**Recording Points**:

```go
// After syncing channels
- SetActiveSources("twitch", "twitch-listener", len(activeChannels))

// When adding channel
- RecordSourceEvent("twitch", "twitch-listener", "added")

// When removing channel
- RecordSourceEvent("twitch", "twitch-listener", "removed")

// When rate limited (join throttling)
- RateLimitHits.WithLabelValues("twitch", "twitch-listener", "join_rate").Inc()
```

---

## 2. YouTube Listener (Port 8086)

### File: `services/youtube-listener/streams/poller.go`

**Metrics to Record**:
- API calls to YouTube (list videos, live chat messages)
- API call duration
- API call results (success, error types)
- Messages received
- Poll interval tracking

**Recording Points**:

```go
// Before API call
- start := time.Now()

// After Videos.List() call
- RecordAPICall("youtube", "youtube-listener", "list_videos", "success", "")
- On error: RecordAPICall("youtube", "youtube-listener", "list_videos", "error", errorType)
- APICallDuration.WithLabelValues("youtube", "youtube-listener", "list_videos").Observe(time.Since(start).Seconds())

// After LiveChatMessages.List() call
- RecordAPICall("youtube", "youtube-listener", "list_messages", "success", "")
- On error: RecordAPICall("youtube", "youtube-listener", "list_messages", "error", errorType)
- APICallDuration.WithLabelValues("youtube", "youtube-listener", "list_messages").Observe(time.Since(start).Seconds())

// For each message received
- RecordMessage("youtube", "youtube-listener", videoID, "chat")
```

### File: `services/youtube-listener/quota/tracker.go`

**Metrics to Record**:
- Quota usage and remaining
- Quota percentage
- Rate limit hits

**Recording Points**:

```go
// After tracking quota usage
- SetQuotaRemaining("youtube", "youtube-listener", "daily", "10000", float64(remaining))
- SetQuotaUsagePercent("youtube", "youtube-listener", "daily", percentUsed)

// When quota exceeded
- RateLimitHits.WithLabelValues("youtube", "youtube-listener", "api_quota").Inc()
```

### File: `services/youtube-listener/publisher/redis.go`

**Metrics to Record**:
- Messages published (success/failure)
- Publish latency

**Recording Points**:

```go
// In Publish() method
- start := time.Now()
- On success: RecordPublish("youtube", "youtube-listener", "success")
- On failure: RecordPublish("youtube", "youtube-listener", "failed")
- MessageLatency.WithLabelValues("youtube", "youtube-listener").Observe(time.Since(start).Seconds())
```

### File: `services/youtube-listener/streams/manager.go`

**Metrics to Record**:
- Active sources (streams) count
- Stream lifecycle events

**Recording Points**:

```go
// After syncing streams
- SetActiveSources("youtube", "youtube-listener", len(activeStreams))

// When stream starts
- RecordSourceEvent("youtube", "youtube-listener", "discovered")

// When stream ends
- RecordSourceEvent("youtube", "youtube-listener", "lost")
```

---

## 3. Kick Listener (Port 8089)

### File: `services/kick-listener/websocket/client.go`

**Metrics to Record**:
- WebSocket connection status
- Connection attempts
- Messages received from Pusher
- Connection duration

**Recording Points**:

```go
// In Connect() method
- RecordConnectionAttempt("kick", "kick-listener", "attempting")
- On success: RecordConnectionAttempt("kick", "kick-listener", "success")
- On success: RecordConnection("kick", "kick-listener", "websocket", true)
- On failure: RecordConnectionAttempt("kick", "kick-listener", "failed")

// In Disconnect() method
- RecordConnection("kick", "kick-listener", "websocket", false)
- ConnectionDuration.Observe() with time since connection

// On message received
- RecordMessage("kick", "kick-listener", channelID, "chat")

// On Pusher events
- For subscription_succeeded, subscription_error, etc.
```

### File: `services/kick-listener/publisher/redis.go`

**Metrics to Record**:
- Messages published (success/failure)
- Publish latency

**Recording Points**:

```go
// In Publish() method
- start := time.Now()
- On success: RecordPublish("kick", "kick-listener", "success")
- On failure: RecordPublish("kick", "kick-listener", "failed")
- MessageLatency.WithLabelValues("kick", "kick-listener").Observe(time.Since(start).Seconds())
```

### File: `services/kick-listener/channels/manager.go`

**Metrics to Record**:
- Active sources count
- Channel subscription events

**Recording Points**:

```go
// After syncing channels
- SetActiveSources("kick", "kick-listener", len(activeChannels))

// When subscribing to channel
- RecordSourceEvent("kick", "kick-listener", "added")

// When unsubscribing
- RecordSourceEvent("kick", "kick-listener", "removed")
```

---

## 4. Message Processor (Port 8087)

### File: `services/message-processor/consumer/streams.go`

**Metrics to Record**:
- Messages consumed from Redis stream
- Stream lag
- Stream errors

**Recording Points**:

```go
// In ConsumeMessages() method
// For each message consumed
- RecordMessageConsumed("message-processor", platform, "message-processors")

// Track stream lag
- if lastMessageTime != nil {
    lagSeconds := time.Since(lastMessageTime).Seconds()
    SetStreamLag("message-processor", "chat:raw", "message-processors", lagSeconds)
  }

// On stream errors
- RecordStreamError("message-processor", errorType)
```

### File: `services/message-processor/normalizer/*.go`

**Metrics to Record**:
- Messages processed through normalization stage
- Stage duration

**Recording Points**:

```go
// In Normalize() method for each normalizer
- start := time.Now()
- On success: RecordMessageProcessed("message-processor", platform, "normalized", "success")
- On failure: RecordMessageProcessed("message-processor", platform, "normalized", "failed")
- StageDuration.WithLabelValues("message-processor", platform, "normalization").Observe(time.Since(start).Seconds())
```

### File: `services/message-processor/enricher/emote_enricher.go`

**Metrics to Record**:
- Emote lookups (by provider)
- Cache hits/misses
- Cache entry counts
- Enrichment duration

**Recording Points**:

```go
// In EnrichMessage() method
- start := time.Now()

// For each emote provider lookup
- On cache hit: RecordEmoteCacheOperation("message-processor", "hit", provider)
- On cache miss: RecordEmoteCacheOperation("message-processor", "miss", provider)
- On API success: RecordEmoteLookup("message-processor", provider, "success")
- On API cached: RecordEmoteLookup("message-processor", provider, "cached")
- On API failure: RecordEmoteLookup("message-processor", provider, "failed")

// Update cache entry count
- SetEmoteCacheEntries("message-processor", provider, cacheSize)

// After enrichment
- emoteCountBucket := getEmoteCountBucket(emoteCount) // "0", "1-5", "6-10", "11+"
- EmoteEnrichmentDuration.WithLabelValues("message-processor", emoteCountBucket).Observe(time.Since(start).Seconds())
```

### File: `services/message-processor/publisher/pubsub.go`

**Metrics to Record**:
- Messages published to overlay channels
- Fanout duration
- Publish results

**Recording Points**:

```go
// In PublishToOverlay() method
- start := time.Now()
- On success: RecordMessagePublished("message-processor", overlayID, platform, "success")
- On failure: RecordMessagePublished("message-processor", overlayID, platform, "failed")
- FanoutDuration.WithLabelValues("message-processor").Observe(time.Since(start).Seconds())
```

### File: `services/message-processor/router/router.go`

**Metrics to Record**:
- Overall processing duration
- Messages routed by platform

**Recording Points**:

```go
// In ProcessMessage() method
- start := time.Now()
- RecordMessageProcessed("message-processor", platform, "consumed", "success")
- After routing: RecordMessageProcessed("message-processor", platform, "routed", "success")
- ProcessingDuration.WithLabelValues("message-processor", platform).Observe(time.Since(start).Seconds())
```

---

## 5. API Gateway (Port 8080)

### File: `services/api-gateway/websocket/hub.go`

**Metrics to Record**:
- Active WebSocket connections
- Connection attempts
- Connection duration
- Messages received from Redis
- Messages sent to clients
- Messages dropped

**Recording Points**:

```go
// When client connects
- RecordWebSocketConnectionAttempt("api-gateway", "success")
- RecordWebSocketConnection("api-gateway", "overlay", 1) // increment

// When client disconnects
- RecordWebSocketConnection("api-gateway", "overlay", -1) // decrement
- ConnectionDuration.WithLabelValues("api-gateway", disconnectReason).Observe(duration.Seconds())

// When message received from Redis pub/sub
- RecordMessageReceived("api-gateway", overlayID, platform)

// When sending message to client
- start := time.Now()
- On success: RecordMessageSent("api-gateway", overlayID, "success")
- On failure: RecordMessageSent("api-gateway", overlayID, "failed")
- MessageDeliveryLatency.WithLabelValues("api-gateway").Observe(time.Since(start).Seconds())

// When dropping message
- RecordMessageDropped("api-gateway", reason) // "client_disconnected", "buffer_full", etc.
```

### File: `services/api-gateway/subscription/manager.go`

**Metrics to Record**:
- Active overlay subscriptions
- Subscription lifecycle events

**Recording Points**:

```go
// When overlay subscription created
- RecordOverlaySubscription("api-gateway", overlayID, 1) // increment
- RecordSubscriptionEvent("api-gateway", "subscribed")

// When overlay subscription removed
- RecordOverlaySubscription("api-gateway", overlayID, -1) // decrement
- RecordSubscriptionEvent("api-gateway", "unsubscribed")
```

### File: `services/api-gateway/middleware/logging.go` (or main router)

**Metrics to Record**:
- HTTP request counts
- HTTP request duration

**Recording Points**:

```go
// In middleware or router
// Before handling request
- start := time.Now()

// After handling request
- RecordHTTPRequest("api-gateway", method, path, strconv.Itoa(statusCode))
- HTTPRequestDuration.WithLabelValues("api-gateway", method, path).Observe(time.Since(start).Seconds())
```

---

## 6. Source Manager (Port 8088)

### File: `services/source-manager/registry/registry.go`

**Metrics to Record**:
- Active sources by platform
- Registry operations

**Recording Points**:

```go
// After syncing sources from database
- For each platform:
    SetActiveSourcesTotal(platform, count)

// On registry operations
- RecordSourceOperation(operation, platform, result)
```

### File: `services/source-manager/election/manager.go`

**Metrics to Record**:
- Leadership status
- Leadership changes
- Leadership duration

**Recording Points**:

```go
// These would need custom Source Manager metrics (not currently in shared package)
// Could add to business metrics or create source_manager.go
```

---

## 7. Auth Service

### File: `services/auth-service/oauth/*.go`

**Metrics to Record**:
- OAuth flow attempts and results
- Token operations
- User operations

**Recording Points**:

```go
// Would need custom Auth metrics in shared/metrics/auth.go
// Following pattern:
- OAuth flows by platform (twitch, youtube, kick, tiktok)
- Token issued/validated/refreshed counts
- User creation/login counts
```

---

## 8. Overlay Manager

### File: `services/overlay-manager/handlers/*.go`

**Metrics to Record**:
- Overlay CRUD operations
- Source operations
- Active overlay counts

**Recording Points**:

```go
// Would need custom Overlay metrics
// Track overlay operations and active overlays
```

---

## 9. Emote Service (Port 8083)

### File: `services/emote-service/cache/cache.go`

**Metrics to Record**:
- Cache operations (same as processor emote enrichment)
- Cache size by provider

**Recording Points**:

```go
// Similar to message processor emote enrichment
- RecordEmoteCacheOperation("emote-service", operation, provider)
- SetEmoteCacheEntries("emote-service", provider, count)
```

### File: `services/emote-service/clients/*.go`

**Metrics to Record**:
- API calls to emote providers (7TV, BTTV, FFZ)
- API call duration
- API call results

**Recording Points**:

```go
// For each provider client
- RecordEmoteLookup("emote-service", provider, result)
```

---

## Implementation Order (Recommended)

### Phase 1: Critical Monitoring (Start Here)
1. **Twitch Listener** - Most established, good reference implementation
2. **Message Processor** - Core pipeline, high value metrics
3. **API Gateway** - WebSocket health critical for UX

### Phase 2: Platform Coverage
4. **YouTube Listener** - Quota tracking is critical
5. **Kick Listener** - Complete listener coverage

### Phase 3: Supporting Services
6. **Source Manager** - Leadership and registry metrics
7. **Emote Service** - Cache performance

### Phase 4: Business Intelligence
8. **Auth Service** - User metrics
9. **Overlay Manager** - Overlay operations

---

## Testing After Each Service

After implementing metrics for each service:

```bash
# Start the service
./service-name

# Check metrics endpoint
curl http://localhost:PORT/metrics | grep service_name

# Verify specific metrics appear and increment
curl http://localhost:PORT/metrics | grep listener_messages_received_total

# Trigger actions and verify counters increase
# e.g., send messages through Twitch, check if counter increments
```

---

## Notes

- All recording calls should be **non-blocking** (they are with Prometheus)
- Add error handling: if metrics recording fails, log but don't crash
- Use appropriate labels to enable filtering in Grafana
- Keep cardinality low (don't use user IDs or message IDs as labels)
- Test metric recording doesn't significantly impact performance

---

**Ready to start with Twitch Listener?** Let me know and I'll begin implementing the metrics recording in each file!
