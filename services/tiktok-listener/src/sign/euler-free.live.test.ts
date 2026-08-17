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
 * Live proof that lever 1 of #698 works: room ID and is-live resolve **without Euler Stream**.
 *
 * This is the acceptance test for step 4 of the issue's sequence ("switch room-id / room-info /
 * is-live to the direct composites"), and it is the one claim in this work that cannot be
 * established by mutating config objects and inspecting them. `installer.integration.test.ts`
 * proves the flags land on the real connector globals; it cannot prove that the resulting
 * configuration actually resolves a room, because Euler is only the composites' *last* leg and a
 * passing double-based test looks identical whether the direct routes work or not.
 *
 * The trick that makes this a proof rather than a smoke test: `SignConfig.basePath` is pointed at
 * a closed port (discard, TCP/9) before anything runs. If any code path reached for Euler the SDK
 * call would fail against that dead address, so a green run means the direct-to-TikTok routes
 * carried the whole thing.
 *
 * Opt-in, because it talks to tiktok.com over the network:
 *
 *     TIKTOK_LIVE_TESTS=1 npx vitest run src/sign/euler-free.live.test.ts
 *
 * It is deliberately excluded from the default `npm test` run. CI has no business depending on a
 * third party's availability, and the acceptance criteria for this task must stay hermetic --
 * a TikTok outage or a blocked egress should never look like our regression.
 */

import { describe, it, expect, beforeAll } from 'vitest';
import { RoomIdRouteConfig, IsLiveRouteConfig, SignConfig, TikTokLiveConnection } from 'tiktok-live-connector';

/** Accounts that exist and are stable enough to resolve a room ID. Liveness is not asserted. */
const ACCOUNTS = ['tiktok', 'charlidamelio', 'khaby.lame'];

/** Closed port. Any Euler SDK call made through this must fail rather than silently succeed. */
const BLACK_HOLE = 'http://127.0.0.1:9';

const enabled = process.env.TIKTOK_LIVE_TESTS === '1';

describe.skipIf(!enabled)('room resolution without Euler Stream (live)', () => {
  beforeAll(() => {
    // Lever 1: drop Euler from the composites, exactly as installSignConfiguration does.
    RoomIdRouteConfig.skipFetchRoomIdFromEulerRoute = true;
    IsLiveRouteConfig.skipFetchRoomIdFromEulerRoute = true;

    // Make any residual Euler dependency fail loudly instead of quietly working.
    SignConfig.basePath = BLACK_HOLE;
    SignConfig.cachedInstance = undefined;
  });

  it.each(ACCOUNTS)(
    'resolves a numeric room ID for @%s with Euler pointed at a closed port',
    async (uniqueId) => {
      const connection = new TikTokLiveConnection(uniqueId, {});

      await connection.fetchRoomInfo();

      // The room ID is what the composite exists to produce, and what connecting needs.
      expect(connection.roomId).toMatch(/^\d+$/);
    },
    30_000
  );

  it(
    'answers is-live without Euler',
    async () => {
      const connection = new TikTokLiveConnection(ACCOUNTS[0], {});

      // Only the type matters. Whether @tiktok happens to be streaming is not ours to assert,
      // and asserting it would make this test fail for reasons unrelated to Euler.
      await expect(connection.fetchIsLive()).resolves.toBeTypeOf('boolean');
    },
    30_000
  );
});
