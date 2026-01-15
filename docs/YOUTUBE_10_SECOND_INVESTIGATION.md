# YouTube gRPC 10-Second Disconnect Investigation

**Date:** 2026-01-15
**Status:** ACTIVE INVESTIGATION - Testing Python Demo Parameters
**Priority:** CRITICAL

---

## Current Situation

Despite fixing the pageToken caching issue, YouTube gRPC streams still close after **exactly ~10 seconds**. This happens with both:
- Low-activity streams (2 responses, ~10s)
- **High-activity streams (5-14 responses, ~10s)** ← Ludwig's stream

### Observed Behavior (Ludwig's Stream - HIGH ACTIVITY)

```json
{"response_num":1, "messages_count":75,  "next_page_token":"GITn99_cjpID",  "has_next_token":true}
{"response_num":2, "messages_count":1,   "next_page_token":"GODTg-HcjpID",  "has_next_token":true}
{"response_num":3, "messages_count":1,   "next_page_token":"GLmW2eHcjpID",  "has_next_token":true}
{"response_num":4, "messages_count":1,   "next_page_token":"GIj-1uPcjpID",  "has_next_token":true}
{"response_num":5, "messages_count":1,   "next_page_token":"GNOh2OPcjpID",  "has_next_token":true}
{"response_num":6, "messages_count":1,   "next_page_token":"GPfGm-TcjpID",  "has_next_token":true}
{"response_num":7, "messages_count":0,   "next_page_token":"GPfGm-TcjpID",  "has_next_token":true} ← Same token!
// EOF after ~10 seconds
{"response_num":1, "messages_count":75,  ...} ← New connection
```

**Key Observations:**
- ✅ `with_page_token: false` - Our fix is working
- ✅ Multiple responses per connection - Stream is active
- ✅ All responses have `nextPageToken` - YouTube NOT signaling end
- ❌ Connection closes with EOF after 10 seconds - YouTube closes it
- 🔄 Response 7 has 0 messages and duplicate token - Possible signal?

---

## What External Source Says (CRITICAL)

According to production implementations:

> "In a correct implementation, streams have durations on the order of **HOURS** (until the live stream actually ends or a network error occurs) instead of 10 seconds."

> "Quota usage drops dramatically – roughly **5 units for opening the stream** and negligible extra cost for maintaining it"

**This confirms:** 10-second disconnects are **NOT normal**. Something is wrong with our implementation.

---

## Fixes Attempted So Far

### ✅ FIXED: pageToken Caching (3 commits)
- Removed TokenStore loading
- Removed StreamState pageToken
- Clear pageToken after stream closes
- **Result:** `with_page_token: false` ✅ BUT still 10s disconnects ❌

### 🧪 TESTING: Request Parameters
- Removed `MaxResults` entirely
- Removed unused imports
- Added detailed debug logging
- **Current test:** Match Python demo exactly (maxResults=20, part=["snippet"])

### ✅ FIXED: OAuth Configuration
- Added YOUTUBE_API_KEY to youtube-listener
- Added OAuth token for Ludwig's channel
- **Result:** Can stream Ludwig's high-activity chat ✅

### ✅ VERIFIED: gRPC Keepalive
```go
Time:                20 * time.Second,  // Ping every 20s
Timeout:             10 * time.Second,  // Wait 10s for ack
PermitWithoutStream: true,              // Allow idle pings
```
- **Result:** Properly configured ✅ BUT still 10s disconnects ❌

---

## Theories Being Investigated

### Theory 1: Request Parameters (TESTING NOW)
**Hypothesis:** Python demo uses `max_results=20` and `part=["snippet"]`, we were using different values.

**Testing:**
- Deploy with maxResults=20 (matching demo)
- Only request "snippet" part (not "id" or "authorDetails")
- Monitor if duration increases

**Expected:** If Python demo works, matching it exactly should work.

### Theory 2: Infrastructure/Proxy Interference
**Hypothesis:** Something between us and `youtube.googleapis.com:443` is closing connections.

