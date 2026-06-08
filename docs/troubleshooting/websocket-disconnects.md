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
1. Network proxy timeout (corporate firewall, NAT/idle reaping)
2. API Gateway pod restarted (check pod events)
3. Client failing the gateway's read deadline (`PongWait`, 60s)

**Keepalive contract** (already implemented — do not re-add ad-hoc keepalive):
- The gateway sends a **protocol-level** ping every `PingInterval` (30s) and
  closes the connection if no pong/data arrives within `PongWait` (60s). Browsers
  auto-answer protocol pings, so a healthy tab stays connected on its own.
- The client (`useOverlayStream`, and the legacy `WebSocketClient`) **also** sends
  an **app-level** `{"type":"ping"}` every 20s; the gateway echoes
  `{"type":"pong"}` (`connection.go handleMessage`). This is what lets the client
  detect a dead path even on a silent channel — see the next entry.

### Connection Silently Dies / Indicator Stuck on "Live"

**Symptom**: messages stop arriving but the `/view` badge keeps showing **Live**;
no reconnect happens. Worse on hardened/background tabs (e.g. LibreWolf).

**Cause**: a **half-open** socket — the network path broke (Wi-Fi blip, NAT/proxy
idle timeout, sleep/wake) but no TCP FIN/RST arrived, so `ws.readyState` stays
`OPEN` and `onclose` never fires. Browsers never surface protocol ping/pong to
JS, so the page can't tell the link died.

**How it's handled** (`useOverlayStream` / `WebSocketClient`):
- Any inbound frame (chat/status/**pong**) refreshes a `lastActivity` timestamp.
- A watchdog checks every 5s; after `LIVENESS_TIMEOUT_MS` (40s) of total silence
  the socket is declared dead, the badge flips to **Reconnecting**, and the client
  force-reconnects with `?since=` (the gateway replays the buffered gap).
- `online` / `visibilitychange→visible` force an immediate reconnect, so a
  backgrounded tab recovers the moment it's focused.
- `isConnected()` / `connectionStatus` are liveness-aware, so the indicator can't
  report a zombie as connected.

If you see this regress, confirm the gateway still echoes `pong` to a client
`ping` and that the client timers aren't being throttled to death (a fully
suspended tab only recovers on the visibility/online events above).

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
