# Kick Listener Service

## Overview

The Kick Listener is a microservice that connects to Kick.com's chat system via Pusher WebSocket and publishes raw chat messages to Redis Streams for processing by the Message Processor service.

**Port**: 8089

## Architecture

The Kick Listener uses a different architecture compared to Twitch (IRC) and YouTube (HTTP polling):

```
┌─────────────────────┐
│  Kick Pusher        │
│  WebSocket Server   │
│  (wss://ws-us2.     │
│   pusher.com)       │
└──────────┬──────────┘
           │ WebSocket
           │ Protocol 7
           ▼
┌─────────────────────┐
│  Kick Listener      │
│  - WebSocket Client │
│  - Channel Manager  │
│  - Publisher        │
└──────────┬──────────┘
           │ XADD
           ▼
┌─────────────────────┐
│  Redis Streams      │
│  (chat:raw)         │
└─────────────────────┘
```

## Key Components

### 1. WebSocket Client (`websocket/client.go`)

Implements the Pusher WebSocket Protocol 7 for connecting to Kick's chat infrastructure.

**Features:**
- Pusher protocol handshake and connection management
- Channel subscription/unsubscription via `pusher:subscribe` events
- Automatic ping/pong for connection keepalive
- Message parsing for `App\\Events\\ChatMessageSentEvent`
- Automatic reconnection with exponential backoff
- Thread-safe channel management

**Pusher WebSocket URL:**
```
wss://ws-us2.pusher.com/app/32cbd69e4b950bf97679?protocol=7&client=js&version=8.4.0-rc2&flash=false
```

**Channel Format:**
```
chatrooms.{chatroom_id}
```

**Example Message:**
```json
{
  "event": "App\\Events\\ChatMessageSentEvent",
  "channel": "chatrooms.123456",
  "data": {
    "id": "msg-uuid",
    "chatroom_id": 123456,
    "content": "Hello world!",
    "type": "message",
    "created_at": "2025-11-15T12:34:56.000000Z",
    "sender": {
      "id": 789,
      "username": "username",
      "slug": "username",
      "identity": {
        "color": "#FF0000",
        "badges": [
          {"type": "subscriber", "text": "Subscriber"},
          {"type": "moderator", "text": "Moderator"}
        ]
      }
    }
  }
}
```

### 2. Channel Manager (`channels/manager.go`)

Manages dynamic subscription to Kick chat channels based on active overlays in the database.

**Features:**
- Syncs active channels from database every 30 seconds
- Fetches chatroom IDs from Kick API (`https://kick.com/api/v2/channels/{slug}`)
- Dynamically subscribes/unsubscribes from channels
- Stores chatroom ID in database for future use
- Maps chatroom IDs to overlay IDs for message routing

**Channel Discovery Flow:**
1. Query `overlay_chat_sources` table for active Kick channels
2. For each channel without a `chatroom_id`, call Kick API
3. Extract chatroom ID from API response
4. Store chatroom ID in database metadata
5. Subscribe to `chatrooms.{chatroom_id}` channel via WebSocket

### 3. Redis Publisher (`publisher/redis.go`)

Publishes raw chat messages to Redis Streams for consumption by Message Processor.

**Stream Key:** `chat:raw`

**Message Format:**
```json
{
  "platform": "kick",
  "overlay_id": "uuid",
  "channel_id": "channel_slug",
  "channel_name": "channel_slug",
  "raw_message": {...},  // Full Kick message
  "timestamp": "2025-11-15T12:34:56Z"
}
```

### 4. Health Handlers (`handlers/health.go`)

Provides Kubernetes health check endpoints.

**Endpoints:**
- `GET /health/live` - Liveness probe (always returns 200)
- `GET /health/ready` - Readiness probe (checks WebSocket + Redis)
- `GET /status` - Detailed status with subscriptions

## Environment Variables

```bash
# Server Configuration
PORT=8089                    # HTTP server port
LOG_LEVEL=info              # Logging level (debug, info, warn, error)

# Database Connection (PostgreSQL)
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=allchat
DATABASE_PASSWORD=allchat_dev_password
DATABASE_NAME=allchat

# Redis Connection
REDIS_HOST=localhost
REDIS_PORT=6379

# Source Manager (leadership)
SOURCE_MANAGER_URL=http://localhost:8088
SOURCE_MANAGER_SECRET=dev-service-secret

# Application
APP_VERSION=dev             # Application version
```

## Database Schema

The Kick Listener reads from `overlay_chat_sources` table:

