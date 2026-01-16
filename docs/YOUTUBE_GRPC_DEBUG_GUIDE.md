# YouTube gRPC StreamList Debug Guide

## Problem Statement

The gRPC streamList endpoint is disconnecting every 10 seconds, causing excessive quota usage:
- Each connection costs **5 quota units**
- Disconnecting every 10s = **6 connections/minute = 8,640 connections/day**
- Daily quota usage: **8,640 × 5 = 43,200 units** (vs 10,000 limit)
- **This will exhaust quota in 2-3 hours!**

## Expected Behavior

According to YouTube's official documentation:
- The gRPC stream should **stay open continuously**
- Messages are pushed in real-time as they arrive
- The stream should only close when:
  - The live chat ends (`offlineAt` field is set)
  - An error occurs
  - The client disconnects

## Debug Logging Added

We've added comprehensive debug logging to identify the root cause:

### 1. gRPC Receive Loop Timing (`grpc_client.go`)

**Look for these logs:**

```
"About to call stream.Recv()" - Marks when we start waiting for messages
"stream.Recv() returned" - Shows how long Recv() blocked and if EOF occurred
"Long delay between Recv() calls" - WARNING if >5s between Recv() calls (handler blocking)
```

**What to check:**
- Is `recv_duration` consistently ~10 seconds before EOF?
- Are there long delays between Recv() calls? (>1s indicates blocking)

### 2. Handler Execution Timing (`grpc_client.go` + `poller.go`)

**Look for these logs:**

```
"Handler invoked with response" - When handler starts
"Message parsing completed" - How long parsing took
"Handler returning (sync work completed)" - Total synchronous handler time
"Handler is blocking receive loop" - WARNING if handler >1s
```

**What to check:**
- Is `total_handler_time` > 500ms? (blocks receive loop)
- Is `parse_duration` slow? (>100ms is concerning)
- Is `mutex_time` slow? (>10ms indicates contention)

### 3. Context Health Monitoring (`grpc_client.go`)

**Look for these logs:**

```
"Context health check: OK" - Every 3s, confirms context is alive
"Context is DONE during streaming" - ERROR if context cancelled
"Context cancelled during stream" - When context cancellation detected
```

**What to check:**
- Is context being cancelled unexpectedly?
- Check `ctx.Err()` for the reason

### 4. EOF Analysis (`grpc_client.go`)

**Look for these logs:**

```
"gRPC stream EOF received - investigating cause" - Detailed EOF context
"Trailer metadata detail" - All trailer metadata from YouTube
"gRPC stream closed with non-zero status" - ERROR indicator
"gRPC stream closed cleanly (EOF with status 0)" - Normal close
```

**What to check:**
- `stream_duration` - Is it consistently ~10 seconds?
- `total_responses` - How many responses before EOF?
- `loop_iterations` - How many Recv() calls?
- `trailer_metadata` - Does YouTube send a reason?
- `grpc-status` - Is it "0" (OK) or error code?

### 5. Connection State Monitoring (`grpc_client.go`)

**Look for these logs:**

```
"Initial connection state at stream start" - Connection state when stream starts
"Connection state check" - Every 5 responses
```

**Connection states:**
- `IDLE` - No active RPCs
- `CONNECTING` - Connecting to server
- `READY` - Connected and ready
- `TRANSIENT_FAILURE` - Temporary failure
- `SHUTDOWN` - Connection shut down

## Common Causes & Solutions

### Cause 1: Handler Blocking Receive Loop

**Symptoms:**
- `total_handler_time` > 500ms
- Long delays between Recv() calls
- EOF after exactly 10 seconds

**Solution:**
- Make ALL handler work async (not just message publishing)
- Move parsing, token extraction, and mutex updates to goroutine

### Cause 2: Keepalive Not Working

**Symptoms:**
- EOF after consistent duration (10s)
- No responses between start and EOF
- Connection state changes to IDLE

**Solution:**
- Verify keepalive settings are correct
- Check if YouTube's GFE is blocking keepalive pings
- Try adjusting keepalive time (currently 5s)

### Cause 3: Flow Control Issues

**Symptoms:**
- Handler is fast but EOF still occurs
- High message volume (>50 msgs/response)
- No WINDOW_UPDATE frames being sent

**Solution:**
- Already configured with 4MB window size
- Ensure handler NEVER blocks
- Check if Redis publish is slow

### Cause 4: Context Cancellation

**Symptoms:**
- "Context is DONE during streaming" error
- `ctx.Err()` shows cancellation reason
- Stream ends abruptly

**Solution:**
- Check poller.go `monitorConnection()` (line 496)
- Verify connection checker isn't incorrectly disconnecting
- Check if stop signal is being sent

### Cause 5: YouTube Server-Side Timeout

**Symptoms:**
- Trailer metadata shows specific error
- `grpc-status` is non-zero
- Consistent EOF timing regardless of traffic

**Solution:**
- Check trailer metadata for YouTube's reason
- May need to add additional headers/metadata
- Could be OAuth token expiration

## Testing Steps

1. **Enable Debug Logging**
   ```bash
   # Set log level to DEBUG
   export LOG_LEVEL=debug

   # Restart youtube-listener
   kubectl rollout restart deployment/youtube-listener -n all-chat
   ```

2. **Watch Logs**
   ```bash
   kubectl logs -f deployment/youtube-listener -n all-chat | grep -E "stream.Recv|Handler|EOF|Context"
   ```

3. **Key Metrics to Track**
   - Time between "About to call stream.Recv()" and "stream.Recv() returned"
   - `total_handler_time` in "Handler returning" log
   - `stream_duration` in EOF log
   - Pattern of `loop_iterations` vs `responses_received`

4. **Expected Normal Behavior**
   - `recv_duration` varies (0ms to seconds, waiting for messages)
   - `total_handler_time` < 100ms consistently
   - Stream stays open for minutes/hours, not seconds
   - EOF only when stream actually ends or overlay disconnects

## Quota Impact Analysis

**Current (broken) behavior:**
- 10-second reconnect cycle
- 5 units per connection
- Expected: 43,200 units/day → **Quota exhausted in 2-3 hours**

**Target (fixed) behavior:**
- Single long-lived connection per stream
- 5 units per connection (one-time cost)
- Expected: 50-100 units/day for typical usage → **Within 10,000 limit**

**Savings:**
- **43,150+ units/day saved** (99.9% reduction)
- Allows monitoring **2,000+ streams** instead of 5

## Next Steps

1. Deploy the debug version
2. Collect logs for one 10-second cycle
3. Analyze logs to identify root cause:
   - If handler blocking → Make handler fully async
   - If keepalive failing → Adjust keepalive settings
   - If context cancelled → Fix cancellation logic
   - If YouTube timeout → Check trailer metadata for reason
4. Implement fix based on findings
5. Verify stream stays open for hours, not seconds

## References

- Official docs: `/docs/YOUTUBE_DOCS_STREAMLIST.txt`
- Python example: `/docs/YOUTUBE_PYTHON_EXAMPLE.txt`
- gRPC keepalive: https://github.com/grpc/grpc/blob/master/doc/keepalive.md
- YouTube API quota: https://developers.google.com/youtube/v3/determine_quota_cost
