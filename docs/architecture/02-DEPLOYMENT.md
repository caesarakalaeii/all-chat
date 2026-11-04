# All-Chat: Kubernetes Deployment Architecture

**Version:** 2.0
**Last Updated:** 2026-11-04
**Related Docs**: [Architecture Overview](./00-OVERVIEW.md) (system map), [Scaling & Performance](./03-SCALING.md), [Observability](./04-OBSERVABILITY.md), [Security](./05-SECURITY.md)

---

## Table of Contents

1. [How production is deployed (the one important thing)](#how-production-is-deployed-the-one-important-thing)
2. [Cluster and namespace](#cluster-and-namespace)
3. [Deployed workloads](#deployed-workloads)
4. [Networking, ingress and TLS](#networking-ingress-and-tls)
5. [Configuration and secrets](#configuration-and-secrets)
6. [Resource budget (quota, requests/limits)](#resource-budget-quota-requestslimits)
7. [Probes and rollout behaviour](#probes-and-rollout-behaviour)
8. [Horizontal Pod Autoscaling](#horizontal-pod-autoscaling)
9. [PodDisruptionBudgets and node reboots (kured)](#poddisruptionbudgets-and-node-reboots-kured)
10. [Observability in the cluster](#observability-in-the-cluster)
11. [CronJobs](#cronjobs)
12. [Local development environments](#local-development-environments)
13. [Operational snippets](#operational-snippets)

---

## How production is deployed (the one important thing)

**Production is not in this repo.** The production manifests live in the sibling
GitOps repository `caesar-deployment` (`apps/workloads/all-chat/`), and an
ArgoCD Application (`caesar-deployment/argocd/apps/all-chat.yaml`) syncs them
into the `allchat` namespace of the production K3s cluster continuously:

- `syncPolicy.automated` with `prune: true` and `selfHeal: true`: every merge
  to `caesar-deployment/main` is applied automatically, manual `kubectl`
  changes are reverted, and deleting a resource from the manifest deletes it
  from the cluster.
- ADR numbers are shared between the two repos (see
  `caesar-deployment/docs/adr/`), so "ADR-XXXX" may resolve in either.

Nothing in this repo is ever applied to production. The manifests under
`deployments/` in **this** repo are for **local development only** (see
[Local development environments](#local-development-environments)).

### Image builds and rollouts

- `.github/workflows/build-and-push.yml` in this repo builds and pushes images
  to `ghcr.io/caesarakalaeii/`: every branch push builds that branch's tag
  (`main`, `beta`, plus a `<branch>-<shortsha>` tag), driven by paths-filtered
  per-service jobs. Required PR checks (`test (*)`, `test-node`) also live in
  this workflow.
- Every production Deployment carries
  `keel.sh/policy: force`, `keel.sh/match-tag: "true"` and
  `keel.sh/trigger: poll` annotations. Keel (deployed in
  `caesar-deployment/apps/infrastructure/keel/`, polling schedule configured
  there) watches the pinned digest of the pinned tag and rolls the Deployment
  when the digest changes. `match-tag` matters: without it, Keel ≥ 0.20 would
  follow the newest tag in the repository instead of this Deployment's tag.
- Consequence: **merging to `main` in this repo deploys prod without any PR
  against caesar-deployment** — when the image digest moves, Keel does the
  rollout. `:beta` works the same way for the two beta workloads (below).
- Database schema changes run as an `allchat-migrations` init container in
  front of the deployments that own their migrations (see
  [Deployed workloads](#deployed-workloads)).

### Release flow

The release cycle, gated by this repo's `CONTRIBUTING.md` → Release Flow:

1. Merge to the `beta` branch → `:beta` images are built; Keel rolls
   `frontend-beta` and `api-gateway-beta`, which serve `beta.allch.at`.
2. After a human checks beta: merge to `main` → `:main` images trigger the
   prod rollout. Agents never promote to `main` themselves.

Beta coverage is incomplete — only frontend and api-gateway have a beta
deployment. The beta-only platform listeners (facebook, goodgame, owncast,
picarto) run `:beta` images in prod directly; every other service runs
`:main` only. A backend feature in one of the `:main`-only services has no
beta environment; staging is a human decision.

---

## Cluster and namespace

- **Platform**: self-managed K3s on Hetzner VPS/VMs, provisioned by the
  Ansible playbook in `caesar-deployment/ansible/` (`provision-cluster.yaml`;
  node inventory in `ansible/inventory/hosts.ini`: three master nodes
  `caes`, `caesar-control-2/3` plus three workers `caesar2/3/4`). kured
  patches OS packages with coordinated single-node reboots in a nightly
  maintenance window (see `caesar-deployment/docs/adr/ADR-0021-*`).
- **Ingress layer**: Envoy Gateway (ADR-0083 in `caesar-deployment/docs/adr/`)
  exposing a single shared `Gateway` (`envoy-gateway-system/public`); each
  app attaches its own HTTPRoute. Cloudflare sits in front as DNS/CDN; the
  LoadBalancer service carries the two external IPs used as Cloudflare
  origins.
- **Namespace**: everything runs in namespace `allchat`
  (`namespace.yaml`), with `pod-security.kubernetes.io/warn|audit: restricted`
  labels. The namespace name is `allchat` (one word), not `all-chat` — older
  docs and some older manifests use the hyphenated name; the cluster uses
  the short one.
- **Secrets**: all secrets are SOPS-encrypted with age in
  `caesar-deployment/apps/workloads/all-chat/secrets/` and rendered in-cluster
  via ksops during the ArgoCD sync. Plain `Secret` manifests do not exist in
  Git (never quote or commit secret *values*; this doc only names keys).
- **Sync waves** control the platform dependency order: infrastructure first
  (cert-manager, Envoy Gateway, CNPG, kube-prometheus-stack, loki, tempo),
  then `allchat-monitoring` (wave 4), then the application workloads (wave 6).
- **A `ResourceQuota` and `LimitRange`** bound the namespace; see
  [Resource budget](#resource-budget-quota-requestslimits).

---

## Deployed workloads

Names below are the actual workload names; the full picture (what each
service does, the ports) lives in `00-OVERVIEW.md` → Service Map.

| Workload | Source manifest (`caesar-deployment/apps/workloads/all-chat/`) | Replicas | Image | Notes |
|---|---|---|---|---|
| `frontend` | `frontend-deployment.yaml` | 2 (+ `frontend-hpa` 2–10) | `allchat-frontend:main` | Next.js UI; mounts the `impressum` ConfigMap |
| `frontend-beta` | `frontend-beta-deployment.yaml` | 1 | `allchat-frontend:beta` | serves `beta.allch.at`, shares every backend with prod |
| `api-gateway` | `api-gateway-deployment.yaml` | 3 (+ `api-gateway-hpa` 3–20) | `allchat-api-gateway:main` | single public HTTP/WS entry |
| `api-gateway-beta` | `api-gateway-beta-deployment.yaml` | 1 | `allchat-api-gateway:beta` | single replica; shares all backend services with prod |
| `auth-service` | `auth-service-deployment.yaml` | 2 | `allchat-auth-service:main` | also builds the `retention-sweeper` and `key-rotator` CronJob images |
| `overlay-manager` | `overlay-manager-deployment.yaml` | 2 | `allchat-overlay-manager:main` | |
| `emote-service` | `emote-service-deployment.yaml` | 2 | `allchat-emote-service:main` | |
| `share-service` | `share-service-deployment.yaml` | 2 | `allchat-share-service:main` | |
| `moderation-service` | `moderation-service-deployment.yaml` | 2 | `allchat-moderation-service:main` | |
| `payment-service` | `payment-service-deployment.yaml` | 1 | `allchat-payment-service:main` | |
| `engagement-service` | `engagement-service-deployment.yaml` | 2 | `allchat-engagement-service:main` | |
| `media-service` | `media-service-deployment.yaml` (+ `-service.yaml`) | 2 | `allchat-media-service:main` | presigns MinIO uploads; stateless |
| `message-processor` | `message-processor-deployment.yaml` | 3 | `allchat-message-processor:main` | consumer group `message-processor` |
| `alert-processor` | `alert-processor-deployment.yaml` | 2 | `allchat-alert-processor:main` | consumer group `alert-processors` (ADR-0068) |
| `source-manager` | `source-manager-deployment.yaml` (+ `-rbac.yaml`) | 2 | `allchat-source-manager:main` | Redis-lease leadership (only leader activates sources) |
| `token-refresh-service` | `token-refresh-service-deployment.yaml` | 1 | `allchat-token-refresh-service:main` | single replica on purpose |
| `twitch-listener` | `twitch-listener-deployment.yaml` (+ `-hpa.yaml`) | 2 (+ `twitch-listener` HPA 2–5) | `allchat-twitch-listener:main` | **deprecated path** (ADR-0026): runs `TWITCH_IRC_DEPRECATION_MODE=enforce`, joins no channels |
| `twitch-eventsub-listener` | `twitch-eventsub-listener-deployment.yaml` | 2 | `allchat-twitch-eventsub-listener:main` | `/webhooks/eventsub` HTTPRoute; only Twitch chat path in prod |
| `youtube-listener-official` | `youtube-listener-official-deployment.yaml` | 2 | `allchat-youtube-listener:main` | premium official-API listener (ADR-0065); lease-claimed, one active |
| `youtube-listener-innertube` | `youtube-listener-innertube-deployment.yaml` | 2 | `allchat-youtube-listener-innertube:main` | InnerTube polling, default ingest path |
| `youtube-quota-monitor` | `youtube-quota-monitor-deployment.yaml` | 1 | `allchat-youtube-quota-monitor:main` | |
| `kick-listener` | `kick-listener-deployment.yaml` (+ `-hpa.yaml`) | 1 (+ `kick-listener` HPA 1–5) | `allchat-kick-listener:main` | Centrifugo/Pusher, pooled connections (ADR-0077) |
| `tiktok-listener` | `tiktok-listener-deployment.yaml` | 2 | `allchat-tiktok-listener:main` | Node; publishes to `tiktok-signer` |
| `tiktok-signer` | `tiktok-signer-deployment.yaml` | 1 | `allchat-tiktok-signer:main` | Node + headless Chromium; pinned to node `caesar3`; single replica on purpose |
| `discord-listener` | `discord-listener-deployment.yaml` | 1 | `allchat-discord-listener:main` | one Gateway connection per bot token |
| `owncast-listener` | `owncast-listener-deployment.yaml` | 1 (+ HPA) | `allchat-owncast-listener:beta` | ADR-0058, beta |
| `goodgame-listener` | `goodgame-listener-deployment.yaml` | 1 (+ HPA) | `allchat-goodgame-listener:beta` | beta |
| `picarto-listener` | `picarto-listener-deployment.yaml` | 1 (+ HPA) | `allchat-picarto-listener:beta` | ADR-0059, beta |
| `facebook-listener` | `facebook-listener-deployment.yaml` | 1 | `allchat-facebook-listener:beta` | ADR-0060, beta |
| `instagram-listener` | `instagram-listener-deployment.yaml` | 1 | `allchat-instagram-listener:beta` | ADR-0062, beta; inert until auth-service has Instagram credentials |
| `altstream-listener` | `altstream-listener-deployment.yaml` | 1 (+ HPA) | `allchat-altstream-listener:main` | ADR-0082, early access |
| `discord-bot` | `discord-bot-deployment.yaml` | 1 | `allchat-discord-bot:main` | TypeScript; single replica (no duplicate messages) |
| `support-bot` | `support-bot-deployment.yaml` (+ `-rbac.yaml`) | 1 | `allchat-support-bot:main` | Go; `support-bot` ServiceAccount with two Roles: patcher of only the `allchat-secrets` Secret (key rotation helper) and a namespace-wide read of pods/events/deployments; talks to an OpenAI-compatible LLM endpoint |
| `minio` (StatefulSet `minio`) | `minio-statefulset.yaml` | 1 | `pgsty/silo:RELEASE.2026-10-26*` | no keel; ADR-0068 alert media; no PDB on purpose |
| CronJobs / backups / CNPG | see below | — | — | |

### Services that are NOT deployed

- `tiktok-relay` runs on Cloud Run (US region), not in the cluster; the
  tiktok-listener reaches it over the public internet via
  `TIKTOK_US_RELAY_URL`.
- `twitch-listener` runs, but in `enforce` mode it effectively does nothing
  (ADR-0026); it stays deployed because it is the mechanism the enforce
  phase stands on and can be retired once the IRC path is fully removed.

### Migrations init container

The `allchat-migrations` image (built from `migrations/Dockerfile`, which
copies the SQL from `migrations/` and `scripts/run-migrations.sh`) runs as
an init container on exactly the deployments whose manifest references it —
currently: auth-service, overlay-manager, emote-service, share-service,
moderation-service, payment-service, engagement-service, media-service,
alert-processor, twitch-listener, twitch-eventsub-listener,
token-refresh-service, support-bot. It is
idempotent, so every rollout re-runs it safely; services that don't touch
the schema simply don't run it.

### CronJobs and backup resources

| Resource | Source | Schedule / trigger | Purpose |
|---|---|---|---|
| `retention-sweeper` | `retention-sweeper-cronjob.yaml` | `*/15 * * * *` | deletes rows past the privacy-policy retention periods (ADR-0089); every 15 min because "sent viewer messages deleted after one hour" is promised |
| `key-rotator-weekly` | `key-rotator-cronjob.yaml` (+ `key-rotator-job.yaml`, not applied by default) | `0 3 * * 0` (Sundays 03:00 UTC) | re-encrypts stored tokens onto the versioned AES-GCM key chain (`shared/encryption/versioned.go`); delete or suspend once the legacy `TOKEN_ENCRYPTION_KEY` is retired |
| `backup-offsite-mirror` | `backup-offsite-mirror-cronjob.yaml` | `0 6 * * *` (Europe/Berlin) | mirrors silo media → Hetzner Object Storage, the only backup of the alert media (see below) |
| `minio-bucket-job` | `minio-bucket-job.yaml` | Job (on sync) | creates the `allchat-media` bucket |
| `allchat-cluster-daily` | `backup-objectstore.yaml` (`kind: ScheduledBackup`) | `0 0 23 * * *` (six-field CNPG schedule, 23:00 UTC), `immediate: true` | nightly Barman base backup |
| `allchat-cluster` | `allchat-cluster.yaml` (+ `-secret.yaml`) | CNPG `Cluster`, 3 instances | the Postgres cluster (see below) |


### Redis

Redis migrated from a single-replica Deployment to a Sentinel-managed HA
group fronted by HAProxy in `caesar-deployment/apps/workloads/all-chat/`:

- `redis-deployment.yaml` now holds only the `redis` ClusterIP Service; its
  selector points at `app: redis-haproxy` (the Stage 2 cutover from
  `redis-ha-runbook.md` in the same directory is already applied). Clients
  were never changed: they still use `REDIS_HOST=redis`.
- `redis-ha-node.yaml` — StatefulSet `redis-node`, 3 pods (1 master + 2
  replicas) with per-pod PVCs, anti-affinity across nodes, stable pod-DNS
  replication (`replica-announce-ip`), `min-replicas-to-write 1` to prevent
  split-brain writes during failover.
- `redis-ha-sentinel.yaml` — StatefulSet `redis-sentinel`, 3 sentinels,
  quorum 2/3, failover orchestration, `podManagementPolicy: Parallel`.
- `redis-ha-haproxy.yaml` — Deployment `redis-haproxy`, 2 replicas, routes
  `:6379` to whichever node currently answers `role:master` (1-second
  tcp-check). No controller in the path means no "stale Service" failure
  mode.
- `redis-ha-networkpolicy.yaml` — ingress allow-lists for node / haproxy /
  sentinel.
- The rollout procedure and rollback steps are in
  `REDIS-HA-RUNBOOK.md` in the same directory; the runbook also documents
  that since the cutover, message-processor recreates the `chat:raw`
  consumer group on NOGROUP without needing a restart after Redis blips.
- `maxmemory` is `2gb` per node; `save ""` (RDB snapshots off, AOF only,
  `appendfsync everysec`). No Redis AUTH is configured today.

The Redis data is transient (chat streams, pub/sub, leases, LRU caches) by
design — an empty cluster self-heals, consumers recreate their groups on
NOGROUP, and demand re-syncs on source-manager's next reconcile tick.

### PostgreSQL (CNPG)

The database is a CloudNativePG `Cluster` named `allchat-cluster` (3
instances: 1 primary + 2 replicas), defined in `allchat-cluster.yaml`:

- Image: `ghcr.io/cloudnative-pg/postgresql:17.11-…-standard-trixie`
  (PostgreSQL 17).
- `primaryUpdateMethod: switchover` so an image update hands the primary
  role to an updated replica instead of restarting the primary in place.
  (Adding/removing a WAL-archiver plugin still restarts the pod spec,
  including the primary, and takes minutes — the manifest header documents
  how to window such a change.)
- Storage: `local-path`, 20Gi per instance. `local-path` volumes are
  node-pinned, so PVCs together with
  `podAntiAffinityType: required` (one instance per node) decide where each
  instance can ever live. See the manifest header comment for the failure
  mode when you flip anti-affinity while two instances share a node —
  re-provisioning the stranded instance is required in the same change.
- `max_connections: 300` sized against the per-service connection budget
  (`shared/database`, ADR-0039 in this repo's `docs/adr/`); ~40 service
  instances share this cluster.
- Services: CNPG creates `-rw` (primary) and `-r`; the `ro`/`r` read
  services are explicitly disabled (`disabledDefaultServices`), and there
  is no PgBouncer pooler — services connect to `allchat-cluster-rw`
  directly and cap their own pools (ADR-0039).
- Monitoring: `enablePodMonitor: true` (Prometheus PodMonitor).

This matches ADR-0003 in this repo (`docs/adr/0003-cloudnative-postgres.md`,
revised 2026-11-03). The CNPG operator itself runs in `cnpg-system` and is
installed by ArgoCD from `caesar-deployment/argocd/apps/`.

### Backups

WAL archiving plus nightly base backups go through the Barman Cloud plugin
to Hetzner Object Storage; the `backup-offsite-mirror` CronJob mirrors the
alert media from silo to the same Hetzner account. The details — bucket,
credentials, rollout order, restore procedure and the reason the mirror
CronJob uses `mc mirror --remove` (so deletions propagate and retention
stays honest) — live in the header comments of `backup-objectstore.yaml`
and `backup-offsite-mirror-cronjob.yaml`. The Backblaze B2 copies (Postgres
archive and media) are paused until the B2 account's processor terms are
confirmed (ADR-0003 → Backups), so Hetzner holds the only off-cluster copy
and the privacy policy names Hetzner only.

Recovery point: any moment in the last 30 days, to within the 5-minute
`archive_timeout`. Restore = a new `Cluster` with `bootstrap.recovery` from
the `ObjectStore`; the manifest header carries the exact YAML shape.

### Alert media (MinIO)

The `minio` StatefulSet (1 replica, image `pgsty/silo`, a maintained MinIO
fork; the entrypoint maps the `server` arg onto the `silo` binary) runs the
alert-media bucket (ADR-0068):

- `minio-service.yaml` exposes S3 API on 9000 and the admin console on
  9001 (in-namespace only — `kubectl port-forward`, never routed).
- `minio-route.yaml` attaches an HTTPRoute for `media.allch.at` to the
  shared Gateway with a SecurityPolicy giving CORS for PUT/GET from
  allch.at, and a BackendTrafficPolicy capping the request buffer at
  12 MiB (alert media ≤ 10 MiB). Browsers PUT uploads with presigned
  URLs minted by media-service.
- No PDB on purpose: with one replica a PDB can only deadlock kured drains.
- Single-node and unreplicated by design; the nightly
  `backup-offsite-mirror` CronJob is the copy.

---

## Networking, ingress and TLS

Production does **not** use ingress-nginx. The edge is Envoy Gateway
(`apps/infrastructure/gateway/`): one shared `Gateway`
(`envoy-gateway-system/public`, listener `https`), and every app attaches
its own `HTTPRoute` to it (ADR-0083 in `caesar-deployment/docs/adr/`).

App routes live in two places:

- `caesar-deployment/ingress/allchat-route.yaml` — the frontend/API edge:
  - `HTTPRoute allchat` (host `allch.at`): `/api/` → `api-gateway:8080`,
    `Exact /webhooks/eventsub` → `twitch-eventsub-listener:8090`,
    `/` → `frontend:3000`.
  - `HTTPRoute allchat-ws` (host `allch.at`, `/ws/`) → `api-gateway:8080`,
    with a BackendTrafficPolicy keeping the stream alive
    (`requestTimeout: 0s`, `streamIdleTimeout: 168h`).
  - `HTTPRoute allchat-beta` / `allchat-beta-ws` (host `beta.allch.at`)
    → `api-gateway-beta` / `frontend-beta`.
  - A `SecurityPolicy` gives CORS: `https://allch.at` and
    `https://beta.allch.at`.
  - A `BackendTrafficPolicy` disables the HTTP request timeout on `/api/`
    routes and caps the proxy request buffer at 10 MiB.
- `caesar-deployment/apps/workloads/all-chat/minio-route.yaml` —
  `media.allch.at` → `minio:9000` with its own CORS policy.

TLS: per-zone wildcard `Certificate`s in
`apps/infrastructure/gateway/certificates.yaml` (`allch.at` / `*.allch.at`
among them), issued through cert-manager with Cloudflare DNS-01
(`clusterissuer.yaml`; the HTTP-01 issuer was retired because the beta and
media certs never got issued through it). The Gateway consumes the
`allch-at-tls` Secret. Cloudflare proxies the public edge; the two nodes it
uses as origins are pinned into the Envoy service via `externalIPs` on the
LoadBalancer.

---

## Configuration and secrets

- **ConfigMap `allchat-config`** (`configmap.yaml`) carries the shared env
  (`envFrom`) for all Go services: hosts and URLs, OAuth redirect URLs
  (`https://allch.at/auth/<platform>/callback`), the WebSocket/CORS allow
  list (`https://allch.at`, `https://beta.allch.at`,
  `chrome-extension://*`, `moz-extension://*`), and the OpenTelemetry
  endpoint (`tempo.monitoring.svc.cluster.local:4317`, 10% trace sampling).
  Services override `OTEL_SERVICE_NAME` per pod.
- **ConfigMap `impressum`** is the German legal notice baked into the
  frontend (mounted read-only at `/etc/allchat`).
- **Secrets** are kept out of Git. `apps/workloads/all-chat/secrets/` holds
  SOPS-encrypted files rendered by a ksops generator into the `allchat-secrets`
  Secret plus database/backup credentials. The `jwt_secret`,
  `database-password`, `minio-root-user/-password`, `twitch-*`, `youtube-*`,
  `patreon-*`, `discord-*` and every OAuth client secret live there — never
  quote their values, and never commit new plaintext keys.
- `api-gateway` (and the beta variant) additionally carries a
  `checksum/config` pod-template annotation; bumping it forces a Keel
  rollout after a ConfigMap-only change.

---

## Resource budget (quota, requests/limits)

`resource-quota.yaml` sets the namespace ceiling: 20 CPU / 40Gi requested,
50 CPU / 80Gi limited, 100 pods. A `LimitRange` defaults any container that
declares no resources to 500m CPU / 512Mi limit and 100m / 128Mi request.

Per-pod requests are deliberately modest (128–256Mi, 100–200m CPU for most
Go services) and limits are 4× requests; api-gateway and message-processor
run 256Mi/200m requests with 1Gi/1 CPU limits. The out-of-band exceptions to
know about:

- `tiktok-signer`: requests 1Gi CPU 500m, limits **6Gi / 3 CPUs**. Chromium
  software-renders the live page (llvmpipe) while xvfb and the signature
  browser share the core; it OOMKilled at 2Gi with three concurrent lanes
  and its pod is pinned to node `caesar3` (the documented fallback node).
- `tiktok-listener` (Node) also runs with a larger footprint than a typical
  Go listener.
- The resource quota was raised to fit the signer's surge; a further
  increase needs the quota bumped in the same change.

---

## Probes and rollout behaviour

- HTTP probes follow the standard shape: `livenessProbe` on `/health/live`
  (process up) and `readinessProbe` on `/health/ready` (DB + Redis ping).
  api-gateway uses a single `/health` endpoint as both; frontend probes
  `/api/health`. A handful of services expose only a live probe; check the
  manifest rather than assuming.
- `initialDelaySeconds` is generally 30 for the liveness probe so a
  migrations init container + slow first-run doesn't restart-loop a fresh
  pod.
- `minReadySeconds: 10` on the frontends and the beta gateway to let
  endpoints settle before the next pod is touched.
- Rolling updates use `maxUnavailable: 1, maxSurge: 1` wherever a hard
  topology spread constraint is set (`DoNotSchedule`, `topologyKey:
  kubernetes.io/hostname`, `matchLabelKeys: [pod-template-hash]`), because
  the default 25% surge-only behaviour rounds down to 0 at these replica
  counts and would deadlock against the spread constraint. The manifest
  comment (message-processor, api-gateway, frontend, twitch-listener) is the
  rationale: no two replicas of a service on one node, so one node reboot
  (kured) can never take the service down alone.
- Graceful shutdown is implemented in each service's `cmd/main.go` (25s
  drain window).

---

## Horizontal Pod Autoscaling

HPAs exist for the workloads that need to scale with load; the rest are
fixed at their "special" replica counts (1 for singletons that must never
multiply side effects, 2–3 for the horizontally-scaled core). All
autoscaling/v2, CPU (and some memory) based. The actual targets live in
five HPA manifests:

- `api-gateway` (`api-gateway-deployment.yaml`, bottom): 3–20, CPU 70%.
- `frontend` (`frontend-deployment.yaml`): 2–10, CPU 70%.
- `twitch-listener-hpa.yaml`: 2–5, CPU 70% + memory 80%, with stabilization
  windows (scale-down waits 5 min).
- `kick-listener-hpa.yaml`: 1–5, CPU + memory.
- `platform-listeners-hpa.yaml`: one HPA each for owncast, goodgame,
  picarto and altstream listeners, 1–5.

Everything else scales only vertically or by human decision — see
`03-SCALING.md` for why (single-master Redis, Postgres write path,
listener connection models).

---

## PodDisruptionBudgets and node reboots (kured)

`pdbs.yaml` sets `maxUnavailable: 1` on every horizontally-scaled service
(api-gateway, message-processor, auth-service, overlay-manager,
emote-service, source-manager, moderation-service, share-service,
tiktok-listener, twitch-listener, twitch-eventsub-listener, kick-listener,
youtube-listener-innertube, youtube-listener-official,
engagement-service). `frontend-pdb.yaml` sets `minAvailable: 1` for the
frontend pair. This keeps kured's 02:00–05:00 Europe/Berlin reboot window
(concurrency 1, cluster-wide lock, drainTimeout 5m — see
`caesar-deployment/apps/infrastructure/kured/` and
`caesar-deployment/docs/adr/ADR-0021-*`) draining one node at a time
without ever blocking on an unsatisfiable drain.

Single-replica services deliberately have **no** PDB (tiktok-signer,
discord-listener, discord-bot, support-bot, token-refresh-service, minio,
the CNPG/Redis exceptions documented in their manifests): a PDB over one
replica can never permit a voluntary eviction and just wedges the drain.

---

## Observability in the cluster

- Three `ServiceMonitor`s in `servicemonitor.yaml` (`allchat-listeners`,
  `allchat-source-manager`, `allchat-services`) plus a
  `kubernetes-pods-annotations` scrape job in the kube-prometheus-stack
  values. Services are deliberately split: a workload is either scraped via
  its pod annotations **or** by a ServiceMonitor, never both, to avoid
  double time-series (the manifest header documents
  youtube-listener-innertube being scraped twice and firing a false alert).
- Logs: Grafana Alloy DaemonSet ships pod stdout to Loki; the Envoy proxy
  access log is privacy-shaped (no client IPs, no query strings) to match
  the privacy policy's anonymized-IP promise.
- Traces: OpenTelemetry in `shared/tracing/` sends OTLP to Tempo, 10%
  sampling via `OTEL_TRACES_SAMPLER_ARG: "0.1"`.
- Alerts, dashboards and contact points live in
  `caesar-deployment/apps/platform/allchat-monitoring/` (PrometheusRule
  files at critical/warning/info levels, several Grafana dashboards, a
  Discord contact point) synced by its own ArgoCD app into the `monitoring`
  namespace. Details: `04-OBSERVABILITY.md`.

---

## CronJobs

The five batch resources (`retention-sweeper`, `key-rotator-weekly`,
`backup-offsite-mirror`, `minio-bucket-job`, and the CNPG
`allchat-cluster-daily` ScheduledBackup) are listed in
[Deployed workloads](#deployed-workloads) with their schedules; each
manifest's header carries the operational playbook (suspend order, manual
run command, what happens if it stops running).

---

## Local development environments

Two local options exist in this repo; production is neither of them.

### `deployments/docker-compose.yml` (default, `make docker-up`)

Full stack on Docker: Postgres 16 (migrations mounted into
`/docker-entrypoint-initdb.d`), Redis 7, MinIO, and the Go/Node services.
Internal ports are bound to 127.0.0.1 only; every secret is a required env
var with no insecure defaults (see the file header for the full list).
`make frontend-dev` boots the smaller profile; `make frontend-seed` /
`make frontend-messages` seed data.

### `deployments/k8s/` (k3d, kustomize)

Base kustomization lists every service plus shared network policies and a
resource quota:

```bash
kubectl apply -k deployments/k8s/base           # everything on k3d
kubectl apply -k deployments/k8s/overlays/tls   # optional TLS overlay
```

Differences from production, by design — these are local-dev manifests,
not a scaled-down prod:

- Single-node, no HA, no kured, no Keel, no ArgoCD sync. Postgres is a
  plain Deployment under `base/postgres/deployment.yaml`, not CNPG.
- `base/api-gateway/deployment.yaml` exposes api-gateway as
  `type: LoadBalancer` on plain HTTP. The TLS overlay
  (`overlays/tls/ingress.yaml`, `overlays/tls/cluster-issuer.yaml`) is
  opt-in cert-manager + Let's Encrypt HTTP-01 and mirrors the old
  ingress-nginx shape, not the Envoy Gateway shape prod uses today.
- Images are pinned to `:main` via the `images:` block in
  `base/kustomization.yaml`, not Keel-tracked digests.
- `base/redis/secret.yaml` (empty-value reference manifest for Redis AUTH)
  is deliberately not listed as a kustomize resource so it cannot
  overwrite a real password (local audit finding M5).
- Production's Cloudflare origins, Envoy HTTPRoutes, SOPS secrets and Keel
  annotations do not exist here. `kubectl apply` of these files never
  touches production.

---

## Operational snippets

These target the **production** cluster (your normal kubectl context) and
the `allchat` namespace. The ArgoCD app that owns the workloads is
`all-chat` in the `argocd` namespace. Never `kubectl apply` anything from
this repo's `deployments/` directory there.

```bash
# What's deployed and how it's doing
kubectl -n allchat get deployments,hpa,pdb,cronjobs
kubectl -n allchat get pods -l app=<service-name>
kubectl -n allchat logs deploy/<service-name> --tail=50

# Trigger a manual CronJob run
kubectl create job --from=cronjob/retention-sweeper retention-sweeper-manual-$(date +%s) -n allchat
kubectl create job --from=cronjob/key-rotator-weekly key-rotator-manual-$(date +%s) -n allchat
kubectl create job --from=cronjob/backup-offsite-mirror backup-offsite-mirror-manual-$(date +%s) -n allchat

# Check ArgoCD state (changes land via Git, not manual apply)
kubectl get application all-chat -n argocd -o yaml | yq '.status'

# Force a rollout without waiting for Keel's next poll (e.g. after fixing a
# broken pod that is otherwise healthy)
kubectl -n allchat rollout restart deployment/<service-name>
kubectl -n allchat rollout status deployment/<service-name>

# Production manifests (read-only from this repo's perspective)
git clone git@github.com:caesarakalaeii/caesar-deployment.git
cd caesar-deployment/apps/workloads/all-chat
```

---

## Summary

1. **Production** is ArgoCD GitOps out of the `caesar-deployment` repo; this
   repo only builds images and holds local-dev manifests. Never `kubectl
   apply` anything from here at prod.
2. **Keel** turns image pushes into rollouts; merging to `main` or `beta`
   is the deployment act.
3. **Redis is Sentinel + HAProxy** (not a single replica), **Postgres is
   CNPG** with Barman backups to Hetzner (Backblaze B2 copies paused),
   and alert media is single-node MinIO behind `media.allch.at`.
4. **Edge** is Envoy Gateway with per-app HTTPRoutes; TLS is
   cert-manager Cloudflare DNS-01 wildcards; Cloudflare fronts the edge.
5. Everything else above is named by file so the manifests themselves can
   be checked — this doc deliberately does not duplicate values that drift.

---

**Doc Maintainers**: All-Chat backend + caesar-deployment repo owners.
**Last Review**: 2026-11-04
