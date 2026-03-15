# Roadmap: All-Chat

## Milestones

- ✅ **v1.0 Message Deletion Support** — Phases 1-3 (partial, shipped 2026-02-18)
- ✅ **v1.1 Listener Load Balancing** — Phases 4-10 (shipped 2026-02-21)
- ✅ **v1.2 InnerTube YouTube Listener** — Phases 11-22 (shipped 2026-03-06)
- ✅ **v1.3 Overlay Sharing + Frontend Redesign** — Phases 23-26 (shipped 2026-03-11)
- 🚧 **v1.4 Viewer Identity & YouTube Enrichment** — Phases 27-31 (in progress)

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

### 🚧 v1.3 Frontend Redesign (In Progress)

**Milestone Goal:** Transform frontend from generic Tailwind defaults to professional streaming-focused design system with comprehensive component library and enforceable style rules.

- [x] **Phase 23: Design Token System & Foundation** - Establish design tokens, Tailwind v4 configuration, and overlay CSS stability contract (completed 2026-03-10)
- [x] **Phase 24: Component Library Setup** - shadcn/ui integration with custom theming and performance budgets (completed 2026-03-10)
- [x] **Phase 25: Page Migration & Split-view Preview** - Redesign all pages with new design system plus live preview feature (completed 2026-03-11)
- [ ] **Phase 26: Enforcement & Quality Gates** - ESLint rules, pre-commit hooks, CI/CD quality gates, and marketplace migration guide

## Phase Details

### Phase 23: Design Token System & Foundation
**Goal**: Design token system established as foundation for consistent styling across all UI
**Depends on**: Nothing (first phase of v1.3)
**Requirements**: FOUND-01, FOUND-02, FOUND-03, FOUND-04, FOUND-05, FOUND-06
**Success Criteria** (what must be TRUE):
  1. Design tokens defined using Tailwind v4 @theme directive with three-layer hierarchy (base → semantic → component)
  2. Platform colors accessible via static mapping object (no dynamic class construction breaking JIT compilation)
  3. Overlay marketplace CSS classes documented as stable public API (events.css stability contract exists)
  4. Tailwind v4 gradient classes migrated (bg-gradient-to-* → bg-linear-to-*) with visual regression validation
  5. CSS cascade layers defined (@layer base, design-system, marketplace-themes, user-overrides) preventing specificity conflicts
**Plans**: 3 plans

Plans:
- [x] 23-01-PLAN.md — globals.css clean-slate rewrite: design tokens (three-tier @theme), cascade layer order, Barlow + DM Mono fonts
- [x] 23-02-PLAN.md — Static platform color map + gradient migration (bg-gradient-to-* → bg-linear-to-*)
- [x] 23-03-PLAN.md — events.css cascade layer migration + EVENTS_CSS_API.md stability contract

### Phase 24: Component Library Setup & Customization
**Goal**: @base-ui/react + shadcn CLI component library integrated, customized with design system tokens, and documented in Storybook
**Depends on**: Phase 23
**Tooling**: @base-ui/react (primitives), shadcn CLI (scaffolding), CVA (variants), Storybook 10 (docs + a11y testing)
**Requirements**: COMP-01, COMP-02, COMP-03, COMP-04, COMP-05, COMP-06, COMP-07, COMP-08, COMP-09
**Success Criteria** (what must be TRUE):
  1. Core components built on @base-ui/react primitives, themed with slate design tokens (Button already done — Card, Input, Badge, Dialog, Toast remaining)
  2. Component variants implemented using CVA for consistent pattern application
  3. Smooth micro-interactions added (hover scale, shadow transitions) and verified visually in Storybook
  4. Platform-color coded components created for multi-platform UI elements (badges, borders, status indicators)
  5. Storybook story exists for every component, a11y addon shows zero violations in 'error' mode
  6. Performance budget established and monitored (<16ms message render time, <100KB bundle increase)
  7. All !important declarations removed from events.css and replaced with CSS cascade layer architecture
**Plans**: 5 plans

Plans:
- [ ] 24-01-PLAN.md — Storybook infrastructure: globals.css import in preview.ts, story stubs for all six components
- [ ] 24-02-PLAN.md — Card (micro-interactions), Input, Skeleton components + Button gradient variant
- [ ] 24-03-PLAN.md — PlatformBadge component with glow dot + COMP-09 verification
- [ ] 24-04-PLAN.md — Dialog (frosted backdrop) + Toast system + wire ToastProvider into root layout
- [ ] 24-05-PLAN.md — A11y error mode gate, performance budget measurement, human visual verify

### Phase 25: Page Migration & Split-view Preview
**Goal**: All application pages redesigned with new design system, plus split-view live preview feature implemented
**Depends on**: Phase 24
**Tooling**: `make frontend-dev` (minimal backend for fast iteration), Storybook a11y addon (accessibility validation), Chromatic (visual regression)
**Requirements**: PAGE-01, PAGE-02, PAGE-03, PAGE-04, PAGE-05, PAGE-06, PAGE-07, PAGE-08, PAGE-09, PAGE-10, FEAT-01, FEAT-02, FEAT-03, FEAT-04
**Success Criteria** (what must be TRUE):
  1. Landing page redesigned with gradient hero, platform login buttons, and feature cards
  2. Dashboard redesigned with overlay grid, hover states, empty states, and creation workflows
  3. Overlay editor redesigned with platform-color coded source management cards
  4. Split-view layout implemented (editor configuration side-by-side with live preview, responsive stacking on mobile)
  5. Settings and admin pages redesigned for visual consistency across all authenticated pages
  6. Responsive layouts validated across all breakpoints (375px mobile, 768px tablet, 1920px desktop)
  7. WCAG 2.1 AA accessibility compliance achieved (keyboard navigation, focus states, Storybook a11y addon passing in 'error' mode)
  8. Loading states and empty states implemented with illustrations and clear CTAs
  9. Overlay preview CSS preserved unchanged (marketplace theme compatibility maintained)
