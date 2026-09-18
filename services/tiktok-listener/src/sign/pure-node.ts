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
 * `WebcastSigner` that leases the signer service's shared WS session and
 * synthesizes the connector's initial fetch result in-process (PR 2 of the
 * pure-Node transport).
 *
 * One captured session — a warm classic room's WS URL + cookie jar, leased
 * via GET /v1/session — serves every room's delivery: the connector sends
 * `im_enter_room` for the target room on the leased socket (cross-room
 * entry, measured 2026-09-18). No Euler, no per-room page capture.
 *
 * Measured constraints this implementation encodes (spike report, PR 2
 * Task 1, 2026-09-18):
 *  - The lease wsUrl's recorded identity params are LOAD-BEARING at the WS
 *    handshake: a bare push origin with only compress/room_id/internal_ext/
 *    cursor is rejected ("Unexpected server response: 200"). The synthesized
 *    fetchResult therefore forwards every recorded param except the four the
 *    connector appends itself.
 *  - The lease is durable (wsUrl ≥16 min measured); a 10-min reuse window
 *    keeps lease fetches ≤ single-digit/hour (constraint 1).
 *  - WS connects flag the shared session by burst rate (~15/hour observed):
 *    a per-pod rolling-hour connect budget paces flap storms. Per-pod, not
 *    per-room: the session's flag budget is shared by all rooms and all
 *    pods leasing it.
 *  - The lane-pin pre-sign re-enters sign() with a fixed
 *    `userAgent: 'ws-pin'` marker; it is a local cache hit, not a WS dial,
 *    and never consumes connect budget.
 */

import { request as undiciRequest } from 'undici';
import {
  SignatureFailure,
  type SignRequest,
  type SignResult,
  type WebcastSigner
} from './signer.js';

/** The signer service's GET /v1/session response (PR 1, SessionLease). */
interface SessionLeaseResponse {
  wsUrl?: string;
  cookieHeader?: string;
  roomId?: string;
  userAgent?: string;
  proxyHost?: string;
  capturedAt?: number;
  error?: string;
}

/** Keys the connector appends to the WS URL itself (lib-YL2P_UWg.js:1831-1838). */
const CONNECTOR_APPENDED_WS_KEYS = new Set(['compress', 'room_id', 'internal_ext', 'cursor']);

export interface PureNodeSignerOptions {
  /** Base URL of the tiktok-signer service, no trailing slash. */
  baseUrl: string;
  /** Bearer token when the signer service runs with SIGNER_AUTH_TOKEN. */
  authToken?: string;
  /** Lease fetch timeout. A lease is served from a warm tab — fast. */
  sessionTimeoutMs?: number;
  /** Lease reuse window. Must sit inside the measured wsUrl durability (≥16 min). */
  staleAfterMs?: number;
  /** Successful lease fetches per rolling hour (constraint 1). */
  maxLeasesPerHour?: number;
  /** Per-pod WS-connect signs per rolling hour. The session's flag budget is shared. */
  maxWsConnectsPerHour?: number;
  /** Injectable fetch for tests. */
  fetchImpl?: typeof undiciRequest;
}

interface CachedLease {
  wsUrl: string;
  cookieHeader: string;
  userAgent: string;
  proxyHost: string;
  capturedAt: number;
}

export class PureNodeSigner implements WebcastSigner {
  readonly name = 'pure-node';

  private readonly baseUrl: string;
  private readonly headers: Record<string, string>;
  private readonly timeoutMs: number;
  private readonly staleAfterMs: number;
  private readonly maxLeasesPerHour: number;
  private readonly maxWsConnectsPerHour: number;
  private readonly fetchImpl: typeof undiciRequest;

  private lease: CachedLease | undefined;
  private inFlight: Promise<void> | undefined;
  private readonly leaseFetchHours: number[] = [];
  private readonly wsConnectHours: number[] = [];

  constructor(options: PureNodeSignerOptions) {
    this.baseUrl = options.baseUrl.replace(/\/+$/, '');
    this.headers = {
      ...(options.authToken ? { Authorization: `Bearer ${options.authToken}` } : {})
    };
    this.timeoutMs = options.sessionTimeoutMs ?? 30_000;
    this.staleAfterMs = options.staleAfterMs ?? 600_000;
    this.maxLeasesPerHour = options.maxLeasesPerHour ?? 8;
    this.maxWsConnectsPerHour = options.maxWsConnectsPerHour ?? 12;
    this.fetchImpl = options.fetchImpl ?? undiciRequest;
  }

