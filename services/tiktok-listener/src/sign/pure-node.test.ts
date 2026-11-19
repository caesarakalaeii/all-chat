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
import type { Dispatcher } from 'undici';
import { PureNodeSigner } from './pure-node.js';
import { classifySignatureFailure, SignatureFailure, type SignRequest } from './signer.js';

/**
 * The lease is the signer service's SessionLease: a warm classic
 * room's captured webcast WS URL plus its cookie jar. The synthesized
 * fetchResult must reproduce the RECORDED URL's shape with the target's
 * room_id — the bare origin is handshake-rejected (measured 2026-09-18).
 */

// X-Bogus rides the fixture: measured handshake-load-bearing (stripping
// it → instant 200-rejection); a filter regression dropping it must
// fail the contract test.
const LEASE_URL =
  'wss://webcast-ws.eu.tiktok.com/webcast/im/ws_proxy/ws_reuse_supplement/?version_code=180800&device_platform=web&browser_platform=Linux%20x86_64&X-Bogus=asdf1234&identity=audience&room_id=111&internal_ext=test&cursor=abc&compress=gzip';

function leaseResponse(over: Record<string, unknown> = {}): unknown {
  return {
    wsUrl: LEASE_URL,
    cookieHeader: 'ttwid=1; msToken=2',
    roomId: '111',
    room: 'warm-a',
    userAgent: 'viewer-ua',
    proxyHost: 'p:1',
    capturedAt: Date.now(),
    ...over
  };
}

function okFetch(response: unknown = leaseResponse()) {
  // Declared so mock.calls is typed with the lease URL the signer builds
  // (the consumer key rides on it), which assertions below read.
  const impl = vi.fn(async (_url: string | URL) => ({
    statusCode: 200,
    headers: {},
    body: { text: async () => JSON.stringify(response) }
  }));
  return { impl };
}

function signRequest(over: Partial<SignRequest> = {}): SignRequest {
  return { roomId: '42', username: 'someone', userAgent: 'test-ua', ...over };
}

async function leasedSigner(over: Partial<ConstructorParameters<typeof PureNodeSigner>[0]> = {}) {
  const { impl } = okFetch();
  const signer = new PureNodeSigner({ baseUrl: 'http://signer', fetchImpl: impl as unknown as typeof import('undici').request, ...over });
  return { signer, impl };
}

describe('PureNodeSigner fetchResult contract', () => {
  it('forwards recorded params via routeParams and synthesizes the connector fields', async () => {
    const { signer, impl } = await leasedSigner();
    const result = await signer.sign(signRequest());
    const fr = result.fetchResult as Record<string, unknown>;

    // Connector contract: cursor non-falsy (the :1829 throw), internalExt
    // from the synthesized result, pushServer origin+path only.
    expect(Boolean(fr.cursor)).toBe(true);
    expect(fr.cursor).toBe('0');
    expect(fr.internalExt).toBe('');
    expect(fr.pushServer).toBe('wss://webcast-ws.eu.tiktok.com/webcast/im/ws_proxy/ws_reuse_supplement/');
    expect(String(fr.pushServer)).not.toContain('?');
    expect(fr.messages).toEqual([]);

    // The four keys the connector appends itself are NOT in routeParams;
    // every other recorded identity param is forwarded — deep-equal, not
    // sampled: a filter regression dropping ANY load-bearing param
    // (X-Bogus stripped it → instant 200-rejection) must fail here.
    expect(fr.routeParams).toEqual({
      version_code: '180800',
      device_platform: 'web',
      browser_platform: 'Linux x86_64',
      'X-Bogus': 'asdf1234',
      identity: 'audience'
    });

    // Passthrough: the lease's jar verbatim, the lease's proxy for egress
    // pinning, and never a re-pointed roomId (the lease's room is another
    // room).
    expect(result.fetchResultCookieHeader).toBe('ttwid=1; msToken=2');
    expect(result.fetchResultProxyHost).toBe('p:1');
    expect(result.fetchResultRoomId).toBeUndefined();
    expect(impl).toHaveBeenCalledTimes(1);
  });
});

