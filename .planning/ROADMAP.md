# Roadmap: All-Chat

## Milestones

- ✅ **v1.0 Message Deletion Support** — Phases 1-3 (partial, shipped 2026-02-18)
- ✅ **v1.1 Listener Load Balancing** — Phases 4-10 (shipped 2026-02-21)
- ✅ **v1.2 InnerTube YouTube Listener** — Phases 11-22 (shipped 2026-03-06)
- ✅ **v1.3 Frontend Redesign** — Phases 23-26 (shipped 2026-03-14)
- ✅ **v1.5 Discord Listener** — Phases 27-32 (shipped 2026-03-16)
- 🚧 **v1.6 Visual Overlay Customizer** — Phases 33-37 (in progress)

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

<details>
<summary>✅ v1.5 Discord Listener (Phases 27-32) — SHIPPED 2026-03-16</summary>

**Milestone Goal:** Add Discord as a bidirectional chat source — read from Discord channels into overlays, and relay overlay messages back to Discord with loop-safe filtering, full OAuth2 bot authorization UX, and production-grade load balancing.

**Delivered:**
- Discord Gateway client with OAuth2 "Add to Server" bot authorization, permission validation, and guild disconnect
- Inbound ingestion: Discord channel messages normalized to unified RawChatMessage schema, delivered to overlays as first-class source
- @user/#channel mention resolution via guild cache (Redis-backed)
- Deletion event propagation through existing deletion pipeline (MESSAGE_DELETE + MESSAGE_DELETE_BULK)
- Outbound relay to Discord REST API with loop-safe filter, single-retry on 5xx
- Per-source relay toggle and configurable outbound channel
- Gateway shard ownership via source-manager leader election; session state persisted in Redis for pod-restart resume
- HPA scaling on Prometheus metrics; discord-listener Deployment + Service + HPA Kubernetes manifests
- Full setup UI: Settings Discord server card, overlay editor guild/channel picker, relay config panel, source card status indicators

**Archive**: [v1.5-REQUIREMENTS.md](milestones/v1.5-REQUIREMENTS.md)

</details>

### 🚧 v1.6 Visual Overlay Customizer (In Progress)

**Milestone Goal:** Enable non-technical users to visually customize their overlay appearance through ~50 CSS property controls organized into 7 categories, with live preview, marketplace theme pre-population, and a cascade layer that preserves raw CSS override power.

## Phase Details

### Phase 27: Auth and Bot Token Foundation
**Goal**: Users can authorize the Discord bot into their server and the service has a stable Gateway connection with correct session management
**Depends on**: Nothing (first phase of v1.5)
**Requirements**: AUTH-01, AUTH-02, AUTH-03, AUTH-04
**Success Criteria** (what must be TRUE):
  1. User can click "Add to Server" and complete the OAuth2 flow, resulting in the bot appearing in their Discord server
  2. After connecting, user sees the list of text channels available in their server (API response from auth-service)
  3. User receives a clear, human-readable error when bot is missing required permissions (VIEW_CHANNEL, READ_MESSAGE_HISTORY, SEND_MESSAGES)
  4. User can disconnect the bot, which removes it from their account and deletes all associated Discord sources
  5. discord-listener establishes a Gateway WebSocket connection with correct intents bitmask and a startup assertion confirms MESSAGE_CONTENT is non-empty on first READY event
**Plans**: 4 plans

Plans:
- [ ] 27-01-PLAN.md — DB migration (discord_guilds) + overlay-manager platform registration
- [ ] 27-02-PLAN.md — discord-listener service scaffold + Gateway WebSocket connection
- [ ] 27-03-PLAN.md — Discord OAuth provider + DiscordRepository in auth-service
- [ ] 27-04-PLAN.md — Discord HTTP handlers + route wiring in auth-service

### Phase 28: Inbound Listener Core
**Goal**: Discord channel messages appear in overlays as a real-time, first-class chat source
**Depends on**: Phase 27
**Requirements**: INBD-01, INBD-02
**Success Criteria** (what must be TRUE):
  1. A message sent in a configured Discord channel appears in the overlay within one second
  2. Discord messages display with platform label "discord" and author username, consistent with Twitch and YouTube messages in the overlay
  3. Bot messages (author.bot == true) are silently filtered and never appear in overlays
  4. Only messages from the configured inbound channel appear — messages from other channels in the same server are ignored
