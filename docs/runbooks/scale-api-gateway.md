# Scale the API gateway

For more WebSocket capacity. Check the current state first:

```bash
kubectl get hpa -n allchat api-gateway
# current replicas vs max (base HPA: min 2, max 10, targets 65% CPU, 75% memory)
kubectl top pods -n allchat -l app=api-gateway
kubectl logs -n allchat deployment/api-gateway --tail=100
```

## Horizontal: raise the HPA max

```bash
kubectl patch hpa api-gateway -n allchat -p '{"spec":{"maxReplicas":15}}'
kubectl get hpa -n allchat api-gateway
```

If the higher max should persist, change `deployments/k8s/base/api-gateway/hpa.yaml` (or the deployment-repo overlay) through the normal pipeline instead of leaving the patch unmanaged.

## Vertical: raise resource limits

Edit the container resources in the deployment manifests (base is 256Mi/200m requests, 1Gi/1000m limits) and let the deploy pipeline roll it:

```bash
kubectl rollout status deployment/api-gateway -n allchat --timeout=5m
```

## Verify

```bash
kubectl get pods -n allchat -l app=api-gateway
```

Connections are per pod; the gauge is labelled by pod:

```bash
kubectl port-forward -n allchat deploy/api-gateway 8080:8080 >/dev/null &
sleep 1; curl -s http://localhost:8080/metrics | grep gateway_websocket_connections_active; kill $!
```

Query all pods through Prometheus instead of port-forwarding each when checking distribution.

## Revert

```bash
kubectl patch hpa api-gateway -n allchat -p '{"spec":{"maxReplicas":10}}'
# or, for manifest changes:
kubectl -n allchat rollout undo deployment/api-gateway
```
