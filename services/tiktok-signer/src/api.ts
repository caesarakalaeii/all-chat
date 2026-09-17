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
 * HTTP API of the signer service. Two endpoints, one per Euler signing seam
 * (see ADR-0052, "There are two Euler signing seams"):
 *
 *  - POST /v1/sign — the webcast WebSocket seam. Signs TikTok's
 *    `/webcast/im/fetch/` and executes it, returning the protobuf body,
 *    Set-Cookie header and room ID exactly the way Euler's
 *    `/webcast/fetch` did, because that is what the connector's
 *    `fetchSignedWebSocketFromProvider` expects back.
 *
 *  - POST /v1/sign-url — the generic HTTP URL seam. Signs any TikTok webcast
 *    URL (gift list and friends) and returns the signed URL plus the identity
 *    to send it with. This is what unblocks `TIKTOK_EXTENDED_GIFT_INFO`.
 *
 *  - GET /v1/identity — the stable browser identity callers must pin their
 *    connector device presets to; signatures are bound to it.
 */

import http from 'node:http';
import { collectDefaultMetrics, Counter, Histogram, Registry } from 'prom-client';
import { request as undiciRequest } from 'undici';
import { SigningSession, type SignerIdentity } from './signing/session.js';
import { CaptureBreaker } from './signing/capture-breaker.js';
import { VIEWER_IDENTITY, type ViewerPool } from './signing/viewer.js';
import { RelayHub, type RelayLogger, type RelayMessage } from './signing/relay.js';

const register = new Registry();
collectDefaultMetrics({ register });

/**
 * Outcome of one sign request, as a bounded metric label. `tiktok_rejected`
 * separates "our signature was computed but TikTok refused it" (the arms-race
 * signal, ADR-0052) from transport and payload problems.
 */
export type SignRequestOutcome = 'success' | 'tiktok_rejected' | 'bad_request' | 'signer_error';

const signRequestsTotal = new Counter({
  name: 'signer_sign_requests_total',
  help: 'Sign requests by endpoint, path and outcome. path=viewer_capture is the headless-Chromium viewer-tab capture (tens of seconds by design); path=signature is the in-page SDK sign',
  labelNames: ['endpoint', 'path', 'outcome'],
  registers: [register]
});

const signRequestDuration = new Histogram({
  name: 'signer_sign_request_duration_seconds',
  help: 'End-to-end sign request latency by endpoint, path and outcome. path=viewer_capture is the headless-Chromium viewer-tab capture (tens of seconds by design); path=signature is the in-page SDK sign',
  labelNames: ['endpoint', 'path', 'outcome'],
  buckets: [0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60, 120],
  registers: [register]
});

// Capture breaker visibility (2026-09-16 transport plan, phase 1). Refusals
// are the fast-fail path working; trips mean a room entered a cooldown.
const captureBreakerRefusals = new Counter({
  name: 'signer_capture_breaker_refusals_total',
  help: 'Viewer captures refused by the per-room breaker (fast 502 instead of pool time)',
  labelNames: ['username'],
  registers: [register]
});

const captureBreakerTrips = new Counter({
  name: 'signer_capture_breaker_trips_total',
  help: 'Rooms that tripped the capture breaker (threshold consecutive failures)',
  labelNames: ['username'],
  registers: [register]
});

/** Shared bearer token; empty disables auth (cluster-internal NetworkPolicy path). */
function readAuthToken(): string {
  return (process.env.SIGNER_AUTH_TOKEN ?? '').trim();
}

export interface SignRequestPayload {
  roomId: string;
  /** Streamer handle; enables the page-viewer path when viewer mode is on. */
  username?: string;
  cursor?: string;
  cookieHeader?: string;
}

export interface SignResponsePayload {
  /** /im/fetch/ protobuf body, base64. Caller decodes with the connector's schemas. */
  fetchResult: string;
  /** Set-Cookie content the response carried; the caller's cookie jar absorbs it. */
  fetchResultCookieHeader: string;
  /** Room ID TikTok actually served, when it redirects. */
  fetchResultRoomId?: string;
  /**
   * Viewer mode only: proxy host:port the capture rode. The signed session
   * is bound to that egress IP — the caller's WebSocket to TikTok must
   * egress via the same proxy or TikTok rejects the handshake. Empty string
   * when the capture went direct (no proxy lanes configured).
   */
  fetchResultProxyHost?: string;
}

