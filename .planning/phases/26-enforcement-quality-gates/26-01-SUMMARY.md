---
phase: 26-enforcement-quality-gates
plan: 01
subsystem: frontend-tooling
tags: [eslint, prettier, design-system, enforcement, tailwindcss]
dependency_graph:
  requires: [25-page-migration-split-view-preview]
  provides: [ENFORCE-01, ENFORCE-02, ENFORCE-03]
  affects: [frontend/src/**/*.tsx, frontend/src/**/*.ts]
tech_stack:
  added:
    - prettier@3.8.1
    - prettier-plugin-tailwindcss@0.7.2
    - eslint-config-prettier@10.1.8
    - eslint-plugin-tailwindcss@4.0.0-beta.0
    - '@typescript-eslint/parser@8.57.0'
  patterns:
    - ESLint v10 flat config (eslint.config.mjs)
    - no-restricted-syntax for design-token enforcement
    - prettier-plugin-tailwindcss with tailwindStylesheet option (v4)
    - @typescript-eslint/parser overrides babel parser (ESLint v10 compat)
key_files:
  created:
    - frontend/eslint.config.mjs
    - frontend/.prettierrc
  modified:
    - frontend/package.json (added format:check, format scripts + 5 new devDeps)
    - frontend/eslint.config.mjs (3 rule groups + ESLint v10 compat fixes)
    - 43 source files (design-system violations fixed: gray->slate, focus->focus-visible, template literals->clsx, <a>-><Link>)
decisions:
  - "eslint-plugin-tailwindcss@4.0.0-beta.0 required for Tailwind v4 support (stable 3.x uses resolveConfig API removed in v4)"
  - "classnames-order, enforces-shorthand, migration-from-tailwind-2 disabled — use context.getSourceCode() removed in ESLint v10"
  - "no-custom-classname disabled — plugin cannot resolve custom @theme tokens (text-destructive, border-primary, etc.)"
  - "@typescript-eslint/parser installed as top-level dep to override bundled babel parser (babel scope manager missing addGlobals in ESLint v10)"
  - "react.version pinned to '19' in settings to bypass eslint-plugin-react@7.x getFilename() API removed in ESLint v10"
  - "Prettier class ordering replaces tailwindcss/classnames-order — prettier-plugin-tailwindcss handles ordering at write time"
  - "tailwindStylesheet option (NOT tailwindConfig) used for Tailwind v4 CSS entry point"
  - "345 pre-existing violations fixed as part of enforcement installation — gray->slate, focus->focus-visible, template literals->clsx"
metrics:
  duration: 31min
  completed: "2026-03-14"
  tasks: 2
  files_created: 2
  files_modified: 145
---

# Phase 26 Plan 01: ESLint Flat Config + Prettier Summary

ESLint v10 flat config with Next.js, Tailwind v4, and custom design-token rules; Prettier with prettier-plugin-tailwindcss ordered by globals.css @theme tokens.

## What Was Built

### Task 1: ESLint flat config + pre-existing violation fixes

Replaced the defunct `frontend/.eslintrc.json` (silently ignored by ESLint v10) with `frontend/eslint.config.mjs` containing:

1. **Next.js rules**: `eslint-config-next/core-web-vitals` (react, react-hooks, @next/next, typescript-eslint)
2. **Tailwind plugin**: `eslint-plugin-tailwindcss@4.0.0-beta.0` with classnames-order disabled (Prettier handles ordering) and incompatible ESLint v10 rules disabled
3. **Custom design-token enforcement** via `no-restricted-syntax` (all at `'error'` level, zero warnings):
   - No `gray-*` Tailwind classes (use `slate-*`)
   - No `gray-*` inside `clsx()/cn()/cva()` calls
   - No template literals in `className` JSX props (use clsx())
   - No bare `focus:` (use `focus-visible:` for keyboard a11y)
4. **Prettier last** (disables conflicting formatting rules)

**ESLint v10 compatibility required 3 workarounds:**
- `@typescript-eslint/parser` installed as top-level dep to override Next.js bundled babel parser (babel scope manager lacks `addGlobals()`)
- `settings.react.version: '19'` to bypass `eslint-plugin-react@7.x getFilename()` removal
- `tailwindcss/classnames-order`, `enforces-shorthand`, `migration-from-tailwind-2` disabled (use `getSourceCode()` removed in ESLint v10)

