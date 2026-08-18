# YouTube Listener InnerTube

> **⚠️ Terms of Service Disclosure:** This service uses the InnerTube API, which is an unofficial YouTube API not documented or supported by Google. Use of this API may violate YouTube's Terms of Service. This implementation is intended for personal use and small-scale deployments. For production use at scale, consider YouTube's official Data API with proper quota management.

**Status**: Phase 12 - Production Rollout (v1.2 Milestone)

Drop-in replacement for the official YouTube Listener using the InnerTube API instead of YouTube Data API v3. This service eliminates OAuth complexity and quota limitations while maintaining 100% compatibility with the existing message-processor.

## Overview

InnerTube-based YouTube chat listener that provides quota-free message ingestion as a drop-in replacement for the official API-based listener. Maintains identical RawChatMessage contract for seamless integration with message-processor.

> **Note on quota**: message *ingestion* is fully quota-free. When `YOUTUBE_API_KEY` is configured, the listener additionally spends a single Data API unit per stream start (`videos.list`) to resolve the official `activeLiveChatId` for the streamer-send / moderation cache — see [Environment Variables](#environment-variables) and ADR-0025. This is optional and negligible; without the key the listener stays fully quota-free.

## Key Differences from Official Listener

| Feature | Official Listener | InnerTube Listener |
|---------|-------------------|-------------------|
| API | YouTube Data API v3 | InnerTube (unofficial) |
| Quota Cost | 5 units/request | Zero for ingestion (+1 unit per stream for the live-chat-id send/mod cache when `YOUTUBE_API_KEY` is set) |
| Stream Discovery | Search API | HTML parsing |
| Event Types | Standard | Standard + deletions (Phase 13) |
| Rate Limiting | API quota limits | IP-based (undocumented) |
| ToS Compliance | Official | Unofficial (use at own risk) |

## Architecture

```
InnerTube Client (HTTP polling)
  ↓ continuation-based requests
Poller (2s fixed interval)
  ↓ parse messages
Redis Streams Publisher
  ↓ XADD to chat:raw
Message Processor (unchanged)
  ↓ normalize + enrich
Overlay WebSocket
```

**Key Components**:
- **InnerTube Client**: HTTP client for `youtubei/v1/live_chat/get_live_chat_replay` endpoint
- **Poller**: Continuation-based polling loop with exponential backoff (2s → 60s max)
- **Publisher**: Redis Streams XADD with exact field mapping from official youtube-listener
- **Health Handlers**: Liveness and readiness probes for Kubernetes

### Continuation token strategy (important)

`GetInitialContinuation` (`innertube/discovery.go`) obtains the polling token by
calling YouTube's `/next` endpoint and using **YouTube's own continuation token**
from the response, then rewriting its chat-type to "Live chat" (all messages) via
`forceChatTypeAll` (`innertube/continuation_rewrite.go`). Subsequent polls follow
YouTube's returned continuation (`Client.ExtractContinuation`).

Do **not** hand-roll the continuation token from scratch. A previous approach
(`GenerateLiveChatContinuation`, removed 2026-07-17) built the protobuf token
locally with a `time.Now()` position anchor; `get_live_chat` accepted it with
HTTP 200 but answered with **zero actions**, so the listener silently captured
almost no messages on active streams (surfacing as endless "150 zero-action
polls" continuation refreshes and "a lot of YouTube messages missing" reports).
YouTube's `/next` token carries a valid live-position anchor and works; it just
defaults to "Top chat" (chattype=4), which `forceChatTypeAll` flips to
chattype=1. The viewSelector "Live chat" sub-menu token is **not** usable
directly — `get_live_chat` rejects it with HTTP 400.

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `INITIAL_CONTINUATION` | **Yes** | - | Initial continuation token from stream HTML (manual extraction for PoC) |
| `CHANNEL_ID` | **Yes** | - | YouTube channel ID for message attribution |
| `LOG_LEVEL` | No | `info` | Log verbosity: `debug` or `info` |
| `REDIS_HOST` | No | `localhost` | Redis hostname |
| `REDIS_PORT` | No | `6379` | Redis port |
| `HTTP_PORT` | No | `8080` | HTTP server port for health checks |
| `YOUTUBE_CANARY_ENABLED` | No | `false` | Run the capture canary (see below). |
| `YOUTUBE_CANARY_CHANNELS` | When enabled | - | Comma-separated `channelID:videoID` pairs. The video ID is mandatory: an unpinned canary would go through stream selection and can land on a chat-less simulcast. |
| `YOUTUBE_CANARY_POLL_INTERVAL` | No | `2s` | Floor between canary `get_live_chat` calls; YouTube's own recommended timeout wins when longer. Unlike production, the canary sleeps this interval after *every* poll (`AlwaysSleep`), so it is a real rate floor — production skips the sleep after a non-empty poll to keep viewer latency low, which on a busy canary channel would mean never sleeping at all. |
| `YOUTUBE_CANARY_REDISCOVER_INTERVAL` | No | `10m` | How long a canary target that is not polling waits before retrying / re-pinning. |
| `YOUTUBE_API_KEY` | No | - | YouTube **Data API** key. When set, the listener resolves each live stream's official `activeLiveChatId` (one `videos.list` call, 1 quota unit per stream) and publishes it to the `youtube:stream:state` cache so auth-service (streamer chat send) and moderation-service can target the chat. Unset ⇒ cache disabled; sends fall back to the unreliable `search.list` path. See ADR-0025. |

### Capture canary

An idle live chat and a broken continuation token are **the same response**:
HTTP 200 with zero actions. Nothing inside the response distinguishes them, so
"the listener captured nothing" can only be turned into "the listener is blind"
against a stream where chat is known to be flowing.

The canary (`canary/`) does exactly that. It treats the configured channels as
permanently demanded — no `overlays` row, no chat source, no WebSocket client —
and runs them through the same `GetInitialContinuation` and poll path as
production traffic, because that is where the continuation bug above lived. A
canary with its own shortcut would have been green throughout that outage.

It **counts the messages and drops them**. Nothing is published to `chat:raw`,
so message-processor throughput, emote enrichment, the
`AllChatPlatformMessagesEmpty` ratio and the DAU/WAU/MAU aggregates never see
canary volume. The blind spot the canary covers is *capture*; Redis publish, the
processor and the gateway each have their own alerts.

Operational notes:

- Runs on the leader only (lease ID `canary:<videoID>`). Every replica polling
  would multiply our YouTube request rate for no extra signal.
- Leadership uses a **separate** coordinator — `SidecarCoordinator("youtube-canary")`,
  not the one the stream manager uses. This is deliberate and load-bearing:
  `Rebalance` derives `maxPerPod` from the production source count the caller
  passes, but compares it against the coordinator's *total* lease count. Canary
  leases in that coordinator would make every pod look over-subscribed, and
  since the shed list is sorted and `canary:` sorts ahead of most video IDs, the
  leases released would be **users' streams**. Do not "simplify" the canary back
  onto the production coordinator; a test pins this.
- Video IDs are **pinned**. 24/7 channels run several concurrent streams and
  browse order routinely puts a near-empty simulcast first (#473). The canary
  re-pins itself (`most_viewers`) once a pinned stream ends, or after three
  consecutive failures on a pin that never worked.
- Configure **two** channels. A single canary going members-only or into slow
  mode would page us for someone else's moderation settings.
- Metrics: `youtube_innertube_canary_polls_total` (liveness of the detector
  itself) and `youtube_innertube_canary_messages_total` (the capture signal),
  both labelled `service, channel_id, video_id`. The alerts that read them are
  `YouTubeInnerTubeCapturingNothing` (critical) and `YouTubeInnerTubeCanaryDown`
  (warning) in caesar-deployment.

## Running Locally

### Prerequisites

1. Redis running locally:
   ```bash
   docker run -d -p 6379:6379 redis:7-alpine
   ```

2. Extract continuation token manually:
   - Visit a live YouTube stream
   - Open browser DevTools → Network tab
   - Filter for `get_live_chat_replay`
   - Copy `continuation` value from request payload

### Start Service

```bash
export INITIAL_CONTINUATION="<token_from_manual_extraction>"
export CHANNEL_ID="UCxxxxxx"  # YouTube channel ID
export LOG_LEVEL="debug"
cd services/youtube-listener-innertube
go run ./cmd/main.go
```

### Verify Operation

**Health Checks**:
```bash
curl http://localhost:8080/health/live   # Should return 200 OK
curl http://localhost:8080/health/ready  # Should return 200 when Redis connected
```

**Monitor Redis Streams**:
```bash
# Terminal 1: Watch for incoming messages
redis-cli XREAD COUNT 10 STREAMS chat:raw 0

# Should show messages with:
# - platform: "youtube"
# - message_id: UUID
# - username: display name
# - text: message content
# - timestamp: RFC3339Nano format
# - data: full JSON payload
```

## Health Checks

### Liveness Probe (`/health/live`)
- **Always returns 200 OK** (no deadlock detection in PoC)
- Indicates service process is running
- Future enhancement: detect deadlocks and return 500

### Readiness Probe (`/health/ready`)
- **Returns 200** when:
  1. Redis connection is healthy (via `publisher.Ping()`)
  2. InnerTube client is initialized
- **Returns 503** when:
  - Redis connection fails
  - InnerTube client not initialized
- Per user decision: "ready even if no stream actively monitored yet"

### Status Endpoint (`/status`)
- Debugging endpoint (not used by Kubernetes)
- Returns poller state and service information

## Contract Compatibility

This service maintains **byte-for-byte compatibility** with the official youtube-listener:

| Field | Format | Notes |
|-------|--------|-------|
| `stream` | `"chat:raw"` | Same Redis Streams key |
| `platform` | `"youtube"` | Exact match |
| `message_id` | UUID | Generated per message |
| `channel_id` | YouTube channel ID | From `CHANNEL_ID` env var |
| `user_id` | YouTube user channel ID | From `authorExternalChannelId` |
| `username` | Display name | From `authorName` |
| `text` | Message content | Concatenated text runs |
| `timestamp` | RFC3339Nano | Parsed from `timestampUsec` |
| `data` | Full JSON | Complete `RawChatMessage` struct |

**No changes required** in message-processor or downstream services.

## Known Limitations (PoC Scope)

1. **Hardcoded API Key**: Uses extracted InnerTube API key (`AIzaSyAO_FJ2SlqU8Q4STEHLGCilw_Y9_11qcW8`)
   - **Phase 10**: Dynamic extraction from stream HTML

2. **Manual Continuation Token**: Requires manually extracting continuation token from browser
   - **Phase 10**: Stream discovery via overlay-manager integration

3. **Single Stream Only**: PoC monitors one stream at a time
   - **Phase 10**: Multi-stream support via control plane

4. **No Deletion Events**: Chat message deletions not implemented
   - **Phase 13**: Deletion event handling (differentiator, not blocker)

5. **Fixed Polling Interval**: 2-second interval (not adaptive)
   - **User decision**: Keep fixed for PoC simplicity

## Deployment

See [ROLLOUT_GUIDE.md](../../docs/deployment/ROLLOUT_GUIDE.md) for canary deployment instructions.

### Docker Build

```bash
cd services/youtube-listener-innertube
docker build -t youtube-listener-innertube:v1.2.0 .
```

### Kubernetes Canary Rollout

Deployed using Argo Rollouts with automatic promotion/rollback:

```bash
kubectl apply -k deployments/k8s/youtube-listener-innertube/production/
```

Health checks:
- Liveness: `GET /health/live` every 10s
- Readiness: `GET /health/ready` every 5s
- Startup: `GET /health/ready` (initial delay 5s)

## Graceful Shutdown

The service handles `SIGTERM` and `SIGINT` signals gracefully:

1. Stop accepting new polls (cancel poller context)
2. Wait for current poll to complete (~2s max)
3. Shutdown HTTP server with 25s timeout
4. Exit cleanly

**Kubernetes consideration**: Service completes shutdown in <25s, leaving 5s buffer before `SIGKILL` at 30s.

## Testing

### Unit Tests

```bash
cd services/youtube-listener-innertube
go test ./publisher -v  # Redis publisher tests
go test ./handlers -v   # Health check tests
go test ./innertube -v  # Parser and client tests
go test ./poller -v     # Polling loop tests
```

### Integration Test (Manual)

See "Running Locally" section above for end-to-end validation.

## Comparison with Official YouTube Listener

| Feature | Official Listener | InnerTube Listener (PoC) |
|---------|-------------------|--------------------------|
| **Authentication** | OAuth 2.0 (complex) | None (public API) |
| **Quota** | 10,000 units/day (limited) | Unlimited (not subject to API quotas) |
| **Stream Discovery** | YouTube API search | Manual continuation (Phase 10: automated) |
| **Message Schema** | RawChatMessage | **Identical** (drop-in compatible) |
| **Deletion Events** | Supported | Not yet (Phase 13) |
| **Multi-stream** | Yes | Not yet (Phase 10) |
| **Rate Limiting** | Quota-based | IP-based (2s polling interval) |

## Next Phases

**Phase 10: Control Plane Integration**
- Dynamic API key extraction from stream HTML
- Stream discovery via overlay-manager
- Multi-stream support with per-stream continuation tracking

**Phase 11: Contract Testing**
- Schema drift detection
- Integration tests with message-processor
- Validation against official youtube-listener output

**Phase 12: Performance Optimization**
- Batch publishing to Redis Streams
- Adaptive polling intervals (respect InnerTube timeout hints)
- Connection pooling and resource management

**Phase 13: Deletion Events**
- Parse `markChatItemAsDeletedAction` from InnerTube responses
- Map `targetItemId` to message-processor registry
- Publish deletion events to `chat:deletions` stream

## References

- [InnerTube API Research](../../.planning/phases/09-core-ingestion-poc/09-RESEARCH.md)
- [Official YouTube Listener](../youtube-listener/README.md)
- [Message Processor](../message-processor/README.md)
- [Phase 9 Context](../../.planning/phases/09-core-ingestion-poc/09-CONTEXT.md)

## Metrics

InnerTube listener exposes Prometheus metrics on `/metrics` (port 8080, `HTTP_PORT`).

### Per-Channel Message Rate

**1-minute rolling average (messages/sec):**
```promql
rate(youtube_listener_messages_published_total{service="youtube-listener-innertube-canary",channel_id="CHANNEL_ID"}[1m])
```

**All channels aggregated:**
```promql
sum(rate(youtube_listener_messages_published_total{service="youtube-listener-innertube-canary"}[1m]))
```

**Identify stuck channels (message rate = 0 for 5+ minutes):**
```promql
youtube_listener_messages_published_total{service="youtube-listener-innertube-canary"}
  unless
ignoring(channel_id) (
  rate(youtube_listener_messages_published_total{service="youtube-listener-innertube-canary"}[5m]) > 0
)
```

### Stream Discovery

Discovery polls YouTube for a channel's live stream until it finds one, backing off
to a 60s cadence and giving up after 1h of fruitless polling. It is **not**
leader-gated (only poller startup is), so every instance runs its own loop for a
demanded channel — always reason per pod, never over a fleet-wide `sum()`.

**Concurrent loops per channel (invariant: ≤ 1 per instance):**
```promql
max by (channel_id) (youtube_listener_discovery_loops_active{namespace="allchat"})
```
Anything above 1 is a leaked loop: it scrapes YouTube on its own cadence and is
unreachable by demand-loss cancellation (which only cancels the loop holding the
`m.discovering` reservation), so it survives until the 1h give-up cap. Alerted on as
`AllChatYouTubeDiscoveryLoopLeak`.

**Discovery poll rate on the busiest instance (healthy: ≈0.017/sec = 1/min):**
```promql
max by (channel_id) (rate(youtube_listener_discovery_attempts_total{namespace="allchat"}[5m]))
```
Alerted on as `AllChatYouTubeDiscoveryRetryStorm`. A fresh loop briefly reaches
≈0.027/sec during its 10s/20s/30s aggressive phase; sustained values above that mean
duplicate loops or something forcing constant rediscovery.

**Channels parked after 1h of fruitless polling:**
```promql
sum by (channel_id) (increase(youtube_listener_discovery_gave_up_total{namespace="allchat"}[6h]))
```
Repeated give-ups for one channel while its overlay stays connected point at either a
chronically-offline source or leaked loops aging out.

### Continuation Health

A poll that returns HTTP 200 with zero chat actions is ambiguous by construction:
idle chat and a stale continuation token look identical. These two counters are
what make the ambiguity legible, and their absence is why the token bug above
took an investigation rather than a glance.

**Zero-action polls per channel:**
```promql
sum by (channel_id) (rate(youtube_listener_zero_action_polls_total{namespace="allchat"}[5m]))
```
High on a quiet channel is normal. High on a channel that is *known* to be busy
is the capture failure.

**Continuation refreshes per channel:**
```promql
sum by (channel_id) (rate(youtube_listener_continuation_refreshes_total{namespace="allchat"}[15m]))
```
One refresh is recovery working as designed (150 consecutive empty polls, ~5min).
A rate that never settles means the poller keeps losing its anchor: look at the
continuation path, not at the streamers.

### Capture Canary

**Is the detector alive?** (see `YouTubeInnerTubeCanaryDown`)
```promql
sum(rate(youtube_innertube_canary_polls_total[10m]))
```
Zero means the canary is not polling, and while that holds the capture alert
cannot fire at all.

**Is capture working?** (see `YouTubeInnerTubeCapturingNothing`)
```promql
sum(rate(youtube_innertube_canary_messages_total[10m]))
```
Near zero *while polls continue* means the listener is blind. Per canary channel:
```promql
sum by (channel_id, video_id) (rate(youtube_innertube_canary_messages_total[10m]))
```
One channel at zero and the other flowing is a moderation change on that channel
(members-only, slow mode), not our bug — which is why two canaries are configured.

> **Label caution**: `service="youtube-listener-innertube-canary"` on *every*
> metric here is the Argo Rollouts canary-deployment label, and predates the
> capture canary. It says nothing about whether a sample came from the capture
> canary; the `youtube_innertube_canary_*` metric names do.

### Error Breakdown by Type

**Error rate by type (errors/sec):**
```promql
sum by (error_type) (
  rate(youtube_listener_errors_total{service="youtube-listener-innertube-canary"}[1m])
)
```

**Network error rate:**
```promql
rate(youtube_listener_errors_total{service="youtube-listener-innertube-canary",error_type="network"}[1m])
```

**HTTP error rate:**
```promql
rate(youtube_listener_errors_total{service="youtube-listener-innertube-canary",error_type="http"}[1m])
```

**Parse error rate:**
```promql
rate(youtube_listener_errors_total{service="youtube-listener-innertube-canary",error_type="parse"}[1m])
```

**Error rate percentage (errors / total requests):**
```promql
sum(rate(youtube_listener_errors_total{service="youtube-listener-innertube-canary"}[5m]))
/ sum(rate(youtube_listener_requests_total{service="youtube-listener-innertube-canary"}[5m]))
* 100
```

### Deletion Buffer Metrics

**Buffer overflow rate:**
```promql
rate(youtube_listener_deletion_buffer_overflows_total{service="youtube-listener-innertube-canary"}[5m])
```

**Channels experiencing overflows:**
```promql
sum by (channel_id) (
  increase(youtube_listener_deletion_buffer_overflows_total{service="youtube-listener-innertube-canary"}[5m])
) > 0
```

### Grafana Dashboard Queries

For Grafana dashboards (Phase 12-03), use these queries:

**Message Rate Panel (per channel):**
- Query: `rate(youtube_listener_messages_published_total{service="youtube-listener-innertube-canary"}[1m])`
- Legend: `{{channel_id}}`
- Unit: messages/sec

**Error Breakdown Panel (stacked area):**
- Query: `sum by (error_type) (rate(youtube_listener_errors_total{service="youtube-listener-innertube-canary"}[1m]))`
- Legend: `{{error_type}}`
- Unit: errors/sec

See `docs/architecture/04-OBSERVABILITY.md` for complete dashboard configuration.

## Monitoring

- **Grafana Dashboard:** "YouTube Listener InnerTube Rollout"
- **Prometheus Metrics:** `/metrics` endpoint
- **Key Metrics:**
  - Error rate (comparison with official listener)
  - Message rate (messages per second)
  - Redis publish success rate
  - Reconnection frequency

## Troubleshooting

See [TROUBLESHOOTING_INNERTUBE.md](../../docs/deployment/TROUBLESHOOTING_INNERTUBE.md) for issue diagnosis and resolution.
