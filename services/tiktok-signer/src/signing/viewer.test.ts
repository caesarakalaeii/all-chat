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

import { describe, expect, it } from 'vitest';
import { ViewerPool } from './viewer.js';

describe('ViewerPool (no browser)', () => {
  it('rejects a capture when the browser cannot launch, without throwing sync', async () => {
    // A nonexistent executable path makes ensureBrowser fail; captureRoom
    // must surface that as a rejection, not hang.
    const pool = new ViewerPool({
      executablePath: '/nonexistent/chromium',
      userDataDir: '/tmp/viewer-test-profile',
      display: ':99'
    });
    await expect(pool.captureRoom('nobody', { timeoutMs: 5000 })).rejects.toThrow();
    await pool.close();
  });

  it('evicts idle tabs on the next capture call', async () => {
    const pool = new ViewerPool({
      executablePath: '/nonexistent/chromium',
      userDataDir: '/tmp/viewer-test-profile',
      tabIdleMs: 1
    });
    // Seed a fake tab entry whose lastUsed is in the past.
    const tabs = (
      pool as unknown as { tabs: Map<string, { page: { close(): Promise<void> } | null; lastUsed: number }> }
    ).tabs;
    let closed = false;
    tabs.set('stale-user', {
      page: { close: () => Promise.resolve().then(() => (closed = true)) },
      lastUsed: Date.now() - 10_000
    });

    // captureRoom calls evictIdleTabs before anything else; the browser
    // launch fails afterwards, but eviction has already run.
    await expect(pool.captureRoom('other-user', { timeoutMs: 2000 })).rejects.toThrow();
    expect(closed).toBe(true);
    expect(tabs.has('stale-user')).toBe(false);
    await pool.close();
  });
});
