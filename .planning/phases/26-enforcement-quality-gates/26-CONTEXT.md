# Phase 26: Enforcement & Quality Gates - Context

**Gathered:** 2026-03-14
**Status:** Ready for planning

<domain>
## Phase Boundary

Automate design system compliance through tooling — ESLint rules, Prettier formatting, pre-commit hooks, CI/CD quality gates, visual regression (Chromatic), performance monitoring, and a marketplace CSS migration guide for overlay theme authors. All pages and components are already migrated (Phase 25). This phase locks in enforcement so regressions cannot be introduced silently.

No new components or pages. No backend changes. Tooling and documentation only.

</domain>

<decisions>
## Implementation Decisions

### Enforcement Strictness

- **Custom design token ESLint rules are errors everywhere** — violations appear as red in IDE, block pre-commit hook AND CI. No warn-only path. Zero ambiguity: you either comply or the commit fails.
- **Rules in scope (ENFORCE-03):** no `gray-*` Tailwind classes, no `className` string concatenation, `focus-visible` required on interactive elements (not `focus`).
- **Pre-commit hook blocks on:** ESLint errors + Prettier formatting + TypeScript type-check (`tsc --noEmit`). Full check. Lint-staged runs ESLint + Prettier on staged files only; tsc runs on full project.
- **Bundle size gate:** PRs that increase the bundle by >20KB fail CI unless the PR description includes an explicit justification comment. Aligns with ROADMAP ENFORCE-05 spec.

### CI/CD Structure

- **Separate `frontend-quality.yml` workflow** — quality gates run independently of the existing `build-and-push.yml` Docker build workflow. Faster feedback on PRs without waiting for image builds.
- `frontend-quality.yml` steps: install deps → ESLint → Prettier check → TypeScript check → build (for bundle size) → Storybook tests (a11y + perf) → Chromatic.

### Visual Regression (Chromatic)

- **Scope:** All Storybook stories snapshotted — component-level coverage using the existing story structure from Phase 24.
- **PR policy:** CI **blocks** the PR until visual changes are explicitly accepted or rejected in Chromatic UI. Intentional — any visual change requires human sign-off.
- **Token setup:** No Chromatic project token exists yet. Planner must include setup instructions: create project at chromatic.com, get token, add as `CHROMATIC_PROJECT_TOKEN` GitHub Actions secret, reference in `frontend-quality.yml`.

### Marketplace Migration Guide

- **Audience:** External overlay theme authors who publish CSS themes to the marketplace.
- **Location:** `frontend/src/styles/MARKETPLACE_MIGRATION_GUIDE.md` — alongside the existing `EVENTS_CSS_API.md` (Phase 23) as a companion document.
- **Content depth:** Class name mapping + frozen API — three sections:
  1. Frozen/stable class names (reference the EVENTS_CSS_API.md contract)
  2. Any class names that changed or moved from pre-v1.3 to v1.3
  3. How the cascade layer system works (`marketplace-themes` layer) so themes don't need `!important`

### Performance Monitoring

- **Render time validation (<16ms at 20 msg/sec):** Storybook interaction test using `@storybook/addon-vitest` — renders ChatMessage rapidly, measures frame timing. Runs in CI as part of the Storybook test suite. No separate Playwright test.
- **Bundle size analysis:** Install `@next/bundle-analyzer`, add `ANALYZE=true` build script. Establish baseline in this phase. CI tracks delta against that baseline for the >20KB gate.
- **A11y:** Storybook a11y addon is already set to `'error'` mode in `preview.ts` — the existing `@storybook/addon-vitest` integration makes CI failures happen automatically when stories run. No additional axe-core setup required. ENFORCE-10 is effectively already satisfied.

### Claude's Discretion

- Exact `lint-staged` configuration (file globs, command order)
- ESLint plugin choice for Tailwind class enforcement (`eslint-plugin-tailwindcss` vs custom rule) — researcher to evaluate
- Prettier config values (print width, trailing commas, etc.) — standard Next.js conventions
- Exact `tsc` flags for pre-commit (e.g., whether to use `--incremental`)
- Specific Storybook vitest interaction test implementation for render timing

</decisions>

<code_context>
## Existing Code Insights

### Reusable Assets
- `frontend/.eslintrc.json` — exists with `next/core-web-vitals` + `storybook/recommended`. Extend, don't replace.
- `frontend/.storybook/main.ts` — already has `@chromatic-com/storybook`, `@storybook/addon-vitest`, `@storybook/addon-a11y`
- `frontend/.storybook/preview.ts` — already imports `globals.css` and sets `a11y.test: 'error'`
- `frontend/package.json` — has ESLint, Chromatic, and Storybook devDeps; no Prettier or Husky yet
- `.github/workflows/build-and-push.yml` — existing CI for Docker builds; quality gates go in a NEW separate workflow

### Established Patterns
- ESLint v10 already installed — use flat config format if extending (check current config format before adding plugins)
- Storybook 10 + `@storybook/addon-vitest` already wired — performance tests slot in as interaction tests in existing story files
- `@chromatic-com/storybook` already installed — just needs project token and CI wiring, no installation step
- Cascade layer `marketplace-themes` already defined in `globals.css` (Phase 23) — migration guide explains this to theme authors

### Integration Points
- `frontend/.husky/` directory does not exist — Husky must be initialized (`husky init`)
- No `lint-staged` config exists yet — add to `package.json` or `.lintstagedrc`
- No Prettier config exists yet — add `.prettierrc` with `prettier-plugin-tailwindcss`
- `EVENTS_CSS_API.md` at `frontend/src/styles/` — migration guide goes alongside it
- GitHub Actions secrets — `CHROMATIC_PROJECT_TOKEN` must be added by the repo owner

</code_context>

<specifics>
## Specific Ideas

- Pre-commit should be strict (lint + format + type-check) even though it's slower. The user explicitly chose this over the lighter option.
- "Errors everywhere" for design token rules — no gradual rollout, no warnings. Phase 25 verified 100% compliance, so there's no existing code to grandfather.
- The Chromatic setup instructions in the plan should be detailed enough for a first-time user: chromatic.com signup → connect GitHub repo → copy project token → add as GitHub secret.
- Migration guide companion doc: EVENTS_CSS_API.md (Phase 23) covers the frozen API; MARKETPLACE_MIGRATION_GUIDE.md covers the upgrade path and cascade layer explanation.

</specifics>

<deferred>
## Deferred Ideas

- None — discussion stayed within phase scope

</deferred>

---

*Phase: 26-enforcement-quality-gates*
*Context gathered: 2026-03-14*
