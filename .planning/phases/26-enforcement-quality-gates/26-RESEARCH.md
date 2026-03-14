# Phase 26: Enforcement & Quality Gates - Research

**Researched:** 2026-03-14
**Domain:** ESLint v10 flat config, Prettier + Tailwind, Husky v9, Chromatic, bundle analysis, CI/CD GitHub Actions
**Confidence:** HIGH

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **Custom design token ESLint rules are errors everywhere** — violations appear as red in IDE, block pre-commit hook AND CI. No warn-only path. Zero ambiguity: you either comply or the commit fails.
- **Rules in scope (ENFORCE-03):** no `gray-*` Tailwind classes, no `className` string concatenation, `focus-visible` required on interactive elements (not `focus`).
- **Pre-commit hook blocks on:** ESLint errors + Prettier formatting + TypeScript type-check (`tsc --noEmit`). Full check. Lint-staged runs ESLint + Prettier on staged files only; tsc runs on full project.
- **Bundle size gate:** PRs that increase the bundle by >20KB fail CI unless the PR description includes an explicit justification comment. Aligns with ROADMAP ENFORCE-05 spec.
- **Separate `frontend-quality.yml` workflow** — quality gates run independently of the existing `build-and-push.yml` Docker build workflow. Faster feedback on PRs without waiting for image builds.
- `frontend-quality.yml` steps: install deps → ESLint → Prettier check → TypeScript check → build (for bundle size) → Storybook tests (a11y + perf) → Chromatic.
- **Visual regression scope:** All Storybook stories snapshotted — component-level coverage using the existing story structure from Phase 24.
- **PR policy:** CI blocks the PR until visual changes are explicitly accepted or rejected in Chromatic UI.
- **Token setup:** No Chromatic project token exists yet. Planner must include setup instructions: create project at chromatic.com, get token, add as `CHROMATIC_PROJECT_TOKEN` GitHub Actions secret.
- **Marketplace migration guide location:** `frontend/src/styles/MARKETPLACE_MIGRATION_GUIDE.md` alongside `EVENTS_CSS_API.md`.
- **Performance render time validation (<16ms at 20 msg/sec):** Storybook interaction test using `@storybook/addon-vitest` — no separate Playwright test.
- **Bundle size analysis:** `@next/bundle-analyzer`, `ANALYZE=true` build script, baseline established in this phase.
- **A11y:** Storybook `preview.ts` already has `a11y.test: 'error'` — ENFORCE-10 is effectively already satisfied. No additional setup required.

### Claude's Discretion

- Exact `lint-staged` configuration (file globs, command order)
- ESLint plugin choice for Tailwind class enforcement (`eslint-plugin-tailwindcss` vs custom rule) — researcher to evaluate (see Standard Stack section for recommendation)
- Prettier config values (print width, trailing commas, etc.) — standard Next.js conventions
- Exact `tsc` flags for pre-commit (e.g., whether to use `--incremental`)
- Specific Storybook vitest interaction test implementation for render timing

### Deferred Ideas (OUT OF SCOPE)

- None — discussion stayed within phase scope
</user_constraints>

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|-----------------|
| ENFORCE-01 | ESLint plugin for Tailwind installed and configured | Use `eslint-plugin-tailwindcss` beta (Tailwind v4 partial) + `no-restricted-syntax` custom rules for the three project-specific rules |
| ENFORCE-02 | Prettier plugin for Tailwind installed with class ordering | `prettier-plugin-tailwindcss` v0.6.x supports Tailwind v4 with `tailwindStylesheet` option pointing to globals.css |
| ENFORCE-03 | ESLint rules: no gray-*, focus-visible required, no string concat in className | Implemented via `no-restricted-syntax` AST selectors in flat config — fully documented below |
| ENFORCE-04 | Pre-commit hooks configured with Husky (lint + format on changed files) | Husky v9 `husky init` pattern; lint-staged on staged files; tsc full-project |
| ENFORCE-05 | CI/CD quality gates: ESLint errors block PRs, bundle >20KB requires justification | `frontend-quality.yml` with hashicorp/nextjs-bundle-analysis action for delta reporting |
| ENFORCE-06 | Visual regression test suite (screenshot diffing all pages/components) | `chromaui/action@latest` in CI with `exitZeroOnChanges: false`; `autoAcceptChanges: "main"` |
| ENFORCE-07 | Marketplace CSS migration guide created | Markdown doc at `frontend/src/styles/MARKETPLACE_MIGRATION_GUIDE.md` alongside EVENTS_CSS_API.md |
| ENFORCE-08 | Performance monitoring: message render time <16ms at 20 msg/sec | Storybook interaction test in existing story files using `performance.now()` timing |
| ENFORCE-09 | Bundle size analysis baseline established | `@next/bundle-analyzer` + `ANALYZE=true` script in `next.config.js`; baseline JSON committed |
| ENFORCE-10 | Accessibility testing automated (axe-core in CI/CD) | Already done: `preview.ts` has `a11y.test: 'error'`; Storybook vitest runs in CI — no new work |
</phase_requirements>

