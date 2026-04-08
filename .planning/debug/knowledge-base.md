# GSD Debug Knowledge Base

Resolved debug sessions. Used by `gsd-debugger` to surface known-pattern hypotheses at the start of new investigations.

---

## kick-messages-not-in-overlay — HeartbeatInterval > HeartbeatTimeout causes false pod failures and dropped assignments
- **Date:** 2026-03-26
- **Error patterns:** assigned=0, no pods available, filtered channels assigned=0, kick assignments cleared, source-manager reconciliation, heartbeat stale, failed_pods
- **Root cause:** HeartbeatInterval (30s) exceeded source-manager HeartbeatTimeout (15s). During reconciliation cycles, listener pods appeared as failed when their heartbeat was between 15–30s old. The source-manager wrote no assignments for those pods, causing listeners to unsubscribe from all channels and stop publishing messages.
- **Fix:** Reduced HeartbeatInterval from 30s to 10s in `shared/listener/config.go` DefaultConfig(). HeartbeatInterval must always be strictly less than HeartbeatTimeout.
- **Files changed:** shared/listener/config.go
---

## production-leadership-tiktok-broken — Both twitch-listener pods joined all 144 channels; readiness probe blocked rolling deploy in leadership mode
- **Date:** 2026-03-28
- **Error patterns:** 144 sources per pod, leadership election bypassed, joinChannelsMultipleConnections, EnsureLeadership, total_active:0, readiness probe 503, channels connecting, initial sync, coordinator heartbeat 404, stuck usernames, SERVICE_JWT_SECRET
- **Root cause:** (1) joinChannelsMultipleConnections never called EnsureLeadership — both pods joined all 144 channels without leadership checks. (2) Readiness probe required activeChannelCount >= filteredAssignmentCount; after fixing (1), the second pod correctly won 0 leadership elections and remained permanently not-ready because filteredAssignmentCount=144 but activeChannelCount=0. (3) TikTok: COORDINATOR_URL/SERVICE_JWT_SECRET/HEARTBEAT_INTERVAL_MS not removed from deployment after coordinator was removed from source-manager, causing TikTok to follow old startup path (heartbeat 404, stale assignments, slow startup).
- **Fix:** (1) Added EnsureLeadership per channel inside joinChannelsMultipleConnections. (2) Added initialSyncDone flag to Manager; updated readiness probe to gate on IsInitialSyncComplete() when IsLeadershipEnabled() instead of channel count. (3) Removed coordinator env vars from tiktok-listener-deployment.yaml.
- **Files changed:** services/twitch-listener/channels/manager.go, services/twitch-listener/channels/manager_test.go, services/twitch-listener/handlers/health.go, caesar-deployment/apps/workloads/all-chat/tiktok-listener-deployment.yaml
---

## new-twitch-source-no-messages — Zombie IRC connection on one pod blocked new channels via unconditional 200 liveness probe
- **Date:** 2026-03-29
- **Error patterns:** no messages, IRC zombie, connectionWatchdog, idle_duration, liveness probe 200, leadership lock, channels not receiving
- **Root cause:** Pod's go-twitch-irc Connect() goroutine became permanently stuck (pinger blocked on TCP write, wg.Wait() never returns). The connectionWatchdog correctly called Disconnect() every 60s but it returned ErrConnectionIsNotOpen (connActive=false during TCP dial retry) — so reconnect never happened. The liveness probe returned HTTP 200 unconditionally, so Kubernetes never restarted the pod. That pod held the leadership lock for the newly-added channel (ironmouse), preventing the healthy peer pod from claiming it.
- **Fix:** (1) Registered OnPingSent callback to update lastActivityAt — go-twitch-irc fires this every 15s (IdlePingInterval default) so it works even on quiet channels with no chat. (2) Added IsStale() returning true when lastActivityAt > 10 minutes old and not zero. (3) Liveness probe returns HTTP 503 when IsStale() — Kubernetes restarts the pod in ~30s, releasing all leadership locks.
- **Files changed:** services/twitch-listener/irc/connection.go, services/twitch-listener/handlers/health.go, services/twitch-listener/irc/connection_stale_test.go, services/twitch-listener/handlers/health_test.go
---

## credits-feature-never-worked — clips_muted column missing from credit_roll_configs; migration 027 failed silently due to table ownership mismatch
- **Date:** 2026-04-06
- **Error patterns:** config not found, failed to get credit roll config, credit roll, creditroll, credits, 500, clips_muted, column does not exist
- **Root cause:** Migration 027 (ADD COLUMN clips_muted) failed silently because credit_roll_configs was owned by the postgres superuser, but the migration runner connects as allchat_user. PostgreSQL requires ownership to ALTER TABLE. ON_ERROR_STOP=0 masked the failure. GetByOverlayID explicitly selects clips_muted, so every SELECT failed with "column clips_muted does not exist", returning HTTP 500 on both public credit roll endpoints.
- **Fix:** (1) Added GetOrCreate to replace GetByOverlayID on public endpoints (deployed first). (2) Added clips_muted column manually via postgres superuser in production pod. (3) Transferred ownership of all postgres-owned tables to allchat_user via REASSIGN OWNED (run as postgres superuser). (4) Added error logging to handler 500 paths.
- **Files changed:** services/overlay-manager/creditroll/handler.go, services/overlay-manager/repository/credit_roll_repo.go
---

