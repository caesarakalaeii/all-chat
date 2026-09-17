import { afterEach, describe, expect, it, vi } from 'vitest';
import http from 'node:http';
import type { AddressInfo } from 'node:net';
import { createServer, type ServerOptions } from './api.js';
import type { SigningSession, SignerIdentity } from './signing/session.js';
import type { ViewerPool, RoomCapture } from './signing/viewer.js';

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
  const identity: SignerIdentity = {
    userAgent: 'test-ua',
    signUrl: async (url: string) => ({
      signedUrl: url,
      userAgent: 'test-ua',
      cookies: ''
    })
  };
  return identity as unknown as SigningSession;
}

function stubViewer(): ViewerPool {
  return {
    captureRoom: async (username: string): Promise<RoomCapture> => ({
      roomId: '42',
      protoBase64: Buffer.alloc(1200, 'x').toString('base64'),
      cookieHeader: '',
      userAgent: 'test-ua',
      proxyHost: '',
      elapsedMs: 1
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
});
