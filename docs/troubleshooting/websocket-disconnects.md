# Troubleshooting: WebSocket Disconnects

API Gateway WebSocket connection issues and message delivery failures.

---

## Client Cannot Connect

### WebSocket Upgrade Failed

**Symptom**: Browser console shows "WebSocket connection failed"

**Check**:
```javascript
// Browser console
ws = new WebSocket('ws://localhost:8080/ws/overlay/YOUR_OVERLAY_ID')
// Should connect successfully
```

**Solutions**:
1. Verify API Gateway is running: `curl http://localhost:8080/health/ready`
2. Check overlay ID is valid (exists in database)
3. Verify CORS allows origin (check API Gateway logs)

**File**: `services/api-gateway/websocket/manager.go:HandleWebSocket()`

### Connection Drops Frequently

**Symptom**: WebSocket connects then disconnects after 30-60 seconds

**Causes**:
1. No ping/pong keepalive (client should send ping every 30s)
2. Network proxy timeout (corporate firewall)
3. API Gateway pod restarted (check pod events)

**Solution**:
```javascript
// Add client-side keepalive
setInterval(() => {
  ws.send(JSON.stringify({type: 'ping'}))
}, 30000)
```

---

## Messages Not Appearing

### Redis Pub/Sub Not Subscribed

**Symptom**: WebSocket connected but no messages received

**Check subscription**:
```bash
# Check API Gateway subscribed to overlay channel
redis-cli PUBSUB CHANNELS overlay:*

# Should see: overlay:YOUR_OVERLAY_ID
```

**Solution**: Check API Gateway logs for subscription errors

**File**: `services/api-gateway/websocket/manager.go:SubscribeOverlay()`

### Message Processor Not Publishing

**Symptom**: Messages in Redis Stream but not Pub/Sub

**Check**:
```bash
# Check messages in stream
redis-cli XREAD COUNT 10 STREAMS chat:raw 0

# Check if Message Processor is consuming
redis-cli XINFO GROUPS chat:raw
# Should show "message-processors" consumer group with lag=0
```

**File**: `services/message-processor/publisher/pubsub.go:Publish()`

---

## Related Documentation

- [decision-tree.md](./decision-tree.md) - High-level triage
- [api-gateway/README.md](../../services/api-gateway/README.md) - WebSocket documentation
- [QUICK-REF-REDIS-OPERATIONS.md](../llm-guides/QUICK-REF-REDIS-OPERATIONS.md) - Redis debugging
