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
  help: 'Sign requests by endpoint and outcome',
  labelNames: ['endpoint', 'outcome'],
  registers: [register]
});

const signRequestDuration = new Histogram({
  name: 'signer_sign_request_duration_seconds',
  help: 'End-to-end sign request latency by endpoint and outcome',
  labelNames: ['endpoint', 'outcome'],
  buckets: [0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30],
  registers: [register]
});

/** Shared bearer token; empty disables auth (cluster-internal NetworkPolicy path). */
function readAuthToken(): string {
  return (process.env.SIGNER_AUTH_TOKEN ?? '').trim();
}

export interface SignRequestPayload {
  roomId: string;
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

export interface ServerOptions {
  port: number;
  session: SigningSession;
  logger?: { info: (msg: string, meta?: Record<string, unknown>) => void; error: (msg: string, meta?: Record<string, unknown>) => void };
}

export function createServer(options: ServerOptions): http.Server {
  const { session, logger } = options;
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
      const identity = session.identity;
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(identity));
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
      const startedAt = Date.now();
      let outcome: SignRequestOutcome;
      let result: RouteResult;
      try {
        result =
          url === '/v1/sign'
            ? await performSignedFetch(session, session.identity, payload as SignRequestPayload)
            : await performSignUrl(session, payload as SignUrlRequestPayload);
      } catch (error) {
        // Thrown sign errors are the signing session itself failing (browser
        // dead, rebuild loop). Counted, then surfaced as 500 to the caller —
        // the listener classifies it via the reason set on its side.
        outcome = 'signer_error';
        signRequestsTotal.inc({ endpoint, outcome });
        signRequestDuration.observe({ endpoint, outcome }, (Date.now() - startedAt) / 1000);
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
      signRequestsTotal.inc({ endpoint, outcome });
      signRequestDuration.observe({ endpoint, outcome }, (Date.now() - startedAt) / 1000);

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
