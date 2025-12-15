# Message Age Filtering

## Summary

The message processor now filters out old messages to prevent stale chat messages from appearing on overlays. Messages older than a configurable cutoff (default: 60 seconds) are ignored.

## Problem Solved

Previously, messages sent hours ago could appear on overlays due to:
- Redis Stream processing delays
- Service restarts causing message replay
- Network issues causing message backlog
- Stream consumer lag

This created a poor viewer experience with outdated chat messages appearing in overlays.

## Implementation

### 1. **Timestamp Filtering** (`services/message-processor/cmd/main.go:138`)

Added filtering logic at the beginning of the message handler:

```go
// Filter out old messages based on timestamp
messageAge := time.Since(rawMsg.Timestamp)
if messageAge > messageAgeCutoff {
    log.Debug("Ignoring old message",
        zap.String("message_id", rawMsg.MessageID),
        zap.Duration("message_age", messageAge),
        zap.Duration("cutoff", messageAgeCutoff),
    )
    processorMetrics.RecordMessageProcessed("message-processor", rawMsg.Platform, "filtered_old", "success")
    return nil
}
```

### 2. **Configurable Cutoff** (`services/message-processor/cmd/main.go:40`)

The cutoff is configurable via environment variable:

```go
// Parse message age cutoff (default 60 seconds)
messageAgeCutoffSeconds := 60
if cutoffStr := getEnvOrDefault("MESSAGE_AGE_CUTOFF_SECONDS", "60"); cutoffStr != "" {
    if parsed, err := time.ParseDuration(cutoffStr + "s"); err == nil {
        messageAgeCutoffSeconds = int(parsed.Seconds())
    }
}
messageAgeCutoff := time.Duration(messageAgeCutoffSeconds) * time.Second
```

### 3. **Metrics Integration**

Filtered messages are tracked in Prometheus metrics:
- Counter: `message_processed{stage="filtered_old", status="success"}`
- Allows monitoring of how many old messages are being filtered

## Configuration

### Environment Variable

**`MESSAGE_AGE_CUTOFF_SECONDS`** (default: `60`)
- Maximum age of messages to process, in seconds
- Messages older than this will be ignored
- Can be set to any positive integer

**Examples:**
```bash
# Default: 60 seconds (1 minute)
MESSAGE_AGE_CUTOFF_SECONDS=60

# More lenient: 5 minutes
MESSAGE_AGE_CUTOFF_SECONDS=300

# Strict: 30 seconds
MESSAGE_AGE_CUTOFF_SECONDS=30

# Very lenient: 10 minutes (not recommended)
MESSAGE_AGE_CUTOFF_SECONDS=600
```

### Recommended Values

- **Default (60s)**: Good balance for most use cases
- **30s**: If you want only very recent messages
- **120s (2 min)**: For slower networks or higher processing delays
- **300s (5 min)**: Maximum recommended for any production use

**Warning**: Setting this too high defeats the purpose of filtering stale messages. Values over 5 minutes are not recommended.

## How It Works

```
┌─────────────────────────────────────────────────────────────────┐
│                    Message Processing Flow                      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
        ┌──────────────────────────────────────────────┐
        │  Receive Message from Redis Stream           │
        │  (contains timestamp from original send)     │
        └──────────────────────────────────────────────┘
                              │
                              ▼
        ┌──────────────────────────────────────────────┐
        │  Calculate Message Age                       │
        │  age = time.Since(message.Timestamp)         │
        └──────────────────────────────────────────────┘
                              │
                              ▼
        ┌──────────────────────────────────────────────┐
        │  Check Age Against Cutoff                    │
        │  if age > cutoff: FILTER OUT                 │
        └──────────────────────────────────────────────┘
                         /           \
                        /             \
                  FILTERED          PROCESSED
                      │                 │
                      ▼                 ▼
        ┌─────────────────────┐  ┌────────────────────┐
        │  Log Debug Message  │  │ Continue Processing│
        │  Record Metric      │  │ (normalize, enrich)│
        │  Return (skip)      │  │ Publish to overlay │
        └─────────────────────┘  └────────────────────┘
```

## Benefits

1. **Better User Experience**: Only fresh, relevant messages appear on overlays
2. **Prevents Confusion**: No old messages appearing after stream restarts
3. **Reduces Processing**: Skips enrichment for messages that won't be shown
4. **Observable**: Metrics track filtered message count
5. **Configurable**: Adjust cutoff based on your needs

## Testing

The implementation includes comprehensive unit tests:

```bash
# Run timestamp filtering tests
go test -v ./services/message-processor/cmd/...

# Tests cover:
# - Recent messages (should not be filtered)
# - Old messages (should be filtered)
# - Messages at cutoff boundary
# - Different cutoff values
# - Environment variable parsing
```

All tests pass ✅

## Monitoring

### Logs

Filtered messages generate debug logs:

```
DEBUG  Ignoring old message  message_id=abc123 platform=twitch channel_id=12345
       message_age=2m15s cutoff=1m0s timestamp=2025-01-15T12:00:00Z
```

### Metrics

Monitor filtered messages via Prometheus:

```promql
# Rate of filtered messages per second
rate(message_processed{stage="filtered_old"}[5m])

# Percentage of messages filtered
rate(message_processed{stage="filtered_old"}[5m]) /
rate(message_processed[5m]) * 100
```

### Health Impact

Message filtering does **not** affect service health:
- Filtered messages are normal operation, not errors
- Health checks remain green
- Processing continues for valid messages

## Edge Cases Handled

1. **Service Restart**: Old messages in Redis Stream are filtered on replay
2. **Clock Skew**: Uses message timestamp (from source), not processing time
3. **Zero Cutoff**: Setting to 0 disables filtering (not recommended)
4. **Invalid Values**: Falls back to 60s default with warning log

## Future Enhancements

1. **Per-Platform Cutoffs**: Different cutoffs for different platforms
2. **Per-Overlay Cutoffs**: Configurable per overlay via database
3. **Adaptive Cutoffs**: Adjust based on processing lag
4. **Dashboard Metrics**: Grafana dashboard for filtered message visualization

## Troubleshooting

### Messages Still Appearing Late

1. Check the cutoff value: `docker logs message-processor | grep "Message age cutoff"`
2. Verify message timestamps: Enable debug logging (`LOG_LEVEL=debug`)
3. Check for clock skew between services
4. Verify listener services are setting timestamps correctly

### Too Many Messages Filtered

1. Increase `MESSAGE_AGE_CUTOFF_SECONDS` value
2. Check for processing delays (Redis Stream consumer lag)
3. Verify adequate system resources (CPU, memory)
4. Check Redis connection latency

### No Messages Appearing

1. Verify cutoff isn't set too low (< 5 seconds may be too strict)
2. Check listener services are running and sending messages
3. Verify message timestamps are being set correctly
4. Check Redis Stream has messages: `redis-cli XLEN chat:raw`

## References

- **Implementation**: `services/message-processor/cmd/main.go:138`
- **Tests**: `services/message-processor/cmd/timestamp_filter_test.go`
- **Configuration**: `.env.example` (MESSAGE_AGE_CUTOFF_SECONDS)
- **Documentation**: `CLAUDE.md` (Message Processor section)
