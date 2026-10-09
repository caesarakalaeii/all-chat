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

import { describe, it, expect, vi, type Mock } from 'vitest';
import { InvalidResponseCompositeError } from 'tiktok-live-connector';
import type * as Connector from 'tiktok-live-connector';
import { TikTokStatusChecker } from './status-checker.js';
import type { RelayLookup, UsRelay } from './us-relay.js';

const euLookups = vi.hoisted(() => ({ count: 0 }));

// vi.mock is hoisted above the imports, so status-checker sees this stub.
vi.mock('tiktok-live-connector', async (importOriginal) => {
  const actual = await importOriginal<typeof Connector>();
  return {
    ...actual,
    TikTokLiveConnection: class {
      async fetchIsLive(): Promise<boolean> {
        euLookups.count++;
        throw new actual.InvalidResponseCompositeError(
          {
            routeId: 'fetchIsLiveRoute',
            requestErrs: [
              new Error('Failed to extract the LiveRoom object from SIGI_STATE.'),
              new Error('API Error 19881007 (user_not_found)'),
            ],
          },
          'Failed to retrieve live status from all sources.'
        );
      }
    },
  };
});

describe('TikTokStatusChecker error reporting', () => {
  it('logs the per-source causes of a composite failure, not just "all sources"', async () => {
    const logger = { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() };
    const checker = new TikTokStatusChecker(logger);

    const result = await checker.checkLiveStatus('wronghandle');

    expect(result.isLive).toBe(false);
    expect(result.error).toBeInstanceOf(InvalidResponseCompositeError);
    const [, meta] = logger.error.mock.calls[0];
    expect(meta.error).toContain('user_not_found');
    expect(meta.error).toContain('SIGI_STATE');
  });
});

describe('TikTokStatusChecker US relay fallback', () => {
  const logger = () => ({ error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() });
  const relayAnswering = (answer: RelayLookup): UsRelay & { lookup: Mock<UsRelay['lookup']> } => ({
    lookup: vi.fn(async () => answer)
  });

  it('asks the relay when TikTok says user_not_found, and returns its room ID', async () => {
    const relay = relayAnswering({ found: true, live: true, roomId: '7694394629390748430' });
    const checker = new TikTokStatusChecker(logger(), 10000, relay);

    const result = await checker.checkLiveStatus('soyabontv');

    expect(result).toEqual({ isLive: true, roomId: '7694394629390748430' });
    expect(relay.lookup).toHaveBeenCalledWith('soyabontv');
  });

  it('goes to the relay directly once a handle is known to need it', async () => {
    const relay = relayAnswering({ found: true, live: false });
    const checker = new TikTokStatusChecker(logger(), 10000, relay);
    await checker.checkLiveStatus('soyabontv');
    checker.clearCache('soyabontv');
    euLookups.count = 0;

    const result = await checker.checkLiveStatus('soyabontv');

    expect(result).toEqual({ isLive: false, roomId: undefined });
    expect(euLookups.count).toBe(0);
    expect(relay.lookup).toHaveBeenCalledTimes(2);
  });

  it('still reports the error when the relay does not know the handle either', async () => {
    const checker = new TikTokStatusChecker(logger(), 10000, relayAnswering({ found: false, live: false }));

    const result = await checker.checkLiveStatus('nobody_here');

    expect(result.isLive).toBe(false);
    expect(result.error).toBeInstanceOf(InvalidResponseCompositeError);
  });
});
