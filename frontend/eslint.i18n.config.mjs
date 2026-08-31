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
 * i18n-only ESLint config — the gate that keeps UI copy in the catalog.
 *
 * Deliberately standalone and MINIMAL, for the same reason
 * eslint.a11y.config.mjs is: it must stay green independently of the
 * pre-existing react-hooks debt in eslint.config.mjs (dispatch-only/dead), so
 * never add a non-i18n plugin here.
 *
 * Run (as in .github/workflows/frontend-i18n.yml):
 *   npx eslint -c eslint.i18n.config.mjs --suppressions-location eslint.i18n.suppressions.json 'src/**\/*.tsx'
 * Ratchet:  pre-existing violations live in eslint.i18n.suppressions.json
 *           (ESLint bulk suppressions). New violations FAIL immediately.
 *           Fixing violations leaves stale suppressions — prune them by adding
 *           --prune-suppressions to the command above.
 *           The suppressions file must only ever shrink.
 *
 * See docs/frontend/I18N.md for what to do about a violation, and
 * docs/adr/0055-ui-string-catalog-without-locale-routing.md for why the catalog
 * looks the way it does.
 */

import { defineConfig, globalIgnores } from 'eslint/config'
import tsParser from '@typescript-eslint/parser'
import { createRequire } from 'node:module'
import { dirname } from 'node:path'

// eslint-plugin-react is not a direct devDependency and must not become one: a
// frontend/ lockfile change has broken the Docker build before, and the
// published peer range stops at ESLint 9 while this repo is on 10.
// eslint-config-next bundles a copy by design — resolve that, exactly as
// eslint.a11y.config.mjs does for eslint-plugin-jsx-a11y.
const require = createRequire(import.meta.url)
const react = require(
  require.resolve('eslint-plugin-react', {
    // Start resolution from inside eslint-config-next so its nested copy is
    // found (its package.json subpath is not exported, so resolve the main
    // entry and walk from there).
    paths: [dirname(require.resolve('eslint-config-next'))],
  })
)

// Punctuation, separators and symbols that are not copy: a translator has
// nothing to do with them, and a key per bullet would be noise in the catalog.
// Copy, including a bare word, does not belong on this list.
const NOT_COPY = [
  '·',
  '•',
  '—',
  '–',
  '-',
  '/',
  '|',
  ':',
  ',',
  '.',
  '?',
  '!',
  '+',
  '×',
  '%',
  '(',
  ')',
  '[',
  ']',
  '#',
  '@',
  '~',
  '=',
  '>',
  '<',
  '&',
  '*',
  '$',
  '€',
  '°',
  '…',
  '↑',
  '↓',
  '←',
  '→',
  '✓',
  '✕',
  '✗',
  '★',
  '☆',
  // A decorative emoji standing alone in its own element is an icon, not copy:
  // a translator has nothing to do with a star, and a key per icon is noise.
  '⭐',
]

// The JSX attributes whose value a user reads: screen-reader labels, tooltips,
// input hints, image alternatives, form labels. Everything else a JSX string
// attribute can be — className, href, type, id, role, key, data-* — is machine
// input, which is why the props half of this gate is an explicit allow list
// rather than react/jsx-no-literals' `ignoreProps: false`. That option flags
// every string attribute, which here is thousands of hits with the real signal
// buried inside them.
const USER_VISIBLE_ATTRIBUTES = ['aria-label', 'placeholder', 'title', 'alt', 'label']

// `alt=""` is the a11y-correct marker for a decorative image, not copy with the
// text missing, so an empty value is excluded rather than routed through t().
const attributeSelector = USER_VISIBLE_ATTRIBUTES.map(
  (name) => `JSXAttribute[name.name="${name}"] > Literal[value!=""]`
).join(', ')

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
      // minimal config does not load would error as unknown rules, and (b) the
      // i18n gate must not be silenceable inline — the suppressions file is the
      // only (shrink-only) escape hatch.
      noInlineConfig: true,
      reportUnusedDisableDirectives: 'off',
    },
    plugins: {
      react,
    },
    rules: {
      // JSX text nodes: `<p>Hello</p>`. `ignoreProps` stays at its default of
      // true — props are covered by the selector below instead.
      'react/jsx-no-literals': [
        'error',
        { noStrings: true, ignoreProps: true, allowedStrings: NOT_COPY },
      ],
      // User-visible props, matched by name. Same technique as the focus: rule
      // in eslint.a11y.config.mjs.
      'no-restricted-syntax': [
        'error',
        {
          selector: attributeSelector,
          message: `A user-visible attribute (${USER_VISIBLE_ATTRIBUTES.join(', ')}) must read its text from the catalog: t('namespace.key'). See docs/frontend/I18N.md.`,
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
    // Stories and tests are developer-facing fixtures, not shipped UI: the JSX
    // they render is the input to an assertion, so routing it through the
    // catalog would mean a test that asserts a key resolves to itself. The
    // Storybook axe gate covers src/stories/** for accessibility.
    'src/stories/**',
    'src/**/__tests__/**',
  ]),
])
