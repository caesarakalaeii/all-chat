# Troubleshooting: build and startup errors

## Go compilation

- `undefined: <package>`: missing or unlabeled import. Fix the import or run `go mod tidy && go mod download`.
- Type mismatches (`*pgxpool.Pool` vs `*sql.DB`, pgx vs database/sql): wrong import for the layer; the repo uses pgx/v5.
- `module ... not found` for `shared`: each service module needs the replace directives in its `go.mod`, e.g. service `api-gateway` has `replace github.com/caesar/all-chat/shared => ../../shared` and `replace github.com/caesar/all-chat/shared/ratelimit => ../../shared/ratelimit`.
## Docker build

- `COPY failed: file not found in build context`: Dockerfile COPY paths are relative to the build context (repository root); service Dockerfiles expect `services/<name>/` and `shared/` inside the context.
- `Sending build context to ...` huge: missing `.dockerignore` (exclude `node_modules/`, `.git/`, `vendor/`).

## Startup

- `bind: address already in use`: another process holds the port. Find it with `lsof -i :<port>` or set a different `PORT` (defaults per service: auth-service 8081, api-gateway 8080, youtube-listener 8086, etc.).
- `panic: <VAR> environment variable not set`: required env missing. Locally check `.env`; in Kubernetes check the pod's env sources (`configMapKeyRef` from `allchat-config`, `secretKeyRef` from `allchat-secrets`).
- `connection refused` on startup to DB/Redis: dependency not up yet. See [connection-errors.md](./connection-errors.md).
