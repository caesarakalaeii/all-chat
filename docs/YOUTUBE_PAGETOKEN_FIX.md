# YouTube gRPC Streaming pageToken Fix

**Date:** 2026-01-15
**Status:** FIX IMPLEMENTED - Ready for Testing
**Priority:** CRITICAL

---

## Problem Summary

The YouTube `liveChatMessages.streamList` gRPC API was disconnecting every ~10 seconds, causing excessive quota consumption that made the service unsustainable.

### Before Fix
- **Stream duration:** ~10 seconds per connection
- **Reconnection rate:** 6 connections/minute
- **Quota cost:** 30 units/minute (5 units × 6 connections)
- **Daily usage:** 43,200 units/day for ONE 24/7 stream
- **Daily quota limit:** 10,000 units
- **Result:** Service unusable - quota exhausted in 5.5 hours

### After Fix (Expected)
- **Stream duration:** Hours (as intended by YouTube)
- **Reconnection rate:** <1 connection/hour
- **Quota cost:** ~5 units per stream startup + minimal reconnections
- **Daily usage:** ~120 units/day per stream (98% reduction!)
- **Result:** Can support 80+ streams with base quota, 8,000+ with increased quota

---

## Root Cause

The issue was in **`services/youtube-listener/streams/poller.go`** lines 361-370.

### What Was Wrong

The code was loading a cached `pageToken` from Redis **on every connection**:

```go
if pageToken == "" && p.tokenStore != nil {
    if storedToken, tokenErr := p.tokenStore.Get(ctx, liveChatID); tokenErr != nil {
        // ...
    } else if storedToken != "" {
        pageToken = storedToken  // ❌ WRONG!
    }
}
```

### Why This Caused 10-Second Disconnects

When you provide a `pageToken` to YouTube's gRPC streaming API:
1. YouTube interprets this as "send me messages SINCE this token"
2. Server sends a batch of "catch-up" messages
3. Server closes the stream after sending the catch-up batch (~10 seconds)
4. Client reconnects with the NEW token from the last response
5. Loop repeats → 6 reconnections/minute

This is NOT how gRPC streaming is intended to work!

### Correct Behavior (Per YouTube Documentation)

According to the [official Python demo](https://developers.google.com/youtube/v3/live/streaming-live-chat):

```python
next_page_token = None  # ✅ Start with NO token
while True:
    request = LiveChatMessageListRequest(
        live_chat_id=sys.argv[2],
        page_token=next_page_token,  # First time: None
    )
    for response in stub.StreamList(request, metadata=metadata):
        print(response)
        next_page_token = response.next_page_token
        if not next_page_token:
            break  # Stream ended
```

**Key insight:**
- **First connection:** NO pageToken
- **Within stream:** Token tracked in-memory from responses
- **Only for recovery:** Use last token after unexpected disconnect

---

## The Fix

### Changes Made

**File:** `services/youtube-listener/streams/poller.go`

1. **Removed pageToken loading** (lines 361-370)
   - No longer loads cached token from Redis on every connection
   - Token is tracked in-memory during streaming session

2. **Removed pageToken persistence** (lines 409-415)
   - No longer saves token to Redis after each response
   - Prevents stale tokens from being reused

3. **Removed token clearing** (line 383-386)
   - No longer clears token when stream goes offline
   - Unnecessary since we're not persisting tokens

### How It Works Now

```
1. New stream detected → NextPageToken = "" (empty)
2. First poll → Connect WITHOUT pageToken
3. YouTube sends:
   - Recent chat history
   - Keeps stream OPEN
   - Pushes new messages in real-time
4. Each response includes nextPageToken
5. Update in-memory: stream.NextPageToken = response.nextPageToken
6. Continue receiving responses on SAME connection
7. Only reconnect on:
   - Network error
   - Context cancellation
   - Stream naturally ends
```

### Benefits

✅ **Dramatically reduced quota usage** (98% reduction)
✅ **True real-time streaming** (no 10-second delays)
✅ **Aligns with YouTube's intended API usage**
✅ **Simpler code** (removed unnecessary persistence)
✅ **Lower latency** (fewer reconnections)

---

## Testing Plan

### 1. Deploy to Cluster
```bash
# Commit and push changes
git add services/youtube-listener/streams/poller.go
git commit -m "fix: YouTube gRPC streaming pageToken issue"
git push

# Watch deployment
kubectl rollout status deployment/youtube-listener -n allchat
```

### 2. Monitor Stream Duration
```bash
# Watch logs for stream duration
kubectl logs -f -n allchat -l app=youtube-listener | grep "gRPC stream closed"
```

**Expected log output:**
```
"reason":"eof",
"duration":"3h45m12s",     # ✅ HOURS, not seconds!
"responses_received":450,   # ✅ Multiple responses
"with_page_token":false     # ✅ First connection without token
```

**vs. Old behavior:**
```
"reason":"eof",
"duration":"10.388s",       # ❌ Only 10 seconds
"responses_received":1,     # ❌ Only 1 response
"with_page_token":true      # ❌ Always with token
```

### 3. Monitor Quota Usage
```bash
# Check quota consumption
kubectl exec -n allchat youtube-listener-<pod> -- \
  wget -qO- http://localhost:8086/quota/status | jq .global

# Expected: ~2-5 units per hour (vs. 150 units/hour before)
```

### 4. Test Reconnection Behavior

**Simulate pod restart:**
```bash
kubectl delete pod -n allchat youtube-listener-<pod>
# Stream should resume WITHOUT cached pageToken
```

**Verify logs show:**
- ✅ First connection: `"with_page_token":false`
- ✅ Stream stays open for hours
- ✅ Messages received continuously

---

## Metrics to Track

| Metric | Before Fix | After Fix (Expected) |
|--------|------------|---------------------|
| Stream Duration | ~10 seconds | >1 hour |
| Reconnections/Hour | 360 | <1 |
| Quota/Hour (per stream) | 150 units | 2-5 units |
| Quota/Day (per stream) | 43,200 units | ~120 units |
| Streams Supported (10K quota) | 0.23 | 83 |
| Streams Supported (1M quota) | 23 | 8,333 |
| Message Latency | ~5-10 seconds | <1 second |

---

## Rollback Plan

If the fix causes issues:

1. **Revert commit:**
   ```bash
   git revert HEAD
   git push
   ```

2. **Watch deployment rollback:**
   ```bash
   kubectl rollout status deployment/youtube-listener -n allchat
   ```

3. **File issue with findings**

---

## Related Documents

- **Investigation Checkpoint:** `YOUTUBE_GRPC_STREAMING_CHECKPOINT.md`
- **Official API Docs:** `docs/YOUTUBE_STREAM_LIST.txt`
- **Python Demo:** `docs/YOUTUBE_GRPC_EXAMPLE.txt`
- **Service README:** `services/youtube-listener/README.md`

---

## Success Criteria

✅ Streams stay open for >1 hour without disconnecting
✅ Quota usage <200 units/day per stream
✅ Can support 50+ concurrent streams with base quota
✅ Can support 1000+ streams with 1M quota increase
✅ Message latency <2 seconds

---

**Status:** FIX IMPLEMENTED - Awaiting deployment and validation
**Next:** Deploy to cluster and monitor for 24 hours
