import { defineConfig, globalIgnores } from 'eslint/config'
import nextVitals from 'eslint-config-next/core-web-vitals'
import prettier from 'eslint-config-prettier/flat'
import tailwind from 'eslint-plugin-tailwindcss'
import tsParser from '@typescript-eslint/parser'
import { fileURLToPath } from 'url'
import { dirname, join } from 'path'

const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)

// eslint-plugin-storybook: check if it exports flat config
// If configs['flat/recommended'] exists, use it; otherwise use FlatCompat wrapper
let storybookConfig = []
try {
  const storybook = await import('eslint-plugin-storybook')
  if (storybook.default?.configs?.['flat/recommended']) {
    storybookConfig = storybook.default.configs['flat/recommended']
  }
} catch {
  // Plugin not available or doesn't support flat config
}

// ENFORCE-03 — design-system rules shared by the configs below.
//
// Rule 1 previously read "use slate-* instead of gray-*". That advice was
// wrong: `slate-*` is not part of this design system either. The token system
// lives in src/app/globals.css (`bg-surface`, `text-text-sub`, …) and both
// scales are equally off it. See frontend/DESIGN_SYSTEM.md.
const DESIGN_SYSTEM_RULES = [
  {
    selector: 'JSXAttribute[name.name="className"] Literal[value=/\\b(gray|slate)-[0-9]/]',
    message:
      'Use a design token (bg-surface, text-text-sub, border-border) instead of gray-*/slate-*. See frontend/DESIGN_SYSTEM.md.',
  },
  {
    selector:
      'CallExpression[callee.name=/^(clsx|cn|cva)$/] Literal[value=/\\b(gray|slate)-[0-9]/]',
    message:
      'Use a design token instead of gray-*/slate-* inside utility functions. See frontend/DESIGN_SYSTEM.md.',
  },
  {
    selector: 'JSXAttribute[name.name="className"] TemplateLiteral',
    message:
      'Do not concatenate className strings. Use cn() or a CVA variant instead. See frontend/DESIGN_SYSTEM.md.',
  },
  {
    // Negative lookbehind prevents matching focus-visible: and focus-within:.
    selector: 'JSXAttribute[name.name="className"] Literal[value=/(?<![\\w-])focus:/]',
    message:
      'Use focus-visible: instead of focus: for keyboard navigation accessibility. See frontend/DESIGN_SYSTEM.md.',
  },
]

