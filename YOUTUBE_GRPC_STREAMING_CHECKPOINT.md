# YouTube gRPC Streaming Investigation Checkpoint

**Date:** 2026-01-15  
**Status:** CRITICAL ISSUE - Quota consumption makes service non-viable for low-viewership streams  
**Urgency:** HIGH - Blocks production deployment

---

## Problem Summary

The YouTube `liveChatMessages.streamList` gRPC API is disconnecting every ~10 seconds, causing excessive quota consumption that makes the service unsustainable for low-viewership streams.

### Current Behavior (BROKEN)
- **Stream duration:** ~10 seconds per connection
- **Reconnection rate:** 6 connections/minute
- **Quota cost:** 5 units per connection
- **Daily usage:** 43,200 units/day for ONE 24/7 stream
- **Daily quota limit:** 10,000 units
- **Time until quota exhausted:** 5.5 hours
- **Streams supported:** ~0.23 streams per day (UNUSABLE)

### Expected Behavior (per YouTube Documentation)
- Stream should stay open indefinitely
- Server pushes new messages as they arrive
- "Reduces the need for constant polling" (FALSE in current implementation)
- Should use ~5 units for initial connection + minimal reconnections

---

## Key Findings

### 1. Quota Tracking is ACCURATE
- Our tracking: 1,105 units
- Google Cloud Console: matches exactly
- We ARE being charged 5 units per connection (correct)
- **This proves the 10s disconnects are REAL, not a tracking bug**

### 2. Stream Always Closes After 1 Response
```
"total_responses":1,"stream_duration":10.388323209
```
- Every connection receives exactly 1 response batch
- Server sends EOF (clean close) after ~10 seconds
- This pattern is consistent across all connections

### 3. We ALWAYS Use pageToken
```
"with_page_token":true
```
- Every connection includes the pageToken from previous response
- We load pageToken from Redis TokenStore on startup
- **HYPOTHESIS:** Using pageToken might prevent long-lived streaming

### 4. Documentation vs Reality
**Documentation says:**
> "The streamList method pushes new messages to the client as they become available, which reduces the need for constant polling and helps to avoid exceeding your quota."

**Reality:**
- 10s reconnections = almost same quota cost as 5s REST polling
- Old REST polling: 86,400 units/day
- Current gRPC: 43,200 units/day
- Only 2x improvement, NOT the 100x+ expected for true streaming

---

## Potential Root Causes

### Theory 1: pageToken Behavior (MOST LIKELY)
**Hypothesis:** Using pageToken on every connection prevents long-lived streaming.

**Evidence:**
- Python demo has `while True` loop (expects reconnections)
- But WHEN should you reconnect? Demo doesn't show.
- Proto definition says `maxResults` is "Not used in the streaming RPC"
- We cache pageToken and reuse it on EVERY connection

**Test Required:**
- Connect WITHOUT pageToken initially
- Only use pageToken on unexpected disconnections
- See if this allows stream to stay open longer

### Theory 2: Idle Stream Behavior
**Hypothesis:** Server closes streams when no new messages for 10 seconds.

**Evidence:**
- Stream went offline recently (low/no activity)
- Maybe server conserves resources by closing idle streams

**Test Required:**
- Monitor stream duration during HIGH activity periods
- Compare idle stream vs active chat stream behavior

### Theory 3: Implementation Error
**Hypothesis:** We're missing a parameter or doing something wrong.

**Checked:**
- ✅ gRPC keepalive parameters added (no effect)
- ✅ Not sleeping in handler (fixed)
- ✅ Using correct proto definitions
- ✅ OAuth credentials working
- ❓ Maybe need different client options?

### Theory 4: API Changed or Documentation Wrong
**Hypothesis:** YouTube API doesn't actually support long-lived streaming.

**Evidence:**
- No community examples of 24/7 streaming success
- Documentation promises don't match observed behavior
- Possible that API was changed after docs written

---

## What We Fixed Today