```sql
SELECT
  ocs.overlay_id,
  ocs.channel_identifier as channel_slug,
  ocs.metadata->>'chatroom_id' as chatroom_id,
  ocs.is_active
FROM overlay_chat_sources ocs
WHERE ocs.platform = 'kick'
  AND ocs.is_active = true
```

**Metadata Structure:**
```json
{
  "chatroom_id": 123456
}
```

The chatroom_id is fetched from Kick API and stored in the metadata JSONB column for performance optimization.

## API Integration

### Kick Channel API

**Endpoint:** `GET https://kick.com/api/v2/channels/{channel_slug}`

**Response:**
```json
{
  "id": 12345,
  "user_id": 67890,
  "slug": "channel_name",
  "is_live": true,
  "chatroom": {
    "id": 123456,
    "chatroom_id": 123456,
    "channel_id": 12345,
    "slow_mode": false,
    "subscribers_mode": false,
    "followers_mode": false
  }
}
```

The `chatroom.id` is used to subscribe to the Pusher channel.

## Development

### Prerequisites

- Go 1.25+
- PostgreSQL 16+
- Redis 7+
- Access to Kick.com API

### Build

```bash
# From repository root
go build -o kick-listener ./services/kick-listener/cmd/

# Or using Make
make build
```

### Run Locally

```bash
# Set environment variables
export DATABASE_HOST=localhost
export REDIS_HOST=localhost
export LOG_LEVEL=debug

# Run the service
./kick-listener
```

### Testing

```bash
# Run tests
go test -v ./services/kick-listener/...

# With coverage
go test -cover ./services/kick-listener/...
```

### Docker

```bash
# Build image
docker build -f services/kick-listener/Dockerfile -t kick-listener:dev .

# Run container
docker run -p 8089:8089 \
  -e DATABASE_HOST=postgres \
  -e REDIS_HOST=redis \
  kick-listener:dev
```

## Deployment

### Kubernetes

The service is deployed to Kubernetes with:

- **Deployment**: 2 replicas (can scale up to 5)
- **Service**: ClusterIP on port 8089
- **HPA**: CPU-based autoscaling (70% threshold)
- **Probes**: Liveness and readiness checks

**Example Deployment:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kick-listener
  namespace: allchat
spec:
  replicas: 2
  selector:
    matchLabels:
      app: kick-listener
  template:
    metadata:
      labels:
        app: kick-listener
    spec:
      containers:
      - name: kick-listener
        image: ghcr.io/caesarakalaeii/allchat-kick-listener:main
        ports:
        - containerPort: 8089
        env:
        - name: DATABASE_HOST
          value: allchat-cluster-rw
        - name: REDIS_HOST
          value: redis
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8089
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /health/ready
            port: 8089
          initialDelaySeconds: 5
          periodSeconds: 10
```

### Scaling

The Kick Listener can scale horizontally:
- Multiple instances connect to the same Pusher WebSocket
- Pusher handles fan-out to all connected clients
- Each instance processes messages independently
- No coordination needed between instances

## Message Flow

```
1. User configures Kick channel in overlay settings
2. Record created in overlay_chat_sources (platform='kick', channel_identifier='channel_slug')
3. Kick Listener syncs from database
4. Listener fetches chatroom_id from Kick API
5. Listener subscribes to chatrooms.{chatroom_id} via WebSocket
6. Chat messages arrive via Pusher WebSocket
7. Listener publishes to Redis Streams (chat:raw)
8. Message Processor consumes from stream
9. Kick Normalizer converts to unified format
10. Emote Enricher adds 7TV/BTTV/FFZ emotes
11. Publisher sends to overlay-specific pub/sub channel
12. API Gateway forwards to WebSocket clients
13. Frontend displays message in overlay
```

## Troubleshooting

### WebSocket Connection Issues

**Symptom:** Service can't connect to Pusher WebSocket

**Solutions:**
1. Check network connectivity: `curl -I https://ws-us2.pusher.com`
2. Verify Pusher URL is current (Kick may change app key)
3. Check logs for connection errors
4. Ensure no firewall blocking WebSocket connections

### Channel Subscription Failures

**Symptom:** Can't subscribe to channels

**Solutions:**
1. Verify channel exists on Kick: `curl https://kick.com/api/v2/channels/{slug}`
2. Check chatroom_id is non-zero
3. Ensure channel is active in database
4. Check WebSocket connection is established before subscribing

### No Messages Received

