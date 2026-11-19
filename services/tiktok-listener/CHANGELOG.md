# TikTok Listener Changelog

## [Unreleased] - Resource Optimization & Message Deduplication

### Fixed
- **Flap retries each took a connect slot**: a WS connect that TikTok answers with a plain HTTP 200 is retried in place, up to `TIKTOK_FLAP_MAX_FAST_RETRIES` times (6 in production), and every retry calls `connection.connect()` again — which re-enters the connector's sign route and so took a fresh WS-connect slot. One flapping room spent 1 + 6 = 7 of the hour's 12 connects in about twelve seconds, and two of them exhausted a pod's whole budget, so every room that went live for the rest of that hour was refused. On 2026-11-19, after the per-session fix had rolled out, each pod logged about one `TikTok stream connected` per window while its window had to be holding ~12 connects to refuse at all, and `TikTokPureNodeConnectBudgetExhausted` kept firing against distinct rooms on both pods. A room's slot now also covers its re-dials for the next 60 seconds, which spans the retry schedule; the burst is the one session-flagging event the budget exists to pace. A room that reconnects later in the hour still pays a second slot.
- **Whole fleet spent one session's connect budget**: TikTok scores its ~15 WS-connects/hour session-flag threshold per session, and the signer already held six warm rooms — each its own viewer tab, proxy lane, profile directory and cookie jar, so each its own session. `GET /v1/session` served the first servable room to every caller, so both listener pods funnelled every connect through one session while five sessions' capacity went unspent. Halving `TIKTOK_MAX_WS_CONNECTS_PER_HOUR` to 6 on 2026-11-18 correctly stopped 2 replicas x 12 overrunning that one session's ceiling, but left 12 connects/hour to cover ~78 churny rooms: on 2026-11-18, 17 refusals across 14 distinct rooms in 35 minutes, with `retryAfterMs` up to 179 minutes, kept `TikTokPureNodeConnectBudgetExhausted` firing. The signer now assigns a warm session per consumer key and reports it on the lease; `TIKTOK_LEASE_CONSUMER` (the pod name, via the downward API) is what claims one, and the listener meters its connect window against the session it was handed, resetting only when that session changes. Replicas now add connect capacity instead of dividing one ceiling, bounded by the number of warm rooms the signer holds.
- **Parked rooms woke together and were refused again**: every room refused by the pure-node WS-connect budget was told to retry when the oldest slot slid. They all came back at the same moment, one connected and the rest were refused again, slot after slot. On 2026-11-12, 37 of the day's 59 refusals were rooms already parked, and the repeats kept `TikTokPureNodeConnectBudgetExhausted` firing and resolving all evening. Refused rooms now queue: each is told when a slot frees that no room ahead of it is promised, and a slot that frees for a queued room is held for it for 90 seconds, so other rooms cannot take it before it returns. One refusal is now one room waiting.
- **Lost leases left rooms in the poller and churned the fleet**: when a pod lost the lease of an offline room (a 410 on renewal), the teardown returned early because the room had no connection, so the room stayed in that pod's live-status poller. The pod kept polling a room another pod owned, and because a polled room is never re-claimed, the owner shed it in every rebalance and took it back about 26 seconds later, when its one-cycle re-claim hold had already run out. On 2026-11-01 a Redis failover took 70 leases at once, and one room bounced every 50 seconds for the rest of the day; rebalances ran at about twice their usual rate. The teardown now drops the poller target whether or not the room is connected, and the re-claim hold lasts two demand cycles.
- **Deaf room drained the connect budget**: a room whose connection never decoded a frame was killed by the heartbeat after ~2 minutes and reconnected about 30 seconds later, all day. The silent-failure streak was meant to slow this down through the error backoff, but every successful connect resets that backoff, so the delay stayed at 4 seconds (prod 2026-10-27: one room reached a streak of 42). Each reconnect is a WS-connect sign, so that one room used most of its pod's 12/hour budget, and other rooms going live were refused, which fired `TikTokPureNodeConnectBudgetExhausted`. A room still deaf after one reconnect is now parked for 10, then 20, then at most 30 minutes, about 2 signs an hour when it stays deaf. Stuck recovery and the rebalance hold respect this park the same way they respect a budget park.
- **Signer outage drained the connect budget**: `PureNodeSigner` took a WS-connect slot before fetching the lease, so a sign whose lease fetch failed (signer `503 no_warm_session`) still used up the hourly budget without dialling anything. During an 8-hour signer outage on 2026-10-10, failed lease fetches filled each pod's 12 slots, every later connect was refused as `reason=budget`, and `TikTokPureNodeConnectBudgetExhausted` kept firing for about two hours after the signer recovered. A sign that gets no lease now gives its slot back.
- **Pure-node lease cache never hit**: `PureNodeSigner` keyed its 10-minute lease reuse window on the signer's `capturedAt`, which is the age of the warm tab's URL and is routinely hours old. Every sign, the lane-pin pre-sign included, re-fetched the lease, so the 8/hour lease budget refused connects after about 4 of them and fired `TikTokPureNodeConnectBudgetExhausted`. The window now counts from the listener's own fetch.
- **Budget park lost on rebalance**: a room parked on this pod's sign budget lost its park when rebalancing shed its lease (the teardown drops backoff state), and the pod re-claimed and re-dialled it about 40 seconds later into the same refusal. The re-claim hold now lasts until the park ends; another pod, with its own budget, can still claim the room.
- **Stuck recovery cut budget parks short**: a room that went live after an offline streak kept that streak's max backoff when it was parked on the sign budget, so stuck-channel recovery treated it as stuck after 5 minutes and re-dialled it into the same refusal. Recovery now skips rooms with an active budget park.
- **Duplicated gift events**: Streakable gifts (`gift.type === 1`) fire the `GIFT` event repeatedly while the combo is in progress; only the final frame carries the full `repeatCount`. The handler now skips intermediate frames (`repeatEnd` falsy) so a single streakable gift is published once instead of twice.
- **Timestamp Issue**: Messages now use TikTok's native `createTime` timestamp instead of generating new ones on receipt
  - Prevents messages from appearing "fresh" when they're actually old/replayed
  - Properly preserves message chronology during reconnects
  
