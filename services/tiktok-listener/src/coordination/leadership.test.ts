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
import {
  HOT_ROOM_THRESHOLD_SECONDS,
  LeadershipCoordinator,
  selectLeasesToRelease
} from './leadership.js';
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
async function settleRebalance(
  coord: LeadershipCoordinator,
  total: number,
  maxPerPod?: number,
  secondsSinceLastMessage?: (streamID: string) => number | undefined
): Promise<string[]> {
  const first = await coord.rebalance(total, maxPerPod, secondsSinceLastMessage); // records peer count, starts the window
  vi.advanceTimersByTime(31_000);
  const second = await coord.rebalance(total, maxPerPod, secondsSinceLastMessage);
  expect(first).toEqual([]);
  return second;
}

const byMap = (freshness: Record<string, number>): ((streamID: string) => number | undefined) =>
  (streamID) => freshness[streamID];

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

  it('sheds down to maxPerPod when the fair share exceeds it', async () => {
    // The 2026-09-14 incident's own numbers: 2 pods, 48 streams → fair share 24,
    // but the Euler ceiling is 20. A pod holding 24 leases must shed to 20,
    // not park 4 leased-but-unconnectable streams.
    const client = new FakeClient(2);
    const coord = new LeadershipCoordinator('tiktok', client as never, silentLogger);
    await claimLeases(coord, ['s01', 's02', 's03', 's04', 's05', 's06']);

    // ceil(6/2) = 3 fair share, but ceiling 2 wins.
    const released = await settleRebalance(coord, 6, 2);

    expect(released).toEqual(['s03', 's04', 's05', 's06']);
    expect(coord.getLeaseCount()).toBe(2);
  });

  it('never keeps more leases than maxPerPod even as the sole pod', async () => {
    const client = new FakeClient(1);
    const coord = new LeadershipCoordinator('tiktok', client as never, silentLogger);
    await claimLeases(coord, ['a', 'b', 'c', 'd']);

    // ceil(4/1) = 4 fair share, ceiling 3.
    const released = await settleRebalance(coord, 4, 3);

    expect(released).toEqual(['d']);
    expect(coord.getLeaseCount()).toBe(3);
  });

  it('ignores maxPerPod when the fair share is lower', async () => {
    const client = new FakeClient(4);
    const coord = new LeadershipCoordinator('tiktok', client as never, silentLogger);
    await claimLeases(coord, ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h']);

    // ceil(8/4) = 2 fair share under a generous ceiling of 6.
    const released = await settleRebalance(coord, 8, 6);

    expect(released).toEqual(['c', 'd', 'e', 'f', 'g', 'h']);
    expect(coord.getLeaseCount()).toBe(2);
  });
});

describe('selectLeasesToRelease', () => {
  // The 2026-09-17 rebalance released a room delivering ~1.6 msg/s to a
  // healthy peer; the receiving pod then burned a full flap cycle
  // re-handshaking. Selection must rank by freshness, not by ID.

  it('releases idle rooms before hot rooms', () => {
    // shed-2 with 2 idle + 3 hot: the 2 idle go, hot rooms survive.
    const released = selectLeasesToRelease(
      ['hot1', 'idle1', 'hot2', 'idle2', 'hot3'],
      2,
      byMap({ hot1: 5, hot2: 10, hot3: 30, idle1: 500, idle2: 900 })
    );
    expect(released).toEqual(['idle1', 'idle2']);
  });

  it('falls back to the least-hot rooms when idle rooms do not cover the shed', () => {
    // shed-3 with 1 idle + 3 hot: the idle goes first, then the hottest
    // rooms by oldest message time; the freshest room survives.
    const released = selectLeasesToRelease(
      ['hot1', 'idle', 'hot2', 'hot3'],
      3,
      byMap({ hot1: 5, hot2: 40, hot3: 70, idle: 900 })
    );
    expect(released).toEqual(['hot2', 'hot3', 'idle']);
  });

  it('treats rooms with no monitor entry as maximally releasable', () => {
    // A never-connected room (fresh demand) must flow to the least-loaded
    // pod as it does today, even next to a hot room.
    const released = selectLeasesToRelease(['hot', 'cold-start'], 1, byMap({ hot: 5 }));
    expect(released).toEqual(['cold-start']);
  });

  it('breaks freshness ties alphabetically, matching the Go coordinator', () => {
    const released = selectLeasesToRelease(['b', 'a', 'c'], 1, byMap({ a: 500, b: 500, c: 5 }));
    expect(released).toEqual(['a']);
  });

  it('sorts the released set alphabetically for deterministic log parity', () => {
    const released = selectLeasesToRelease(['z', 'y', 'x'], 3, byMap({ z: 900, y: 800, x: 700 }));
    expect(released).toEqual(['x', 'y', 'z']);
  });
});

describe('LeadershipCoordinator.rebalance with freshness', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  it('sheds idle leases and keeps delivering ones when ranking is available', async () => {
    const client = new FakeClient(2);
    const coord = new LeadershipCoordinator('tiktok', client as never, silentLogger);
    await claimLeases(coord, ['idle1', 'hot1', 'idle2', 'hot2', 'hot3']);

    // ceil(4/2) = 2 → shed 3; only 2 are idle, so the least-hot room goes too.
    const released = await settleRebalance(coord, 4, undefined, byMap({ hot1: 5, hot2: 10, hot3: 30, idle1: 500, idle2: 900 }));

    expect(released).toEqual(['hot3', 'idle1', 'idle2']);
    expect(coord.hasLeadership('hot1')).toBe(true);
    expect(coord.hasLeadership('hot2')).toBe(true);
  });

  it('keeps the alphabetical Go behaviour when no freshness accessor is given', async () => {
    const client = new FakeClient(2);
    const coord = new LeadershipCoordinator('tiktok', client as never, silentLogger);
    await claimLeases(coord, ['a', 'b', 'c', 'd', 'e']);

    const released = await settleRebalance(coord, 4);

    expect(released).toEqual(['c', 'd', 'e']);
  });

  it('releases only idle rooms when they cover the whole shed', async () => {
    const client = new FakeClient(2);
    const coord = new LeadershipCoordinator('tiktok', client as never, silentLogger);
    await claimLeases(coord, ['hot1', 'hot2', 'idle1', 'idle2']);

    // ceil(2/2) = 1 → shed 3; both idle go plus the least-hot.
    const released = await settleRebalance(coord, 2, undefined, byMap({ hot1: 5, hot2: 7, idle1: 400, idle2: 800 }));

    expect(released).toEqual(['hot2', 'idle1', 'idle2']);
    expect(coord.hasLeadership('hot1')).toBe(true);
  });
});
