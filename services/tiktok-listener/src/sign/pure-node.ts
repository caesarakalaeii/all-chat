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
 *  - WS connects are paced by a per-pod token bucket (takeConnectSlot): a
 *    sustained rate plus a burst. TikTok can throttle a session's WS
 *    surface on connect rate (one lab observation, ~15 attempts in an hour
 *    on a session a fetch burst had already flagged), but prod has never
 *    been refused: one session carried 28 connects in an hour and 12 in 40
 *    seconds (2026-11-19) with no handshake refusal in 30 days of logs. In
 *    prod every pod leases the SAME session: the signer keeps one warm tab
 *    and serves whichever room has one, so the fleet's rate is replicas x
 *    this pod's rate. A sign that gets no lease never dials, so it gives
 *    its token back: a signer outage must not drain the budget. Refused
 *    rooms queue in order. If TikTok does refuse handshakes, the bucket
 *    throttles itself (noteHandshakeRefused).
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
 * How long a connect token stays held for the queued room it was promised
 * to: the live poller's 30s tick plus the live-status pre-check.
 */
const CONNECT_QUEUE_HOLD_MS = 90_000;

/** Connects per hour, with no burst, while TikTok is refusing handshakes. */
const THROTTLED_CONNECTS_PER_HOUR = 6;

/** How long one refused handshake keeps the bucket throttled. */
const HANDSHAKE_REFUSAL_THROTTLE_MS = 3_600_000;

/** A room refused a connect token: when its token is due, and since when it has waited. */
interface QueuedRoom {
  readyAt: number;
  since: number;
}

