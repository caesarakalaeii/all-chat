---
phase: 04-grafana-dashboard-audit-metrics-gap-implementation
plan: 02
subsystem: metrics-wiring
tags: [metrics, prometheus, message-flow, instrumentation]
dependency_graph:
  requires: ["04-01"]
  provides: ["message-flow-metrics-wired", "WIRE-01", "WIRE-02", "WIRE-03", "WIRE-04", "WIRE-05"]
  affects: ["grafana-dashboards", "04-03-dashboards", "04-04-alerts"]
tech_stack:
  added: []
  patterns:
    - "Dependency injection via SetMetrics() for post-construction metrics wiring"
    - "RecordMessage + RecordPublish per-message in handler closures"
    - "Stream lag from Redis stream entry ID millisecond timestamp"
    - "Package-level functions for local metrics packages (discord-listener, no shared/ import)"
key_files:
  created: []
  modified:
    - services/youtube-listener/cmd/message_handler.go
    - services/youtube-listener/cmd/main.go
    - services/twitch-eventsub-listener/cmd/main.go
    - services/twitch-eventsub-listener/webhooks/handler.go
    - services/discord-listener/metrics/metrics.go
    - services/discord-listener/gateway/client.go
    - services/api-gateway/cmd/main.go
    - services/message-processor/enricher/emote_enricher.go
    - services/message-processor/consumer/stream_consumer.go
    - services/message-processor/cmd/main.go
decisions:
  - "youtube-listener RecordMessage/RecordPublish wired in MessageHandler (cmd package), not in poller, to avoid adding shared/metrics import to streams package and to stay close to the actual publish point"
  - "twitch-eventsub-listener metrics passed to webhooks.Handler; RecordMessage recorded per notification before routing, RecordPublish after routeEvent success/failure"
  - "discord-listener uses local metrics package functions (not shared/metrics/ListenerMetrics) to avoid promauto duplicate registration panic"
  - "api-gateway RecordMessageReceived fires once per Redis pub/sub message; RecordMessageSent fires per WebSocket client that received the broadcast"
  - "message-processor emote enricher uses SetMetrics() post-construction injection to avoid changing NewEnricher signature (used in many places)"
  - "stream lag computed from Redis stream entry ID millisecond prefix — no additional Redis XLEN query needed"
metrics:
  duration: "2057s (~34 min)"
  completed_date: "2026-03-26"
  tasks_completed: 3
  files_modified: 10
---

# Phase 04 Plan 02: Wire Message Flow Metrics Summary

End-to-end message flow instrumented: all 5 pipeline services now emit RecordMessage/RecordPublish/RecordMessageReceived/RecordMessageSent/RecordEmoteLookup/SetStreamLag via their respective metrics packages.

## What Was Built

### Task 1: youtube-listener and twitch-eventsub-listener

**youtube-listener** (`listenerMetrics` already initialized in `cmd/main.go`):
- Added `listenerMetrics *metrics.ListenerMetrics` field to `MessageHandler` struct in `cmd/message_handler.go`
- `RecordMessage("youtube", "youtube-listener", channelID, "chat")` on each message before publish
- `RecordPublish("youtube", "youtube-listener", "success"/"error")` after `PublishBatch`
- `RecordError` on publish failure
- Updated `NewMessageHandler` call in `cmd/main.go` to pass `listenerMetrics`

**twitch-eventsub-listener** (was completely unwired):
- Added `metrics.NewListenerMetrics("twitch-eventsub", "twitch-eventsub-listener")` to `cmd/main.go`
- Added `listenerMetrics *metrics.ListenerMetrics` field to `webhooks.Handler`
- Updated `webhooks.NewHandler` signature to accept metrics
- `RecordConnection("twitch-eventsub", ..., "webhook", true)` on subscription challenge verification
- `RecordMessage("twitch-eventsub", ..., "", subscriptionType)` on each notification before routing
- `RecordPublish("twitch-eventsub", ..., "success"/"error")` based on `routeEvent` return

### Task 2: discord-listener and api-gateway

