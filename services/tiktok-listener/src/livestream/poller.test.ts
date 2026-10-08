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

import { describe, it, expect, vi, afterEach } from 'vitest';
import { BackoffManager } from './backoff-manager.js';
import { LiveStreamPoller } from './poller.js';
import { Logger } from '../types/logger.js';

const noopLogger: Logger = {
  error: vi.fn(),
  warn: vi.fn(),
  info: vi.fn(),
  debug: vi.fn()
};

/** An offline streak up to the max offline backoff, the state a room has when it goes live. */
function offlineStreak(mgr: BackoffManager, username: string): void {
  for (let i = 0; i < 5; i++) mgr.recordOfflineCheck(username);
}

describe('LiveStreamPoller stuck recovery', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('leaves a budget park alone even when the room carries a max offline backoff', () => {
    const now = vi.spyOn(Date, 'now').mockReturnValue(1_000_000);
    vi.spyOn(Math, 'random').mockReturnValue(0.5);
    const mgr = new BackoffManager(noopLogger);
    const poller = new LiveStreamPoller({} as never, mgr, noopLogger);

    offlineStreak(mgr, 'parked');
    mgr.recordBudgetRefusal('parked', 39 * 60_000);
    now.mockReturnValue(1_000_000 + 6 * 60_000);
    poller.recoverStuckChannels();

    expect(mgr.shouldCheckNow('parked')).toBe(false);
    expect(mgr.budgetParkRemainingMs('parked')).toBe(33 * 60_000);
  });

  it('still recovers a room stuck at max offline backoff with no park', () => {
    const now = vi.spyOn(Date, 'now').mockReturnValue(1_000_000);
    vi.spyOn(Math, 'random').mockReturnValue(0.5);
    const mgr = new BackoffManager(noopLogger);
    const poller = new LiveStreamPoller({} as never, mgr, noopLogger);

    offlineStreak(mgr, 'stuck');
    now.mockReturnValue(1_000_000 + 6 * 60_000);
    poller.recoverStuckChannels();

    expect(mgr.getState('stuck')).toBeUndefined();
  });
});