**Plans**: 8 plans

Plans:
- [ ] 25-01-PLAN.md — Shared infrastructure: AppNav component + logo-ring + Storybook story stubs (Wave 0)
- [ ] 25-02-PLAN.md — Landing page redesign: magnetic glow hero, platform login buttons, feature grid
- [ ] 25-03-PLAN.md — Dashboard redesign: overlay grid, skeleton loading, empty state, Dialog delete
- [ ] 25-04-PLAN.md — New overlay form + settings page migration
- [ ] 25-05-PLAN.md — Overlay editor redesign + SplitView component (FEAT-01 to FEAT-04)
- [ ] 25-06-PLAN.md — Admin layout + dashboard dark theme conversion
- [ ] 25-07-PLAN.md — Admin sub-pages dark theme migration (users, overlays, sources, viewers)
- [ ] 25-08-PLAN.md — Automated quality gates + human visual verification checkpoint

### Phase 26: Enforcement & Quality Gates
**Goal**: Design system compliance automated through tooling, preventing regression and ensuring marketplace compatibility
**Depends on**: Phase 25
**Tooling**: ESLint + Prettier (already in package.json), Husky (pre-commit), Chromatic (visual regression — already installed via @chromatic-com/storybook), Storybook a11y addon set to 'error' mode (already installed)
**Requirements**: ENFORCE-01, ENFORCE-02, ENFORCE-03, ENFORCE-04, ENFORCE-05, ENFORCE-06, ENFORCE-07, ENFORCE-08, ENFORCE-09, ENFORCE-10
**Success Criteria** (what must be TRUE):
  1. ESLint plugin for Tailwind configured with design system rules (no gray-*, focus-visible required, no string concat in className)
  2. Prettier plugin installed and configured for consistent Tailwind class ordering
  3. Pre-commit hooks configured with Husky (lint + format on changed files, blocking violations)
  4. CI/CD quality gates block PRs with ESLint errors or bundle size increases >20KB without justification
  5. Chromatic visual regression configured in CI (Storybook stories snapshot all components, PRs fail on unreviewed changes)
  6. Marketplace CSS migration guide created documenting class name changes and providing upgrade path
  7. Performance monitoring configured validating message render time <16ms at 20 msg/sec load
  8. Storybook a11y addon set to 'error' mode (currently 'todo') — CI fails on a11y violations
  9. Bundle size baseline established (Next.js built-in bundle analyzer)
**Plans**: TBD

Plans:
- [ ] 26-01: TBD

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
**Services affected**: `message-processor`, `api-gateway`, frontend overlay component, admin pages
**Requirements**: BADGE-01, BADGE-02, BADGE-03, BADGE-04
**Success Criteria** (what must be TRUE):
  1. `badge_definitions` catalog table seeded with two entries: `"allchat"` (logo icon) and `"premium"` (gem/star icon), with 1× and 2× CDN URLs
  2. `viewer_badges` table assigns badge types to viewer IDs; system auto-assigns `"premium"` badge to viewers with `is_premium=true` and `"allchat"` badge to admins
  3. `ViewerBadgeEnricher` in message-processor prepends All-Chat badges to `UserInfo.Badges` for resolved viewers
  4. Badge renders in overlay and extension at 18px height (matching `h-[1em]` pattern) with `title` attribute tooltip
  5. Admin can manually grant or revoke badges from the admin users page
**Plans**: TBD (target ~3 plans)

## Progress

**Execution Order:**
Phases execute in numeric order: 27 → 28 → 29 → 30 → 31 (28 can start in parallel with 27)

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1-3 | v1.0 | 11/11 | Complete | 2026-02-18 |
| 4-10 | v1.1 | 21/21 | Complete | 2026-02-21 |
| 11-22 | v1.2 | 21/21 | Complete | 2026-03-06 |
| 23 Design Token System & Foundation | v1.3 | 3/3 | Complete | 2026-03-10 |
| 24 Component Library Setup | v1.3 | 5/5 | Complete | 2026-03-11 |
| 25 Page Migration & Split-view Preview | v1.3 | 8/8 | Complete | 2026-03-11 |
| 26 Enforcement & Quality Gates | v1.3 | 0/? | Not started | - |
| 27 InnerTube Enrichment — Badges & Emotes | 3/3 | Complete    | 2026-03-14 | - |
| 28 Viewer Identity Foundation — Auth & Platform Linking | 6/6 | Complete    | 2026-03-15 | - |
| 29 Viewer Color & Gradient Editor | 3/3 | Complete    | 2026-03-15 | - |
| 30 Avatar Frame & Flair System | v1.4 | 1/4 | In progress | 2026-03-16 |
| 31 All-Chat Platform Badges | v1.4 | 0/? | Not started | - |

---
*Last updated: 2026-03-15 after Phase 29 planning*
