# Discord Call for Help - YouTube Live Chat API

---

## Short Version (For Quick Posts):

```
Has anyone used YouTube's gRPC liveChatMessages.streamList in production?

I'm hitting a weird issue where streams close after exactly 10 seconds with io.EOF, regardless of configuration. Makes it basically unusable - 43,200 quota/day per stream instead of the expected ~120.

Already tried: aggressive keepalive (5s), 4MB flow control windows, async message processing, proper pageToken resumption. We're correctly resuming with tokens and getting new messages (not re-downloading history), connection stays READY, no errors in trailer metadata.

Still closes at 10.2-10.7s every time. Pattern is usually: final response has 0 messages + duplicate token, then EOF.

Is this just how the API works? Documentation suggests long-lived connections but I can't get past 10 seconds. Would love to know if anyone's actually achieved hour-long streams or if I'm chasing something that doesn't exist.

Code is in Go using google.golang.org/grpc v1.77.0, running in k8s with valid OAuth tokens.
```

---

## Medium Version (For Detailed Posts):

```
Need help with YouTube Live Chat API gRPC streaming

I'm building a chat aggregation service and running into a frustrating issue with YouTube's liveChatMessages.streamList gRPC endpoint. Streams consistently close after exactly 10 seconds with io.EOF, which makes the quota math completely unusable for production.

The problem:
- Every stream closes at 10.2-10.7 seconds with clean EOF
- This means 6 reconnects/min × 5 quota units = 1,800 units/hour per stream
- With the default 10k quota limit, I can support 0.23 streams total
- Expected behavior would be ~1 reconnect/hour for 5 units/hour

What I've tried so far:
- Aggressive keepalive (5s ping interval, 2s timeout, PermitWithoutStream)
- 4MB flow control windows to handle burst traffic
- Async message processing so nothing blocks the receive loop
- Proper pageToken handling (resume where we left off, not re-fetching history)
- Tested with and without maxResults parameter
- Matched the Python demo parameters exactly
- Multiple different channels and OAuth tokens

Current state (after fixes):
The good news is pageToken resumption is working correctly now:
- Reconnections use pageToken: true
- Only getting 1-3 new messages per connection (not 83-message history dumps)
- Tokens updating properly between responses
- gRPC connection state stays READY
- No errors in trailer metadata

But streams still close at exactly 10 seconds every single time.

Typical pattern I'm seeing:
```
Response 15: 1 message, token changes, 9.7s elapsed
Response 16: 1 message, token changes, 10.2s elapsed
Response 17: 0 messages, token unchanged (duplicate) → EOF
```

Sometimes it closes right after a regular message too, not just after empty responses.

My question:
Is this just how YouTube's streamList works? The Python demo has a `while True` loop around the stream which makes me think maybe frequent reconnections are expected. But I've seen people mention streams lasting "hours" with proper configuration, and the docs describe it as "the most efficient way" which doesn't match 360x quota overhead.

Has anyone actually gotten YouTube gRPC streams to last longer than 10 seconds? If so, what am I missing?

Running Go 1.23 with google.golang.org/grpc v1.77.0 in k8s, using valid OAuth2 tokens. Connection state stays READY throughout, trailer metadata shows no errors.

Would really appreciate any insights or pointers to working implementations. This is blocking our production deployment.
```

---

## Long Version (For Forums/GitHub Discussions):