**Symptom:** Subscribed but no messages

**Solutions:**
1. Verify channel is actually live and has chat activity
2. Check WebSocket connection: `/status` endpoint
3. Verify Redis Stream is receiving messages: `XREAD STREAMS chat:raw 0`
4. Check logs for message handling errors
5. Ensure Message Processor is running

### High Memory/CPU Usage

**Symptom:** Service consuming too many resources

**Solutions:**
1. Reduce number of subscribed channels
2. Increase pod resource limits
3. Scale horizontally (add more replicas)
4. Check for message processing bottlenecks
5. Monitor Redis Streams backlog

## Monitoring

### Key Metrics

- **WebSocket Connection**: Check `/health/ready`
- **Active Subscriptions**: Check `/status` endpoint
- **Message Rate**: Monitor Redis Stream length
- **Error Rate**: Check logs for failed publishes
- **Reconnection Count**: Track reconnection events in logs

### Logs

```bash
# View logs
kubectl logs -n allchat deployment/kick-listener

# Follow logs
kubectl logs -n allchat deployment/kick-listener -f

# Filter for errors
kubectl logs -n allchat deployment/kick-listener | grep ERROR
```

### Health Checks

```bash
# Liveness
curl http://localhost:8089/health/live

# Readiness
curl http://localhost:8089/health/ready

# Detailed status
curl http://localhost:8089/status
```

## Performance

### Resource Usage

**Typical Usage (per pod):**
- Memory: 50-150 MB
- CPU: 50-200m (0.05-0.2 cores)
- Network: Low (WebSocket + Redis)

**Peak Usage (100+ channels):**
- Memory: 200-300 MB
- CPU: 500m-1 core
- Network: Moderate

### Scaling Guidelines

- **1-50 channels**: 1-2 replicas sufficient
- **50-200 channels**: 2-3 replicas recommended
- **200+ channels**: 3-5 replicas with HPA

## Differences from Other Listeners

### vs Twitch Listener (IRC)

| Feature | Twitch | Kick |
|---------|--------|------|
| Protocol | IRC | WebSocket (Pusher) |
| Connection | Single IRC connection | WebSocket per instance |
| Rate Limits | 20 JOIN/10s | No JOIN limits |
| Reconnection | Built-in IRC reconnect | Custom reconnect logic |
| Message Format | IRC PRIVMSG | JSON events |

### vs YouTube Listener (HTTP Polling)

| Feature | YouTube | Kick |
|---------|---------|------|
| Protocol | HTTP REST | WebSocket (Pusher) |
| Latency | 2-5 seconds | Real-time (~100ms) |
| API Quota | 1,009,000 units/day | No quota (WebSocket) |
| Authentication | OAuth per user | No auth required |
| Scalability | Leader election needed | Fully horizontal |

## Security Considerations

### WebSocket Security

- ✅ Uses TLS (wss://)
- ✅ No authentication required for public chat
- ⚠️ Pusher app key is public (embedded in URL)
- ℹ️ No sensitive data transmitted

### Data Privacy

- Messages are public chat data
- No user tokens or credentials handled
- OAuth tokens managed by auth-service
- Messages stored transiently in Redis Streams

## Known Limitations

1. **Channel Discovery**: Requires additional API call to get chatroom_id
2. **No Private Channels**: Only public chat supported
3. **Pusher Protocol**: Dependent on Kick's Pusher configuration
4. **API Changes**: Kick may change WebSocket URL or protocol
5. **Rate Limiting**: Kick API calls for channel info may be rate limited

## Future Improvements

- [ ] Batch channel info API calls
- [ ] Cache chatroom IDs longer (currently per-overlay)
- [ ] Add Prometheus metrics endpoint
- [ ] Implement circuit breaker for Kick API calls
- [ ] Support private/subscriber-only channels
- [ ] Add reconnection backoff metrics
- [ ] Implement health check for individual channel subscriptions

## References

- **Kick Developer Docs**: https://docs.kick.com
- **Kick Dev API GitHub**: https://github.com/KickEngineering/KickDevDocs
- **Pusher Protocol**: https://pusher.com/docs/channels/library_auth_reference/pusher-websockets-protocol/
- **Go WebSocket Library**: https://github.com/gorilla/websocket

## Support

For issues or questions:
- Check logs: `kubectl logs -n allchat deployment/kick-listener`
- View status: `curl http://kick-listener:8089/status`
- GitHub Issues: https://github.com/caesarakalaeii/all-chat/issues
