# Codebase Structure

**Analysis Date:** 2026-02-16

## Directory Layout

```
all-chat/
├── .github/                    # GitHub Actions workflows
├── .planning/                  # GSD planning documents (this directory)
├── bin/                        # Compiled binaries and scripts
├── deployments/                # Kubernetes & Ansible deployment manifests
│   ├── k8s/                    # Kubernetes YAML files (services, deployments, configmaps)
│   └── ansible/                # Ansible playbooks for infrastructure
├── docs/                       # Architecture, ADRs, guides, troubleshooting
│   ├── adr/                    # Architecture Decision Records
│   ├── architecture/           # System design documents
│   ├── development/            # Testing guides, coding standards
│   ├── llm-guides/             # Quick reference guides for common tasks
│   ├── operations/             # Runbooks, observability setup
│   ├── troubleshooting/        # Decision trees, error guides
│   ├── overlay-themes/         # Theme definitions for overlays
│   ├── phase-reports/          # Implementation phase summaries
│   └── tracing/                # Distributed tracing setup
├── frontend/                   # Next.js React frontend application
│   ├── src/
│   │   ├── app/                # Next.js App Router (pages as directories)
│   │   │   ├── admin/          # Admin dashboard pages
│   │   │   ├── auth/           # Authentication pages (login, logout)
│   │   │   ├── chat/           # Chat-related pages
│   │   │   ├── dashboard/      # Streamer dashboard
│   │   │   ├── overlay/        # Single overlay view
│   │   │   ├── overlays/       # Overlay management list
│   │   │   ├── settings/       # User settings
│   │   │   └── page.tsx        # Home page
│   │   ├── components/         # Reusable React components
│   │   ├── hooks/              # Custom React hooks (WebSocket, auth)
│   │   ├── lib/                # Utilities (API client, helpers)
│   │   ├── public/             # Static assets (fonts, images, logos)
│   │   └── styles/             # Global CSS and Tailwind config
│   ├── tests/                  # Playwright E2E tests
│   ├── public/                 # Build output (Docker served files)
│   ├── package.json            # Node.js dependencies
│   ├── next.config.js          # Next.js configuration (API rewrites)
│   ├── tailwind.config.js      # Tailwind CSS configuration
│   └── tsconfig.json           # TypeScript configuration
├── migrations/                 # SQL migration files (numbered, up/down)
│   ├── 001_*.sql               # Create overlays, users, channels tables
│   ├── 002_*.sql               # Add YouTube support
│   └── ...
├── services/                   # Backend microservices (Go)
│   ├── api-gateway/            # WebSocket hub, HTTP reverse proxy (port 8080)
│   │   ├── cmd/main.go         # Service entry point
│   │   ├── handlers/           # HTTP handlers (health, proxy, websocket)
│   │   ├── middleware/         # Local middleware (auth, rate limit)
│   │   ├── models/             # Data structures (message, connection)
│   │   ├── sessions/           # WebSocket session management
│   │   ├── subscription/       # Subscription registry (overlay → connections)
│   │   ├── websocket/          # WebSocket hub and connection logic
│   │   ├── go.mod              # Go module definition
│   │   └── Dockerfile          # Multi-stage Docker build
│   ├── auth-service/           # OAuth, JWT, user management (port 8081)
│   │   ├── cmd/main.go         # Entry point
│   │   ├── handlers/           # HTTP handlers (/login, /callback, /me)
│   │   ├── oauth/              # OAuth client (Twitch, YouTube)
│   │   ├── repository/         # Database queries (users, credentials)
│   │   ├── models/             # Data structures (user, token)
│   │   ├── shared/             # Auth service-specific utils
│   │   ├── go.mod              # Go module definition
│   │   └── Dockerfile
│   ├── overlay-manager/        # Overlay CRUD, source configuration (port 8082)
│   │   ├── cmd/main.go         # Entry point
│   │   ├── handlers/           # HTTP handlers (POST/GET/PUT/DELETE /overlays)
│   │   ├── repository/         # Database queries (overlays, sources)
│   │   ├── models/             # Data structures
│   │   ├── creditroll/         # Credit roll feature
│   │   ├── youtube/            # YouTube-specific overlay features
│   │   ├── clients/            # HTTP clients (upstream services)
│   │   ├── go.mod              # Go module definition
│   │   └── Dockerfile
│   ├── emote-service/          # 7TV, BTTV, FFZ emote caching (port 8083)
│   │   ├── cmd/main.go         # Entry point
│   │   ├── handlers/           # HTTP handlers (GET /emotes/channel/:id)
│   │   ├── cache/              # In-memory cache (with TTL)
│   │   ├── providers/          # 7TV, BTTV, FFZ client implementations
│   │   ├── models/             # Emote data structures
│   │   ├── go.mod              # Go module definition
│   │   └── Dockerfile
│   ├── twitch-listener/        # Twitch IRC chat listener (port 8085)
│   │   ├── cmd/main.go         # Entry point
│   │   ├── irc/                # IRC protocol client (connect, join, listen)
│   │   ├── channels/           # Channel management (join/part)
│   │   ├── publisher/          # Publish to Redis Streams
│   │   ├── handlers/           # HTTP handlers (health, metrics)
│   │   ├── models/             # IRC message parsing
│   │   ├── go.mod              # Go module definition
│   │   └── Dockerfile
│   ├── youtube-listener/       # YouTube Live Chat API poller (port 8086)
│   │   ├── cmd/main.go         # Entry point
│   │   ├── api/                # YouTube API client wrapper
│   │   ├── streams/            # Stream management (watch, fetch messages)
│   │   ├── quota/              # Quota tracking state machine
│   │   ├── oauth/              # OAuth token refresh
│   │   ├── notifications/      # YouTube notification parsing
│   │   ├── publisher/          # Publish to Redis Streams
│   │   ├── handlers/           # HTTP handlers
│   │   ├── models/             # Data structures
│   │   ├── metrics/            # Quota metrics
│   │   ├── go.mod              # Go module definition
│   │   └── Dockerfile
│   ├── kick-listener/          # Kick Pusher WebSocket listener (port ~8087)
│   │   ├── cmd/main.go         # Entry point
│   │   ├── pusher/             # Pusher WebSocket client
│   │   ├── channels/           # Kick channel subscriptions
│   │   ├── publisher/          # Publish to Redis Streams
│   │   ├── handlers/           # HTTP handlers
│   │   ├── models/             # Message parsing
│   │   ├── go.mod              # Go module definition
│   │   └── Dockerfile
│   ├── tiktok-listener/        # TikTok Live Chat listener (port ~8088)
│   │   ├── cmd/main.go         # Entry point
│   │   ├── client/             # TikTok unofficial client
│   │   ├── publisher/          # Publish to Redis Streams
│   │   ├── models/             # Message structures
│   │   ├── go.mod              # Go module definition
│   │   └── Dockerfile
│   ├── message-processor/      # Redis Streams → normalize, enrich, route, pub/sub
│   │   ├── cmd/main.go         # Entry point
│   │   ├── consumer/           # Redis Stream consumer (XREADGROUP, XACK)
│   │   ├── normalizer/         # Platform → unified message schema
│   │   ├── enricher/           # Call emote-service, add emote details
│   │   ├── router/             # Query DB, match overlays to channels
│   │   ├── publisher/          # Publish to Redis Pub/Sub (overlay:*)
│   │   ├── cache/              # Local caches (overlays, channels, emotes)
│   │   ├── classifier/         # Message classification (type, severity)
│   │   ├── filter/             # Message filtering (age, duplicates)
│   │   ├── dedup/              # Deduplication logic
│   │   ├── integration/        # External integrations
│   │   ├── seventv/            # 7TV-specific emote handling
│   │   ├── sessions/           # Session tracking
│   │   ├── models/             # Data structures
│   │   ├── handlers/           # HTTP handlers (health, metrics)
│   │   ├── go.mod              # Go module definition
│   │   └── Dockerfile
│   ├── source-manager/         # Leader election, active source registry
│   │   ├── cmd/main.go         # Entry point
│   │   ├── handlers/           # HTTP handlers (status endpoints)
│   │   ├── election/           # Leader election logic
│   │   ├── models/             # Data structures
│   │   ├── go.mod              # Go module definition
│   │   └── Dockerfile
│   ├── token-refresh-service/  # OAuth token refresh (port ~8089)
│   │   ├── cmd/main.go         # Entry point
│   │   ├── oauth/              # Token refresh implementation
│   │   ├── handlers/           # HTTP handlers
│   │   ├── models/             # Data structures
│   │   ├── go.mod              # Go module definition
│   │   └── Dockerfile
│   ├── twitch-eventsub-listener/ # Alternative Twitch listener (EventSub WebSocket)
│   │   ├── cmd/main.go         # Entry point
│   │   ├── eventsub/           # EventSub client
│   │   └── ... (similar structure)
│   └── discord-bot/            # (Future) Discord chat integration
├── shared/                     # Shared Go libraries imported by services
│   ├── auth/                   # JWT validation, auth helpers
│   ├── database/               # PostgreSQL connection pool, health checks
│   ├── logger/                 # Zap logger factory
│   ├── metrics/                # Prometheus metrics definitions
│   ├── middleware/             # Gin middleware (auth, error handling, tracing)
│   ├── redis/                  # Redis client wrappers, stream helpers
│   ├── ratelimit/              # Rate limiting logic
│   ├── tracing/                # OpenTelemetry configuration
│   ├── sourcemanager/          # Source registry helpers
│   ├── encryption/             # Token encryption utilities
│   ├── crypto/                 # Cryptographic functions
│   ├── signing/                # Message signing utilities
│   ├── cmd/                    # CLI utilities
│   ├── go.mod                  # Go workspace module (go.work)
│   └── go.sum
├── scripts/                    # Utility scripts (setup, backfill, etc.)
├── test/                       # Integration test harnesses
│   ├── python-stream-test/     # Python test client for streaming platforms
│   └── youtube-stream-test/    # YouTube stream testing
├── deployments/k8s/            # Kubernetes manifests
│   ├── services/               # Service definitions
│   ├── deployments/            # Deployment YAML
│   ├── configmaps/             # ConfigMap for non-secret config
│   └── secrets/                # Secret definitions (sealed-secrets)
├── Makefile                    # Development commands (docker-up, test, migrate)
├── go.work                     # Go workspace (mono-repo)
├── go.work.sum                 # Go workspace sum file
├── CLAUDE.md                   # Project conventions and guidelines
├── CONTRIBUTING.md             # Pull request process
├── GETTING_STARTED.md          # Onboarding guide
├── README.md                   # Project overview
└── .env.example                # Example environment variables
```

