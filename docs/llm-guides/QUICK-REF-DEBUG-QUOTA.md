# Quick Reference: Debug YouTube Quota Issues

Goal: diagnose YouTube API quota exhaustion or tracking drift. Implementation: `services/youtube-listener/quota/tracker.go`, ADR-0006 (reserve-confirm-rollback), ADR-0065 (official-API mode is opted-in premium channels only; all others come from InnerTube quota-free).

## Check status

```bash
curl http://localhost:8086/quota/status | jq .            # local
kubectl port-forward -n allchat deployment/youtube-listener 8086:8086 >/dev/null &
sleep 1; curl -s http://localhost:8086/quota/status | jq .; kill $!
```

Also available: `/quota/history?days=7`, `/quota/channels/:channel_id`, `/quota/predictions`, `/quota/circuit-breakers`, and `/status` (service + active streams).

## State machine

| State | Usage | Behavior |
|---|---|---|
| HEALTHY | 0-70% | Normal operation |
| DEGRADED | 70-85% | Reduce non-critical polling |
| CRITICAL | 85-95% | Only high-priority channels polled |
| EXHAUSTED | 95-100% | No new stream discovery; existing polling continues |
| DEPLETED | >100% | All polling stopped until reset |

Defaults in `tracker.go`; configurable via `QUOTA_HEALTHY_THRESHOLD` etc.

## Common issues

### EXHAUSTED / DEPLETED
Daily limit (`DefaultDailyQuota = 1009000` units) reached. Quota resets at midnight Pacific; check `.global.resets_at`. Recovery is automatic after reset. For a lasting fix, request a higher per-day quota in Google Cloud Console (APIs and Services, YouTube Data API v3).

### Tracking drift
Compare the database against the YouTube console (`youtube_quota_usage`); expected drift is small from reserve-confirm-rollback.

```sql
SELECT date, units_used, units_reserved, units_limit
FROM youtube_quota_usage WHERE date = CURRENT_DATE;

SELECT operation_type, units, created_at FROM youtube_quota_reservations
WHERE date = CURRENT_DATE ORDER BY created_at DESC LIMIT 20;
```

Reservations should be seconds old; a background goroutine rolls back reservations older than 5 minutes and calls `cleanup_stale_quota_reservations()`. High `units_reserved` means crashes or YouTube timeouts leaving unconfirmed reservations.

### Usage higher than expected
Other services consume quota, e.g. overlay-manager resolving a YouTube `@handle` to a channel id (a `search.list`, ~100 units, logged as `channel_resolve`). Check `kubectl logs -n allchat deployment/overlay-manager` for quota-recording lines.

### Polling not resuming after reset
Restart the service and verify the state again after reset time.

## Cost reference

All Data API v3 calls cost 1 unit unless listed otherwise. Per official-API polling: roughly 1-2 units per minute per offline opted-in channel while its overlay is connected, plus 5 units per gRPC `streamList` chat connection (not per message batch). See ADR-0065 for why non-opted-in channels cost nothing.

## Recovery

```bash
# Rolling restart (safe)
kubectl rollout restart deployment/youtube-listener -n allchat
kubectl rollout status deployment/youtube-listener -n allchat

# Reduce consumption while exhausted
kubectl scale deployment/youtube-listener --replicas=1 -n allchat
```

Dev-only emergency reset: zero `units_used`/`units_reserved` in `youtube_quota_usage` for today and clear `youtube_quota_reservations`, then restart. Never in prod.

## Monitoring

`youtube-quota-monitor` reads the shared `youtube_quota_usage` table, exports the Prometheus quota metrics, and publishes state-transition events to `quota:alerts` for the discord-bot. Alert when quota percentage exceeds 70 and when reserved units sit above 50.

## Links

- [services/youtube-listener/README.md](../../services/youtube-listener/README.md)
- [services/youtube-quota-monitor/README.md](../../services/youtube-quota-monitor/README.md)
- [docs/llm-guides/QUICK-REF-REDIS-OPERATIONS.md](QUICK-REF-REDIS-OPERATIONS.md) (checking Streams/Pub/Sub if messages stop)
