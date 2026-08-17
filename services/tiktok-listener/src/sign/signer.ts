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
 * The signer interface: the one thing Euler Stream does for us that the connector has no
 * direct-to-TikTok alternative for (issue #698).
 *
 * Everything else the connector asks Euler for — room ID, room info, is-live, gift list — has a
 * route that talks to TikTok directly. The WebSocket signature does not, which is why this is
 * the only abstraction in this directory and why the rest of the retirement is just flag flips.
 *
 * The shape here is deliberately the connector's own: `fetchSignedWebSocketFromEulerRoute`
 * resolves to `{ fetchResult, fetchResultCookieHeader, fetchResultRoomId }`, so an implementation
 * of `WebcastSigner` can be dropped into `RouteConfig.fetchSignedWebSocketFromProvider` with an
 * adapter that is almost nothing. Keeping the boundary here rather than at the connector's own
 * types also means a sign *service* (issue step 3) can serve exactly this JSON without inventing
 * a second vocabulary.
 */

/** What the caller knows about the room it wants a signed socket for. */
export interface SignRequest {
  /** TikTok room ID to connect to. */
  roomId: string;

  /**
   * Cursor to resume from, if this is a reconnect rather than a fresh join.
   *
   * TikTok uses the cursor to decide which backlog to replay. Omitting it on a reconnect is not
   * fatal, it just means the gap is not filled.
   */
  cursor?: string;

  /**
   * User-Agent the signature should be bound to.
   *
   * The signature and the request that carries it have to agree: TikTok checks. The connector
   * randomises a device preset per connection, so this is per-request rather than per-signer.
   */
  userAgent: string;

  /**
   * Cookie header to bind the signature to an authenticated TikTok session, if any.
   *
   * Populated only for authenticated connections. Note the reason this abstraction exists at
   * all is partly this field: with Euler, this cookie is forwarded to a third party's sign
   * server. Signing ourselves keeps the credential inside our own trust boundary.
   */
  cookieHeader?: string;
}

/**
 * A signed WebSocket handshake, in the form the connector needs to proceed.
 *
 * Deliberately mirrors `FetchSignedWebSocketFromEulerRouteResponse`. `fetchResult` is the decoded
 * `ProtoMessageFetchResult`, which carries the push server URL, the initial message batch, the
 * cursor and `internalExt`; the connector cannot connect without all four.
 */
export interface SignResult {
  /** Decoded `ProtoMessageFetchResult` from TikTok's `/im/fetch/` response. */
  fetchResult: unknown;

  /** `Set-Cookie` content TikTok returned with the fetch, which the cookie jar must absorb. */
  fetchResultCookieHeader: string;

  /**
   * Room ID TikTok actually served, when it differs from the one we asked for.
   *
   * TikTok occasionally redirects a room; the connector re-points at this value when present.
   */
  fetchResultRoomId?: string;
}

/**
 * Something that can produce a signed webcast WebSocket handshake.
 *
 * Implementations: `EulerSigner` (delegates to the library's Euler route) and, once the spike
 * lands, our own. `ShadowSigner` composes two of these.
 */
export interface WebcastSigner {
  /** Stable identifier used in logs and in the `signer` metric label. */
  readonly name: string;

  /**
   * Sign and perform the initial fetch for a room.
   *
   * @param request What room to join, and the identity to bind the signature to.
   * @throws When the signature cannot be produced or TikTok rejects it. Callers treat any throw
   *         as a signature failure for metrics purposes, so implementations should not swallow.
   */
  sign(request: SignRequest): Promise<SignResult>;
}

/**
 * Raised when a signer cannot produce a signature.
 *
 * Carries the signer's name so the fallback path can say which one failed without the caller
 * having to thread it through, and so the alert on signature failure rate (asked for in the
 * issue) can be broken down by signer rather than being a single undifferentiated number.
 */
export class SignatureFailure extends Error {
  readonly signer: string;
  override readonly cause?: unknown;

  constructor(signer: string, message: string, cause?: unknown) {
    super(`[${signer}] ${message}`);
    this.name = 'SignatureFailure';
    this.signer = signer;
    this.cause = cause;
  }
}

/**
 * Classify a signature failure into a small, bounded set of reasons.
 *
 * Bounded because this becomes a Prometheus label, and an unbounded label set built from raw
 * error messages is how a metrics backend falls over. The categories are chosen to separate the
 * three operationally distinct cases:
 *
 *  - `rate_limit`  — Euler's free-tier ceiling. The thing #698 exists to escape, and the reason
 *                    this classification matters: it should visibly go to zero as we cut over.
 *  - `paywall`     — Euler wants money for this endpoint. Also escapable.
 *  - `signature`   — our signature was produced but TikTok rejected it. This is the arms-race
 *                    signal, and the one that should page.
 *  - `network`     — transport failure reaching whoever signs. Usually not our algorithm's fault.
 *  - `unknown`     — everything else.
 *
 * @param error The thrown value, of any shape.
 */
export function classifySignatureFailure(error: unknown): string {
  const name = (error as { name?: unknown })?.name;
  if (typeof name === 'string') {
    if (name === 'SignatureRateLimitError') return 'rate_limit';
    if (name === 'PremiumFeatureError') return 'paywall';
    if (name === 'SignatureMissingTokensError') return 'signature';
  }

  const message = (
    error instanceof Error ? error.message : String(error ?? '')
  ).toLowerCase();

  // The connector's own 429 handling reads `retry-after` off a response it never checked for
  // undefined, so an exhausted free tier can surface as this TypeError rather than as
  // SignatureRateLimitError. Observed on 2026-08-14 and called out in #698; without this arm the
  // single most important failure mode lands in `unknown`.
  //
  // Matched on the property name alone rather than on V8's full wording, because that wording is
  // not stable: the same fault reads "Cannot read properties of undefined (reading 'retry-after')"
  // on current V8 and "Cannot read property 'retry-after' of undefined" on older ones. Tying the
  // rate-limit signal to one engine's phrasing would make it silently stop working on a runtime
  // upgrade, which is the worst possible time to lose it.
  if (message.includes('retry-after') && message.includes('undefined')) return 'rate_limit';

  if (message.includes('rate limit') || message.includes('too many')) return 'rate_limit';
  if (message.includes('business plan') || message.includes('premium')) return 'paywall';
  if (
    message.includes('econnrefused') ||
    message.includes('etimedout') ||
    message.includes('enotfound') ||
    message.includes('socket hang up') ||
    message.includes('failed to connect')
  ) {
    return 'network';
  }
  if (message.includes('signature') || message.includes('sign error')) return 'signature';

  return 'unknown';
}
