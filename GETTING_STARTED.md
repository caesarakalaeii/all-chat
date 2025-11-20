# Getting Started - LLM Agent Navigation Guide

**Purpose**: This guide helps LLM agents quickly navigate the All-Chat repository and locate relevant files for any task.

**Last Updated**: 2025-11-13 (Phase 4.5)

---

## Quick Reference - Most Important Files

| File | Purpose | When to Read |
|------|---------|--------------|
| [CLAUDE.md](./CLAUDE.md) | Project overview, tech stack, architecture principles | **START HERE** - Every session |
| [README.md](./README.md) | User-facing documentation, setup instructions | Understanding project from user perspective |
| [CHECKPOINT.md](./CHECKPOINT.md) | Current project status, completed phases | Checking what's done and what's next |
| [docs/CSS_CUSTOMIZATION.md](./docs/CSS_CUSTOMIZATION.md) | Complete CSS reference for overlay customization | Working on frontend, overlay themes, CSS |
| [docs/CRITICAL_ARCHITECTURE_ANALYSIS.md](./docs/CRITICAL_ARCHITECTURE_ANALYSIS.md) | Known issues, security gaps, scalability limits | Understanding technical debt and blockers |
| [Makefile](./Makefile) | Build commands, test targets | Running builds, tests, docker commands |
| [deployments/docker-compose.yml](./deployments/docker-compose.yml) | Local development environment | Understanding service configuration |

---

## Repository Structure Overview

```
all-chat/
├── services/                      # 🔹 All microservices (Go)
│   ├── twitch-listener/          # Twitch IRC chat listener
│   ├── youtube-listener/         # YouTube Live Chat API listener
│   ├── message-processor/        # Message normalization & enrichment
│   ├── source-manager/           # Active source registry & leader election
│   └── api-gateway/              # HTTP proxy + WebSocket hub
├── shared/                        # 🔹 Shared Go packages
│   ├── auth/                     # JWT utilities
│   ├── database/                 # PostgreSQL connection
│   ├── redis/                    # Redis client wrapper
│   ├── logger/                   # Zap logger
│   └── middleware/               # HTTP middleware (CORS, Auth)
├── frontend/                      # 🔹 React + Next.js (Phase 5)
├── deployments/                   # 🔹 Infrastructure as Code
│   ├── docker-compose.yml        # Local dev environment
│   ├── k8s/                      # Kubernetes manifests
│   └── ansible/                  # Cluster provisioning (local testing)
├── docs/                          # 🔹 Architectural documentation
│   ├── architecture/             # Detailed design docs
│   ├── CRITICAL_ARCHITECTURE_ANALYSIS.md  # Known issues & gaps
│   ├── TESTING_COMPREHENSIVE.md  # Testing strategy
│   └── PHASE_*.md                # Phase completion reports
├── migrations/                    # 🔹 Database schema migrations
├── .github/workflows/            # 🔹 CI/CD pipelines
└── CLAUDE.md                     # 🔹 Primary instructions for LLM agents
```

---

## Service-Specific Navigation

### Twitch Listener (`services/twitch-listener/`)

**Purpose**: Connects to Twitch IRC, joins channels, publishes raw messages to Redis Streams

**Key Files**:
- `cmd/main.go` - Service entry point, initialization
- `irc/client.go` - Twitch IRC client implementation
- `channels/manager.go` - Dynamic channel join/leave management
- `publisher/redis.go` - Publishes to Redis Stream `chat:raw`
- `go.mod` - Dependencies (gempir/go-twitch-irc)

**Read When**: Working on Twitch integration, IRC issues, channel management

---

### YouTube Listener (`services/youtube-listener/`)

**Purpose**: Polls YouTube Live Chat API, publishes messages to Redis Streams, leader election

**Key Files**:
- `cmd/main.go` - Service entry point with leader election
- `youtube/client.go` - YouTube API client wrapper
- `streams/poller.go` - Live chat polling logic (2-5s interval)
- `channels/manager.go` - Active stream tracking
- `publisher/redis.go` - Publishes to Redis Stream `chat:raw`
- `README.md` - Service-specific documentation

**Read When**: Working on YouTube integration, leader election, API polling

---

### Message Processor (`services/message-processor/`)

**Purpose**: Consumes from Redis Streams, normalizes messages, enriches with emotes, publishes to overlay-specific Pub/Sub

