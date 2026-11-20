# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

## Project Overview

All-Chat is a **cloud-native microservices platform** for aggregating and displaying chat messages from **multiple live streaming platforms** (Twitch, YouTube, Kick, TikTok, Discord; Owncast, GoodGame, Picarto, Altstream, Facebook and Instagram in beta) on streaming overlays with support for 7TV, BTTV, and FFZ emotes.

**Core Concept**: Users can create multiple overlays, each configured with one or more chat sources. An overlay can combine messages from every supported platform simultaneously, providing full flexibility for streamers who multistream.

**Architecture**: Standard Go Layout with microservices communicating via Redis Streams (raw messages) → Message Processor (normalization + enrichment) → Redis Pub/Sub (overlay-specific) → API Gateway WebSocket (client delivery).

**Platform Status**:
- ✅ Twitch (EventSub primary; IRC listener deprecated per ADR-0026) | ✅ YouTube (HTTP polling with quota tracking + InnerTube polling) | ✅ Kick (Centrifugo or Pusher, whichever Kick picks; ADR-0077) | ✅ TikTok (Unofficial library) | ✅ Discord (channel relay)
- 🚧 Beta only: Owncast (ADR-0058, instance URL as channel) | GoodGame (chat websocket, channel key) | Picarto (ADR-0059, unofficial pop-out websocket) | Facebook (ADR-0060, Graph API polling + moderation write path) | Instagram (ADR-0062, Graph API live_comments polling, read-only). The listeners are deployed and the backend is shared, but the frontend offers these platforms only on `beta.allch.at` (`frontend/src/lib/platform-availability.ts`); allch.at hides them. Adding a source is still limited by the ADR-0008 `platform_*` gate (seeded `is_premium=TRUE`, migrations 091-094 and 096; beta testers are granted premium). The gate also honours `early_access` (ADR-0020, beta testers and ambassadors only): keep `platform_facebook` and `platform_instagram` on early access until Meta App Review passes, since until then only Meta app testers can connect. Release to production by removing the platform from `platform-availability.ts`, then flip its gate via the feature-gate admin endpoint to open it beyond premium.
- 🚧 Early access: Altstream (ADR-0082, official public SSE chat API, read-only). Offered on every host, but on allch.at only to early-access users (beta testers and ambassadors), via `EARLY_ACCESS_PLATFORMS` in `platform-availability.ts`; adding a source is limited by the `platform_altstream` gate, seeded `early_access=TRUE`, `is_premium=FALSE` (migration 117). The listener runs from `:main`. Release by removing `altstream` from `EARLY_ACCESS_PLATFORMS`, then turning `early_access` off on the gate in `/admin/features`.
- ✅ Alerts overlays (ADR-0070, ADR-0075) and the alerts editor (ADR-0071), including custom HTML/CSS/JS and KLIPY GIF search, are free for everyone on every host: the `alerts_overlay` and `alerts_editor` gates were opened on 2026-11-15. Uploading your own images, GIFs and sounds stays premium through the `media_upload` gate (migration 128) on media-service's presign.
- ✅ Kick events (ADR-0094) reach chat overlays (per-overlay toggles) and alerts overlays (own alert profiles): subscriptions, gifted subs and hosts from Kick's undocumented realtime feed, follows and KICKs gifts from Kick's official webhooks (`POST /webhooks/kick` on kick-listener, subscribed per Kick source with an app token; needs `KICK_CLIENT_ID`/`KICK_CLIENT_SECRET` and the webhook URL set in the Kick developer dashboard).
- ✅ Chat commands `/commands` (ADR-0079, ADR-0080, ADR-0081) are free for everyone on every host: the `chat_commands` gate was opened on 2026-11-15. Replies and timers in YouTube chat stay premium (checked per send on the owner's premium), and connecting your own bot account is limited by the `custom_chat_bot` premium gate (early access off).
- 🚧 Beta only (overlay kinds): Goal and List overlays (ADR-0072). The same `platform-availability.ts` list hides them outside `beta.allch.at`: the create-overlay kind picker, the onboarding "Optional: go further" entries and the `/upgrade` rows. Existing overlays of these kinds still render in OBS (`/overlay/[id]`) and open in the editor on any host, and creating one is still limited by the `goal_overlay` / `list_overlay` premium gates. Release to production by removing the kind from `BETA_ONLY_OVERLAY_KINDS` in `platform-availability.ts`.

---

## Quick Start

```bash
# Full development environment (all services)
make docker-up         # Start postgres, redis, all services
make test              # Run tests
make migrate           # Apply database migrations (use `make migrate-down` to roll back)

# Frontend-only development (minimal backend)
make frontend-dev      # Start postgres, redis, gateway, overlay-manager, message-processor
make frontend-seed     # Create test overlay and chat sources
make frontend-messages # Generate mock chat messages
cd frontend && npm run dev  # Start frontend

# Access services
# - API Gateway: http://localhost:8080
# - Frontend: http://localhost:3000
```

**First Time Setup**: See [GETTING_STARTED.md](./docs/GETTING_STARTED.md) for complete onboarding guide.

**Frontend Development**: See [FRONTEND_QUICK_START.md](./docs/frontend/FRONTEND_QUICK_START.md) for minimal backend setup. All-in-one: `make frontend-quick` (starts services, seeds data, verifies setup).

---

## Navigation by Task

### Common Tasks → Quick Reference Guides

**Need to...**

|Task|Guide|
|---|---|
|Build or change any frontend UI|[DESIGN_SYSTEM.md](./frontend/DESIGN_SYSTEM.md) (skill: `shadcn-ui`)|
|Add support for a new platform|[QUICK-REF-ADD-PLATFORM.md](./docs/llm-guides/QUICK-REF-ADD-PLATFORM.md)|
|Debug YouTube quota issues|[QUICK-REF-DEBUG-QUOTA.md](./docs/llm-guides/QUICK-REF-DEBUG-QUOTA.md)|
|Add a new HTTP endpoint|[QUICK-REF-ADD-ENDPOINT.md](./docs/llm-guides/QUICK-REF-ADD-ENDPOINT.md)|
|Perform security audit|[QUICK-REF-SECURITY-AUDIT.md](./docs/llm-guides/QUICK-REF-SECURITY-AUDIT.md)|
|Scale services or infrastructure|[QUICK-REF-SCALING.md](./docs/llm-guides/QUICK-REF-SCALING.md)|
|Create database migration|[QUICK-REF-DATABASE-MIGRATION.md](./docs/llm-guides/QUICK-REF-DATABASE-MIGRATION.md)|
|Debug Kubernetes issues|[QUICK-REF-KUBERNETES-DEBUG.md](./docs/llm-guides/QUICK-REF-KUBERNETES-DEBUG.md)|
|Inspect Redis Streams/Pub/Sub|[QUICK-REF-REDIS-OPERATIONS.md](./docs/llm-guides/QUICK-REF-REDIS-OPERATIONS.md)|

### Troubleshooting

Start with the [decision tree](./docs/troubleshooting/decision-tree.md); it links the specific guides in `docs/troubleshooting/` (build errors, connection errors, WebSocket disconnects, YouTube). Operational procedures are in `docs/runbooks/`.

### Architecture & Design Decisions

- [docs/README.md](./docs/README.md) - index of all docs
- Architecture: [Overview](./docs/architecture/00-OVERVIEW.md), [Data Flow](./docs/architecture/01-DATA-FLOW.md), [Deployment](./docs/architecture/02-DEPLOYMENT.md), [Scaling](./docs/architecture/03-SCALING.md), [Observability](./docs/architecture/04-OBSERVABILITY.md), [Security](./docs/architecture/05-SECURITY.md)
- [ADR Index](./docs/adr/README.md) - Architecture Decision Records: why each design was chosen and how it is implemented today

### Service Documentation

Each service has a detailed README:
- [api-gateway](./services/api-gateway/README.md) - WebSocket server, HTTP routing
- [auth-service](./services/auth-service/README.md) - OAuth flows, JWT tokens; also ships `retention-sweeper`, the 15-minute CronJob that enforces the privacy-policy retention periods (ADR-0089)
- [emote-service](./services/emote-service/README.md) - 7TV, BTTV, FFZ emote APIs
- [twitch-listener](./services/twitch-listener/README.md) - IRC client, channel management
- [twitch-eventsub-listener](./services/twitch-eventsub-listener/README.md) - EventSub webhooks (channel points, moderation)
- [youtube-listener](./services/youtube-listener/README.md) - Data API/gRPC chat for premium official-API mode only: opted-in, owner-verified channels (unlisted + members-only streams), claimed away from innertube (ADR-0065)
- [youtube-listener-innertube](./services/youtube-listener-innertube/README.md) - InnerTube API polling (quota-free chat ingestion for every unclaimed channel, pinned stream links for unlisted streams; optional subscriber-alert polling spends accounted Data API units)
- [youtube-quota-monitor](./services/youtube-quota-monitor/README.md) - Reads the shared YouTube quota table; exports the quota metric + publishes `quota:alerts` for the discord-bot (ADR-0023)
- [kick-listener](./services/kick-listener/README.md) - Kick realtime client: Centrifugo (pooled, ≤100 channels per connection, two per chatroom) or Pusher, chosen by Kick at startup (ADR-0077); publishes chat plus Kick events from the feed and from Kick's official webhooks (ADR-0094)
- [tiktok-listener](./services/tiktok-listener/README.md) - Unofficial TikTok Live library
- [tiktok-signer](./services/tiktok-signer/README.md) - Self-hosted TikTok webcast signing (X-Bogus/X-Gnarly via TikTok's own SDK in a headless browser; replaces Euler Stream, ADR-0052)
- [tiktok-relay](./services/tiktok-relay/README.md) - US-region relay for TikTok's live lookup by handle (Cloud Run, not in the cluster); used by tiktok-listener for accounts that only resolve from the US
- discord-listener — Discord channel chat relay; relay webhook URLs are credentials, encrypted with `TOKEN_ENCRYPTION_KEY_V<n>` and never returned by the API (ADR-0091) (`services/discord-listener/`, no README yet)
- [owncast-listener](./services/owncast-listener/README.md) - Self-hosted Owncast instances; one websocket per instance URL (ADR-0058)
- [goodgame-listener](./services/goodgame-listener/README.md) - GoodGame.ru chat websocket; resolves channel key to numeric chat id
- [picarto-listener](./services/picarto-listener/README.md) - Picarto pop-out chat websocket, defensively parsed (ADR-0059)
- [altstream-listener](./services/altstream-listener/README.md) - Altstream chat via the official public SSE chat API, read-only; the `lastId` parameter is mandatory (ADR-0082)
- [facebook-listener](./services/facebook-listener/README.md) - Facebook Live comments via Graph API polling + moderation (ADR-0060)
- [instagram-listener](./services/instagram-listener/README.md) - Instagram Live comments via Graph API `live_comments` polling, read-only (ADR-0062)
- [message-processor](./services/message-processor/README.md) - Normalization, emote enrichment
- [alert-processor](./services/alert-processor/README.md) - Second `chat:raw` consumer (group `alert-processors`): routes alert event types (an allowlist, so the owner-only `mod_action` log and system warnings never pass) to alerts/goal/list overlays on `overlay:{id}:alerts`, persists `alert_events`, scores list leaderboards and pushes debounced `list_update` frames, counts bits/Super Chat goals and polls follower/subscriber goals of connected goal overlays (ADR-0068, ADR-0072)
- [media-service](./services/media-service/README.md) - Alert-media uploads (presigned PUT straight to MinIO, `media_objects` registry; presigning needs premium via the `media_upload` gate); sweeps the public bucket for files whose registry row is gone, e.g. with a deleted account (ADR-0088)
- [overlay-manager](./services/overlay-manager/README.md) - Overlay CRUD, source configuration
- [source-manager](./services/source-manager/README.md) - Leader election, active source registry
- share-service — Shareable overlay links; admin premium/beta-tester/ambassador role grants + feature gates; public ambassador showcase (ADR-0041); localization contributor tool `/translate` + admin review/export (ADR-0063); UI language gate probe `GET /api/v1/ui-languages` (ADR-0066) (`services/share-service/`, no README yet)
- [moderation-service](./services/moderation-service/README.md) - Cross-platform chat moderation write-path (delete/timeout/ban, ADR-0017)
- [payment-service](./services/payment-service/README.md) - Patreon premium entitlements; writes users.is_premium (streamer, ADR-0018) + viewers.is_premium (viewer split, ADR-0019)
- [token-refresh-service](./services/token-refresh-service/README.md) - OAuth token refresh
- [discord-bot](./services/discord-bot/README.md) - TypeScript Discord bot (community ops, not a listener)
- [engagement-service](./services/engagement-service/README.md) - Cross-platform polls, predictions, and per-overlay viewer points economy (#523); streamer chat commands and keyword triggers with templated/JS replies, `{urlfetch}`, Twitch clips and images on alerts overlays (ADR-0079, ADR-0080)
- [support-bot](./services/support-bot/README.md) - Discord support/admin agent on Claude: the Claude Code CLI on a subscription token in prod (bot tools served to it over localhost MCP), else an Anthropic API key, else an OpenAI-compatible gateway; files confirmed bugs as `agent` issues for Caterpillar to fix (ADR-0093)

### Development Guides

- [GETTING_STARTED.md](./docs/GETTING_STARTED.md) - Local setup and where to read next
- [CONTRIBUTING.md](./CONTRIBUTING.md) - Pull request process, code review guidelines
- [Testing](./docs/TESTING.md) - Test targets and CI workflows
- [Accessibility](./docs/ACCESSIBILITY.md) - WCAG 2.2 AA scope, CI gates (shrink-only ratchets), contracts for new UI code
- [Frontend i18n](./docs/frontend/I18N.md) - the UI string catalog (English, German, French, Spanish): how to add a string, placeholder syntax, how a locale is chosen and what stays English, why there is no provider and no locale in the URL
- [Agent UI verification](./docs/frontend/AGENT_UI_VERIFICATION.md) - the screenshot ritual and pixel-baseline gate every UI change must pass before "done"
- [Design System](./frontend/DESIGN_SYSTEM.md) - shadcn primitives, design tokens, and the CI gates that keep the UI consistent (ADR-0056). Agents: use the `shadcn-ui` skill

---

## Documentation Conventions

Keep documentation from rotting. **There are no point-in-time docs anywhere in the tree** (`docs/`, `services/*/`, `.github/`, the root). The repo has twice accumulated thousands of lines of them and had to be cleaned out; recover anything deleted from git history.

- **The repo root holds only three markdown docs:** `README.md`, `CLAUDE.md`, and `CONTRIBUTING.md`. Everything else lives under `docs/`; [docs/README.md](./docs/README.md) is its index.
- No `*-FIX-SUMMARY.md`, `*_IMPLEMENTATION.md`, `*-CHECKLIST.md`, `*_COMPLETE.md`, `*_STATUS.md`, plans, spikes, handoffs, review findings, phase reports, or one-off audit/TODO trackers. They describe a single past change, go stale within weeks, and cross-link mostly to each other, so they look referenced while nothing living points at them.
- **The point-in-time account goes in the PR description or commit body**, where it is dated and attributed and never has to be maintained. A lasting decision goes in an ADR.
- Three questions decide what to do with an existing doc. Does it describe a single past change? Delete it. Does it state facts about how the system works today? Fix those facts in place. Does it record a decision and why it was taken? It belongs in `docs/adr/`.
- Keep docs short: what it is, how it works, how to run it, where the code lives. Link to the source or another doc instead of copying code or restating it.
- `docs/adr/` describes the current state. An ADR records a decision *and* how the system implements it today: when code changes what an ADR says, fix the ADR body in the same change, fold amendments from later ADRs into it (with a cross-link), and add or bump a `**Revised**: YYYY-MM-DD` line under `**Status**`. A replaced ADR gets `Superseded by ADR-XXXX` and a body that points at its successor; a removed feature gets `Withdrawn`. Never delete an ADR or renumber one. ADR numbers are shared with the deployment repo, so gaps in `docs/adr/` are expected.

**Update docs in the same change as the code.** When you change behavior, update the living docs it affects in the same PR: `README.md`, `CLAUDE.md`, `docs/GETTING_STARTED.md`, the relevant `services/*/README.md`, and the ADR index. A doc that contradicts the code is worse than no doc.

**Do not hardcode volatile counts.** Prefer "see `services/`" over "17 services" and "see `docs/adr/`" over "12 ADRs": exact counts drift every time something is added. Where a count is unavoidable, treat it as something to re-verify, not to trust.

**When a doc goes stale, anywhere in the tree:** fix the facts in place if it is a living reference; `git rm` it (recoverable via history) if it is a completed point-in-time artifact. Deleting is the normal outcome and needs no ceremony — no deprecation banner, no tombstone file, no note in an index. When you delete one, fix every inbound link in the same change: use `git grep`, and leave no "see X" pointing at a file that is gone.

---

## Commits and Pull Requests

**Commits and PRs describe the change, never the tool that produced it.** We do not advertise any third-party tool: no editor, assistant, model, bot or service gets credit or a plug in commit messages, squash-merge messages, PR titles or PR bodies. That rules out a `Co-authored-by:` trailer naming such a tool, a "Generated with ..." footer, a robot emoji, and a link to a tool's website. Many tools add these by default; turn that default off rather than deleting the line by hand each time. People are credited as usual.

**Caterpillar is exempt.** It is our own agent: `caterpillar-agent[bot]` authors, co-authors and opens PRs under its own name, and that stays. Its commits must still not credit a third-party tool. Bots that author their own PRs, such as dependabot, keep their name as author or co-author too; that is authorship, not advertising.

GitHub copies every `Co-authored-by:` trailer from a PR's branch commits into the squash commit, so one tainted branch commit taints `main`. Keep the branch commits clean, and read the squash message before merging. Removing a trailer from `main` afterwards means rewriting published history and force-pushing; that has already had to happen more than once.

The `no tool attribution` check (`.github/workflows/no-tool-attribution.yml`) fails a PR whose commit messages, title or body carry such a line. It matches a list of known tools, not every tool there is. It cannot see text typed into the squash dialog at merge time, and admins pushing straight to `main` bypass it.

---

## Shipping a Feature

**Release cycle** (set by the maintainer; the full version is in [CONTRIBUTING.md](./CONTRIBUTING.md#release-flow)):

1. **Build locally.** Develop and test the feature on a branch.
2. **Release to beta** by merging into the `beta` branch. A push to `beta` builds `:beta` images (`build-and-push.yml`, tag `type=ref,event=branch`), and Keel deploys `frontend-beta` and `api-gateway-beta`, which serve beta.allch.at.
3. **Release to `main` (prod)** only after a human has checked beta and greenlit it. **Agents never promote to `main` on their own.**

Fixes may still go to `main` through PRs.

**Gap:** only frontend and api-gateway have a beta deployment next to prod. The beta-platform listeners (facebook, goodgame, owncast, picarto) run from `:beta` only, and every other service runs from `:main` only. A backend feature in any other service has no beta environment, so a human decides how to stage it.

On top of the cycle, a new user-facing feature is not "done" until all four of these release steps are covered:

1. **Premium toggle.** New features ship behind a premium feature gate (ADR-0008: the `feature_gates` table, enforced via `shared/middleware` RequirePremium/RequireEarlyAccess, managed through the admin feature-gates API). This lets premium/beta access be flipped without a deploy.
2. **Onboarding extras tour.** Because features are premium-toggled, every new feature must be added to the optional tour in the first-run setup guide: the "Optional: go further" list in `frontend/src/components/onboarding/OnboardingChecklist.tsx` AND the premium features list in `frontend/src/app/upgrade/page.tsx`. The tour copy mirrors `/upgrade` and the two must stay in sync (there is a keep-in-sync comment at both sites).
3. **Public pages.** The public pages are documentation, and they must say what the product does today in the same PR that ships, releases, renames or removes the feature. The files and the rules are under [Public pages are documentation](#public-pages-are-documentation) below.
4. **Patreon post.** Treat the announcement as part of the release: once the feature is merged and deployed, publish a Patreon update describing what changed and what it does for streamers.

> A workflow to help draft and publish these posts is planned. Until it exists, write the post manually.

### Public pages are documentation

Search engines and AI assistants quote the public pages as fact, and they keep
quoting them long after the product has moved on. When alerts shipped, the
site went on saying "alerts are in development", and assistants told streamers
All-Chat had no alerts and recommended a competitor. A change that ships,
releases (beta to everyone, premium to free), renames or removes a
user-facing feature is not done until every page below says what the product
does today, in the same PR:

- `frontend/src/lib/llms.ts`: `/llms.txt` and `/llms-full.txt`. Summary line, feature
  list, free and premium split, limits.
- `frontend/src/app/docs/page.tsx` and `docs.guide.*`: the public docs.
- The guides (`GUIDES` in `frontend/src/components/guides/GuidePage.tsx`, copy in
  `guides.*`). A feature people would search for by name gets its own guide.
- `frontend/src/components/guides/compareData.ts`: the All-Chat row of `/compare` and
  the `/compare/<tool>` pages, plus the `guides.vs.*` prose.
- The landing FAQ (`frontend/src/lib/faq.ts`, `marketing.faq.*`), the homepage
  `SoftwareApplication` `featureList` in `frontend/src/app/page.tsx`, and
  `metadata.site` / `metadata.home`.
- `frontend/src/app/upgrade/page.tsx` and the onboarding tour (step 2 above).

Rules for that copy:

- Claim only what works today. Read the code for the exact scope (which
  platforms, which tier) before writing a sentence; do not copy it from another
  page.
- State limits plainly. A missing platform or a premium part goes under
  "Limits", not out of sight.
- A statement about another product needs a source on that product's own
  pages; without one the cell is "not documented".
- English, German, French and Spanish change together.
- `frontend/src/app/__tests__/seo-metadata.test.ts` locks the claims that have gone
  stale before. When a page gains a claim worth protecting, add it there.

---

## Tech Stack

**Backend**: Go 1.26+, Gin (HTTP), PostgreSQL 16 (pgx/v5), Redis 7 (go-redis/v9), Zap (logging)

**Frontend**: React 19+, Next.js 16+ (App Router), TypeScript, Tailwind CSS, Zustand

**Infrastructure**: Kubernetes (CNPG for PostgreSQL), Docker Compose (local dev)

**Key Libraries**:
- `gempir/go-twitch-irc/v4` - Twitch IRC client
- `gorilla/websocket` - Kick Centrifugo/Pusher WebSocket, API Gateway WebSocket server
- `golang-jwt/jwt/v5` - JWT authentication

---

## Standard Go Service Layout

All services follow this structure:

```
services/<service-name>/
├── cmd/main.go           # Entry point (logger, DB/Redis, HTTP server)
├── handlers/             # HTTP handlers (Gin)
├── models/               # Data models
├── repository/           # Database layer (if needed)
├── <domain-packages>/    # Domain logic (oauth/, streams/, channels/, etc.)
├── go.mod                # Dependencies
└── Dockerfile            # Container image
```

**Key Principles**:
- Clear separation: handlers → domain logic → repository
- Dependency injection for testability
- Graceful shutdown (25s timeout)
- Health checks: `/health/live` (always 200), `/health/ready` (checks DB + Redis)
- Runtime image: `gcr.io/distroless/static-debian13`, so no shell in the pod: debug with `kubectl port-forward` or `kubectl debug --target`, never `kubectl exec -- sh` (ADR-0092)

---

## Message Flow Architecture

```
Listeners (Twitch/YouTube/Kick/TikTok/Discord/Owncast/GoodGame/Picarto/Altstream/Facebook/Instagram)
  ↓ publish raw messages
Redis Streams (chat:raw)
  ↓ consume via XREADGROUP (group: message-processor)
Message Processor
  ├─ Normalize (platform → unified format)
  ├─ Enrich (7TV, BTTV, FFZ emotes)
  └─ Publish to Redis Pub/Sub (overlay:{overlay_id})
    ↓ subscribe
API Gateway WebSocket
  ↓ broadcast
Frontend Overlay (React)
```

**Unified Message Format**: All platforms normalized to common schema with user, message, emotes, badges, metadata.

**See**: [Data Flow Integration](./docs/architecture/01-DATA-FLOW.md) for complete details.

---

## Environment Variables

**Required for local development**:

```bash
# Twitch
TWITCH_BOT_USERNAME=your_bot
TWITCH_BOT_OAUTH=oauth:token_from_twitchapps

# YouTube
YOUTUBE_CLIENT_ID=xxx.apps.googleusercontent.com
YOUTUBE_CLIENT_SECRET=GOCSPX-xxx

# Database (localhost defaults)
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_NAME=allchat
DATABASE_USER=allchat
DATABASE_PASSWORD=allchat_dev_password

# Redis (localhost defaults)
REDIS_HOST=localhost
REDIS_PORT=6379
```

**See**: `.env.example` for complete list. **Kubernetes**: Secrets managed via sealed-secrets.

---

## Known Issues & Technical Debt

- Service-to-service signing is implemented in `shared/signing/` (HMAC-SHA256, constant-time compare, query+service-name signed, replay-protected) but NOT YET WIRED into any prod service; Kubernetes NetworkPolicies are the current service-to-service isolation control. See docs/security/SECURITY_AUDIT_REPORT.md L2 + docs/security/RESIDUALS.md
- Integration tests incomplete for YouTube Listener; no E2E test covers the full message flow.
- Shared PostgreSQL database across services.

**See**: [Security Architecture](./docs/architecture/05-SECURITY.md) and [Deployment](./docs/architecture/02-DEPLOYMENT.md).

---

## External Resources

- **Twitch OAuth**: https://twitchapps.com/tmi/
- **YouTube API Console**: https://console.developers.google.com/
- **7TV API**: https://7tv.io/docs
- **BTTV API**: https://betterttv.com/developers
- **FFZ API**: https://www.frankerfacez.com/developers

---

## Need More Help?

1. **Start with**: [GETTING_STARTED.md](./docs/GETTING_STARTED.md) - Complete navigation guide
2. **Troubleshooting**: [Decision Tree](./docs/troubleshooting/decision-tree.md) - Triage common issues
3. **Architecture**: [ADR Index](./docs/adr/README.md) - Understand design decisions
4. **Service Details**: [services/*/README.md](./services/) - Service-specific documentation
5. **Quick Tasks**: [docs/llm-guides/](./docs/llm-guides/) - Task-oriented quick references

**Database Note**: PostgreSQL is deployed in Kubernetes cluster (namespace: allchat) as a CNPG (CloudNativePG) cluster with automated failover and backups.
