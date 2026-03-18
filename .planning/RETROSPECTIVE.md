# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.3 — Chat Overlay Sharing

**Shipped:** 2026-03-11
**Phases:** 6 (14-19) | **Plans:** 23 | **Commits:** 45

### What Was Built
- New share-service microservice with PostgreSQL schema, premium enforcement middleware, admin controls, background expiry job, and WebSocket notification relay
- Share acceptance flow: DFS cycle detection, SELECT FOR UPDATE transactions, bidirectional AddSourceModal, overlay-specific message deduplication, and color-coded status badges
- Shared overlay platform type wired end-to-end (DB migration → Go model → TypeScript → overlay editor UI)
- SQL UNION fan-out in `FindOverlaysForMessage` routes messages from source overlay to all recipient overlays without duplicate enrichment
- Atomic revocation: dual-UPDATE transaction (share_requests + overlay_chat_sources), real-time WS notifications, inactive source rendering in overlay editor
- Multi-platform stream lifecycle detection with 60s debounce: Twitch EventSub stream.offline, YouTube HandleStreamOffline publish, TikTok disconnected handler; Kick gracefully disabled

### What Worked
- **3-day delivery for a complex cross-service feature** — 6 phases, 23 plans, 246 files changed. The phased approach (foundation → acceptance → sources → routing → revocation → lifecycle) kept complexity manageable
- **Nyquist Wave 0 stubs (Phase 15+)** — RED test scaffolding before implementation caught integration gaps early and created clear test coverage from the start
- **Stub-based tests without pgxmock** — Phase 17 router tests used in-process stubs (no testcontainers/pgxmock dependency), keeping the test suite fast and dependency-free
- **Feature flag via graceful disable** — Rather than blocking progress on Kick lifecycle uncertainty, the AcceptModal gracefully disables "This stream" for Kick with a clear UI message. Correct decision.
- **Redis pub/sub for lifecycle relay** — Listeners publish lifecycle:stream_end; share-service subscribes. Redis ping failure is non-fatal. Loose coupling kept listeners unaware of share-service.

### What Was Inefficient
- **SHARE-05 gap** — AddSourceModal was built (15-02) but the full flow wasn't end-to-end verified as a complete user journey. The requirement remained unchecked at milestone close.
- **STATE.md progress percent** — Progress tracking showed 93% at milestone close (stale from earlier session). The state tracking needs to be updated more frequently during execution.
- **Phase 19 plan 02 outlier** — A 525,609-second duration entry in STATE.md performance tracking was clearly a leftover from a paused session, not real execution time.

### Patterns Established
- **Service-per-concern for premium features** — share-service owns all sharing logic (search, requests, acceptance, expiry, lifecycle) as a clean separation from overlay-manager
- **ON DELETE RESTRICT + application-layer cascade** — Explicit cleanup logic preferred over CASCADE deletes for audit trail and data safety
- **Redis lifecycle relay pattern** — Listeners publish domain events (lifecycle:stream_end) to Redis; dependent services (share-service) subscribe. Zero direct coupling.
- **60s debounce for stream lifecycle expiry** — Prevents false positive expiry during Twitch stream restarts/category changes; pattern reusable for any stream-lifecycle-triggered action

### Key Lessons
1. **Premium enforcement belongs at the API middleware layer** — Client-side bypass is trivial; server validates on every mutation, not just the initial gate
2. **UNION (not UNION ALL) for fan-out deduplication** — When overlays appear in both direct and shared branches, UNION ensures exactly-once delivery
3. **Shared overlay sources are immediately active** — `shared_overlay` platform type doesn't go through listener activation; set `is_active=true` at creation time (share already accepted)
4. **Revocation bypass on WS reconnect** — `ActivateSourcesForOverlay` must filter `shared_overlay` sources where `share_request.status IN (revoked, expired)` to prevent revoked sources from reactivating on reconnect

