# Quick Reference: Kubernetes Debugging

Goal: debug common Kubernetes issues with All-Chat services.

```bash
kubectl get pods -n allchat
kubectl describe pod <pod> -n allchat
kubectl get events -n allchat --sort-by='.lastTimestamp' | tail -20

kubectl logs -n allchat <pod> --previous                     # after a crash
kubectl logs -n allchat -l app=twitch-listener --tail=100

kubectl debug -n allchat <pod> -it --image=busybox --target=<container> -- sh   # distroless images, ADR-0092: no shell in containers
kubectl port-forward -n allchat pod/<pod> 8080:8080 >/dev/null &
sleep 1; curl -s http://localhost:8080/health/ready; kill $!
```

## Common issues

- **CrashLoopBackOff**: read `kubectl logs --previous`. Typical causes: DB/Redis connection failure, missing env vars, port in use.
- **ImagePullBackOff**: Events section of `kubectl describe pod`; wrong image tag or registry auth failure.
- **Pending**: Events usually say "Insufficient cpu/memory"; scale nodes or lower resource requests.

## Service discovery

```bash
kubectl get svc -n allchat
kubectl get endpoints <service> -n allchat    # empty = selector/label mismatch
kubectl get pods -n allchat --show-labels
```

References: [docs/architecture/02-DEPLOYMENT.md](../architecture/02-DEPLOYMENT.md), [docs/architecture/03-SCALING.md](../architecture/03-SCALING.md).
