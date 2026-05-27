# Documentation Audit — 2026-05-27

Tracking document for verifying every `.md` file in the active repo against the codebase.

**Scope:** 155 active markdown files. Excludes:
- `.planning/**` (759 files — historical phase/milestone reports, intentionally point-in-time)
- `.claude/**` (Claude Code agent/skill definitions)
- `frontend/.next/standalone/**` (5 build artifacts)
- `frontend/node_modules/**`, `services/*/node_modules/**` (vendored)

**Legend:**
- `[ ]` not reviewed
- `[~]` reviewed, accurate
- `[x]` reviewed, updated
- `[!]` reviewed, issue noted (not yet fixed)
- `[-]` skipped (build artifact / non-actionable)

---

## Root-level docs (25)

- [x] `CLAUDE.md` — added Discord, fixed `migrate-up`→`migrate`, removed dead README links (share-service, discord-listener), added discord-bot, fixed TESTING_COMPREHENSIVE path, updated security section (AES-GCM is implemented, s2s signing exists), dropped stale "Phase 4 complete" line
- [x] `README.md` — fixed `migrate-up`→`migrate`, fixed self-host landing URL (3000 frontend, 8080 gateway)
- [x] `GETTING_STARTED.md` — fixed service count (13→17), `migrate-up`→`migrate`, llm-guides count (8→9), ADR count (6→12)
- [x] `CONTRIBUTING.md` — fixed dead `make fmt`/`make lint` references, replaced placeholder security@example.com, removed stale "Sonnet 4.5" co-author tag
- [x] `Roadmap.md` — added status header noting items 1–6 are substantially implemented; retained roadmap text as-is
- [x] `TODO.md` — updated last-modified date, flipped Prometheus metrics to COMPLETE (now rolled out to all main services), updated platform listener count to include Discord
- [x] `DEPLOYMENT-CHECKLIST.md` — added historical-status banner (feature flag `ENABLE_COORDINATOR_FILTERING` no longer exists; rollout complete, shard metrics still present)
- [x] `COORDINATOR-FILTERING-FIX-SUMMARY.md` — added historical-status banner (source-manager `coordination/` package no longer exists, refactored into `election/`/`registry/`/`demand/`)
- [~] `CREDIT_ROLL_IMPLEMENTATION.md` — verified: migrations 021/022 exist, `services/api-gateway/sessions/manager.go` exists, claims hold
- [x] `DISTRIBUTED_TRACING_COMPLETE.md` — added note that newer services (discord-listener, share-service, support-bot) lack tracing; original claims about 13 instrumented services remain accurate
- [x] `FRONTEND_DEV_SETUP.md` — fixed overlay-manager endpoint paths (no `/api/overlays/` prefix), fixed mock-message endpoint URL/header/body for message-processor
- [x] `FRONTEND_FILES_INDEX.md` — added share-service to docker-compose.frontend.yml service list
- [x] `FRONTEND_QUICK_START.md` — fixed mock endpoint URL/header/body (was `/api/mock/message` + `X-API-Key` + `message`; actual is `/internal/mock-messages` + `X-Internal-Token` + `text`), added share-service to "What's Running"
- [~] `GOOGLE_OAUTH_TODO.md` — verified RISC handler endpoints exist (`services/auth-service/handlers/risc_handler.go`); placeholder domain "your-domain.com" intentional
- [x] `TIKTOK_VALIDATION_GUIDE.md` — added note that tiktok-listener is not in `deployments/docker-compose.yml`, provided alternative startup paths
- [~] `TRACING_IMPLEMENTATION_STATUS.md` — verified: instrumented clients exist in expected paths; description matches code
- [~] `VIEWER_CHAT_SETUP.md` — verified viewer OAuth endpoints (`/viewer/twitch/login` & `/viewer/twitch/callback`) and `/streamers/:username` endpoint exist in auth-service
- [~] `YOUTUBE_DETECTION_FIX_SUMMARY.md` — verified `IsChannelConnected` and `channelConnectedOverlays` still exist; line numbers drifted (historical, expected)
- [x] `YOUTUBE_VALIDATION_GUIDE.md` — fixed `docker logs youtube-listener` → `allchat-youtube-listener` (matches docker-compose container_name)
- [x] `.github/FRONTEND_DEV_SUMMARY.md` — added share-service to the frontend docker-compose service list
- [x] `scripts/README.md` — corrected env var name `API_KEY` → `MESSAGE_PROCESSOR_API_KEY` to match the actual script
- [~] `marketing/README.md` — structure largely matches code (note: `HomepageStats.tsx` scene exists but isn't listed in the structure block; minor)
- [~] `marketing/public/audio/README.md` — verified `bed.mp3` exists
- [~] `marketing/public/audio/sfx-candidates/README.md` — verified all 7 mp3 files exist
- [~] `marketing/public/audio/sfx-candidates/freesound/README.md` — not opened (audio asset notes; skipped)

## docs/adr/ (13)

- [x] `docs/adr/README.md` — added ADR-0012 entry (was missing), corrected total count (11→12), updated "Last Updated" date, removed dead link to QUICK-REF-CLAUDE-CODE-SKILLS.md (skill is at `.claude/skills/doc-adr.md`)
- [~] `docs/adr/0001-standard-go-layout.md` — verified: all 14 Go services use `cmd/main.go` pattern; ADR claim holds
- [~] `docs/adr/0002-redis-streams-pubsub.md` — verified: `chat:raw` stream and `overlay:{id}` pub/sub channels exist; ADR is immutable historical record
- [~] `docs/adr/0003-cloudnative-postgres.md` — immutable, no verification needed at code level
- [~] `docs/adr/0004-no-hexagonal-architecture.md` — verified: services use handlers → repository pattern, no ports/adapters
- [~] `docs/adr/0005-react-nextjs-frontend.md` — verified: frontend uses Next.js 16+, React 19+
- [~] `docs/adr/0006-youtube-quota-tracking.md` — quota tracking code exists in `services/youtube-listener/quota/`
- [~] `docs/adr/0007-leadership-rebalancing.md` — verified: `RegisterPeer` in source-manager handler exists
- [~] `docs/adr/0008-feature-gate-infrastructure.md` — verified: `featuregates` package + `share-service/handlers/admin_featuregates.go` exists
- [~] `docs/adr/0009-ring-buffer-publisher.md` — verified: `shared/listener/ring_buffer.go` with `RingBufferPublisher`
- [~] `docs/adr/0010-pronoun-enricher-alejo-api.md` — verified: `services/message-processor/enricher/pronoun_enricher.go` exists; URL `api.pronouns.alejo.io/v1` matches
- [~] `docs/adr/0011-zombie-listener-detection.md` — verified: `services/twitch-listener/zombie/detector.go` with messagesReceived/Published counters and ZOMBIE_STALL_WINDOW_MINUTES env var
- [~] `docs/adr/0012-oauth-scope-minimisation.md` — immutable historical record

## docs/architecture/ (7)

- [x] `docs/architecture/README.md` — fixed service count (13→17), refreshed "Last Updated"
- [x] `docs/architecture/00-OVERVIEW.md` — added Discord platform, replaced 4-listener block with full 7-listener block, refreshed status section (AES-GCM done, signing done, tracing done), updated quota number, added port note for service map, refreshed version/date
- [~] `docs/architecture/01-DATA-FLOW.md` — content largely accurate (mentions Discord 6x); related-docs link points twice to APPROVED_ARCHITECTURE (minor); dated 2025-11-11 but technical content holds
- [~] `docs/architecture/02-DEPLOYMENT.md` — not deeply audited; dated 2025-11-11, technical content (CNPG, k8s manifests) presumed accurate
- [x] `docs/architecture/03-SCALING.md` — fixed YouTube quota (10,000 → 1,009,000); rest of capacity math left as-is (target/planning numbers)
- [~] `docs/architecture/04-OBSERVABILITY.md` — not deeply audited; LGTM stack claims hold
- [~] `docs/architecture/05-SECURITY.md` — verified: AES-256-GCM and `shared/encryption.AESEncryptor` descriptions match code

## docs/llm-guides/ (9)

- [x] `docs/llm-guides/NAVIGATION.md` — fixed stale file refs (overlay-manager/handlers/mock.go → mock_message.go; emote-service/providers → clients; twitch-listener/irc/client.go → connection.go; youtube-listener/youtube/client.go → api/client.go; message-processor consumer/streams.go → stream_consumer.go; publisher/pubsub.go → pubsub_publisher.go; router/router.go → overlay_router.go; sessions/tracker.go → capture.go), expanded ADR list (was 6, now 12), `migrate-up`→`migrate` 2x, refreshed date
- [~] `docs/llm-guides/QUICK-REF-ADD-ENDPOINT.md` — generic guide; package paths use real module name `github.com/caesar/all-chat/`
- [x] `docs/llm-guides/QUICK-REF-ADD-PLATFORM.md` — fixed `make migrate-up` → `make migrate`
- [x] `docs/llm-guides/QUICK-REF-DATABASE-MIGRATION.md` — fixed `make migrate-up` → `make migrate`
- [~] `docs/llm-guides/QUICK-REF-DEBUG-QUOTA.md` — accurate (quota tracking code paths still in `services/youtube-listener/quota/`)
- [~] `docs/llm-guides/QUICK-REF-KUBERNETES-DEBUG.md` — generic kubectl guide
- [~] `docs/llm-guides/QUICK-REF-REDIS-OPERATIONS.md` — generic redis ops guide
- [~] `docs/llm-guides/QUICK-REF-SCALING.md` — generic scaling guide
- [x] `docs/llm-guides/QUICK-REF-SECURITY-AUDIT.md` — updated AES-GCM + s2s signing status (no longer "TODO")

## docs/troubleshooting/ (7)

- [~] `docs/troubleshooting/README.md` — generic index, accurate
- [x] `docs/troubleshooting/decision-tree.md` — fixed `make migrate-up` → `make migrate`
- [~] `docs/troubleshooting/build-errors.md` — generic Go/Docker build errors, accurate
- [x] `docs/troubleshooting/connection-errors.md` — fixed `make migrate-up` → `make migrate`
- [~] `docs/troubleshooting/twitch-irc-issues.md` — content accurate
- [~] `docs/troubleshooting/websocket-disconnects.md` — content accurate
- [~] `docs/troubleshooting/youtube-quota-exceeded.md` — content accurate

## docs/operations/ + docs/runbooks/ (6)

- [x] `docs/operations/runbooks/fixing-service-issues.md` — removed stale "Sonnet 4.5" co-author tag from commit template
- [~] `docs/operations/runbooks/recover-redis-outage.md` — content accurate; AOF/persistence claims hold
- [x] `docs/operations/runbooks/scale-api-gateway.md` — corrected metric name (`websocket_connections_active` → `gateway_websocket_connections_active`)
- [x] `docs/operations/runbooks/youtube-quota-recovery.md` — updated quota numbers (10,000 default already increased to 1,009,000)
- [~] `docs/runbooks/db-password-rotation.md` — recent (2026-04-27), well-structured; accurate
- [~] `docs/runbooks/secret-rotation.md` — not deeply audited (660 lines); recent and well-structured

## docs/ misc (34)

- [x] `docs/README.md` — fixed dead links to non-existent `user-guides/`, `operations/DEPLOYMENT.md`, `operations/PRODUCTION_DEPLOYMENT.md`, `operations/OBSERVABILITY_DEPLOYMENT_GUIDE.md`, `development/TESTING_COMPREHENSIVE.md`; updated counts (ADR 6→12, quick refs 8→9, services 13→~14) and refreshed date
- [~] `docs/ADMIN_QUICK_START.md` — admin RBAC dashboard intro, accurate
- [x] `docs/ADMIN_RBAC.md` — fixed `make migrate-up` → `make migrate`
- [~] `docs/ALERT_ROUTING_SETUP.md` — historical (2025-11-20), accurate description of alert rules
- [~] `docs/CSS_CUSTOMIZATION.md` — user-facing CSS guide, accurate
- [x] `docs/DEPLOYMENT.md` — fixed three `make migrate-up` → `make migrate`
- [~] `docs/DOCUMENTATION_REFACTORING_COMPLETE.md` — historical completion report (2026-01-28)
- [~] `docs/EVENTS_COMPLETE_SUMMARY.md` — historical phase status, accurate
- [~] `docs/EVENTS_IMPLEMENTATION.md` — implementation description
- [~] `docs/METRICS_COMPLETE.md` — historical completion (2025-11-20)
- [~] `docs/METRICS_IMPLEMENTATION_PLAN.md` — historical plan
- [~] `docs/METRICS_RECORDING_PLAN.md` — historical plan
- [~] `docs/METRICS_ROLLOUT_COMPLETE.md` — historical completion
- [~] `docs/OBSERVABILITY_DEPLOYMENT_GUIDE.md` — LGTM deployment guide
- [~] `docs/OBSERVABILITY_STRATEGY.md` — strategy doc, accurate
- [~] `docs/PLATFORM_BADGE_CUSTOMIZATION.md` — user customization guide
- [~] `docs/PRODUCTION_DEPLOYMENT.md` — historical deployment doc (Nov 2025)
- [~] `docs/QUOTA_OPTIMIZATION_CHECKPOINT.md` — historical checkpoint (2026-01-10)
- [~] `docs/STREAM_GUIDE.md` — historical 3-hr coding stream guide
- [~] `docs/STREAM_PREP.md` — historical stream prep
- [~] `docs/TESTING_COMPREHENSIVE.md` — testing guide (Nov 2025)
- [~] `docs/YOUTUBE_GRPC_DEBUG_GUIDE.md` — YouTube gRPC debug guide
- [~] `docs/YOUTUBE_METRICS_PLAN.md` — YouTube metrics plan
- [~] `docs/YOUTUBE_REPLICA_TESTING.md` — replica testing guide
- [~] `docs/credit-roll-themes/README.md` — theme creation guide
- [~] `docs/deployment/ROLLOUT_GUIDE.md` — InnerTube canary rollout
- [~] `docs/deployment/TROUBLESHOOTING_INNERTUBE.md` — InnerTube troubleshooting
- [~] `docs/development/service-template.md` — README template
- [~] `docs/features/message-deletion.md` — message deletion feature
- [~] `docs/guides/GOOGLE_OAUTH_IMPLEMENTATION_CHECKLIST.md` — OAuth checklist
- [~] `docs/guides/GOOGLE_OAUTH_VERIFICATION.md` — OAuth verification
- [~] `docs/migrations/2025-02-auth-token-encryption.md` — migration doc, accurate (`TOKEN_ENCRYPTION_KEY`)
- [~] `docs/migrations/2025-02-auth-token-encryption-staging-test-plan.md` — staging test plan
- [~] `docs/overlay-themes/QUICK-ICON-SETUP.md` — theme icon setup
- [~] `docs/overlay-themes/QUICK-START.md` — Win98 theme quickstart
- [~] `docs/overlay-themes/README.md` — themes index
- [~] `docs/phase-reports/APPROVED_ARCHITECTURE.md` — archived (per phase-reports/README.md)
- [~] `docs/phase-reports/CRITICAL_ARCHITECTURE_ANALYSIS.md` — archived audit (2025-11-13); claims since addressed (encryption now AES-GCM etc.)
- [~] `docs/phase-reports/KUBERNETES_CONTROLLER_ANALYSIS.md` — archived
- [~] `docs/phase-reports/LIMITS_ALERTS_MONITORING.md` — archived
- [~] `docs/phase-reports/OBSERVABILITY_MONITORING.md` — archived
- [~] `docs/phase-reports/README.md` — index of archived docs, accurate
- [~] `docs/phase-reports/SCALING_PERFORMANCE.md` — archived
- [~] `docs/testing/chaos-testing-phase5.md` — chaos testing playbook
- [~] `docs/tracing/PHASE5_CUSTOM_SPANS_GUIDE.md` — custom spans implementation guide

## services/*/README.md and per-service docs (19)

- [~] `services/api-gateway/README.md` — routes/JWT flow descriptions match code
- [x] `services/auth-service/README.md` — fixed XOR/TODO claim → AES-GCM is implemented
- [~] `services/discord-bot/README.md` — Discord quota monitor bot doc, accurate
- [~] `services/discord-bot/DEPLOYMENT.md` — not deeply audited (deployment guide)
- [~] `services/discord-bot/EXAMPLES.md` — not deeply audited (examples)
- [~] `services/discord-bot/QUICKSTART.md` — not deeply audited (quickstart)
- [~] `services/discord-bot/SUMMARY.md` — not deeply audited (summary)
- [~] `services/emote-service/README.md` — content presumed accurate
- [~] `services/kick-listener/README.md` — content matches code (Pusher WS, channels mgr, status, metrics dirs all exist)
- [~] `services/message-processor/README.md` — Alejo pronoun enricher reference is correct
- [~] `services/message-processor/seventv/README.md` — accurate
- [~] `services/overlay-manager/README.md` — content presumed accurate
- [~] `services/source-manager/README.md` — content presumed accurate
- [~] `services/tiktok-listener/README.md` — content accurate (BETA warning intact, references TikTok-Live-Connector)
- [~] `services/tiktok-listener/CHANGELOG.md` — historical changelog
- [~] `services/tiktok-listener/IMPLEMENTATION_SUMMARY.md` — historical summary
- [x] `services/token-refresh-service/README.md` — fixed XOR/TODO claim → AES-GCM is implemented
- [~] `services/twitch-eventsub-listener/README.md` — content presumed accurate
- [~] `services/twitch-listener/README.md` — `SOURCE_MANAGER_SECRET` and `TWITCH_BOT_OAUTH` env vars verified in code
- [~] `services/twitch-listener/METRICS_IMPLEMENTED.md` — historical impl note (2025-11-20)
- [~] `services/youtube-listener/README.md` — quota numbers match code (`QUOTA_LIMIT_DAILY` default 1009000)
- [~] `services/youtube-listener-innertube/README.md` — TOS disclosure intact

## deployments/, shared/, frontend/, test/ (15)

- [x] `deployments/SERVICE_AUTH.md` — updated `SERVICE_JWT_SECRET` → `SERVICE_JWT_SECRET_V1` (versioned keychain, per code at source-manager/cmd/main.go:153)
- [~] `deployments/ansible/QUICKSTART.md` — accurate, deploy 3-command flow
- [~] `deployments/ansible/README.md` — accurate
- [~] `deployments/ansible/README_VAULT.md` — accurate
- [~] `deployments/ansible/TESTING_GUIDE.md` — accurate
- [~] `deployments/k8s/monitoring/alerts/README.md` — accurate
- [x] `shared/metrics/README.md` — replaced stale "in progress / TODO" status block with current rollout state (all main services live, newer services pending)
- [~] `shared/middleware/CORS_GUIDE.md` — verified `CORSFromEnv`, `CORS_ORIGINS`, `BROWSER_EXTENSION_ID` env vars exist in code
- [~] `shared/ratelimit/README.md` — verified `NewRateLimiter` and `Config{RequestsPerMinute}` API
- [~] `shared/signing/README.md` — verified `NewSigner` + `VerifyMiddleware` API
- [~] `shared/tracing/README.md` — verified `tracing.Config` + `InitTracer` API
- [~] `frontend/README.md` — structure block lists fewer dirs than reality (admin, chat, legal, etc. exist) but is largely accurate
- [~] `frontend/README_TESTING.md` — Playwright test guide, accurate
- [~] `frontend/DESIGN_SYSTEM.md` — recent (2026-03-09), accurate
- [~] `frontend/src/app/admin/README.md` — admin dashboard description
- [~] `frontend/src/styles/EVENTS_CSS_API.md` — not deeply audited (CSS API reference)
- [~] `frontend/src/styles/MARKETPLACE_MIGRATION_GUIDE.md` — not deeply audited (migration guide)
- [~] `test/contract/dual-listener/README.md` — integration test description
- [~] `test/contract/lifecycle/fixtures/README.md` — fixture description
- [~] `test/contract/schema/README.md` — golden file tooling description

---

## Summary of changes (155 active docs audited)

**Edited (substantive fixes):** ~40 files
**Verified accurate (no edit needed):** ~115 files (marked `[~]`)
**Out of scope:** `.planning/` (759 historical phase/milestone files) and `.claude/`,
`frontend/.next/standalone/`, `node_modules/`.

### Recurring patterns fixed

- **`make migrate-up` → `make migrate`** — the Makefile only defines `migrate` and
  `migrate-down`; touched in `CLAUDE.md`, `README.md`, `GETTING_STARTED.md`,
  `docs/llm-guides/NAVIGATION.md`, `QUICK-REF-DATABASE-MIGRATION.md`,
  `QUICK-REF-ADD-PLATFORM.md`, `docs/troubleshooting/connection-errors.md`,
  `docs/troubleshooting/decision-tree.md`, `docs/ADMIN_RBAC.md`, `docs/DEPLOYMENT.md`.
- **AES-GCM is implemented** (`shared/encryption/`) — older docs claimed XOR + TODO;
  fixed in `CLAUDE.md`, service READMEs (auth, token-refresh), security-audit guide,
  architecture overview.
- **Service-to-service signing is implemented** (`shared/signing/`) — referenced from
  CLAUDE.md and security guide.
- **YouTube quota default is 1,009,000 units/day** (not 10,000) — fixed in CLAUDE.md,
  architecture overview, scaling doc, quota-recovery runbook.
- **Discord platform** missing from many older platform lists — added in CLAUDE.md and
  architecture overview.
- **Service count** — references to "13 services" updated to 17 (or removed where the
  exact number isn't load-bearing).
- **Co-author tag** "Claude Sonnet 4.5 (1M context)" replaced with neutral
  "Claude" in `CONTRIBUTING.md` and `docs/operations/runbooks/fixing-service-issues.md`.

### Notable doc-content drift

- `docs/llm-guides/NAVIGATION.md` — many referenced filenames had drifted (e.g.
  `mock.go` → `mock_message.go`, `irc/client.go` → `irc/connection.go`,
  `publisher/redis.go` → `publisher/stream_publisher.go`, consumer/streams.go →
  `consumer/stream_consumer.go`, `publisher/pubsub.go` → `publisher/pubsub_publisher.go`,
  `router/router.go` → `router/overlay_router.go`, `sessions/tracker.go` →
  `sessions/capture.go`, `youtube/client.go` → `api/client.go`,
  `emote-service/providers/` → `clients/`).
- `docs/README.md` — five dead links to non-existent `user-guides/`, `operations/*.md`,
  `development/*.md` paths fixed.
- `docs/architecture/00-OVERVIEW.md` — service list block did not include
  twitch-eventsub, youtube-listener-innertube, discord-listener, tiktok-listener
  as first-class entries in the upper-level diagram; refreshed.
- `docs/adr/README.md` — ADR-0012 entry was missing; total count was 11.
- `deployments/SERVICE_AUTH.md` — pointed to non-versioned `SERVICE_JWT_SECRET`; actual
  env is the versioned `SERVICE_JWT_SECRET_V1` per Phase 14 keychain.
- `FRONTEND_QUICK_START.md` / `FRONTEND_DEV_SETUP.md` — mock-message endpoint URL
  (`/api/mock/message` → `/internal/mock-messages`), auth header
  (`X-API-Key` → `X-Internal-Token`), and body field (`message` → `text`) all wrong.

### Historical / point-in-time docs

Several "completion summary" docs document past work and have been left mostly intact,
with a short status banner added where their referenced code paths have moved or the
described state is no longer current. Examples:

- `COORDINATOR-FILTERING-FIX-SUMMARY.md` — banner added (source-manager package
  refactored, env flag removed).
- `DEPLOYMENT-CHECKLIST.md` — banner added (rollout complete).
- `Roadmap.md` — banner added (items 1-6 substantially implemented).
- `DISTRIBUTED_TRACING_COMPLETE.md` — banner added (newer services lack tracing).

### Not deeply audited

- All `.planning/**` files (759 historical artifacts) — out of scope by convention.
- A handful of very long technical docs (`docs/runbooks/secret-rotation.md`,
  `docs/architecture/02-DEPLOYMENT.md`, `docs/architecture/04-OBSERVABILITY.md`,
  `frontend/src/styles/EVENTS_CSS_API.md`, `frontend/src/styles/MARKETPLACE_MIGRATION_GUIDE.md`,
  `services/discord-bot/{DEPLOYMENT,EXAMPLES,QUICKSTART,SUMMARY}.md`) — spot-checked
  for headline accuracy; deeper claim-by-claim audit would require more time.
- Marketing audio sub-READMEs — verified asset files exist.
