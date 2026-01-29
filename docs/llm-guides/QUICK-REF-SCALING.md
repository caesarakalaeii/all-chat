# Quick Reference: Scaling Services

**Time Estimate**: 30-60 minutes | **Difficulty**: ⭐⭐ Moderate

**Goal**: Scale All-Chat services horizontally or vertically based on load.

---

## Quick Scaling Commands

### Horizontal Scaling (Add Replicas)

```bash
# Scale deployment manually
kubectl scale deployment <service> --replicas=5 -n allchat

# Check scaling status
kubectl get deployment <service> -n allchat

# Check HPA status (if configured)
kubectl get hpa -n allchat
```

### Check Current Resource Usage

```bash
# Pod resource usage
kubectl top pods -n allchat

# Node resource usage
kubectl top nodes
```

---

## Service-Specific Scaling

### API Gateway

**Scale when**:
- WebSocket connections >2,000 per pod
- CPU >60%
- Memory >70%

```bash
kubectl scale deployment api-gateway --replicas=6 -n allchat
```

### Message Processor

**Scale when**:
- Consumer lag >10,000 messages (XPENDING)
- CPU >70%
- Processing latency P95 >500ms

```bash
kubectl scale deployment message-processor --replicas=7 -n allchat
```

### Listeners (Twitch/YouTube/Kick)

**Scale when**:
- Active channels/streams >400 per pod
- CPU >70%

```bash
kubectl scale deployment twitch-listener --replicas=3 -n allchat
```

---

## HPA Configuration

### Edit HPA

```bash
kubectl edit hpa <service> -n allchat

# Change minReplicas/maxReplicas
spec:
  minReplicas: 3
  maxReplicas: 10
```

---

## Related Documentation

- [03-SCALING.md](../architecture/03-SCALING.md) - Detailed scaling strategies
- [04-OBSERVABILITY.md](../architecture/04-OBSERVABILITY.md) - Metrics to monitor
