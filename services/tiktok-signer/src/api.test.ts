import { afterEach, describe, expect, it, vi } from 'vitest';
import http from 'node:http';
import type { AddressInfo } from 'node:net';
import { request as undiciRequestMock } from 'undici';
import { createServer, type ServerOptions } from './api.js';
import type { SigningSession, SignerIdentity } from './signing/session.js';
import type { ViewerPool, RoomCapture, SessionLease } from './signing/viewer.js';
import type { RelayHub } from './signing/relay.js';
import { CaptureBreaker } from './signing/capture-breaker.js';

// The signature path executes the signed fetch through undici; the test
// must not reach the real webcast endpoint.
vi.mock('undici', () => ({
  request: vi.fn(async () => ({
    statusCode: 200,
    headers: {},
    body: { arrayBuffer: async () => new ArrayBuffer(1500) }
  }))
}));

/**
 * The path label on signer_sign_requests_total / ..._duration_seconds is
 * what the deployment alerts key on (caesar-deployment alert path split): a
 * dropped or wrong label silently breaks TikTokSignerSlowSigning and
 * TikTokSignerViewerCaptureSlowTail. These tests drive the real HTTP server
 * for both sign paths and assert the exposed series.
 */

function stubSession(): SigningSession {
  // SigningSession exposes its identity via the `identity` getter; the
  // stub nests it as a property. Full SignerIdentity shape: imFetchParams
  // derives its self-describing params from every field.
  const identity: SignerIdentity = {
    userAgent: 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36',
    browserPlatform: 'Linux x86_64',
    os: 'linux',
    screenWidth: 1920,
    screenHeight: 1080
  };
  return {
    identity,
    signUrl: async (url: string) => ({
      signedUrl: url,
      userAgent: identity.userAgent,
      cookies: ''
    })
  } as unknown as SigningSession;
}
function stubViewer(): ViewerPool {
  return {
    captureRoom: async (username: string): Promise<RoomCapture> => ({
      roomId: '42',
      protoBase64: Buffer.alloc(1200, 'x').toString('base64'),
      cookieHeader: '',
      userAgent: 'test-ua',
      proxyHost: '',
      elapsedMs: 1,
      wsUrl: ''
    })
  } as unknown as ViewerPool;
}

