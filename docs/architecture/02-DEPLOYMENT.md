# Kubernetes Deployment

How All-Chat runs in production (Kubernetes, namespace `allchat`) and how a change gets there.

Related: [Architecture Overview](./00-OVERVIEW.md), [Scaling](./03-SCALING.md), [Observability](./04-OBSERVABILITY.md), ADR-0003 (Storage), ADR-0065 (two YouTube listeners), ADR-0092 (distroless images).

## How a change reaches production

1. Open a PR. GitHub Actions (`.github/workflows/build-and-push.yml`) runs the Go and frontend tests and builds the changed services.
2. Merge to `main`. The workflow builds images for every changed service (a change under `shared/` rebuilds all Go services) and pushes them to `ghcr.io/caesarakalaeii/allchat-<service>:main`, tagged `type=ref,event=branch`, plus `latest` on the default branch.
3. Keel picks up the new `:main` tag and rolls every Deployment that uses it. There is no kubectl apply step in the shipping path.
4. Beta: merges to the `beta` branch build `:beta` images. Only `frontend` and `api-gateway` have beta deployments (`frontend-beta`, `api-gateway-beta`, serving beta.allch.at); every other service runs from `:main` only, so there is no beta environment for them and each backend feature needs a human call on how to stage it.

Migrations are part of the same flow: `migrations/Dockerfile` builds `allchat-migrations:main` (alpine, `scripts/run-migrations.sh` around `psql` + `pg_isready`) and it runs as an init container in the pod spec of each service that connects to the database, so a new service image and its migration move together. The runner takes a Postgres advisory lock (id 8439271), waits up to ~120 s for the database, runs every `NNN_*.sql` except `*_down.sql` in lexicographic order with `ON_ERROR_STOP=1`, and propagates psql's exit code. A new migration MUST be idempotent on re-run (CREATE ... IF NOT EXISTS, ON CONFLICT DO NOTHING), because a pod restart re-runs the init container against a database where it already succeeded. ADR-0092 covers why the runner stays on alpine while every service image is distroless.

## What the base manifests are for

`deployments/k8s/base/` is a kustomize overlay (`kubectl apply -k deployments/k8s/base/`) with one Deployment (+ optional `hpa.yaml`) per service, shared `configmap.yaml`, `networkpolicies.yaml`, and the `images:` block that rewrites local image names to `ghcr.io/caesarakalaeii/allchat-*:main`. Keep it as a self-contained local/dev layer; production is managed in the deploy repository.

ConfigMap `allchat-config` holds non-secret settings (service URLs, `DATABASE_HOST: allchat-cluster-rw.allchat.svc.cluster.local`, `FRONTEND_URL`, Patreon campaign keys, `LOG_LEVEL`). Secrets live in `allchat-secrets` (database password, `JWT_SECRET`, Twitch/YouTube/Kick OAuth, `TOKEN_ENCRYPTION_KEY_V1`, `SUPPORT_BOT_*`) and are deployed from outside this repo — never commit secret values. `redis/secret.yaml` is deliberately not listed in the kustomization so an apply cannot overwrite a live `REDIS_PASSWORD` back to empty.

Env-var key names in the base manifests follow the conventions in the manifests themselves; the live secret uses lowercase keys (`database-password`, `service-jwt-secret`, `facebook-app-id`) while the base manifests frequently spell the reference uppercase (`DATABASE_PASSWORD`, `SERVICE_JWT_SECRET`). Applying base manifests verbatim against production therefore fails with `CreateContainerConfigError`; `docs/runbooks/platform-listener-deployment.md` records the mapping and the live wiring.

Drivers: services allocate resources around a **1,009,000 units/day** YouTube Data API quota (raised from the default 10,000; see `services/youtube-quota-monitor/`) and a **2-second InnerTube polling cadence**. Both are documented in `services/youtube-listener/README.md` and `services/youtube-listener-innertube/README.md`; this document covers the deployment wrapper only.

## Storage

- **PostgreSQL**: CloudNativePG cluster `allchat-cluster` (3 instances, 1 primary + 2 replicas, automatic failover, backups to Hetzner Object Storage with a 30-day PITR window, mirrored to Backblaze B2). Applications connect to `allchat-cluster-rw.allchat.svc.cluster.local:5432` as `allchat_user`; CNPG also creates `-ro` (read-only) and `-r` services. See ADR-0003.
- **Redis**: a single Redis 7 instance with AOF persistence (`appendfsync everysec`, ~1 s RPO), `maxmemory 512mb`, `allkeys-lru`. Data lives on a 5Gi PVC (`postgres` gets 10Gi). In Deployments/k8s these run as plain Deployments with an `if [ "${REDIS_PASSWORD:-}" ]` conditional on `--requirepass` (Redis AUTH) support.
- **Object storage**: Alert-media uploads go to S3-compatible object storage (MinIO-compatible; see `deployments/docker-compose.yml` and `services/media-service/README.md` for the local and prod wiring).

## Autoscaling (base manifests)