---

## Summary

Phase 26 is a tooling-only phase: no new components, no backend changes. All existing code is already design-system-compliant (Phase 25 verified 100% compliance), so enforcement tooling can be added at maximum strictness with zero grandfathering.

The critical blocker discovered in research is **ESLint v10 (already installed at `^10.0.0`) requires flat config** — `.eslintrc.json` is no longer honored. The existing `frontend/.eslintrc.json` must be replaced with `frontend/eslint.config.mjs`. This is mandatory, not optional. The new format using `defineConfig()` from `eslint/config` with `eslint-config-next/core-web-vitals` spread is well-documented by Next.js 16 official docs.

The ESLint Tailwind plugin situation requires a nuanced decision: `eslint-plugin-tailwindcss` has only partial Tailwind v4 support (beta, false positives). For the three project-specific rules (no `gray-*`, no string concat, focus-visible), using ESLint's built-in `no-restricted-syntax` with AST selectors is more reliable than any external plugin. Ordering enforcement (ENFORCE-01/02) is covered by `prettier-plugin-tailwindcss`, which has solid Tailwind v4 support via the `tailwindStylesheet` config option.

**Primary recommendation:** Use `eslint-plugin-tailwindcss` at beta for classnames-order enforcement (then disable contradicting-classname rule which has false positives in v4), implement the three custom design-token rules via `no-restricted-syntax`, and cover class ordering with `prettier-plugin-tailwindcss` pointing to `globals.css`.

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `eslint-plugin-tailwindcss` | `^3.0.0` (beta/latest) | Tailwind class ordering, no-custom-classname | Official ecosystem plugin; partial v4 support via beta channel |
| `prettier` | `^3.x` | Code formatting | Industry standard; required by `prettier-plugin-tailwindcss` |
| `prettier-plugin-tailwindcss` | `^0.6.x` | Automatic Tailwind class sorting | Official Tailwind Labs plugin; v4 support via `tailwindStylesheet` option |
| `eslint-config-prettier` | `^10.x` | Disables ESLint formatting rules that conflict with Prettier | Required when using both tools |
| `husky` | `^9.x` | Git hooks (pre-commit) | Standard de facto for Node projects; v9 uses `husky init` |
| `lint-staged` | `^15.x` | Run linters on staged files only | Pairs with Husky; keeps pre-commit fast |
| `@next/bundle-analyzer` | `^16.x` | Bundle size visualization and baseline | Official Next.js package; matches installed Next.js version |
| `hashicorp/nextjs-bundle-analysis` | GitHub Action | PR bundle size delta reporting with comments | Widely used; compares against committed baseline JSON |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `chromaui/action` | `latest` | Chromatic visual regression in GitHub Actions | Already have `@chromatic-com/storybook` installed; just needs CI wiring |
| `eslint-config-prettier/flat` | (part of eslint-config-prettier) | Prettier-compatible flat config export | Required for ESLint v10 flat config format |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `eslint-plugin-tailwindcss` beta | `eslint-plugin-better-tailwindcss` | better-tailwindcss has v4 support but is a smaller ecosystem project; preset-in-flat-config issues noted in GitHub issues |
| `no-restricted-syntax` custom rules | `eslint-plugin-tailwindcss` for all three ENFORCE-03 rules | The plugin does not provide rules for focus-visible enforcement or className string concat — `no-restricted-syntax` is required regardless |
| `hashicorp/nextjs-bundle-analysis` | Manual bundle size script | hashicorp action posts PR comments with delta automatically; manual approach requires custom scripting |