**Plans**: 2 plans

Plans:
- [ ] 28-01-PLAN.md — MESSAGE_CREATE dispatch + ChannelRegistry + publisher package + overlay-manager Redis wiring (INBD-01)
- [ ] 28-02-PLAN.md — DiscordNormalizer in message-processor + registration (INBD-02)

### Phase 29: Inbound Enrichment
**Goal**: Discord messages carry deletion events and resolved mention text through the existing platform pipelines
**Depends on**: Phase 28
**Requirements**: INBD-03, INBD-04
**Success Criteria** (what must be TRUE):
  1. When a Discord message is deleted, a deletion event propagates through the pipeline and the message disappears from active overlays (consistent with Twitch/YouTube deletion behavior)
  2. A message containing @username or #channel mentions renders with resolved names (e.g., "@alice" not "@123456789012345678") in the overlay
**Plans**: 2 plans

Plans:
- [ ] 29-01-PLAN.md — MESSAGE_DELETE/MESSAGE_DELETE_BULK dispatch + HandleMessageDelete + channel filter (INBD-03)
- [ ] 29-02-PLAN.md — GuildCache interface + GUILD_CREATE/CHANNEL_*/ROLE_* handlers + mention resolution in HandleMessageCreate (INBD-04)

### Phase 30: Outbound Relay
**Goal**: Non-Discord overlay messages are posted to a user-configured Discord channel with no echo loops
**Depends on**: Phase 28
**Requirements**: RELY-01, RELY-02, RELY-03, RELY-04
**Success Criteria** (what must be TRUE):
  1. A Twitch message that appears in an overlay is relayed to the configured Discord channel within two seconds, formatted as "[emoji] username: text"
  2. A message that originated from Discord is never posted back to Discord — verified by a test asserting no REST call is made for platform="discord" pub/sub messages
  3. When relay_enabled is set to false for a source, no messages are relayed for that source even if other sources in the same overlay have relay active
  4. The relay target channel can be configured independently from the inbound channel (same or different channel ID accepted on save)
**Plans**: 2 plans

Plans:
- [ ] 30-01-PLAN.md — relay package TDD: Manager, Repository, DiscordPoster interface + httpPoster, loop-safety filter, format function (RELY-01 to RELY-04)
- [ ] 30-02-PLAN.md — pgx/v5 dependency + relay.Manager wiring in cmd/main.go

### Phase 31: Load Balancing
**Goal**: discord-listener runs safely across multiple pods with deterministic shard ownership and auto-scales under load
**Depends on**: Phase 30
**Requirements**: LOAD-01, LOAD-02, LOAD-03
**Success Criteria** (what must be TRUE):
  1. When two discord-listener pods are running, exactly one pod holds the Gateway connection for each shard — the second pod does not connect until the first fails or releases ownership
  2. After a pod restart, the Gateway session resumes (RESUME opcode) using the persisted session_id and resume_gateway_url from Redis, avoiding a full re-IDENTIFY
  3. HPA scales discord-listener pods up when events/sec or active guild count exceeds configured thresholds, and the new pod acquires shard ownership within 60 seconds of the prior pod terminating
**Plans**: 3 plans

Plans:
- [ ] 31-01-PLAN.md — Gateway RESUME protocol TDD: OpResume types, BuildResumePayload, IDENTIFY/RESUME branch in Connect(), InvalidSession clear logic (LOAD-03)
- [ ] 31-02-PLAN.md — metrics package + go.mod shared deps + cmd/main.go ownership gating via LeadershipCoordinator + /metrics endpoint (LOAD-01, LOAD-02)
- [ ] 31-03-PLAN.md — Kubernetes manifests: Deployment+Service, HPA, ServiceMonitor + kustomization.yaml update (LOAD-02)

### Phase 32: Setup UI
**Goal**: Users can configure Discord sources end-to-end from the frontend without leaving the app
**Depends on**: Phase 31
**Requirements**: UI-01, UI-02, UI-03, UI-04
**Success Criteria** (what must be TRUE):
  1. Settings page shows a Discord server connect card — clicking it initiates the OAuth2 redirect; after completion the card updates to show the connected server name and icon
  2. Overlay editor allows adding a Discord source by selecting a guild and choosing an inbound channel from a dropdown populated from the channel listing API
  3. Each Discord source card in the overlay editor shows connection status and a visual indicator of whether relay is active or inactive
  4. Per-source relay configuration panel lets the user toggle relay on/off and pick an outbound channel; the visual filter indicator updates immediately on toggle
