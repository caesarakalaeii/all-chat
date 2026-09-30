import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    globals: true,
    environment: 'node',
    // The compiled tests in dist/ duplicate src/ and go stale the moment
    // src changes; vitest's default include pattern picks them up and
    // runs ghost tests against old code.
    exclude: ['**/node_modules/**', '**/dist/**'],
  },
});