**Key Files**:
- `cmd/main.go` - Consumer group initialization
- `consumer/streams.go` - Redis Streams XREADGROUP consumer
- `normalizer/twitch_normalizer.go` - Twitch-specific message parsing
- `normalizer/youtube_normalizer.go` - YouTube-specific message parsing
- `enricher/emote_enricher.go` - Fetches emotes from Emote Service
- `publisher/pubsub.go` - Publishes to `overlay:{overlay_id}`
- `router/router.go` - Routes messages based on platform

**Read When**: Working on message normalization, emote enrichment, multi-platform support

---

### Source Manager (`services/source-manager/`)

**Purpose**: Maintains registry of active chat sources, provides leader election for YouTube Listener

**Key Files**:
- `cmd/main.go` - HTTP server + registry initialization
- `registry/registry.go` - Polls database for active sources
- `registry/repository.go` - Database queries for overlay_chat_sources
- `election/leader.go` - Redis-based distributed locks (10s TTL)
- `handlers/sources.go` - REST API for active sources
- `handlers/health.go` - Health checks (liveness/readiness)

**Read When**: Working on leader election, active source tracking, service coordination

**Note**: Renamed from "Source Controller" in Phase 4.5 - it is NOT a Kubernetes controller/operator

---

### API Gateway (`services/api-gateway/`)

**Purpose**: HTTP reverse proxy, WebSocket server, Redis Pub/Sub to WebSocket bridge

**Key Files**:
- `cmd/main.go` - HTTP server + WebSocket hub
- `handlers/websocket.go` - WebSocket connection management
- `proxy/reverse_proxy.go` - Routes to backend services
- `pubsub/subscriber.go` - Subscribes to `overlay:{overlay_id}`

**Read When**: Working on WebSocket, HTTP routing, real-time message delivery

---

## Shared Packages (`shared/`)

**Purpose**: Reusable code across all services

| Package | Files | Purpose |
|---------|-------|---------|
| `shared/auth/` | `jwt.go`, `claims.go` | JWT generation, validation, claims parsing |
| `shared/database/` | `postgres.go` | PostgreSQL connection pooling with pgx |
| `shared/redis/` | `client.go` | Redis client initialization |
| `shared/logger/` | `logger.go` | Zap structured logger |
| `shared/middleware/` | `cors.go`, `auth.go` | HTTP middleware for Gin |

**Read When**: Implementing auth, database queries, logging, CORS configuration

---

## Documentation Navigation

### Architecture Documentation (`docs/architecture/`)

| File | Purpose | Read When |
|------|---------|-----------|
| `DATA_FLOW_INTEGRATION.md` | Message flow, Redis Streams + Pub/Sub architecture | Understanding how messages flow through system |
| `DEPLOYMENT_KUBERNETES.md` | K8s manifests, HPA, resource limits | Deploying to Kubernetes |
| `SCALING_PERFORMANCE.md` | Scalability analysis, bottlenecks | Performance optimization, capacity planning |
| `OBSERVABILITY_MONITORING.md` | Health checks, metrics, logging | Adding monitoring, debugging |
| `SECURITY_ARCHITECTURE.md` | Auth, secrets, RBAC, NetworkPolicies | Security hardening, threat mitigation |
| `IMPLEMENTATION_ROADMAP.md` | Phase-by-phase plan | Understanding project timeline |
| `PHASE_4_PLAN.md` | Current phase details | Active development tasks |
| `KUBERNETES_CONTROLLER_ANALYSIS.md` | Why Source Manager is not a K8s controller | Understanding architectural decisions |

### Status Documentation (`docs/`)

| File | Purpose | Read When |
|------|---------|-----------|
| `CSS_CUSTOMIZATION.md` | Complete CSS reference for overlay customization | Working on frontend, overlay display, themes |
| `overlay-themes/README.md` | Theme gallery and creation guide | Creating or modifying overlay themes |
| `CRITICAL_ARCHITECTURE_ANALYSIS.md` | Known issues, security gaps, technical debt | Understanding what needs fixing |
| `TESTING_COMPREHENSIVE.md` | Test strategy, coverage, integration tests | Writing tests |
| `PHASE_4_IMPLEMENTATION_COMPLETE.md` | Phase 4 completion report | Understanding what was delivered |
| `PHASE_5_FRONTEND_COMPLETE.md` | Frontend implementation (future) | Frontend work |
| `PRODUCTION_DEPLOYMENT.md` | Production deployment guide | Going to production |