export interface SignUrlRequestPayload {
  url: string;
  method?: string;
}

export interface SignUrlResponsePayload {
  response: {
    signedUrl: string;
    userAgent?: string;
  };
}

interface RouteResult {
  status: number;
  body: unknown;
}


/** 19 random digits, the device-id shape TikTok's web client uses. */
function randomDeviceId(): string {
  let digits = '';
  for (let i = 0; i < 19; i++) digits += Math.floor(Math.random() * 10);
  return digits;
}

/** The query params TikTok's own web client sends on /webcast/im/fetch/. */
function imFetchParams(roomId: string, cursor: string | undefined): URLSearchParams {
  const params = new URLSearchParams({
    aid: '1988',
    app_language: 'en',
    app_name: 'tiktok_web',
    browser_language: 'en-US',
    browser_name: 'Mozilla',
    browser_online: 'true',
    browser_platform: 'MacIntel',
    browser_version: '5.0',
    channel: 'tiktok_web',
    cookie_enabled: 'true',
    cursor: cursor ?? '',
    debug: 'false',
    device_platform: 'web',
    did_rule: '3',
    fetch_rule: '1',
    focus_state: 'true',
    from_page: 'user',
    history_comment_count: '6',
    identity: 'audience',
    internal_ext: '',
    is_fullscreen: 'false',
    is_page_visible: 'true',
    last_rtt: '0',
    live_id: '12',
    os: 'mac',
    priority_region: 'US',
    region: 'US',
    resp_content_type: 'protobuf',
    screen_height: '1080',
    screen_width: '1920',
    sup_ws_ds_opt: '1',
    tz_name: 'UTC',
    user_is_login: 'false',
    webcast_language: 'en',
    // 19-digit random device id, same shape the connector generates per
    // connection (generateDeviceId). It identifies the "device" making the
    // fetch and must be present or TikTok 403s.
    device_id: randomDeviceId(),
    room_id: roomId
  });
  return params;
}

/**
 * Sign + execute TikTok's /webcast/im/fetch/ and return the raw exchange the
 * connector needs. The fetch goes through undici (not the browser): the
 * signature describes the request, the browser itself is only needed to compute
 * it.
 */
async function performSignedFetch(
  session: SigningSession,
  identity: SignerIdentity,
  payload: SignRequestPayload
): Promise<RouteResult> {
  if (!payload.roomId || typeof payload.roomId !== 'string') {
    return { status: 400, body: { error: 'roomId is required' } };
  }

  const target = new URL('https://webcast.tiktok.com/webcast/im/fetch/');
  imFetchParams(payload.roomId, payload.cursor).forEach((value, key) => {
    target.searchParams.set(key, value);
  });

  const signed = await session.signUrl(target.toString());

  const headers: Record<string, string> = {
    'User-Agent': signed.userAgent,
    Accept: 'text/html,application/json,application/protobuf',
    Referer: 'https://www.tiktok.com/',
    Origin: 'https://www.tiktok.com',
    'Accept-Language': 'en-US,en;q=0.9'
  };
  if (signed.cookies) {
    // The browser's cookies (ttwid, msToken-carrier et al.) are what make the
    // fetch look like it comes from the warmed session.
    headers.Cookie = signed.cookies;
  }
  if (payload.cookieHeader) {
    // Authenticated connect: the caller's session cookies. Merge by letting
    // the caller's value win: it is the authenticated identity.
    headers.Cookie = signed.cookies
      ? `${signed.cookies}; ${payload.cookieHeader}`
      : payload.cookieHeader;
  }

  const response = await undiciRequest(signed.signedUrl, {
    method: 'GET',
    headers
  });

  if (response.statusCode === 429) {
    return {
      status: 429,
      body: { error: 'rate_limited', message: 'TikTok rate limited the sign target' }
    };
  }
  if (response.statusCode !== 200) {
    return {
      status: 502,
      body: {
        error: 'sign_target_error',
        message: `TikTok returned ${response.statusCode} for /webcast/im/fetch/`
      }
    };
  }

  const body = Buffer.from(await response.body.arrayBuffer());
  const setCookieHeaders = response.headers['set-cookie'];
  let fetchResultCookieHeader = '';
  if (Array.isArray(setCookieHeaders)) {
    fetchResultCookieHeader = setCookieHeaders.map((c) => c.split(';')[0]).join('; ');
  } else if (typeof setCookieHeaders === 'string') {
    fetchResultCookieHeader = setCookieHeaders.split(';')[0];
  }

  return {
    status: 200,
    body: {
      fetchResult: body.toString('base64'),
      fetchResultCookieHeader,
      fetchResultRoomId:
        typeof response.headers['x-room-id'] === 'string'
          ? (response.headers['x-room-id'] as string)
          : undefined
    } satisfies SignResponsePayload
  };
}