HPAs live per service in `deployments/k8s/base/<service>/hpa.yaml`.

| Service | Min | Max |
|---|---|---|
| api-gateway | 2 | 10 |
| message-processor | 2 | 8 |
| twitch-listener / kick-listener | 1 | 5 |
| goodgame-listener, owncast-listener, picarto-listener, altstream-listener | 1 | 5 |
| discord-listener, tiktok-listener | 1 | 3 |
| twitch-eventsub-listener | 2 | 2 (fixed: leader election, keep 2 for failover) |

Listeners are leadership-coordinated via source-manager: extra replicas are safe because only the leader ingests. Deeper tuning lives in [03-SCALING.md](./03-SCALING.md).

## YouTube: two listeners

Production runs both, coordinated by source-manager (ADR-0065):

- `youtube-listener` (Data API): opted-in, owner-verified, premium official API. Claims channels away from InnerTube (our own quota accounting: reserve-confirm-rollback, ADR-0006).
- `youtube-listener-innertube`: the unofficial InnerTube API, serving every unclaimed channel at a fixed 2-second poll. Ships through an **Argo Rollout** with a Prometheus-based analysis (next section) rather than a plain Deployment, because a bad rollout would blindly take over all chat for every non-premium streamer.

## Argo Rollout for youtube-listener-innertube

Manifests: `deployments/k8s/youtube-listener-innertube/{base,production}` (`kubectl apply -k deployments/k8s/youtube-listener-innertube/production/`).

- A Rollout resource (replaces the Deployment), 10 replicas, separate canary and stable Services, a dedicated ServiceMonitor (`deployments/k8s/monitoring/servicemonitor-innertube.yaml`, 1-minute scrape interval to match the analysis cadence).
- Analysis template `innertube-metrics-analysis` (`deployments/k8s/youtube-listener-innertube/base/analysis-template.yaml`), evaluated every minute for 240 consecutive minutes (4 hours) per gate: error rate under 1% (failure above 5%), message-rate deviation from the official listener baseline under 5% (failure above 20%), Redis publish success above 99% (failure below 95%), and zero pod restarts (failure above 2 per minute triggers an immediate rollback).
- Steps: set weight to 10%, pause indefinitely, run the analysis, set weight to 50%, pause, run the analysis again, then promote to 100%. The indefinite pauses make promotion analysis-driven, not time-driven: the rollout advances as soon as each analysis run collects 240 clean minutes, and rolls back when the failure limit of a metric fires. Only abort manually when the analysis is stuck for a technical reason while the metrics are clearly healthy:

```bash
kubectl argo rollouts abort youtube-listener-innertube -n allchat
```

## Hardening and TLS

The base manifests are hardened: services run non-root (`runAsNonRoot`, fixed uid/gid), with `seccompProfile: RuntimeDefault`, `allowPrivilegeEscalation: false`, `capabilities.drop: [ALL]`, and `readOnlyRootFilesystem: true` plus an emptyDir `/tmp` where the image needs it. Pods do not automount the ServiceAccount token (the support-bot, which clones repos over the GitHub API, uses a dedicated ServiceAccount instead). Service images are distroless, per ADR-0092; the one exception is the `allchat-migrations` alpine image, which is a shell script around psql.

Production:
- TLS terminates at the ingress controller. `deployments/k8s/overlays/tls/` ships a cert-manager ClusterIssuer (Let's Encrypt, HTTP-01) and an Ingress for allch.at / www.allch.at with WebSocket settings and secret `allchat-tls`. These manifests are deliberately not part of the base kustomization.
- The base api-gateway Service is `type: LoadBalancer` on plain HTTP; with the TLS overlay, traffic enters through the Ingress instead. Self-hosters terminating TLS at a cloud load balancer can skip the overlay but must keep the HTTP-to-HTTPS redirect.
- `WEBSOCKET_ALLOWED_ORIGINS` (api-gateway env, read in `services/api-gateway/handlers/websocket.go`) must list the exact frontend origins (allch.at, beta.allch.at) in production; `*` is for local dev only.


## Operational notes

- **Keel**: the live cluster sets `keel.sh/policy: force` and `keel.sh/trigger: poll` on each workload in the deploy repository's manifests, so a new image tag is rolled automatically within minutes. This repository's own manifests carry no Keel annotations.
- **Secrets drift**: live `allchat-secrets` key names differ from what several base manifests reference (`database-password` vs `DATABASE_PASSWORD`, `service-jwt-secret` vs `SERVICE_JWT_SECRET`). Applying base manifests against the live cluster yields `CreateContainerConfigError` on the affected pods. The deploy repository's manifests use the live lowercase keys; the mapping and the full per-listener env block are in `docs/runbooks/platform-listener-deployment.md`.
- **redis-auth Secret**: `deployments/k8s/base/redis/secret.yaml` is intentionally not in the base kustomization resources. Shipping it as an apply-able resource meant any `kubectl apply -k` overwrote a live `REDIS_PASSWORD` back to empty, silently disabling Redis AUTH. The Secret must exist before anything sets `--requirepass`.