---

## Directory Purposes

**`.github/`:**
- Purpose: GitHub Actions CI/CD workflows
- Contains: Workflow YAML files for testing, building, deploying
- Key files: `.github/workflows/test.yml`, `build.yml`, `deploy.yml`

**`docs/`:**
- Purpose: Architecture, design decisions, operational guides
- Contains: ADRs, architecture diagrams, troubleshooting guides, LLM quick-refs
- Key files: `docs/architecture/00-OVERVIEW.md`, `docs/adr/README.md`

**`frontend/`:**
- Purpose: React + Next.js web application (overlays, dashboard, admin)
- Contains: TypeScript components, hooks, API client, Playwright tests
- Key files: `frontend/src/app/`, `frontend/next.config.js`

**`services/`:**
- Purpose: Go microservices (listeners, processors, gateways)
- Contains: Each service in own directory following Standard Go Layout
- Key files: `services/*/cmd/main.go` (entry points)

**`shared/`:**
- Purpose: Reusable Go libraries (database, logging, middleware, etc.)
- Contains: Common infrastructure code imported by all services
- Key files: `shared/database/`, `shared/logger/`, `shared/middleware/`

**`migrations/`:**
- Purpose: Database schema evolution
- Contains: Numbered SQL files (001, 002, ...) with UP and DOWN DDL
- Key files: Each file targets specific schema changes (overlays, YouTube, quotas, etc.)

