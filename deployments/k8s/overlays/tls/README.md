# TLS overlay (M14)

Adds TLS termination for the All-Chat API gateway via an Ingress +
cert-manager.

## What this fixes

`../../../../docs/security/SECURITY_AUDIT_REPORT.md` finding **M14** — the base manifests expose
`api-gateway` as a raw `LoadBalancer` on plain HTTP (port 8080) with no TLS
resource, no Ingress, and no cert-manager integration.

## Files

- `ingress.yaml` — `Ingress` for `allch.at` / `www.allch.at` → `api-gateway:8080`,
  TLS via `secretName: allchat-tls`, WebSocket upgrade snippets for chat
  overlays.
- `cluster-issuer.yaml` — cert-manager `ClusterIssuer` (Let's Encrypt prod,
  HTTP-01 solver, nginx ingress class).

## Enable

```bash
# 1. Install cert-manager (if not present).
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.16.3/cert-manager.yaml

# 2. Edit cluster-issuer.yaml — set a real operations email.
# 3. Apply the issuer and ingress.
kubectl apply -f deployments/k8s/overlays/tls/cluster-issuer.yaml
kubectl apply -f deployments/k8s/overlays/tls/ingress.yaml

# 4. Point allch.at DNS at the ingress controller's external IP.
kubectl get ingress -n allchat
```

## Notes

- These manifests are **not** wired into `deployments/k8s/base/kustomization.yaml`
  so the base layer stays self-contained and does not require cert-manager or an
  ingress controller. Enable them as an overlay.
- The base `api-gateway` `Service` remains `type: LoadBalancer`. Operators that
  terminate TLS at a cloud load balancer instead can skip this overlay and
  configure TLS at the LB, but should still ensure HTTP→HTTPS redirect.
- Replace the placeholder email in `cluster-issuer.yaml` before applying.
