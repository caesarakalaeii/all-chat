# Troubleshooting: Twitch IRC Issues

Twitch IRC connection, channel join, and message parsing issues.

---

## Bot Not Joining Channels

### OAuth Token Invalid

**Symptom**: IRC connection fails with authentication error

**Solution**:
```bash
# Get new OAuth token from Twitch
# Visit: https://twitchapps.com/tmi/

# Update environment variable
export TWITCH_BOT_OAUTH=oauth:new_token_here

# Restart service
kubectl rollout restart deployment/twitch-listener -n allchat
```

**File**: `services/twitch-listener/irc/client.go:Connect()`

### Rate Limit Hit

**Symptom**: Channels not joining, logs show "rate limited"

**Cause**: Twitch IRC rate limit is **20 JOIN per 10 seconds**

**Solution**:
```bash
# Check logs for rate limit messages
kubectl logs -n allchat deployment/twitch-listener | grep "rate limit"

# Service automatically retries with backoff
# Wait 10-15 seconds for joins to complete
```

**File**: `services/twitch-listener/channels/manager.go:JoinChannel()`

### Channel Name Wrong

**Symptom**: JOIN succeeds but no messages received

**Solution**:
- Verify channel name is correct (case-insensitive but must be exact)
- Check channel is actually streaming
- Ensure bot is not banned from channel

---

## Messages Not Received

### IRC Connection Dropped

**Symptom**: Connected but messages stopped arriving

**Check connection status**:
```bash
curl http://localhost:8085/status | jq .connection.status
# Expected: "connected"
```

**Solution**: Service automatically reconnects, check logs for reconnection

**File**: `services/twitch-listener/irc/client.go:OnDisconnect()`

### Redis Publish Failing

**Symptom**: Messages received but not in Redis Stream

**Check Redis**:
```bash
redis-cli XINFO STREAM chat:raw
# Should show messages with platform="twitch"
```

**File**: `services/twitch-listener/publisher/redis.go:Publish()`

---

## Related Documentation

- [twitch-listener/README.md](../../services/twitch-listener/README.md) - Complete service documentation
- [decision-tree.md](./decision-tree.md) - High-level triage
