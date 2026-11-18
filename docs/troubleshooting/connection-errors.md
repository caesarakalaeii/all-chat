# Troubleshooting: connection errors

PostgreSQL and Redis connection failures.

## PostgreSQL

```bash
# dev: docker-compose.frontend.yml only; full dev uses the Makefile targets
docker compose -f docker-compose.frontend.yml ps postgres
# kubernetes
kubectl get pods -n allchat -l cnpg.io/cluster=allchat-cluster
```

- `connection refused`: PostgreSQL not running. Local: `docker compose -f docker-compose.frontend.yml up -d postgres`. Cluster: check the CNPG pods (in-cluster host is `allchat-cluster-rw.allchat.svc.cluster.local`).
- `password authentication failed for user "allchat"`: wrong `DATABASE_USER`/`DATABASE_PASSWORD`. They come from the `allchat-config` ConfigMap and the `allchat-secrets` Secret.
- `database "allchat" does not exist`: run `make migrate` after creating the database.
- `i/o timeout`: env var points at the wrong host or a NetworkPolicy blocks it. Dev expects `DATABASE_HOST=localhost`; in-cluster the CNPG read-write service.

## Redis

```bash
docker compose -f deployments/docker-compose.yml ps redis   # dev (full compose)
kubectl get pods -n allchat -l app=redis
```

- `connection refused`: Redis not running; start it (`docker compose -f deployments/docker-compose.yml up -d redis` locally).
- `NOAUTH Authentication required`: `REDIS_PASSWORD` mismatch. Prod currently runs Redis without AUTH.
- `i/o timeout`: wrong `REDIS_HOST` (dev: `localhost`) or blocked by NetworkPolicy.

## Readiness probe fails (503 from /health/ready)

`services/*/cmd` health handlers check DB and Redis; a 503 is usually one of those connections. Check the service logs at the time of the probe failure and the two sections above. Probe paths are `/health/live` (always 200) and `/health/ready`.
