/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published
 * by the Free Software Foundation, either version 3 of the License, or
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
import { WarmCurator, type Discover, type IsLive } from './warm-curator.js';
import type { ViewerPool } from './viewer.js';

/** Pool double: tab registry plus capture results per room. */
function stubbedViewer(config: {
  tabs?: string[];
  canary?: string[];
  captures?: Record<string, { wsUrl: string } | { error: string }>;
}) {
  const calls = { captures: [] as string[] };
  const tabs = new Set(config.tabs ?? []);
  return {
    calls,
    viewer: {
      hasTab: (room: string) => tabs.has(room),
      async captureRoom(room: string) {
        calls.captures.push(room);
        const outcome = config.captures?.[room];
        if (!outcome) throw new Error(`no capture configured for ${room}`);
        if ('error' in outcome) throw new Error(outcome.error);
        tabs.add(room);
        return {
          roomId: '42',
          protoBase64: 'x',
          cookieHeader: '',
          userAgent: 'u',
          proxyHost: '',
          elapsedMs: 1,
          wsUrl: outcome.wsUrl
        };
      }
    } as unknown as ViewerPool
  };
}

function curatorWith(config: {
  seed?: string[];
  canary?: string[];
  live: Record<string, boolean>;
  discovered?: string[];
  viewer: ReturnType<typeof stubbedViewer>['viewer'];
  minCover?: number;
}) {
  const isLive: IsLive = async (room) => config.live[room] ?? false;
  const discover: Discover = async () => config.discovered;
  const warmRooms = new Set(config.seed ?? []);
  const curator = new WarmCurator({
    warmRooms,
    canaryRooms: new Set(config.canary ?? []),
    viewer: config.viewer,
    isLive,
    discover,
    minCover: config.minCover
  });
  return { curator, warmRooms };
}

describe('WarmCurator', () => {
  it('does not acquire when coverage meets the floor', async () => {
    const { viewer, calls } = stubbedViewer({ tabs: ['seeda'] });
    const { curator } = curatorWith({
      seed: ['seeda'],
      live: {},
      viewer: viewer
    });
    await curator.tick();
    expect(calls.captures).toEqual([]);
  });

  it('acquires a discovered live room via a classic capture', async () => {
    const { viewer, calls } = stubbedViewer({
      captures: { newroom: { wsUrl: 'wss://webcast.example/ws' } }
    });
    const { curator, warmRooms } = curatorWith({
      seed: ['seeda'],
      live: { seeda: false, newroom: true },
      discovered: ['newroom'],
      viewer: viewer
    });
    await curator.tick();
    expect(calls.captures).toEqual(['newroom']);
    expect(warmRooms.has('newroom')).toBe(true);
    expect(warmRooms.has('seeda')).toBe(true);
  });

  it('skips offline and canary candidates without spending captures', async () => {
    const { viewer, calls } = stubbedViewer({
      captures: { goodroom: { wsUrl: 'wss://webcast.example/ws' } }
    });
    const { curator, warmRooms } = curatorWith({
      seed: ['seeda', 'listedroom'],
      canary: ['canaryroom'],
      live: { seeda: false, listedroom: false, offroom: false, canaryroom: true, goodroom: true },
      discovered: ['offroom', 'canaryroom', 'listedroom', 'goodroom'],
      viewer: viewer
    });
    await curator.tick();
    // offroom not live, canaryroom excluded, listedroom already listed;
    // only goodroom earns a capture.
    expect(calls.captures).toEqual(['goodroom']);
    expect(warmRooms.has('goodroom')).toBe(true);
  });


  it('cooldowns a non-classic capture failure instead of retrying every cycle', async () => {
    const { viewer, calls } = stubbedViewer({
      captures: { newroom: { wsUrl: '' } }
    });
    const { curator, warmRooms } = curatorWith({
      seed: ['seeda'],
      live: { seeda: false, newroom: true },
      discovered: ['newroom'],
      viewer: viewer
    });
    await curator.tick();
    expect(warmRooms.has('newroom')).toBe(false);
    const firstCycleCaptures = calls.captures.length;
    await curator.tick();
    expect(calls.captures.length).toBe(firstCycleCaptures);
  });

  it('benches all acquisition for an hour after a capture refusal', async () => {
    const { viewer, calls } = stubbedViewer({
      captures: {
        rooma: { error: '403 Forbidden' },
        roomb: { wsUrl: 'wss://webcast.example/ws' }
      }
    });
    const { curator, warmRooms } = curatorWith({
      seed: ['seed'],
      live: { seed: false, rooma: true, roomb: true },
      discovered: ['rooma', 'roomb'],
      viewer: viewer
    });
    await curator.tick();
    // rooma refused -> bench; roomb must NOT be attempted even though it
    // is live and classic: the surface sleeps for the hour.
    expect(calls.captures).toEqual(['rooma']);
    expect(warmRooms.has('roomb')).toBe(false);
    await curator.tick();
    expect(calls.captures.length).toBe(1);
  });

  it('benches after a discovery refusal without any capture attempts', async () => {
    const { viewer, calls } = stubbedViewer({ captures: {} });
    const warmRooms = new Set(['seeda']);
    const curator = new WarmCurator({
      warmRooms,
      canaryRooms: new Set<string>(),
      viewer,
      isLive: async () => true,
      discover: async () => {
        throw new Error('HTTP 403 blocked');
      }
    });
    await curator.tick();
    expect(calls.captures).toEqual([]);
    // Benched: a later cycle must not even reach discovery's candidates.
    await curator.runAcquisitionCycle();
    expect(calls.captures).toEqual([]);
  });
});