**Installation:**
```bash
# From frontend/ directory
npm install --save-dev prettier prettier-plugin-tailwindcss eslint-config-prettier husky lint-staged @next/bundle-analyzer eslint-plugin-tailwindcss
```

---

## Architecture Patterns

### Critical: ESLint v10 Requires Flat Config

ESLint v10 (already installed) removed support for `.eslintrc.*` files entirely. The existing `frontend/.eslintrc.json` will be silently ignored. It must be replaced with `frontend/eslint.config.mjs`.

**Verified by:** Next.js 16.1 official docs (https://nextjs.org/docs/app/api-reference/config/eslint), ESLint v10.0.0 release notes.

### Pattern 1: ESLint Flat Config (eslint.config.mjs)

**What:** Single flat config file replacing `.eslintrc.json`, using `defineConfig()` API.
**When to use:** Always — it is now the only config format supported.

```javascript
// Source: https://nextjs.org/docs/app/api-reference/config/eslint
// frontend/eslint.config.mjs
import { defineConfig, globalIgnores } from 'eslint/config'
import nextVitals from 'eslint-config-next/core-web-vitals'
import prettier from 'eslint-config-prettier/flat'
import tailwind from 'eslint-plugin-tailwindcss'

export default defineConfig([
  // Next.js core-web-vitals (includes react, react-hooks, @next/next rules)
  ...nextVitals,

  // Storybook rules (replaces plugin:storybook/recommended from old .eslintrc.json)
  // NOTE: eslint-plugin-storybook must also export flat config — verify on install
  // If not available as flat, add rules manually or use legacy compat

  // Tailwind plugin (beta — v4 partial support; disable false-positive rules)
  ...tailwind.configs['flat/recommended'],
  {
    rules: {
      // Disable rules that generate false positives with Tailwind v4
      'tailwindcss/no-contradicting-classname': 'off',
    },
    settings: {
      tailwindcss: {
        // Point to Tailwind v4 CSS entry point
        config: 'src/app/globals.css',
      },
    },
  },

  // Custom design-token enforcement rules (ENFORCE-03)
  {
    rules: {
      // No gray-* Tailwind classes (use slate-* instead per design system)
      'no-restricted-syntax': [
        'error',
        {
          selector: 'JSXAttribute[name.name="className"] Literal[value=/\\bgray-/]',
          message: 'Use slate-* instead of gray-* (design system uses slate scale).',
        },
        {
          selector: 'JSXAttribute[name.name="className"] TemplateLiteral',
          message: 'Do not concatenate className strings. Use clsx() or CVA variants instead.',
        },
        {
          selector: 'CallExpression[callee.name="clsx"] > BinaryExpression',
          message: 'Do not use string concatenation inside clsx(). Use object or array syntax.',
        },
      ],
      // focus-visible enforcement: disallow bare focus: without focus-visible:
      // NOTE: This rule is complex to express in AST selectors.
      // Simpler approach: use a regex-based Literal check for the class string.
      // Full implementation detail: see Code Examples section.
    },
  },

  // Prettier must be last — disables all formatting rules
  prettier,

  // Ignore patterns (equivalent to old .eslintignore)
  globalIgnores([
    '.next/**',
    'out/**',
    'build/**',
    'next-env.d.ts',
    'node_modules/**',
  ]),
])
```

### Pattern 2: Prettier Configuration

**What:** `.prettierrc` with Tailwind plugin pointing to globals.css for v4 support.
**When to use:** Always — Prettier needs explicit config for v4 `tailwindStylesheet`.

```json
// Source: https://github.com/tailwindlabs/prettier-plugin-tailwindcss
// frontend/.prettierrc
{
  "semi": false,
  "singleQuote": true,
  "trailingComma": "es5",
  "printWidth": 100,
  "tabWidth": 2,
  "plugins": ["prettier-plugin-tailwindcss"],
  "tailwindStylesheet": "./src/app/globals.css"
}
```

### Pattern 3: Husky v9 + lint-staged

**What:** Husky v9 pre-commit hook running lint-staged (staged files) then tsc (full project).
**When to use:** Pre-commit enforcement — blocks violations before they reach CI.

```bash
# Initialize Husky (from frontend/ directory)
npx husky init
# This creates .husky/pre-commit and adds "prepare": "husky" to package.json
```

```sh
# .husky/pre-commit (created by husky init, then edited)
cd frontend
npx lint-staged
npx tsc --noEmit
```

```javascript
// Source: https://nextjs.org/docs/app/api-reference/config/eslint#running-lint-on-staged-files
// frontend/.lintstagedrc.js
const path = require('path')

const buildEslintCommand = (filenames) =>
  `eslint --fix ${filenames.map((f) => `"${path.relative(process.cwd(), f)}"`).join(' ')}`

module.exports = {
  '*.{js,jsx,ts,tsx}': [buildEslintCommand, 'prettier --write'],
  '*.{json,css,md}': ['prettier --write'],
}
```

### Pattern 4: GitHub Actions frontend-quality.yml

**What:** Separate CI workflow for frontend quality gates, independent of Docker build workflow.
**When to use:** On all PRs targeting main that touch `frontend/**`.

```yaml
# .github/workflows/frontend-quality.yml
name: Frontend Quality Gates

on:
  pull_request:
    branches: [main]
    paths: ['frontend/**']

jobs:
  quality:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: frontend

    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0  # Required for Chromatic baseline tracking

      - uses: actions/setup-node@v6
        with:
          node-version: 24
          cache: 'npm'
          cache-dependency-path: frontend/package-lock.json

      - name: Install dependencies
        run: npm ci

      - name: ESLint
        run: npx eslint .

      - name: Prettier check
        run: npx prettier --check .

      - name: TypeScript check
        run: npx tsc --noEmit

      - name: Build (for bundle analysis)
        run: npm run build
        env:
          ANALYZE: 'false'  # Don't open browser; still generates stats

      - name: Bundle size analysis
        uses: hashicorp/nextjs-bundle-analysis@v1
        with:
          working-directory: frontend

      - name: Storybook tests (a11y + performance)
        run: npx vitest --project storybook --run

      - name: Chromatic visual regression
        uses: chromaui/action@latest
        with:
          projectToken: ${{ secrets.CHROMATIC_PROJECT_TOKEN }}
          workingDir: frontend
          exitZeroOnChanges: false      # Block PR until visual changes reviewed
          autoAcceptChanges: "main"     # Auto-accept on main branch merges
```

### Pattern 5: Bundle Size Gate

**What:** `@next/bundle-analyzer` for local inspection + `hashicorp/nextjs-bundle-analysis` for PR delta comments + 20KB gate enforcement.
**When to use:** Both configured together — analyzer for developer workflow, action for CI.

```javascript
// next.config.js — wrap with bundle analyzer
const withBundleAnalyzer = require('@next/bundle-analyzer')({
  enabled: process.env.ANALYZE === 'true',
})

module.exports = withBundleAnalyzer({
  // ... existing nextConfig
})
```

The 20KB gate is enforced by `hashicorp/nextjs-bundle-analysis` via its `minimumChangeThreshold` configuration in `package.json`:

```json
// package.json — add under "nextBundleAnalysis" key
"nextBundleAnalysis": {
  "budget": 20480,
  "budgetPercentIncreaseRed": 20,
  "minimumChangeThreshold": 0,
  "showDetails": true
}
```

The action automatically fails the CI step if the bundle increases beyond the configured budget, satisfying the >20KB justification requirement.

### Anti-Patterns to Avoid

- **Do NOT keep `.eslintrc.json`:** ESLint v10 ignores it silently. Rename/delete it.
- **Do NOT use `eslint-plugin-tailwindcss` `no-contradicting-classname` with Tailwind v4:** Known false positives — disable this specific rule.
- **Do NOT run `tsc` inside lint-staged:** TypeScript type checking requires the full project context; running it per-file produces incorrect results. Run `tsc --noEmit` on the full project after lint-staged.
- **Do NOT use `--incremental` with `tsc --noEmit` in pre-commit:** The `--incremental` flag writes to `.tsbuildinfo`, which can be included in git commits accidentally. Run bare `tsc --noEmit`.
- **Do NOT set `exitZeroOnChanges: true` in Chromatic CI config:** This defeats the purpose of visual regression — Chromatic would always succeed regardless of visual changes.
- **Do NOT put Husky in the root `package.json`:** The frontend is in `frontend/` subdirectory. Husky must be initialized inside `frontend/` with the `.husky/` directory there. The `prepare` script in `frontend/package.json` is correct.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Tailwind class ordering | Custom sort script | `prettier-plugin-tailwindcss` | Official Tailwind Labs plugin; handles responsive/modifier prefixes correctly |
| Bundle size diff on PRs | Shell script comparing build output | `hashicorp/nextjs-bundle-analysis` | Handles Next.js page-level granularity, PR comments, baseline JSON management |
| Visual regression diffing | Manual screenshot comparison | Chromatic (`chromaui/action`) | Pixel-level diffing, review UI, baseline management, Storybook integration already present |
| Pre-commit hook management | Raw `.git/hooks/pre-commit` shell script | Husky v9 | Version-controlled, team-shareable, works with `npm ci` via `prepare` script |
| className string concat detection | Regex scanning | ESLint `no-restricted-syntax` AST selectors | AST-based detection handles multiline, nested expressions; regex has false positives |

**Key insight:** All five enforcement problems have solved solutions. The risk in this phase is misconfiguration (wrong ESLint config format, Tailwind v4 plugin false positives), not missing features.

---

## Common Pitfalls

### Pitfall 1: .eslintrc.json Still Present After Adding eslint.config.mjs

**What goes wrong:** ESLint v10 ignores `.eslintrc.json` entirely without error. If both files exist, only `eslint.config.mjs` is used — but developers may not notice the old file is dead.
**Why it happens:** ESLint v10 changed the discovery algorithm; it no longer falls back.
**How to avoid:** Delete `frontend/.eslintrc.json` when creating `frontend/eslint.config.mjs`. Do not keep both.
**Warning signs:** `npx eslint --inspect-config` shows config from `eslint.config.mjs` only.

### Pitfall 2: eslint-plugin-tailwindcss False Positives with Tailwind v4

**What goes wrong:** The `no-contradicting-classname` rule incorrectly flags valid Tailwind v4 utility classes as contradicting. CI fails on legitimate code.
**Why it happens:** Plugin has partial v4 support; class resolution logic still uses v3 patterns in places.
**How to avoid:** Explicitly disable `tailwindcss/no-contradicting-classname: 'off'` in the flat config. The classnames-order rule is safer and worth keeping.
**Warning signs:** ESLint errors on classes like `bg-linear-to-r` or custom `@theme` tokens.

### Pitfall 3: prettier-plugin-tailwindcss Doesn't Know About Custom Tokens

**What goes wrong:** Prettier plugin cannot sort custom `@theme` classes (like `text-twitch`, `border-l-youtube`) because it doesn't know about them.
**Why it happens:** Plugin reads Tailwind config to determine class order; with v4 CSS config it needs `tailwindStylesheet` option.
**How to avoid:** Set `"tailwindStylesheet": "./src/app/globals.css"` in `.prettierrc`. This is the v4-specific config option.
**Warning signs:** Custom token classes sort to the end or get flagged as unknown.

### Pitfall 4: Husky Pre-commit Runs from Repo Root, Not frontend/

**What goes wrong:** `npx eslint .` from repo root finds no config and lints Go files or nothing.
**Why it happens:** Git hooks execute from the repository root directory, not the subdirectory.
**How to avoid:** The `.husky/pre-commit` script must `cd frontend` before running any frontend commands, OR use absolute paths. Place `.husky/` inside `frontend/` and configure `prepare` in `frontend/package.json`.
**Warning signs:** Pre-commit hook exits 0 without running anything.

### Pitfall 5: Chromatic First Run Has No Baseline

**What goes wrong:** First Chromatic CI run fails with "no baseline to compare against" — this is expected behavior, not a bug.
**Why it happens:** Chromatic needs a committed baseline from the default branch before it can diff PRs.
**How to avoid:** The first time `frontend-quality.yml` runs on `main` (after merge of the Phase 26 setup PR), Chromatic will establish the baseline. Document this in the plan as an expected one-time failure.
**Warning signs:** Chromatic error message explicitly says "no baseline".

### Pitfall 6: focus-visible Rule is Hard to Express as AST Selector

**What goes wrong:** Attempting to write an AST selector that catches `focus:` classes while allowing `focus-visible:` is complex and error-prone in `no-restricted-syntax`.
**Why it happens:** Class names are strings inside JSX attributes — string pattern matching via AST selectors requires matching against `Literal` values.
**How to avoid:** Use a regex-pattern Literal selector:
```javascript
{
  selector: 'JSXAttribute[name.name="className"] Literal[value=/(?<![\\w-])focus:/]',
  message: 'Use focus-visible: instead of focus: for keyboard navigation accessibility.'
}
```
The negative lookbehind `(?<![\\w-])` prevents matching `focus-visible:` and `focus-within:`.
**Warning signs:** Rule fires on `focus-visible:` classes or doesn't fire on bare `focus:` classes.

---

## Code Examples

Verified patterns from official sources:

### ESLint Flat Config for Next.js 16 (Official)
```javascript
// Source: https://nextjs.org/docs/app/api-reference/config/eslint
import { defineConfig, globalIgnores } from 'eslint/config'
import nextVitals from 'eslint-config-next/core-web-vitals'
import prettier from 'eslint-config-prettier/flat'

export default defineConfig([
  ...nextVitals,
  prettier,
  globalIgnores(['.next/**', 'out/**', 'build/**', 'next-env.d.ts']),
])
```

### lint-staged with ESLint for Next.js (Official)
```javascript
// Source: https://nextjs.org/docs/app/api-reference/config/eslint#running-lint-on-staged-files
const path = require('path')
const buildEslintCommand = (filenames) =>
  `eslint --fix ${filenames.map((f) => `"${path.relative(process.cwd(), f)}"`).join(' ')}`

module.exports = {
  '*.{js,jsx,ts,tsx}': [buildEslintCommand],
}
```

### Chromatic GitHub Action (Official)
```yaml
# Source: https://www.chromatic.com/docs/github-actions/
- name: Run Chromatic
  uses: chromaui/action@latest
  with:
    projectToken: ${{ secrets.CHROMATIC_PROJECT_TOKEN }}
    exitZeroOnChanges: false    # Block PR until changes reviewed
    autoAcceptChanges: "main"   # Auto-accept on main branch
```

### Storybook Interaction Test for Render Performance
```typescript
// In an existing story file, e.g. src/stories/ChatMessage.stories.tsx
import { expect } from '@storybook/test'

export const PerformanceTest: Story = {
  play: async ({ canvasElement }) => {
    const MESSAGES = 20
    const start = performance.now()

    for (let i = 0; i < MESSAGES; i++) {
      // Force re-render by updating props — implementation depends on story setup
      // Use canvas interactions to simulate rapid message arrival
    }

    const elapsed = performance.now() - start
    const perMessage = elapsed / MESSAGES

    // <16ms per message at 20 msg/sec
    expect(perMessage).toBeLessThan(16)
  },
}
```

### @next/bundle-analyzer Setup
```javascript
// Source: https://nextjs.org/docs/app/guides/package-bundling
// next.config.js
const withBundleAnalyzer = require('@next/bundle-analyzer')({
  enabled: process.env.ANALYZE === 'true',
})
module.exports = withBundleAnalyzer(nextConfig)
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `.eslintrc.json` | `eslint.config.mjs` with `defineConfig()` | ESLint v9 (April 2024), mandatory in v10 (Feb 2026) | BREAKING: old config silently ignored in v10 |
| `next lint` CLI command | `eslint .` CLI | Next.js v16.0.0 | `next lint` removed from Next.js 16; use `npx eslint .` directly |
| `husky install` / `husky add` | `husky init` (v9) | Husky v9 (2024) | `husky add` deprecated; `husky init` creates `.husky/` and prepare script |
| `eslint-plugin-tailwindcss` stable | `eslint-plugin-tailwindcss` beta for v4 | Tailwind v4 release (2025) | Stable does not support v4; beta has partial support with known false positives |

**Deprecated/outdated:**
- `next lint` command: removed in Next.js 16. Replace with `npx eslint .` in all scripts and CI.
- `.eslintrc.json`: no longer honored in ESLint v10. Must migrate to flat config.
- `husky add` command: deprecated in Husky v9. Use manual file creation after `husky init`.
- `"prepare": "husky install"`: the old v8 prepare script. v9 uses `"prepare": "husky"`.

---

## Open Questions

1. **eslint-plugin-storybook flat config support**
   - What we know: `eslint-plugin-storybook` is installed at `^10.2.17`. The old config used `plugin:storybook/recommended`.
   - What's unclear: Whether `eslint-plugin-storybook` v10 exports a flat config. If not, the `legacy-recommended` compat wrapper is needed.
   - Recommendation: During implementation, check `require('eslint-plugin-storybook').configs` for a `flat/recommended` export. If absent, use ESLint's `FlatCompat` utility class to wrap the legacy config.

2. **`no-restricted-syntax` rule coverage for all gray-* patterns**
   - What we know: The selector `Literal[value=/\\bgray-/]` catches string literals in className JSX props.
   - What's unclear: Whether classes inside `clsx()`, `cn()`, or CVA `variants` objects are caught by a JSX-scoped selector.
   - Recommendation: Add a second selector scoped to `CallExpression[callee.name=/^(clsx|cn|cva)$/]` to catch utility function arguments as well.

3. **hashicorp/nextjs-bundle-analysis compatibility with Next.js 16 Turbopack**
   - What we know: Next.js 16.1 introduced a new experimental Turbopack bundle analyzer. The `hashicorp/nextjs-bundle-analysis` action uses the webpack stats JSON output.
   - What's unclear: Whether the action produces correct output when `next build` uses Turbopack by default in Next.js 16.
   - Recommendation: Keep `TURBOPACK=false` or use `next build --no-turbopack` in the quality CI step until this is verified. The classic webpack analyzer is stable.

---

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Vitest 4.x + `@storybook/addon-vitest` 10.x |
| Config file | `frontend/vitest.config.ts` (exists) |
| Quick run command | `cd frontend && npx vitest --project unit --run` |
| Full suite command | `cd frontend && npx vitest --run` (runs unit + storybook projects) |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| ENFORCE-01 | ESLint runs without error on all frontend files | smoke | `cd frontend && npx eslint .` | ❌ Wave 0: `eslint.config.mjs` |
| ENFORCE-02 | Prettier check passes on all frontend files | smoke | `cd frontend && npx prettier --check .` | ❌ Wave 0: `.prettierrc` |
| ENFORCE-03 | ESLint catches gray-*, string concat, bare focus: | unit | `cd frontend && npx eslint src/stories/ --rule 'no-restricted-syntax: error'` | ❌ Wave 0: config in `eslint.config.mjs` |
| ENFORCE-04 | Pre-commit hook blocks on violations | manual-only | Manual: stage a file with ESLint error, attempt `git commit` | ❌ Wave 0: `.husky/pre-commit` |
| ENFORCE-05 | Bundle size delta reported on PR | manual-only | Manual: create PR, verify `nextjs-bundle-analysis` bot comment appears | ❌ Wave 0: `frontend-quality.yml` |
| ENFORCE-06 | Chromatic snapshots all stories | manual-only | Manual: PR against main with story change, verify Chromatic blocks | ❌ Wave 0: `frontend-quality.yml` + Chromatic token |
| ENFORCE-07 | Migration guide exists and is accurate | manual-only | Manual: read `MARKETPLACE_MIGRATION_GUIDE.md`, verify class names match events.css | ❌ Wave 0: doc file |
| ENFORCE-08 | ChatMessage renders <16ms per message | unit | `cd frontend && npx vitest --project storybook --run` | ❌ Wave 0: performance story |
| ENFORCE-09 | Bundle analyzer script works | smoke | `cd frontend && ANALYZE=true npm run build` | ❌ Wave 0: `@next/bundle-analyzer` in next.config.js |
| ENFORCE-10 | A11y violations fail Storybook tests | unit | `cd frontend && npx vitest --project storybook --run` | ✅ Already: `preview.ts` has `a11y.test: 'error'` |

### Sampling Rate
- **Per task commit:** `cd frontend && npx eslint . && npx prettier --check .`
- **Per wave merge:** `cd frontend && npx vitest --run`
- **Phase gate:** Full suite green before `/gsd:verify-work`

### Wave 0 Gaps
- [ ] `frontend/eslint.config.mjs` — replaces `.eslintrc.json`; needed before any ESLint tasks
- [ ] `frontend/.prettierrc` — needed before Prettier tasks
- [ ] `frontend/.husky/pre-commit` — needed before hook verification
- [ ] `frontend/.lintstagedrc.js` — needed with Husky setup
- [ ] `.github/workflows/frontend-quality.yml` — CI workflow
- [ ] Performance interaction test in existing story file (e.g., `ChatMessage.stories.tsx` or new `Performance.stories.tsx`)
- [ ] `frontend/src/styles/MARKETPLACE_MIGRATION_GUIDE.md` — documentation task

---

## Sources

### Primary (HIGH confidence)
- Next.js 16.1 official ESLint docs (https://nextjs.org/docs/app/api-reference/config/eslint) — flat config format, `defineConfig()`, `next lint` removal, lint-staged integration
- Chromatic official GitHub Actions docs (https://www.chromatic.com/docs/github-actions/) — `exitZeroOnChanges`, `autoAcceptChanges`, token setup
- ESLint v10.0.0 release notes (https://eslint.org/blog/2026/02/eslint-v10.0.0-released/) — `.eslintrc.*` removal confirmation
- Husky official docs (https://typicode.github.io/husky/) — `husky init` v9 pattern
- Next.js bundle analyzer docs (https://nextjs.org/docs/app/guides/package-bundling) — `@next/bundle-analyzer` setup
- existing `frontend/vitest.config.ts`, `frontend/.storybook/preview.ts`, `frontend/.storybook/main.ts` — confirmed current infrastructure

### Secondary (MEDIUM confidence)
- prettier-plugin-tailwindcss GitHub (https://github.com/tailwindlabs/prettier-plugin-tailwindcss) — `tailwindStylesheet` v4 config option; v0.6.x supports v4
- hashicorp/nextjs-bundle-analysis GitHub (https://github.com/hashicorp/nextjs-bundle-analysis) — PR comment format, `budget` configuration
- eslint-plugin-tailwindcss GitHub issue #325 (https://github.com/francoismassart/eslint-plugin-tailwindcss/issues/325) — Tailwind v4 support status

### Tertiary (LOW confidence)
- WebSearch finding: `eslint-plugin-better-tailwindcss` as alternative — not deeply verified; project-level recommendation is to stay with `eslint-plugin-tailwindcss` beta given smaller ecosystem of alternative
- WebSearch finding: `eslint-plugin-storybook` flat config availability — not verified; flagged as Open Question

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — official docs confirm all major packages; versions verified against existing package.json
- Architecture: HIGH — ESLint v10 flat config requirement is documented in official Next.js 16 and ESLint v10 release docs; patterns code-verified against existing codebase
- Pitfalls: HIGH for ESLint/Next.js pitfalls (verified by official docs); MEDIUM for Tailwind plugin false positives (verified by GitHub issues, not official docs)

**Research date:** 2026-03-14
**Valid until:** 2026-04-14 (30 days — tooling versions stable; Tailwind v4 ESLint plugin may improve)