### Cost Observations
- Model mix: ~100% sonnet (balanced profile configured)
- Sessions: ~6 sessions across 3 days
- Notable: Fine granularity setting kept individual plans small and focused; wave parallelization kept execution fast

---

## Milestone: v1.3 — Frontend Redesign

**Shipped:** 2026-03-14
**Phases:** 4 (23-26) | **Plans:** 20 | **Tasks:** 46 | **Commits:** 70

### What Was Built
- Tailwind v4 three-tier design token system (`@layer base, design-system, marketplace-themes, user-overrides`) with static platform color map — eliminates all !important from events.css
- @base-ui/react + shadcn/ui component library: Button, Card, Input, Badge, Dialog, Toast, Skeleton with CVA variants, micro-interactions, platform-color glow dots
- All 6 pages redesigned in streaming dark aesthetic (landing, dashboard, editor, settings, admin + sub-pages) — zero legacy gray-* classes
- Draggable SplitView component for live overlay preview (pointer-capture drag, 25–70% range, keyboard navigation, responsive column stacking)
- ESLint v10 flat config (`eslint.config.mjs`) + Prettier with prettier-plugin-tailwindcss — 345 pre-existing violations fixed across 43 files
- 7-gate GitHub Actions CI pipeline: ESLint, Prettier, tsc, Next.js build, bundle analysis (20KB budget), Storybook vitest (45/45 pass), Chromatic visual regression baseline

### What Worked
- **Wave parallelization across design work** — Plans 26-01 and 26-03 ran simultaneously (ESLint and bundle analysis are independent). Wave 2 (Husky) correctly blocked on Wave 1 outputs.
- **Storybook-first component development** — Building story stubs before components (24-01) forced API design clarity and caught a11y issues before page integration
- **Cascade layers over !important** — All 14 `!important` declarations in events.css removed cleanly. Marketplace themes continue to override via layer ordering, no behavioral change.
- **Static PLATFORM_COLORS map** — Enforced from day 1. Zero dynamic class construction across the codebase. Tailwind JIT scans cleanly.
- **Checkpoint for Chromatic setup** — Plan 26-04 used `autonomous: false` to pause for the human to configure the GitHub secret. The right call for an external service dependency.

### What Was Inefficient
- **Pre-existing ESLint violations** — 345 violations across 43 files needed to be fixed as part of Plan 26-01. More expensive than expected; could have enforced sooner in Phase 24.
- **A11y failures discovered at enforcement phase** — 5 Storybook a11y failures (heading-order h1→h3, color-contrast, null pathname crash) were caught in Phase 26 rather than Phase 24/25. Should have run vitest storybook earlier.
- **Prettier formatting chase** — Performance.stories.tsx needed Prettier formatting after execution. Minor but indicates the executor didn't run Prettier before committing.

