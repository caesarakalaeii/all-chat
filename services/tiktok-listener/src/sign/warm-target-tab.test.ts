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

import { describe, expect, it, vi } from 'vitest';
import { warmTargetTab } from './warm-target-tab.js';

type MockResponse = { statusCode: number; body: { text: () => Promise<string> } };

/** undici `request` stub returning a fixed status + JSON body. */
function mockFetch(statusCode: number, body: unknown) {
  const text = typeof body === 'string' ? body : JSON.stringify(body);
  const fn = vi.fn(async () => ({
    statusCode,
    body: { text: async () => text }
  }));
  return fn as unknown as typeof import('undici').request & { mock: { calls: unknown[][] } };
}

const OPTS = { signerUrl: 'http://signer:8092', authToken: 't', username: 'someone' };

describe('warmTargetTab', () => {
  it('sends one POST /v1/sign with the username on the viewer path', async () => {
    const fetchImpl = mockFetch(200, { fetchResult: 'aGk=' });
    const result = await warmTargetTab({ ...OPTS, fetchImpl });
    expect(result).toEqual({ ok: true });
    const [url, init] = fetchImpl.mock.calls[0] as [string, Record<string, unknown>];
    expect(url).toBe('http://signer:8092/v1/sign');
    expect(init.method).toBe('POST');
    expect(JSON.parse(init.body as string)).toEqual({ roomId: 'unused', username: 'someone' });
    expect((init.headers as Record<string, string>).Authorization).toBe('Bearer t');
  });

  it('classifies a 502 viewer_capture_refused as capture_refused', async () => {
    const result = await warmTargetTab({
      ...OPTS,
      fetchImpl: mockFetch(502, { error: 'viewer_capture_refused', message: 'breaker open' })
    });
    expect(result).toEqual({ ok: false, reason: 'capture_refused' });
  });

  it('classifies a 502 viewer_capture_failed as capture_failed', async () => {
    const result = await warmTargetTab({
      ...OPTS,
      fetchImpl: mockFetch(502, { error: 'viewer_capture_failed', message: 'room not live' })
    });
    expect(result).toEqual({ ok: false, reason: 'capture_failed' });
  });

  it('classifies a fetch throw as network', async () => {
    const fn = vi.fn(async () => {
      throw new Error('ECONNREFUSED');
    });
    const result = await warmTargetTab({
      ...OPTS,
      fetchImpl: fn as unknown as typeof import('undici').request
    });
    expect(result).toEqual({ ok: false, reason: 'network' });
  });

  it('classifies a non-JSON body as network (no capture assumed)', async () => {
    const result = await warmTargetTab({
      ...OPTS,
      fetchImpl: mockFetch(500, '<html>gateway error</html>')
    });
    expect(result).toEqual({ ok: false, reason: 'network' });
  });

  it('classifies an unexpected status as network, not a capture failure', async () => {
    const result = await warmTargetTab({
      ...OPTS,
      fetchImpl: mockFetch(404, { error: 'not found' })
    });
    expect(result).toEqual({ ok: false, reason: 'network' });
  });
});
