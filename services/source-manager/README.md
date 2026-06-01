# Source Manager

The Source Manager service maintains an active source registry and provides Redis-based leader election for YouTube Listener instances. It coordinates distributed polling to prevent duplicate API calls and quota waste.

**Port**: 8088
**Status**: ✅ Production Ready

---

## Features

- **Active Source Registry**: Syncs active chat sources from database every 30 seconds
- **Redis-Based Leader Election**: Coordinates YouTube Listener replicas to prevent duplicate polling
- **Leadership API**: Claim, renew, and release leadership locks
- **Health Checks**: Liveness and readiness probes for Kubernetes
- **PostgreSQL LISTEN**: Real-time source change notifications
- **Metrics**: Prometheus metrics for leadership, active sources, API operations

---

## Architecture

```
Database (overlay_chat_sources)
  ↓ periodic sync (30s) + LISTEN/NOTIFY
Active Source Registry (in-memory)
  ↓ provides
Leadership API (/leadership/claim, /leadership/renew, /leadership/release)
  ↓ Redis locks (stream:{video_id})
YouTube Listener (multiple replicas)
  ├─ Replica 1: Claims leadership for stream A → polls chat
  ├─ Replica 2: Claims leadership for stream B → polls chat
  └─ Replica 3: Standby (takes over if Replica 1/2 fails)
```

---

## Environment Variables

### Required

```bash
# Database connection
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_USER=allchat
DATABASE_PASSWORD=allchat_dev_password
DATABASE_NAME=allchat

# Redis connection (for leader election)
REDIS_HOST=localhost
REDIS_PORT=6379

# Service-to-service authentication
SOURCE_MANAGER_SECRET=dev-service-secret
```

### Optional

```bash
# Server configuration
PORT=8088
LOG_LEVEL=info  # debug, info, warn, error

# Leader election settings
LEADER_TTL_SECONDS=60          # Leadership lock duration
LEADER_RENEW_INTERVAL=30       # Renew lock every 30s

# Registry sync
REGISTRY_SYNC_INTERVAL=30      # Sync from database every 30s

# OpenTelemetry tracing
OTEL_ENABLED=false
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317

# Application
APP_VERSION=dev
ENVIRONMENT=development
```

---

## API Endpoints

### Leadership Management

```bash
# Claim leadership for a stream
POST /leadership/claim
Authorization: Bearer <service-secret>
Body: {
  "stream_id": "youtube-video-id",
  "consumer_id": "youtube-listener-pod-abc123",
  "ttl_seconds": 60
}
→ Returns: { "success": true, "leader": "youtube-listener-pod-abc123", "expires_at": "..." }

# Renew leadership (extend TTL)
POST /leadership/renew
Authorization: Bearer <service-secret>
Body: {
  "stream_id": "youtube-video-id",
  "consumer_id": "youtube-listener-pod-abc123",
  "ttl_seconds": 60
}
→ Returns: { "success": true, "renewed_until": "..." }

# Release leadership (graceful shutdown)
POST /leadership/release
Authorization: Bearer <service-secret>
Body: {
  "stream_id": "youtube-video-id",
  "consumer_id": "youtube-listener-pod-abc123"
}
→ Returns: { "success": true }

# Get current leader for stream
GET /leadership/:stream_id
Authorization: Bearer <service-secret>
→ Returns: { "leader": "youtube-listener-pod-abc123", "expires_at": "..." }
```

### Active Sources

```bash
# Get all active sources
GET /sources
Authorization: Bearer <service-secret>
→ Returns: [
  {
    "overlay_id": "uuid",
    "platform": "youtube",
    "channel_id": "UCxxxxxx",
    "is_active": true
  },
  ...
]

# Get active sources for specific platform
GET /sources?platform=youtube
Authorization: Bearer <service-secret>
```

### Health Checks

```bash
GET /health/live   # Liveness
GET /health/ready  # Readiness (checks DB + Redis)
GET /metrics       # Prometheus metrics
```

---

## How It Works

### Leader Election Flow

**YouTube Listener replicas coordinate to avoid duplicate polling:**

```
1. YouTube Listener Replica 1 starts polling stream "video123"
   ↓
2. Before first API call, claim leadership:
   POST source-manager:8088/leadership/claim
   Body: { stream_id: "video123", consumer_id: "replica-1" }
   ↓
3. Source Manager sets Redis key:
   SET leader:stream:video123 "replica-1" EX 60
   ↓
4. Replica 1 receives leadership → starts polling
   ↓
5. Replica 2 tries to claim same stream:
   POST source-manager:8088/leadership/claim
   Body: { stream_id: "video123", consumer_id: "replica-2" }
   ↓
6. Source Manager checks Redis key exists → leadership denied
   ↓
7. Replica 2 enters standby mode (monitors leader health)
   ↓
8. Replica 1 renews leadership every 30s:
   POST /leadership/renew (extends TTL to 60s)
   ↓
9. If Replica 1 crashes (stops renewing):
   - Redis key expires after 60s
   - Replica 2 claims leadership
   - Polling continues with <60s interruption
```

