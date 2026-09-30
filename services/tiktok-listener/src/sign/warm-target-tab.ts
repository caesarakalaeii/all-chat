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

/**
 * Warm a target-room viewer tab on the signer (PR 3 of the pure-Node
 * transport).
 *
 * POST /v1/sign on the viewer path opens (or reuses) a real tab on the
 * room's live page and registers it in the signer's ViewerPool — exactly
 * the tab the relay endpoint (GET /v1/stream/:username) needs before it
 * can attach its CDP tap. The lease endpoint (GET /v1/session) never
 * creates target-room tabs, which is why pure-node mode's canary mirror
 * and premium fallback could not attach before this module existed.
 *
 * The viewer path consumes only `username` — roomId is not read; 'unused'
 * mirrors the lane-pin pre-sign precedent (index.ts). The response body is
 * discarded: the tab registration is the purpose, not the fetchResult.
 *
 * ONE capture per call: this is the signer-side capture-breaker budget
 * (single-digit captures/hour) every caller must respect — the canary
 * calls it once per stint, promotion once per flap exhaustion.
 */

import { request } from 'undici';

export type WarmOutcome =
  | { ok: true }
  | { ok: false; reason: 'capture_refused' | 'capture_failed' | 'network' };

export interface WarmTargetTabOptions {
  /** Signer base URL, no trailing slash (SIGN_CONFIG.signerBaseUrl). */
  signerUrl: string;
  /** Bearer token when the signer runs with SIGNER_AUTH_TOKEN. */
  authToken?: string;
  username: string;
  /** Injectable fetch for tests. */
  fetchImpl?: typeof request;
}

/** Capture round-trip budget; mirrors the SelfSigner timeout. */
const TIMEOUT_MS = 180_000;

export async function warmTargetTab(options: WarmTargetTabOptions): Promise<WarmOutcome> {
  const baseUrl = options.signerUrl.replace(/\/+$/, '');
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (options.authToken) headers.Authorization = `Bearer ${options.authToken}`;

  let response: Awaited<ReturnType<typeof request>>;
  try {
    response = await (options.fetchImpl ?? request)(`${baseUrl}/v1/sign`, {
      method: 'POST',
      headers,
      body: JSON.stringify({ roomId: 'unused', username: options.username }),
      bodyTimeout: TIMEOUT_MS,
      headersTimeout: TIMEOUT_MS
    });
  } catch {
    // Unreachable signer or timeout: the warm did not happen; callers
    // treat this as relay_unavailable and retry on their own discipline,
    // never by re-warming immediately.
    return { ok: false, reason: 'network' };
  }

  // Status first: a 200 means the capture ran and the tab registered,
  // whatever the body looks like (a rewriting proxy must not turn a
  // spent capture budget into a reported failure).
  if (response.statusCode === 200) return { ok: true };

  let bodyJson: { error?: string } = {};
  if (response.statusCode === 502) {
    // The signer ran the capture and TikTok/the breaker refused it —
    // the two failure shapes the API distinguishes.
    try {
      const text = await response.body.text();
      bodyJson = text ? (JSON.parse(text) as { error?: string }) : {};
    } catch {
      // Undecodable refusal body: still a capture-side failure.
    }
    return { ok: false, reason: bodyJson.error === 'viewer_capture_refused' ? 'capture_refused' : 'capture_failed' };
  }
  // Anything else (401/404 config, 429, 5xx): no capture ran, but the warm
  // did not succeed. Network is the honest bucket — the bounded reason
  // sets callers already use do not need a fourth label for one-off HTTP
  // statuses.
  return { ok: false, reason: 'network' };
}
