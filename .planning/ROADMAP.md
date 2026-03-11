# Roadmap: All-Chat

## Milestones

- ✅ **v1.0 Message Deletion Support** — Phases 1-3 (partial, shipped 2026-02-18)
- ✅ **v1.1 Listener Load Balancing** — Phases 4-10 (shipped 2026-02-21)
- ✅ **v1.2 InnerTube YouTube Listener** — Phases 11-22 (shipped 2026-03-06)
- 🚧 **v1.3 Frontend Redesign** — Phases 23-26 (in progress)

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
- [ ] **Phase 25: Page Migration & Split-view Preview** - Redesign all pages with new design system plus live preview feature
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

## Progress

**Execution Order:**
Phases execute in numeric order: 23 → 24 → 25 → 26

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1-3 | v1.0 | 11/11 | Complete | 2026-02-18 |
| 4-10 | v1.1 | 21/21 | Complete | 2026-02-21 |
| 11-22 | v1.2 | 21/21 | Complete | 2026-03-06 |
| 23. Design Token System & Foundation | 3/3 | Complete    | 2026-03-10 | - |
| 24. Component Library Setup | 5/5 | Complete   | 2026-03-11 | - |
| 25. Page Migration & Split-view Preview | 3/8 | In Progress|  | - |
| 26. Enforcement & Quality Gates | v1.3 | 0/? | Not started | - |

---
*Last updated: 2026-03-09 after v1.3 roadmap creation*
