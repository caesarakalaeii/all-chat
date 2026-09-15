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

import { describe, expect, it, vi, beforeEach } from 'vitest';
import { LeadershipCoordinator } from './leadership.js';
import type { Logger } from '../types/logger.js';

/**
 * A claim/renew/release/register client whose behaviour the test dictates.
 * Mirrors the fakeLeadershipClient in shared/sourcemanager/coordinator_test.go.
 */
class FakeClient {
  peerCount: number;
  released: string[] = [];

  constructor(peerCount: number) {
    this.peerCount = peerCount;
  }

  claimLeadership(_platform: string, _streamID: string): Promise<boolean> {
    return Promise.resolve(true);
  }

  renewLeadership(): Promise<boolean> {
    return Promise.resolve(true);
  }

  releaseLeadership(_platform: string, streamID: string): Promise<void> {
    this.released.push(streamID);
    return Promise.resolve();
  }

  registerPeer(): Promise<number> {
    return Promise.resolve(this.peerCount);
  }
}

const silentLogger: Logger = {
  info: vi.fn(),
  warn: vi.fn(),
  error: vi.fn(),
  debug: vi.fn()
};

/** Claim `n` leases so the coordinator has a fleet to shed. */
async function claimLeases(coord: LeadershipCoordinator, names: string[]): Promise<void> {
  for (const name of names) {
    await coord.ensureLeadership(name, () => {});
  }
}

/** Two rebalance cycles with a stable peer count in between, so the stabilization gate opens. */
async function settleRebalance(coord: LeadershipCoordinator, total: number): Promise<string[]> {
  const now = Date.now;
  const first = await coord.rebalance(total); // records peer count, starts the window
  vi.advanceTimersByTime(31_000);
  const second = await coord.rebalance(total);
  expect(first).toEqual([]);
  return second;
}

describe('LeadershipCoordinator.rebalance', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  it('sheds excess leases down to ceil(total/peers)', async () => {
    const client = new FakeClient(2);
    const coord = new LeadershipCoordinator('tiktok', client as never, silentLogger);
    await claimLeases(coord, ['a', 'b', 'c', 'd', 'e']);

    const released = await settleRebalance(coord, 4); // ceil(4/2) = 2 → shed 3

    expect(released).toEqual(['c', 'd', 'e']);
    expect(coord.getLeaseCount()).toBe(2);
    expect(client.released).toEqual(['c', 'd', 'e']);
  });

  it('releases nothing when within budget', async () => {
    const client = new FakeClient(2);
    const coord = new LeadershipCoordinator('tiktok', client as never, silentLogger);
    await claimLeases(coord, ['a', 'b']);

    const released = await settleRebalance(coord, 6); // ceil(6/2) = 3 → holding 2, no excess

    expect(released).toEqual([]);
    expect(client.released).toEqual([]);
  });

  it('keeps the alphabetically first leases, matching the Go coordinator', async () => {
    const client = new FakeClient(3);
    const coord = new LeadershipCoordinator('tiktok', client as never, silentLogger);
    await claimLeases(coord, ['m', 'z', 'a', 'x', 'k', 'b', 'y']);

    const released = await settleRebalance(coord, 6); // ceil(6/3) = 2

    expect(coord.getLeaseCount()).toBe(2);
    expect(coord.hasLeadership('a')).toBe(true);
    expect(coord.hasLeadership('b')).toBe(true);
    expect(released).toEqual(['k', 'm', 'x', 'y', 'z']);
  });

  it('waits for the stabilization window when the peer count changes', async () => {
    const client = new FakeClient(2);
    const coord = new LeadershipCoordinator('tiktok', client as never, silentLogger);
    await claimLeases(coord, ['a', 'b', 'c', 'd', 'e', 'f']);

    // First call only records the peer count.
    expect(await coord.rebalance(2)).toEqual([]);
    // Second call is still inside the 30s window — nothing released even at 3x excess.
    vi.advanceTimersByTime(15_000);
    expect(await coord.rebalance(2)).toEqual([]);
    // After the window: releases.
    vi.advanceTimersByTime(16_000);
    const released = await coord.rebalance(2);
    expect(released).toEqual(['b', 'c', 'd', 'e', 'f']);
    expect(coord.hasLeadership('a')).toBe(true);
  });

  it('returns nothing when peer registration fails', async () => {
    const client = new FakeClient(2);
    client.registerPeer = () => Promise.reject(new Error('source-manager down'));
    const coord = new LeadershipCoordinator('tiktok', client as never, silentLogger);
    await claimLeases(coord, ['a', 'b', 'c']);

    expect(await coord.rebalance(1)).toEqual([]);
    expect(coord.getLeaseCount()).toBe(3); // holds everything; next cycle retries
  });

  it('treats a zero peer count as one pod', async () => {
    const client = new FakeClient(0);
    const coord = new LeadershipCoordinator('tiktok', client as never, silentLogger);
    await claimLeases(coord, ['a', 'b', 'c']);

    // ceil(3/1) = 3 → no excess, even though registerPeer returned 0.
    expect(await settleRebalance(coord, 3)).toEqual([]);
  });
});