**Redis Keys**:
```
leader:stream:<video_id>  → "youtube-listener-pod-abc123"  (TTL: 60s)
leader:stream:<video_id>  → "youtube-listener-pod-def456"  (another stream)
```

### Demand Publishing (overlay → listeners)

Demand-gated listeners (twitch-eventsub chat, youtube, …) only do work for sources whose overlay
has a live WebSocket. source-manager computes that demand and publishes a full-replacement snapshot
to the `source:demand` Pub/Sub channel.

- **Source of truth:** the `overlay:connected:{overlay_id}` keys api-gateway sets (with a TTL) for
  every live overlay WebSocket. The demanded set = sources of every overlay whose key exists.
- **Triggers:** overlay connect/disconnect events (`overlay:connections`), source-config changes
  (PostgreSQL `LISTEN`), and a **periodic reconcile every 15s**.
- **Periodic reconcile is required for correctness.** source-manager runs on multiple replicas and
  Redis Pub/Sub has no replay, so a replica that briefly drops its `overlay:connections`
  subscription misses events and its in-memory demand diverges. Two replicas would then publish
  conflicting snapshots and demand-gated listeners flap or get stuck on the wrong channel set. The
  reconcile rebuilds demand from the `overlay:connected:*` keys (via SCAN) so every replica
  converges to the same set; publishes are fingerprint-gated, so an unchanged set is not re-sent.

### Active Source Registry

**Syncs from database every 30 seconds**:

```go
// registry/registry.go
func (r *Registry) Sync(ctx context.Context) {
    sources, _ := r.db.Query(ctx, `
        SELECT ocs.overlay_id, ocs.platform, ocs.channel_identifier, ocs.is_active
        FROM overlay_chat_sources ocs
        JOIN overlays o ON o.id = ocs.overlay_id
        WHERE o.is_active = true
    `)

    r.mu.Lock()
    r.activeSources = sources
    r.mu.Unlock()
}
```

**Also listens for real-time changes**:
```go
// PostgreSQL LISTEN/NOTIFY
LISTEN source_changes;

// When source added/updated/deleted, overlay-manager sends:
NOTIFY source_changes, '{"action": "added", "overlay_id": "...", "platform": "youtube"}'
```

---

## Testing

```bash
# Run all tests
go test ./... -v

# Run with coverage
go test ./... -cover

# Test leader election
go test ./election -v
```

---

## Monitoring

### Key Metrics

```promql
# Active sources
source_manager_active_sources_total{platform="youtube"}

# Leadership operations
rate(source_manager_leadership_claims_total{result="success|denied"}[5m])
rate(source_manager_leadership_renewals_total{result="success|failed"}[5m])

# Current leaders
source_manager_active_leaders_total
```

---

## Troubleshooting

### Leadership Claims Always Denied

**Symptom**: YouTube Listener cannot claim leadership for any stream

**Check Redis**:
```bash
redis-cli KEYS "leader:stream:*"
redis-cli GET leader:stream:<video_id>
redis-cli TTL leader:stream:<video_id>
```

**Solutions**:
1. Check Redis connection (Source Manager logs)
2. Verify Redis keys expiring correctly (TTL should be 60s)
3. Check if another replica already has leadership
4. Verify `SOURCE_MANAGER_SECRET` matches across services

**File**: `election/leader.go:ClaimLeadership()`

### Registry Not Syncing

**Symptom**: Active sources not updated, listeners not joining new channels

**Check logs**:
```bash
kubectl logs -n allchat deployment/source-manager | grep "Registry sync"

# Expected every 30s:
# INFO: Registry sync completed  total_sources=42
```

**Solutions**:
1. Check database connection (Source Manager logs)
2. Verify `overlay_chat_sources` table has data
3. Check PostgreSQL LISTEN connection active

**File**: `registry/registry.go:StartSyncLoop()`

---

## Production Considerations

1. **Service Secret**: Use strong random secret for `SOURCE_MANAGER_SECRET` (min 32 chars)
2. **Leader TTL**: Balance between failover speed (shorter TTL) and renewal overhead (longer TTL)
3. **Registry Sync**: 30s interval balances freshness vs database load
4. **Redis Persistence**: Leader keys are ephemeral (OK to lose on Redis restart)
5. **Authentication**: All endpoints require service secret (service-to-service auth)

---

## Related Services

- **YouTube Listener**: Uses leadership API to coordinate polling across replicas
- **Overlay Manager**: Triggers registry updates via PostgreSQL NOTIFY
- **Redis**: Stores leader election locks (ephemeral keys with TTL)

---

## Further Reading

- **[ADR-0002](../../docs/adr/0002-redis-streams-pubsub.md)** - Redis patterns
- **[YouTube Listener README](../youtube-listener/README.md)** - Leader election usage

---

## License

Copyright © 2025 All-Chat. All rights reserved.
