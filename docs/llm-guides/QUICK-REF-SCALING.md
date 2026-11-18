# Quick Reference: Scaling Services

Goal: scale All-Chat services horizontally.

## Commands

```bash
kubectl scale deployment <service> --replicas=5 -n allchat
kubectl get hpa -n allchat                # HPA status if configured
kubectl top pods -n allchat               # per-pod resource usage
```

## When to scale

| Service | Signals |
|---|---|
| api-gateway | WebSocket connections >2,000 per pod; CPU >60%; memory >70% |
| message-processor | Consumer lag >10,000 (Redis XPENDING); CPU >70%; P95 latency >500ms |
| listeners (twitch/youtube/kick) | Active channels/streams >400 per pod; CPU >70% |

## HPA

```bash
kubectl edit hpa <service> -n allchat   # adjust spec.minReplicas / maxReplicas
```

See [docs/architecture/03-SCALING.md](../architecture/03-SCALING.md) for strategy details and [04-OBSERVABILITY.md](../architecture/04-OBSERVABILITY.md) for the metrics behind these thresholds.
