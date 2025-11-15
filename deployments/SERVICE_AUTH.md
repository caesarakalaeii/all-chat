# Service-to-service authentication

The Source Manager HTTP API now enforces authentication for every endpoint except the health checks. Internal callers must send a signed JWT in the `Authorization: Bearer <token>` header.

## 1. Configure the shared signing secret

Set `SERVICE_JWT_SECRET` for the Source Manager deployment. The Docker Compose file defaults this to `dev-service-secret`; override it in production via an `.env` file or your orchestration secrets store.

## 2. Issue per-service credentials

Use the helper located at `shared/cmd/service-token` to mint tokens for each service identity:

```bash
go run ./shared/cmd/service-token --service youtube-listener --secret "$SERVICE_JWT_SECRET" --expiry 24h
```

The output token can be stored as a secret (for example `YOUTUBE_LISTENER_SOURCE_MANAGER_TOKEN`) and injected into the calling service's environment. Each service should keep its token private and rotate it as needed.

## 3. Distribute and use the tokens

When calling the Source Manager API, include the token in the HTTP `Authorization` header:

```
Authorization: Bearer <service-token>
```

Health endpoints (`/health/live`, `/health/ready`, `/status`) remain unauthenticated so infrastructure probes can continue to check liveness and readiness without credentials.
