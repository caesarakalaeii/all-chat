# Pitfalls Research

**Domain:** Frontend Design System Integration — Adding shadcn/ui and comprehensive design tokens to existing All-Chat streaming overlay platform
**Researched:** 2026-03-09
**Confidence:** HIGH

## Critical Pitfalls

### Pitfall 1: Breaking Overlay Marketplace CSS Themes

**What goes wrong:**
Marketplace theme creators have built custom CSS targeting existing class names (`event-message`, `event-tier-high`, `event-icon`, etc.). When design system migration changes or removes these class names, all marketplace themes break simultaneously, creating support chaos and user frustration.

**Why it happens:**
- Tailwind v4 renames gradient classes (`bg-gradient-to-r` → `bg-linear-to-r`) which are used in `events.css` line 111
- shadcn/ui components use different base class structures than current implementation
- Design tokens replace hardcoded values but marketplace themes depend on those exact values
- Developers focus on app UI migration and forget overlay CSS is a public API

**How to avoid:**
1. **Class Name Stability Contract**: Treat all classes in `frontend/src/styles/events.css` as immutable public API
2. **Parallel Implementation**: Create new design system classes alongside old ones, never rename/remove existing
3. **Migration Guide First**: Write marketplace CSS migration guide BEFORE any code changes
4. **Deprecation Timeline**: If class changes are unavoidable, require 6+ month deprecation notice with automated migration tools
5. **Prefix Isolation**: Use `ac-` prefix for all new design system classes to avoid conflicts (e.g., `ac-button`, `ac-card`)

**Warning signs:**
- Any `git diff` showing deletions in `events.css`
- Tailwind v4 upgrade without codemod review
- shadcn/ui components added without checking for `.event-*` class conflicts
- Bundle size analysis showing removed CSS that marketplace themes depend on
- Missing entries in "Breaking Changes" section of release notes

**Phase to address:**
- **Phase 1 (Design Tokens):** Document all public CSS classes as stable API
- **Phase 2 (Component Library):** Implement prefix isolation strategy
- **Phase 3 (Page Migration):** Validate no overlay CSS classes changed
- **Phase 4 (Enforcement):** Add pre-commit hook to prevent `events.css` modifications

---

### Pitfall 2: Tailwind v4 Gradient Class Renames Breaking Production

**What goes wrong:**
Tailwind v4 renames `bg-gradient-to-*` → `bg-linear-to-*` across ALL gradient utilities. Your design system spec uses `bg-gradient-to-r from-purple-500 to-blue-500` extensively (Button component line 180, Card component line 111 in events.css). After Tailwind v4 migration, all gradients disappear or render as solid colors, breaking visual hierarchy.

**Why it happens:**
- Tailwind v4 removes all class aliases that don't match underlying CSS properties
- Automated upgrade tool (`npx @tailwindcss/upgrade`) handles 90% of cases but misses dynamic class construction
- `events.css` has CSS-in-JS patterns the codemod cannot detect
- Testing focuses on functionality, not visual regression

**How to avoid:**
1. **Pre-Migration Audit**: Grep entire codebase for `bg-gradient-to-`, `flex-shrink-`, `overflow-ellipsis` (all renamed in v4)
2. **Run Codemod First**: Execute `npx @tailwindcss/upgrade` before any manual changes
3. **Visual Regression Tests**: Capture screenshots of all overlay event types before migration, diff after
4. **Dynamic Class Review**: Manually search for string concatenation patterns like `'bg-gradient-to-' + direction`
5. **CSS-in-CSS Migration**: Move all Tailwind classes from CSS files to React components before v4 upgrade (codemods work better in JSX)

**Warning signs:**
- Tailwind v4 appears in `package.json` without corresponding `CHANGELOG.md` entry documenting breaking changes
- Gradient buttons rendering as solid colors in dev environment
- Event tier backgrounds (`event-tier-high` line 40-48) losing gradient effects
- User reports of "flat" or "missing" visual effects after deployment
- Bundle size decrease from removed gradient classes