describe('PureNodeSigner lease caching', () => {
  it('reuses a fresh lease without fetching and refetches when stale', async () => {
    const { signer, impl } = await leasedSigner();
    await signer.sign(signRequest());
    await signer.sign(signRequest({ roomId: '43' }));
    expect(impl).toHaveBeenCalledTimes(1);

    // Exactly at the boundary the lease is stale (the freshness window is
    // strict <), pinned with fake timers so no clock tick hides it.
    const { signer: edge, impl: edgeImpl } = await leasedSigner({ staleAfterMs: 10_000 });
    vi.useFakeTimers();
    try {
      await edge.sign(signRequest());
      vi.setSystemTime(new Date(Date.now() + 10_000));
      await edge.sign(signRequest({ roomId: '44' }));
    } finally {
      vi.useRealTimers();
    }
    expect(edgeImpl).toHaveBeenCalledTimes(2);
  });

  it('reuses a lease whose wsUrl was captured long ago: the window counts from our fetch', async () => {
    // Prod 2026-10-08: warm tabs serve their original URL and capturedAt for
    // hours. Keyed on capturedAt, every lease was stale on arrival, so every
    // sign (pre-sign included) re-fetched and the 8/h lease budget refused
    // connects after ~4 of them.
    const { impl } = okFetch(leaseResponse({ capturedAt: Date.now() - 3 * 3_600_000 }));
    const signer = new PureNodeSigner({ baseUrl: 'http://signer', fetchImpl: impl as never });
    await signer.sign({ roomId: 'unused', username: 'a', userAgent: 'ws-pin' });
    for (let i = 0; i < 10; i++) {
      await signer.sign(signRequest({ roomId: String(i) }));
    }
    expect(impl).toHaveBeenCalledTimes(1);
  });
});

describe('PureNodeSigner lease budget', () => {
  it('allows 8 successful fetches per hour; the 9th sign throws without fetching', async () => {
    const { signer, impl } = await leasedSigner({ staleAfterMs: 1 });
    for (let i = 0; i < 8; i++) {
      await signer.sign(signRequest({ roomId: String(i) }));
      await new Promise((r) => setTimeout(r, 2));
    }
    expect(impl).toHaveBeenCalledTimes(8);
    await expect(signer.sign(signRequest({ roomId: '9' }))).rejects.toThrow(/lease budget exhausted/);
    expect(impl).toHaveBeenCalledTimes(8);
  });

  it('failed fetches do not consume budget', async () => {
    const impl = vi.fn();
    for (let i = 0; i < 3; i++) {
      impl.mockResolvedValueOnce({
        statusCode: 503,
        headers: {},
        body: { text: async () => JSON.stringify({ error: 'no_warm_session' }) }
      });
    }
    impl.mockResolvedValue({
      statusCode: 200,
      headers: {},
      body: { text: async () => JSON.stringify(leaseResponse()) }
    });
    // Explicit WS-connect burst above the 11 signs this test makes, so the
    // final assertion can only be thrown by the LEASE budget — the two
    // budgets stay decoupled from each other's defaults.
    const signer = new PureNodeSigner({ baseUrl: 'http://signer', staleAfterMs: 1, wsConnectBurst: 32, fetchImpl: impl as never });

    for (let i = 0; i < 3; i++) {
      await expect(signer.sign(signRequest())).rejects.toThrow(SignatureFailure);
      await new Promise((r) => setTimeout(r, 2));
    }
    // The 3 failures left the full 8-success allowance intact.
    for (let i = 0; i < 8; i++) {
      await signer.sign(signRequest({ roomId: String(i) }));
      await new Promise((r) => setTimeout(r, 2));
    }
    await expect(signer.sign(signRequest({ roomId: 'x' }))).rejects.toThrow(/lease budget exhausted/);
  });
});

