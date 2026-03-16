# Roadmap: All-Chat

## Milestones

- ✅ **v1.0 Message Deletion Support** — Phases 1-3 (partial, shipped 2026-02-18)
- ✅ **v1.1 Listener Load Balancing** — Phases 4-10 (shipped 2026-02-21)
- ✅ **v1.2 InnerTube YouTube Listener** — Phases 11-22 (shipped 2026-03-06)
- ✅ **v1.3 Frontend Redesign** — Phases 23-26 (shipped 2026-03-14)
- ✅ **v1.4 Viewer Identity & YouTube Enrichment** — Phases 27-32 (shipped 2026-03-16)

## Phases

<details>
<summary>✅ v1.0 Message Deletion Support (Phases 1-3) — SHIPPED 2026-02-18</summary>

### Phase 1: Foundation + Twitch
**Goal**: Establish message deletion infrastructure with Twitch platform
**Plans**: 5/5 complete
**Status**: Complete (2026-02-18)

### Phase 2: YouTube Integration
**Goal**: YouTube deletion support via polling
**Plans**: 2/2 complete
**Status**: Complete (2026-02-18)

### Phase 3: Kick Integration + Edge Cases
**Goal**: Kick WebSocket deletion events and reconnection handling
**Plans**: 4/4 complete
**Status**: Complete (2026-02-18)

**Note**: Phase 4 (TikTok Integration) was deferred, ending v1.0 at 80% completion.

</details>

<details>
<summary>✅ v1.1 Listener Load Balancing (Phases 4-10) — SHIPPED 2026-02-21</summary>

**Milestone Goal:** Implement hybrid hash-based sharding with load-aware rebalancing for all listener services, enabling cost-effective scaling and reliable service for high-volume streams.

### Phase 4: Sharding Infrastructure & Coordinator Service
**Goal**: Production-ready consistent hashing and coordinator service with split-brain prevention
**Plans**: 5/5 complete
**Status**: Complete (2026-02-19)

### Phase 5: Connection Management & Migration Protocol
**Goal**: All platform listeners integrate with coordinator and support graceful zero-loss channel migration
**Plans**: 8/8 complete
**Status**: Complete (2026-02-20)

### Phase 6: Dynamic Rebalancing & HPA Integration
**Goal**: Automatic load-aware rebalancing with safeguards against thundering herd and quota exhaustion
**Plans**: 4/4 complete
**Status**: Complete (2026-02-20)

### Phase 7: Observability & Production Readiness
**Goal**: Comprehensive metrics, distributed tracing, Grafana dashboards, and alerting for production operations
**Plans**: 4/4 complete
**Status**: Complete (2026-02-20)

**Delivered:**
- Bounded-load consistent hashing with Kubernetes Lease-based coordinator
- Zero-loss channel migration protocol across all platforms (Twitch, Kick, TikTok)
- Load-aware automatic rebalancing with composite scoring and cooldown safeguards
- Comprehensive observability with 16 distributed tracing spans, Grafana dashboards, and Prometheus alerts

**Archive**: [v1.1-ROADMAP.md](milestones/v1.1-ROADMAP.md) | [v1.1-REQUIREMENTS.md](milestones/v1.1-REQUIREMENTS.md) | [v1.1-MILESTONE-AUDIT.md](milestones/v1.1-MILESTONE-AUDIT.md)

</details>

<details>
<summary>✅ v1.2 InnerTube YouTube Listener (Phases 11-22) — SHIPPED 2026-03-06</summary>

**Milestone Goal:** Build quota-free YouTube listener using InnerTube API as drop-in replacement for official API listener, maintaining identical downstream behavior while eliminating quota limitations.

### Phase 11: Core Ingestion PoC
**Goal**: Validate InnerTube API viability by establishing basic message flow from InnerTube to Redis Streams
**Plans**: 5/5 complete
**Status**: Complete (2026-02-21)

### Phase 12: Production Minimum
**Goal**: Enable dynamic stream management and production lifecycle behaviors
**Plans**: 4/4 complete
**Status**: Complete (2026-02-21)

