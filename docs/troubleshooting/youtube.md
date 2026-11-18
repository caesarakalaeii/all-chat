# Troubleshooting: YouTube

Covers the official YouTube listener (Data API, quota) and the Innertube listener. See `services/youtube-listener/README.md` and `services/youtube-listener-innertube/README.md` for service architecture. Quota tracking design is in [ADR-0006](../adr/0006-youtube-quota-tracking.md).

## Listener down / no YouTube messages

1. Check pods and logs:

   ```bash
   kubectl get pods -n allchat -l app=youtube-listener
   kubectl logs -n allchat -l app=youtube-listener --tail=100
   # Innertube:
   kubectl logs -n allchat -l app=youtube-listener-innertube --tail=100
   ```

2. Common causes:
   - OAuth token expired or invalid. Check `youtube_oauth_tokens` in the database, re-authorize through the auth-service OAuth flow, and confirm token-refresh-service logs show refreshes.
   - Redis connection failures. Grep the listener logs for `redis`.
   - Source-manager assignment problems. Check `kubectl logs -n allchat -l app=source-manager --tail=100`: the channel must be assigned to a live listener for polling to start. [ADR-0006] discovery claims are leadership-coordinated; only the global-sync leader performs discovery.
   - Channel actually live? Verify on YouTube before debugging further.
   - Innertube: YouTube changes parser shapes frequently. A drop in message rate right after a YouTube change is usually parser drift; reproduce from `services/youtube-listener-innertube` logs.

## Quota exceeded (official listener)
The listener polls the Data API and also exposes the gRPC `liveChatMessages.streamList` path for premium official-API mode; a full connection costs 5 units (see `services/youtube-listener/README.md` for the cost table). A stream connection that drops and reconnects every few seconds burns quota fast; `recv_duration` or `stream_duration` in the logs should show long-lived connections, not a repeated short cycle. Listener source: `services/youtube-listener/api/grpc_client.go`.

### Diagnosis

  ```bash
  kubectl port-forward -n allchat deployment/youtube-listener 8086:8086 >/dev/null &
  sleep 1; curl -s http://localhost:8086/quota/status | jq .global; kill $!
  ```

States: `HEALTHY` (normal), `DEGRADED` (70-85%), `CRITICAL` (85-95%, new discoveries stop), `EXHAUSTED` (95-100%, polling for new streams stops), `DEPLETED` (over 100%, all polling stops).

### Recovery

- Wait for reset. Quota resets at midnight Pacific Time; the service detects the reset automatically:

  ```bash
  curl -s http://localhost:8086/quota/status | jq .global.resets_at
  TZ=America/Los_Angeles date
  ```

- Stale reservations. If quota stays high after a reset, reservations were never confirmed or rolled back:

  ```sql
  SELECT * FROM youtube_quota_reservations WHERE date = CURRENT_DATE;
  SELECT cleanup_stale_quota_reservations();
  ```

- Tracking drift is normally within a few units. Larger drifts show in `youtube_quota_usage`:

  ```sql
  SELECT date, units_used, units_reserved, units_limit
  FROM youtube_quota_usage WHERE date = CURRENT_DATE;
  ```

- Long term: request a quota increase in Google Cloud Console
  (APIs & Services → YouTube Data API v3 → Quotas). The project already
  runs at 1,009,000 units/day, up from the default 10,000.

## Innertube Argo Rollout problems

The Innertube service ships through an Argo Rollout with an analysis run against Prometheus metrics. Manifests and thresholds: [architecture/02-DEPLOYMENT.md](../architecture/02-DEPLOYMENT.md).

| Symptom | Where to look |
|---|---|
| Rollout status `Degraded`, canary traffic back to 0% | `kubectl describe analysisrun <name> -n allchat`: `error-rate` exceeded 5% or `message-rate-deviation` exceeded 20% vs baseline |
| Stuck at a canary weight for hours | `kubectl get analysisrun -n allchat`: analysis needs sustained samples below threshold, intermittent spikes reset the counter |
| Canary pods crashlooping | `kubectl logs <pod> -n allchat --previous` and `kubectl describe pod`; missing env vars typically show on first boot |
| Analysis queries return no data | ServiceMonitor and Prometheus target for `youtube-listener-innertube` must be UP |

Emergency manual promote (only when metrics are clearly healthy but the analysis is not progressing for a technical reason):

```bash
kubectl argo rollouts promote youtube-listener-innertube -n allchat
```

Thundering herd on rollback: the official listener reabsorbs traffic; if its HPA lags, scale it manually (`kubectl scale deployment youtube-listener -n allchat --replicas=15`) and revert after the rollout stabilizes.
