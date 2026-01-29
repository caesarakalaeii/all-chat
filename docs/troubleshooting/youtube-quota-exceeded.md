# Troubleshooting: YouTube Quota Exceeded

YouTube API quota exhaustion diagnosis and recovery.

---

## Quick Diagnosis

```bash
# Check quota status
curl http://localhost:8086/quota/status | jq .global.state

# Possible states:
# - HEALTHY (0-70%): ✅ Normal
# - DEGRADED (70-85%): ⚠️ Monitor
# - CRITICAL (85-95%): 🚨 Reduce usage
# - EXHAUSTED (95-100%): ⛔ New discoveries stopped
# - DEPLETED (>100%): 🛑 All polling stopped
```

---

## Recovery Steps

### EXHAUSTED or DEPLETED State

**Immediate Action**: Wait for quota reset at midnight Pacific Time

```bash
# Check reset time
curl http://localhost:8086/quota/status | jq .global.resets_at
# Output: "2026-01-29T00:00:00-08:00"

# Service automatically resumes after reset
```

**Long-Term Solution**: Request quota increase from Google (1,000,000 units/day)

### High Reserved Units (>50)

**Symptom**: Quota not releasing after API calls

**Check reservations**:
```sql
SELECT * FROM youtube_quota_reservations WHERE date = CURRENT_DATE;

# Clean up stale reservations
SELECT cleanup_stale_quota_reservations();
```

### Tracking Drift

**Symptom**: Database quota differs from expected usage

**Expected**: ±5 units drift (99.95% accuracy with reserve-confirm-rollback)

**Check**:
```sql
SELECT date, units_used, units_reserved, units_limit
FROM youtube_quota_usage
WHERE date = CURRENT_DATE;
```

---

## Related Documentation

- [QUICK-REF-DEBUG-QUOTA.md](../llm-guides/QUICK-REF-DEBUG-QUOTA.md) - Comprehensive quota debugging
- [youtube-listener/README.md](../../services/youtube-listener/README.md) - Complete service documentation
- [ADR-0006](../adr/0006-youtube-quota-tracking.md) - Quota tracking architecture
