# Phase 5: Custom Spans Implementation Guide

This guide provides complete implementation details for adding custom OpenTelemetry spans to critical operations in AllChat.

## Overview

Custom spans provide deep visibility into:
1. **YouTube Quota State Machine** - Reserve/Confirm/Rollback operations
2. **Redis Stream Processing** - Message consumption from `chat:raw`
3. **WebSocket Broadcasts** - Message delivery to overlay clients

## 1. YouTube Quota Operations

### File: `services/youtube-listener/quota/tracker.go`

**Add imports:**
```go
import (
    // ... existing imports ...
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
    "go.opentelemetry.io/otel/trace"
)

// Add tracer variable after imports
var tracer = otel.Tracer("youtube-listener.quota")
```

### A. Instrument `ReserveQuotaWithPriority`

**Location:** Line 612

**Add span around entire function:**
```go
func (t *Tracker) ReserveQuotaWithPriority(ctx context.Context, units int, allowCritical bool) (string, error) {
    ctx, span := tracer.Start(ctx, "quota.reserve",
        trace.WithSpanKind(trace.SpanKindInternal),
        trace.WithAttributes(
            attribute.Int("quota.units", units),
            attribute.Bool("quota.allow_critical", allowCritical),
        ),
    )
    defer span.End()

    t.checkDateRollover()
    today := t.getCurrentDate()

    // Emergency shutoff check
    t.mu.RLock()
    currentPercentage := float64(t.usageToday) / float64(t.dailyLimit) * 100
    currentUsage := t.usageToday
    t.mu.RUnlock()

    // Add span attributes
    span.SetAttributes(
        attribute.Float64("quota.current_percentage", currentPercentage),
        attribute.Int("quota.current_usage", currentUsage),
        attribute.Int("quota.daily_limit", t.dailyLimit),
        attribute.String("quota.state", string(t.currentState)),
    )

    if currentPercentage >= t.emergencyThreshold && !allowCritical {
        span.RecordError(fmt.Errorf("emergency shutoff"))
        span.SetAttributes(
            attribute.String("quota.rejection_reason", "emergency_shutoff"),
            attribute.Float64("quota.emergency_threshold", t.emergencyThreshold),
        )
        span.SetStatus(codes.Error, "emergency shutoff")

        t.logger.Error("EMERGENCY SHUTOFF: Quota at or above emergency threshold",
            // ... existing log fields ...
        )

        return "", fmt.Errorf("emergency shutoff: quota at %.1f%% (threshold: %.1f%%)", currentPercentage, t.emergencyThreshold)
    }

    // ... rest of function ...

    // AFTER successful reservation in database:
    reservationID := fmt.Sprintf("reserve-%s", uuid)

    span.SetAttributes(
        attribute.String("quota.reservation_id", reservationID),
        attribute.Int("quota.reserved_units", units),
        attribute.Int("quota.remaining", remaining),
    )
    span.SetStatus(codes.Ok, "reservation successful")

    t.logger.Info("Reserved quota",
        // ... existing log fields ...
    )

    return reservationID, nil
}
```

### B. Instrument `ConfirmReservation`

**Location:** Line 690

**Add span:**
```go
func (t *Tracker) ConfirmReservation(ctx context.Context, reservationID string, units int) error {
    ctx, span := tracer.Start(ctx, "quota.confirm",
        trace.WithSpanKind(trace.SpanKindInternal),
        trace.WithAttributes(
            attribute.String("quota.reservation_id", reservationID),
            attribute.Int("quota.units", units),
        ),
    )
    defer span.End()

    today := t.getCurrentDate()

    // Confirm in database with retry
    if err := t.confirmReservationWithRetry(ctx, today, units, 3); err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "confirmation failed")
        t.logger.Error("Failed to confirm reservation after retries",
            // ... existing log fields ...
        )
        return err
    }

    // Update in-memory state
    t.mu.Lock()
    t.usageToday += units
    percentage := float64(t.usageToday) / float64(t.dailyLimit) * 100
    remaining := t.dailyLimit - t.usageToday
    currentState := t.currentState

    // Update state machine
    t.updateStateAndNotify(percentage)

    // Update metrics
    t.metrics.SetQuotaRemaining("youtube", "youtube-listener", "daily", fmt.Sprintf("%d", t.dailyLimit), float64(remaining))
    t.metrics.SetQuotaUsagePercent("youtube", "youtube-listener", "daily", percentage)
    t.mu.Unlock()

    span.SetAttributes(
        attribute.Int("quota.total_used", t.usageToday),
        attribute.Int("quota.remaining", remaining),
        attribute.Float64("quota.percentage", percentage),
        attribute.String("quota.state", string(currentState)),
    )
    span.SetStatus(codes.Ok, "confirmed")

    t.logger.Debug("Confirmed reservation",
        // ... existing log fields ...
    )

    return nil
}
```

### C. Instrument `RollbackReservation`

**Location:** Line 728

