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
 * synthesizes the connector's initial fetch result in-process (the
 * pure-Node transport).
 *
 * One captured session — a warm classic room's WS URL + cookie jar, leased
 * via GET /v1/session — serves every room's delivery: the connector sends
 * `im_enter_room` for the target room on the leased socket (cross-room
 * entry, measured 2026-09-18). No Euler, no per-room page capture.
 *
 * The session is leased per pod: passing a `consumer` key gets this pod its
 * own warm session, so pods no longer share one session's connect budget.
 *
 * Measured constraints this implementation encodes (measured 2026-09-18,
 * Task 1, 2026-09-18):
 *  - The lease wsUrl's recorded identity params are LOAD-BEARING at the WS
 *    handshake: a bare push origin with only compress/room_id/internal_ext/
 *    cursor is rejected ("Unexpected server response: 200"). The synthesized
 *    fetchResult therefore forwards every recorded param except the four the
 *    connector appends itself.
 *  - The lease is durable (wsUrl ≥16 min measured); a 10-min reuse window,
 *    counted from OUR fetch, keeps lease fetches ≤ single-digit/hour
 *    (constraint 1). Never from the lease's capturedAt: a warm tab keeps
 *    serving its original URL and stamp for hours, so a capturedAt-keyed
 *    window made every lease stale on arrival: every sign, pre-signs
 *    included, re-fetched and burned the 8/h lease budget in ~4 connects.
 *  - WS connects flag a session by burst rate (~15/hour observed): a
 *    rolling-hour connect budget paces flap storms. The budget is PER
 *    LEASED SESSION, not per room and not per pod: TikTok scores the rate
 *    against the session, and the signer assigns this pod its own warm
 *    session (GET /v1/session?consumer=...), so the window follows that
 *    session and resets when the signer re-assigns us. A sign that gets no
 *    lease never dials, so it gives its slot back: a signer outage must not
 *    drain the budget. Refused rooms queue, each for its own freeing slot
 *    (takeConnectSlot).
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

/** The signer service's GET /v1/session response (a SessionLease). */
interface SessionLeaseResponse {
  wsUrl?: string;
  cookieHeader?: string;
  roomId?: string;
  /** The warm room whose tab captured the session: its identity. */
  room?: string;
  userAgent?: string;
  proxyHost?: string;
  error?: string;
}

/** Keys the connector appends to the WS URL itself (lib-YL2P_UWg.js:1831-1838). */
const CONNECTOR_APPENDED_WS_KEYS = new Set(['compress', 'room_id', 'internal_ext', 'cursor']);

/**
 * How long a freed connect slot stays held for the queued room it was
 * promised to: the live poller's 30s tick plus the live-status pre-check.
 */
const CONNECT_QUEUE_HOLD_MS = 90_000;

export interface PureNodeSignerOptions {
  /** Base URL of the tiktok-signer service, no trailing slash. */
  baseUrl: string;
  /** Bearer token when the signer service runs with SIGNER_AUTH_TOKEN. */
  authToken?: string;
  /** Lease fetch timeout. A lease is served from a warm tab — fast. */
  sessionTimeoutMs?: number;
  /** Lease reuse window from the local fetch. Must sit inside the measured wsUrl durability (≥16 min). */
  staleAfterMs?: number;
  /** Successful lease fetches per rolling hour (constraint 1). */
  maxLeasesPerHour?: number;
  /**
   * WS-connect signs per rolling hour against ONE leased session, which is
   * the granularity TikTok scores its ~15/hour flag budget at
   * (TIKTOK_MAX_WS_CONNECTS_PER_HOUR). Pods leasing distinct sessions each
   * get this budget; pods sharing a session still divide it, so size it
   * for the number of pods that can land on one warm room.
   */
  maxWsConnectsPerHour?: number;
  /**
   * Stable key identifying this pod to the signer's lease endpoint, so it
   * is assigned its own warm session instead of sharing the fleet's first
   * servable one (TIKTOK_LEASE_CONSUMER, the pod name). Empty leaves the
   * signer's pre-assignment behaviour, which every pod shares.
   */
  consumer?: string;
  /** Injectable fetch for tests. */
  fetchImpl?: typeof undiciRequest;
}

interface CachedLease {
  wsUrl: string;
  cookieHeader: string;
  /** Which warm session this lease is on; see sessionRoom. */
  room: string;
  userAgent: string;
  proxyHost: string;
  fetchedAt: number;
}

export class PureNodeSigner implements WebcastSigner {
  readonly name = 'pure-node';

  private readonly baseUrl: string;
  private readonly headers: Record<string, string>;
  private readonly timeoutMs: number;
  private readonly staleAfterMs: number;
  private readonly maxLeasesPerHour: number;
  private readonly maxWsConnectsPerHour: number;
  private readonly consumer: string;
  private readonly fetchImpl: typeof undiciRequest;

  private lease: CachedLease | undefined;
  private inFlight: Promise<void> | undefined;
  private readonly leaseFetchHours: number[] = [];
  private readonly wsConnectHours: number[] = [];
  /** Rooms refused a connect slot → when the slot promised to them frees. */
  private readonly connectQueue = new Map<string, number>();
  /**
   * The warm session the counted connects were spent on. The budget meters
   * ONE session's TikTok flag window, so when the signer re-assigns this
   * pod (its warm room went offline) the old session's connects stop
   * applying — see noteSession.
   */
  private sessionRoom = '';

