# ADR-0006: YouTube Quota Reserve-Confirm-Rollback

**Date**: 2025-11-15
**Status**: Accepted
**Revised**: 2026-11-18
**Deciders**: YouTube Listener Team, Infrastructure Lead

## Context

YouTube API quota is tracked in a shared `youtube_quota_usage` table (the project's daily limit is 1,009,000 units, migration 066 and `shared/quota.DefaultDailyQuota`; mostly unused today because the default chat path runs on InnerTube quota-free, ADR-0065). Quota consumers: the premium official-API listener (`youtube-listener`: `liveChatMessages.list` 5 units, `videos.list` 1, `liveBroadcasts.list` 1, `channels.list(mine=true)` 1), `moderation-service` bans (50 units), `auth-service` streamer sends (5 units).

The first implementation was a plain increment after the API call. Across concurrent listener replicas this drifted by several hundred units a day (~5%): a crash between the call and the increment loses tracking forever, and units were even charged for 4xx errors the listener itself caused.

## Decision

Track quota with a reserve-confirm-rollback pattern:

- Before each official-API call, `ReserveQuota(units)` runs the SQL function `reserve_youtube_quota(date, units, limit)`, which locks the day's row `FOR UPDATE`, creates it when missing, and fails when `used + reserved + requested > limit`.
- After the call, the reservation is settled with `ConfirmReservation` (success, or 5xx: YouTube charges anyway) or `RollbackReservation` (failure classified by `shouldChargeQuota` as not having reached YouTube, i.e. 4xx).
- In-flight units live in a `units_reserved` counter on the day's row (no reservations table; reservation ids are local bookkeeping). `cleanup_stale_quota_reservations()` recovers `units_reserved` older than 5 minutes on past-date rows, so a crash mid-call leaks nothing.

This eliminated the drift (decision-time measurements moved from ~±500 units/day to ~±1 unit, 99.95%+ accuracy over a 7-day run) and makes consumption predictable across the several consumers of the shared table.

Alternatives rejected: the simple counter (drift, no crash recovery); row-level locking around an increment (fixes races but still charges for 4xx and still loses tracking on a crash between call and record).

## Implementation

- SQL: `migrations/003_youtube_support.sql` (table), `migrations/008_quota_reservations.sql` (`units_reserved`, the reserve/confirm/rollback/cleanup functions, `get_youtube_quota_with_reserved`), `migrations/016_youtube_quota_audit_log.sql` (per-call audit via `LogAPICall`).
- Go: `services/youtube-listener/quota/tracker.go` (`Tracker` with `Start`, `ReserveQuota`, `ReserveQuotaWithPriority`, `ConfirmReservation`, `RollbackReservation`, `RecordUsage` for plain increments). The stale-reservation sweep also runs in `services/youtube-quota-monitor/cmd/main.go`.
- Callers in `services/youtube-listener/api/client.go` and `grpc_client.go` inline reserve, then confirm-or-rollback, around every official-API call. `ReserveQuotaWithPriority` blocks non-critical reservations at or above the emergency threshold (default 90% of the daily limit) unless `allowCritical=true` (OAuth, login).
- `auth-service` and `moderation-service` use the shared `shared/quota` package directly via SQL (ADR-0023); the listener exposes HTTP endpoints (`/v1/quota/check|reserve|confirm` in `cmd/main.go`) for services not on `shared/quota`.
- Metrics, `/quota/status` and the `YouTubeQuotaHigh|Critical|Exhausted` alerts are owned by `youtube-quota-monitor` (ADR-0023).

Related: ADR-0023 (quota monitor), ADR-0065 (official-API listener), [QUICK-REF-DEBUG-QUOTA.md](../llm-guides/QUICK-REF-DEBUG-QUOTA.md).
