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
import { existsSync, mkdirSync } from 'node:fs';
import { mkdtemp, rm } from 'node:fs/promises';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { ViewerPool, findDualListedRooms } from './viewer.js';
import { VIEWER_UA } from './identity.js';

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
  it('creates both class lanes per proxy, and only the direct pair without proxies', async () => {
    const pooled = new ViewerPool({ proxyHosts: ['h1:1', 'h2:2', 'h3:3'], userDataDir: '/tmp/x' });
    const pooledLanes = (pooled as unknown as { lanes: Map<string, unknown> }).lanes;
    // One primary and one canary lane per proxy: class routing never falls
    // back to the other class because its lane was never created.
    expect(pooledLanes.size).toBe(6);
    expect(pooledLanes.has('h1:1|primary')).toBe(true);
    expect(pooledLanes.has('h1:1|canary')).toBe(true);
    await pooled.close();

    const direct = new ViewerPool({ userDataDir: '/tmp/x' });
    const directLanes = (direct as unknown as { lanes: Map<string, unknown> }).lanes;
    expect(directLanes.size).toBe(2);
    expect(directLanes.has('|primary')).toBe(true);
    expect(directLanes.has('|canary')).toBe(true);
    await direct.close();
  });

  it('pins a room to its compound-key lane and skips benched lanes', () => {
    const pool = new ViewerPool({ proxyHosts: ['a:1', 'b:2'], userDataDir: '/tmp/x' });
    const internals = pool as unknown as {
      lanes: Map<string, { host: string; benchedUntil: number }>;
      roomLane: Map<string, string>;
      pickLane(username: string): { host: string; klass: string };
    };
    // Room captured on lane a:1 -> pinned there while the lane is healthy.
    internals.roomLane.set('streamer', 'a:1|primary');
    expect(internals.pickLane('streamer').host).toBe('a:1');

    // Bench it: the pinned room must fall to another PRIMARY lane.
    internals.lanes.get('a:1|primary')!.benchedUntil = Date.now() + 60_000;
    const fell = internals.pickLane('streamer');
    expect(fell.host).not.toBe('a:1');
    expect(fell.klass).toBe('primary');

    // Cooldown elapsed: the pinned lane serves the room again.
    internals.lanes.get('a:1|primary')!.benchedUntil = Date.now() - 1_000;
    expect(internals.pickLane('streamer').host).toBe('a:1');
  });

  it('refresh keeps surviving lanes, drops removed ones and adds new ones — both classes', async () => {
    const pool = new ViewerPool({ proxyHosts: ['a:1', 'b:2'], userDataDir: '/tmp/x' });
    const internals = pool as unknown as {
      lanes: Map<string, { host: string }>;
      roomLane: Map<string, string>;
    };
    internals.roomLane.set('streamer', 'a:1|primary');
    internals.roomLane.set('gone', 'b:2|primary');

    await pool.refreshProxies(['a:1', 'c:3']);
    expect(internals.lanes.has('a:1|primary')).toBe(true);
    expect(internals.lanes.has('a:1|canary')).toBe(true);
    expect(internals.lanes.has('b:2|primary')).toBe(false);
    expect(internals.lanes.has('b:2|canary')).toBe(false);
    expect(internals.lanes.has('c:3|primary')).toBe(true);
    expect(internals.lanes.has('c:3|canary')).toBe(true);
    // The surviving pin survived; the removed lane's pin was dropped.
    expect(internals.roomLane.get('streamer')).toBe('a:1|primary');
    expect(internals.roomLane.has('gone')).toBe(false);
    await pool.close();
  });

  it('drops the bootstrap direct lanes when proxies arrive and stores credentials', async () => {
    // Pool constructed with no proxies (webshare flow: list arrives later).
    const pool = new ViewerPool({ userDataDir: '/tmp/x' });
    const internals = pool as unknown as {
      lanes: Map<string, { host: string }>;
      options: { proxyUser?: string; proxyPass?: string };
    };
    expect(internals.lanes.has('|primary')).toBe(true);

    await pool.refreshProxies(['a:1', 'b:2'], { username: 'u', password: 'p' });
    // The direct lanes are gone once real proxies exist...
    expect(internals.lanes.has('|primary')).toBe(false);
    expect(internals.lanes.has('|canary')).toBe(false);
    expect(internals.lanes.size).toBe(4);
    // ...and the credentials are stored for lane pages to authenticate with.
    expect(internals.options.proxyUser).toBe('u');
    expect(internals.options.proxyPass).toBe('p');

    // An empty refresh keeps the direct lanes (nothing else to serve on).
    const directPool = new ViewerPool({ userDataDir: '/tmp/x' });
    await directPool.refreshProxies([], { username: 'u', password: 'p' });
    expect(((directPool as unknown as { lanes: Map<string, unknown> }).lanes).has('|primary')).toBe(true);
    await directPool.close();
    await pool.close();
  });

  it('routes canary rooms to canary-profile lanes and other rooms to primary lanes', () => {
    const pool = new ViewerPool({
      proxyHosts: ['a:1'],
      userDataDir: '/tmp/pool-primary',
      canaryProfileDir: '/tmp/pool-canary',
      canaryRooms: new Set(['canaryroom'])
    });
    const internals = pool as unknown as {
      pickLane(username: string): { profileDir: string; klass: string };
    };
    // The canary lane carries the canary profile dir; the primary lane
    // the primary one. If either launch site kept an inline template,
    // every canary browser would boot on the PRIMARY jar — the exact
    // isolation failure this routing exists to prevent.
    expect(internals.pickLane('canaryroom').profileDir).toBe('/tmp/pool-canary-a:1');
    expect(internals.pickLane('canaryroom').klass).toBe('canary');
    expect(internals.pickLane('warmroom').profileDir).toBe('/tmp/pool-primary-a:1');
    expect(internals.pickLane('warmroom').klass).toBe('primary');
  });


  it('rotateLaneProfile wipes the lane it is called on, keyed by profileDir', async () => {
    // Real temp dirs, real rm(): the rotation must wipe the CANARY lane's
    // dir and leave the primary dir of the same host alive — the inline
    // profileDir template regression (a canary rotation wiping the primary
    // jar) would leave the primary dir gone.
    const base = await mkdtemp(join(tmpdir(), 'viewer-test-'));
    const pool = new ViewerPool({
      proxyHosts: ['a:1'],
      userDataDir: join(base, 'primary'),
      canaryProfileDir: join(base, 'canary')
    });
    const internals = pool as unknown as {
      lanes: Map<string, { host: string; klass: string; profileDir: string; consecutiveFailures: number }>;
      rotateLaneProfile(lane: unknown): Promise<void>;
    };
    const canaryDir = internals.lanes.get('a:1|canary')!.profileDir;
    const primaryDir = internals.lanes.get('a:1|primary')!.profileDir;
    expect(canaryDir).toBe(join(base, 'canary-a:1'));
    expect(primaryDir).toBe(join(base, 'primary-a:1'));
    mkdirSync(canaryDir, { recursive: true });
    mkdirSync(primaryDir, { recursive: true });

    await internals.rotateLaneProfile(internals.lanes.get('a:1|canary'));
    expect(existsSync(canaryDir)).toBe(false);
    expect(existsSync(primaryDir)).toBe(true);
    await rm(base, { recursive: true, force: true });
    await pool.close();
  });
});