**Plans**: 3 plans

Plans:
- [ ] 32-01-PLAN.md — Backend PATCH endpoint + type system + discord.ts API module + design tokens + test scaffolds
- [ ] 32-02-PLAN.md — Settings page Discord server connect card (OAuth, guild list, disconnect)
- [ ] 32-03-PLAN.md — Overlay editor: Discord SourceCard + relay panel + AddSourceForm 2-step dialog

### Phase 33: CSS Architecture Foundation

**Goal:** Establish the technical foundation: new cascade layer, backend `visual_settings` support, TypeScript types, and the CSS generator utility.

**Depends on:** Nothing (first phase of v1.6)

**Requirements:** VISM-01, VISM-03

**Success Criteria** (what must be TRUE):
1. `visual-customizer` cascade layer added to overlay preview embed page CSS, positioned above `marketplace-themes` and below `user-css-overrides`
2. `visual_settings JSONB` column added to `overlay_configs` table via database migration
3. `VisualSettings` TypeScript type defined in `frontend/src/lib/types/visual-settings.ts` with all ~50 CSS property fields, all optional
4. `visual-settings-to-css.ts` utility converts `VisualSettings` object → `@layer visual-customizer { :root { --chat-*: value } }` CSS string
5. `overlay_configs` Go model updated with `VisualSettings` field; API round-trips `visual_settings` JSON unchanged
6. Unit tests for `visual-settings-to-css.ts`: empty input → empty string, partial input → only set properties, full input → all properties present

**Plans:** 3/3 plans complete

Plans:
- [x] 33-01-PLAN.md — DB migration: `visual_settings JSONB DEFAULT '{}'` column + Go model + API passthrough
- [x] 33-02-PLAN.md — TypeScript `VisualSettings` type + `visual-settings-to-css.ts` generator utility + unit tests
- [x] 33-03-PLAN.md — Inject `visual-customizer` cascade layer into overlay embed preview page

### Phase 34: Appearance Controls — Core

**Goal:** Implement the highest-impact visual controls: Typography, Colors, and Background & Bubbles groups.

**Depends on:** Phase 33

**Requirements:** APPR-01, APPR-02, APPR-03, APPR-04, APPR-08

**Success Criteria** (what must be TRUE):
1. `TypographyGroup` component renders font family picker (curated list), weight select, line height slider, letter spacing slider
2. `ColorsGroup` component renders color pickers for message body text, username text, timestamp text
3. `BackgroundGroup` component renders overlay background color + opacity, bubble background color + opacity, border radius/width/color, padding, gap, backdrop blur sliders
4. All controls are wired: changing a value calls `setVisualSettings` → `visual-settings-to-css` → injected `<style>` in preview iframe → live update
5. `AppearancePanel` hosts all three groups with a collapsible sub-group wrapper for each
6. `CollapsibleSection` reusable wrapper component exists and is used by all groups

**Plans:** 3/3 plans complete

Plans:
- [ ] 34-01-PLAN.md — VisualSettings type extension + test scaffolds + CollapsibleSection + TypographyGroup
- [ ] 34-02-PLAN.md — Live preview wiring (SplitView + embed page + editor state + react-colorful install)
- [ ] 34-03-PLAN.md — ColorPickerControl + ColorsGroup + BackgroundGroup + AppearancePanel mount

### Phase 35: Appearance Controls — Extended

**Goal:** Add Visibility toggles, Sizing controls, and Platform Color overrides.

**Depends on:** Phase 34

**Requirements:** APPR-05, APPR-06, APPR-07

**Success Criteria** (what must be TRUE):
1. `VisibilityGroup` renders a toggle for each component: avatars, badges, timestamps, platform badge, emotes, username — toggling hides/shows via `--chat-show-*: none/inline` CSS custom properties
2. `SizingGroup` renders sliders for avatar size (px), badge size (px), emote scale (multiplier)
3. `PlatformColorsGroup` renders a color picker per platform (Twitch, YouTube, Kick, TikTok, Discord) overriding `--platform-*-accent` CSS custom properties
4. All three groups wired to live preview, following the same pattern as Phase 34 groups
5. All three groups appear inside `AppearancePanel` with collapsible wrappers

**Plans:** 3 plans

