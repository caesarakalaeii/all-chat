# Requirements: All-Chat

**Defined:** 2026-03-09
**Core Value:** Streamers can aggregate chat from all platforms they stream to, with reliable message delivery even during high-traffic events through intelligent load balancing, auto-scaling, and unlimited YouTube chat access.

## v1.3 Requirements

Requirements for Frontend Redesign milestone. Each maps to roadmap phases.

### Design System Foundation

- [x] **FOUND-01**: Design token system established with Tailwind v4 @theme directive (colors, spacing, typography, shadows)
- [x] **FOUND-02**: Three-layer token hierarchy implemented (base → semantic → component)
- [x] **FOUND-03**: Static platform color mapping object created (no dynamic class construction)
- [x] **FOUND-04**: Overlay CSS stability contract documented (events.css classes as public API)
- [x] **FOUND-05**: Tailwind v4 gradient codemod executed (bg-gradient-to-* → bg-linear-to-*)
- [x] **FOUND-06**: CSS cascade layers defined (@layer base, design-system, marketplace-themes, user-overrides)

### Component Library

- [x] **COMP-01**: shadcn/ui core primitives installed (Button, Card, Input, Badge, Dialog, Toast)
- [x] **COMP-02**: Components customized with design tokens (slate scale, not zinc)
- [x] **COMP-03**: Component variant patterns implemented with CVA
- [x] **COMP-04**: Smooth micro-interactions added (hover scale + shadow transitions)
- [x] **COMP-05**: Gradient CTAs implemented (purple → blue gradient for primary actions)
- [x] **COMP-06**: Platform-color coded components created (badges, borders, status indicators)
- [x] **COMP-07**: Animated loading states and skeletons implemented
- [ ] **COMP-08**: Performance budget established (<16ms message render, <100KB bundle increase)
- [x] **COMP-09**: All !important removed from events.css (replaced with cascade layers)

### Page Redesigns

- [ ] **PAGE-01**: Landing page redesigned (hero with gradient CTAs, platform login buttons, feature cards)
- [ ] **PAGE-02**: Dashboard redesigned (overlay grid with hover states, create button, empty states)
- [ ] **PAGE-03**: Overlay editor redesigned (source management cards with platform-color coding)
- [ ] **PAGE-04**: Overlay preview maintained (existing CSS unchanged for marketplace compatibility)
- [ ] **PAGE-05**: Settings page redesigned (account management, profile display)
- [ ] **PAGE-06**: Admin pages redesigned (users, overlays, sources, viewers - visual consistency)
- [ ] **PAGE-07**: Responsive layouts validated across breakpoints (375px, 768px, 1920px)
- [ ] **PAGE-08**: Accessibility compliance achieved (WCAG 2.1 AA, keyboard navigation, focus states)
- [ ] **PAGE-09**: Loading states implemented for all data fetching scenarios
- [ ] **PAGE-10**: Empty states implemented with illustrations and clear CTAs

### Split-view Live Preview

- [ ] **FEAT-01**: Split-view layout implemented (editor configuration + live preview side-by-side)
- [ ] **FEAT-02**: Preview updates in real-time as configuration changes
- [ ] **FEAT-03**: Responsive split-view (stacks vertically on mobile, side-by-side on desktop)
- [ ] **FEAT-04**: Preview window maintains overlay CSS (marketplace theme compatibility)

### Enforcement & Migration

- [ ] **ENFORCE-01**: ESLint plugin for Tailwind installed and configured
- [ ] **ENFORCE-02**: Prettier plugin for Tailwind installed with class ordering
- [ ] **ENFORCE-03**: ESLint rules defined (no gray-*, focus-visible required, no string concat in className)
- [ ] **ENFORCE-04**: Pre-commit hooks configured with Husky (lint + format on changed files)
- [ ] **ENFORCE-05**: CI/CD quality gates implemented (ESLint errors block PRs, bundle size >20KB requires justification)
- [ ] **ENFORCE-06**: Visual regression test suite implemented (screenshot diffing all pages)
- [ ] **ENFORCE-07**: Marketplace CSS migration guide created (class name changes documented)
- [ ] **ENFORCE-08**: Performance monitoring configured (message render time <16ms at 20 msg/sec)
- [ ] **ENFORCE-09**: Bundle size analysis baseline established (webpack-bundle-analyzer)
- [ ] **ENFORCE-10**: Accessibility testing automated (axe-core in CI/CD)

