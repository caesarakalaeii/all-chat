# Quick Reference: Kubernetes Debugging

**Time Estimate**: 30-60 minutes | **Difficulty**: ⭐⭐ Moderate

**Goal**: Debug common Kubernetes issues with All-Chat services.

---

## Quick Commands

### Pod Status

```bash
# List all pods
kubectl get pods -n allchat

# Get pod details
kubectl describe pod <pod-name> -n allchat

# Check pod events
kubectl get events -n allchat --sort-by='.lastTimestamp' | tail -20
```

### Logs

```bash
# View logs
kubectl logs -n allchat <pod-name>

# Follow logs
kubectl logs -n allchat <pod-name> -f

# Previous container (after crash)
kubectl logs -n allchat <pod-name> --previous

# All replicas for a service
kubectl logs -n allchat -l app=twitch-listener --tail=100
```

### Execute Commands in Pod

```bash
# Shell access
kubectl exec -n allchat <pod-name> -it -- /bin/sh

# One-off command
kubectl exec -n allchat <pod-name> -- wget -qO- http://localhost:8080/health/ready
```

---

## Common Issues

### CrashLoopBackOff

**Symptom**: Pod restarting repeatedly

**Diagnosis**:
```bash
# Check previous logs
kubectl logs -n allchat <pod-name> --previous

# Common causes:
# - Database connection failed
# - Missing environment variables
# - Port already in use
```

### ImagePullBackOff

**Symptom**: Cannot pull container image

**Check image**:
```bash
kubectl describe pod <pod-name> -n allchat | grep -A 5 Events

# Common causes:
# - Image doesn't exist in registry
# - Wrong image tag
# - Registry authentication failed
```

### Pending (No Resources)

**Symptom**: Pod stuck in Pending state

**Check resources**:
```bash
kubectl describe pod <pod-name> -n allchat | grep -A 10 Events

# Common cause: "Insufficient cpu/memory"
# Solution: Scale nodes or reduce resource requests
```

---

## Service Discovery Issues

**Check Service**:
```bash
# Verify service exists
kubectl get svc -n allchat

# Check endpoints
kubectl get endpoints <service-name> -n allchat

# Should list pod IPs (if empty, selector mismatch)
```

**Check Labels**:
```bash
kubectl get pods -n allchat --show-labels
kubectl get svc <service-name> -n allchat -o yaml | grep selector
```

---

## Related Documentation

- [02-DEPLOYMENT.md](../architecture/02-DEPLOYMENT.md) - K8s architecture
- [03-SCALING.md](../architecture/03-SCALING.md) - HPA, resource limits
