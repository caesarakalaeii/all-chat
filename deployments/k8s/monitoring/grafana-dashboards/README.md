# All-Chat Grafana Dashboards

Dashboards are **not** stored in this repo. They live in the GitOps repo, as
ConfigMaps carrying the `grafana_dashboard: "1"` label that Grafana's sidecar
watches:

    caesar-deployment: apps/platform/allchat-monitoring/
      allchat-grafana-dashboard-<slug>.yaml   # the dashboard, JSON embedded
      kustomization.yaml                      # must list the file, or ArgoCD
                                              # reports Synced and creates nothing

The ArgoCD app `allchat-monitoring` syncs that path automatically with
`prune: true` and `selfHeal: true`, so a dashboard applied by hand here would
either drift from git or be reverted. Keeping a second copy in this repo is what
made the alert ConfigMaps diverge from the cluster in the past.

This file exists to document the metrics the All-Chat dashboards read, since
those are produced by code in *this* repo.

## User Growth & Actual Usage

Dashboard uid `allchat-user-growth`, "All-Chat: User Growth & Actual Usage".
Daily / weekly / monthly active streamers, stickiness (DAU/MAU), activation rate,
and sign-ups for context.

| Metric | Emitted by | Notes |
|--------|-----------|-------|
| `allchat_active_users{window="24h"\|"7d"\|"30d"}` | auth-service | DAU/WAU/MAU of real overlay usage, sampled from the database every `USAGE_SAMPLE_INTERVAL_SECONDS` (default 120) |
| `allchat_total_users_by_platform{platform}` | auth-service | Cumulative sign-ups, seeded from the database at startup |
| `allchat_user_registrations_total{platform}` | auth-service | Sign-up counter |

Aggregate the gauges with `max(...)` / `max by (platform) (...)`, never `sum(...)`:
every auth-service replica reports the same fleet-wide value, so summing would
multiply the numbers by the replica count.

### What "active" means

A distinct, non-banned owner of an overlay whose `overlays.last_connected_at`
falls inside the window. api-gateway bumps that column on every demand-bearing
WebSocket attach and on each heartbeat tick (~2 min) while the overlay stays
attached, so it tracks overlays actually being used in a stream, not logins:

- Viewer "participate" sockets are demand-free and deliberately never bump it.
- An overlay created but never opened is excluded (its `last_connected_at` still
  equals its `created_at`) - otherwise the graph would just retell the sign-up
  story.
- Logins are not a usable proxy: `users.updated_at` is churned by the daily
  automated token refresh.

The definition lives in `services/auth-service/repository/usage_repository.go`
and is shared with the admin dashboard's active-user tiles, so the graph and the
admin page cannot drift apart.

### Retroactive history

`allchat_active_users` is a Prometheus gauge, so the graph starts filling when
the metric ships and cannot backfill. For history that predates the rollout,
`stream_sessions` (written by api-gateway on every session start) holds
per-overlay session rows:

```sql
SELECT date_trunc('day', s.started_at) AS day,
       COUNT(DISTINCT o.user_id)       AS daily_active_users
FROM stream_sessions s
JOIN overlays o ON o.id = s.overlay_id
JOIN users u ON u.id = o.user_id
WHERE u.is_banned = false
  AND s.started_at >= NOW() - INTERVAL '180 days'
GROUP BY 1
ORDER BY 1;
```

Graphing that needs a PostgreSQL datasource pointed at the CNPG cluster, which
does not exist yet.
