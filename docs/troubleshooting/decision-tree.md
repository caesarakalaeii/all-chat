# Troubleshooting triage

Start from the symptom, then follow the specific guide. Most issues are a pod health problem, a dependency connection failure, or a break somewhere along the message flow

```bash
# 1. Are the pods healthy?
kubectl get pods -n allchat

# 2. What does the failing service say?
kubectl logs -n allchat deploy/<service> --tail=100

# 3. Is it ready?
kubectl port-forward -n allchat deploy/<service> <port>:<port> >/dev/null &
sleep 1; curl -s http://localhost:<port>/health/ready; kill $!
```

## Guides

| Symptom | Guide |
|---|---|
| Build / compilation / startup failure | [build-errors.md](./build-errors.md) |
| Cannot connect to PostgreSQL or Redis | [connection-errors.md](./connection-errors.md) |
| YouTube: no messages or quota exhausted | [youtube.md](./youtube.md) |
| WebSocket disconnects, overlays not updating | [websocket-disconnects.md](./websocket-disconnects.md) |
| Other platform listeners | `services/<platform>-listener/README.md` |
| Innertube Argo Rollout problems | [youtube.md](./youtube.md#innertube-argo-rollout-problems) |
| Altstream SSE connects but no messages, or 502s | [platform-listener-deployment.md](../runbooks/platform-listener-deployment.md), troubleshooting section |

## Tracing the message flow

Listener -> Redis Stream `chat:raw` -> message-processor -> Redis Pub/Sub `overlay:<id>` -> API Gateway -> browser overlay. To find where messages stop:

```bash
kubectl exec -n allchat deploy/redis -- redis-cli XINFO STREAM chat:raw      # messages arriving at all?
kubectl exec -n allchat deploy/redis -- redis-cli XINFO GROUPS chat:raw      # message-processor consuming, lag 0?
kubectl exec -n allchat deploy/redis -- redis-cli PUBSUB CHANNELS "overlay:*" # publishing reaching overlays?
```

Per-platform message questions go to the relevant listener README under `services/`. The Twitch IRC listener is retired ([ADR-0026](../adr/0026-twitch-irc-listener-deprecation.md)); chat runs on EventSub, see `services/twitch-eventsub-listener/README.md`.
