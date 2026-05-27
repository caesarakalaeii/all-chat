# All-Chat: Scaling & Performance

**Version**: 2.0 (Consolidated)
**Last Updated**: 2026-01-28
**Status**: Production Ready (with known limitations)

---

## Table of Contents

1. [Introduction](#introduction)
2. [Scalability Analysis](#scalability-analysis)
3. [Service Scaling Strategies](#service-scaling-strategies)
4. [Infrastructure Scaling](#infrastructure-scaling)
5. [Performance Bottlenecks](#performance-bottlenecks)
6. [Capacity Planning](#capacity-planning)
7. [Optimization Techniques](#optimization-techniques)

---

## Introduction

All-Chat is designed to scale horizontally across all components. This document provides scaling strategies, performance optimization techniques, capacity planning guidance, and **honest assessment of current limitations**.

### Scaling Principles

- **Horizontal scaling**: Add more replicas (preferred for stateless services)
- **Vertical scaling**: Increase CPU/memory (for databases, Redis)
- **Caching**: Reduce database load (emote cache, Redis)
- **Partitioning**: Distribute load (Redis Cluster, PostgreSQL read replicas)
- **Backpressure**: Prevent overload (quota management, rate limiting)

### Current Architecture Capacity

**Realistic Sustained Throughput**:
- **Current**: ~500-1,000 messages/second (Phase 1)
- **With Redis Cluster**: ~3,000-5,000 messages/second (Phase 2)
- **With Message Queue (Kafka)**: 10,000+ messages/second (Phase 3+)

**Bottlenecks**:
1. **Redis Pub/Sub fan-out** (O(n) memory growth with replicas)
2. **Redis Streams MAXLEN** (50K messages = 5 seconds at 10K msg/s)
3. **Single Redis instance** (no clustering in Phase 1)
4. **PostgreSQL single-primary writes** (read replicas available but not fully utilized)

---

## Scalability Analysis

### Message Flow Bottlenecks

```
Twitch IRC (1000s msg/s)     YouTube API (polling 2-5s)
         ↓                              ↓
   Twitch Listener              YouTube Listener
    (1-2 replicas)              (1-5 replicas, leader election)
         ↓                              ↓
         └──────────────┬──────────────┘
                        ↓
              Redis Streams (chat:raw)
              🔴 BOTTLENECK: Single instance
              MAXLEN 50K (~5s at 10K msg/s)
                        ↓
              Message Processor
              (3-5 replicas, consumer group)
                        ↓
              Redis Pub/Sub (overlay:*)
              🔴 BOTTLENECK: O(n) fan-out
                        ↓
              API Gateway WebSocket
              (2-26 replicas, 2.5K connections each)
```

**Critical Issues**:

1. **Redis Pub/Sub Fan-out** (🔴 CRITICAL):
   - 10,000 msg/s × 26 Gateway pods = **260,000 message deliveries/sec**
   - O(n) memory growth with subscribers
   - Messages lost if subscriber crashes (not persisted)
   - **Mitigation**: Limit Gateway replicas, implement Redis Cluster (Phase 2)

2. **Redis Streams Trimming**:
   - MAXLEN 50,000 messages (~25MB)
   - At 10,000 msg/s, fills in **5 seconds**
   - Trimming discards unprocessed messages if consumer lags
   - **Mitigation**: Increase MAXLEN, implement backpressure, monitor consumer lag

3. **No Backpressure Mechanism**:
   - Listeners publish without checking consumer lag
   - High-burst scenarios can overflow Redis Streams
   - **Mitigation**: Implement circuit breaker, monitor XPENDING

### Realistic Capacity

| Architecture Phase | Sustained msg/s | Peak msg/s | Infrastructure |
|--------------------|-----------------|------------|----------------|
| **Phase 1 (Current)** | 500-1,000 | 2,000 | Single Redis, CNPG 3-node |
| **Phase 2 (Redis Cluster)** | 3,000-5,000 | 8,000 | Redis Cluster 6-node, CNPG 3-node |
| **Phase 3+ (Kafka)** | 10,000+ | 20,000+ | Kafka 3-node, Redis Cluster, CNPG 5-node |

**Evidence for Phase 1 Limits**:
- Redis Pub/Sub throughput: ~100K messages/sec single instance
- With 26 Gateway pods: 100K / 26 = **3,846 msg/s theoretical max**
- Minus protocol overhead, serialization, network: **~1,000 msg/s realistic**
- Redis Streams with AOF persistence: ~50K writes/sec
- **Conclusion**: 500-1,000 msg/s is honest capacity estimate for Phase 1

---

## Service Scaling Strategies

### API Gateway

**Capacity**: 2,500 concurrent WebSocket connections per pod

**Calculation**:
- Memory per connection: ~128KB (goroutine + buffers)
- Pod memory limit: 512Mi
- Theoretical max: 4,000 connections
- **Safe limit with headroom: 2,500 connections**
- Per-overlay limit: 1,000 connections (prevent monopolization)

**HPA Configuration**:
```yaml
minReplicas: 2
maxReplicas: 20
metrics:
  - type: Resource
    resource:
      name: cpu
      target: {averageUtilization: 60}
  - type: Resource
    resource:
      name: memory
      target: {averageUtilization: 70}
  - type: Pods
    pods:
      metric:
        name: websocket_connections_active
      target:
        type: AverageValue
        averageValue: "2000"  # Scale before hitting 2,500
```

**Scaling Formula**:
```
Required Pods = ceil(Total WebSocket Connections / 2000) + 1 buffer

Examples:
- 5,000 connections  → 3 pods
- 10,000 connections → 6 pods
- 50,000 connections → 26 pods
```

**Example Scenarios**:
| Concurrent Overlays | Pods | Total Memory | Total CPU |
|---------------------|------|--------------|-----------|
| 1,000 | 2 (min) | 1Gi | 200m |
| 5,000 | 3 | 1.5Gi | 300m |
| 10,000 | 6 | 3Gi | 600m |
| 50,000 | 26 | 13Gi | 2.6 cores |

### Message Processor

**Capacity**: ~3,000 messages/second per pod (with emote enrichment)

**Bottlenecks**:
- Emote enrichment (7TV, BTTV, FFZ API calls)
- Emote cache miss rate (target: >95% hit rate)
- JSON serialization/deserialization
- Redis Pub/Sub publish latency

**HPA Configuration**:
```yaml
minReplicas: 3
maxReplicas: 10
metrics:
  - type: Resource
    resource:
      name: cpu
      target: {averageUtilization: 70}
  - type: Pods
    pods:
      metric:
        name: processor_consumer_lag  # XPENDING count
      target:
        type: AverageValue
        averageValue: "1000"  # Scale if lag exceeds 1,000 messages
```

**Scaling Formula**:
```
Required Pods = ceil(msg_rate / 3000) + 1 buffer

Examples:
- 1,000 msg/s → 3 pods (min)
- 5,000 msg/s → 3 pods (burst capacity)
- 10,000 msg/s → 5 pods
```

### Twitch Listener

**Capacity**: 500+ concurrent channels per pod

**Limits**:
- Twitch IRC rate limit: 20 JOIN/10 seconds
- Memory per channel: ~200KB (IRC connection overhead)
- Pod memory limit: 512Mi → ~2,500 channels theoretical max
- **Safe limit: 500 channels per pod**

**HPA Configuration**:
```yaml
minReplicas: 1
maxReplicas: 5
metrics:
  - type: Pods
    pods:
      metric:
        name: listener_active_sources_total
      target:
        type: AverageValue
        averageValue: "400"  # Scale before hitting 500
```

**Scaling Formula**:
```
Required Pods = ceil(Active Channels / 400) + 1 buffer

Examples:
- 100 channels → 1 pod
- 500 channels → 2 pods
- 2,000 channels → 6 pods
```

### YouTube Listener

**Capacity**: 50+ concurrent live streams per pod (with leader election)

**Limits**:
- YouTube API quota: 1,009,000 units/day default (`QUOTA_LIMIT_DAILY` env; original Google default is 10,000 — Google quota increase already in place)
- Poll interval: 2-5 seconds per stream
- Expected usage: 2,000-3,000 units/day with optimizations
- **Real bottleneck: API quota, not pod capacity**

**HPA Configuration**:
```yaml
minReplicas: 1
maxReplicas: 5
metrics:
  - type: Resource
    resource:
      name: cpu
      target: {averageUtilization: 70}
  - type: Pods
    pods:
      metric:
        name: listener_quota_usage_percentage
      target:
        type: AverageValue
        averageValue: "70"  # Scale if approaching quota limit
```

**Important**: YouTube Listener uses **leader election** for stream discovery (expensive 100-unit API calls) to prevent quota waste. Only one replica performs discovery; all replicas poll chat.

### Kick Listener

**Capacity**: 200+ concurrent channels per pod

**Limits**:
- Pusher WebSocket connections: No hard limit from platform
- Memory per channel: ~150KB (WebSocket connection)
- Pod memory limit: 512Mi → ~3,400 channels theoretical max
- **Safe limit: 200 channels per pod**

**HPA Configuration**:
```yaml
minReplicas: 1
maxReplicas: 5
metrics:
  - type: Pods
    pods:
      metric:
        name: listener_active_sources_total
      target:
        type: AverageValue
        averageValue: "150"
```

---

## Infrastructure Scaling

### PostgreSQL (CloudNativePG)

**Current**: 1 primary + 2 replicas (3-node cluster)

**Capacity**:
- Writes: Single primary, ~5,000 writes/second
- Reads: Load-balanced across 3 nodes, ~15,000 reads/second
- Connection pool: 20 connections per service × 10 services = 200 connections total

**Scaling Options**:

1. **Vertical Scaling** (Phase 1-2):
   - Increase primary instance size (more CPU/memory)
   - Increase storage (50Gi → 100Gi → 500Gi)

2. **Read Replicas** (Phase 2-3):
   - Add 2 more replicas (5-node cluster)
   - Route read-heavy queries to replicas
   - Primary handles only writes

3. **Connection Pooling** (Phase 2):
   - Deploy PgBouncer (already enabled in CNPG)
   - Transaction pooling mode (default)
   - Reduce connection overhead

**HPA Not Applicable** (PostgreSQL is stateful, vertical scaling only)

### Redis

**Current**: Single instance (Phase 1)

**Capacity**:
- Memory: 512Mi (docker-compose) / 1Gi (Kubernetes)
- Commands: ~50K ops/second with AOF persistence
- Pub/Sub: ~100K messages/second (degraded with many subscribers)

**Scaling Options**:

1. **Vertical Scaling** (Phase 1):
   - Increase memory: 1Gi → 4Gi → 16Gi
   - Increase CPU: 500m → 2 cores

2. **Redis Cluster** (Phase 2, **CRITICAL FOR >1K msg/s**):
   - Deploy 6-node cluster (3 primary + 3 replicas)
   - Shard by key (overlay:*, chat:raw partitioning)
   - Horizontal scaling for throughput

3. **Dedicated Instances** (Phase 3):
   - Separate Redis for Streams (chat:raw)
   - Separate Redis for Pub/Sub (overlay:*)
   - Separate Redis for caching (emotes, sessions)

**Migration to Redis Cluster**:
```yaml
# Phase 2: Redis Cluster StatefulSet
replicas: 6  # 3 primary + 3 replicas
slots: 16384 # Distributed across nodes
```

---

## Performance Bottlenecks

### 1. Redis Pub/Sub Fan-out (🔴 CRITICAL)

**Problem**: O(n) memory growth with Gateway replicas

**Example**:
- 10,000 msg/s × 26 Gateway pods = 260,000 deliveries/sec
- Each message duplicated 26 times in memory
- Redis Pub/Sub does not persist messages

**Mitigation**:
- **Phase 1**: Limit Gateway replicas to 5-10 (reduce fan-out)
- **Phase 2**: Deploy Redis Cluster with sharding
- **Phase 3**: Replace Pub/Sub with Kafka/NATS (persistent, partitioned)

### 2. Redis Streams Trimming

**Problem**: MAXLEN 50,000 messages fills in 5 seconds at 10K msg/s

**Mitigation**:
- Increase MAXLEN: 50K → 500K (monitor memory usage)
- Monitor consumer lag: `XPENDING chat:raw message-processors`
- Alert if lag > 10,000 messages
- Implement backpressure (stop publishing if lag too high)

### 3. Emote Enrichment Latency

**Problem**: External API calls to 7TV, BTTV, FFZ (50-200ms per request)

**Mitigation**:
- ✅ Emote cache (Redis, 1-hour TTL) - 95%+ hit rate
- ✅ 7TV EventAPI WebSocket for real-time updates
- Batch API requests (fetch multiple emotes in one call)
- Preload popular channel emotes

### 4. PostgreSQL Single-Primary Writes

**Problem**: All writes go to single primary instance

**Mitigation**:
- Use read replicas for queries (overlay config, user data)
- Batch writes when possible (bulk inserts)
- Cache frequently-read data (Redis)
- Consider eventual consistency for non-critical data

### 5. YouTube API Quota

**Problem**: 10,000 units/day limit (can hit limit with 5-10 concurrent streams)

**Mitigation**:
- ✅ Reserve-confirm-rollback quota tracking (99.95%+ accuracy)
- ✅ Smart optimizations (9,000+ units/day waste eliminated)
- ✅ Leader election (prevents duplicate search API calls)
- Request quota increase to 1,000,000 units/day from Google

---

## Capacity Planning

### Resource Requirements by Phase

#### Phase 1 (Current - 500-1,000 msg/s)

| Component | Replicas | CPU | Memory | Storage |
|-----------|----------|-----|--------|---------|
| **Application Services** | 15 | 1.5 cores | 2.5Gi | - |
| **PostgreSQL (CNPG)** | 3 | 1.5 cores | 3Gi | 150Gi |
| **Redis** | 1 | 500m | 1Gi | 20Gi |
| **LGTM Stack** | 5 | 2.5 cores | 9Gi | 230Gi |
| **TOTAL** | 24 pods | ~6 cores | ~16Gi | ~400Gi |

**Infrastructure**: Hetzner CX41 (8 vCPU, 32GB RAM, 240GB SSD) + Cloud Volumes

#### Phase 2 (Redis Cluster - 3,000-5,000 msg/s)

| Component | Replicas | CPU | Memory | Storage |
|-----------|----------|-----|--------|---------|
| **Application Services** | 25 | 3 cores | 5Gi | - |
| **PostgreSQL (CNPG)** | 5 | 3 cores | 8Gi | 250Gi |
| **Redis Cluster** | 6 | 3 cores | 12Gi | 120Gi |
| **LGTM Stack** | 5 | 2.5 cores | 9Gi | 230Gi |
| **TOTAL** | 41 pods | ~12 cores | ~34Gi | ~600Gi |

**Infrastructure**: 3× Hetzner CX41 (24 vCPU, 96GB RAM total)

#### Phase 3+ (Kafka - 10,000+ msg/s)

| Component | Replicas | CPU | Memory | Storage |
|-----------|----------|-----|--------|---------|
| **Application Services** | 50 | 6 cores | 10Gi | - |
| **PostgreSQL (CNPG)** | 5 | 5 cores | 16Gi | 500Gi |
| **Redis Cluster** | 6 | 3 cores | 12Gi | 120Gi |
| **Kafka Cluster** | 3 | 3 cores | 9Gi | 500Gi |
| **LGTM Stack** | 8 | 4 cores | 16Gi | 500Gi |
| **TOTAL** | 72 pods | ~21 cores | ~63Gi | ~1.6Ti |

**Infrastructure**: 5× Hetzner CX41 (40 vCPU, 160GB RAM total)

### Cost Optimization

| Phase | Infrastructure | Monthly Cost (Hetzner) |
|-------|----------------|------------------------|
| **Phase 1** | 1× CX41 + Volumes | ~€50 |
| **Phase 2** | 3× CX41 | ~€90 |
| **Phase 3** | 5× CX41 | ~€150 |

**Cost Breakdown**:
- CX41 (8 vCPU, 32GB RAM, 240GB SSD): €30/month
- Cloud Volume (100GB): €5/month
- Backup storage: €10/month

---

## Optimization Techniques

### Caching Strategy

1. **Emote Cache** (Redis, 1-hour TTL):
   - 7TV, BTTV, FFZ emotes cached per channel
   - 95%+ cache hit rate
   - Real-time updates via 7TV EventAPI WebSocket

2. **OAuth Token Cache** (Redis, until expiry):
   - YouTube OAuth tokens cached per user
   - Automatic refresh when expired

3. **Channel Metadata Cache** (Redis, 5-minute TTL):
   - Kick chatroom IDs
   - YouTube video IDs (for live stream discovery)

### Connection Pooling

1. **PostgreSQL** (pgx):
   - MaxConns: 20 per service
   - MinConns: 5 (keep warm)
   - Connection recycling: 1 hour
   - Health checks: 1 minute

2. **Redis** (go-redis):
   - PoolSize: 50 per service
   - MinIdleConns: 10 (keep warm)
   - Connection recycling: 10 minutes

### Message Age Filtering

- **Message Processor**: Ignores messages >60 seconds old
- Prevents processing stale messages from Redis Streams
- Configurable via `MESSAGE_AGE_CUTOFF_SECONDS`

### Leader Election

- **YouTube Listener**: Only one replica performs stream discovery (100 units per search)
- **Source Manager**: Coordinates leadership via Redis locks
- Prevents quota waste when running multiple replicas

---

## Summary

**Current Capacity** (Phase 1):
- ✅ 500-1,000 messages/second sustained
- ✅ 2,500 WebSocket connections per Gateway pod
- ✅ 50+ concurrent YouTube streams (quota-limited)
- ✅ 500+ concurrent Twitch channels per pod
- ✅ 200+ concurrent Kick channels per pod

**Known Limitations**:
- 🔴 Redis Pub/Sub fan-out (O(n) with Gateway replicas)
- 🔴 Single Redis instance (no clustering in Phase 1)
- ⚠️ YouTube API quota (10,000 units/day, request increase to 1M)
- ⚠️ Redis Streams MAXLEN (50K messages, ~5s at 10K msg/s)

**Scaling Roadmap**:
- **Phase 2**: Deploy Redis Cluster → 3,000-5,000 msg/s
- **Phase 3**: Deploy Kafka → 10,000+ msg/s
- **Phase 4**: Global CDN, multi-region deployment

**For detailed deployment steps**, see:
- [02-DEPLOYMENT.md](./02-DEPLOYMENT.md) - Kubernetes deployment
- [04-OBSERVABILITY.md](./04-OBSERVABILITY.md) - Metrics and monitoring
- [QUICK-REF-SCALING.md](../llm-guides/QUICK-REF-SCALING.md) - Quick reference for scaling operations