**Add span:**
```go
func (t *Tracker) RollbackReservation(ctx context.Context, reservationID string, units int) error {
    ctx, span := tracer.Start(ctx, "quota.rollback",
        trace.WithSpanKind(trace.SpanKindInternal),
        trace.WithAttributes(
            attribute.String("quota.reservation_id", reservationID),
            attribute.Int("quota.units", units),
            attribute.String("quota.reason", "api_call_failed"),
        ),
    )
    defer span.End()

    today := t.getCurrentDate()

    // Rollback in database with retry
    if err := t.rollbackReservationWithRetry(ctx, today, units, 3); err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "rollback failed")
        t.logger.Error("Failed to rollback reservation after retries",
            // ... existing log fields ...
        )
        return err
    }

    span.SetStatus(codes.Ok, "rolled back")

    t.logger.Info("Rolled back reservation (API call failed before reaching YouTube)",
        // ... existing log fields ...
    )

    return nil
}
```

---

## 2. Redis Stream Processing

### File: `services/message-processor/processor/processor.go`

**Add imports:**
```go
import (
    // ... existing imports ...
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
    "go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("message-processor")
```

### Instrument `processStream` function

**Find the XREADGROUP call** (look for `XReadGroup` in the processor):

```go
func (p *Processor) processStream(ctx context.Context) error {
    consumerID := fmt.Sprintf("consumer-%s", uuid.New().String()[:8])

    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            // Start span for this batch
            ctx, span := tracer.Start(ctx, "redis.xreadgroup",
                trace.WithSpanKind(trace.SpanKindConsumer),
                trace.WithAttributes(
                    attribute.String("redis.stream", "chat:raw"),
                    attribute.String("redis.group", "message-processor"),
                    attribute.String("redis.consumer", consumerID),
                    attribute.Int("redis.batch_size", 10),
                ),
            )

            // Read messages from stream
            messages, err := p.redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
                Group:    "message-processor",
                Consumer: consumerID,
                Streams:  []string{"chat:raw", ">"},
                Count:    10,
                Block:    5 * time.Second,
            }).Result()

            if err != nil {
                if err == redis.Nil {
                    // No messages, this is normal
                    span.SetAttributes(attribute.Int("redis.message_count", 0))
                    span.SetStatus(codes.Ok, "no messages")
                    span.End()
                    continue
                }

                span.RecordError(err)
                span.SetStatus(codes.Error, "xreadgroup failed")
                span.End()

                p.logger.Error("Failed to read from stream", zap.Error(err))
                time.Sleep(1 * time.Second)
                continue
            }

            // Count total messages
            messageCount := 0
            for _, stream := range messages {
                messageCount += len(stream.Messages)
            }

            span.SetAttributes(
                attribute.Int("redis.message_count", messageCount),
            )

            if messageCount == 0 {
                span.SetStatus(codes.Ok, "no messages")
                span.End()
                continue
            }

            // Process each message
            processedCount := 0
            failedCount := 0

            for _, stream := range messages {
                for _, msg := range stream.Messages {
                    // Create child span for each message processing
                    msgCtx, msgSpan := tracer.Start(ctx, "message.process",
                        trace.WithAttributes(
                            attribute.String("message.id", msg.ID),
                            attribute.String("message.platform", msg.Values["platform"].(string)),
                        ),
                    )

                    if err := p.processMessage(msgCtx, msg); err != nil {
                        failedCount++
                        msgSpan.RecordError(err)
                        msgSpan.SetStatus(codes.Error, "processing failed")
                        p.logger.Error("Failed to process message", zap.String("msg_id", msg.ID), zap.Error(err))
                    } else {
                        processedCount++
                        msgSpan.SetStatus(codes.Ok, "processed")
                    }

                    msgSpan.End()

                    // ACK message
                    p.redisClient.XAck(ctx, "chat:raw", "message-processor", msg.ID)
                }
            }

            span.SetAttributes(
                attribute.Int("redis.processed_count", processedCount),
                attribute.Int("redis.failed_count", failedCount),
            )

            if failedCount > 0 {
                span.SetStatus(codes.Error, fmt.Sprintf("%d messages failed", failedCount))
            } else {
                span.SetStatus(codes.Ok, "batch processed")
            }

            span.End()
        }
    }
}
```

---

## 3. WebSocket Broadcasts

### File: `services/api-gateway/websocket/hub.go`

**Add imports:**
```go
import (
    // ... existing imports ...
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/codes"
    "go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("api-gateway.websocket")
```

### Instrument broadcast function

**Find the broadcast method** (usually in Hub):