**Phase to address:**
- **Phase 1 (Design Tokens):** Identify all gradient usage, plan v4 migration
- **Phase 2 (Component Library):** Refactor `events.css` gradients to CSS variables
- **Pre-Phase 3:** Run Tailwind v4 upgrade with full visual regression suite
- **Phase 3 (Page Migration):** Validate all gradients render correctly across browsers

---

### Pitfall 3: Real-Time Performance Regression from Design System Overhead

**What goes wrong:**
Adding shadcn/ui + Radix UI primitives increases bundle size by 50-100KB. Real-time WebSocket updates (every 50-100ms) now trigger heavier React reconciliation due to complex component trees. Message rendering slows from <16ms to >50ms, causing visible stutter in high-traffic streams (raids, 20+ messages/second). Users complain of "laggy chat" and switch to competitors.

**Why it happens:**
- shadcn/ui components wrap simple elements in Radix UI primitives with accessibility features (focus management, ARIA attributes)
- Each message re-render now processes deeper component trees (Button → Radix Primitive → slot composition)
- Design system adds CSS-in-JS or runtime theme calculations
- Developers test with 1-2 messages/second, not real-world 20+ messages/second during raids
- No performance budget established before migration

**How to avoid:**
1. **Performance Budget**: Establish baseline BEFORE migration (target: <16ms per message render, <100KB additional bundle size)
2. **Selective Adoption**: Use shadcn/ui for static UI (dashboard, settings) NOT for real-time overlays
3. **Keep Overlay Simple**: Overlay message components should remain plain HTML/CSS without Radix primitives
4. **Bundle Analysis**: Run `npm run analyze` before/after each phase, flag >20KB increases
5. **Real-World Load Testing**: Test with 20 messages/second WebSocket feed, measure frame rate
6. **React.memo() Strategy**: Wrap all message components with `React.memo()` and custom comparison functions
7. **requestAnimationFrame Batching**: Buffer WebSocket updates in `useRef`, flush once per animation frame (reduce 20 renders/sec to 60fps max)

**Warning signs:**
- Bundle size increase >50KB after adding first shadcn/ui components
- Chrome DevTools Performance tab showing >16ms render times for message components
- Users reporting "choppy" or "laggy" chat during high-traffic events
- Lighthouse Performance score dropping below 90
- Frame rate drops below 60fps when WebSocket receives rapid messages
- `yarn why` showing multiple Radix UI packages totaling >100KB

**Phase to address:**
- **Phase 1 (Design Tokens):** Establish performance budget and monitoring
- **Phase 2 (Component Library):** Isolate overlay components from design system (use plain components)
- **Phase 3 (Page Migration):** Apply design system to static pages only, benchmark before/after
- **Phase 4 (Enforcement):** Add bundle size CI checks, performance regression tests

---

### Pitfall 4: CSS Specificity Wars Between Design System and Marketplace Themes

**What goes wrong:**
Marketplace theme creators use high-specificity selectors (`.event-message.event-tier-high`, `!important` flags) to override defaults. Design system introduces Tailwind v4's highest-precedence layer that overrides everything regardless of source order. Marketplace themes stop working, theme customization breaks, users can't personalize overlays.

**Why it happens:**
- Tailwind v4 utilities are in the highest-precedence layer by design
- Design system uses `!important` modifiers to force consistency
- Marketplace themes expect to override with higher specificity
- No documented override mechanism for theme creators
- Existing `events.css` uses `!important` extensively (12+ occurrences), setting precedent

**How to avoid:**
1. **CSS Layers Architecture**: Define explicit cascade layers:
   ```css
   @layer base, design-system, marketplace-themes, user-overrides;
   ```
2. **Remove !important**: Refactor `events.css` to use cascade layers instead of `!important` flags
3. **Theme Override API**: Provide CSS custom properties for all themeable values:
   ```css
   --event-tier-high-border: var(--marketplace-override-border, #FFD700);
   ```