---

## Deployment & Infrastructure

### Kubernetes Manifests (`deployments/k8s/`)

```
deployments/k8s/
├── namespace.yaml                # Namespace definition
├── configmaps/                   # Environment variables
├── secrets/                      # Secret templates (NOT checked in)
└── base/                         # Service manifests
    ├── postgres/                 # CloudNativePG cluster
    ├── redis/                    # Redis single instance
    ├── twitch-listener/          # Deployment + Service + HPA
    ├── youtube-listener/         # Deployment + Service + HPA
    ├── message-processor/        # Deployment + Service
    ├── source-manager/           # Deployment + Service + HPA
    └── api-gateway/              # Deployment + Service + Ingress
```

**Read When**: Deploying services, configuring autoscaling, debugging pod issues

### Docker Compose (`deployments/docker-compose.yml`)

**Purpose**: Local development environment with all services

**Services Defined**:
- `postgres` - PostgreSQL 16 with pgvector
- `redis` - Redis 7 with AOF persistence
- `twitch-listener` - Twitch IRC listener
- `youtube-listener` - YouTube API poller
- `message-processor` - Message normalizer
- `source-manager` - Source registry
- `api-gateway` - HTTP + WebSocket server

**Read When**: Setting up local development, understanding service dependencies

### Ansible Playbooks (`deployments/ansible/`)

**Purpose**: Provisions local Kubernetes cluster (k3s) for testing

**Key Files**:
- `site.yml` - Main playbook
- `inventory/local.ini` - Localhost inventory
- `verify-deployment.sh` - Checks all pods are running
- `test-integration.sh` - Integration tests
- `TESTING_GUIDE.md` - How to test deployments

**Read When**: Setting up local K8s cluster, testing deployments before production

---

## Database Schema (`migrations/`)

**Migration Files** (Applied in order):

| File | Purpose | Breaking Changes |
|------|---------|------------------|
| `001_initial_schema.sql` | Users, overlays, overlay_configs (single Twitch channel) | N/A |
| `002_add_multi_source_support.sql` | **Multi-source support** - adds `overlay_chat_sources`, `supported_platforms` | **YES** - Removes `twitch_channel` from `overlay_configs` |

**Tables**:
- `users` - Twitch OAuth user data
- `overlays` - Overlay definitions (one-to-many with users)
- `overlay_configs` - Display settings, emote flags
- `overlay_chat_sources` - Chat sources per overlay (Twitch, YouTube, Kick, TikTok)
- `supported_platforms` - Platform registry
- `active_platform_channels` - Currently active channels per platform

**Read When**: Understanding data model, writing database queries, adding new tables

---

## Common Tasks - Where to Look

### Task: Add Support for a New Platform (e.g., Kick)

**Files to Create**:
1. `services/kick-listener/` - New listener service (copy from twitch-listener)
2. `services/message-processor/normalizer/kick_normalizer.go` - Platform-specific parser
3. `deployments/k8s/base/kick-listener/` - K8s manifests

**Files to Modify**:
- `services/message-processor/router/router.go` - Add platform detection
- `migrations/003_add_kick_support.sql` - Add to `supported_platforms`
- `CLAUDE.md` - Update documentation

**Read First**:
- `docs/architecture/DATA_FLOW_INTEGRATION.md` - Message flow
- `services/twitch-listener/README.md` - Listener pattern
- `services/message-processor/normalizer/twitch_normalizer.go` - Example normalizer

---

### Task: Fix a Security Vulnerability

**Files to Check**:
1. `docs/CRITICAL_ARCHITECTURE_ANALYSIS.md` - Known vulnerabilities (§3. Security Analysis)
2. `docs/architecture/SECURITY_ARCHITECTURE.md` - Current security design
3. `shared/auth/jwt.go` - JWT implementation
4. `deployments/k8s/network-policies/` - NetworkPolicies (if they exist)

**Common Issues**:
- OAuth tokens not encrypted (no `shared/crypto/encryption.go` exists)
- No service-to-service auth
- JWT secret hardcoded in docker-compose

---

### Task: Improve Scalability