```go
func (h *Hub) broadcast(ctx context.Context, message []byte, overlayID string) {
    ctx, span := tracer.Start(ctx, "websocket.broadcast",
        trace.WithSpanKind(trace.SpanKindProducer),
        trace.WithAttributes(
            attribute.String("overlay_id", overlayID),
            attribute.Int("message_size", len(message)),
        ),
    )
    defer span.End()

    // Get clients for this overlay
    h.mu.RLock()
    clients := h.overlayClients[overlayID]
    clientCount := len(clients)
    h.mu.RUnlock()

    span.SetAttributes(
        attribute.Int("websocket.client_count", clientCount),
    )

    if clientCount == 0 {
        span.SetStatus(codes.Ok, "no clients")
        return
    }

    // Broadcast to all clients
    successCount := 0
    failureCount := 0

    for _, client := range clients {
        // Create child span for each client write
        clientCtx, clientSpan := tracer.Start(ctx, "websocket.send",
            trace.WithAttributes(
                attribute.String("client_id", client.ID),
                attribute.String("client_ip", client.RemoteAddr),
            ),
        )

        if err := client.WriteMessage(websocket.TextMessage, message); err != nil {
            failureCount++
            clientSpan.RecordError(err)
            clientSpan.SetStatus(codes.Error, "write failed")
            clientSpan.AddEvent("client_disconnected",
                trace.WithAttributes(
                    attribute.String("reason", err.Error()),
                ),
            )

            h.logger.Warn("Failed to write to client",
                zap.String("client_id", client.ID),
                zap.Error(err),
            )

            // Remove disconnected client
            h.removeClient(client.ID)
        } else {
            successCount++
            clientSpan.SetStatus(codes.Ok, "sent")
        }

        clientSpan.End()
    }

    span.SetAttributes(
        attribute.Int("websocket.success_count", successCount),
        attribute.Int("websocket.failure_count", failureCount),
    )

    if failureCount > 0 {
        span.SetStatus(codes.Error, fmt.Sprintf("%d/%d clients failed", failureCount, clientCount))
    } else {
        span.SetStatus(codes.Ok, "broadcast complete")
    }
}
```

---

## Testing Custom Spans

### 1. Enable Debug Logging

```bash
export OTEL_LOG_LEVEL=debug
```

### 2. Test Quota Operations

```bash
# Watch quota operations in logs
kubectl logs -f deployment/youtube-listener -n allchat | grep "quota\."

# Expected spans:
# - quota.reserve (with reservation_id, units, percentage)
# - quota.confirm (with confirmation, new totals)
# OR
# - quota.rollback (if API call failed)
```

### 3. Test Redis Stream Processing

```bash
# Send a test message to trigger processing
redis-cli XADD chat:raw * platform twitch channel_id testchannel message "Hello"

# Watch message-processor logs
kubectl logs -f deployment/message-processor -n allchat | grep "redis\."

# Expected spans:
# - redis.xreadgroup (with message_count)
# - message.process (for each message)
```

### 4. Test WebSocket Broadcasts

```bash
# Connect overlay client and watch api-gateway logs
kubectl logs -f deployment/api-gateway -n allchat | grep "websocket\."

# Expected spans when message arrives:
# - websocket.broadcast (with client_count, success_count)
# - websocket.send (for each client)
```

### 5. Query Tempo for Traces

```bash
# Port-forward Tempo
kubectl port-forward -n monitoring svc/tempo 3200:3200

# Search for quota traces
curl -s "http://localhost:3200/api/search?q=quota.reserve" | jq .

# Get full trace by ID
curl -s "http://localhost:3200/api/traces/<trace-id>" | jq .
```

---

## Expected Trace Structure

### End-to-End Message Flow Trace

```
twitch-listener.irc.receive (span 1)
  └─ redis.xadd (span 2) - Publish to chat:raw stream
       └─ redis.xreadgroup (span 3) - Message-processor reads
            └─ message.process (span 4) - Normalize + enrich
                 └─ emote.fetch (span 5) - Fetch 7TV emotes
                      └─ redis.publish (span 6) - Publish to overlay:{id}
                           └─ websocket.broadcast (span 7) - API Gateway
                                ├─ websocket.send (span 8) - Client 1
                                └─ websocket.send (span 9) - Client 2
```

### YouTube Quota Trace

```
quota.reserve (span 1)
  └─ db.transaction (span 2) - Reserve in PostgreSQL
       └─ api.youtube.videos.list (span 3) - YouTube API call
            └─ quota.confirm (span 4) - Confirm reservation
                 └─ db.transaction (span 5) - Update quota
```

---

## Benefits

1. **Debugging**: Quickly identify which step in the pipeline is slow
2. **Quota Visibility**: See exactly when quotas are reserved/confirmed/rolled back
3. **Client Issues**: Track which WebSocket clients are failing to receive messages
4. **Performance**: Identify bottlenecks (e.g., slow emote API calls)
5. **Error Correlation**: Link errors across services using trace IDs

---

## Rollout Strategy

1. **Implement one at a time**: Start with YouTube quota (highest value)
2. **Test in dev**: Verify spans appear in Tempo with correct attributes
3. **Deploy to production**: Monitor for performance impact (should be <1% CPU)
4. **Create Grafana dashboard**: Visualize quota operations, message latency, broadcast success rates
5. **Iterate**: Add more spans to other critical paths as needed

---

## Performance Considerations

- Each span adds ~100-200 bytes to trace payload
- Typical overhead: <5% CPU for span creation
- Use sampling in production (10%) to reduce storage
- Child spans (per-message, per-client) can be numerous - consider conditionally enabling for debugging only

---

## Next Steps

After implementing custom spans:

1. **Create Grafana Dashboard** for trace analysis
2. **Set up alerts** for high failure rates in quota/broadcast operations
3. **Add more spans** to other critical paths (OAuth flows, database queries, etc.)
4. **Document trace IDs** in logs for easier correlation