- **Message Replay on Reconnect**: Implemented deduplication using TikTok's native message IDs
  - Uses `data.common.msgId` from TikTok's message structure
  - Prevents duplicate messages from being published when service restarts
  - Reduces unnecessary load on downstream services

### Added
- **Message Deduplication System**:
  - Tracks seen message IDs with 5-minute TTL (configurable via `TIKTOK_DEDUP_TTL_MS`)
  - Automatic cleanup every minute (configurable via `TIKTOK_DEDUP_CLEANUP_INTERVAL_MS`)
  - Bounded cache with 10k message limit (configurable via `TIKTOK_DEDUP_MAX_CACHE_SIZE`)
  - Exposed statistics via `/status` endpoint

- **New Environment Variables**:
  ```bash
  TIKTOK_DEDUP_TTL_MS=300000              # 5 minutes default
  TIKTOK_DEDUP_CLEANUP_INTERVAL_MS=60000  # 1 minute default
  TIKTOK_DEDUP_MAX_CACHE_SIZE=10000       # 10k messages default
  ```

### Changed
- **Resource Limits**:
  - Memory request increased: 128Mi → 256Mi
  - Memory limit increased: 512Mi → 1Gi (better stability under load)
  - CPU limits unchanged (100m request, 500m limit)

- **Message Format**:
  - `message_id`: Now uses TikTok's native `msgId` instead of random UUID
  - `timestamp`: Now uses TikTok's `createTime` converted from Unix timestamp
  - Added `native_msg_id` and `native_create_time` to `tags` for debugging

### Technical Details

#### Before (Issues)
```typescript
// Generated new timestamp on receipt
timestamp: new Date().toISOString(),  // ❌ Always current time

// Generated random UUID
message_id: randomUUID(),  // ❌ No deduplication possible
```

#### After (Fixed)
```typescript
// Use TikTok's native timestamp
const createTime = data.common?.createTime;
const timestampMs = parseInt(createTime) * 1000;
timestamp: new Date(timestampMs).toISOString(),  // ✅ Original message time

// Use TikTok's native message ID
message_id: data.common?.msgId || randomUUID(),  // ✅ Enables deduplication

// Check for duplicates before publishing
if (this.messageDeduplicator.isDuplicate(msgId, username, text)) {
  return; // Skip duplicate
}
```

### Testing

#### Verify Deduplication
1. Start the service
2. Check `/status` endpoint for deduplication stats:
   ```bash
   curl http://localhost:8089/status
   ```
   
   Look for:
   ```json
   {
     "deduplication": {
       "processedCount": 100,
       "duplicateCount": 5,
       "duplicateRatePercent": 5.0,
       "totalEntries": 95
     }
   }
   ```

3. Monitor logs for duplicate detection:
   ```
   [INFO] Duplicate message detected (prevented replay)
   ```

#### Verify Timestamp Preservation
1. Check Redis Stream messages:
   ```bash
   redis-cli XREAD COUNT 1 STREAMS chat:raw 0
   ```

2. Verify timestamp is from TikTok (not current time):
   - `timestamp` should match message's actual send time
   - `native_create_time` in tags shows raw TikTok timestamp

### Migration Notes

**No Breaking Changes**: Existing deployments will automatically benefit from:
- Deduplication (prevents message replay)
- Accurate timestamps (preserves message chronology)
- Better resource stability (increased memory limits)

**Optional Configuration**: Environment variables have sensible defaults and don't require configuration unless you need to tune cache behavior.

### Performance Impact

- **Memory**: +~100-200KB for deduplication cache (10k messages @ ~20 bytes each)
- **CPU**: Negligible (O(1) hash map lookups)
- **Latency**: <1ms per message (hash lookup + timestamp conversion)
- **Duplicate Rate**: Typically 5-10% during reconnects, 0% during normal operation

### Future Enhancements

- [ ] Connection pooling (share connections across overlays)
- [ ] Circuit breaker pattern (prevent rapid reconnection attempts)
- [ ] Message rate limiting and queue management
- [ ] Prometheus metrics for deduplication rates
