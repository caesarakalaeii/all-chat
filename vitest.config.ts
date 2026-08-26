/**
 * Root vitest config, so `npx vitest --project unit --run frontend/src/...`
 * works from the repo root as well as from inside frontend/.
 *
 * frontend/vitest.config.ts stays the real config, and frontend/node_modules the
 * real install -- frontend/scripts/link-root-toolchain.mjs symlinks the root
 * entry points at it during the frontend's postinstall. This file only
 * re-declares the "unit" project so the root invocation finds it:
 *
 *   `root` is frontend/, which is what makes bare specifiers such as
 *   `@testing-library/react` and the jsdom environment resolve inside the
 *   frontend install.
 *
 *   `include` is spelled as absolute paths under frontend/ so that the
 *   `frontend/src/...` argument a caller types at the repo root still matches.
 *   Vitest resolves a positional filename filter against the process cwd, so the
 *   two spellings have to agree.
 *
 *   `env: { NODE_ENV: 'test' }` is load-bearing. Vitest only defaults NODE_ENV
 *   to "test" when it is unset, and these containers export
 *   NODE_ENV=production. Inheriting that drops the "development" export
 *   condition, so @testing-library/react loads react-dom's production
 *   test-utils build, which has no `act`, and every renderHook test fails with
 *   "React.act is not a function". It has to be set on the project rather than
 *   as a top-level `mode`, because that is what reaches the forked workers.
 *
 * The storybook project is deliberately not duplicated here: it needs a browser
 * provider and is only ever run from frontend/ (see .github/workflows).
 */
import path from 'node:path'
import { fileURLToPath } from 'node:url'

import { defineConfig } from 'vitest/config'

const frontendDir = path.join(path.dirname(fileURLToPath(import.meta.url)), 'frontend')

export default defineConfig({
  test: {
    projects: [
      {
        test: {
          name: 'unit',
          root: frontendDir,
          environment: 'node',
          env: { NODE_ENV: 'test' },
          include: [
            path.join(frontendDir, 'src/**/__tests__/**/*.test.ts'),
            path.join(frontendDir, 'src/**/__tests__/**/*.test.tsx'),
          ],
          alias: {
            '@': path.join(frontendDir, 'src'),
          },
        },
      },
    ],
  },
})
