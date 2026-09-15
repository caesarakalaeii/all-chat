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
import type { request as undiciRequest } from 'undici';
import { SelfSigner } from './self.js';
import { SignatureFailure } from './signer.js';

type FetchImpl = typeof undiciRequest;

function stubFetch(status: number, body: unknown): FetchImpl {
  return (async () => ({
    statusCode: status,
    body: {
      text: async () => JSON.stringify(body)
    }
  })) as unknown as FetchImpl;
}

describe('SelfSigner', () => {
  const request = { roomId: '740123', userAgent: 'UA' };

  it('decodes the service protobuf response into a ProtoMessageFetchResult', async () => {
    // Minimal valid wire format for the schema: field 2 (cursor) = "abc".
    const wire = Buffer.from([0x12, 0x03, 0x61, 0x62, 0x63]);

    const signer = new SelfSigner({
      baseUrl: 'http://signer:8092/',
      fetchImpl: stubFetch(200, {
        fetchResult: wire.toString('base64'),
        fetchResultCookieHeader: 'ttwid=abc',
        fetchResultRoomId: '740124'
      })
    });

    const result = await signer.sign(request);
    const fetchResult = result.fetchResult as { cursor?: string };
    expect(fetchResult.cursor).toBe('abc');
    expect(result.fetchResultCookieHeader).toBe('ttwid=abc');
    expect(result.fetchResultRoomId).toBe('740124');
  });

  it('surfaces a 429 from the service as a rate-limit-shaped failure', async () => {
    const signer = new SelfSigner({
      baseUrl: 'http://signer:8092',
      fetchImpl: stubFetch(429, { error: 'rate_limited', message: 'TikTok rate limited the sign target' })
    });

    await expect(signer.sign(request)).rejects.toThrow(SignatureFailure);
    await expect(signer.sign(request)).rejects.toThrow(/rate limited/);
  });

  it('wraps transport failures with the signer name and endpoint', async () => {
    const signer = new SelfSigner({
      baseUrl: 'http://signer-invalid:8092',
      fetchImpl: (async () => {
        throw new Error('connect ECONNREFUSED');
      }) as unknown as FetchImpl
    });

    await expect(signer.sign(request)).rejects.toThrow(
      /sign service unreachable at http:\/\/signer-invalid:8092/
    );
  });

  it('rejects a body that does not decode as ProtoMessageFetchResult', async () => {
    const signer = new SelfSigner({
      baseUrl: 'http://signer:8092',
      fetchImpl: stubFetch(200, {
        fetchResult: Buffer.from('not protobuf at all').toString('base64'),
        fetchResultCookieHeader: ''
      })
    });

    await expect(signer.sign(request)).rejects.toThrow(/did not decode as ProtoMessageFetchResult/);
  });
});