### Phase 13: Contract Validation
**Goal**: Prove behavioral equivalence with official youtube-listener through comprehensive contract testing
**Plans**: 4/4 complete
**Status**: Complete (2026-02-21)

### Phase 14: Production Rollout
**Goal**: Deploy to production with gradual canary rollout, monitoring, and automatic rollback
**Plans**: 3/3 complete
**Status**: Complete (2026-03-05)

### Phase 15: Feature Parity
**Goal**: Add deletion event detection and advanced metrics leveraging InnerTube advantages
**Plans**: 5/5 complete
**Status**: Complete (2026-03-06)

**Delivered:**
- InnerTube API integration eliminates YouTube API quota constraints (10,000 units/day → unlimited)
- RawChatMessage schema byte-for-byte compatible (drop-in replacement, zero downstream changes)
- All event types supported (messages, Super Chat, Super Sticker, memberships, milestones, deletions)
- Batch deletion detection with time-windowed aggregation (5+ deletions in 100ms)
- Production canary infrastructure (Argo Rollouts, Prometheus metrics, automatic rollback)
- 8,684 LOC InnerTube service with 69 automated tests

**Archive**: [v1.2-ROADMAP.md](milestones/v1.2-ROADMAP.md) | [v1.2-REQUIREMENTS.md](milestones/v1.2-REQUIREMENTS.md) | [v1.2-MILESTONE-AUDIT.md](milestones/v1.2-MILESTONE-AUDIT.md)

</details>

<details>
<summary>✅ v1.3 Frontend Redesign (Phases 23-26) — SHIPPED 2026-03-14</summary>

**Milestone Goal:** Transform frontend from generic Tailwind defaults to professional streaming-focused design system with comprehensive component library and enforceable style rules.

- [x] **Phase 23: Design Token System & Foundation** — Tailwind v4 three-tier @theme tokens, cascade layers, static platform colors, events.css stability contract (3/3 plans, completed 2026-03-10)
- [x] **Phase 24: Component Library Setup & Customization** — @base-ui/react + shadcn/ui, CVA variants, micro-interactions, platform-color components, all Storybook tests pass (5/5 plans, completed 2026-03-11)
- [x] **Phase 25: Page Migration & Split-view Preview** — All 6 pages redesigned, draggable SplitView, Dialog/Toast patterns, zero legacy gray-* classes (8/8 plans, completed 2026-03-11)
- [x] **Phase 26: Enforcement & Quality Gates** — ESLint flat config + Prettier, Husky pre-commit, 7-gate CI pipeline, Chromatic visual regression, marketplace migration guide (4/4 plans, completed 2026-03-14)

**Delivered:**
- Tailwind v4 design token system with cascade layer architecture eliminating all !important overrides
- Full component library (Button, Card, Input, Badge, Dialog, Toast, Skeleton) with CVA variants and platform-color coding
- Professional streaming dark aesthetic across all pages with WCAG 2.1 AA accessibility compliance
- Draggable SplitView live preview component with pointer-capture drag and keyboard navigation
- Automated enforcement: ESLint + Prettier + Husky pre-commit + 7-gate CI + Chromatic baseline
- 45/45 Storybook tests passing with a11y in 'error' mode

**Archive**: [v1.3-ROADMAP.md](milestones/v1.3-ROADMAP.md) | [v1.3-REQUIREMENTS.md](milestones/v1.3-REQUIREMENTS.md)

</details>

---

## 🚧 v1.4 Viewer Identity & YouTube Enrichment

**Milestone Goal:** Give viewers global cosmetic control over how their name appears in overlays, unlock premium identity features (gradients, avatar frames/flairs, badges), and enrich YouTube chat with InnerTube-sourced real membership badge images and inline emotes.