4. **Documentation**: Create `THEMING_API.md` documenting all overridable CSS variables
5. **Escape Hatch**: Provide `data-allow-override` attribute to explicitly allow marketplace CSS to win

**Warning signs:**
- Marketplace theme preview showing default styles instead of custom theme
- User reports: "My custom CSS stopped working after update"
- CSS inspector showing Tailwind classes overriding marketplace `.event-*` selectors
- GitHub issues with "theme broken" or "can't customize" in title
- Testing reveals `!important` count increasing in codebase

**Phase to address:**
- **Phase 1 (Design Tokens):** Design CSS layers architecture, document override API
- **Phase 2 (Component Library):** Refactor `events.css` to use layers, remove `!important`
- **Phase 3 (Page Migration):** Validate marketplace theme compatibility
- **Phase 4 (Enforcement):** Add ESLint rule forbidding new `!important` usage

---

### Pitfall 5: Accessibility Regression from Insufficient Focus State Testing

**What goes wrong:**
Design system defines focus states (`focus:ring-2 focus:ring-blue-500/20`) but existing components lack focus styles. Migration adds focus rings to new components but misses legacy overlay controls. Keyboard users can't navigate overlays, WCAG 2.1 AA compliance breaks, ADA Title II deadline (April 24, 2026) missed.

