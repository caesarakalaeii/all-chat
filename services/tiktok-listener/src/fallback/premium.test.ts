/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program. If not, see <https://www.gnu.org/licenses/>.
 */

import { describe, expect, it, vi } from 'vitest';
import { PremiumChecker } from './premium.js';

// The check is a SQL join over overlay_chat_sources/overlays/users; the
// pool is stubbed so the tests assert the query contract (params, fail-
// closed) and the TTL cache, not PostgreSQL itself. The SQL runs against
// lab-pg in the lab verification step.

type QueryCall = { text: string; values: unknown[] };

/** The checker only calls db.query; a stub of exactly that, not a full Pool. */
interface QueryPool {
  query(text: string, values?: unknown[]): Promise<{ rows: Array<{ premium: boolean }> }>;
}

function stubPool(rows: Array<{ premium: boolean }>, error?: Error): { pool: QueryPool; calls: QueryCall[] } {
  const calls: QueryCall[] = [];
  const pool: QueryPool = {
    query: async (text, values) => {
      calls.push({ text, values: values ?? [] });
      if (error) throw error;
      return { rows };
    }
  };
  return { pool, calls };
}

describe('PremiumChecker', () => {
  it('answers true when a premium overlay demands the room', async () => {
    const { pool, calls } = stubPool([{ premium: true }]);
    const checker = new PremiumChecker({ db: pool });

    await expect(checker.isPremiumRoom('somestreamer')).resolves.toBe(true);
    expect(calls).toHaveLength(1);
    // The lookup must be case-insensitive on the handle: demand payloads
    // carry whatever casing the overlay was configured with.
    expect(calls[0].values).toEqual(['somestreamer']);
    expect(calls[0].text).toContain('is_premium');
    expect(calls[0].text).toContain("platform = 'tiktok'");
    expect(calls[0].text).toContain('is_active');
  });

  it('answers false when no overlay owner is premium', async () => {
    const { pool } = stubPool([{ premium: false }]);
    const checker = new PremiumChecker({ db: pool });
    await expect(checker.isPremiumRoom('somestreamer')).resolves.toBe(false);
  });

  it('fails closed on a database error', async () => {
    const { pool } = stubPool([], new Error('db down'));
    const checker = new PremiumChecker({ db: pool });
    await expect(checker.isPremiumRoom('somestreamer')).resolves.toBe(false);
  });

  it('caches answers within the TTL and re-queries after it', async () => {
    vi.useFakeTimers();
    try {
      const { pool, calls } = stubPool([{ premium: true }]);
      const checker = new PremiumChecker({ db: pool, ttlMs: 1000 });

      await checker.isPremiumRoom('somestreamer');
      await checker.isPremiumRoom('somestreamer');
      expect(calls).toHaveLength(1); // second read hit the cache

      vi.advanceTimersByTime(1500);
      await checker.isPremiumRoom('somestreamer');
      expect(calls).toHaveLength(2); // TTL expired, re-queried
    } finally {
      vi.useRealTimers();
    }
  });

  it('a failed check is not cached as not-premium', async () => {
    vi.useFakeTimers();
    try {
      let fail = true;
      const calls: QueryCall[] = [];
      const pool = {
        query: async (text: string, values: unknown[]) => {
          calls.push({ text, values });
          if (fail) throw new Error('db down');
          return { rows: [{ premium: true }] };
        }
      };
      const checker = new PremiumChecker({ db: pool, ttlMs: 60_000 });

      await expect(checker.isPremiumRoom('somestreamer')).resolves.toBe(false);
      fail = false;
      // Within the TTL the check must re-run: a transient DB blip must not
      // bench a premium room for the whole TTL window.
      await expect(checker.isPremiumRoom('somestreamer')).resolves.toBe(true);
      expect(calls).toHaveLength(2);
    } finally {
      vi.useRealTimers();
    }
  });

  it('invalidate drops the cached answer', async () => {
    const { pool, calls } = stubPool([{ premium: true }]);
    const checker = new PremiumChecker({ db: pool });
    await checker.isPremiumRoom('somestreamer');
    checker.invalidate('somestreamer');
    await checker.isPremiumRoom('somestreamer');
    expect(calls).toHaveLength(2);
  });
});