describe('PureNodeSigner single-flight', () => {
  it('coalesces concurrent cold signs into one lease fetch', async () => {
    const { impl } = okFetch();
    let release!: () => void;
    impl.mockImplementationOnce(async () => {
      await new Promise<void>((r) => (release = r));
      return { statusCode: 200, headers: {}, body: { text: async () => JSON.stringify(leaseResponse()) } };
    });
    const signer = new PureNodeSigner({ baseUrl: 'http://signer', fetchImpl: impl as never });

    const first = signer.sign(signRequest({ roomId: '1' }));
    const second = signer.sign(signRequest({ roomId: '2' }));
    release!();
    const [a, b] = await Promise.all([first, second]);
    expect(impl).toHaveBeenCalledTimes(1);
    expect((a.fetchResult as Record<string, unknown>).pushServer).toBe(
      (b.fetchResult as Record<string, unknown>).pushServer
    );
  });
});

describe('PureNodeSigner classification', () => {
  async function classifyWith(impl: ReturnType<typeof vi.fn>): Promise<string> {
    const signer = new PureNodeSigner({ baseUrl: 'http://signer', fetchImpl: impl as never });
    try {
      await signer.sign(signRequest());
    } catch (error) {
      return classifySignatureFailure(error);
    }
    throw new Error('expected a throw');
  }

  it('401 lands the bounded signature reason (auth misconfig, not a blip)', async () => {
    const impl = vi.fn(async () => ({
      statusCode: 401,
      headers: {},
      body: { text: async () => JSON.stringify({ error: 'unauthorized' }) }
    }));
    expect(await classifyWith(impl)).toBe('signature');
  });

  it('404 lands the bounded signature reason (endpoint disabled is fatal config)', async () => {
    const impl = vi.fn(async () => ({
      statusCode: 404,
      headers: {},
      body: { text: async () => JSON.stringify({ error: 'session_endpoint_disabled' }) }
    }));
    expect(await classifyWith(impl)).toBe('signature');
  });

  it('503 no_warm_session lands the bounded network reason (retryable)', async () => {
    const impl = vi.fn(async () => ({
      statusCode: 503,
      headers: {},
      body: { text: async () => JSON.stringify({ error: 'no_warm_session' }) }
    }));
    expect(await classifyWith(impl)).toBe('network');
  });

  it('connection refused lands the bounded network reason', async () => {
    const impl = vi.fn(async () => {
      throw Object.assign(new Error('fetch failed: connect ECONNREFUSED 127.0.0.1:18092'), {
        cause: new Error('connect ECONNREFUSED 127.0.0.1:18092')
      });
    });
    expect(await classifyWith(impl)).toBe('network');
  });
});

