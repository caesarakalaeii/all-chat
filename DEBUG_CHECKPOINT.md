# Debug Session Checkpoint - Chat Messages Not Appearing

**Date:** 2025-11-14
**Session Goal:** Debug why chat messages from Twitch aren't appearing on the frontend overlay at https://allchat.caes.ar

---

## 🐛 Root Cause Identified & Fixed

### The Bug
The API Gateway WebSocket handler was using the HTTP request context (`c.Request.Context()`) for Redis Pub/Sub subscription. When the HTTP handler returned, this context was **cancelled immediately**, stopping the Redis subscription listener from receiving messages.

### The Fix (COMMITTED & DEPLOYED)
**File:** `services/api-gateway/handlers/websocket.go`
**Lines Changed:** 126, 138
**Commit:** `b3d87f2` - "fix(api-gateway): use background context for Redis subscription"

```go
// BEFORE (buggy):
if err := h.subscriber.Subscribe(ctx, overlayID); err != nil {
h.wsManager.AddConnection(ctx, wsConn)

// AFTER (fixed):
if err := h.subscriber.Subscribe(context.Background(), overlayID); err != nil {
h.wsManager.AddConnection(context.Background(), wsConn)
```

**Deployment Status:** ✅ CI/CD pipeline completed, new pods running (35-49 seconds old at last check)

---

## ✅ Verified Working Components

1. **Twitch Bot Connection** ✅
   - Script: `/home/moersener/Hobby/all-chat/test_twitch_chat.py`
   - Successfully connects to Twitch IRC
   - Sends messages to #caesarlp channel
   - 5 test messages sent successfully

2. **Backend Message Flow** ✅ (verified earlier in session)
   - Twitch Listener → receives IRC messages
   - Publishes to Redis Stream `chat:raw`
   - Message Processor → consumes from stream
   - Enriches with emotes (7TV, BTTV, FFZ)
   - Publishes to `overlay:23ca3940-c4c0-4ddf-85df-6b7cfe19f629` via Redis Pub/Sub
   - Redis PUBLISH returned "2" (two subscribers)

3. **WebSocket Connection** ✅
   - Frontend successfully connects to `wss://allchat.caes.ar/ws/overlay/23ca3940-c4c0-4ddf-85df-6b7cfe19f629`
   - Console log shows: `[WebSocket] Connected`
   - UI shows: "● Connected" (green)
   - Status footer shows: "Status: Connected" (green)

---

## ❌ Current Problem

**Messages still not appearing in the overlay preview despite:**
- WebSocket showing "Connected"
- Test messages sent successfully
- Backend processing confirmed earlier
- Context fix deployed

**Evidence:**
- Screenshot saved: `/home/moersener/Hobby/all-chat/.playwright-mcp/overlay-test-screenshot.png`
- Shows "Waiting for messages..." with "Messages: 0"
- Status: Connected (green)
- No chat messages visible

---

## 🔍 Next Debugging Steps

### 1. Verify Message Processor is Publishing
Check if messages are actually reaching the overlay-specific Redis channel:

```bash
export KUBECONFIG=~/.kube/caesar-clusters
kubectl -n allchat exec deployment/redis -- redis-cli MONITOR | grep "overlay:23ca3940"
```

Then send test message:
```bash
python3 /home/moersener/Hobby/all-chat/test_twitch_chat.py
```

### 2. Check API Gateway Subscription Logs
Verify the API Gateway is subscribing and receiving messages:

```bash
kubectl -n allchat logs deployment/api-gateway --tail=100 | grep -E "Subscribed|Broadcast|message"
```

### 3. Check API Gateway Message Handler
Review the message handler code in `services/api-gateway/cmd/main.go` lines 82-89:

```go
messageHandler := func(overlayID string, message []byte) {
    count := wsManager.BroadcastToOverlay(overlayID, message)
    log.Debug("Broadcast message to overlay",
        zap.String("overlay_id", overlayID),
        zap.Int("connections", count),
    )
}
```

**Potential Issue:** The handler uses `log.Debug()` which may not show in logs unless `LOG_LEVEL=debug`

### 4. Check WebSocket Manager Broadcast
Review `services/api-gateway/websocket/manager.go` `BroadcastToOverlay()` method to ensure messages are being sent to connections.

### 5. Check Frontend WebSocket Message Handling
Review the frontend code handling WebSocket messages - there may be a parsing or rendering issue.

---

## 📁 Key Files & Locations

### Source Code
- **Main Repo:** `/home/moersener/Hobby/all-chat`
- **Deployment Configs:** `/home/moersener/Hobby/caesar-deployment/all-chat`

### Modified Files (in this session)
- `services/api-gateway/handlers/websocket.go` (lines 126, 138)