/** What the connect budget is doing right now, for the metrics endpoint. */
export interface ConnectBudgetStatus {
  /** Rooms refused a connect that have not connected since. */
  waitingRooms: number;
  /** How long the longest-waiting of those rooms has waited. */
  oldestWaitMs: number;
  /** Whether a refused handshake has the bucket on its reduced rate. */
  throttled: boolean;
}

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
   * Sustained WS-connect signs per hour for this pod: the refill rate of
   * the connect token bucket (TIKTOK_MAX_WS_CONNECTS_PER_HOUR). Every pod
   * dials on the one session the signer keeps warm, so the session sees
   * replicas x this rate.
   */
  maxWsConnectsPerHour?: number;
  /**
   * Connect signs this pod may make back to back after a quiet spell: the
   * capacity of the token bucket (TIKTOK_WS_CONNECT_BURST). Sized for the
   * moments TikTok cuts every socket at once and each room re-dials.
   */
  wsConnectBurst?: number;
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
  private readonly wsConnectBurst: number;
  private readonly consumer: string;
  private readonly fetchImpl: typeof undiciRequest;

  private lease: CachedLease | undefined;
  private inFlight: Promise<void> | undefined;
  private readonly leaseFetchHours: number[] = [];
  /** Connect tokens in the bucket, as of connectTokensAt. Starts full. */
  private connectTokens: number;
  private connectTokensAt = Date.now();
  /** Rooms refused a connect token, in the order their tokens fall due. */
  private readonly connectQueue = new Map<string, QueuedRoom>();
  /** Until when a refused handshake keeps the bucket throttled. */
  private throttledUntil = 0;

  constructor(options: PureNodeSignerOptions) {
    this.baseUrl = options.baseUrl.replace(/\/+$/, '');
    this.headers = {
      ...(options.authToken ? { Authorization: `Bearer ${options.authToken}` } : {})
    };
    this.timeoutMs = options.sessionTimeoutMs ?? 30_000;
    this.staleAfterMs = options.staleAfterMs ?? 600_000;
    this.maxLeasesPerHour = options.maxLeasesPerHour ?? 8;
    this.maxWsConnectsPerHour = options.maxWsConnectsPerHour ?? 18;
    this.wsConnectBurst = options.wsConnectBurst ?? 12;
    this.connectTokens = this.wsConnectBurst;
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
      // The token is taken before the await so concurrent signs cannot
      // all pass the check; returned here because nothing was dialled.
      if (!isPreSign) this.connectTokens += 1;
      throw error;
    }
  }

  /**
   * Take a connect token, or refuse and queue the room for its own token.
   *
   * A token bucket, not a rolling-hour count: the bucket refills at the
   * sustained rate and holds at most the burst. A rolling hour made a
   * refused room wait for a connect from up to an hour earlier to age out
   * (prod 2026-11-19: TikTok cut all 15 sockets at once, the re-dials used
   * up the hour, and rooms going live were still waiting 35 minutes
   * later). Here the wait is the room's place in the queue divided by the
   * refill rate.
   *
   * Each refused room is told when its own token falls due, and that token
   * is held for it until CONNECT_QUEUE_HOLD_MS after that time. Told to
   * retry at one shared moment, every refused room came back together, one
   * connected and the rest were refused again (prod 2026-11-12: 37 of a
   * day's 59 refusals were rooms already parked).
   */
  private takeConnectSlot(roomId: string, now: number): void {
    const mine = this.connectQueue.get(roomId);
    for (const [room, queued] of this.connectQueue) {
      if (queued.readyAt < now - CONNECT_QUEUE_HOLD_MS) this.connectQueue.delete(room); // never came back
    }
    // Refill while this room still counts towards the queue: the bucket
    // may hold one token per queued room, however small the burst.
    this.refillConnectTokens(now);
    this.connectQueue.delete(roomId);

    let held = 0;
    for (const queued of this.connectQueue.values()) {
      if (queued.readyAt <= now) held++;
    }
    if (this.connectTokens - held >= 1 - 1e-9) {
      this.connectTokens = Math.max(0, this.connectTokens - 1);
      return;
    }

    // Back before its turn, the room keeps its place. Otherwise every room
    // already queued takes a token before this one does.
    let readyAt = mine?.readyAt ?? 0;
    if (readyAt <= now) {
      const missing = this.connectQueue.size + 1 - this.connectTokens;
      readyAt = now + Math.ceil(missing / this.connectRatePerMs(now) - 1e-6);
    }
    this.connectQueue.set(roomId, { readyAt, since: mine?.since ?? now });
    throw this.connectRefusal(readyAt - now);
  }

  private connectRefusal(retryAfterMs: number): SignatureFailure {
    return new SignatureFailure(
      this.name,
      'WS connect budget exhausted: too many connects in a row (rate limiting self-imposed)',
      undefined,
      retryAfterMs
    );
  }

  private connectRatePerMs(now: number): number {
    const perHour =
      now < this.throttledUntil
        ? Math.min(THROTTLED_CONNECTS_PER_HOUR, this.maxWsConnectsPerHour)
        : this.maxWsConnectsPerHour;
    return perHour / 3_600_000;
  }

  /** Add the tokens earned since the last refill, up to the bucket's capacity. */
  private refillConnectTokens(now: number): void {
    const burst = now < this.throttledUntil ? 1 : this.wsConnectBurst;
    const capacity = Math.max(burst, this.connectQueue.size);
    const earned = Math.max(0, now - this.connectTokensAt) * this.connectRatePerMs(now);
    this.connectTokens = Math.min(capacity, this.connectTokens + earned);
    this.connectTokensAt = now;
  }

  /**
   * TikTok refused a WS handshake and the flap retries did not clear it.
   *
   * That is the one real sign that the session's connect rate is being
   * scored against us, so the bucket drops to THROTTLED_CONNECTS_PER_HOUR
   * with no burst for an hour. The configured rate is above anything prod
   * has been refused at, not above anything TikTok might ever refuse, and
   * a pod that kept dialling at full rate into a throttled session would
   * only extend the throttle.
   */
  noteHandshakeRefused(now: number = Date.now()): void {
    this.refillConnectTokens(now);
    this.throttledUntil = now + HANDSHAKE_REFUSAL_THROTTLE_MS;
    this.connectTokens = Math.min(this.connectTokens, 1);
  }

  /**
   * Rooms waiting on the connect budget, for the metrics endpoint.
   *
   * A room that was refused and then went offline, or was claimed by
   * another pod, still counts until its held token lapses.
   */
  connectBudgetStatus(now: number = Date.now()): ConnectBudgetStatus {
    let waitingRooms = 0;
    let oldestWaitMs = 0;
    for (const queued of this.connectQueue.values()) {
      if (queued.readyAt < now - CONNECT_QUEUE_HOLD_MS) continue;
      waitingRooms++;
      oldestWaitMs = Math.max(oldestWaitMs, now - queued.since);
    }
    return { waitingRooms, oldestWaitMs, throttled: now < this.throttledUntil };
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