**discord-listener** (local metrics package — no shared/metrics import):
- Added `discord_listener_messages_received_total{guild_id, channel_id}` counter
- Added `discord_listener_messages_published_total{result}` counter
- Added `IncMessageReceived(guildID, channelID)` and `IncMessagePublished(result)` package functions
- Called in `HandleMessageCreate` after filters pass (just before `publisher.Publish`)
- `IncMessagePublished("success"/"error")` based on publish result

**api-gateway**:
- `RecordMessageReceived("api-gateway", overlayID, platform)` in the Redis pub/sub message handler closure (platform extracted from JSON)
- `RecordMessageSent("api-gateway", overlayID, "success")` per WebSocket client in broadcast
- `RecordMessageDropped("api-gateway", "no_clients")` when `BroadcastToOverlay` returns 0

### Task 3: message-processor emote enrichment and stream health

**Emote enricher**:
- Added `processorMetrics *metrics.ProcessorMetrics` field to `Enricher` struct
- Added `SetMetrics(m *metrics.ProcessorMetrics)` post-construction injection method
- `RecordEmoteCacheOperation("message-processor", "hit"/"miss", "all")` in `fetchEmotes` on cache hit/miss
- `RecordEmoteLookup("message-processor", provider, "hit")` per unique provider in API response
- `RecordEmoteLookup("message-processor", "all", "miss")` when API returns no emotes
- Wired via `emoteEnricher.SetMetrics(processorMetrics)` in `cmd/main.go`

**Stream consumer** (already had `metrics` field and partial wiring):
- Added `RecordStreamError("message-processor", "read_error")` on `XReadGroup` failure
- Added `SetStreamLag` call per message using `streamEntryLag()` helper
- `streamEntryLag(streamID string)` parses Redis stream entry millisecond prefix to compute age

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | `875b2f3` | youtube-listener + twitch-eventsub-listener message flow metrics |
| 2 | `efd1ee8` | discord-listener + api-gateway message delivery metrics |
| 3 | `f942a3b` | message-processor emote enrichment + stream health metrics |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Plan referenced non-existent files**
- **Found during:** Task 1 analysis
- **Issue:** Plan referenced `services/youtube-listener/publisher/redis.go` and `services/twitch-eventsub-listener/eventsub/handler.go` which do not exist; actual files are `publisher/stream_publisher.go` and `webhooks/handler.go`
- **Fix:** Identified correct file locations via directory listing; wired metrics at the actual publish points
- **Files modified:** `services/youtube-listener/cmd/message_handler.go`, `services/twitch-eventsub-listener/webhooks/handler.go`
- **Commit:** `875b2f3`

**2. [Rule 1 - Bug] youtube-listener RecordMessage/RecordPublish placed in MessageHandler, not poller**
- **Found during:** Task 1 — reading poller.go
- **Issue:** Poller calls `messageHandler.HandleMessages()` asynchronously in goroutines (to avoid blocking gRPC receive loop); adding shared/metrics import to streams package would be correct but the publish point is in MessageHandler which is in `cmd` package where `listenerMetrics` already lives
- **Fix:** Wired metrics in `cmd/message_handler.go` — same package as `listenerMetrics` init, no new import in streams package
- **Commit:** `875b2f3`

**3. [Rule 2 - Missing] go.mod update needed for discord-listener**
- **Found during:** Task 2 build
- **Issue:** `go build` failed with "updates to go.mod needed"
- **Fix:** Ran `go mod tidy` to resolve
- **Files modified:** `services/discord-listener/go.mod`, `go.sum`
- **Commit:** `efd1ee8`

## Verification

All 5 services compiled successfully:
```
youtube-listener: OK
twitch-eventsub-listener: OK
discord-listener: OK
api-gateway: OK
message-processor: OK
```

Key metric calls confirmed via grep:
- `listener_messages_received_total{platform="youtube"}` — youtube-listener
- `listener_messages_published_total{platform="twitch-eventsub"}` — twitch-eventsub-listener
- `discord_listener_messages_received_total` — discord-listener
- `discord_listener_messages_published_total` — discord-listener
- `gateway_messages_received_total` — api-gateway
- `gateway_messages_sent_total` — api-gateway
- `processor_emote_lookups_total` — message-processor
- `processor_emote_cache_operations_total` — message-processor
- `processor_stream_lag_seconds` — message-processor
- `processor_stream_errors_total{error_type="read_error"}` — message-processor