### Patterns Established
- **ESLint no-restricted-syntax for design token enforcement** — Using AST-level rules to ban `bg-gray-*`, bare `focus:`, and template literals in `className`. Portable pattern for any token system.
- **`nextBundleAnalysis` budget config in package.json** — 20KB threshold on top-level and individual pages. Baseline established; CI alerts on regressions without blocking unless threshold exceeded.
- **Storybook vitest as a11y enforcement** — `a11y: { test: 'error' }` in preview.ts + `npx vitest --project storybook` in CI. Catches real bugs (null crashes, heading hierarchy, contrast ratios).
- **`text-bg` for accessible platform buttons** — Platform colors (Twitch #A37BFF, YouTube #FF4444) don't meet 4.5:1 with white text. Using `text-bg` (near-black) consistently with the Kick button pattern.

### Key Lessons
1. **ESLint v9 flat config is a breaking change** — `.eslintrc.json` is silently ignored by ESLint v10. Never rely on legacy config format in new projects; use `eslint.config.mjs` from the start.
2. **Enforce linting early, not at the gate phase** — Installing ESLint in Phase 24 but not enforcing it meant 345 violations accumulated across Phases 24-25. Pre-commit hooks should go in with the first linting config.
3. **WCAG contrast ratios for platform brand colors** — All 4 platform colors need WCAG AA verification before use in text contexts. Twitch required lightening (#9146FF → #A37BFF), YouTube/Kick need dark text.
4. **`usePathname()` returns null outside Next.js router** — Any component using `usePathname()` must guard with `?.` for Storybook compatibility (and for any non-router context).

### Cost Observations
- Model mix: ~100% sonnet (balanced profile)
- Sessions: 2 sessions across 3 days
- Notable: All 4 plans executed in a single `/gsd:execute-phase 26` session with 3 waves. Parallelization in Wave 1 saved ~30 min.

---

## Milestone: v1.6 — Listener SDK

**Shipped:** 2026-03-18
**Phases:** 6 (33-38) | **Plans:** 15 | **Commits:** 18 feat + 55 docs/test | **Timeline:** 2 days

### What Was Built
- `shared/listener` SDK (797 LOC): `ListenerBase` with 3-goroutine lifecycle (heartbeat, assignment refresh, migration subscriber), `LeadershipListener` embedding `ListenerBase` with nil-safe `SourceManagerClient`, `ShutdownCoordinator` with parallel stop + 10s HTTP drain, `ChannelManager` interface (7 methods)
- Pre-migration cleanup (Phase 33): source ID suffix normalization stripped at intake in both kick-listener and twitch-listener; `HandleMigrationEvent` canonical error signature deployed before SDK extraction
- Migration of all 6 Go listeners: twitch-listener (ListenerBase), kick-listener (ListenerBase + LeadershipListener), youtube-listener (LeadershipListener), youtube-listener-innertube (LeadershipListener), discord-listener (LeadershipListener container), twitch-eventsub-listener (ListenerBase)
- `make build-all` CI target — single command builds all listener modules, catches `replace`-directive version drift
- Compile-time `ChannelManager` assertions and goroutine leak smoke tests (`goleak.VerifyNone`) in every migrated listener

### What Worked
- **Phase 33 prerequisite cleanup first** — Behavioral inconsistencies (source ID suffix, `HandleMigrationEvent` signature) were resolved and deployed before SDK code was written. Zero mid-migration surprises.
- **Two-archetype SDK design** — Splitting into `ListenerBase` (assignment-based) and `LeadershipListener` (per-stream ownership) mapped cleanly onto all 6 listeners with no edge cases. Each `cmd/main.go` migration was mechanical once the archetype was chosen.
- **goleak-as-direct-dep pattern** — Adding `goleak` as a direct dependency before any `.go` import (Phase 35, extended to all subsequent phases) prevented `go mod tidy` stripping the dep. Established pattern eliminated friction in every subsequent migration.
- **compile-time assertion first, then migration** — Each listener had `var _ listener.ChannelManager = (*channels.Manager)(nil)` added before `cmd/main.go` migration. Build failures surfaced interface gaps immediately, before wiring complexity.
- **Incremental migration (simplest first)** — twitch → kick → (innertube + discord) → (youtube + twitch-eventsub) ordered by complexity. Each migration validated the SDK under live traffic before the next began.

### What Was Inefficient
- **twitch-eventsub Phase 38-02 ChannelManager gap** — `channels.Manager` needed `Start` re-signature and 3 new methods (`HandleMigrationEvent`, `UpdateAssignedSourceIDs`, `GetFilteredAssignmentCount`) before SDK wiring could proceed. A dedicated plan for interface gap analysis before each migration would have been cleaner.
- **discord-listener edge case: ListenerBase as container only** — Discord uses a custom shutdown sequence and can't call `base.Start`/`base.Stop`. The SDK pattern works but required a non-obvious workaround (ll used for construction only). Should document this pattern explicitly for future listeners.
- **STATE.md progress stale at 0%** — The GSD tool set `percent: 0` on the v1.6 state despite all 15 plans completing. Percent tracking not updated by plan execution in this milestone.

### Patterns Established
- **Pre-migration cleanup phase** — Fix behavioral inconsistencies (API contracts, naming conventions) in a dedicated phase before any SDK extraction begins. Prevents mid-migration surprises.
- **goleak-as-direct-dep before any .go import** — Use `go mod edit -require + go mod download` to pin `goleak` as a direct dep before the first smoke test `.go` file imports it; avoids `go mod tidy` stripping it as unused.
- **Compile-time assertion before migration** — Add `var _ Interface = (*Impl)(nil)` in the first plan of each migration phase; build failures surface all interface gaps before wiring complexity begins.
- **Leadership-only container pattern** — For services with custom shutdown sequences, `LeadershipListener` can be used as a construction container (env-based wiring) without calling `Start`/`Stop`.

### Key Lessons
1. **Interface gap analysis belongs in Phase 0** — Before migrating a service to a new interface, explicitly audit all required methods. `twitch-eventsub` needed 3 new methods that weren't caught until Plan 38-02.
2. **`make build-all` catches cross-module drift that single-module tests miss** — Replace directives can diverge silently; the CI target makes this visible immediately.
3. **Two archetypes is the right level of abstraction** — Not one (too restrictive for leadership-only services) and not five (too complex). The boundary between `ListenerBase` and `LeadershipListener` is stable.
4. **Strip platform suffix at intake, not inside Manager** — The `channels.Manager` interface stays simple; the caller in `cmd/main.go` normalizes IDs before passing them in. Easier to test, easier to read.

### Cost Observations
- Model mix: ~100% sonnet (balanced profile)
- Sessions: ~3 sessions across 2 days
- Notable: Fine granularity + wave parallelization kept plans small (2-3 tasks each); 2-day delivery for 6 phases across 6 services

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Phases | Plans | Days | Key Change |
|-----------|--------|-------|------|------------|
| v1.0 | 3 | 11 | 1 | Initial deletion support |
| v1.1 | 4 | 21 | 3 | Load balancing infrastructure |
| v1.2 | 5 | 21 | 13 | InnerTube + production rollout |
| v1.3 (Sharing) | 6 | 23 | 3 | Cross-service feature with new microservice |
| v1.3 (Frontend) | 4 | 20 | 3 | Full frontend redesign + design system |
| v1.5 (Discord) | 6 | 16 | 2 | New platform listener + bidirectional relay |
| v1.6 (SDK) | 6 | 15 | 2 | SDK extraction + full listener migration |

### Cumulative Quality

| Milestone | New Tests | Pattern |
|-----------|-----------|---------|
| v1.0 | 88.5% deletion coverage | Integration tests for pipeline |
| v1.1 | 16 tracing spans | Observability-first approach |
| v1.2 | 69 automated tests | Contract validation before prod |
| v1.3 (Sharing) | Nyquist RED stubs + modal TDD | Test scaffolding before implementation |
| v1.3 (Frontend) | 45 Storybook vitest tests | a11y 'error' mode as CI gate |
| v1.5 (Discord) | Bot auth + relay integration | End-to-end Discord flow testing |
| v1.6 (SDK) | goleak smoke tests in all 6 listeners | Goroutine lifecycle correctness at SDK boundary |

### Top Lessons (Verified Across Milestones)

1. **Drop-in replacement beats migration** — v1.2 InnerTube succeeded because schema compatibility was a hard constraint from day one. Applying this to v1.3: `shared_overlay` is a virtual platform type, not a new routing tier.
2. **Nyquist Wave 0 RED stubs prevent integration gaps** — First introduced in v1.2, proven essential in v1.3 for catching missing test coverage before implementation begins.
3. **Loose coupling via Redis pub/sub scales across milestones** — Used in v1.1 (load balancing), v1.2 (InnerTube integration), and v1.3 (lifecycle events). The pattern is battle-tested.
