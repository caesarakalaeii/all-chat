# Runbook: Recover from Redis Outage

**Scenario**: Redis pod crashed or became unresponsive

**Time Estimate**: 10-20 minutes
**Risk**: Medium (message loss during outage, but services recover automatically)

---

## Immediate Actions

### 1. Check Redis Status

```bash
# Check pod status
kubectl get pods -n allchat -l app=redis

# Check logs
kubectl logs -n allchat -l app=redis --tail=100

# Test connection
kubectl exec -n allchat deployment/redis -- redis-cli ping
# Expected: PONG
```

### 2. Restart Redis (If Crashed)

```bash
# Delete pod (StatefulSet recreates it)
kubectl delete pod redis-0 -n allchat

# Wait for ready
kubectl wait --for=condition=Ready pod/redis-0 -n allchat --timeout=60s

# Verify data persisted (AOF recovery)
kubectl exec -n allchat redis-0 -- redis-cli DBSIZE
# Should show keys recovered
```

---

## Recovery Steps

### 1. Verify Services Reconnected

All services automatically reconnect to Redis after outage. Check logs:

```bash
# Message Processor (Redis Streams consumer)
kubectl logs -n allchat deployment/message-processor | grep -i redis

# API Gateway (Redis Pub/Sub)
kubectl logs -n allchat deployment/api-gateway | grep -i "redis\|subscribe"

# Listeners
kubectl logs -n allchat deployment/twitch-listener | grep -i redis
```

**Expected**: "Redis connection established" messages

### 2. Check Message Flow

```bash
# Verify messages flowing through Redis Streams
redis-cli XINFO STREAM chat:raw

# Check consumer group lag
redis-cli XPENDING chat:raw message-processors
# Should show lag=0 or low numbers

# Check Pub/Sub channels active
redis-cli PUBSUB CHANNELS overlay:*
```

### 3. Restart Services (If Not Auto-Recovering)

```bash
# Restart Message Processor (Redis Streams consumer)
kubectl rollout restart deployment/message-processor -n allchat

# Restart API Gateway (Redis Pub/Sub)
kubectl rollout restart deployment/api-gateway -n allchat

# Restart Listeners (Redis publishers)
kubectl rollout restart deployment/twitch-listener -n allchat
kubectl rollout restart deployment/youtube-listener -n allchat
```

---

## Data Loss Assessment

### What Was Lost

**During outage**:
- ✅ **Redis Streams (chat:raw)**: Persisted with AOF, **NO DATA LOSS**
- ❌ **Redis Pub/Sub (overlay:*)**: NOT persisted, **messages lost during outage**
- ✅ **Database**: Unaffected, **NO DATA LOSS**

**Expected Impact**:
- Messages sent during outage (1-5 minutes) not delivered to overlays
- Overlays show connection drop, automatically reconnect when Redis recovers
- User experience: 1-5 minute gap in chat messages

### What Was Preserved

- ✅ User accounts, overlays, chat sources (PostgreSQL)
- ✅ YouTube quota tracking (PostgreSQL)
- ✅ OAuth tokens (PostgreSQL)
- ✅ Redis Streams data (AOF recovery)
- ✅ Emote cache (rehydrated from external APIs if lost)

---

## Prevention

### Enable Redis Cluster (Phase 2)

**For HA**, deploy 6-node Redis Cluster (3 primary + 3 replicas):
```bash
# Deploy Redis Cluster (planned Phase 2)
kubectl apply -f deployments/k8s/base/redis-cluster/
```

### Monitor Redis Health

**Alerts**:
- Redis pod down > 2 minutes → PagerDuty
- Redis memory > 80% → Slack warning
- Redis CPU > 80% → Slack warning

---

## Related Documentation

- [02-DEPLOYMENT.md](../../architecture/02-DEPLOYMENT.md) - Redis deployment
- [QUICK-REF-REDIS-OPERATIONS.md](../../llm-guides/QUICK-REF-REDIS-OPERATIONS.md) - Redis debugging