async function performSignUrl(
  session: SigningSession,
  payload: SignUrlRequestPayload
): Promise<RouteResult> {
  if (!payload.url || typeof payload.url !== 'string') {
    return { status: 400, body: { error: 'url is required' } };
  }

  let parsed: URL;
  try {
    parsed = new URL(payload.url);
  } catch {
    return { status: 400, body: { error: 'url is not a valid URL' } };
  }
  if (parsed.hostname !== 'webcast.tiktok.com') {
    // This service signs webcast requests only; anything else is a caller bug.
    return { status: 400, body: { error: 'only webcast.tiktok.com URLs can be signed' } };
  }

  const signed = await session.signUrl(payload.url);
  return {
    status: 200,
    body: {
      response: {
        signedUrl: signed.signedUrl,
        userAgent: signed.userAgent
      }
    } satisfies SignUrlResponsePayload
  };
}

/**
 * Page-viewer fetch: open (or reuse) a real viewer tab on the streamer's live
 * page and capture the SDK-signed /im/fetch/ response TikTok serves the
 * player. Measured 2026-09-15: TikTok answers the same request made outside a
 * real browser session with an empty 200 (signature path) or 403, so the
 * viewer capture is the only shape that still yields a full
 * ProtoMessageFetchResult. See ADR-0052's follow-up and viewer.ts.
 */
async function performViewerFetch(
  viewer: ViewerPool,
  breaker: CaptureBreaker,
  payload: SignRequestPayload,
  logger?: ServerOptions['logger']
): Promise<RouteResult> {
  const username = payload.username;
  if (!username || typeof username !== 'string') {
    return { status: 400, body: { error: 'username is required for viewer mode' } };
  }

  // Per-room breaker: a room whose captures keep failing is gated on
  // TikTok's side (room-correlated, per the 2026-09-16 lab verdict), and
  // paying maxLaneAttempts x ~60s per retry cycle for it wedges the pool
  // behind rooms that would capture fine. Fast-fail instead; the listener's
  // own backoff paces the retry.
  if (breaker.isRefused(username)) {
    captureBreakerRefusals.inc({ username });
    return {
      status: 502,
      body: {
        error: 'viewer_capture_refused',
        message: `capture breaker open for ${username}; retry in ${Math.ceil(breaker.refusedForMs(username) / 1000)}s`
      }
    };
  }

  try {
    const capture = await viewer.captureRoom(username);
    breaker.recordSuccess(username);
    return {
      status: 200,
      body: {
        fetchResult: capture.protoBase64,
        fetchResultCookieHeader: capture.cookieHeader,
        fetchResultRoomId: capture.roomId,
        fetchResultProxyHost: capture.proxyHost
      } satisfies SignResponsePayload
    };
  } catch (error) {
    const tripped = breaker.recordFailure(username);
    if (tripped) {
      captureBreakerTrips.inc({ username });
      logger?.error?.('capture breaker tripped for room', {
        username,
        error: (error as Error).message,
        refused_for_ms: breaker.refusedForMs(username)
      });
    }
    // Viewer captures fail when the room is not live (player never fetches)
    // or TikTok withholds data from the session. Both are TikTok-side
    // rejections from the caller's point of view: 502 keeps the listener's
    // failure classification alert-legible (reason="signature").
    return {
      status: 502,
      body: {
        error: 'viewer_capture_failed',
        message: (error as Error).message
      }
    };
  }
}

