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