export default defineConfig([
  // Next.js core-web-vitals rules (react, react-hooks, @next/next)
  ...nextVitals,

  // Override parser for all files to @typescript-eslint/parser.
  // eslint-config-next defaults .js/.mjs to a bundled babel parser whose
  // scope manager is missing addGlobals() required by ESLint v10.
  // @typescript-eslint/parser handles both JS and TS/TSX files correctly.
  {
    files: ['**/*.{js,mjs,cjs,ts,tsx,mts,cts}'],
    languageOptions: {
      parser: tsParser,
      parserOptions: {
        sourceType: 'module',
        ecmaVersion: 'latest',
        ecmaFeatures: { jsx: true },
      },
    },
  },

  // Storybook rules (if flat config available)
  ...storybookConfig,

  // Override react version detection — eslint-plugin-react@7.x uses context.getFilename()
  // which was removed in ESLint v10. Pinning to explicit version bypasses detection entirely.
  {
    settings: {
      react: {
        version: '19',
      },
    },
  },

  // Tailwind plugin (beta, v4 partial support)
  // classnames-order is disabled — Prettier (prettier-plugin-tailwindcss) handles class ordering.
  // no-contradicting-classname disabled (false positives with v4 custom tokens).
  // getSourceCode() used by classnames-order is deprecated in ESLint v10 — disable to avoid crash.
  // v4.0.4 exports a single flat config object (configs['flat/recommended'] was removed)
  tailwind.configs.recommended,
  {
    rules: {
      // Disabled — Prettier (prettier-plugin-tailwindcss) handles class ordering
      'tailwindcss/classnames-order': 'off',
      // Disabled — uses context.getSourceCode() removed in ESLint v10
      'tailwindcss/enforces-shorthand': 'off',
      // Disabled — uses context.getSourceCode() removed in ESLint v10
      'tailwindcss/migration-from-tailwind-2': 'off',
      // Disabled — false positives with v4 custom @theme tokens
      'tailwindcss/no-contradicting-classname': 'off',
      // Disabled — plugin doesn't recognize custom @theme tokens (text-destructive,
      // border-primary, text-foreground, platform-badge, etc.) defined in globals.css
      'tailwindcss/no-custom-classname': 'off',
    },
    settings: {
      tailwindcss: {
        // Point to Tailwind v4 CSS entry point (not tailwind.config.js)
        // Must be absolute path — plugin runs as worker from a different cwd
        // v4.0.4 renamed the setting from `config` to `cssConfigPath`
        cssConfigPath: join(__dirname, 'src/app/globals.css'),
      },
    },
  },

  // Custom design-token enforcement rules (ENFORCE-03)
  // All rules are 'error' — no warnings. Violations block commits and CI.
  // The design system they enforce: frontend/DESIGN_SYSTEM.md.
  {
    rules: {
      'no-restricted-syntax': ['error', ...DESIGN_SYSTEM_RULES],
    },
  },

  // Vendored shadcn registry primitives.
  //
  // These files are copied verbatim from the registry so `npx shadcn@4 diff`
  // stays meaningful (ADR-0056), and the registry writes `focus:` in the two
  // places where it is the CORRECT selector, not a mistake:
  //   - listbox/menu items (select, dropdown), which Base UI moves DOM focus
  //     onto for pointer highlighting too — `focus-visible:` would delete the
  //     hover highlight entirely;
  //   - `focus:z-10` in joined groups, a stacking fix that lifts the focused
  //     item so its ring is not clipped by its neighbour (and which already
  //     sits next to a `focus-visible:z-10`).
  // Exempting the directory keeps the files byte-identical to upstream. The
  // rule still applies to every hand-written component, which is where the
  // mouse-click-focus bug it guards against actually happens.
  {
    files: ['src/components/ui/**/*.tsx'],
    rules: {
      'no-restricted-syntax': [
        'error',
        ...DESIGN_SYSTEM_RULES.filter((rule) => !rule.message.startsWith('Use focus-visible:')),
      ],
    },
  },

  // OBS render surfaces and theme previews.
  //
  // These files are BROADCAST ART rendered inside OBS, not app chrome:
  // user-themable, composited over arbitrary gameplay footage, and already
  // carved out of the app's design guarantees elsewhere — see the scope note in
  // src/app/__tests__/token-contrast.test.ts and the separate floor in
  // tests/e2e/theme-contrast.spec.ts.
  //
  // The app's neutral tokens (bg / surface / text) describe the CHROME's
  // palette and are the wrong vocabulary for a credits roll or a chat bubble
  // that has to read over someone's stream. The other ENFORCE-03 rules
  // (no template classNames, focus-visible) still apply here.
  //
  // LISTED INDIVIDUALLY, not as `src/app/overlay/**`. ADR-0056 originally
  // excluded the whole directory, which was wrong: only four of its six routes
  // render into a browser source. `/overlay/[id]/view` is the streamer's
  // MONITOR page (chat plus moderation controls) and `/overlay/[id]/participate`
  // is a viewer-facing web page — both are ordinary app chrome that was being
  // let out of the design system by an over-broad glob. Add a route here only
  // if it actually renders transparent into OBS.
  {
    files: [
      // `*` rather than the literal `[id]`: square brackets are a glob
      // CHARACTER CLASS, so 'src/app/overlay/[id]/page.tsx' matches
      // `overlay/i/page.tsx` and `overlay/d/page.tsx` and never the real
      // route — an exclusion that silently excludes nothing. (Caught here by
      // the rule firing on files it was meant to skip; there is one dynamic
      // segment under overlay/, so `*` is exact.)
      'src/app/overlay/*/page.tsx',
      'src/app/overlay/*/credits/page.tsx',
      'src/app/overlay/*/poll/page.tsx',
      'src/app/overlay/*/prediction/page.tsx',
      'src/components/overlay/EventContent.tsx',
      'src/app/overlays/**/preview/embed/*.tsx',
      'src/components/theme-marketplace/ThemePreview.tsx',
    ],
    rules: {
      'no-restricted-syntax': [
        'error',
        ...DESIGN_SYSTEM_RULES.filter((rule) => !rule.message.includes('gray-*/slate-*')),
      ],
    },
  },

  // Prettier must be LAST — disables all ESLint formatting rules that conflict with Prettier
  prettier,

  // Ignore patterns
  globalIgnores([
    '.next/**',
    'out/**',
    'build/**',
    'next-env.d.ts',
    'node_modules/**',
    'storybook-static/**',
    // Self-hosted Monaco vendor bundle (ADR-0040), copied into public/ by the
    // `copy:monaco` pre-dev/pre-build step and gitignored. Minified upstream
    // output — linting it only ever yields false positives (react-hooks
    // rules-of-hooks fires on mangled identifiers like `J.useCaseSensitive…`).
    'public/monaco/**',
  ]),
])