describe('PureNodeSigner WS-connect budget', () => {
  const refusalOf = (signer: PureNodeSigner, roomId: string) =>
    signer.sign(signRequest({ roomId })).then(
      () => undefined,
      (error: SignatureFailure) => error.retryAfterMs
    );

  it('caps back-to-back connects at the burst with distinct rooms, and pre-signs survive an empty bucket', async () => {
    const { signer, impl } = await leasedSigner();
    // 12 connect signs with DISTINCT roomIds: a per-room-keyed counter
    // would pass these; only a pool-wide bucket runs dry.
    for (let i = 0; i < 12; i++) {
      await signer.sign(signRequest({ roomId: String(i) }));
    }
    expect(impl).toHaveBeenCalledTimes(1);
    // The 13th connect sign throws rate_limit...
    await expect(signer.sign(signRequest({ roomId: '13' }))).rejects.toThrow(/WS connect budget exhausted/);
    // ...but a pre-sign (ws-pin marker) still succeeds on the cached lease.
    const pre = await signer.sign({ roomId: 'unused', username: 'someone', userAgent: 'ws-pin' });
    expect((pre.fetchResult as Record<string, unknown>).cursor).toBe('0');
  });

  it('identifies itself as a lease consumer so the signer can assign it a session', async () => {
    const { signer, impl } = await leasedSigner({ consumer: 'tiktok-listener-abc' });
    await signer.sign(signRequest());
    expect(String(impl.mock.calls[0]![0])).toBe(
      'http://signer/v1/session?consumer=tiktok-listener-abc'
    );
  });

  it('omits the consumer parameter when no consumer key is configured', async () => {
    const { signer, impl } = await leasedSigner();
    await signer.sign(signRequest());
    expect(String(impl.mock.calls[0]![0])).toBe('http://signer/v1/session');
  });

  it('does not refill the bucket when the signer moves it to another warm room', async () => {
    // Warm rooms share one browser profile under direct egress, so a
    // different room is not known to be a different session to TikTok. A
    // refill on every move would hand out a fresh burst each time a warm
    // streamer goes offline.
    const impl = vi.fn();
    impl.mockResolvedValueOnce({
      statusCode: 200,
      headers: {},
      body: { text: async () => JSON.stringify(leaseResponse({ room: 'warm-a' })) }
    });
    impl.mockResolvedValue({
      statusCode: 200,
      headers: {},
      body: { text: async () => JSON.stringify(leaseResponse({ room: 'warm-b' })) }
    });
    const signer = new PureNodeSigner({
      baseUrl: 'http://signer',
      wsConnectBurst: 2,
      staleAfterMs: 0, // re-lease on every sign, so the room change lands
      fetchImpl: impl as never
    });

    await signer.sign(signRequest({ roomId: 'a' }));
    await signer.sign(signRequest({ roomId: 'b' }));
    await expect(signer.sign(signRequest({ roomId: 'c' }))).rejects.toThrow(
      /WS connect budget exhausted/
    );
  });

  it('refills at the sustained rate, so a refused room waits minutes and not an hour', async () => {
    // Prod 2026-11-19: under the rolling-hour count a room refused after a
    // mass re-dial was still waiting 35 minutes later.
    vi.useFakeTimers();
    try {
      const { signer } = await leasedSigner({ maxWsConnectsPerHour: 6, wsConnectBurst: 2 });
      const start = Date.now();
      await signer.sign(signRequest({ roomId: 'a' }));
      await signer.sign(signRequest({ roomId: 'b' }));
      // 6 an hour is a token every 10 minutes.
      expect(await refusalOf(signer, 'c')).toBe(10 * 60_000);
      vi.setSystemTime(start + 10 * 60_000);
      expect(await refusalOf(signer, 'c')).toBeUndefined();
      expect(await refusalOf(signer, 'd')).toBe(10 * 60_000);
    } finally {
      vi.useRealTimers();
    }
  });

  it('saves up no more than the burst over a quiet spell', async () => {
    vi.useFakeTimers();
    try {
      const { signer } = await leasedSigner({ maxWsConnectsPerHour: 6, wsConnectBurst: 2 });
      vi.setSystemTime(Date.now() + 24 * 3_600_000);
      await signer.sign(signRequest({ roomId: 'a' }));
      await signer.sign(signRequest({ roomId: 'b' }));
      expect(await refusalOf(signer, 'c')).toBeGreaterThan(0);
    } finally {
      vi.useRealTimers();
    }
  });

  it('a sign that gets no lease gives its connect token back', async () => {
    const impl = vi.fn();
    // A signer outage (2026-10-10): every lease fetch answers 503
    // no_warm_session, and nothing is dialled.
    for (let i = 0; i < 12; i++) {
      impl.mockResolvedValueOnce({
        statusCode: 503,
        headers: {},
        body: { text: async () => JSON.stringify({ error: 'no_warm_session' }) }
      });
    }
    impl.mockResolvedValue({
      statusCode: 200,
      headers: {},
      body: { text: async () => JSON.stringify(leaseResponse()) }
    });
    const signer = new PureNodeSigner({ baseUrl: 'http://signer', fetchImpl: impl as never });

    for (let i = 0; i < 12; i++) {
      await expect(signer.sign(signRequest({ roomId: String(i) }))).rejects.toThrow(/no session lease/);
    }
    // Once the signer recovers, the full burst is still there.
    for (let i = 0; i < 12; i++) {
      await signer.sign(signRequest({ roomId: `r${i}` }));
    }
    await expect(signer.sign(signRequest({ roomId: 'over' }))).rejects.toThrow(/WS connect budget exhausted/);
  });

  it('refused rooms queue for distinct tokens instead of all waking on the next one', async () => {
    // Prod 2026-11-12: 7 rooms refused together were all told to retry at
    // the same moment, one connected and six were refused again, every
    // time, so 37 of 59 refusals in a day were rooms already parked.
    vi.useFakeTimers();
    try {
      const { signer } = await leasedSigner({ maxWsConnectsPerHour: 6, wsConnectBurst: 2 });
      const start = Date.now();
      const minute = 60_000;

      await signer.sign(signRequest({ roomId: 'a' }));
      await signer.sign(signRequest({ roomId: 'b' }));
      // c waits for the first token (+10m), d for the second (+20m).
      expect(await refusalOf(signer, 'c')).toBe(10 * minute);
      expect(await refusalOf(signer, 'd')).toBe(20 * minute);

      // The first token arrives at +10m but is held for c: a new room is
      // refused and queued behind d.
      vi.setSystemTime(start + 10 * minute);
      expect(await refusalOf(signer, 'e')).toBe(20 * minute);
      vi.setSystemTime(start + 10 * minute + 30_000);
      expect(await refusalOf(signer, 'c')).toBeUndefined();

      // d never comes back (went offline): its token is held for the
      // grace period, then anyone may take it.
      vi.setSystemTime(start + 20 * minute + 30_000);
      expect(await refusalOf(signer, 'f')).toBeGreaterThan(0);
      vi.setSystemTime(start + 22 * minute);
      expect(await refusalOf(signer, 'f')).toBeUndefined();

      // e still gets the token it was promised for +30m.
      vi.setSystemTime(start + 30 * minute);
      expect(await refusalOf(signer, 'e')).toBeUndefined();
    } finally {
      vi.useRealTimers();
    }
  });

  it('a room back before its turn keeps its place in the queue', async () => {
    vi.useFakeTimers();
    try {
      const { signer } = await leasedSigner({ maxWsConnectsPerHour: 6, wsConnectBurst: 1 });
      const start = Date.now();
      await signer.sign(signRequest({ roomId: 'a' }));
      expect(await refusalOf(signer, 'c')).toBe(10 * 60_000);
      expect(await refusalOf(signer, 'd')).toBe(20 * 60_000);
      // c is re-dialled early (a rebalance): it is not sent behind d.
      vi.setSystemTime(start + 4 * 60_000);
      expect(await refusalOf(signer, 'c')).toBe(6 * 60_000);
      vi.setSystemTime(start + 10 * 60_000);
      expect(await refusalOf(signer, 'c')).toBeUndefined();
    } finally {
      vi.useRealTimers();
    }
  });

  it('drops to a slow rate with no burst once TikTok refuses a handshake', async () => {
    // The configured rate is above anything prod has been refused at, not
    // above anything TikTok might refuse. Dialling at full rate into a
    // throttled session only extends the throttle.
    vi.useFakeTimers();
    try {
      const { signer } = await leasedSigner();
      const start = Date.now();
      expect(signer.connectBudgetStatus().throttled).toBe(false);

      signer.noteHandshakeRefused();
      expect(signer.connectBudgetStatus().throttled).toBe(true);
      // The 12-token burst is gone: one connect, then 6 an hour.
      await signer.sign(signRequest({ roomId: 'a' }));
      expect(await refusalOf(signer, 'b')).toBe(10 * 60_000);
      vi.setSystemTime(start + 10 * 60_000);
      expect(await refusalOf(signer, 'b')).toBeUndefined();

      // An hour after the refusal the configured rate is back.
      vi.setSystemTime(start + 3_600_000 + 1);
      expect(signer.connectBudgetStatus().throttled).toBe(false);
    } finally {
      vi.useRealTimers();
    }
  });

  it('reports the rooms waiting on the budget and how long the oldest has waited', async () => {
    vi.useFakeTimers();
    try {
      const { signer } = await leasedSigner({ maxWsConnectsPerHour: 6, wsConnectBurst: 1 });
      const start = Date.now();
      await signer.sign(signRequest({ roomId: 'a' }));
      expect(signer.connectBudgetStatus()).toMatchObject({ waitingRooms: 0, oldestWaitMs: 0 });

      await refusalOf(signer, 'c');
      vi.setSystemTime(start + 2 * 60_000);
      await refusalOf(signer, 'd');
      vi.setSystemTime(start + 5 * 60_000);
      expect(signer.connectBudgetStatus()).toMatchObject({
        waitingRooms: 2,
        oldestWaitMs: 5 * 60_000
      });

      // c connects on its token: only d is still waiting.
      vi.setSystemTime(start + 10 * 60_000);
      expect(await refusalOf(signer, 'c')).toBeUndefined();
      expect(signer.connectBudgetStatus()).toMatchObject({
        waitingRooms: 1,
        oldestWaitMs: 8 * 60_000
      });

      // d never comes back: it stops counting once its held token lapses.
      vi.setSystemTime(start + 20 * 60_000 + 91_000);
      expect(signer.connectBudgetStatus()).toMatchObject({ waitingRooms: 0, oldestWaitMs: 0 });
    } finally {
      vi.useRealTimers();
    }
  });
});