### ✅ Completed
1. **Removed sleep from handler** - Was blocking gRPC receive loop
2. **Added gRPC keepalive** - 20s ping interval (didn't help)
3. **Stream state persistence** - Redis-based state for instant pod resumption
4. **Bypass 5-minute delay** - Check stream state before applying overlay connection delay
5. **Quota tracking accuracy** - Reserve-confirm-rollback pattern working correctly

### ❌ Not Fixed
- **10-second stream disconnects** - Root cause still unknown
- **Excessive quota consumption** - Makes service non-viable

---

## Quota Math (Current State)

### Single Stream (24/7)
```
6 connections/min × 60 min × 24 hours × 5 units = 43,200 units/day
Daily quota: 10,000 units
Exceeds quota in: 5.5 hours
```

### Multiple Streams
With 10,000 unit daily quota:
```
10,000 units ÷ 43,200 units per stream = 0.23 streams
```
**Can only support ONE stream for 5.5 hours/day!**

### With Quota Increase (1M units/day)
```
1,000,000 units ÷ 43,200 units per stream = 23 streams (24/7)
```
**Still WAY too few for production service!**

### If We Fix the 10s Disconnects
Assuming stream stays open 24 hours with 1 reconnection/hour:
```
24 reconnections/day × 5 units = 120 units/day per stream
10,000 units ÷ 120 units = 83 streams (with base quota)
1,000,000 units ÷ 120 units = 8,333 streams (with increased quota)
```
**This would be VIABLE!**

---

## Immediate Action Items

### PRIORITY 1: Test pageToken Theory
**What:** Connect without pageToken initially, only use for recovery
**Why:** Most likely cause of 10s disconnects
**How:**
1. Modify `poll()` to NOT load pageToken on initial connection
2. Only use pageToken from response for NEXT connection
3. Monitor if stream stays open longer
4. Compare behavior with/without pageToken

**Files to modify:**
- `services/youtube-listener/streams/poller.go` - Don't load cached token initially
- `services/youtube-listener/streams/token_store.go` - Maybe clear on poller start?

### PRIORITY 2: Test During High Activity
**What:** Monitor stream behavior during active chat
**Why:** Determine if idle streams are treated differently
**How:**
1. Find a high-activity stream (1000+ viewers)
2. Monitor connection duration and reconnection frequency
3. Compare quota usage vs low-activity stream

### PRIORITY 3: Request Quota Increase
**What:** Request increase to 1M units/day from Google
**Why:** Buys time while investigating root cause
**How:**
1. Go to Google Cloud Console → APIs & Services → YouTube Data API v3
2. Click "Request quota increase"
3. Explain use case: live chat aggregation service
4. Request 1M units/day

**Form justification:**
> "We are building a live chat aggregation service for streamers. We use the liveChatMessages.streamList gRPC API which costs 5 units per connection. Even with persistent streaming connections, we need higher quota to support multiple concurrent streams. We are experiencing reconnections every ~10 seconds (investigating cause) which temporarily increases our quota needs."

### PRIORITY 4: Investigate Google Community
**What:** Search for others with same issue
**Where:**
- Stack Overflow: "youtube api streamlist quota"
- Google Issue Tracker
- YouTube API GitHub issues
- Reddit r/youtubegaming

**Questions to ask:**
- Has anyone successfully kept streamList connections open for hours?
- What's the expected reconnection frequency?
- Is there a maximum connection duration?

---

## Code Locations

### gRPC Client
- **File:** `services/youtube-listener/api/grpc_client.go`
- **Key function:** `StreamChatMessagesGRPC()` (lines 77-224)
- **Quota tracking:** Lines 103-119, 149-159
- **Current behavior:** Charges 5 units per connection with pageToken

### Poller
- **File:** `services/youtube-listener/streams/poller.go`
- **Key function:** `poll()` (lines 356-467)
- **pageToken loading:** Lines 361-370
- **Problem:** Loads cached pageToken on EVERY poll attempt

### Token Store
- **File:** `services/youtube-listener/streams/token_store.go`
- **Stores:** `youtube:streamlist:token:{live_chat_id}`
- **TTL:** 24 hours
- **Current value:** `GP2UrobCjpID`

### Stream State Store (NEW)
- **File:** `services/youtube-listener/streams/stream_state_store.go`
- **Purpose:** Persist active stream state for pod resumption
- **Works:** Successfully allows instant resumption
- **Doesn't solve:** Quota consumption issue

---

## Questions to Answer

1. **What is the EXPECTED behavior of pageToken in gRPC streaming?**
   - Should it be used on every connection?
   - Or only for recovery after unexpected disconnect?

2. **How long SHOULD a gRPC stream stay open?**
   - Indefinitely until chat ends?
   - Fixed duration then reconnect?
   - Depends on activity level?

3. **Why does proto say maxResults is "Not used"?**
   - What controls batch size then?
   - Is there a different parameter?

4. **Is 10s disconnect YouTube's intentional behavior?**
   - Load balancing?
   - Security (credential refresh)?
   - Bug/limitation?

5. **How do production services handle this?**
   - Do they request massive quota increases?
   - Use different API methods?
   - Implement smart fallbacks?

---

## References

### Official Documentation
- **streamList API:** https://developers.google.com/youtube/v3/live/docs/liveChatMessages/streamList
- **Python Demo:** https://developers.google.com/youtube/v3/live/guides/streaming_live_chat
- **Proto Definition:** `services/youtube-listener/api/proto/stream_list.proto`

### Key Quote from Docs
> "Note: To poll for live chat messages, use the liveChatMessages.streamList method. The streamList method pushes new messages to the client as they become available, which reduces the need for constant polling and helps to avoid exceeding your quota."

**This promise is currently FALSE with 10s reconnections!**

---

## Metrics to Track

When testing solutions, monitor:
1. **Stream duration** - How long before EOF?
2. **Reconnection frequency** - Connections per minute
3. **Quota consumption** - Units per hour
4. **Message latency** - Time from post to receive
5. **Response batch size** - Messages per response
6. **Activity correlation** - Does active chat = longer streams?

---

## Fallback Plans

If we cannot fix the 10s disconnects:

### Plan A: Quota Increase
- Request 1M units/day
- Supports ~23 streams (still too few)
- Not a real solution, just buys time

### Plan B: Hybrid Approach
- Use gRPC for high-activity streams
- Fall back to REST polling for low-activity
- Smart detection of which method to use
- Requires complex logic

### Plan C: Time-Limited Streaming
- Only stream during "active hours" (e.g., 12-hour window)
- Rest of time use polling or no YouTube support
- Bad user experience

### Plan D: REST Polling Only
- Abandon gRPC streaming
- Use old `liveChatMessages.list` method
- Accept quota limitations
- Works but defeats the purpose

---

## Next Session TODO

1. [ ] Test connecting WITHOUT initial pageToken
2. [ ] Monitor behavior during high-activity stream
3. [ ] Submit quota increase request to Google
4. [ ] Search community for similar issues
5. [ ] Consider filing bug report with Google
6. [ ] Document actual vs expected API behavior
7. [ ] Calculate ROI of different solutions

---

## Success Criteria

We'll know this is fixed when:
- ✅ Streams stay open for >1 hour without reconnecting
- ✅ Quota usage <200 units/day per stream
- ✅ Can support 50+ concurrent streams with base quota
- ✅ Can support 1000+ streams with 1M quota increase

---

**Status:** Investigation paused - needs deeper research and testing  
**Owner:** To be resumed when time permits  
**Impact:** BLOCKS production deployment for multi-stream support
