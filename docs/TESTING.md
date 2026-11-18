# Testing

How tests are organized and run in All-Chat.

## Backend (Go)

Each service module carries its own tests (`*_test.go` next to the code).

```bash
make test                 # go test -v ./... for every service module
make test-coverage        # adds -coverprofile=coverage.out
cd services/auth-service && go test -v ./...   # one service
```

Long-running suites (concurrency, leadership rebalancing) run nightly in `.github/workflows/nightly-tests.yml` with `go test -v -timeout 15m ./...` per service, without `-short`.

## Frontend (frontend/)

```bash
npm run test              # vitest
npm run test:e2e          # playwright test
npm run test:visual       # playwright visual-regression, chromium
npm run test:visual:update  # update snapshots
npm run test:e2e:ui / test:e2e:debug / test:e2e:report
```

CI quality gates in `.github/workflows/`: `frontend-quality.yml` (ESLint with zero warnings, prettier, `tsc --noEmit`, build), `frontend-a11y.yml`, `frontend-i18n.yml`, `frontend-visual.yml`.

UI changes also go through the screenshot baseline ritual, see [docs/frontend/AGENT_UI_VERIFICATION.md](frontend/AGENT_UI_VERIFICATION.md).

## Rust/Chaos and load

`scripts/chaos-test-phase5.sh` interacts with services for chaos scenarios; write-up in [docs/testing/chaos-testing-phase5.md](testing/chaos-testing-phase5.md). Load tests live in `tests/load/` (k6-style scripts, e.g. `batch-deletion-processor.js`).

## Test data helpers

```bash
make docker-up            # postgres, redis, all services
make frontend-seed        # scripts/seed-test-data.sh: create test overlay and sources
make frontend-messages    # scripts/generate-test-messages.sh: mock chat messages
make test-stream          # scripts/start-test-stream.sh: public test-stream generator
```

Manual smoke: `curl http://localhost:8080/health/ready`, WebSocket checks with `scripts/test-websocket.js`.

## Troubleshooting

- Service fails to connect in tests: PostgreSQL/Redis not running; `make docker-up` first.
- Frontend tests fail on missing backend: use `make frontend-dev` bootstraps before running e2e.
- Flaky long tests locally: they run without `-short` only in nightly; locally scope with `go test ./<package> -run <name>`.