## twitch-messages-not-reaching-overlay — SyncChannels write-lock held during 40s rate-limited IRC join loop blocked readiness probe, causing Kubernetes pod cycling
- **Date:** 2026-04-07
- **Error patterns:** context deadline exceeded, readiness probe, TCP timeout, no messages, twitch-listener, caesarlp, SyncChannels, mutex, IRC join rate limit, pods cycling, channel membership lost
- **Root cause:** SyncChannels in channels/manager.go held m.mu (write-lock) via defer m.mu.Unlock() for the entire function body, including the rate-limited IRC join loop (~40s for 80 channels at 20 joins/10s). Readiness probe handlers (GetActiveChannelCount, IsInitialSyncComplete) called m.mu.RLock() which blocked for 40s. Kubernetes readiness probe timeoutSeconds=3 caused TCP timeout (not 503), cycling pods continuously and preventing any IRC session from stabilizing. caesarlp channel memberships were lost on every restart.
- **Fix:** Restructured SyncChannels into brief lock acquisitions only for shared-state reads/writes (m.activeChans, filteredAssignmentCount, initialSyncDone). All IRC operations (Depart, Join, rate-limiter waits, leadership election) now happen outside the mutex. joinChannel and joinChannelsMultipleConnectionsUnlocked each acquire lock only for the single map write. Added TestManager_SyncChannels_DoesNotBlockHealthProbe regression test.
- **Files changed:** services/twitch-listener/channels/manager.go, services/twitch-listener/channels/manager_test.go
---

## twitch-messages-not-reaching-overlay-hesplayingroblox — Twitch pipeline working; stale Redis PEL entries from past pod crashes cleaned up operationally
- **Date:** 2026-03-29
- **Error patterns:** messages not appearing, overlay, twitch-listener, chat:raw, stale PEL, pending messages, XPENDING, XAUTOCLAIM, consumer group, hesplayingroblox
- **Root cause:** Pipeline was fully operational when investigated. The original user report likely coincided with a past pod restart that caused the same race condition as the CaesarLP bug (channel joined on dead IRC client) — already fixed by onConnect callback deployment. 44,632 stale pending entries in chat:raw PEL from pod crashes spanning December 2025 to March 2026 were found but did not block processing (lag=0 for new messages). TikTok listener was also identified as missing SERVICE_JWT_SECRET causing both pods to connect to all streams, producing ~8% duplicate rate in chat:raw.
- **Fix:** Operational cleanup only. Used XAUTOCLAIM + XACK to clear stale PEL entries for processor-1 consumer. No code changes. Separate bug: TikTok listener needs SERVICE_JWT_SECRET env var added to Kubernetes deployment to enable coordinator-based stream assignment.
- **Files changed:** none
---

## viewer-matching-youtube-gradient — Viewer YouTube OAuth stored Google account ID instead of YouTube channel ID; enricher lookup never matched
- **Date:** 2026-04-08
- **Error patterns:** white messages, no gradient, no styling, youtube, viewer, platform_user_id, enricher, cosmetics, channel ID, google account ID, UCRs6QcV9kwHu7V0LLlIvwxQ, 101802728631468199113
- **Root cause:** The viewer YouTube OAuth callback called /oauth2/v2/userinfo which returns a numeric Google account ID. This was stored as platform_user_id in viewer_sessions and viewer_platform_identities. The InnerTube parser sets msg.User.ID = AuthorExternalChannelID (UC... format YouTube channel ID). The enricher lookup WHERE vpi.platform='youtube' AND vpi.platform_user_id=$2 never matched because the two ID systems are incompatible — Google account IDs are numeric strings, YouTube channel IDs start with UC.
- **Fix:** Added GetChannelID() to ViewerYouTubeOAuth calling youtube/v3/channels?part=id&mine=true to fetch the UC... channel ID. Updated HandleYouTubeCallback to store channel ID as platform_user_id for new sessions. Added legacy migration path: on login with unknown channel ID, look up by Google account ID and migrate both tables. Manually migrated existing CaesarLP record in DB. Cleared stale Redis cache keys.
- **Files changed:** services/auth-service/oauth/viewer_youtube.go, services/auth-service/handlers/viewer_auth.go, services/auth-service/repository/viewer_identity_repository.go, services/auth-service/repository/viewer_repository.go, services/auth-service/handlers/viewer_resolve_test.go
---