**`deployments/`:**
- Purpose: Infrastructure as code (Kubernetes, Ansible)
- Contains: YAML manifests for services, config, secrets
- Key files: `deployments/k8s/services/`, `deployments/k8s/deployments/`

---

## Key File Locations

**Entry Points:**
- Backend services: `services/*/cmd/main.go` (Go)
  - Example: `services/api-gateway/cmd/main.go` → initializes logger, DB, Redis, starts Gin server
- Frontend: `frontend/src/app/page.tsx` (Next.js) → home page layout
- API routes: `frontend/src/app/api/` (Next.js Server Actions/API Routes)

**Configuration:**
- Environment variables: `.env.example` → copy to `.env` for local dev
- Frontend config: `frontend/next.config.js` → API rewrites, image domains
- Kubernetes config: `deployments/k8s/configmaps/` → service environment variables
- Go modules: Each `services/*/go.mod` → service dependencies

**Core Logic:**
- Message normalization: `services/message-processor/normalizer/`
- Emote enrichment: `services/message-processor/enricher/`
- WebSocket management: `services/api-gateway/websocket/`
- OAuth flows: `services/auth-service/oauth/`
- Database repository: `services/*/repository/` (SQL queries)

**Testing:**
- Unit tests: Colocated with source (`*_test.go`)
- Integration tests: `services/*/tests/` or same directory
- E2E tests: `frontend/tests/` (Playwright)
- Test fixtures: `services/message-processor/cache/` (test data)

---

## Naming Conventions

**Files:**
- `.go` files: lowercase_with_underscores for package-private, PascalCase structs in same file
  - Example: `services/message-processor/normalizer/normalizer.go` (package file)
- `.ts`/`.tsx` files: PascalCase for components, camelCase for utilities
  - Example: `frontend/src/components/OverlayCard.tsx`
