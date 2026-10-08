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

// Runs on Cloud Run in a US region. TikTok answers a lookup by handle only
// within the region the request comes from, so accounts hosted by TikTok's US
// entity are user_not_found from our EU cluster. Everything after the lookup
// (check_alive, the WebSocket by room ID) works from the EU; only this one
// request needs a US egress.

import { createServer } from 'node:http';
import { createHash, timingSafeEqual } from 'node:crypto';

const HANDLE = /^[a-z0-9._]{2,24}$/;
const UA = 'Mozilla/5.0 (X11; Linux x86_64; rv:130.0) Gecko/20100101 Firefox/130.0';
const digest = (s) => createHash('sha256').update(s).digest();

// Not the Authorization header: Cloud Run's frontend treats any Bearer token
// there as a Google ID token and rejects ours before it reaches the service.
export function authorized(header, token) {
  return token.length > 0 && timingSafeEqual(digest(header ?? ''), digest(token));
}

export function summarize(body) {
  if (body.statusCode !== 0) {
    return { found: false, code: body.statusCode, message: body.message };
  }
  const room = body.data?.liveRoom ?? {};
  const user = body.data?.user ?? {};
  return {
    found: true,
    live: room.status === 2,
    status: room.status ?? null,
    roomId: user.roomId || null,
    userId: user.id ?? null,
  };
}

function send(res, code, body) {
  res.writeHead(code, { 'content-type': 'application/json' });
  res.end(JSON.stringify(body));
}

export function createRelay({ token, fetchImpl = fetch }) {
  return createServer(async (req, res) => {
    const url = new URL(req.url, 'http://relay');
    if (req.method !== 'GET' || url.pathname !== '/v1/live') return send(res, 404, { error: 'not_found' });
    if (!authorized(req.headers['x-relay-token'], token)) return send(res, 401, { error: 'unauthorized' });

    const handle = (url.searchParams.get('handle') ?? '').toLowerCase();
    if (!HANDLE.test(handle)) return send(res, 400, { error: 'bad_handle' });

    try {
      const upstream = await fetchImpl(
        `https://www.tiktok.com/api-live/user/room/?aid=1988&uniqueId=${handle}&sourceType=54`,
        { headers: { 'user-agent': UA }, signal: AbortSignal.timeout(10_000) }
      );
      send(res, 200, summarize(await upstream.json()));
    } catch (e) {
      send(res, 502, { error: 'upstream', detail: String(e?.message ?? e) });
    }
  });
}

if (import.meta.url === `file://${process.argv[1]}`) {
  const token = process.env.RELAY_TOKEN ?? '';
  if (!token) {
    console.error('RELAY_TOKEN is required');
    process.exit(1);
  }
  createRelay({ token }).listen(Number(process.env.PORT ?? 8080));
}
