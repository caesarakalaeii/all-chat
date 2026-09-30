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
 * BackoffManager unit tests
 *
 * Focuses on the ±10% scheduling jitter that prevents a thundering herd of
 * live-status checks when many channels share the same backoff interval, and
 * verifies that the jitter is purely additive (the exponential progression and
 * caps are unchanged).
 */

import { describe, it, expect, vi, afterEach } from 'vitest';
import { BackoffManager } from './backoff-manager.js';
import { Logger } from '../types/logger.js';

const noopLogger: Logger = {
  error: vi.fn(),
  warn: vi.fn(),
  info: vi.fn(),
  debug: vi.fn(),
};

describe('BackoffManager scheduling jitter', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('adds +10% jitter at the top of the random range for offline checks', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1_000_000);
    vi.spyOn(Math, 'random').mockReturnValue(1); // jitter = +10%

    const mgr = new BackoffManager(noopLogger);
    mgr.recordOfflineCheck('user');

    const state = mgr.getState('user')!;
    expect(state.currentBackoffMs).toBe(20000); // first offline check = base (20s)
    expect(state.nextCheckTime).toBe(1_000_000 + 20000 + 2000);
  });

  it('subtracts 10% jitter at the bottom of the random range for offline checks', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1_000_000);
    vi.spyOn(Math, 'random').mockReturnValue(0); // jitter = -10%

    const mgr = new BackoffManager(noopLogger);
    mgr.recordOfflineCheck('user');

    expect(mgr.getState('user')!.nextCheckTime).toBe(1_000_000 + 20000 - 2000);
  });

  it('applies zero jitter at the midpoint (random = 0.5)', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1_000_000);
    vi.spyOn(Math, 'random').mockReturnValue(0.5);

    const mgr = new BackoffManager(noopLogger);
    mgr.recordOfflineCheck('user');

    expect(mgr.getState('user')!.nextCheckTime).toBe(1_000_000 + 20000);
  });

  it('jitters connection-error backoff scheduling', () => {
    vi.spyOn(Date, 'now').mockReturnValue(500_000);
    vi.spyOn(Math, 'random').mockReturnValue(1); // +10%

    const mgr = new BackoffManager(noopLogger);
    mgr.recordConnectionError('user', new Error('boom'));

    const state = mgr.getState('user')!;
    expect(state.currentBackoffMs).toBe(2000); // first error: 2^1 * 1000ms
    expect(state.nextCheckTime).toBe(500_000 + 2000 + 200);
  });

  it('jitters the reset-to-base backoff on disconnection', () => {
    vi.spyOn(Date, 'now').mockReturnValue(700_000);
    vi.spyOn(Math, 'random').mockReturnValue(0); // -10%

    const mgr = new BackoffManager(noopLogger);
    mgr.recordDisconnection('user');

    const state = mgr.getState('user')!;
    expect(state.currentBackoffMs).toBe(20000);
    expect(state.nextCheckTime).toBe(700_000 + 20000 - 2000);
  });

  it('keeps next-check within ±10% and decorrelates many channels', () => {
    // Freeze the clock so the only source of variation is the jitter itself
    // (real Math.random, not mocked).
    vi.spyOn(Date, 'now').mockReturnValue(1_000_000);

    const mgr = new BackoffManager(noopLogger);
    const schedules = new Set<number>();

    for (let i = 0; i < 200; i++) {
      const user = `user-${i}`;
      mgr.recordOfflineCheck(user);
      const state = mgr.getState(user)!;

      expect(state.currentBackoffMs).toBe(20000);
      const offset = state.nextCheckTime - 1_000_000 - 20000;
      expect(Math.abs(offset)).toBeLessThanOrEqual(2000); // ±10% of 20s

      schedules.add(state.nextCheckTime);
    }

    // With real randomness, the 200 channels must not all land on the same tick.
    expect(schedules.size).toBeGreaterThan(150);
  });

  it('preserves the exponential offline progression (jitter is additive only)', () => {
    vi.spyOn(Math, 'random').mockReturnValue(0.5); // no jitter — isolate the curve

    const mgr = new BackoffManager(noopLogger);
    const progression: number[] = [];
    for (let i = 0; i < 6; i++) {
      mgr.recordOfflineCheck('user');
      progression.push(mgr.getState('user')!.currentBackoffMs);
    }

    // 20s, 40s, 80s, 160s, then capped at the 3-minute max.
    expect(progression).toEqual([20000, 40000, 80000, 160000, 180000, 180000]);
  });
});

