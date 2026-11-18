# Recover from a Redis outage

Redis down: services lose Pub/Sub delivery until it returns; the `chat:raw` stream and PostgreSQL are not affected.

## Check

```bash
kubectl get pods -n allchat -l app=redis
kubectl logs -n allchat -l app=redis --tail=100
kubectl exec -n allchat deployment/redis -- redis-cli ping   # expect PONG
```

## Restart

```bash
kubectl delete pod -n allchat -l app=redis              # Deployment recreates it (PVC-backed, no StatefulSet)
kubectl wait --for=condition=Ready -n allchat -l app=redis pod --timeout=60s
kubectl exec -n allchat deploy/redis -- redis-cli DBSIZE   # keys recovered from AOF
```

## Verify services reconnected

All services reconnect automatically. Check logs for reconnection lines:

```bash
for d in message-processor api-gateway twitch-listener twitch-eventsub-listener; do
  kubectl logs -n allchat deployment/$d --tail=50 | grep -i redis
done
```

If a service did not recover:

```bash
kubectl rollout restart deployment/<service> -n allchat
kubectl rollout status deployment/<service> -n allchat --timeout=5m
```

## What was lost

- Redis Streams (`chat:raw`): persisted via AOF, no data loss.
- Redis Pub/Sub (`overlay:*`): not persisted; messages published during the outage are gone. Overlays show a brief gap and reconnect on their own.
- PostgreSQL: unaffected.

## Confirm flow is back

```bash
kubectl exec -n allchat deploy/redis -- redis-cli XINFO STREAM chat:raw
kubectl exec -n allchat deploy/redis -- redis-cli XPENDING chat:raw message-processor   # lag near zero
kubectl exec -n allchat deploy/redis -- redis-cli PUBSUB CHANNELS "overlay:*"
```