**Why it happens:**
- Design system spec focuses on visual design, accessibility treated as secondary
- Testing uses mouse/clicks, not keyboard navigation
- Overlay preview lacks focus indicators (chat messages aren't focusable, skip links missing)
- Automated tools (axe, Lighthouse) run on dashboard but not overlay iframe
- Focus states work in dev (high contrast) but invisible in production dark theme

**How to avoid:**
1. **Keyboard-First Testing**: Test every page with Tab key BEFORE mouse testing
2. **Focus Visible Enforcement**: Add ESLint rule requiring `focus-visible:` on all interactive elements
3. **Contrast Validation**: Run contrast checker on `ring-blue-500/20` against `bg-slate-900` (ensure 3:1 ratio)
4. **Overlay Accessibility**: Add skip links, landmark regions, keyboard shortcuts to overlay preview
5. **CI/CD Integration**: Run axe-core in Playwright tests against all pages including overlays
6. **Manual Audit**: Schedule quarterly keyboard-only navigation audit

**Warning signs:**
- Playwright tests don't include keyboard navigation scenarios
- `focus:` appears in design system but not in overlay component code
- Axe violations in CI logs (Focus order, focus visible, color contrast)
- User reports: "Can't use app with keyboard"
- Manual Tab navigation reveals invisible focus states on dark backgrounds

**Phase to address:**
- **Phase 1 (Design Tokens):** Validate all focus ring colors meet WCAG AA contrast ratios
- **Phase 2 (Component Library):** Add focus states to ALL interactive components
- **Phase 3 (Page Migration):** Keyboard navigation testing for every migrated page
- **Phase 4 (Enforcement):** Add focus state coverage to CI/CD, block PRs with violations

---

### Pitfall 6: Incomplete Migration Leaving Visual Inconsistencies

**What goes wrong:**
Phase 3 migrates landing page, dashboard, editor to new design system. Settings and admin pages remain on old styles due to scope creep fatigue. Users see professional StreamElements-style UI, then jarring gray-scale generic Tailwind on settings page. Brand perception damaged, app feels unfinished.

**Why it happens:**
- Migration fatigue after redesigning 3-4 major pages
- Settings/admin pages seem "low priority" (not user-facing)
- No visual consistency audit between phases
- Screenshots in docs show new design, users expect it everywhere
- Design system enforcement starts before migration complete

**How to avoid:**
1. **All-or-Nothing Approach**: Migrate ALL pages in Phase 3, or delay enforcement until Phase 4
2. **Transition Stylesheet**: Create temporary `migration.css` that makes old pages acceptable (not perfect) until migration
3. **Migration Dashboard**: Track completion percentage, visualize inconsistencies
4. **Staging Environment**: Deploy partial migrations to staging only, block production until 100% complete
5. **Visual Consistency Tests**: Automated screenshot diffing across all pages, flag style discrepancies >20%

**Warning signs:**
- Phase 3 PR shows 3 pages migrated, 5 pages unchanged
- Design system enforcement PR merged while old components still in use
- User screenshots showing mix of old/new styles in same session
- QA testing reveals "This page looks different" observations
- Analytics showing drop-off on settings page (users confused by style change)

**Phase to address:**
- **Phase 3 (Page Migration):** Migrate ALL pages before marking phase complete
- **Phase 4 (Enforcement):** Only enable ESLint rules after 100% migration verified
- **Pre-deployment:** Visual consistency audit across all routes

---

### Pitfall 7: Insufficient Testing of Dynamic Class Construction

**What goes wrong:**
Codebase contains dynamic Tailwind class construction patterns like `className={'bg-' + platform + '-500'}` for platform colors. Tailwind v4's just-in-time engine doesn't detect these dynamically constructed classes, resulting in missing styles. Platform badges render without background colors, status indicators disappear.

**Why it happens:**
- Tailwind's JIT compiler only detects static class strings
- Dynamic construction was working in v3 due to different compilation strategy
- PurgeCSS safelist not updated for v4
- Developers test with hardcoded examples, miss dynamic edge cases
- TypeScript doesn't validate Tailwind class strings

**How to avoid:**
1. **Static Class Mapping**: Replace dynamic construction with explicit mapping objects:
   ```tsx
   const platformColors = {
     twitch: 'bg-purple-500',
     youtube: 'bg-red-500',
     // ... (as shown in DESIGN_SYSTEM.md line 543-569)
   }
   ```
2. **Safelist Configuration**: Add dynamic patterns to `tailwind.config.ts`:
   ```ts
   safelist: [
     { pattern: /^bg-(purple|red|green|slate)-(500|400)/ },
   ]
   ```
3. **TypeScript Validation**: Use `clsx` or `classnames` with TypeScript to catch invalid classes
4. **Grep Audit**: Search for `className.*\+.*` and `className.*\$\{` patterns before migration
5. **E2E Tests**: Add tests for all platform badge variations, verify colors render

**Warning signs:**
- Visual diff shows missing platform colors after Tailwind upgrade
- Badge components render with no background (just borders)
- Browser console warnings: "Class not found" or similar
- Users report: "Can't tell which platform a message is from"
- Dynamic class patterns found in `git grep "className.*+" frontend/`

**Phase to address:**
- **Phase 1 (Design Tokens):** Audit all dynamic class construction, create static mappings
- **Phase 2 (Component Library):** Refactor to use `platformColors` mapping from DESIGN_SYSTEM.md
- **Phase 3 (Page Migration):** Add E2E tests for all dynamic color variations
- **Phase 4 (Enforcement):** Add ESLint rule forbidding string concatenation in className

---

## Technical Debt Patterns

Shortcuts that seem reasonable but create long-term problems.

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Using `!important` to fix specificity issues | Quick fix for override conflicts | CSS maintenance nightmare, cascade layers broken, marketplace themes conflict | Never — use cascade layers or increase specificity properly |
| Inline styles for "just this one case" | Faster than creating component variant | Inconsistent design, hard to theme, no design system compliance | Only for dynamic runtime values (e.g., user-configurable positions) |
| Copy-paste shadcn/ui without customization | Get components quickly | Generic look, doesn't match StreamElements aesthetic, harder to change later | Phase 2 initial exploration only — must customize before Phase 3 |
| Skip accessibility testing "for now" | Move faster on visual implementation | ADA compliance violations, April 2026 deadline missed, lawsuits | Never — April 24, 2026 deadline is HARD |
| Migrate only user-facing pages | Reduce scope, ship faster | Inconsistent UI, brand perception damage, tech debt for later | Never — creates "half-finished" perception |
| Dynamic Tailwind class construction | DRY principle, fewer lines of code | JIT compilation fails, styles disappear, hard to debug | Never — use static mappings |
| Using gray-* instead of slate-* "by accident" | Works fine, minor difference | Design system violation, visual inconsistency, fails ESLint later | Never — enforce during code review |
| Skipping bundle size analysis | One less tool to run | Performance regressions undetected, 50-100KB bloat | Never — critical for real-time performance |

## Integration Gotchas

Common mistakes when connecting design system to existing architecture.

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| shadcn/ui + Tailwind v4 | Installing shadcn before Tailwind v4 upgrade, version conflicts | Upgrade Tailwind v4 FIRST, then install shadcn/ui configured for v4 |
| Design tokens + CSS variables | Defining tokens in CSS without TypeScript types | Generate TypeScript types from design tokens, enable autocomplete |
| Radix UI + Real-time overlays | Using Radix primitives in high-frequency render components | Isolate overlays from Radix, use plain HTML/CSS for messages |
| CSS layers + existing !important | Adding layers but keeping !important flags | Remove ALL !important before defining layers (line-by-line refactor) |
| Next.js App Router + shadcn | Using shadcn components in Server Components without 'use client' | Add 'use client' to all shadcn components or mark as client-only |
| Marketplace themes + Tailwind JIT | Expecting JIT to detect marketplace CSS classes | Add marketplace class patterns to safelist configuration |
| ESLint rules + gradual migration | Enabling all rules immediately, blocking all PRs | Enable rules per-phase: Phase 1 warnings, Phase 4 errors |
| Bundle splitting + design system | Importing entire shadcn/ui library in single chunk | Tree-shake: only import used components, analyze per-route bundles |

## Performance Traps

Patterns that work at small scale but fail as usage grows.

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Re-rendering entire message list on each WebSocket update | Smooth at 1-5 msg/sec, stutters at 20+ msg/sec (raids) | Use `React.memo()` + virtualization (react-window), requestAnimationFrame batching | >10 messages/second sustained (typical raid scenario) |
| Loading all shadcn components upfront | Fine for dashboard (<1s load), slow for overlays (3-5s) | Code split by route, lazy load modal/dialog components | Overlay bundle >200KB (affects OBS load time) |
| CSS-in-JS theme calculation on every render | Imperceptible with 10 components, laggy with 100+ messages | Use CSS variables, compute theme once at mount | >50 simultaneous chat messages on screen |
| Unoptimized Radix animations | Smooth with 1-2 modals, janky with rapid state changes | Disable animations for real-time components, use CSS transforms only | Rapid open/close cycles (e.g., toast notifications) |
| Not code-splitting design system | 5s initial load acceptable for dashboard, terrible for overlays | Separate bundles: app.js (design system) + overlay.js (minimal) | Overlay users have slower internet, 5s load unacceptable |
| Importing entire Lucide icon set | Works until 20+ icons added, then bundle bloats | Use `lucide-react/icons/IconName` for tree-shaking | Bundle includes >50 unused icons (check with webpack-bundle-analyzer) |

## Security Mistakes

Domain-specific security issues beyond general web security.

| Mistake | Risk | Prevention |
|---------|------|------------|
| Allowing unsanitized marketplace CSS | XSS via `background: url('javascript:...')` or `expression()` | CSP headers blocking unsafe-inline, CSS sanitization library |
| Design system exposes internal routes via debug tools | Admin pages leaked in Storybook build | Separate Storybook config for public components only |
| Theme customization allows arbitrary URLs | SSRF via `@import url('http://attacker.com')` | Validate all URLs against allowlist, block external resources |
| Client-side theme loading without validation | Malicious themes injected via localStorage tampering | Validate theme JSON schema, sanitize before applying |
| Exposing API keys in CSS custom properties | Credentials leaked in DevTools | Never store secrets in CSS variables, use server-side only |
| Marketplace themes executing JavaScript via CSS | Old IE expression() or behavior: url() still works in some contexts | Strict CSP, remove all `<script>` from theme uploads |

## UX Pitfalls

Common user experience mistakes in this domain.

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Changing overlay class names without deprecation period | All custom themes break immediately, user panic, negative reviews | 6+ month deprecation notice, automated migration tool, email notifications |
| Making low-contrast focus states "prettier" | Keyboard users can't navigate, accessibility violations, ADA non-compliance | Test focus states with high-contrast mode enabled, ensure 3:1 ratio minimum |
| Migrating dashboard but not settings | Users see professional UI, then jarring old styles, app feels broken | Migrate all pages together, or delay rollout until 100% complete |
| Auto-applying design system updates to overlays | User's carefully crafted overlay suddenly changes appearance on stream | Version-lock overlay styles, require explicit opt-in to updates |
| Removing animation controls for "consistency" | Power users lose customization they depend on, switch to competitors | Provide "advanced" toggle for animation preferences |
| Not providing migration guide for marketplace creators | Theme creators abandoned, marketplace dies, users lose content | Write migration guide FIRST, provide migration CLI tool, offer support channel |
| Changing platform badge colors for aesthetics | Users can't quickly identify platforms they're monitoring (muscle memory) | Preserve exact platform brand colors, only adjust transparency/borders |
| Focus trap in modals without escape | Users get stuck in dialogs, frustration, appears broken | Always provide ESC key, click-outside, and visible close button |

## "Looks Done But Isn't" Checklist

Things that appear complete but are missing critical pieces.

- [ ] **Tailwind v4 Migration:** Often missing dynamic class construction fixes — verify `git grep "className.*+" frontend/` returns zero results
- [ ] **shadcn/ui Integration:** Often missing customization to match design system — verify all components use slate (not zinc) colors
- [ ] **Focus States:** Often missing on custom components — verify Tab navigation works on ALL interactive elements
- [ ] **Bundle Size:** Often missing route-level splitting — verify each route <100KB via `yarn analyze`
- [ ] **Marketplace Compatibility:** Often missing migration guide — verify `MARKETPLACE_CSS_MIGRATION.md` exists and is tested
- [ ] **Performance Budget:** Often missing real-world load testing — verify 20 msg/sec WebSocket test passes <16ms render time
- [ ] **Accessibility Audit:** Often missing keyboard-only testing — verify entire app navigable without mouse
- [ ] **Visual Consistency:** Often missing admin/settings pages — verify ALL routes use design system (screenshot diff)
- [ ] **CSS Layers:** Often missing layer definitions — verify `@layer` declarations in `globals.css`
- [ ] **Color Contrast:** Often missing dark mode testing — verify all text meets WCAG AA (4.5:1 normal, 3:1 large)
- [ ] **Platform Color Mapping:** Often missing static object — verify no dynamic `'bg-' + platform` construction
- [ ] **Event Class Stability:** Often missing public API documentation — verify `events.css` classes documented as stable
- [ ] **Responsive Testing:** Often missing mobile overlay testing — verify overlays render correctly at 375px, 768px, 1920px
- [ ] **Animation Performance:** Often missing frame rate validation — verify 60fps maintained during rapid updates
- [ ] **Theme Override API:** Often missing CSS variable documentation — verify all themeable values use `var(--marketplace-*, default)`

## Recovery Strategies

When pitfalls occur despite prevention, how to recover.

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Breaking marketplace themes | HIGH (user trust, support load, potential revenue loss) | 1. Hotfix: revert class changes immediately 2. Deploy compatibility shim 3. Publish migration tool 4. Email all theme creators 5. Extend deprecation 6 months |
| Tailwind v4 gradient breaking | MEDIUM (visual regression, no functionality lost) | 1. Run `npx @tailwindcss/upgrade` 2. Manual audit of CSS files 3. Visual regression test suite 4. Deploy fix within 24h 5. Document in post-mortem |
| Performance regression | MEDIUM (user complaints, not immediate breakage) | 1. Rollback to previous version 2. Profile with Chrome DevTools 3. Add React.memo() to message components 4. Implement RAF batching 5. Deploy with monitoring |
| CSS specificity conflicts | LOW (theme customization broken, not core functionality) | 1. Add `@layer marketplace-themes` with higher precedence 2. Document override API 3. Provide CSS variable escape hatch 4. Update theme documentation |
| Accessibility violations | HIGH (legal risk, ADA deadline) | 1. Audit with axe-core immediately 2. Prioritize WCAG A violations 3. Add focus states to all interactive elements 4. Deploy fix before April 24, 2026 deadline 5. Document compliance |
| Incomplete migration | LOW (perception issue, not broken) | 1. Prioritize remaining pages 2. Apply temporary styling to un-migrated pages 3. Complete migration in next sprint 4. Deploy all-at-once |
| Dynamic class construction failing | LOW (missing styles, easy to spot) | 1. Create static `platformColors` mapping object 2. Replace all dynamic construction 3. Add safelist for edge cases 4. Deploy with E2E tests |
| Bundle size bloat | MEDIUM (performance degradation) | 1. Run webpack-bundle-analyzer 2. Identify largest dependencies 3. Lazy load non-critical components 4. Remove unused Radix primitives 5. Deploy optimized bundle |

## Pitfall-to-Phase Mapping

How roadmap phases should address these pitfalls.

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Breaking marketplace themes | Phase 1 (Design Tokens) | Document all `events.css` classes as public API, create stability contract |
| Tailwind v4 gradient renames | Phase 1 (Design Tokens) | Run upgrade tool, audit dynamic construction, visual regression tests pass |
| Performance regression | Phase 2 (Component Library) | Bundle size <100KB increase, message render time <16ms at 20 msg/sec |
| CSS specificity conflicts | Phase 2 (Component Library) | Marketplace theme test suite passes, no `!important` added |
| Accessibility violations | Phase 3 (Page Migration) | axe-core CI tests pass, keyboard navigation works on all pages |
| Incomplete migration | Phase 3 (Page Migration) | Visual consistency audit shows 100% pages using design system |
| Dynamic class construction | Phase 1 (Design Tokens) | Zero results for `git grep "className.*+" frontend/` |
| Bundle size bloat | Phase 4 (Enforcement) | CI blocks PRs adding >20KB without justification |
| Focus state missing | Phase 4 (Enforcement) | ESLint rule `focus-visible-required` enabled, no violations |
| Gray vs slate confusion | Phase 4 (Enforcement) | ESLint rule `no-gray-colors` blocks gray-* usage |
| Animation performance | Phase 3 (Page Migration) | Frame rate monitoring shows sustained 60fps during raids |
| Theme override breaking | Phase 2 (Component Library) | `THEMING_API.md` published, CSS variable override tests pass |

## Sources

### Tailwind CSS v4 Migration & Breaking Changes
- [Tailwind CSS v4 2026: Migration Best Practices](https://www.digitalapplied.com/blog/tailwind-css-v4-2026-migration-best-practices) — HIGH confidence
- [Tailwind CSS v4 Migration: New Features Guide 2026](https://www.digitalapplied.com/blog/tailwind-css-v4-migration-new-features-guide) — HIGH confidence
- [What's New in Tailwind CSS 4.0: Migration Guide (2026) | DesignRevision](https://designrevision.com/blog/tailwind-4-migration) — HIGH confidence
- [Upgrading to Tailwind CSS v4: A Migration Guide](https://typescript.tv/hands-on/upgrading-to-tailwind-css-v4-a-migration-guide/) — MEDIUM confidence

### shadcn/ui + Tailwind v4 Integration
- [Tailwind v4 - shadcn/ui](https://ui.shadcn.com/docs/tailwind-v4) — HIGH confidence (official docs)
- [Shadcn/ui upgrade to Tailwindcss v.4 · Discussion #2996](https://github.com/shadcn-ui/ui/discussions/2996) — HIGH confidence
- [Migrating from Tailwind 3 to Tailwind 4 with shadcn/ui](https://zippystarter.com/blog/guides/migrating-tailwind3-to-tailwind4-with-shadcn) — MEDIUM confidence
- [Updating shadcn/ui to Tailwind 4 at Shadcnblocks](https://www.shadcnblocks.com/blog/tailwind4-shadcn-themeing) — MEDIUM confidence

### Performance & Bundle Size
- [React & Next.js Best Practices in 2026: Performance, Scale & Cleaner Code](https://fabwebstudio.com/blog/react-nextjs-best-practices-2026-performance-scale) — MEDIUM confidence
- [Reducing NextJS Bundle Size by 30%: A Practical Guide](https://www.coteries.com/en/articles/reduce-size-nextjs-bundle) — MEDIUM confidence
- [React Performance and Bundle Size Optimization in 2025](https://www.averagedevs.com/blog/optimize-react-apps-performance) — MEDIUM confidence
- [Optimizing Real-Time Performance: WebSockets and React.js Integration Part II](https://medium.com/@SanchezAllanManuel/optimizing-real-time-performance-websockets-and-react-js-integration-part-ii-4a3ada319630) — MEDIUM confidence
- [Streaming Backends & React: Controlling Re-render Chaos in High-Frequency Data](https://www.sitepoint.com/streaming-backends-react-controlling-re-render-chaos/) — HIGH confidence

### CSS Specificity & Naming Conflicts
- [Naming - Nord Design System](https://nordhealth.design/naming/) — HIGH confidence
- [BEM Methodology: A Step-by-Step Guide for Beginners](https://www.valoremreply.com/resources/insights/guide/bem-methodology-a-step-by-step-guide-for-beginners/) — MEDIUM confidence
- [Tailwind CSS: !important & Selector Guide](https://tailkits.com/blog/tailwind-important-selector/) — MEDIUM confidence
- [Using the important modifier in Tailwind CSS](https://windybase.com/blog/using-the-important-modifier-in-tailwind-css) — MEDIUM confidence

### Accessibility & WCAG Compliance
- [WebAIM: 2026 Predictions: The Next Big Shifts in Web Accessibility](https://webaim.org/blog/2026-predictions/) — HIGH confidence
- [WCAG 3.0 Updates Explained: Accessibility Guidelines 2026-2030](https://rubyroidlabs.com/blog/2025/10/how-to-prepare-for-wcag-3-0/) — MEDIUM confidence
- [WCAG 2.2 AA: Why Accessibility Is Now a Required Part of Your 2026 Digital Roadmap](https://www.stauffer.com/news/blog/wcag-is-no-longer-optional-and-what-that-means-for-your-organization) — HIGH confidence
- [ADA Title II Digital Accessibility 2026: WCAG 2.1 AA](https://www.sdettech.com/blogs/ada-title-ii-digital-accessibility-2026-wcag-2-1-aa) — HIGH confidence

### Design System Migration Best Practices
- [How to Implement a Design System: Reasons, Approach, and Migration Path](https://www.designsystemscollective.com/how-to-implement-a-design-system-reasons-approach-and-migration-path-051c41734caf) — MEDIUM confidence
- [Migrating to USWDS 3.0](https://designsystem.digital.gov/documentation/migration/) — HIGH confidence (official government design system)
- [Telerik and Kendo UI Themes v13.0.0 Breaking Changes](https://www.telerik.com/design-system/docs/themes/release-notes/breaking-changes/v13-0-0/) — MEDIUM confidence

### Project-Specific Knowledge
- `/home/caesar/git/all-chat/frontend/DESIGN_SYSTEM.md` — HIGH confidence (project spec)
- `/home/caesar/git/all-chat/frontend/src/styles/events.css` — HIGH confidence (existing overlay CSS)
- `/home/caesar/git/all-chat/.planning/PROJECT.md` — HIGH confidence (project context)

---
*Pitfalls research for: All-Chat Frontend Design System Integration*
*Researched: 2026-03-09*
*Confidence: HIGH (verified against official docs, current project code, and 2026 best practices)*