### Phase 27: InnerTube Enrichment — Badges & Emotes
**Goal**: Extract real membership badge image URLs and inline emote images from InnerTube chat payloads and deliver them through the existing pipeline to overlays
**Depends on**: Nothing (surgical changes to existing InnerTube service + message-processor)
**Services affected**: `youtube-listener-innertube`, `message-processor`
**Requirements**: YTBADGE-01, YTBADGE-02, YTBADGE-03, YTBADGE-04, YTEMOTE-01, YTEMOTE-02, YTEMOTE-03, YTEMOTE-04, YTEMOTE-05
**Success Criteria** (what must be TRUE):
  1. `extractBadges()` in innertube parser returns badge image URLs for membership tiers (from `customThumbnail.thumbnails[1].URL`) and passes them via `tags["badge_member_url"]` and `tags["badge_member_tooltip"]`
  2. `EmojiData` struct gains `IsCustomEmoji bool` field; `extractMessageText()` emits `Emote{}` entries for custom emojis alongside the text placeholder
  3. YouTube normalizer in message-processor reads `tags["badge_member_url"]` to populate real `Badge.IconURL` (SVG fallback preserved for old listener and system badges)
  4. Emote cache per channel stored in Redis keyed by `yt:emote:{channel_id}:{emoji_id}` (TTL 24h), populated as messages arrive
  5. Unicode emoji (non-custom) continue to render as text — no regression
  6. Old quota-based youtube-listener unaffected (backward compatible)
**Plans**: 3 plans

Plans:
- [ ] 27-01-PLAN.md — Test scaffolds: failing tests for all badge and emote requirements (Wave 0, TDD)
- [ ] 27-02-PLAN.md — InnerTube parser: IsCustomEmoji, extractBadgesRich, extractMessageText emote extraction, yt_emote_cache
- [ ] 27-03-PLAN.md — Message-processor normalizer: badge_member_url handling, emote_data tag merge

### Phase 28: Viewer Identity Foundation — Auth & Platform Linking
**Goal**: Establish viewer account model, OAuth flow from browser extension, and platform identity linking so All-Chat knows which platform user corresponds to which viewer
**Depends on**: Nothing (new feature, no phase dependency)
**Services affected**: `auth-service`, `api-gateway`, browser extension (`all-chat-extension`)
**Requirements**: VID-03, VID-04, VID-05, VID-06, EXT-01, EXT-02, EXT-03, EXT-04
**Success Criteria** (what must be TRUE):
  1. `viewer_platform_identities` table exists: maps (platform, platform_user_id) → viewer_id
  2. Extension popup shows auth status (signed-in display name + avatar) and sign-in buttons for Twitch and YouTube
  3. OAuth sign-in from extension returns a viewer JWT stored in `chrome.storage.local`
  4. Extension popup has inline `<input type="color">` picker; color change saves to server immediately via PATCH `/api/viewer/cosmetics`
  5. Extension popup has "Open Settings" button navigating to `/settings/viewer` on the website
  6. `viewer_cosmetics` table exists: stores `name_color (VARCHAR(7))` per viewer
  7. Message processor `ViewerBadgeEnricher` resolves platform user → viewer_id via Redis cache (5min TTL) and injects viewer's `name_color` into `UserInfo.Color` when the platform provides none
**Plans**: 6 plans

Plans:
- [x] 28-01-PLAN.md — DB migration 035 + ViewerClaims extension + ViewerIdentityRepository + Wave 0 test scaffolds
- [x] 28-02-PLAN.md — Auth-service POST exchange handlers + PATCH cosmetics endpoint + route wiring
- [x] 28-03-PLAN.md — message-processor ViewerBadgeEnricher (Redis cache + DB fallback + wiring)
- [x] 28-04-PLAN.md — Browser extension: manifest, popup (OAuth + color picker), content script (platform detection + EXT-04)
- [x] 28-05-PLAN.md — Frontend /settings/viewer page stub
- [ ] 28-06-PLAN.md — Gap closure: content script session writes, context-aware popup buttons, color picker reset

