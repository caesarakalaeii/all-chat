# Repository Guidelines

## Project Structure & Module Organization
Go services live under `services/<name>` following the standard `cmd/`, `handlers/`, and domain packages layout described in `GETTING_STARTED.md`. Shared utilities (`shared/auth`, `shared/logger`, `shared/redis`, etc.) exist specifically so code is not copied between services. The Next.js frontend is in `frontend/`, static assets in `static/`, and SQL migrations in `migrations/`. Documentation sits in `docs/` (architecture, deployment, testing) while infra definitions are in `deployments/` (Docker Compose and Kubernetes manifests). Keep binaries out of the repo by using the `bin/` folder that the Makefile already points to.

## Build, Test, and Development Commands
Use the Makefile targets unless a service needs custom tooling:
- `make deps` installs Go modules (and TikTok listener npm deps).
- `make build` compiles every Go service into `bin/`.
- `make docker-up` / `make docker-down` bring up the full stack via `deployments/docker-compose.yml`.
- `make test` runs `go test ./...`; `make test-coverage` also emits `coverage.html`.
- `make migrate` applies the baseline schema using `psql`. Run it before local testing so Redis and Postgres-backed services have the expected tables.

## Coding Style & Naming Conventions
Follow idiomatic Go: gofmt + goimports on every change, short receiver names, and package-level `var` only when necessary. Prefer `context.Context` plumbing over global state and keep service-specific configuration in `cmd/main.go`. TypeScript/React components in `frontend/` should stay consistent with the existing Next.js 14 App Router patterns (functional components, hooks, CSS modules or Tailwind). Run `golangci-lint run ./...` and `npm run lint` (when editing the frontend) before opening a PR.

## Testing Guidelines
Unit tests belong next to the code (`*_test.go`). Name them after the behavior being validated (e.g., `TestNormalizer_MapsBadges`). Integration flows are described in `docs/TESTING_COMPREHENSIVE.md`; mirror that checklist when touching Redis Streams, WebSocket bridging, or migrations. Aim to keep coverage above what `make test-coverage` currently reports (~70%) and add regression tests whenever fixing bugs in listeners or message normalization.

## Commit & Pull Request Guidelines
Recent history uses Conventional Commit prefixes (`feat`, `fix`, `chore`, scoped with the service name). Continue that style so changelog automation stays simple. Each PR should describe the affected services, list manual or automated tests executed, and link any roadmap or issue IDs from `Roadmap.md`. If frontend work changes visuals, attach screenshots or a short Loom. Keep PRs focused (ideally <300 lines diff) and wait for at least one review before merging.

## Security & Configuration Tips
Secrets live in `.env` files referenced by `deployments/.env`; never commit real tokens. When testing locally, run Redis and Postgres through Docker Compose so TLS/port assumptions match production. Be mindful that Source Manager performs leader election via Redis locks—avoid running duplicate instances without unique IDs to prevent flapping listeners.
