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

describe('ViewerPool lane selection', () => {
  it('creates one lane per proxy and a single direct lane without proxies', async () => {
    const pooled = new ViewerPool({ proxyHosts: ['h1:1', 'h2:2', 'h3:3'], userDataDir: '/tmp/x' });
    expect(((pooled as unknown as { lanes: Map<string, unknown> }).lanes).size).toBe(3);
    await pooled.close();

    const direct = new ViewerPool({ userDataDir: '/tmp/x' });
    expect(((direct as unknown as { lanes: Map<string, unknown> }).lanes).size).toBe(1);
    expect(((direct as unknown as { lanes: Map<string, unknown> }).lanes).has('')).toBe(true);
    await direct.close();
  });

  it('pins a room to its lane and skips benched lanes', () => {
    const pool = new ViewerPool({ proxyHosts: ['a:1', 'b:2'], userDataDir: '/tmp/x' });
    const internals = pool as unknown as {
      lanes: Map<string, { host: string; benchedUntil: number }>;
      roomLane: Map<string, string>;
      pickLane(username: string): { host: string };
    };
    // Room captured on lane a:1 -> pinned there while the lane is healthy.
    internals.roomLane.set('streamer', 'a:1');
    expect(internals.pickLane('streamer').host).toBe('a:1');

    // Bench it: the pinned room must fall to another lane.
    internals.lanes.get('a:1')!.benchedUntil = Date.now() + 60_000;
    expect(internals.pickLane('streamer').host).not.toBe('a:1');

    // Cooldown elapsed: the pinned lane serves the room again.
    internals.lanes.get('a:1')!.benchedUntil = Date.now() - 1_000;
    expect(internals.pickLane('streamer').host).toBe('a:1');
  });

  it('refresh keeps surviving lanes, drops removed ones and adds new ones', async () => {
    const pool = new ViewerPool({ proxyHosts: ['a:1', 'b:2'], userDataDir: '/tmp/x' });
    const internals = pool as unknown as {
      lanes: Map<string, { host: string }>;
      roomLane: Map<string, string>;
    };
    // One room pinned to a surviving proxy, one to the lane being removed.
    internals.roomLane.set('streamer', 'a:1');
    internals.roomLane.set('gone', 'b:2');

    await pool.refreshProxies(['a:1', 'c:3']);
    expect(internals.lanes.has('a:1')).toBe(true);
    expect(internals.lanes.has('b:2')).toBe(false);
    expect(internals.lanes.has('c:3')).toBe(true);
    // The surviving pin survived; the removed lane's pin was dropped.
    expect(internals.roomLane.get('streamer')).toBe('a:1');
    expect(internals.roomLane.has('gone')).toBe(false);
    await pool.close();
  });

  it('drops the bootstrap direct lane when proxies arrive and stores credentials', async () => {
    // Pool constructed with no proxies (webshare flow: list arrives later).
    const pool = new ViewerPool({ userDataDir: '/tmp/x' });
    const internals = pool as unknown as {
      lanes: Map<string, { host: string }>;
      options: { proxyUser?: string; proxyPass?: string };
    };
    expect(internals.lanes.has('')).toBe(true);

    await pool.refreshProxies(['a:1', 'b:2'], { username: 'u', password: 'p' });
    // The direct lane is gone once real proxies exist...
    expect(internals.lanes.has('')).toBe(false);
    expect(internals.lanes.size).toBe(2);
    // ...and the credentials are stored for lane pages to authenticate with.
    expect(internals.options.proxyUser).toBe('u');
    expect(internals.options.proxyPass).toBe('p');

    // An empty refresh keeps the direct lane (nothing else to serve on).
    const directPool = new ViewerPool({ userDataDir: '/tmp/x' });
    await directPool.refreshProxies([], { username: 'u', password: 'p' });
    expect(((directPool as unknown as { lanes: Map<string, unknown> }).lanes).has('')).toBe(true);
    await directPool.close();
    await pool.close();
  });
});