### Phase 29: Viewer Color & Gradient Editor
**Goal**: All-authenticated-users can set a fallback name color; premium users get a full gradient editor with multi-stop color picker and live preview
**Depends on**: Phase 28
**Services affected**: `auth-service`, `message-processor`, frontend (`/settings/viewer`), overlay render component, browser extension
**Requirements**: VID-01, VID-02, PREM-01, PREM-02, WEB-01, WEB-02, WEB-05
**Success Criteria** (what must be TRUE):
  1. `/settings/viewer` page exists with "Viewer Identity" section visible to all authenticated users
  2. Color picker (hex input + color swatch) persists `name_color` server-side; overlay applies it when platform provides no color
  3. Premium users see gradient editor: 2–4 color stops, angle slider (0–360°), live preview of gradient on sample username
  4. `name_gradient` stored as JSONB `{"type":"linear","colors":["#...","#..."],"angle":90}` in `viewer_cosmetics`
  5. Overlay chat message component renders gradient name using `bg-clip-text text-transparent` with inline `backgroundImage` style — no JS animation in v1.4
  6. Non-premium users cannot access gradient controls (gated by `viewer.is_premium` flag)
**Plans**: 3 plans

Plans:
- [ ] 29-01-PLAN.md — DB migration 036 + Go type extensions (ViewerClaims IsPremium, gradient PATCH, enricher, TS types)
- [ ] 29-02-PLAN.md — Settings page: tabbed card (Solid Color + Gradient), autosave, premium gate, live preview
- [ ] 29-03-PLAN.md — Overlay + extension gradient render branch (website overlay + ChatContainer)

### Phase 30: Avatar Frame & Flair System
**Goal**: Premium viewers can select an avatar frame (decorative ring) and flair (corner icon) from an admin-curated catalog; changes render live in overlays
**Depends on**: Phase 29
**Services affected**: `api-gateway`, `overlay-manager`, frontend (`/settings/viewer`), overlay avatar component, admin pages
**Requirements**: PREM-03, PREM-04, PREM-05, WEB-03, WEB-04
**Success Criteria** (what must be TRUE):
  1. `cosmetic_frames` and `cosmetic_flairs` catalog tables exist; admin page allows adding/removing entries and marking as premium-only
  2. Premium users can browse frame and flair catalogs in `/settings/viewer` with live preview
  3. Avatar component renders: base avatar (circle) + frame PNG (centered, 1.4× size, pointer-events-none) + flair PNG (absolute bottom-right, 0.4× avatar size)
  4. `avatar_frame_id` and `avatar_flair_id` persisted in `viewer_cosmetics`; message processor injects `avatar_frame_url` and `avatar_flair_url` into `UserInfo`
  5. Non-premium viewers see catalog with items locked (visible but not selectable)
**Plans**: 4 plans

Plans:
- [x] 30-01-PLAN.md — DB migration 037 + type extensions (Go UserInfo, cosmeticsUpsertRepo, TS UserInfo in both repos)
- [ ] 30-02-PLAN.md — Auth-service: AdminCosmeticsHandler, catalog public endpoints, PATCH cosmetics extension + route wiring
- [ ] 30-03-PLAN.md — Message-processor enricher: extend viewerIdentityCache + DB join + frame/flair URL injection
- [ ] 30-04-PLAN.md — Frontend: UserAvatar component, AvatarCosmeticsCard, /admin/cosmetics page, overlay integration, extension wiring

### Phase 31: All-Chat Platform Badges
**Goal**: Admin and premium viewers receive All-Chat-specific badges that appear in all overlays, prepended before platform badges
**Depends on**: Phase 28 (requires viewer_id resolution in message processor)
**Services affected**: `message-processor`, frontend overlay component, browser extension
**Requirements**: BADGE-01, BADGE-02, BADGE-03, BADGE-04
**Success Criteria** (what must be TRUE):
  1. `badge_definitions` catalog table seeded with two entries: `"allchat"` (logo icon) and `"premium"` (gem/star icon)
  2. `ViewerBadgeEnricher` in message-processor prepends All-Chat badges to `UserInfo.Badges` for resolved viewers (derived from users.is_admin / users.is_premium — no viewer_badges table)
  3. AllChatBadge (InfinityLogo wrapper) and PremiumBadge (inline SVG gem) components render in overlay and extension
  4. Badge renders at h-[1em] responsive height with `title` attribute tooltip; allchat sorts at -2, premium at -1 in ROLE_PRIORITIES
  5. Admin grant/revoke = existing is_admin / is_premium toggles on admin users page (no new badge UI)