**Files to Check**:
1. `docs/CRITICAL_ARCHITECTURE_ANALYSIS.md` - Bottleneck analysis (§1. Scalability Analysis)
2. `docs/architecture/SCALING_PERFORMANCE.md` - Scaling strategies
3. `deployments/k8s/base/*/hpa.yaml` - HorizontalPodAutoscaler configs
4. `deployments/docker-compose.yml` - Resource limits (Redis 512MB - too small!)

**Known Bottlenecks**:
- Single Redis instance (no cluster)
- Redis Pub/Sub fan-out to 26+ API Gateway pods
- No backpressure from Message Processor to Redis Streams

---

### Task: Write Tests

**Files to Check**:
1. `docs/TESTING_COMPREHENSIVE.md` - Testing strategy
2. `services/*/handlers/*_test.go` - HTTP handler tests
3. `services/*/election/leader_test.go` - Leader election tests
4. `Makefile` - Test targets (`make test`, `make test-coverage`)

**Test Patterns**:
- Use `miniredis` for Redis mocking
- Use `httptest` for HTTP handlers
- Use `testify/assert` for assertions

---

### Task: Deploy to Kubernetes

**Files to Read** (In Order):
1. `docs/PRODUCTION_DEPLOYMENT.md` - Deployment guide
2. `deployments/k8s/namespace.yaml` - Create namespace first
3. `deployments/k8s/secrets/` - Create secrets from templates
4. `deployments/k8s/configmaps/` - Apply config maps
5. `deployments/k8s/base/postgres/` - Deploy database
6. `deployments/k8s/base/redis/` - Deploy cache
7. `deployments/k8s/base/*/` - Deploy services

**Verification**:
```bash
kubectl get pods -n allchat
kubectl get svc -n allchat
kubectl logs -n allchat -l app=twitch-listener
```

---

### Task: Add Monitoring/Observability

**Files to Check**:
1. `docs/architecture/OBSERVABILITY_MONITORING.md` - Current observability setup
2. `services/*/handlers/health.go` - Health check endpoints
3. `docs/CRITICAL_ARCHITECTURE_ANALYSIS.md` - Missing metrics/tracing (Phase 5)

**What Exists**:
- ✅ Health checks (`/health/live`, `/health/ready`)
- ❌ Prometheus metrics (not implemented)
- ❌ Distributed tracing (not implemented)
- ❌ Grafana dashboards (not implemented)

---

## Development Workflow

### Quick Start - Run Locally

```bash
# 1. Read project overview
cat CLAUDE.md

# 2. Set up environment
cp .env.example .env
# Edit .env with Twitch credentials

# 3. Start infrastructure
make docker-up

# 4. Run migrations
make migrate-up

# 5. Check logs
make docker-logs

# 6. Run tests
make test
```

### Build Individual Service

```bash
# Build specific service
make build-twitch-listener
make build-youtube-listener
make build-message-processor
make build-source-manager
make build-api-gateway

# Or build all
make build
```

### Code Quality Checks

```bash
# Format code
make fmt

# Run linter
make lint

# Run tests with coverage
make test-coverage
```

---

## Git Workflow & Recent Changes

### Current Branch
- `main` - Primary development branch

### Recent Major Changes (Check `git log`)
- **Phase 4.5** (2025-11-13): Renamed "Source Controller" → "Source Manager"
- **Phase 4** (2025-11-11): Completed 5 core services
- **Phase 3** (2025-11-08): Multi-source support, YouTube + Twitch

### Uncommitted Changes (Check `git status`)
```bash
# See what's changed
git status

# See specific changes
git diff CLAUDE.md
```

---

## Environment Variables

**Required for All Services**:
- `DATABASE_HOST`, `DATABASE_PORT`, `DATABASE_USER`, `DATABASE_PASSWORD`, `DATABASE_NAME`
- `REDIS_HOST`, `REDIS_PORT`
- `LOG_LEVEL` (debug, info, warn, error)

**Service-Specific**:
- **Twitch Listener**: `TWITCH_BOT_USERNAME`, `TWITCH_BOT_OAUTH`
- **YouTube Listener**: `YOUTUBE_API_KEY`, `YOUTUBE_CLIENT_ID`, `YOUTUBE_CLIENT_SECRET`
- **Auth Service**: `TWITCH_CLIENT_ID`, `TWITCH_CLIENT_SECRET`, `JWT_SECRET`

**See**: `deployments/docker-compose.yml` for all environment variables

---

## Useful Commands Reference

