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
  ...tailwind.configs['flat/recommended'],
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
        config: join(__dirname, 'src/app/globals.css'),
      },
    },
  },

  // Custom design-token enforcement rules (ENFORCE-03)
  // All rules are 'error' — no warnings. Violations block commits and CI.
  {
    rules: {
      'no-restricted-syntax': [
        'error',
        // Rule 1: No gray-* Tailwind classes — use slate-* (design system uses slate scale)
        {
          selector:
            'JSXAttribute[name.name="className"] Literal[value=/\\bgray-/]',
          message:
            'Use slate-* instead of gray-* (design system uses slate scale). See DESIGN_SYSTEM.md.',
        },
        // Also catch gray-* inside clsx(), cn(), cva() calls
        {
          selector:
            'CallExpression[callee.name=/^(clsx|cn|cva)$/] Literal[value=/\\bgray-/]',
          message:
            'Use slate-* instead of gray-* inside utility functions (design system uses slate scale).',
        },
        // Rule 2: No template literals in className JSX props — use clsx() or CVA
        {
          selector: 'JSXAttribute[name.name="className"] TemplateLiteral',
          message:
            'Do not concatenate className strings. Use clsx() or CVA variants instead. See DESIGN_SYSTEM.md.',
        },
        // Rule 3: No bare focus: — must use focus-visible: for keyboard a11y
        // Negative lookbehind prevents matching focus-visible: and focus-within:
        {
          selector:
            'JSXAttribute[name.name="className"] Literal[value=/(?<![\\w-])focus:/]',
          message:
            'Use focus-visible: instead of focus: for keyboard navigation accessibility. See DESIGN_SYSTEM.md.',
        },
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
  ]),
])