**Plans**: 3 plans

Plans:
- [ ] 31-01-PLAN.md — DB migration 038 + enricher extension (viewerIdentityCache, LATERAL JOIN, badge injection, unit tests)
- [ ] 31-02-PLAN.md — Frontend: AllChatBadge + PremiumBadge components, overlay page.tsx name-check render, badgeOrder.ts + tests
- [ ] 31-03-PLAN.md — Extension: mirror AllChatBadge + PremiumBadge, extend badgeOrder.ts, update ChatContainer name-check render

### Phase 32: Integration Wiring Fixes
**Goal**: Close all 7 integration-level gaps identified in v1.4 milestone audit — three surgical fixes across enricher SQL, overlay WebSocket handler, and API gateway routing
**Depends on**: Phases 27-31 (gap closure, no new features)
**Services affected**: `message-processor`, `api-gateway`, frontend (`/overlay/[id]/page.tsx`)
**Requirements**: BADGE-02, PREM-02, PREM-03, PREM-04, PREM-05, WEB-03, WEB-04
**Gap Closure:** Closes all gaps from v1.4-MILESTONE-AUDIT.md
**Success Criteria** (what must be TRUE):
  1. `viewer_badge_enricher.go` adds `LEFT JOIN viewers v ON v.id = vpi.viewer_id` and reads `COALESCE(v.is_premium, false)` — premium badge appears for premium viewers in overlays
  2. `overlay/[id]/page.tsx` `ws.onmessage` handler parses `msg.user.name_gradient` from JSON string to `NameGradient` object before calling `buildGradientCSS` — gradient usernames render without TypeError
  3. API gateway registers `GET /auth/viewer/catalog/frames` and `GET /auth/viewer/catalog/flairs` in public block; registers all 6 admin cosmetics routes (`GET/POST/DELETE /admin/cosmetics/frames` and `/admin/cosmetics/flairs`) in protected block — catalog and admin pages return 200
**Plans**: 3 plans

Plans:
- [ ] 32-01-PLAN.md — Fix enricher SQL: add viewers JOIN, update scan order, update fakeViewerDB test double (closes BADGE-02)
- [ ] 32-02-PLAN.md — Fix overlay gradient parse: JSON.parse in ws.onmessage handler (closes PREM-02)
- [ ] 32-03-PLAN.md — Add 8 proxy routes to API gateway (closes PREM-03, PREM-04, PREM-05, WEB-03, WEB-04)

## Progress

**Execution Order:**
Phases execute in numeric order: 27 → 28 → 29 → 30 → 31 → 32 (28 can start in parallel with 27)

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1-3 | v1.0 | 11/11 | Complete | 2026-02-18 |
| 4-10 | v1.1 | 21/21 | Complete | 2026-02-21 |
| 11-22 | v1.2 | 21/21 | Complete | 2026-03-06 |
| 23 Design Token System & Foundation | v1.3 | 3/3 | Complete | 2026-03-10 |
| 24 Component Library Setup | v1.3 | 5/5 | Complete | 2026-03-11 |
| 25 Page Migration & Split-view Preview | v1.3 | 8/8 | Complete | 2026-03-11 |
| 26 Enforcement & Quality Gates | v1.3 | 4/4 | Complete | 2026-03-14 |
| 27 InnerTube Enrichment — Badges & Emotes | 3/3 | Complete    | 2026-03-14 | - |
| 28 Viewer Identity Foundation — Auth & Platform Linking | 6/6 | Complete    | 2026-03-15 | - |
| 29 Viewer Color & Gradient Editor | 3/3 | Complete    | 2026-03-15 | - |
| 30 Avatar Frame & Flair System | 4/4 | Complete    | 2026-03-16 | 2026-03-16 |
| 31 All-Chat Platform Badges | 3/3 | Complete    | 2026-03-16 | - |
| 32 Integration Wiring Fixes | 3/3 | Complete    | 2026-03-16 | - |

---
*Last updated: 2026-03-16 after v1.4 milestone completion*