**345 pre-existing violations fixed across 43 source files** (auto-fixed per Deviation Rules 1-2):
- `gray-*` → `slate-*`: 243 replacements across chat, legal, admin, overlay, theme-marketplace files
- `focus:` → `focus-visible:`: 28 replacements for keyboard navigation a11y
- Template literals in className → `clsx()`: 38 conversions
- `<a href="/">` → `<Link href="/">`: 7 replacements
- `@storybook/react` → `@storybook/nextjs-vite`: 5 story files
- Various react-hooks fixes, img eslint-disable comments

### Task 2: Prettier with prettier-plugin-tailwindcss

Created `frontend/.prettierrc`:
```json
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

The `tailwindStylesheet` option (NOT `tailwindConfig`) is Tailwind v4-specific — tells the plugin to read the CSS entry point so custom `@theme` tokens (`text-twitch`, `border-l-youtube`, etc.) sort correctly.

Added `format:check` and `format` scripts to `package.json`. Ran `prettier --write .` to apply initial formatting to all 140 files.

## Verification

```bash
# Both exit 0:
cd frontend && npx eslint . --max-warnings 0  # ✓ 0 errors, 0 warnings
cd frontend && npx prettier --check .          # ✓ All matched files use Prettier code style!
```

## Deviations from Plan

### Auto-fixed Issues (Rule 1 - Bug)

**1. eslint-plugin-tailwindcss@3.18.2 incompatible with Tailwind v4**
- **Found during:** Task 1 (package install)
- **Issue:** Stable release uses `tailwindcss/resolveConfig` which doesn't exist in v4
- **Fix:** Installed `eslint-plugin-tailwindcss@4.0.0-beta.0` (explicit beta with `tailwindcss@^4.0.0` peer dep support), used `--legacy-peer-deps`
- **Commit:** 27415da

**2. ESLint v10 babel parser incompatibility (scopeManager.addGlobals)**
- **Found during:** Task 1 (first ESLint run)
- **Issue:** Next.js bundled babel parser returns scope manager without `addGlobals()` required by ESLint v10
- **Fix:** Installed `@typescript-eslint/parser@8.57.0` and added override config to use it for all JS/TS files
- **Commit:** 27415da

**3. ESLint v10 context.getFilename() removal (eslint-plugin-react@7.x)**
- **Found during:** Task 1
- **Issue:** React version detection calls deprecated `context.getFilename()` removed in ESLint v10
- **Fix:** Added `settings.react.version: '19'` to bypass auto-detection
- **Commit:** 27415da

**4. tailwindcss plugin rules use context.getSourceCode() (ESLint v10 removed)**
- **Found during:** Task 1
- **Issue:** `classnames-order`, `enforces-shorthand`, `migration-from-tailwind-2` crash on ESLint v10
- **Fix:** Disabled all three rules (Prettier handles ordering; shorthand and migration rules not critical for ENFORCE requirements)
- **Commit:** 27415da

**5. tailwindcss/no-custom-classname: false positives on design tokens**
- **Found during:** Task 1
- **Issue:** Plugin doesn't resolve custom `@theme` tokens (`text-destructive`, `border-primary`, `platform-badge`, etc.)
- **Fix:** Disabled `tailwindcss/no-custom-classname`
- **Commit:** 27415da

**6. tailwindcss plugin config path must be absolute**
- **Found during:** Task 1
- **Issue:** Worker runs from a different cwd; relative path `src/app/globals.css` can't resolve
- **Fix:** Used `join(__dirname, 'src/app/globals.css')` with ESM `__dirname` computed from `import.meta.url`
- **Commit:** 27415da

### Auto-fixed Issues (Rule 2 - Missing Critical Functionality)

**7. 345 pre-existing design-system violations**
- **Found during:** Task 1 (ESLint run revealed all violations)
- **Issue:** Installing enforcement rules immediately blocked CI with 345 errors — enforcement is only useful if it passes on the clean codebase
- **Fix:** Fixed all violations: gray→slate, focus→focus-visible, template literals→clsx, Link migrations, storybook renderer imports, react-hooks fixes
- **Files:** 43 source files
- **Commit:** 27415da

## Self-Check

### Files Exist
- FOUND: `frontend/eslint.config.mjs`
- FOUND: `frontend/.prettierrc`
- DELETED: `frontend/.eslintrc.json` (correct)

### Commits Exist
- FOUND: 27415da (feat(26-01): install ESLint flat config with design-token rules)
- FOUND: 857b28e (feat(26-01): add Prettier with prettier-plugin-tailwindcss and format all files)

## Self-Check: PASSED