## v2 Requirements

Deferred to future release. Tracked but not in current roadmap.

### Advanced Interactions

- **ADV-01**: Drag-and-drop source reordering (intuitive priority management)
- **ADV-02**: Command palette (Cmd+K for quick navigation and actions)
- **ADV-03**: Inline click-to-edit patterns (reduce friction for overlay editing)
- **ADV-04**: Keyboard shortcuts for common actions (create overlay, preview, settings)

### Theming Enhancements

- **THEME-01**: Light theme toggle (doubles maintenance burden, defer until dark theme perfected)
- **THEME-02**: Theme presets (2-3 curated color schemes)
- **THEME-03**: Component marketplace (user-contributed themes beyond overlay marketplace)

### Animation Enhancements

- **ANIM-01**: Advanced animations library integration (Framer Motion vs Tailwind Motion)
- **ANIM-02**: Complex transitions for modal/dialog appearances
- **ANIM-03**: Parallax effects for marketing pages

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Light theme in v1.3 | Streaming tools default to dark (StreamElements, OBS). Perfecting single dark theme is higher value than offering mediocre light theme. |
| Real-time collaboration editing | High complexity, unclear user need. Chat aggregation is not a collaborative tool. |
| Per-component animation customization | Creates jarring UX inconsistencies. Design system requires consistent motion patterns. |
| Customizable everything (colors, fonts, sizes) | Destroys design consistency. User customization limited to overlay marketplace themes only. |
| Heavy animations (GSAP, complex parallax) | Performance issues in OBS browser sources. Micro-interactions sufficient for professional feel. |
| Mobile app (iOS/Android) | Web-first approach. Mobile web responsive design covers mobile use cases. |
| Backend service redesigns | v1.3 scope is frontend only. Backend services (Go microservices) remain unchanged. |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| FOUND-01 | Phase 23 | Complete |
| FOUND-02 | Phase 23 | Complete |
| FOUND-03 | Phase 23 | Complete |
| FOUND-04 | Phase 23 | Complete |
| FOUND-05 | Phase 23 | Complete |
| FOUND-06 | Phase 23 | Complete |
| COMP-01 | Phase 24 | Complete |
| COMP-02 | Phase 24 | Complete |
| COMP-03 | Phase 24 | Complete |
| COMP-04 | Phase 24 | Complete |
| COMP-05 | Phase 24 | Complete |
| COMP-06 | Phase 24 | Complete |
| COMP-07 | Phase 24 | Complete |
| COMP-08 | Phase 24 | Pending |
| COMP-09 | Phase 24 | Complete |
| PAGE-01 | Phase 25 | Pending |
| PAGE-02 | Phase 25 | Pending |
| PAGE-03 | Phase 25 | Pending |
| PAGE-04 | Phase 25 | Pending |
| PAGE-05 | Phase 25 | Pending |
| PAGE-06 | Phase 25 | Pending |
| PAGE-07 | Phase 25 | Pending |
| PAGE-08 | Phase 25 | Pending |
| PAGE-09 | Phase 25 | Pending |
| PAGE-10 | Phase 25 | Pending |
| FEAT-01 | Phase 25 | Pending |
| FEAT-02 | Phase 25 | Pending |
| FEAT-03 | Phase 25 | Pending |
| FEAT-04 | Phase 25 | Pending |
| ENFORCE-01 | Phase 26 | Pending |
| ENFORCE-02 | Phase 26 | Pending |
| ENFORCE-03 | Phase 26 | Pending |
| ENFORCE-04 | Phase 26 | Pending |
| ENFORCE-05 | Phase 26 | Pending |
| ENFORCE-06 | Phase 26 | Pending |
| ENFORCE-07 | Phase 26 | Pending |
| ENFORCE-08 | Phase 26 | Pending |
| ENFORCE-09 | Phase 26 | Pending |
| ENFORCE-10 | Phase 26 | Pending |

**Coverage:**
- v1.3 requirements: 39 total
- Mapped to phases: 39
- Unmapped: 0 ✓

---
*Requirements defined: 2026-03-09*
*Last updated: 2026-03-09 after roadmap creation (phase numbers updated 23-26)*
