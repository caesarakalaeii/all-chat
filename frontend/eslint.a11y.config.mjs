/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

/**
 * Accessibility-only ESLint config — the a11y CI gate (WCAG 2.2 AA initiative).
 *
 * Deliberately standalone and MINIMAL: jsx-a11y strict + the design system's
 * focus-visible rule, nothing else. It must stay green independently of the
 * main lint debt (react-hooks errors, plugin/ESLint-version breakage in
 * eslint.config.mjs), so never add non-a11y plugins here.
 *
 * Run (glob = all .tsx under src, as in .github/workflows/frontend-a11y.yml):
 *   npx eslint -c eslint.a11y.config.mjs --suppressions-location eslint.a11y.suppressions.json src
 * Ratchet:  pre-existing violations live in eslint.a11y.suppressions.json
 *           (ESLint bulk suppressions). New violations FAIL immediately.
 *           Fixing violations leaves stale suppressions — prune them by
 *           adding --prune-suppressions to the command above.
 *           The suppressions file must only ever shrink.
 */

import { defineConfig, globalIgnores } from 'eslint/config'
import tsParser from '@typescript-eslint/parser'
import { createRequire } from 'node:module'
import { dirname } from 'node:path'

// eslint-plugin-jsx-a11y has no ESLint-10-compatible release yet (peers stop
// at ^9), so it cannot be a direct devDependency while the repo is on ESLint
// 10. eslint-config-next (core-web-vitals) bundles it by design — resolve
// that copy. TODO: switch to a direct devDependency import once upstream
// publishes ESLint 10 peers (https://github.com/jsx-eslint/eslint-plugin-jsx-a11y).
const require = createRequire(import.meta.url)
const jsxA11y = require(
  require.resolve('eslint-plugin-jsx-a11y', {
    // Start resolution from inside eslint-config-next so its nested copy is
    // found (its package.json subpath is not exported, so resolve the main
    // entry and walk from there).
    paths: [dirname(require.resolve('eslint-config-next'))],
  })
)

export default defineConfig([
  {
    files: ['src/**/*.{jsx,tsx}'],
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        sourceType: 'module',
        ecmaVersion: 'latest',
        ecmaFeatures: { jsx: true },
      },
    },
    linterOptions: {
      // Ignore ALL inline eslint comments: (a) disable-comments for rules this
      // minimal config doesn't load would error as unknown rules, and (b) the
      // a11y gate must not be silenceable inline — the suppressions file is
      // the only (shrink-only) escape hatch.
      noInlineConfig: true,
      reportUnusedDisableDirectives: 'off',
    },
    plugins: {
      'jsx-a11y': jsxA11y,
    },
    rules: {
      ...jsxA11y.flatConfigs.strict.rules,
      // Design-system rule (mirrors eslint.config.mjs Rule 3): bare focus:
      // suppresses keyboard focus indication — always use focus-visible:.
      'no-restricted-syntax': [
        'error',
        {
          selector: 'JSXAttribute[name.name="className"] Literal[value=/(?<![\\w-])focus:/]',
          message:
            'Use focus-visible: instead of focus: for keyboard navigation accessibility. See DESIGN_SYSTEM.md.',
        },
      ],
    },
  },

  globalIgnores([
    '.next/**',
    'out/**',
    'build/**',
    'next-env.d.ts',
    'node_modules/**',
    'storybook-static/**',
    // Storybook stories are exercised by the Storybook axe gate instead.
    'src/stories/**',
  ]),
])
