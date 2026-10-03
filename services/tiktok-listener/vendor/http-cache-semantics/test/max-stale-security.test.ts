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
 * Regression suite for GHSA-ch52-4w7c-c8xp in the vendored
 * http-cache-semantics copy (upstream advisory:
 * https://github.com/advisories/GHSA-ch52-4w7c-c8xp).
 *
 * maxAge() deliberately zeroes the freshness of a shared-cache response that
 * carries Set-Cookie unless the response opts in with `public`/`immutable`, but
 * evaluateRequest()'s max-stale shortcut used to let any client max-stale
 * directive resurrect that deliberately-zeroed entry, so one user's cached
 * Set-Cookie answered another user's request. The first three tests fail
 * against pristine 4.2.0 and pin the fix; the last three pin that the fix
 * leaves the paths where max-stale is legitimate alone.
 */

import { createRequire } from 'node:module';

import { describe, expect, it } from 'vitest';

type Headers = Record<string, string>;

type CachePolicyInstance = {
  satisfiesWithoutRevalidation(request: { headers: Headers }): boolean;
  evaluateRequest(request: { headers: Headers }): {
    response?: { headers: Headers };
    revalidation?: unknown;
  };
};

// The vendored module is CommonJS and must load through Node's CJS loader:
// vitest transforms project files as ESM, where this module's `module.exports`
// has no meaning.
const require = createRequire(import.meta.url);
const CachePolicy = require('../index.js') as new (
  request: { headers: Headers },
  response: { headers: Headers },
  options?: { shared?: boolean },
) => CachePolicyInstance;

const COOKIE = "session=another-user's-credentials";

/** A shared cache entry the policy must never reuse without revalidation. */
function zeroedSharedEntry() {
  return new CachePolicy(
    { headers: {} },
    { headers: { 'set-cookie': COOKIE, 'cache-control': 'max-age=99' } },
    { shared: true },
  );
}

function requestWith(directive: string) {
  return { headers: { 'cache-control': directive } };
}

describe('GHSA-ch52-4w7c-c8xp: max-stale must not resurrect security-zeroed entries', () => {
  it('refuses a bare max-stale request on a shared entry zeroed for Set-Cookie', () => {
    const entry = zeroedSharedEntry();

    expect(entry.satisfiesWithoutRevalidation(requestWith('max-stale'))).toBe(false);
  });

  it('refuses max-stale=999999 on a shared entry zeroed for Set-Cookie', () => {
    const entry = zeroedSharedEntry();

    expect(entry.satisfiesWithoutRevalidation(requestWith('max-stale=999999'))).toBe(false);
  });

  it('offers revalidation rather than the zeroed entry', () => {
    const entry = zeroedSharedEntry();

    const outcome = entry.evaluateRequest(requestWith('max-stale'));

    expect(outcome.response).toBeUndefined();
    expect(outcome.revalidation).toBeDefined();
  });

  it('still serves bare max-stale on an ordinary expired shared entry', () => {
    const entry = new CachePolicy(
      { headers: {} },
      { headers: { age: '100', 'cache-control': 'max-age=60' } },
      { shared: true },
    );

    expect(entry.satisfiesWithoutRevalidation(requestWith('max-stale'))).toBe(true);
  });

  it('still serves max-stale when a cookie response opts in with public', () => {
    const entry = new CachePolicy(
      { headers: {} },
      {
        headers: {
          age: '120',
          'set-cookie': COOKIE,
          'cache-control': 'max-age=99, public',
        },
      },
      { shared: true },
    );

    expect(entry.satisfiesWithoutRevalidation(requestWith('max-stale=100'))).toBe(true);
  });

  it('still serves max-stale from a private cache despite Set-Cookie', () => {
    const entry = new CachePolicy(
      { headers: {} },
      {
        headers: {
          age: '120',
          'set-cookie': COOKIE,
          'cache-control': 'max-age=99',
        },
      },
      { shared: false },
    );

    expect(entry.satisfiesWithoutRevalidation(requestWith('max-stale=100'))).toBe(true);
  });
});