```markdown
# YouTube Live Chat API gRPC StreamList - Can't Get Past 10 Second Limit

## TL;DR

I can't get YouTube's liveChatMessages.streamList gRPC endpoint to stay open longer than 10 seconds. Streams consistently close with io.EOF at the 10-second mark, consuming way more quota than expected. Tried everything I can think of but stuck at this limit. Looking for help from anyone who's actually gotten this working in production.

## The Problem

I'm using YouTube's liveChatMessages.streamList gRPC endpoint to get live chat messages. The documentation describes this as a "server-streaming connection" that's "the most efficient way to consume live chat messages" since it pushes new messages as they arrive.

What actually happens:
- Streams close with io.EOF after exactly 10 seconds every single time
- Never seen a stream last longer than 10.7 seconds
- Means 6 reconnects per minute at 5 quota units each = 1,800 units/hour per stream
- For a 24/7 stream that's 43,200 units/day (vs the expected ~120 if streams actually stayed open)

With the default 10k quota limit, I can support less than one full stream. Kind of defeats the purpose.

## What I've Tried

Spent the last day testing every configuration I could think of:

**gRPC keepalive:**
- Set to ping every 5 seconds with 2 second timeout
- PermitWithoutStream enabled to handle quiet chat periods
- Connection state stays READY throughout

**Flow control:**
- 4MB initial window sizes to handle high-volume streams
- Async message processing in goroutines so nothing blocks the receive loop

**Request parameters:**
- Tried with maxResults=20 (matching the Python demo)
- Tried without maxResults entirely
- Only requesting "snippet" part
- Tested with and without pageToken

**Different scenarios:**
- High-activity channels (100+ messages/min)
- Low-activity channels (5 messages/min)
- Multiple OAuth tokens from different accounts
- Different times of day and network conditions

## What's Actually Happening Now

After all the fixes, the pageToken handling is working correctly:
- Reconnections properly use the pageToken from the previous stream
- Getting 1-3 new messages per connection instead of re-downloading the 83-message history
- Tokens updating correctly between responses
- gRPC connection state stays READY the whole time
- Trailer metadata shows no errors or status codes

But the 10-second closure still happens:
- Every stream closes at exactly 10.2-10.7 seconds
- Doesn't matter if it's a high-activity or low-activity channel
- Clean io.EOF, no error indication whatsoever

## The Pattern (Happens Every Time)

Typical stream lifecycle:
```
Response 1-15: Getting messages, tokens updating, everything normal
Response 16 (at 10.2s): 1 message, token changes
Response 17 (at 10.6s): 0 messages, token unchanged (duplicate)
→ EOF immediately after
→ Reconnect with saved token
→ Repeat every 10 seconds
```

Sometimes it closes right after a normal message too (not always after empty response).

The trailer metadata YouTube sends back just has load balancing metrics and server stats, no grpc-status or grpc-message errors. Connection state stays READY. It's a completely clean closure, just happens to be at exactly 10 seconds every time.

## What I'm Trying to Figure Out

1. Is this just how the API works? Maybe 10 seconds is the actual intended max duration and the documentation is just misleading about being "efficient"?

2. Has anyone actually gotten YouTube gRPC streams to last longer? I've read claims about streams lasting "hours" but can't reproduce it.

3. Am I missing some critical setting? Different auth scopes, metadata headers, some server-side policy I need to request?

4. The official Python demo has a `while True` loop wrapping the stream call. Is that there BECAUSE streams are expected to close every 10 seconds? Or is it just for error handling?

```python
while True:  # ← Why loop if streams last hours?
    for response in stub.StreamList(request):
        # Process response
```

## Context

Building this in Go 1.23 with google.golang.org/grpc v1.77.0, running in Kubernetes with stable network. Using valid OAuth2 tokens, tested with multiple user accounts. Channels range from 5 to 100+ messages per minute.

For comparison, our Twitch IRC and Kick WebSocket connections stay alive for hours/days without any issues. YouTube's behavior is uniquely problematic.

If 10-second reconnections are actually intended, that's fine - I just need to know so I can request a quota increase and move on. But if there's a way to achieve longer streams, I'd love to know what I'm missing.

Happy to share code, logs, or provide test access if anyone wants to dig into this.
```

---

## Ultra-Short Version (Twitter/X):

```
YouTube Live Chat API question: Does anyone's gRPC streamList actually stay open longer than 10 seconds?

Mine closes with io.EOF at exactly 10s every time despite:
- 5s keepalive
- 4MB flow control
- Async processing
- Proper pageToken resume

Connection stays READY, no errors, just clean EOF at 10s.

Is this intended or am I missing something?

360x quota overhead is killing me.
```

---

## Copy-Paste Ready for Discord:

```
Hey, anyone have experience with YouTube's Live Chat API gRPC streaming?

I'm using the `liveChatMessages.streamList` endpoint and streams consistently close after exactly 10 seconds with io.EOF. Tried all the usual fixes - aggressive keepalive (5s ping), 4MB flow control windows, async message processing, proper pageToken resumption.

The good news is everything's working correctly now (using tokens properly, getting new messages instead of re-downloading history), but streams still close at the 10-second mark every time. Connection stays READY throughout, no errors in trailer metadata, just a clean EOF.

Typical pattern: 15-18 responses over 10 seconds, final response often has 0 messages with a duplicate token, then EOF.

Is this just how YouTube's API works? The docs describe it as "the most efficient way" to get chat messages but 6 reconnects/min at 5 quota each = 1,800 units/hour per stream, which seems insane.

Has anyone actually gotten YouTube gRPC streams to last longer than 10 seconds? Or is the Python demo's `while True` loop there because frequent reconnections are expected?

Running Go 1.23 with google.golang.org/grpc v1.77.0 in k8s, valid OAuth tokens. For comparison, our Twitch and Kick connections stay alive for hours without issues.

Would love any insights - this is blocking production deployment.
```
