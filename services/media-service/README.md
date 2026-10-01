# media-service

All-Chat's **alert-media registry** (ADR-0064, issue #949): the first service to hold an S3/MinIO
client. Users attach media to overlay event alerts — sounds, images, GIFs, short video clips — and
this service hands out presigned PUT URLs so the bytes go **directly** to MinIO, never through a
service pod.

The service is deliberately a dumb file registry. It does not transcode, re-host, or fetch external
URLs (Klippy GIF search re-hosting is a separate follow-up), and it never serves media bytes: the
bucket is public-read and anonymous GETs go straight to MinIO (`media.allch.at`), so uploads are the
only path through this service.

## Endpoints

All require a user JWT; `user_id` is resolved from the token, never from the request. Mounted under
`/api/v1/media`:

| Method | Path | Description |
|---|---|---|
| `POST` | `/presign` | `{ filename, content_type, size }` → **201** `{ object_key, upload_url, public_url }` |
| `GET`  | `/`      | the caller's registered media, from the DB registry (never a bucket listing) |
| `DELETE` | `/:object_key` | owner-checked delete of the registry row and the MinIO object |

`object_key` is `{user_id}/{uuid}/{filename}`: the random uuid segment makes public-read URLs
unguessable (ADR-0064), and the user_id prefix scopes the delete's owner check. The filename is
reduced to its bare base name before it reaches the key, so no path component can travel through
it.

`upload_url` is a presigned PUT valid for `MEDIA_PRESIGN_EXPIRY` (default 5 minutes); the client
performs the upload with it directly against MinIO. `public_url` is `MEDIA_PUBLIC_URL` + `/` +
`object_key` — the URL an alert (or the frontend) plays.

### Validation and quota

- `content_type` must be one of: `image/png`, `image/jpeg`, `image/gif`, `image/webp`,
  `audio/mpeg`, `audio/ogg`, `audio/wav`, `audio/webm`, `video/webm` — anything else is a **422**.
- `size` must be positive and ≤ 10 MiB — over the limit is a **413**.
- Per-user quota: at most `MEDIA_MAX_OBJECTS_PER_USER` (default 100) registered objects; over is a
  **422**. The count is taken from the registry, so orphans count until deleted.

### Registry rows and orphan tolerance

The row is written at **presign** time, before the client uploads: single-node MinIO has no event
notification to confirm completion, so the service never learns whether the PUT happened. A row
whose object never arrived is a harmless orphan — `GET /media` lists the registry, not the bucket,
and deleting a row only logs a failed MinIO removal rather than failing the request.

### Status codes worth knowing

- **503** on every media route when `MINIO_ENDPOINT` is unset: the service still starts (env-gating,
  like the youtube-listener-innertube optional subscribers), health stays green, and routes come
  back when MinIO is deployed (issue #948, bucket `allchat-media`).
- **502** when MinIO itself fails to presign — no registry row is left behind.
- **404** on delete of a key that is not the caller's (indistinguishable from one that never
  existed — not a 403, which would be an existence oracle for other users' keys).

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8094` | HTTP listen port |
| `LOG_LEVEL` | `info` | zap log level |
| `GIN_MODE` | `debug` | `release` in production |
| `DATABASE_HOST` / `_PORT` / `_NAME` / `_USER` / `_PASSWORD` | `localhost` / `5432` / `allchat` / `allchat` / — | PostgreSQL (media_objects, migration 099) |
| `REDIS_HOST` / `_PORT` / `_PASSWORD` | `localhost` / `6379` / — | JWT logout-blacklist check; optional, fail-open without it |
| `JWT_SECRET` / `JWT_SECRET_V1` | — | JWT key chain (same secrets auth-service signs with) |
| `MINIO_ENDPOINT` | *(unset)* | `host:port`. Unset ⇒ media routes serve 503 |
| `MINIO_MEDIA_USER` / `MINIO_MEDIA_PASSWORD` | — | dedicated MinIO media credentials (preferred over root) |
| `MINIO_USE_SSL` | `true` | TLS to MinIO; `false` for local compose |
| `MINIO_BUCKET` | `allchat-media` | bucket provisioned with the MinIO instance (issue #948) |
| `MEDIA_PUBLIC_URL` | `https://media.allch.at` | base for `public_url`; include the bucket path if MinIO serves it under one |
| `MEDIA_PRESIGN_EXPIRY` | `5m` | presigned PUT lifetime (Go duration) |
| `MEDIA_MAX_OBJECTS_PER_USER` | `100` | per-user quota on registered media |

## Kubernetes

No manifests live in this repo — the cluster is ArgoCD GitOps from
[caesar-deployment](https://github.com/caesarakalaeii/caesar-deployment): see
`apps/workloads/all-chat/media-service-deployment.yaml` and
`apps/workloads/all-chat/media-service-service.yaml` there. Local dev uses
`deployments/docker-compose.yml` in this repo (a MinIO compose service comes with the infra
follow-up, issue #948; until then `MINIO_ENDPOINT` points at `minio:9000` and media routes 503 or
presign-error without it).

## Tests

Handler-level tests run against in-memory mocks of the registry and object-store seams — no MinIO
or Postgres needed:

```sh
GOTOOLCHAIN=go1.25.7 go test ./...
```
