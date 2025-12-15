# 7TV Real-time Event API Integration

## Summary

The message processor now listens to the 7TV EventAPI WebSocket to receive immediate notifications when emotes are added, updated, or removed. This ensures the emote cache is always up-to-date without waiting for cache TTL expiration.

## What Was Implemented

### 1. **7TV EventAPI WebSocket Client** (`services/message-processor/seventv/client.go`)
- Connects to `wss://events.7tv.io/v3`
- Implements full EventAPI protocol:
  - HELLO: Initial connection handshake
  - HEARTBEAT: Keep-alive mechanism
  - SUBSCRIBE/UNSUBSCRIBE: Event subscription management
  - DISPATCH: Event data reception
  - ACK: Subscription confirmation
  - RECONNECT/END_OF_STREAM: Connection lifecycle
- Automatic reconnection with exponential backoff (5s → 5 minutes)
- Concurrent-safe subscription tracking

### 2. **Channel Tracking Manager** (`services/message-processor/seventv/manager.go`)
- Maps Twitch channels to their 7TV emote set IDs
- Fetches channel information from 7TV REST API (`https://7tv.io/v3`)
- Subscribes to emote set updates for active channels
- Invalidates emote cache when updates are received
- Handles channels without 7TV gracefully

### 3. **Cache Invalidation** (`services/message-processor/cache/emote_cache.go`)
- Added `Delete` method to cache.Store interface
- Implemented cache deletion in EmoteCache
- Enables immediate cache invalidation on emote updates

### 4. **Message Processor Integration** (`services/message-processor/cmd/main.go`)
- Initializes 7TV manager on startup
- Tracks channels automatically during message processing
- Graceful shutdown handling
- Non-blocking operation (failures don't affect message processing)

### 5. **Tests** (`services/message-processor/seventv/client_test.go`)
- HELLO message handling
- DISPATCH event processing
- Emote set update parsing
- Event handler invocation

## How It Works

```
┌─────────────────────────────────────────────────────────────────┐
│                     Message Processor Startup                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
        ┌──────────────────────────────────────────────┐
        │  Initialize 7TV Manager with Cache Store     │
        └──────────────────────────────────────────────┘
                              │
                              ▼
        ┌──────────────────────────────────────────────┐
        │  Connect to wss://events.7tv.io/v3           │
        └──────────────────────────────────────────────┘
                              │
                              ▼
        ┌──────────────────────────────────────────────┐
        │  Receive HELLO, Start Heartbeat Loop         │
        └──────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                    Message Processing Flow                      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
        ┌──────────────────────────────────────────────┐
        │  Receive Chat Message from Redis Stream      │
        └──────────────────────────────────────────────┘
                              │
                              ▼
        ┌──────────────────────────────────────────────┐
        │  Track Channel (if not already tracked)      │
        │  • Fetch 7TV user data                       │
        │  • Get emote set ID                          │
        │  • Subscribe to emote_set.update events      │
        └──────────────────────────────────────────────┘
                              │
                              ▼
        ┌──────────────────────────────────────────────┐
        │  Enrich Message with Emotes (from cache)     │
        └──────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                   Real-time Emote Update Flow                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
        ┌──────────────────────────────────────────────┐
        │  7TV: Streamer adds/removes emote            │
        └──────────────────────────────────────────────┘
                              │
                              ▼
        ┌──────────────────────────────────────────────┐
        │  EventAPI sends DISPATCH event               │
        │  • Type: emote_set.update                    │
        │  • Contains: pushed/pulled/updated emotes    │
        └──────────────────────────────────────────────┘
                              │
                              ▼
        ┌──────────────────────────────────────────────┐
        │  Manager receives event, finds channels      │
        │  using this emote set                        │
        └──────────────────────────────────────────────┘
                              │
                              ▼
        ┌──────────────────────────────────────────────┐
        │  Invalidate cache for all affected channels  │
        └──────────────────────────────────────────────┘
                              │
                              ▼
        ┌──────────────────────────────────────────────┐
        │  Next message triggers fresh emote fetch     │
        │  • New emotes immediately visible in chat    │
        └──────────────────────────────────────────────┘
```

## Benefits

1. **Immediate Updates**: Emotes appear in chat within seconds of being added to 7TV
2. **Reduced Latency**: No need to wait for 6-hour cache TTL expiration
3. **Efficient**: Only subscribes to active channels, not all channels
4. **Resilient**: Automatic reconnection on connection loss
5. **Non-blocking**: Failures don't affect message processing
6. **Scalable**: Lightweight WebSocket connection with minimal overhead

## Configuration

No additional configuration is required. The integration:
- Automatically initializes on message processor startup
- Silently handles failures (logs warnings)
- Works alongside existing emote enrichment
- Requires no environment variables

## Testing

The integration includes comprehensive tests:

```bash
# Run all message processor tests
go test ./services/message-processor/...

# Run only 7TV tests
go test -v ./services/message-processor/seventv/...
```

## Monitoring

The 7TV manager logs important events:
- Connection establishment and reconnection attempts
- Channel tracking (channel → emote set mapping)
- Subscription success/failure
- Emote set updates received
- Cache invalidation operations

Example log output:
```
INFO  Connected to 7TV EventAPI
INFO  Received HELLO from 7TV EventAPI  session_id=abc123 heartbeat_interval=30s
INFO  Mapped channel to 7TV emote set   channel_id=12345 emote_set_id=xyz789
INFO  Subscribed to emote set updates   emote_set_id=xyz789
INFO  Received emote set update         emote_set_id=xyz789 pushed=1 pulled=0
INFO  Invalidated emote cache for channel  channel_id=12345 emote_set_id=xyz789
```

## Future Enhancements

1. **Metrics**: Add Prometheus metrics for:
   - Active subscriptions count
   - Event processing rate
   - Cache invalidation count
   - Connection uptime

2. **Multi-platform Support**: Extend beyond Twitch to other platforms supported by 7TV

3. **Proactive Subscription**: Subscribe to popular channels before messages arrive

4. **Admin API**: Provide endpoints to:
   - View active subscriptions
   - Manually subscribe/unsubscribe channels
   - View connection status

5. **Delta Updates**: Instead of invalidating cache, apply delta updates directly

## Troubleshooting

### Connection Issues
- Check firewall allows WebSocket connections to `events.7tv.io`
- Verify network connectivity to 7TV services
- Check logs for connection errors

### Cache Not Updating
- Verify 7TV manager started successfully (check startup logs)
- Confirm channel has 7TV emote set (check 7TV website)
- Check if subscription was successful (search logs for "Subscribed to emote set")

### High Reconnection Rate
- May indicate network instability
- Check for proxy/load balancer issues
- Verify WebSocket keep-alive settings

## References

- **7TV EventAPI Documentation**: https://github.com/SevenTV/EventAPI
- **7TV API Documentation**: https://7tv.io/docs
- **Implementation**: `services/message-processor/seventv/`
- **Tests**: `services/message-processor/seventv/client_test.go`