async function withServer(
  options: Partial<ServerOptions>,
  run: (base: string) => Promise<void>
): Promise<void> {
  const server = createServer({
    port: 0,
    session: stubSession(),
    ...options
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const { port } = server.address() as AddressInfo;
  try {
    await run(`http://127.0.0.1:${port}`);
  } finally {
    server.close();
  }
}

function post(base: string, path: string, body: unknown): Promise<number> {
  return new Promise((resolve, reject) => {
    const payload = JSON.stringify(body);
    const req = http.request(
      `${base}${path}`,
      { method: 'POST', headers: { 'Content-Type': 'application/json' } },
      (res) => {
        res.resume();
        res.on('end', () => resolve(res.statusCode ?? 0));
      }
    );
    req.on('error', reject);
    req.end(payload);
  });
}

async function scrape(base: string): Promise<string> {
  return new Promise((resolve, reject) => {
    http.get(`${base}/metrics`, (res) => {
      let data = '';
      res.on('data', (chunk: Buffer) => (data += chunk.toString()));
      res.on('end', () => resolve(data));
    }).on('error', reject);
  });
}

async function scrapeCounts(): Promise<{ viewerCapture: number; signature: number }> {
  let counts = { viewerCapture: 0, signature: 0 };
  await withServer({}, async (base) => {
    const metrics = await scrape(base);
    const value = (path: string): number => {
      const line = metrics
        .split('\n')
        .find(
          (l) =>
            l.startsWith('signer_sign_requests_total{') &&
            l.includes(`path="${path}"`) &&
            l.includes('outcome="success"')
        );
      return line ? Number(line.split(' ').pop()) : 0;
    };
    counts = { viewerCapture: value('viewer_capture'), signature: value('signature') };
  });
  return counts;
}



describe('sign metrics path split', () => {
  it('labels viewer-capture and signature requests with the right path', async () => {
    await withServer({ viewer: stubViewer() }, async (base) => {
      // Viewer path: /v1/sign with a username while viewer mode is on.
      expect(await post(base, '/v1/sign', { roomId: '42', username: 'someone' })).toBe(200);
      // Signature path: /v1/sign without a username.
      expect(await post(base, '/v1/sign', { roomId: '42' })).toBe(200);

      const metrics = await scrape(base);
      const viewerLine = metrics
        .split('\n')
        .find((l) => l.startsWith('signer_sign_requests_total{') && l.includes('path="viewer_capture"'));
      const signatureLine = metrics
        .split('\n')
        .find((l) => l.startsWith('signer_sign_requests_total{') && l.includes('path="signature"'));
      expect(viewerLine).toBeDefined();
      expect(signatureLine).toBeDefined();
      expect(viewerLine!.trim().endsWith(' 1')).toBe(true);
      expect(signatureLine!.trim().endsWith(' 1')).toBe(true);

      const histogram = metrics
        .split('\n')
        .find((l) => l.startsWith('signer_sign_request_duration_seconds_bucket{') && l.includes('path="viewer_capture"'));
      expect(histogram).toBeDefined();
      // The 60s and 120s buckets the capture-tail alert needs exist.
      expect(metrics).toMatch(/le="60"/);
      expect(metrics).toMatch(/le="120"/);
    });
  });

  it('routes to the signature path when no viewer is configured even with a username', async () => {
    // The registry is module-global, so earlier tests' series persist:
    // assert on the values this request moved, not on global absence.
    const before = await scrapeCounts();
    await withServer({}, async (base) => {
      expect(await post(base, '/v1/sign', { roomId: '42', username: 'someone' })).toBe(200);
      const metrics = await scrape(base);
      expect(metrics).toContain('path="signature"');
      // The viewer_capture counter must not have moved for a request made
      // with no viewer configured.
      const after = await scrapeCounts();
      expect(after.viewerCapture).toBe(before.viewerCapture);
      expect(after.signature).toBe(before.signature + 1);
    });
  });



  it('the signature path fetches the signed URL with every im/fetch param', async () => {
    // Round-1 regression this pins: the query-param copy loop was dropped
    // once, and nothing failed because the undici mock ignored its input
    // and the session stub echoes the URL. Record what undici actually
    // receives and assert the params survived onto the fetched URL.
    const seenUrls: string[] = [];
    vi.mocked(undiciRequestMock).mockImplementationOnce(async (input: unknown) => {
      seenUrls.push(String(input));
      return {
        statusCode: 200,
        headers: {},
        body: { arrayBuffer: async () => new ArrayBuffer(1500) }
      } as never;
    });
    await withServer({}, async (base) => {
      expect(await post(base, '/v1/sign', { roomId: '42', cursor: '7' })).toBe(200);
    });
    expect(seenUrls).toHaveLength(1);
    const fetched = new URL(seenUrls[0]!);
    expect(fetched.origin + fetched.pathname).toBe('https://webcast.tiktok.com/webcast/im/fetch/');
    // room_id and cursor from the payload, device_id from the identity:
    // any of these missing is the round-1 regression back again.
    expect(fetched.searchParams.get('room_id')).toBe('42');
    expect(fetched.searchParams.get('cursor')).toBe('7');
    expect(fetched.searchParams.get('device_id')).toBeTruthy();
    expect(fetched.searchParams.get('browser_version')).toBeTruthy();
  });
});



// The stub viewer models the pool's contract the handler consumes:
// sessionLease / hasTab / closeTab / captureRoom. The breaker is real so
// refusal/fallthrough behavior is exercised, not mocked.

function lease(over: Partial<SessionLease> = {}): SessionLease {
  return {
    wsUrl: 'wss://webcast-ws.example/ws',
    cookieHeader: 'ttwid=1; msToken=2',
    roomId: '42',
    userAgent: 'viewer-ua',
    proxyHost: 'p:1',
    capturedAt: Date.now(),
    ...over
  };
}

/** Viewer double driven by per-room tables the tests arrange. */
function stubbedViewer(config: {
  leases: Record<string, SessionLease | undefined>;
  hasTabs?: Record<string, boolean>;
  captures?: Record<string, { lease: SessionLease } | { error: string }>;
}) {
  const calls = { closedTabs: [] as string[], captures: [] as string[] };
  const viewer = {
    async sessionLease(username: string) {
      return config.leases[username];
    },
    hasTab(username: string) {
      return config.hasTabs?.[username] ?? false;
    },
    closeTab(username: string) {
      calls.closedTabs.push(username);
      // A closed tab is gone: neither the lease nor the registration
      // survives, matching the real pool's closeTab.
      delete config.leases[username];
      if (config.hasTabs) delete config.hasTabs[username];
      return true;
    },
    async captureRoom(username: string) {
      calls.captures.push(username);
      const outcome = config.captures?.[username];
      if (!outcome) throw new Error('no capture configured');
      if ('error' in outcome) throw new Error(outcome.error);
      // A successful capture registers a warm tab: the next sessionLease
      // for the room serves the captured session, as the real pool does.
      config.leases[username] = outcome.lease;
      return { roomId: '42', protoBase64: 'x', cookieHeader: '', userAgent: 'u', proxyHost: '', elapsedMs: 1, wsUrl: outcome.lease.wsUrl };
    },
    pinTab: () => undefined
  } as unknown as ViewerPool;
  return { viewer, calls };
}

function getJson(base: string, path: string, token?: string): Promise<{ status: number; body: Record<string, unknown> }> {
  return new Promise((resolve, reject) => {
    const req = http.get(
      `${base}${path}`,
      token ? { headers: { Authorization: `Bearer ${token}` } } : {},
      (res) => {
        let data = '';
        res.on('data', (chunk: Buffer) => (data += chunk.toString()));
        res.on('end', () => {
          let body: Record<string, unknown> = {};
          try {
            body = JSON.parse(data) as Record<string, unknown>;
          } catch {
            body = {};
          }
          resolve({ status: res.statusCode ?? 0, body });
        });
      }
    );
    req.on('error', reject);
    req.end();
  });
}

describe('GET /v1/session', () => {
  it('serves a warm lease with a non-empty wsUrl', async () => {
    const { viewer } = stubbedViewer({ leases: { rooma: lease() } });
    await withServer({ viewer, warmRooms: ['rooma'] }, async (base) => {
      const res = await getJson(base, '/v1/session');
      expect(res.status).toBe(200);
      expect(res.body.wsUrl).toBe('wss://webcast-ws.example/ws');
      expect(typeof res.body.capturedAt).toBe('number');
      expect(res.body.cookieHeader).toBe('ttwid=1; msToken=2');
      expect(res.body.userAgent).toBe('viewer-ua');
      expect(res.body.proxyHost).toBe('p:1');
    });
  });

  it('skips a warm-but-empty lease and serves the next room', async () => {
    const { viewer } = stubbedViewer({
      leases: { rooma: lease({ wsUrl: '' }), roomb: lease() }
    });
    await withServer({ viewer, warmRooms: ['rooma', 'roomb'] }, async (base) => {
      const res = await getJson(base, '/v1/session');
      expect(res.status).toBe(200);
      expect(res.body.wsUrl).toBe('wss://webcast-ws.example/ws');
    });
  });

  it('captures a tab-less room when no warm lease serves', async () => {
    const { viewer, calls } = stubbedViewer({
      leases: {},
      hasTabs: { rooma: false },
      captures: { rooma: { lease: lease() } }
    });
    await withServer({ viewer, warmRooms: ['rooma'] }, async (base) => {
      const res = await getJson(base, '/v1/session');
      expect(res.status).toBe(200);
      expect(calls.captures).toEqual(['rooma']);
    });
  });

  it('skips an offline warm room before capturing and serves the next live one', async () => {
    // 2026-09-20 WarmCaptureFailing alert: the offline half of a warm
    // pair burned a ~90s capture (plus breaker churn) on every cold
    // lease fetch. The liveness pre-check must skip it entirely.
    const { viewer, calls } = stubbedViewer({
      leases: {},
      hasTabs: { rooma: false, roomb: false },
      captures: { roomb: { lease: lease() } }
    });
    const probed: string[] = [];
    await withServer(
      {
        viewer,
        warmRooms: ['rooma', 'roomb'],
        warmRoomIsLive: async (username) => {
          probed.push(username);
          return username === 'roomb'; // rooma's stream ended
        }
      },
      async (base) => {
        const res = await getJson(base, '/v1/session');
        expect(res.status).toBe(200);
        expect(res.body.wsUrl).toBe('wss://webcast-ws.example/ws');
        expect(probed).toEqual(['rooma', 'roomb']);
        expect(calls.captures).toEqual(['roomb']); // rooma never captured
      }
    );
  });

  it('still attempts the capture when the liveness probe fails', async () => {
    // A liveness-route outage must not make warm rooms uncapturable:
    // the probe erroring (or answering an unknown shape) means TRUE.
    const { viewer, calls } = stubbedViewer({
      leases: {},
      hasTabs: { rooma: false },
      captures: { rooma: { lease: lease() } }
    });
    await withServer(
      {
        viewer,
        warmRooms: ['rooma'],
        warmRoomIsLive: async () => {
          throw new Error('liveness route down');
        }
      },
      async (base) => {
        const res = await getJson(base, '/v1/session');
        expect(res.status).toBe(200);
        expect(calls.captures).toEqual(['rooma']);
      }
    );
  });

  it('falls through to the next candidate when a capture throws, accruing breaker failures', async () => {
    const breaker = new CaptureBreaker();
    const { viewer, calls } = stubbedViewer({
      leases: {},
      hasTabs: { rooma: false, roomb: false },
      captures: {
        rooma: { error: 'dead room' },
        // roomb captures but yields an EMPTY wsUrl: a soft failure that
        // accrues breaker state without registering a servable lease, so
        // every request re-runs the capture phase (no sticky warm lease).
        roomb: { lease: lease({ wsUrl: '' }) }
      }
    });
    await withServer({ viewer, warmRooms: ['rooma', 'roomb'], captureBreaker: breaker }, async (base) => {
      const res = await getJson(base, '/v1/session');
      expect(res.status).toBe(503);
      expect(calls.captures).toEqual(['rooma', 'roomb']);
      // Repeated failing rounds accrue: rooma throws, roomb resolves empty.
      await getJson(base, '/v1/session');
      await getJson(base, '/v1/session');
      expect(breaker.isRefused('rooma')).toBe(true);
      expect(breaker.isRefused('roomb')).toBe(true);
      // The refused rooms are skipped: the fourth request captures nothing.
      const before = calls.captures.length;
      const res4 = await getJson(base, '/v1/session');
      expect(res4.status).toBe(503);
      expect(calls.captures.length).toBe(before);
    });
  });

  it('records breaker success when a capture serves (working rooms are not paced)', async () => {
    const breaker = new CaptureBreaker();
    breaker.recordFailure('rooma');
    breaker.recordFailure('rooma');
    const { viewer } = stubbedViewer({
      leases: {},
      hasTabs: { rooma: false },
      captures: { rooma: { lease: lease() } }
    });
    await withServer({ viewer, warmRooms: ['rooma'], captureBreaker: breaker }, async (base) => {
      const res = await getJson(base, '/v1/session');
      expect(res.status).toBe(200);
      // The success cleared the streak: with the streak reset, ONE more
      // failure is the first of a new streak and must not trip the breaker.
      // Without the recordSuccess call this failure is the THIRD
      // consecutive one and isRefused flips true — the test fails.
      breaker.recordFailure('rooma');
      expect(breaker.isRefused('rooma')).toBe(false);
    });
  });

  it('closes unservable warm tabs in the recovery pass and cold-captures the first servable one', async () => {
    const { viewer, calls } = stubbedViewer({
      leases: { rooma: lease({ wsUrl: '' }), roomb: undefined },
      hasTabs: { rooma: true, roomb: true },
      captures: { rooma: { lease: lease() } }
    });
    await withServer({ viewer, warmRooms: ['rooma', 'roomb'] }, async (base) => {
      const res = await getJson(base, '/v1/session');
      expect(res.status).toBe(200);
      // Both unservable tabs (empty-wsUrl rooma, dead roomb) were closed...
      expect(calls.closedTabs).toEqual(['rooma', 'roomb']);
      // ...and the recovery captured exactly the dropped rooms in order,
      // ending on the first served lease (bounded loop, by design).
      expect(calls.captures).toEqual(['rooma']);
    });
  });

  it('never closes or captures a warm-but-empty room with an active relay subscriber', async () => {
    const relay = { hasSubscribers: (u: string) => u === 'rooma' } as unknown as RelayHub;
    const { viewer, calls } = stubbedViewer({
      leases: { rooma: lease({ wsUrl: '' }) },
      hasTabs: { rooma: true },
      captures: {}
    });
    await withServer({ viewer, warmRooms: ['rooma'], relay }, async (base) => {
      const res = await getJson(base, '/v1/session');
      expect(res.status).toBe(503);
      expect(res.body.error).toBe('no_warm_session');
      expect(calls.closedTabs).toEqual([]);
      expect(calls.captures).toEqual([]);
    });
  });

  it('recovers a dead registered tab (a Chromium crash heals, not 503 forever)', async () => {
    const { viewer, calls } = stubbedViewer({
      leases: { rooma: undefined },
      hasTabs: { rooma: true },
      captures: { rooma: { lease: lease() } }
    });
    await withServer({ viewer, warmRooms: ['rooma'] }, async (base) => {
      const res = await getJson(base, '/v1/session');
      expect(res.status).toBe(200);
      expect(calls.closedTabs).toEqual(['rooma']);
      expect(calls.captures).toEqual(['rooma']);
    });
  });

  it('answers 503 with the capture error when every capture fails', async () => {
    const { viewer } = stubbedViewer({
      leases: {},
      hasTabs: { rooma: false },
      captures: { rooma: { error: 'room offline' } }
    });
    await withServer({ viewer, warmRooms: ['rooma'] }, async (base) => {
      const res = await getJson(base, '/v1/session');
      expect(res.status).toBe(503);
      expect(res.body.error).toBe('no_warm_session');
      expect(res.body.message).toBe('room offline');
    });
  });

  it('answers 404 when warm rooms are unset, 503 when viewer mode is off', async () => {
    await withServer({}, async (base) => {
      const disabled = await getJson(base, '/v1/session');
      expect(disabled.status).toBe(404);
      expect(disabled.body.error).toBe('session_endpoint_disabled');
    });
    await withServer({ warmRooms: ['rooma'] }, async (base) => {
      const off = await getJson(base, '/v1/session');
      expect(off.status).toBe(503);
      expect(off.body.error).toBe('viewer mode not enabled');
    });
  });

  it('enforces the bearer token like /v1/sign', async () => {
    const { viewer } = stubbedViewer({ leases: { rooma: lease() } });
    const token = 'sekrit';
    vi.stubEnv('SIGNER_AUTH_TOKEN', token);
    try {
      await withServer({ viewer, warmRooms: ['rooma'] }, async (base) => {
        const denied = await getJson(base, '/v1/session');
        expect(denied.status).toBe(401);
        const allowed = await getJson(base, '/v1/session', token);
        expect(allowed.status).toBe(200);
        // The POST routes mint signatures and drive captures — the same
        // gate must cover them (round-2 council: inserting the session
        // branch once deleted it and both routes answered 200 bare).
        expect(await post(base, '/v1/sign', { roomId: '42' })).toBe(401);
        expect(await post(base, '/v1/sign-url', { roomId: '42' })).toBe(401);
      });
    } finally {
      vi.unstubAllEnvs();
    }
  });

  it('counts a resolved-but-empty capture under capture_failed, and the 503 under no_session', async () => {
    const { viewer } = stubbedViewer({
      leases: {},
      hasTabs: { rooma: false },
      captures: { rooma: { lease: lease({ wsUrl: '' }) } }
    });
    await withServer({ viewer, warmRooms: ['rooma'] }, async (base) => {
      // Before/after counts, not existence: the registry is module-global,
      // so earlier tests' increments would make a bare toBeDefined pass.
      const before = await leaseOutcomeCounts(base);
      const res = await getJson(base, '/v1/session');
      expect(res.status).toBe(503);
      const after = await leaseOutcomeCounts(base);
      // One resolved-but-empty capture attempt -> one capture_failed;
      // the exhausted phases -> one no_session.
      expect(after.capture_failed).toBe(before.capture_failed + 1);
      expect(after.no_session).toBe(before.no_session + 1);
    });
  });

  it('labels every bounded outcome: success, captured, disabled, viewer_off', async () => {
    // disabled: endpoint off.
    await withServer({}, async (base) => {
      const before = await leaseOutcomeCounts(base);
      await getJson(base, '/v1/session');
      const after = await leaseOutcomeCounts(base);
      expect(after.disabled).toBe(before.disabled + 1);
    });
    // viewer_off: warm rooms set, no viewer.
    await withServer({ warmRooms: ['rooma'] }, async (base) => {
      const before = await leaseOutcomeCounts(base);
      await getJson(base, '/v1/session');
      const after = await leaseOutcomeCounts(base);
      expect(after.viewer_off).toBe(before.viewer_off + 1);
    });
    // success: warm lease served.
    const warm = stubbedViewer({ leases: { rooma: lease() } });
    await withServer({ viewer: warm.viewer, warmRooms: ['rooma'] }, async (base) => {
      const before = await leaseOutcomeCounts(base);
      const res = await getJson(base, '/v1/session');
      expect(res.status).toBe(200);
      const after = await leaseOutcomeCounts(base);
      expect(after.success).toBe(before.success + 1);
    });
    // captured: a capture ran and was served.
    const cold = stubbedViewer({
      leases: {},
      hasTabs: { rooma: false },
      captures: { rooma: { lease: lease() } }
    });
    await withServer({ viewer: cold.viewer, warmRooms: ['rooma'] }, async (base) => {
      const before = await leaseOutcomeCounts(base);
      const res = await getJson(base, '/v1/session');
      expect(res.status).toBe(200);
      const after = await leaseOutcomeCounts(base);
      expect(after.captured).toBe(before.captured + 1);
    });
  });
});

describe('username canonicalization', () => {
  it('/v1/sign viewer path keys the room under the lowercased form', async () => {
    const breaker = new CaptureBreaker();
    const seen = { capture: '', breaker: '' };
    const viewer = {
      async captureRoom(username: string): Promise<RoomCapture> {
        seen.capture = username;
        return {
          roomId: '42',
          protoBase64: 'x',
          cookieHeader: '',
          userAgent: 'u',
          proxyHost: '',
          elapsedMs: 1,
          wsUrl: ''
        };
      }
    } as unknown as ViewerPool;
    const watchingBreaker = {
      isRefused(username: string) {
        seen.breaker = username;
        return false;
      },
      recordSuccess: () => undefined,
      recordFailure: () => false
    } as unknown as CaptureBreaker;
    await withServer({ viewer, captureBreaker: watchingBreaker }, async (base) => {
      const status = await post(base, '/v1/sign', { roomId: '42', username: 'SomeUser' });
      expect(status).toBe(200);
      // One canonical key: the breaker fast-fail and the pool capture both
      // saw the lowercased form — a mixed-case caller must not fork its
      // tab/breaker state across two keys.
      expect(seen.capture).toBe('someuser');
      expect(seen.breaker).toBe('someuser');
    });
  });

  it('/v1/stream lowercases the path before the canary check and the subscribe', async () => {
    const seen = { pin: '', subscribe: '' };
    const page = { close: async () => undefined };
    const viewer = {
      pinTab(username: string) {
        seen.pin = username;
        return page;
      }
    } as unknown as ViewerPool;
    const relay = {
      hasSubscribers: () => false,
      subscribe: async (username: string, _page: unknown) => {
        seen.subscribe = username;
        return () => undefined;
      }
    } as unknown as RelayHub;
    await withServer(
      { viewer, relay, canaryRooms: new Set(['someuser']) },
      async (base) => {
        // Mixed-case path against a lowercased canary set: without the
        // lowercasing this 404s ('room is not a canary') and never reaches
        // pinTab/subscribe; with it, one canonical key everywhere. SSE
        // streams stay open (heartbeat), so read the status and abort.
        const status = await new Promise<number>((resolve, reject) => {
          const req = http.get(`${base}/v1/stream/SomeUser`, (res) => {
            res.resume();
            resolve(res.statusCode ?? 0);
            req.destroy();
          });
          req.on('error', reject);
          req.setTimeout(2_000, () => {
            req.destroy();
            reject(new Error('stream status never arrived'));
          });
        });
        expect(status).toBe(200);
        expect(seen.pin).toBe('someuser');
        expect(seen.subscribe).toBe('someuser');
      }
    );
  });
});

/** Per-outcome counts of signer_session_leases_total, for before/after diffs. */
async function leaseOutcomeCounts(base: string): Promise<Record<string, number>> {
  const metrics = await scrape(base);
  const counts: Record<string, number> = {};
  for (const line of metrics.split('\n')) {
    if (!line.startsWith('signer_session_leases_total{')) continue;
    const outcome = line.match(/outcome="([^"]+)"/)?.[1] ?? '';
    counts[outcome] = Number(line.split(' ').pop());
  }
  return counts;
}
