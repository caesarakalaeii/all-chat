# Runbook: Scale API Gateway

**Scenario**: Increase API Gateway capacity for more WebSocket connections

**Time Estimate**: 15-30 minutes
**Risk**: Low (rolling update, zero downtime)

---

## When to Scale

**Indicators**:
- WebSocket connections approaching 2,000 per pod (limit: 2,500)
- HPA already at max replicas (20 pods)
- CPU consistently >70%
- Memory consistently >70%

**Check current capacity**:
```bash
kubectl get hpa -n allchat api-gateway
# Check CURRENT replicas vs MAX replicas
```

---

## Scaling Options

### Option 1: Increase HPA Max Replicas (Horizontal)

**Best for**: More connections needed, current pods not saturated

```bash
# Edit HPA
kubectl edit hpa api-gateway -n allchat

# Change:
# spec.maxReplicas: 20  →  spec.maxReplicas: 30

# Or patch:
kubectl patch hpa api-gateway -n allchat -p '{"spec":{"maxReplicas":30}}'

# Verify
kubectl get hpa -n allchat api-gateway
```

### Option 2: Increase Resource Limits (Vertical)

**Best for**: Pods running out of memory/CPU

```bash
# Edit deployment
kubectl edit deployment api-gateway -n allchat

# Change:
# resources.limits.memory: 512Mi  →  1Gi
# resources.limits.cpu: 500m  →  1000m

# Trigger rolling update
kubectl rollout restart deployment/api-gateway -n allchat
kubectl rollout status deployment/api-gateway -n allchat
```

---

## Verification

```bash
# Check pod count increased
kubectl get pods -n allchat -l app=api-gateway

# Check connections distributed
for pod in $(kubectl get pods -n allchat -l app=api-gateway -o name); do
  echo "$pod:"
  kubectl exec -n allchat $pod -- wget -qO- http://localhost:8080/metrics | grep websocket_connections_active
done
```

---

## Rollback

```bash
# Revert HPA max replicas
kubectl patch hpa api-gateway -n allchat -p '{"spec":{"maxReplicas":20}}'

# Or revert resource limits (triggers rolling update)
kubectl rollout undo deployment/api-gateway -n allchat
```
