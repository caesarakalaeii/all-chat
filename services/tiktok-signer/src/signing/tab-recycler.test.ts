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
import { TabRecycler } from './tab-recycler.js';
import type { ViewerPool } from './viewer.js';

const HOUR = 3_600_000;

/** Pool double: tab ages per room, plus a recycle outcome per room. */
function recyclerWith(config: {
  ageHours: Record<string, number>;
  live?: Record<string, boolean>;
  subscribed?: string[];
  failures?: Record<string, string>;
}) {
  const recycled: string[] = [];
  const openedAt = new Map(Object.entries(config.ageHours).map(([room, h]) => [room, Date.now() - h * HOUR]));
  const viewer = {
    tabOpenedAt: (room: string) => openedAt.get(room),
    async recycleTab(room: string) {
      recycled.push(room);
      const failure = config.failures?.[room];
      if (failure) throw new Error(failure);
      openedAt.set(room, Date.now());
    }
  } as unknown as ViewerPool;
  const subscribed = new Set(config.subscribed ?? []);
  const recycler = new TabRecycler({
    pinnedRooms: new Set(Object.keys(config.ageHours)),
    viewer,
    relay: { hasSubscribers: (room) => subscribed.has(room) },
    maxAgeMs: 6 * HOUR,
    isLive: async (room) => config.live?.[room] ?? true
  });
  return { recycler, recycled };
}

describe('TabRecycler', () => {
  it('recycles only the oldest stale tab per tick and leaves young tabs alone', async () => {
    const { recycler, recycled } = recyclerWith({ ageHours: { young: 1, old: 8, oldest: 20 } });
    await recycler.tick();
    expect(recycled).toEqual(['oldest']);
    await recycler.tick();
    expect(recycled).toEqual(['oldest', 'old']);
    await recycler.tick();
    expect(recycled).toEqual(['oldest', 'old']);
  });

  it('keeps an offline room\'s tab and one with relay subscribers, recycling the next eligible', async () => {
    const { recycler, recycled } = recyclerWith({
      ageHours: { offline: 30, relayed: 20, eligible: 10 },
      live: { offline: false },
      subscribed: ['relayed']
    });
    await recycler.tick();
    expect(recycled).toEqual(['eligible']);
  });

  it('does not retry a failed room within the age window, but moves on to others', async () => {
    const { recycler, recycled } = recyclerWith({
      ageHours: { flaky: 20, other: 10 },
      failures: { flaky: 'im/fetch 200 not captured within 90000ms' }
    });
    await recycler.tick();
    await recycler.tick();
    await recycler.tick();
    expect(recycled).toEqual(['flaky', 'other']);
  });

  it('benches every recycle after a 403', async () => {
    const { recycler, recycled } = recyclerWith({
      ageHours: { refused: 20, other: 10 },
      failures: { refused: 'navigation failed: 403 Forbidden' }
    });
    await recycler.tick();
    await recycler.tick();
    expect(recycled).toEqual(['refused']);
  });
});