export interface ServerOptions {
  port: number;
  session: SigningSession;
  /**
   * Page-viewer pool. When set and the request carries a username, /v1/sign is
   * served by a real viewer tab instead of the signature path (ADR-0052
   * post-2026-09-09: TikTok gates webcast data on browser-grade sessions).
   */
  viewer?: ViewerPool;
  /**
   * Per-room capture breaker for the viewer path. Defaults to a breaker with
   * the standard thresholds; injectable for tests.
   */
  captureBreaker?: CaptureBreaker;
  /**
   * Canary relay hub. Present when relay mode is configured
   * (SIGNER_RELAY_CANARY_ROOMS non-empty); enables GET /v1/stream/:username.
   */
  relay?: RelayHub;
  /**
   * Rooms allowed to be relayed (canary set, SIGNER_RELAY_CANARY_ROOMS).
   * Requests for rooms outside the set answer 404.
   */
  canaryRooms?: ReadonlySet<string>;
  /**
   * Fallback gate (phase 3, SIGNER_RELAY_FALLBACK=on): when on, the relay
   * endpoint serves any room with a warm tab, not just the canary set —
   * premium rooms the listener promotes after flap exhaustion. The canary
   * set stays the read-only mirror cohort; this gate is the delivery tier.
   */
  relayFallbackEnabled?: boolean;
  logger?: { info: (msg: string, meta?: Record<string, unknown>) => void; error: (msg: string, meta?: Record<string, unknown>) => void };
}

export function createServer(options: ServerOptions): http.Server {
  const { session, viewer, logger } = options;
  const captureBreaker = options.captureBreaker ?? new CaptureBreaker();
  const relay = options.relay;
  const canaryRooms = options.canaryRooms ?? new Set<string>();
  const authToken = readAuthToken();

  const server = http.createServer((req, res) => {
    void handle(req, res).catch((error: unknown) => {
      logger?.error('sign request failed', { error: (error as Error).message });
      if (!res.headersSent) {
        res.writeHead(500, { 'Content-Type': 'application/json' });
      }
      res.end(JSON.stringify({ error: 'internal_error' }));
    });
  });

  async function handle(req: http.IncomingMessage, res: http.ServerResponse): Promise<void> {
    const url = req.url ?? '/';

    if (req.method === 'GET' && url === '/health/live') {
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ status: 'ok' }));
      return;
    }

    // Prometheus scrape. No auth: cluster-internal behind the default-deny
    // NetworkPolicy, same trust boundary as the other allchat /metrics ports.
    if (req.method === 'GET' && url === '/metrics') {
      try {
        res.writeHead(200, { 'Content-Type': register.contentType });
        res.end(await register.metrics());
      } catch (error) {
        logger?.error('metrics scrape failed', { error: (error as Error).message });
        res.writeHead(500);
        res.end();
      }
      return;
    }

    if (req.method === 'GET' && url === '/v1/identity') {
      // Viewer mode: report the viewer's identity, not the signature
      // session's. The connector pins its presets to this and the session
      // TikTok judges the WS handshake against was captured by the viewer —
      // a mismatch makes TikTok answer the upgrade with HTTP 200.
      const identity = viewer ? VIEWER_IDENTITY : session.identity;
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(identity));
      return;
    }

    // Canary relay: SSE stream of the room's viewer-tab WS frames. Same
    // bearer token as /v1/sign; long-lived GET, no body.
    const streamMatch =
      req.method === 'GET' ? url.match(/^\/v1\/stream\/([A-Za-z0-9_.]+)$/) : null;
    if (streamMatch) {
      const streamUsername = streamMatch[1];
      if (authToken && req.headers.authorization !== `Bearer ${authToken}`) {
        res.writeHead(401, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ error: 'unauthorized' }));
        return;
      }
      if (!canaryRooms.has(streamUsername) && !options.relayFallbackEnabled) {
        res.writeHead(404, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ error: 'room is not a canary' }));
        return;
      }
      if (!viewer || !relay) {
        res.writeHead(503, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ error: 'relay mode not enabled' }));
        return;
      }
      await handleRelayStream(req, res, streamUsername, viewer, relay, logger);
      return;
    }

    if (authToken && req.headers.authorization !== `Bearer ${authToken}`) {
      res.writeHead(401, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'unauthorized' }));
      return;
    }

    if (req.method === 'POST' && (url === '/v1/sign' || url === '/v1/sign-url')) {
      const body = await readBody(req);
      if (body === null) {
        res.writeHead(413, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ error: 'payload too large' }));
        return;
      }
      let payload: unknown;
      try {
        payload = body.length === 0 ? {} : JSON.parse(body.toString('utf-8'));
      } catch {
        res.writeHead(400, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ error: 'invalid JSON body' }));
        return;
      }

      const endpoint = url === '/v1/sign' ? 'sign' : 'sign_url';
      const isViewerPath =
        url === '/v1/sign' && Boolean(viewer) && Boolean((payload as SignRequestPayload).username);
      const path = isViewerPath ? 'viewer_capture' : 'signature';
      const startedAt = Date.now();
      let outcome: SignRequestOutcome;
      let result: RouteResult;
      try {
        if (url === '/v1/sign' && viewer && (payload as SignRequestPayload).username) {
          // Page-viewer path: a real tab on the room's live page captures the
          // SDK-signed im/fetch TikTok serves the player. Returns the same
          // { fetchResult, fetchResultCookieHeader } contract.
          result = await performViewerFetch(viewer, captureBreaker, payload as SignRequestPayload, logger);
        } else {
          result =
            url === '/v1/sign'
              ? await performSignedFetch(session, session.identity, payload as SignRequestPayload)
              : await performSignUrl(session, payload as SignUrlRequestPayload);
        }
      } catch (error) {
        // Thrown sign errors are the signing session itself failing (browser
        // dead, rebuild loop). Counted, then surfaced as 500 to the caller —
        // the listener classifies it via the reason set on its side.
        outcome = 'signer_error';
        signRequestsTotal.inc({ endpoint, path, outcome });
        signRequestDuration.observe({ endpoint, path, outcome }, (Date.now() - startedAt) / 1000);
        logger?.error('sign request failed', { endpoint, error: (error as Error).message });
        res.writeHead(500, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify({ error: 'signer_error', message: (error as Error).message }));
        return;
      }
      outcome =
        result.status === 200
          ? 'success'
          : result.status === 400
            ? 'bad_request'
            : 'tiktok_rejected';
      signRequestsTotal.inc({ endpoint, path, outcome });
      signRequestDuration.observe({ endpoint, path, outcome }, (Date.now() - startedAt) / 1000);

      res.writeHead(result.status, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(result.body));
      return;
    }

    res.writeHead(404, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: 'not found' }));
  }

  return server;
}