describe('BackoffManager budget-refusal parking', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('parks the room for the stated window-slide time, not the error curve', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1_000_000);
    vi.spyOn(Math, 'random').mockReturnValue(0.5); // no jitter

    const mgr = new BackoffManager(noopLogger);
    mgr.recordBudgetRefusal('user', 3_600_000);

    const state = mgr.getState('user')!;
    expect(state.nextCheckTime).toBe(1_000_000 + 3_600_000);
    expect(state.currentBackoffMs).toBe(0);
    expect(mgr.shouldCheckNow('user')).toBe(false);
    expect(mgr.getTimeUntilNextCheck('user')).toBe(3_600_000);
  });

  it('parks through the stuck-recovery threshold without looking stuck', () => {
    // recoverStuckChannels (poller.ts) treats currentBackoffMs >= 180000
    // with a 5-minute-old lastCheckTime as stuck and force-removes the
    // state — which would defeat an hour-long park 5 minutes in. The park
    // must ride nextCheckTime alone; currentBackoffMs is derived state the
    // next record* call recomputes anyway.
    vi.spyOn(Date, 'now').mockReturnValue(1_000_000);
    vi.spyOn(Math, 'random').mockReturnValue(0.5);

    const mgr = new BackoffManager(noopLogger);
    mgr.recordBudgetRefusal('user', 3_600_000);

    const state = mgr.getState('user')!;
    expect(state.currentBackoffMs).toBe(0);
    expect(state.lastCheckTime).toBe(1_000_000);
  });

  it('does not touch consecutiveErrors mid-curve', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1_000_000);
    vi.spyOn(Math, 'random').mockReturnValue(0.5);

    const mgr = new BackoffManager(noopLogger);
    mgr.recordConnectionError('user', new Error('first'));
    mgr.recordConnectionError('user', new Error('second'));
    expect(mgr.getState('user')!.consecutiveErrors).toBe(2);

    // A budget refusal between two genuine errors must neither increment
    // nor reset the streak: the refusal is not the room's fault, and the
    // next genuine error resumes the curve where it left off (4s, step 3).
    mgr.recordBudgetRefusal('user', 3_600_000);
    expect(mgr.getState('user')!.consecutiveErrors).toBe(2);
    expect(mgr.getState('user')!.currentBackoffMs).toBe(4000); // untouched: still the step-2 error value

    mgr.recordConnectionError('user', new Error('third'));
    expect(mgr.getState('user')!.currentBackoffMs).toBe(8000);
    expect(mgr.getState('user')!.consecutiveErrors).toBe(3);
  });

  it('clamps a negative retryAfterMs to an immediate re-check, never a past park', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1_000_000);
    vi.spyOn(Math, 'random').mockReturnValue(0.5);

    const mgr = new BackoffManager(noopLogger);
    mgr.recordBudgetRefusal('user', -5_000);
    expect(mgr.getState('user')!.nextCheckTime).toBe(1_000_000);
  });

  it('parks at the stated time even when it is shorter than the error curve would be', () => {
    vi.spyOn(Date, 'now').mockReturnValue(1_000_000);
    vi.spyOn(Math, 'random').mockReturnValue(0.5);

    const mgr = new BackoffManager(noopLogger);
    // A nearly-slid window: waiting 5s is correct, error backoff would say 2s
    // but hammering a window with 5s left is still a refusal loop.
    mgr.recordBudgetRefusal('user', 5_000);
    expect(mgr.getState('user')!.nextCheckTime).toBe(1_000_000 + 5_000);
  });
});
