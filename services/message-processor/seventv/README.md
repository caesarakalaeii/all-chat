# 7TV EventAPI Integration

This package provides real-time emote update tracking from the 7TV EventAPI WebSocket.

## Overview

The 7TV EventAPI integration allows the message processor to receive immediate notifications when emotes are added, updated, or removed from 7TV emote sets. This ensures that the emote cache is always up-to-date without relying solely on TTL-based expiration.

## Architecture

### Components

1. **Client (`client.go`)**: WebSocket client that connects to `wss://events.7tv.io/v3` and handles the EventAPI protocol
   - Handles connection lifecycle (connect, reconnect, heartbeat)
   - Implements EventAPI protocol (HELLO, HEARTBEAT, SUBSCRIBE, DISPATCH, ACK)
   - Processes incoming emote set update events
   - Automatic reconnection with exponential backoff

2. **Manager (`manager.go`)**: Manages channel tracking and cache invalidation
   - Maps Twitch channels to their 7TV emote set IDs
   - Subscribes to emote set updates for active channels
   - Invalidates emote cache when updates are received
   - Fetches channel information from 7TV API

## How It Works

1. **Initialization**: The manager is initialized when the message processor starts
2. **Connection**: The client connects to the 7TV EventAPI WebSocket
3. **Channel Tracking**: When a message is processed from a channel:
   - The manager fetches the channel's 7TV emote set ID (if not already cached)
   - The client subscribes to updates for that emote set
4. **Real-time Updates**: When 7TV emotes are added/removed/updated:
   - The EventAPI sends a DISPATCH event with the changes
   - The manager receives the event and invalidates the cache for affected channels
   - Next message enrichment will fetch fresh emote data

## Protocol Details

### EventAPI Opcodes

- `0` (DISPATCH): Event data (emote updates)
- `1` (HELLO): Initial connection handshake
- `2` (HEARTBEAT): Keep-alive ping
- `4` (RECONNECT): Server requests reconnection
- `5` (ACK): Subscription acknowledged
- `6` (ERROR): Error message
- `7` (END_OF_STREAM): Stream ended
- `35` (SUBSCRIBE): Subscribe to event type
- `36` (UNSUBSCRIBE): Unsubscribe from event type

### Event Types

- `emote_set.update`: Emote set changes (added, removed, updated emotes)

## Usage

The integration is automatically initialized in the message processor's `main.go`:

```go
// Initialize 7TV event manager for real-time emote updates
seventvManager := seventv.NewManager(emoteCacheStore, log)
if err := seventvManager.Start(ctx); err != nil {
    log.Warn("Failed to start 7TV event manager", zap.Error(err))
}
defer seventvManager.Stop()
```

Channel tracking happens automatically during message processing:

```go
// Track channel for 7TV real-time emote updates
go func() {
    if err := seventvManager.TrackChannel(ctx, platform, channelID); err != nil {
        log.Debug("Failed to track channel", zap.Error(err))
    }
}()
```

## Benefits

1. **Immediate Updates**: Emotes appear in chat as soon as they're added to 7TV
2. **Reduced Latency**: No need to wait for cache TTL expiration
3. **Efficient**: Only subscribes to active channels
4. **Resilient**: Automatic reconnection and resubscription on connection loss
5. **Non-blocking**: Failures don't affect message processing

## Error Handling

- Connection failures log warnings but don't stop the message processor
- Cache invalidation errors are logged but don't affect the event stream
- Channels without 7TV emote sets are silently skipped
- Automatic reconnection with exponential backoff (5s → 5 minutes)

## Testing

Run tests with:
```bash
go test -v ./services/message-processor/seventv/...
```

Tests cover:
- HELLO message handling
- DISPATCH event processing
- Event handler invocation
- Emote set update parsing

## Future Enhancements

- Support for other platforms beyond Twitch
- Proactive subscription to popular channels
- Metrics for subscription count and event processing
- Admin API to view active subscriptions