Plans:
- [ ] 35-01-PLAN.md — ToggleSwitch primitive + VisibilityGroup (6 show/hide toggles) + iframe defaults wiring
- [ ] 35-02-PLAN.md — SizingGroup (avatar 16-64px, badge 12-32px, emote scale 0.5-3.0×)
- [ ] 35-03-PLAN.md — PlatformColorsGroup (5 accent colors + reset) + AppearancePanel wiring

### Phase 36: Events Styling + Theme Import

**Goal:** Add special events styling controls, implement theme CSS parser, and wire theme loading to pre-populate all visual controls.

**Depends on:** Phase 35

**Requirements:** APPR-09, APPR-10, VISM-02, VISM-04

**Success Criteria** (what must be TRUE):
1. `EventsGroup` renders controls: show/hide toggles for Super Chat, subscriptions, raids; size modifier slider for each event type
2. `theme-css-parser.ts` utility parses a CSS string and returns a `Partial<VisualSettings>` by mapping `--chat-*` custom property values to VisualSettings fields
3. When user loads a marketplace theme, the theme's CSS file is fetched and parsed; parsed values are merged into `visualSettings` state — all controls update
4. "Reset to theme defaults" button clears `visualSettings` back to parsed-theme values (or empty if no theme loaded), restoring theme appearance
5. Unit tests for `theme-css-parser.ts`: full theme CSS → all matched properties extracted, unknown properties ignored
6. All visual control changes update live preview without save (APPR-10 fully verified end-to-end)

**Plans:** 3 plans

Plans:
- [ ] 36-01-PLAN.md — `EventsGroup` (Super Chat / subscription / raid show/hide + size modifier)
- [ ] 36-02-PLAN.md — `theme-css-parser.ts` + unit tests
- [ ] 36-03-PLAN.md — Theme load → parse → pre-populate controls + "Reset to defaults" button

### Phase 37: Editor UX Rework

**Goal:** Restructure the overlay editor panel: marketplace-first layout, collapsible sections, CSS editor moved to "Expert" section.

**Depends on:** Phase 36

**Requirements:** EDUX-01, EDUX-02, EDUX-03, EDUX-04

**Success Criteria** (what must be TRUE):
1. Theme Marketplace section is the first visible element in the editor panel (above all other sections)
2. Editor panel renders collapsible sections in order: Theme → Appearance → Sources → Behavior → Expert
3. `AppearancePanel` is inside the "Appearance" collapsible with 7 visible sub-group headers: Typography, Colors, Background & Bubbles, Visibility, Sizing, Platform Colors, Events
4. CSS editor is inside the "Expert" collapsible section, collapsed by default
5. All sections open/close independently with smooth animation; state preserved across re-renders
6. Existing functionality (sources, behavior settings, CSS editor) works unchanged — only relocated
7. Visual settings save correctly when "Save" is clicked; reload restores all control values

**Plans:** 3 plans

Plans:
- [ ] 37-01-PLAN.md — `CollapsibleSection` integration into editor page + section order scaffolding
- [ ] 37-02-PLAN.md — Wire AppearancePanel into Appearance section; move CSS editor to Expert section
- [ ] 37-03-PLAN.md — Save/load round-trip verification + end-to-end UAT checkpoint

## Progress

**Execution Order:** 27 → 28 → 29 → 30 → 31 → 32 → 33 → 34 → 35 → 36 → 37

| Phases | Milestone | Plans Complete | Status | Completed |
|--------|-----------|----------------|--------|-----------|
| 1-3 | v1.0 | 11/11 | Complete | 2026-02-18 |
| 4-10 | v1.1 | 21/21 | Complete | 2026-02-21 |
| 11-22 | v1.2 | 21/21 | Complete | 2026-03-06 |
| 23-26 | v1.3 | 20/20 | Complete | 2026-03-14 |
| 27-32 | v1.5 | 15/15 | Complete | 2026-03-16 |
| 33 | CSS Architecture Foundation | Complete    | 2026-03-18 | — |
| 34 | 3/3 | Complete    | 2026-03-18 | — |
| 35 | Appearance Controls — Extended | 0/3 | Not started | — |
| 36 | Events Styling + Theme Import | 0/3 | Not started | — |
| 37 | Editor UX Rework | 0/3 | Not started | — |

---
*Last updated: 2026-03-18 after v1.6 milestone start — phases 33-37 added*
