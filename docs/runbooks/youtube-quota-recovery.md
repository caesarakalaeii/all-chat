# YouTube quota recovery

Quota exhausted on the official `youtube-listener` (Data API). Quota tracking design: [ADR-0006](../adr/0006-youtube-quota-tracking.md).

## Confirm

```bash
kubectl port-forward -n allchat deployment/youtube-listener 8086:8086 >/dev/null &
sleep 1; curl -s http://localhost:8086/quota/status | jq .global; kill $!
```

The project quota is 1,009,000 units/day and resets at midnight Pacific Time.

## Recover

- **Wait for the reset (default).** The listener detects the reset and resumes polling on its own; nothing to do:

  ```bash
  watch -n 60 'kubectl port-forward -n allchat deployment/youtube-listener 8086:8086 >/dev/null & sleep 1; curl -s http://localhost:8086/quota/status | jq .global.state; kill %1'
  # DEPLETED -> HEALTHY after midnight PT
  ```

- **Emergency: disable low-priority sources to redirect remaining units.** High-priority channels keep working; re-enable after the reset:
  ```bash
  PRIMARY=$(kubectl get pods -n allchat -l cnpg.io/cluster=allchat-cluster,cnpg.io/instanceRole=primary -o jsonpath='{.items[0].metadata.name}')
  kubectl exec -n allchat "$PRIMARY" -- psql -U allchat -c "
    UPDATE overlay_chat_sources SET is_active = false
    WHERE platform = 'youtube' AND priority > 1;"
  # after reset:
  kubectl exec -n allchat "$PRIMARY" -- psql -U allchat -c "
    UPDATE overlay_chat_sources SET is_active = true WHERE platform = 'youtube';"
  ```

- **Long term:** request a higher Google Cloud quota (APIs & Services → YouTube Data API v3 → Quotas), or offload traffic to `youtube-listener-innertube` (quota-free ingestion, see its README).

## Verify

```bash
curl -s http://localhost:8086/quota/status | jq .global.state        # HEALTHY
kubectl logs -n allchat deployment/youtube-listener --tail=50        # "Polling chat messages" lines
kubectl exec -n allchat deploy/redis -- redis-cli XREVRANGE chat:raw + - COUNT 5 | grep -i youtube
```

Full diagnosis (state machine, reservations, drift): [troubleshooting/youtube.md](../troubleshooting/youtube.md).