### Docker Compose
```bash
make docker-up         # Start all services
make docker-down       # Stop all services
make docker-logs       # View logs
make docker-restart    # Restart services
```

### Kubernetes
```bash
kubectl get pods -n allchat
kubectl logs -n allchat -l app=twitch-listener -f
kubectl describe pod -n allchat <pod-name>
kubectl port-forward -n allchat svc/api-gateway 8080:8080
```

### Database
```bash
make migrate-up        # Run all migrations
make migrate-down      # Rollback last migration
psql postgresql://allchat:allchat_dev_password@localhost:5432/allchat
```

### Testing
```bash
make test              # Run all tests
make test-coverage     # With coverage report
go test -v ./services/twitch-listener/...  # Single service
```

---

## Known Issues & Technical Debt

**Read**: `docs/CRITICAL_ARCHITECTURE_ANALYSIS.md` for full analysis

**Critical Issues** (🔴):
1. OAuth tokens stored in plaintext (no encryption implemented)
2. Cannot achieve 10,000 msg/s (Redis bottleneck)
3. No service-to-service authentication
4. Missing NetworkPolicies
5. Auth Service, Overlay Manager, Emote Service missing from codebase

**High Priority** (🟠):
1. Single Redis instance (no cluster)
2. No HPA for API Gateway, Message Processor
3. No rate limiting implemented

**See**: `docs/CRITICAL_ARCHITECTURE_ANALYSIS.md` §5-6 for full issue list and roadmap

---

## Tips for LLM Agents

### Before Starting Any Task

1. ✅ Read `CLAUDE.md` - Project overview and architecture principles
2. ✅ Read `CHECKPOINT.md` - Current status and what's done
3. ✅ Check `docs/CRITICAL_ARCHITECTURE_ANALYSIS.md` - Known issues
4. ✅ Run `git status` - See uncommitted changes
5. ✅ Check relevant service README (if exists)

### When Lost or Confused

1. Check this file (GETTING_STARTED.md) for navigation
2. Search for patterns: `grep -r "pattern" --include="*.go" services/`
3. Check existing tests for examples
4. Read architecture docs in `docs/architecture/`

### When Making Changes

1. Update documentation (CLAUDE.md, README.md, service READMEs)
2. Run tests: `make test`
3. Format code: `make fmt`
4. Check for references: `grep -r "old-name" .`
5. Verify builds: `make build`

### When Adding New Features

1. Check `docs/architecture/IMPLEMENTATION_ROADMAP.md` - Is it planned?
2. Check `docs/CRITICAL_ARCHITECTURE_ANALYSIS.md` - Any blockers?
3. Follow existing patterns in similar services
4. Add tests for new functionality
5. Update documentation

---

## Quick Links Summary

**Start Here**:
- [CLAUDE.md](./CLAUDE.md) - Project instructions
- [README.md](./README.md) - User guide
- [CHECKPOINT.md](./CHECKPOINT.md) - Current status

**Architecture**:
- [docs/architecture/DATA_FLOW_INTEGRATION.md](./docs/architecture/DATA_FLOW_INTEGRATION.md)
- [docs/architecture/DEPLOYMENT_KUBERNETES.md](./docs/architecture/DEPLOYMENT_KUBERNETES.md)
- [docs/CRITICAL_ARCHITECTURE_ANALYSIS.md](./docs/CRITICAL_ARCHITECTURE_ANALYSIS.md)

**Development**:
- [Makefile](./Makefile) - Build commands
- [deployments/docker-compose.yml](./deployments/docker-compose.yml) - Local environment
- [docs/TESTING_COMPREHENSIVE.md](./docs/TESTING_COMPREHENSIVE.md) - Testing guide

**Services** (Read cmd/main.go for entry point):
- [services/twitch-listener/](./services/twitch-listener/)
- [services/youtube-listener/](./services/youtube-listener/)
- [services/message-processor/](./services/message-processor/)
- [services/source-manager/](./services/source-manager/)
- [services/api-gateway/](./services/api-gateway/)

---

**Last Updated**: 2025-11-13 (Phase 4.5)
**Maintained By**: Claude Code Agents
**Questions?**: Check [CLAUDE.md](./CLAUDE.md) or [docs/CRITICAL_ARCHITECTURE_ANALYSIS.md](./docs/CRITICAL_ARCHITECTURE_ANALYSIS.md)