  constructor(options: PureNodeSignerOptions) {
    this.baseUrl = options.baseUrl.replace(/\/+$/, '');
    this.headers = {
      ...(options.authToken ? { Authorization: `Bearer ${options.authToken}` } : {})
    };
    this.timeoutMs = options.sessionTimeoutMs ?? 30_000;
    this.staleAfterMs = options.staleAfterMs ?? 600_000;
    this.maxLeasesPerHour = options.maxLeasesPerHour ?? 8;
    this.maxWsConnectsPerHour = options.maxWsConnectsPerHour ?? 12;
    this.consumer = options.consumer ?? '';
    this.fetchImpl = options.fetchImpl ?? undiciRequest;
  }

  async sign(request: SignRequest): Promise<SignResult> {
    // The lane-pin pre-sign identifies itself with a fixed marker UA
    // (index.ts): it rides the cached lease and must survive an exhausted
    // connect budget — it is not a WS dial.
    const isPreSign = request.userAgent === 'ws-pin';
    const now = Date.now();
    if (!isPreSign) this.takeConnectSlot(request.roomId, now);

    try {
      const lease = await this.currentLease();
      return this.synthesize(request, lease);
    } catch (error) {
      // The slot is reserved before the await so concurrent signs cannot
      // all pass the check; released here because nothing was dialled.
      if (!isPreSign) {
        const slot = this.wsConnectHours.lastIndexOf(now);
        if (slot >= 0) this.wsConnectHours.splice(slot, 1);
      }
      throw error;
    }
  }

  /**
   * Take a WS-connect slot, or refuse and queue the room for its own slot.
   *
   * Every refused room is told when a slot frees that no room ahead of it
   * is promised, and a slot that frees for a queued room is held for it
   * until CONNECT_QUEUE_HOLD_MS after that time. Told to retry when the
   * oldest slot slid, every refused room came back together, one connected
   * and the rest were refused again, slot after slot (prod 2026-11-12: 37
   * of a day's 59 refusals were rooms already parked).
   */
  private takeConnectSlot(roomId: string, now: number): void {
    this.pruneWindow(this.wsConnectHours, now);
    this.connectQueue.delete(roomId);
    let held = 0;
    for (const [room, slotAt] of this.connectQueue) {
      if (slotAt < now - CONNECT_QUEUE_HOLD_MS) this.connectQueue.delete(room); // never came back
      else if (slotAt <= now) held++;
    }
    const free = this.maxWsConnectsPerHour - this.wsConnectHours.length;
    if (free > held) {
      this.wsConnectHours.push(now);
      return;
    }
    // Every connect, made or promised, frees its slot an hour later; the
    // rooms already queued take the earliest of those in turn.
    const slotsFreeAt = [...this.wsConnectHours, ...this.connectQueue.values()]
      .map((t) => t + 3_600_000)
      .sort((a, b) => a - b);
    const slotAt = slotsFreeAt[this.connectQueue.size - free]!;
    this.connectQueue.set(roomId, slotAt);
    throw new SignatureFailure(
      this.name,
      'WS connect budget exhausted: too many connects this hour (rate limiting self-imposed)',
      undefined,
      slotAt - now
    );
  }

  /**
   * Fresh cached lease, or a coalesced fetch. Concurrent cache misses join
   * the single in-flight promise — a cold start with N simultaneous connects
   * issues exactly one GET /v1/session.
   */
  private async currentLease(): Promise<CachedLease> {
    const now = Date.now();
    if (this.lease && now - this.lease.fetchedAt < this.staleAfterMs) {
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
        'session lease rate limited: lease budget exhausted for this hour',
        undefined,
        Math.max(0, this.leaseFetchHours[0]! + 3_600_000 - now)
      );
    }

    let bodyJson: SessionLeaseResponse;
    let status: number;
    try {
      const leaseUrl = this.consumer
        ? `${this.baseUrl}/v1/session?consumer=${encodeURIComponent(this.consumer)}`
        : `${this.baseUrl}/v1/session`;
      const response = await this.fetchImpl(leaseUrl, {
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
      room: bodyJson.room ?? '',
      userAgent: bodyJson.userAgent ?? '',
      proxyHost: bodyJson.proxyHost ?? '',
      fetchedAt: now
    };
    this.noteSession(this.lease.room);
  }

  /**
   * Follow the leased session, dropping the connect window when it changes.
   *
   * The window counts against one session's ~15/hour TikTok flag budget. A
   * re-assignment hands us a session with its own untouched window, and
   * carrying the old counts over would refuse rooms that session has room
   * for. Only a CHANGE resets: resetting per lease fetch (one every ten
   * minutes) would leave the budget unenforceable.
   *
   * A signer too old to report `room` sends '', which never changes, so the
   * budget keeps its previous single-session meaning.
   *
   * The sign that triggered this lease fetch has already taken its slot and
   * will dial on the NEW session, so one slot is carried over rather than
   * dropped: clearing the window wholesale would let it connect for free.
   * Learning the first session's name is not a re-assignment and keeps the
   * window as it stands.
   */
  private noteSession(room: string): void {
    if (room === this.sessionRoom) return;
    const reassigned = this.sessionRoom !== '';
    this.sessionRoom = room;
    if (!reassigned) return;
    const carried = this.wsConnectHours.slice(-1);
    this.wsConnectHours.length = 0;
    this.wsConnectHours.push(...carried);
    this.connectQueue.clear();
  }

  /**
   * Build the SignResult: a synthesized ProtoMessageFetchResult that
   * reproduces the recorded WS URL's shape with the target's room_id.
   *
   * The connector appends `compress`, `room_id`, `internal_ext` and
   * `cursor` to the pushServer itself, so those stay in the synthesized
   * fields and every OTHER recorded param is forwarded via routeParams
   * (merged into the WS query at :1838) — the handshake rejects the bare
   * origin without them (measured 2026-09-18).
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