**Evidence:**
- Kubernetes ingress-nginx closes gRPC after 60s (we're at 10s, so not this)
- Load balancers can have gRPC timeouts
- Could be Google Cloud Load Balancer or network infrastructure

**How to test:**
- Run test client from **outside Kubernetes** (local machine)
- Monitor network traffic with tcpdump to see actual GOAWAY frames
- Check if YouTube sends RST_STREAM or connection-level close

### Theory 3: Undocumented YouTube Limitation
**Hypothesis:** YouTube intentionally closes streams after N responses or 10s.

**Evidence:**
- No official documentation about stream duration
- Community implementations advertise "auto-reconnect" feature
- Python demo has `while True` loop (suggests reconnects expected?)

**Counter-evidence:**
- External source says streams last "hours"
- Documentation promises "long-lived connection"
- This would defeat the entire purpose of streaming

### Theory 4: Missing Metadata/Headers
**Hypothesis:** We're missing some required metadata header.

**Current metadata:**
```go
md := metadata.New(map[string]string{
    "x-goog-request-params": fmt.Sprintf("live_chat_id=%s", liveChatID),
})
```

**Python demo uses:**
- `x-goog-api-key` for API key auth
- `authorization: Bearer <token>` for OAuth

**How to test:**
- Verify our gRPC credentials are actually sending auth headers
- Add additional metadata headers if documented

---

## Python Demo Analysis

The official Python demo structure:

```python
next_page_token = None
while True:  # ← Why is this needed if streams last hours?
    request = LiveChatMessageListRequest(
        part=["snippet"],  # ← Only snippet!
        live_chat_id=sys.argv[2],
        max_results=20,  # ← Only 20!
        page_token=next_page_token,
    )
    for response in stub.StreamList(request, metadata=metadata):
        print(response)
        next_page_token = response.next_page_token
        if not next_page_token:  # ← Break if token is empty
            break
```

**Questions:**
1. Does the `for response in` loop naturally end after a few responses?
2. Is the `while True` there BECAUSE streams close every few seconds?
3. Or is it for handling stream end + reconnection?

**If streams last hours**, the `while True` should almost never execute - the inner `for` loop would run for hours until the stream ends.

**If streams close every 10s**, the `while True` would execute frequently - which matches our observation!

---

## What We Know FOR SURE

### Facts
1. ✅ Our pageToken handling is now correct
2. ✅ Ludwig's stream is VERY active (75+ messages per batch)
3. ✅ All responses have valid `nextPageToken`
4. ✅ gRPC connection established successfully
5. ❌ Stream closes with EOF after exactly ~10 seconds EVERY TIME
6. ❌ No error messages, no offline signal, just clean EOF

### Quota Impact
**Current (10s disconnects):**
- 6 connections/minute × 5 units = 30 units/min
- 1,800 units/hour
- 43,200 units/day (for 24/7 streaming)
- **Exhausts daily quota in 5.5 hours**

**If we achieve 5-minute connections:**
- 12 connections/hour × 5 units = 60 units/hour
- 1,440 units/day (24/7)
- **Within quota limit! ✅**

**If we achieve 1-hour connections:**
- 24 connections/day × 5 units = 120 units/day
- **Completely sustainable! ✅**

---

## Next Steps

### Immediate (In Progress)
- [x] Deploy with Python demo exact parameters (maxResults=20, part=["snippet"])
- [ ] Monitor Ludwig's stream for 5 minutes
- [ ] Check if duration changes or response count changes

### If Python Demo Params Work
- ✅ Document exact working configuration
- ✅ Update all streams to use these params
- ✅ Monitor quota usage for 24 hours
- ✅ Declare RESOLVED

### If Python Demo Params DON'T Work
- [ ] Test from outside Kubernetes (local machine)
- [ ] Capture network traffic (tcpdump) to see actual close frames
- [ ] Try REST polling as fallback (may actually be cheaper!)
- [ ] File issue with Google YouTube API team
- [ ] Request quota increase as stopgap

---

## Resources

**Official Documentation:**
- [streamList API](https://developers.google.com/youtube/v3/live/docs/liveChatMessages/streamList)
- [Streaming Live Chat Guide](https://developers.google.com/youtube/v3/live/streaming-live-chat)
- [gRPC Keepalive Guide](https://grpc.io/docs/guides/keepalive/)

**External Research:**
- User-provided source confirms streams should last "hours"
- YouTube API quota: [Quota Calculator](https://developers.google.com/youtube/v3/determine_quota_cost)

---

**Status:** Deploying Python demo parameter test
**ETA:** 2-3 minutes for deployment
**Next Update:** Monitor Ludwig's stream for duration changes