const MAX_BODY_BYTES = 64 * 1024;

async function readBody(req: http.IncomingMessage): Promise<Buffer | null> {
  const chunks: Buffer[] = [];
  let total = 0;
  const { promise, resolve, reject } = Promise.withResolvers<Buffer | null>();
  req.on('data', (chunk: Buffer) => {
    total += chunk.length;
    if (total > MAX_BODY_BYTES) {
      reject(null);
      req.destroy();
      return;
    }
    chunks.push(chunk);
  });
  req.on('end', () => resolve(Buffer.concat(chunks)));
  req.on('error', (error: Error) => reject(error));
  try {
    return await promise;
  } catch (error) {
    // A destroyed oversized request resolves as null (413); real errors rethrow.
    if (error === null) return null;
    throw error;
  }
}

/** Heartbeat comment interval for the SSE relay stream. */
const SSE_HEARTBEAT_MS = 15_000;

/**
 * SSE stream of a canary room's viewer-tab frames. One request = one
 * subscriber; frames are opaque base64 PushFrames the listener decodes
 * with its own connector schemas.
 */
async function handleRelayStream(
  req: http.IncomingMessage,
  res: http.ServerResponse,
  username: string,
  viewer: ViewerPool,
  relay: RelayHub,
  logger?: RelayLogger
): Promise<void> {
  // The room must already have a warm tab (a capture ran for it — the
  // listener's pre-sign guarantees this on every connect). Pin it first so
  // idle eviction cannot reclaim the tab between pin and attach.
  const page = viewer.pinTab(username);
  if (!page) {
    res.writeHead(409, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: 'no warm tab for room; run a capture first' }));
    return;
  }

  res.writeHead(200, {
    'Content-Type': 'text/event-stream',
    'Cache-Control': 'no-cache',
    Connection: 'keep-alive'
  });
  res.write(': connected\n\n');

  const unsubscribe = await relay.subscribe(username, page, (msg: RelayMessage) => {
    if (msg.type === 'frame') {
      res.write(`event: frame\ndata: ${JSON.stringify(msg)}\n\n`);
    } else {
      res.write(`event: state\ndata: ${JSON.stringify(msg)}\n\n`);
    }
  });

  const heartbeat = setInterval(() => {
    res.write(': ping\n\n');
  }, SSE_HEARTBEAT_MS);

  const cleanup = () => {
    clearInterval(heartbeat);
    unsubscribe();
    logger?.info('relay stream closed', { username });
  };
  req.on('close', cleanup);
  res.on('close', cleanup);
}