describe('PureNodeSigner budget refusal classification', () => {
  it('the exhausted WS-connect budget refuses with reason budget and the wait for its token', async () => {
    vi.useFakeTimers();
    try {
      const { signer } = await leasedSigner({ maxWsConnectsPerHour: 6, wsConnectBurst: 2 });
      const start = Date.now();
      await signer.sign(signRequest({ roomId: 'a' }));
      vi.setSystemTime(new Date(start + 30_000));
      await signer.sign(signRequest({ roomId: 'b' }));
      vi.setSystemTime(new Date(start + 60_000));

      let refusal: unknown;
      try {
        await signer.sign(signRequest({ roomId: 'c' }));
      } catch (error) {
        refusal = error;
      }

      expect(refusal).toBeInstanceOf(SignatureFailure);
      expect(classifySignatureFailure(refusal)).toBe('budget');
      // 6 an hour is a token every 10 minutes, and the minute since the
      // bucket ran dry has earned a tenth of one.
      expect((refusal as SignatureFailure).retryAfterMs).toBe(9 * 60_000);
    } finally {
      vi.useRealTimers();
    }
  });

  it('the exhausted lease budget refuses with reason budget and its own retryAfterMs', async () => {
    vi.useFakeTimers();
    try {
      const impl = vi.fn(async () => ({
        statusCode: 200,
        headers: {},
        body: { text: async () => JSON.stringify(leaseResponse()) }
      }));
      const signer = new PureNodeSigner({ baseUrl: 'http://signer', staleAfterMs: 1, maxLeasesPerHour: 1, fetchImpl: impl as never });
      const start = Date.now();
      await signer.sign(signRequest({ roomId: 'a' }));
      vi.setSystemTime(new Date(start + 30_000));
      // Lease stale (staleAfterMs: 1) → a second fetch is demanded, but the
      // lease budget (1/h) is already consumed.
      let refusal: unknown;
      try {
        await signer.sign(signRequest({ roomId: 'b' }));
      } catch (error) {
        refusal = error;
      }
      expect(refusal).toBeInstanceOf(SignatureFailure);
      expect(classifySignatureFailure(refusal)).toBe('budget');
      // Lease window opened at start; 30s later the room waits 59.5 minutes.
      expect((refusal as SignatureFailure).retryAfterMs).toBe(3_600_000 - 30_000);
    } finally {
      vi.useRealTimers();
    }
  });
});

describe('PureNodeSigner pre-sign tolerance', () => {
  it("serves the lane-pin pre-sign's fake roomId from the cached lease without fetching", async () => {
    const { signer, impl } = await leasedSigner();
    const first = await signer.sign({ roomId: 'unused', username: 'someone', userAgent: 'ws-pin' });
    expect(impl).toHaveBeenCalledTimes(1);
    expect(first.fetchResultProxyHost).toBe('p:1');
    // Warm cache: a second pre-sign must be a pure cache hit — no fetch.
    await signer.sign({ roomId: 'unused', username: 'someone-else', userAgent: 'ws-pin' });
    expect(impl).toHaveBeenCalledTimes(1);
  });
});