### Test Script
- `/home/moersener/Hobby/all-chat/test_twitch_chat.py`
  - Bot: caesarlp
  - Channel: #caesarlp
  - Sends 5 test messages

### Environment
- **Cluster:** k3d-caesar-cluster (remote, not local)
- **Namespace:** allchat
- **Overlay ID:** `23ca3940-c4c0-4ddf-85df-6b7cfe19f629`
- **Frontend URL:** https://allchat.caes.ar
- **Overlay Preview:** https://allchat.caes.ar/overlays/23ca3940-c4c0-4ddf-85df-6b7cfe19f629/preview

### Credentials
- Twitch credentials in `.env` file (not committed)
- GitHub Container Registry: `ghcr.io/caesarakalaeii/allchat-api-gateway:main`

---

## 🔧 Useful Commands

### Check Pod Status
```bash
export KUBECONFIG=~/.kube/caesar-clusters
kubectl -n allchat get pods -l app=api-gateway
```

### Send Test Messages
```bash
cd /home/moersener/Hobby/all-chat
python3 test_twitch_chat.py
```

### Monitor Redis Pub/Sub
```bash
kubectl -n allchat exec deployment/redis -- redis-cli SUBSCRIBE "overlay:23ca3940-c4c0-4ddf-85df-6b7cfe19f629"
```

### Check API Gateway Logs
```bash
kubectl -n allchat logs deployment/api-gateway --tail=100 --follow
```

### Check Message Processor Logs
```bash
kubectl -n allchat logs deployment/message-processor --tail=100 --follow
```

---

## 📊 Architecture Flow

```
Twitch IRC (#caesarlp)
    ↓
Twitch Listener (receives messages)
    ↓
Redis Stream (chat:raw)
    ↓
Message Processor (normalizes + enriches)
    ↓
Redis Pub/Sub (overlay:23ca3940-c4c0-4ddf-85df-6b7cfe19f629)
    ↓
API Gateway (subscribes with context.Background()) ← FIX APPLIED
    ↓
WebSocket (wss://allchat.caes.ar/ws/overlay/...)
    ↓
Frontend (React/Next.js) ← CONNECTED BUT NOT RECEIVING MESSAGES
```

---

## 🚨 Known Issues

1. **kubectl Context Issue:** Local kubectl having intermittent connection issues with error:
   ```
   couldn't get current server API group list: Get "http://localhost:8080/api?timeout=32s"
   ```
   This is a local kubectl config issue, not related to the application.

2. **WebSocket Reconnecting:** Before the fix, WebSocket was closing with code 1006 and reconnecting. This is now resolved.

3. **LOG_LEVEL:** API Gateway may be logging at INFO level, so DEBUG logs for message broadcast aren't visible.

---

## 💡 Hypothesis for Remaining Issue

Based on the symptoms, the most likely remaining issues are:

1. **Log Level Issue:** Messages ARE being broadcast but not logged (use `LOG_LEVEL=debug`)
2. **Frontend Parsing:** WebSocket receives messages but frontend fails to parse/render them
3. **Message Format Mismatch:** Frontend expects different JSON structure than what's being sent
4. **Another Context Issue:** There may be another place using request context instead of background context

---

## 📝 Session Notes

- Used Playwright MCP to navigate and test the frontend
- Confirmed WebSocket connection established
- Sent 3 batches of 5 test messages each during session
- All backend components appeared healthy
- API Gateway pods restarted successfully with new image
- Fix was clean and minimal (2 lines changed, 2 comments added)

---

## 🎯 Recommended Next Session Actions

1. **Enable DEBUG logging:**
   ```bash
   kubectl -n allchat set env deployment/api-gateway LOG_LEVEL=debug
   ```

2. **Send test message and check logs in real-time:**
   ```bash
   # Terminal 1
   kubectl -n allchat logs deployment/api-gateway --follow | grep -v health

   # Terminal 2
   python3 test_twitch_chat.py
   ```

3. **Check WebSocket raw messages in browser console:**
   - Open browser dev tools
   - Go to Network → WS tab
   - Filter for overlay WebSocket
   - Send test message
   - Check if messages appear in WebSocket frames

4. **If messages are in WebSocket but not displaying:**
   - Issue is in frontend React code
   - Check `frontend/` for WebSocket message handler
   - Look for message parsing/rendering logic

---

## 📚 References

- **Main Documentation:** `GETTING_STARTED.md`
- **Architecture Docs:** `services/api-gateway/README.md`
- **Deployment Docs:** `/home/moersener/Hobby/caesar-deployment/all-chat/README.md`

---

**End of Checkpoint**

Resume debugging by:
1. Checking if messages are in WebSocket frames (browser dev tools)
2. Enabling DEBUG logging on API Gateway
3. Monitoring logs while sending test messages
