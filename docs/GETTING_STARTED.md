# Getting Started

Before anything else, read [CLAUDE.md](../CLAUDE.md): project overview, architecture, tech stack, documentation conventions.

## Local development

```bash
make docker-up          # Postgres, Redis, all services (docker compose)
make migrate            # Apply database migrations
make test               # Run Go tests
```

Services: API Gateway on http://localhost:8080, frontend on http://localhost:3000.

Frontend-only work (minimal backend):

```bash
make frontend-dev       # Postgres, Redis, gateway, overlay-manager, message-processor
make frontend-seed      # Create test overlay and chat sources
make frontend-messages  # Generate mock chat messages
cd frontend && npm run dev
```

Dependencies for all service modules: `make deps`. See `make help` (and the `Makefile`) for the full target list, and [FRONTEND_QUICK_START.md](./frontend/FRONTEND_QUICK_START.md) for frontend setup details.

## Where to read next

| Task | Doc |
|---|---|
| Task-oriented quick refs (add a platform, endpoint, migration; debug quota; Redis ops; security audit) | [docs/llm-guides/](./llm-guides/) |
| System architecture and data flow | [docs/architecture/](./architecture/) |
| Why a decision was made | [docs/adr/](./adr/) |
| Troubleshooting | [docs/troubleshooting/decision-tree.md](./troubleshooting/decision-tree.md) |
| Per-service details | [services/*/README.md](../services/) |
| PR process | [CONTRIBUTING.md](../CONTRIBUTING.md) |