- SQL migration files: `NNN_snake_case_description.sql`
  - Example: `migrations/005_multi_platform_auth.sql`
- Docker files: `Dockerfile` (standard)
- Config files: `*.config.js` or `*.config.yml` (standard format)

**Directories:**
- Go packages: lowercase, no underscores (Go convention)
  - Example: `handlers/`, `models/`, `repository/`, `normalizer/`, `enricher/`
- React components: lowercase with subdirectory per feature
  - Example: `frontend/src/components/overlays/OverlayCard.tsx`
- Go services: kebab-case (because directory names become service names)
  - Example: `services/api-gateway/`, `services/message-processor/`

**Go packages:**
- Import paths: `github.com/caesar/all-chat/services/{service}/{package}`
  - Example: `github.com/caesar/all-chat/services/message-processor/normalizer`
- Shared imports: `github.com/caesar/all-chat/shared/{package}`
  - Example: `github.com/caesar/all-chat/shared/logger`

**TypeScript/React:**
- Component props interface: `{ComponentName}Props`
- API response types: `{Endpoint}Response`, `{Endpoint}Request`
- Hooks: `use{Feature}` (React convention)
  - Example: `useWebSocket`, `useAuth`, `useOverlay`

---

## Where to Add New Code

**New Feature (e.g., add Discord chat source):**
- Primary code: `services/discord-listener/` (new service following Go Layout)
  - Entry: `cmd/main.go`
  - Platform client: `discord/client.go`
  - Publisher: `publisher/publisher.go`
  - Handlers: `handlers/health.go`
- Tests: `services/discord-listener/*_test.go` (colocated)
- Add to `docker-compose.yml`: new service with port mapping
- Add to `go.work`: new service workspace entry
- Update frontend: `frontend/src/components/sources/` (add Discord option)

**New Endpoint (e.g., GET /api/overlays/:id/stats):**
- Handler: `services/overlay-manager/handlers/stats.go`
- Repository query: `services/overlay-manager/repository/stats.go`
- Router registration: In `cmd/main.go` Gin setup
- Frontend: `frontend/src/lib/api.ts` → add fetch function
- Component: `frontend/src/components/overlays/StatsCard.tsx` → display data

**New Database Migration (e.g., add column to overlays):**
- File: `migrations/NNN_add_field_to_overlays.sql`
  - UP: `ALTER TABLE overlays ADD COLUMN ...;`
  - DOWN: `ALTER TABLE overlays DROP COLUMN ...;`
- Run: `make migrate-up` (applies pending migrations)
- Query update: `services/overlay-manager/repository/` → update SQL queries

**New Shared Utility (e.g., queue abstraction):**
- Location: `shared/queue/`
- File: `shared/queue/queue.go` → interface definition
- Implementations: `shared/queue/redis.go` (Redis impl), `shared/queue/memory.go` (in-memory)
- Usage: `import "github.com/caesar/all-chat/shared/queue"`
- Update all services' `go.mod` to reference workspace

**New Frontend Page (e.g., /api-docs):**
- App Router: `frontend/src/app/api-docs/page.tsx`
- Components: `frontend/src/components/docs/` → create docs components
- Layout: Inherit from `frontend/src/app/layout.tsx`
- Styles: Colocate CSS or use Tailwind classes

---

## Special Directories

**`bin/`:**
- Purpose: Compiled binaries and local build artifacts
- Generated: Yes (by `go build`, `docker build`)
- Committed: No (in .gitignore)

**`migrations/`:**
- Purpose: Database schema version control
- Generated: No (manually created SQL files)
- Committed: Yes (part of repo history)
- Execution: Applied via custom migration tool in each service's `cmd/`

**`.next/`:**
- Purpose: Next.js build cache
- Generated: Yes (by `next build`)
- Committed: No (in .gitignore)

**`node_modules/`:**
- Purpose: Node.js package dependencies (frontend)
- Generated: Yes (by `npm install`)
- Committed: No (in .gitignore)

**`deployments/k8s/`:**
- Purpose: Kubernetes manifests for production
- Generated: No (manually written YAML)
- Committed: Yes (part of repo for GitOps)
- Namespace: `allchat` (defined in manifests)

**`.planning/codebase/`:**
- Purpose: GSD planning documents (architecture, structure, conventions, testing)
- Generated: Yes (by GSD mappers)
- Committed: Yes (for future planning phases)
- Usage: Referenced by `/gsd:plan-phase` to guide implementation

**`docs/phase-reports/`:**
- Purpose: Implementation phase completion reports
- Generated: Yes (by GSD executors)
- Committed: Yes (historical record)
- Usage: Track what was built and when

