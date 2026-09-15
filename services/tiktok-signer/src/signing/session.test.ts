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
import { encode as encodeXGnarly } from '../../vendor/xgnarly.mjs';
import { SigningSession } from './session.js';

describe('vendored xgnarly encoder', () => {
  it('is deterministic for identical inputs including pinned entropy', () => {
    const options = {
      ubcode: 4,
      sdkVersion: '1.0.0.368',
      timestampMs: 1789482000000,
      randomLow16: Buffer.from([0xab, 0xcd]),
      random32: Buffer.from([0x01, 0x02, 0x03, 0x04]),
      randomKey: Buffer.alloc(48, 7)
    };
    const a = encodeXGnarly('aid=1988&app_name=tiktok_web', '', 'UA', {}, options);
    const b = encodeXGnarly('aid=1988&app_name=tiktok_web', '', 'UA', {}, options);
    expect(a).toBe(b);
    // Custom alphabet, base64-shaped: only alphabet chars plus optional '=' padding.
    expect(a).toMatch(/^[u09tbS3UvgDEe6r\-ZVMXzLpsAohTn7mdINQlW412GqBjfYiyk8JORCF5/xKHwacP=]+$/);
  });

  it('changes when any signed input changes', () => {
    const base = {
      ubcode: 4,
      sdkVersion: '1.0.0.368',
      timestampMs: 1789482000000,
      randomLow16: Buffer.from([0xab, 0xcd]),
      random32: Buffer.from([0x01, 0x02, 0x03, 0x04]),
      randomKey: Buffer.alloc(48, 7)
    };
    const query = 'aid=1988&room_id=123';
    const ua = 'Mozilla/5.0 Test UA';
    expect(encodeXGnarly(query, '', ua, {}, base)).not.toBe(
      encodeXGnarly(query + '&x=1', '', ua, {}, base)
    );
    expect(encodeXGnarly(query, '', ua, {}, base)).not.toBe(
      encodeXGnarly(query, '', 'Other UA', {}, base)
    );
    expect(encodeXGnarly(query, '', ua, {}, base)).not.toBe(
      encodeXGnarly(query, '', ua, { totalXHRRequests: 5 }, base)
    );
  });
});

describe('SigningSession identity', () => {
  it('exposes a stable macOS Safari identity the listener pins its presets to', () => {
    const session = new SigningSession({ skipWarmUp: true });
    const identity = session.identity;
    expect(identity.userAgent).toContain('Macintosh');
    expect(identity.browserPlatform).toBe('MacIntel');
    expect(identity.os).toBe('mac');
    // The same instance every read: callers can cache it.
    expect(session.identity).toBe(identity);
  });
});

describe('SigningSession queue', () => {
  it('runs sign jobs strictly sequentially', async () => {
    const session = new SigningSession({ skipWarmUp: true });
    const order: string[] = [];

    // Drive the queue without a browser: enqueue is private, so exercise it
    // through signUrl's failure path. Without a warmed page every job throws
    // immediately, which is enough to observe ordering and rejection.
    const jobs = [1, 2, 3].map((i) =>
      session.signUrl(`https://webcast.tiktok.com/x/?i=${i}`).then(
        () => 'resolved',
        (error: Error) => {
          order.push(`reject:${i}:${error.message}`);
          return 'rejected';
        }
      )
    );

    const results = await Promise.all(jobs);
    expect(results.every((r) => r === 'rejected')).toBe(true);
    // All three attempts ran, and in submission order.
    expect(order.length).toBe(3);
    expect(order.map((o) => o.split(':')[1])).toEqual(['1', '2', '3']);
    await session.close();
  });
});
