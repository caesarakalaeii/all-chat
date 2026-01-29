# Quick Reference: Redis Operations

**Time Estimate**: 15-30 minutes | **Difficulty**: ⭐ Easy

**Goal**: Inspect and debug Redis Streams, Pub/Sub, and cache operations.

---

## Redis CLI Access

```bash
# Local development
redis-cli

# Kubernetes
kubectl exec -n allchat deployment/redis -- redis-cli
```

---

## Redis Streams Commands

### Inspect Stream

```bash
# Get stream info
XINFO STREAM chat:raw

# Output shows:
# - length: 1542 (current messages)
# - first-entry: oldest message ID
# - last-entry: newest message ID
# - groups: consumer groups

# Read last 10 messages
XREAD COUNT 10 STREAMS chat:raw 0-0

# Read from specific ID
XREAD COUNT 10 STREAMS chat:raw 1706400000000-0
```

### Check Consumer Groups

```bash
# List consumer groups
XINFO GROUPS chat:raw

# Output:
# - name: message-processors
# - consumers: 3
# - pending: 42 (unacknowledged messages)

# Check pending messages (lag)
XPENDING chat:raw message-processors

# Output:
# 1) (integer) 42            # Total pending
# 2) "1706400000-0"          # Oldest pending ID
# 3) "1706400099-0"          # Newest pending ID
# 4) Consumer breakdown

# View specific pending messages
XPENDING chat:raw message-processors - + 10
```

### Troubleshoot Consumer Lag

```bash
# If lag >10,000 messages, check consumers
XINFO CONSUMERS chat:raw message-processors

# Output shows per-consumer stats:
# - name: processor-1
# - pending: 15000 (stuck consumer!)
# - idle: 300000 (5 minutes since last activity)

# Claim stuck messages for another consumer
XAUTOCLAIM chat:raw message-processors processor-2 300000 0-0 COUNT 100
```

---

## Redis Pub/Sub Commands

### Check Active Channels

```bash
# List all active Pub/Sub channels
PUBSUB CHANNELS overlay:*

# Output:
# overlay:uuid-1
# overlay:uuid-2
# ...

# Count subscribers per channel
PUBSUB NUMSUB overlay:uuid-1

# Output:
# overlay:uuid-1
# 5  (5 API Gateway instances subscribed)
```

### Subscribe to Channel (Debug)

```bash
# Subscribe to overlay channel (see messages in real-time)
SUBSCRIBE overlay:uuid-1

# Output (when messages arrive):
# 1) "message"
# 2) "overlay:uuid-1"
# 3) "{"id":"...","platform":"twitch","user":{...}}"

# Press Ctrl+C to stop
```

### Publish Test Message

```bash
# Publish mock message (testing)
PUBLISH overlay:uuid-1 '{"id":"test","platform":"test","user":{"username":"test"},"message":{"text":"Test"}}'

# Output:
# (integer) 5  (5 subscribers received message)
```

---

## Cache Operations

### Inspect Emote Cache

```bash
# List emote cache keys
KEYS emote:*

# Get emote data
GET emote:Kappa:twitch

# Output (JSON):
# {"code":"Kappa","provider":"twitch","url":"https://..."}

# Check TTL
TTL emote:Kappa:twitch

# Output:
# (integer) 3458  (seconds remaining, ~57 minutes)
```

### Clear Cache

```bash
# Delete specific emote
DEL emote:Kappa:twitch

# Delete all emotes for channel
DEL emote:*:twitch:xqc

# Clear all emote cache (use with caution)
EVAL "return redis.call('del', unpack(redis.call('keys', 'emote:*')))" 0
```

---

## Performance Debugging

### Check Memory Usage

```bash
INFO memory

# Key metrics:
# used_memory_human: 128.45M
# used_memory_peak_human: 256.12M
# maxmemory_human: 1G
```

### Check Slow Commands

```bash
SLOWLOG GET 10

# Output shows slowest commands (>10ms threshold)
# Useful for identifying bottlenecks
```

### Monitor Commands Per Second

```bash
INFO stats

# Look for:
# instantaneous_ops_per_sec: 5420
# total_commands_processed: 1542890
```

---

## Common Issues

### Stream Filling Too Fast

**Symptom**: `XINFO STREAM chat:raw` shows length approaching MAXLEN (50,000)

**Solution**: Increase MAXLEN or scale Message Processor
```bash
# Increase MAXLEN (modify publisher code)
XADD chat:raw MAXLEN ~ 500000 * field value
```

### High Memory Usage

**Symptom**: Redis using >80% of maxmemory

**Check largest keys**:
```bash
# Requires redis-cli --bigkeys (run on server)
redis-cli --bigkeys

# Or use memory doctor
MEMORY DOCTOR
```

---

## Related Documentation

- [ADR-0002](../adr/0002-redis-streams-pubsub.md) - Redis architecture
- [01-DATA-FLOW.md](../architecture/01-DATA-FLOW.md) - Message flow