  async sign(request: SignRequest): Promise<SignResult> {
    // The lane-pin pre-sign identifies itself with a fixed marker UA
    // (index.ts): it rides the cached lease and must survive an exhausted
    // connect budget — it is not a WS dial.
    const isPreSign = request.userAgent === 'ws-pin';
    if (!isPreSign) {
      const now = Date.now();
      this.pruneWindow(this.wsConnectHours, now);
      if (this.wsConnectHours.length >= this.maxWsConnectsPerHour) {
        throw new SignatureFailure(
          this.name,
          'WS connect budget exhausted: too many connects this hour (rate limiting self-imposed)'
        );
      }
      this.wsConnectHours.push(now);
    }

    const lease = await this.currentLease();
    return this.synthesize(request, lease);
  }

  /**
   * Fresh cached lease, or a coalesced fetch. Concurrent cache misses join
   * the single in-flight promise — a cold start with N simultaneous connects
   * issues exactly one GET /v1/session.
   */
  private async currentLease(): Promise<CachedLease> {
    const now = Date.now();
    if (this.lease && now - this.lease.capturedAt < this.staleAfterMs) {
      return this.lease;
    }
    if (!this.inFlight) {
      this.inFlight = this.fetchLease().finally(() => {
        this.inFlight = undefined;
      });
    }
    await this.inFlight;
    if (!this.lease) {
      throw new SignatureFailure(this.name, 'no session lease available');
    }
    return this.lease;
  }

  private async fetchLease(): Promise<void> {
    const now = Date.now();
    this.pruneWindow(this.leaseFetchHours, now);
    if (this.leaseFetchHours.length >= this.maxLeasesPerHour) {
      throw new SignatureFailure(
        this.name,
        'session lease rate limited: lease budget exhausted for this hour'
      );
    }

    let bodyJson: SessionLeaseResponse;
    let status: number;
    try {
      const response = await this.fetchImpl(`${this.baseUrl}/v1/session`, {
        headers: this.headers,
        bodyTimeout: this.timeoutMs,
        headersTimeout: this.timeoutMs
      });
      status = response.statusCode;
      const text = await response.body.text();
      bodyJson = text ? (JSON.parse(text) as SessionLeaseResponse) : {};
    } catch (error) {
      throw new SignatureFailure(
        this.name,
        `sign service unreachable at ${this.baseUrl}: ${(error as Error).message}`,
        error
      );
    }

    if (status !== 200 || typeof bodyJson.wsUrl !== 'string' || bodyJson.wsUrl === '') {
      // Only a successful fetch consumes budget (a transient outage plus
      // backoff retries must not mask its own recovery as rate_limit).
      const detail = bodyJson.error ?? `status ${status}`;
      if (status === 401 || status === 404) {
        // Auth misconfig / endpoint disabled: fatal configuration, worded
        // to land the bounded `signature` reason, never silently retried as
        // a network blip.
        throw new SignatureFailure(
          this.name,
          `TikTok signature source misconfigured — session endpoint refused (via sign service): ${detail}`
        );
      }
      // 503 no_warm_session and anything else: retryable, worded to land
      // the bounded `network` reason ("failed to connect" is a classifier
      // token; "connection failed" is not).
      throw new SignatureFailure(
        this.name,
        `sign service failed to connect — no session lease (via sign service): ${detail}`
      );
    }

    this.leaseFetchHours.push(now);
    this.lease = {
      wsUrl: bodyJson.wsUrl,
      cookieHeader: bodyJson.cookieHeader ?? '',
      userAgent: bodyJson.userAgent ?? '',
      proxyHost: bodyJson.proxyHost ?? '',
      capturedAt: typeof bodyJson.capturedAt === 'number' ? bodyJson.capturedAt : now
    };
  }

  /**
   * Build the SignResult: a synthesized ProtoMessageFetchResult that
   * reproduces the recorded WS URL's shape with the target's room_id.
   *
   * The connector appends `compress`, `room_id`, `internal_ext` and
   * `cursor` to the pushServer itself, so those stay in the synthesized
   * fields and every OTHER recorded param is forwarded via routeParams
   * (merged into the WS query at :1838) — the handshake rejects the bare
   * origin without them (measured, PR 2 Task 1).
   */
  private synthesize(request: SignRequest, lease: CachedLease): SignResult {
    const wsUrl = new URL(lease.wsUrl);
    const routeParams: Record<string, string> = {};
    for (const [key, value] of wsUrl.searchParams.entries()) {
      if (!CONNECTOR_APPENDED_WS_KEYS.has(key)) {
        routeParams[key] = value;
      }
    }

    return {
      fetchResult: {
        cursor: '0',
        internalExt: '',
        pushServer: `${wsUrl.origin}${wsUrl.pathname}`,
        messages: [],
        routeParams,
        needsAck: false,
        roomID: request.roomId
      },
      fetchResultCookieHeader: lease.cookieHeader,
      fetchResultProxyHost: lease.proxyHost || undefined
    };
  }

  /** Drop entries older than one rolling hour. */
  private pruneWindow(times: number[], now: number): void {
    const cutoff = now - 3_600_000;
    while (times.length > 0 && times[0]! < cutoff) times.shift();
  }
}