describe('ViewerPool session lease accessors', () => {
  it('sessionLease returns undefined with no warm tab and on a dead page', async () => {
    const pool = new ViewerPool({ userDataDir: '/tmp/x' });
    expect(await pool.sessionLease('nobody')).toBeUndefined();

    // Dead page: cookies() rejects (Target closed) — undefined, never a 500.
    const internals = pool as unknown as {
      tabs: Map<string, unknown>;
      roomLane: Map<string, string>;
    };
    const deadPage = {
      cookies: async () => {
        throw new Error('Target closed');
      },
      close: async () => undefined
    };
    internals.tabs.set('dead', {
      page: deadPage,
      lastUsed: Date.now(),
      wsUrl: 'wss://webcast-ws.example/x',
      roomId: '1',
      capturedAt: Date.now() - 5_000
    });
    internals.roomLane.set('dead', '|primary');
    expect(await pool.sessionLease('dead')).toBeUndefined();
    await pool.close();
  });

  it('sessionLease re-reads cookies at lease time and refreshes lastUsed', async () => {
    const pool = new ViewerPool({ userDataDir: '/tmp/x' });
    const internals = pool as unknown as {
      tabs: Map<string, unknown>;
      roomLane: Map<string, string>;
    };
    let cookieCalls = 0;
    const page = {
      cookies: async () => {
        cookieCalls++;
        return [
          { name: 'ttwid', value: 'fresh' },
          { name: 'msToken', value: 'abc' }
        ];
      },
      close: async () => undefined
    };
    const before = Date.now() - 60_000;
    internals.tabs.set('warm', {
      page,
      lastUsed: before,
      wsUrl: 'wss://webcast-ws.example/w',
      roomId: '7',
      capturedAt: before
    });
    internals.roomLane.set('warm', '|primary');

    const lease = await pool.sessionLease('warm');
    expect(lease).toBeDefined();
    // The jar is re-read from the page, not a capture-time cache.
    expect(cookieCalls).toBe(1);
    expect(lease!.cookieHeader).toBe('ttwid=fresh; msToken=abc');
    expect(lease!.wsUrl).toBe('wss://webcast-ws.example/w');
    expect(lease!.userAgent).toBe(VIEWER_UA);
    expect(lease!.proxyHost).toBe('');
    // lastUsed refreshed so idle eviction cannot reclaim it between leases.
    const entry = internals.tabs.get('warm') as { lastUsed: number };
    expect(entry.lastUsed).toBeGreaterThanOrEqual(before + 59_000);
    await pool.close();
  });

  it('closeTab unregisters the room so the next capture cold-captures', () => {
    const pool = new ViewerPool({ userDataDir: '/tmp/x' });
    const internals = pool as unknown as {
      tabs: Map<string, unknown>;
      roomLane: Map<string, string>;
    };
    let closed = false;
    const page = { cookies: async () => [], close: async () => {
      closed = true;
    } };
    internals.tabs.set('room', {
      page,
      lastUsed: Date.now(),
      wsUrl: 'wss://webcast-ws.example/x',
      roomId: '1',
      capturedAt: Date.now()
    });
    internals.roomLane.set('room', '|primary');

    expect(pool.hasTab('room')).toBe(true);
    expect(pool.closeTab('room')).toBe(true);
    // BOTH maps cleared: with roomLane left behind, the reuse precondition
    // stays armed and the next capture would ride a dead page.
    expect(internals.tabs.has('room')).toBe(false);
    expect(internals.roomLane.has('room')).toBe(false);
    expect(closed).toBe(true);
    expect(pool.hasTab('room')).toBe(false);
    expect(pool.closeTab('room')).toBe(false);
  });

  it('skips pinned rooms in idle eviction', async () => {
    const pool = new ViewerPool({
      userDataDir: '/tmp/x',
      tabIdleMs: 5,
      pinnedRooms: new Set(['pinnedroom'])
    });
    const internals = pool as unknown as {
      tabs: Map<string, unknown>;
      roomLane: Map<string, string>;
    };
    const pages = {
      pinned: { cookies: async () => [], close: async () => undefined },
      other: { cookies: async () => [], close: async () => undefined }
    };
    internals.tabs.set('pinnedroom', {
      page: pages.pinned,
      lastUsed: Date.now() - 60_000,
      wsUrl: '',
      roomId: '1',
      capturedAt: Date.now()
    });
    internals.roomLane.set('pinnedroom', '|primary');
    internals.tabs.set('otherroom', {
      page: pages.other,
      lastUsed: Date.now() - 60_000,
      wsUrl: '',
      roomId: '2',
      capturedAt: Date.now()
    });
    internals.roomLane.set('otherroom', '|primary');

    // captureRoom evicts before anything else; the browser launch fails
    // afterwards, but eviction has already run.
    await expect(pool.captureRoom('thirdroom', { timeoutMs: 500 })).rejects.toThrow();
    expect(internals.tabs.has('pinnedroom')).toBe(true);
    expect(internals.tabs.has('otherroom')).toBe(false);
    await pool.close();
  });

  it('findDualListedRooms returns the case-insensitive intersection', () => {
    expect(findDualListedRooms(new Set(['SomeRoom', 'other']), ['someroom', 'third'])).toEqual([
      'someroom'
    ]);
    expect(findDualListedRooms(new Set(['a']), ['b'])).toEqual([]);
  });
});
