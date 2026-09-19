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
import { CaptureBreaker } from './signing/capture-breaker.js';
import { VIEWER_IDENTITY, browserVersionFromUserAgent } from './signing/identity.js';
const register = new Registry();
collectDefaultMetrics({ register });
const signRequestsTotal = new Counter({
    name: 'signer_sign_requests_total',
    help: 'Sign requests by endpoint, path and outcome. path=viewer_capture is the viewer-tab capture on a real-display Chromium (tens of seconds by design); path=signature is the in-page SDK sign',
    labelNames: ['endpoint', 'path', 'outcome'],
    registers: [register]
});
const signRequestDuration = new Histogram({
    name: 'signer_sign_request_duration_seconds',
    help: 'End-to-end sign request latency by endpoint, path and outcome. path=viewer_capture is the viewer-tab capture on a real-display Chromium (tens of seconds by design); path=signature is the in-page SDK sign',
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
function readAuthToken() {
    return (process.env.SIGNER_AUTH_TOKEN ?? '').trim();
}
/** 19 random digits, the device-id shape TikTok's web client uses. */
function randomDeviceId() {
    let digits = '';
    for (let i = 0; i < 19; i++)
        digits += Math.floor(Math.random() * 10);
    return digits;
}
/**
 * The query params TikTok's own web client sends on /webcast/im/fetch/. The
 * browser_* / os / screen_* params describe the identity that signs and
 * sends the fetch — TikTok silently empty-200s a request whose UA disagrees
 * with them (measured 2026-09-18), so they are derived from the identity,
 * never hardcoded.
 */
export function imFetchParams(roomId, cursor, identity) {
    const params = new URLSearchParams({
        aid: '1988',
        app_language: 'en',
        app_name: 'tiktok_web',
        browser_language: 'en-US',
        browser_name: 'Mozilla',
        browser_online: 'true',
        browser_platform: identity.browserPlatform,
        browser_version: browserVersionFromUserAgent(identity.userAgent),
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
        os: identity.os,
        priority_region: 'US',
        region: 'US',
        resp_content_type: 'protobuf',
        screen_height: String(identity.screenHeight),
        screen_width: String(identity.screenWidth),
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
async function performSignedFetch(session, identity, payload) {
    if (!payload.roomId || typeof payload.roomId !== 'string') {
        return { status: 400, body: { error: 'roomId is required' } };
    }
    const target = new URL('https://webcast.tiktok.com/webcast/im/fetch/');
    imFetchParams(payload.roomId, payload.cursor, identity).forEach((value, key) => {
        target.searchParams.set(key, value);
    });
    const signed = await session.signUrl(target.toString());
    const headers = {
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
    }
    else if (typeof setCookieHeaders === 'string') {
        fetchResultCookieHeader = setCookieHeaders.split(';')[0];
    }
    return {
        status: 200,
        body: {
            fetchResult: body.toString('base64'),
            fetchResultCookieHeader,
            fetchResultRoomId: typeof response.headers['x-room-id'] === 'string'
                ? response.headers['x-room-id']
                : undefined
        }
    };
}
async function performSignUrl(session, payload) {
    if (!payload.url || typeof payload.url !== 'string') {
        return { status: 400, body: { error: 'url is required' } };
    }
    let parsed;
    try {
        parsed = new URL(payload.url);
    }
    catch {
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
        }
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
async function performViewerFetch(viewer, breaker, payload, logger) {
    // One canonical lowercase username at every comparison site: pool tabs,
    // breaker state and the session endpoint's warm-room config all key on
    // the lowercased form, so a mixed-case caller must not fork its identity
    // across two keys.
    // Type-check before lowercasing: a non-string username must 400, not
    // throw inside toLowerCase and surface as a 500.
    if (typeof payload.username !== 'string' || payload.username === '') {
        return { status: 400, body: { error: 'username is required for viewer mode' } };
    }
    const username = payload.username.toLowerCase();
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
            }
        };
    }
    catch (error) {
        const tripped = breaker.recordFailure(username);
        if (tripped) {
            captureBreakerTrips.inc({ username });
            logger?.error?.('capture breaker tripped for room', {
                username,
                error: error.message,
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
                message: error.message
            }
        };
    }
}
const sessionLeasesTotal = new Counter({
    name: 'signer_session_leases_total',
    help: 'Session lease requests by outcome. success = warm lease served; captured = a capture ran and was served; capture_failed = a capture threw or resolved empty (counted per attempt); no_session = phases exhausted with no servable lease; disabled = endpoint off; viewer_off = viewer mode off.',
    labelNames: ['outcome'],
    registers: [register]
});
/** Count one request outcome. Bounded labels keep the metric cheap to query. */
function leaseOutcome(outcome) {
    sessionLeasesTotal.inc({ outcome });
}
export function createServer(options) {
    const { session, viewer, logger } = options;
    const captureBreaker = options.captureBreaker ?? new CaptureBreaker();
    const relay = options.relay;
    const canaryRooms = options.canaryRooms ?? new Set();
    const warmRooms = options.warmRooms ?? [];
    const authToken = readAuthToken();
    const server = http.createServer((req, res) => {
        void handle(req, res).catch((error) => {
            logger?.error('sign request failed', { error: error.message });
            if (!res.headersSent) {
                res.writeHead(500, { 'Content-Type': 'application/json' });
            }
            res.end(JSON.stringify({ error: 'internal_error' }));
        });
    });
    async function handle(req, res) {
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
            }
            catch (error) {
                logger?.error('metrics scrape failed', { error: error.message });
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
        const streamMatch = req.method === 'GET' ? url.match(/^\/v1\/stream\/([A-Za-z0-9_.]+)$/) : null;
        if (streamMatch) {
            // Lowercase before every keyed consumer (canary set, pool tab, relay
            // subscriber) so one mixed-case caller cannot key its subscriber
            // apart from its pool tab.
            const streamUsername = streamMatch[1].toLowerCase();
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
        // Bearer gate for everything stateful: the lease endpoint, and the POST
        // routes that mint signatures or drive captures. (The SSE stream checks
        // the same token inline in its own branch; /metrics and /health stay
        // deliberately open.)
        if (authToken && req.headers.authorization !== `Bearer ${authToken}`) {
            res.writeHead(401, { 'Content-Type': 'application/json' });
            res.end(JSON.stringify({ error: 'unauthorized' }));
            return;
        }
        // Shared-session lease: the warm classic room's WS URL + cookie jar the
        // listener's pure-Node client re-enters per target room (cross-room
        // entry, measured 2026-09-18). No capture cadence — harvest only on
        // request (budget discipline, constraint 1).
        if (req.method === 'GET' && url === '/v1/session') {
            if (!warmRooms.length) {
                leaseOutcome('disabled');
                res.writeHead(404, { 'Content-Type': 'application/json' });
                res.end(JSON.stringify({ error: 'session_endpoint_disabled' }));
                return;
            }
            if (!viewer) {
                leaseOutcome('viewer_off');
                res.writeHead(503, { 'Content-Type': 'application/json' });
                res.end(JSON.stringify({ error: 'viewer mode not enabled' }));
                return;
            }
            await handleSessionLease(res, viewer, warmRooms, captureBreaker, relay);
            return;
        }
        if (req.method === 'POST' && (url === '/v1/sign' || url === '/v1/sign-url')) {
            const body = await readBody(req);
            if (body === null) {
                res.writeHead(413, { 'Content-Type': 'application/json' });
                res.end(JSON.stringify({ error: 'payload too large' }));
                return;
            }
            let payload;
            try {
                payload = body.length === 0 ? {} : JSON.parse(body.toString('utf-8'));
            }
            catch {
                res.writeHead(400, { 'Content-Type': 'application/json' });
                res.end(JSON.stringify({ error: 'invalid JSON body' }));
                return;
            }
            const endpoint = url === '/v1/sign' ? 'sign' : 'sign_url';
            // Route once: the metric's path label and the dispatch below must
            // read the same condition, or the histogram silently mislabels.
            const viewerPath = url === '/v1/sign' && Boolean(viewer) && Boolean(payload.username);
            const path = viewerPath ? 'viewer_capture' : 'signature';
            const startedAt = Date.now();
            let outcome;
            let result;
            try {
                if (viewerPath && viewer) {
                    // Page-viewer path: a real tab on the room's live page captures the
                    // SDK-signed im/fetch TikTok serves the player. Returns the same
                    // { fetchResult, fetchResultCookieHeader } contract.
                    result = await performViewerFetch(viewer, captureBreaker, payload, logger);
                }
                else {
                    result =
                        url === '/v1/sign'
                            ? await performSignedFetch(session, session.identity, payload)
                            : await performSignUrl(session, payload);
                }
            }
            catch (error) {
                // Thrown sign errors are the signing session itself failing (browser
                // dead, rebuild loop). Counted, then surfaced as 500 to the caller —
                // the listener classifies it via the reason set on its side.
                outcome = 'signer_error';
                signRequestsTotal.inc({ endpoint, path, outcome });
                signRequestDuration.observe({ endpoint, path, outcome }, (Date.now() - startedAt) / 1000);
                logger?.error('sign request failed', { endpoint, error: error.message });
                res.writeHead(500, { 'Content-Type': 'application/json' });
                res.end(JSON.stringify({ error: 'signer_error', message: error.message }));
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
async function readBody(req) {
    const chunks = [];
    let total = 0;
    const { promise, resolve, reject } = Promise.withResolvers();
    req.on('data', (chunk) => {
        total += chunk.length;
        if (total > MAX_BODY_BYTES) {
            reject(null);
            req.destroy();
            return;
        }
        chunks.push(chunk);
    });
    req.on('end', () => resolve(Buffer.concat(chunks)));
    req.on('error', (error) => reject(error));
    try {
        return await promise;
    }
    catch (error) {
        // A destroyed oversized request resolves as null (413); real errors rethrow.
        if (error === null)
            return null;
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
async function handleRelayStream(req, res, username, viewer, relay, logger) {
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
    const unsubscribe = await relay.subscribe(username, page, (msg) => {
        if (msg.type === 'frame') {
            res.write(`event: frame\ndata: ${JSON.stringify(msg)}\n\n`);
        }
        else {
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
/**
 * GET /v1/session: serve the shared WS session (warm lease, or capture a
 * tab-less room). Deterministic four-phase flow — the phases and their
 * recovery semantics are the plan's contract; see the PR 1 plan in
 * docs/phase-reports/ for the state-machine rationale.
 *
 * The per-room capture breaker (the same instance /v1/sign consults) paces
 * a dead or live_new configured room's retries. It does NOT prevent the
 * pool's own PROFILE_ROTATE_FAILURES profile rotation — that accrual is
 * unconditional on thrown captures, and its threshold equals the breaker's.
 * One rotation per breaker cooldown cycle is the priced cost of a
 * misconfigured warm room; the actual control is SIGNER_WARM_ROOMS listing
 * classic, verified-live rooms only (README).
 */
async function handleSessionLease(res, viewer, warmRooms, breaker, relay) {
    // Phase 1: warm leases. The per-room outcome (undefined lease = dead page,
    // empty wsUrl = unservable) feeds the recovery pass.
    const outcomes = [];
    for (const room of warmRooms) {
        const lease = await viewer.sessionLease(room);
        outcomes.push({ room, lease });
        if (lease && lease.wsUrl !== '') {
            leaseOutcome('success');
            res.writeHead(200, { 'Content-Type': 'application/json' });
            res.end(JSON.stringify(lease));
            return;
        }
    }
    // Phase 2: capture the tab-less, breaker-eligible rooms in order. A thrown
    // capture or a resolved-but-empty one moves to the next candidate; both
    // accrue breaker failures so a dead room's cooldown paces it.
    let lastError = '';
    const captureCandidates = async (rooms) => {
        for (const room of rooms) {
            if (viewer.hasTab(room) || breaker.isRefused(room))
                continue;
            try {
                const capture = await viewer.captureRoom(room);
                if (capture.wsUrl === '') {
                    breaker.recordFailure(room);
                    leaseOutcome('capture_failed');
                    continue;
                }
                breaker.recordSuccess(room);
                const lease = await viewer.sessionLease(room);
                if (lease && lease.wsUrl !== '') {
                    leaseOutcome('captured');
                    return lease;
                }
            }
            catch (error) {
                breaker.recordFailure(room);
                leaseOutcome('capture_failed');
                lastError = error.message;
            }
        }
        return undefined;
    };
    const captured = await captureCandidates(warmRooms);
    if (captured) {
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(captured));
        return;
    }
    // Phase 3: recovery pass. Close the tabs whose phase-1 lease could not
    // serve (dead page or empty wsUrl) — unless a relay subscriber is
    // attached — then cold-capture exactly the dropped rooms. This is the
    // only path that clears a dead registered entry: hasTab stays true
    // otherwise, so no capture is reachable, and pinned rooms never idle-evict.
    const dropped = [];
    for (const { room, lease } of outcomes) {
        if (!viewer.hasTab(room))
            continue;
        if (lease && lease.wsUrl !== '')
            continue; // servable; another room may serve later
        if (relay?.hasSubscribers(room))
            continue; // live SSE stream; never killed
        if (viewer.closeTab(room))
            dropped.push(room);
    }
    const recovered = dropped.length > 0 ? await captureCandidates(dropped) : undefined;
    if (recovered) {
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(recovered));
        return;
    }
    // Phase 4: no servable lease. Retryable: the breaker paces the rooms,
    // closed tabs re-capture cold, dead entries were cleared.
    leaseOutcome('no_session');
    res.writeHead(503, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({
        error: 'no_warm_session',
        ...(lastError ? { message: lastError } : {})
    }));
}
//# sourceMappingURL=api.js.map