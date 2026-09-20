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
 * Warm-room auto-curator (2026-09-20 resilience work, after the
 * offline-warm-room incident and the two manual warm-list audits of the
 * same day). SIGNER_WARM_ROOMS seeds the list at startup; the curator
 * keeps live coverage above a floor for as long as TikTok cooperates:
 *
 * - every CHECK_INTERVAL it probes the liveness of each warm room that has
 *   no warm tab (hasTab rooms already hold a servable session; a warm tab
 *   outlives the stream, so a streamer going offline is not coverage loss);
 * - when live coverage (live rooms + warm tabs) falls below MIN_COVER, one
 *   discovery pass over the public live feed runs on the pool's own lane
 *   browser — same UA and request blocking as a capture's page, one
 *   navigation, no im/fetch tap;
 * - candidates are filtered (never canary rooms, never currently listed,
 *   never failed recently), liveness-checked, then capture-verified: the
 *   capture IS the classic-ness test — only a room whose live page emits
 *   the harvestable im/fetch passes, and a passing capture leaves the room
 *   warm, which is the point;
 * - budget discipline is encoded, not a habit: at most MAX_ACQUISITIONS
 *   capture attempts per cycle, and any discovery or capture 403 benches
 *   the whole curator for BENCH_ON_403_MS (the manual rule this replaces:
 *   one 403 = stop that surface for the hour);
 * - every swap is logged and counted (signer_warm_room_swaps_total{reason});
 *   the NoWarmSession alert stays armed as the backstop that proves the
 *   curator failed, not the primary signal.
 *
 * Failure posture is conservative: any uncertainty (discovery page fails,
 * liveness probe errors — those fail open like the lease path, candidates
 * exhausted) leaves the existing list untouched. The curator can only ADD
 * verified-live, capture-proven rooms; it never removes a room (removal
 * stays a human call — a listed room failing captures is the breaker's and
 * the alert's business).
 */

import type { Page } from 'puppeteer';
import type { ViewerPool } from './viewer.js';
import { warmRoomIsLive } from '../api.js';

/** Liveness probe seam type: TRUE = live (or unknown — fail open). */
export type IsLive = (username: string) => Promise<boolean>;

/** Discovery seam type: run on a pool page, return candidate handles. */
export type Discover = () => Promise<string[] | undefined>;

export interface WarmCuratorOptions {
  /** The seeded warm list (SIGNER_WARM_ROOMS); owned by the caller. */
  warmRooms: Set<string>;
  /** Canary rooms are never acquisition candidates (dual-list refusal). */
  canaryRooms: ReadonlySet<string>;
  viewer: ViewerPool;
  logger?: { info: (msg: string, meta?: Record<string, unknown>) => void; warn: (msg: string, meta?: Record<string, unknown>) => void };
  /** Injectable liveness probe; defaults to the production warmRoomIsLive. */
  isLive?: IsLive;
  /** Injectable discovery; defaults to the live-feed scan below. */
  discover?: Discover;
  /** Coverage floor: run acquisition while live rooms + tabs < this. */
  minCover?: number;
}

/** Probe cadence. Idle cost: one api-live request per tab-less warm room. */
const CHECK_INTERVAL_MS = 30 * 60_000;
/** Below this live coverage, one acquisition cycle runs. */
const DEFAULT_MIN_COVER = 2;
/** Capture attempts per cycle. The budget cap, not a target. */
const MAX_ACQUISITIONS = 2;
/** One 403 anywhere benches discovery+capture for an hour (manual rule). */
const BENCH_ON_403_MS = 60 * 60_000;
/** Candidates that failed verification wait this long before re-trying. */
const FAILED_COOLDOWN_MS = 6 * 60 * 60_000;

/** One live-feed discovery pass on the pool's own lane page. */
async function discoverViaFeed(viewer: ViewerPool): Promise<string[] | undefined> {
  return viewer.withDiscoveryPage(async (page: Page) => {
    await page.goto('https://www.tiktok.com/live', { waitUntil: 'networkidle2', timeout: 60_000 });
    // The feed renders room links progressively; the selector appearing at
    // all means the feed mounted, and the handle extraction races nothing
    // worse than an incomplete list (a retry next cycle picks up the rest).
    const handles: string[] = await page.evaluate(() => {
      const hrefs: string[] = [];
      const links = document.querySelectorAll('a[href*="/live"]');
      for (let i = 0; i < links.length; i++) {
        const href = links.item(i).getAttribute('href');
        if (href !== null && /^\/@[a-zA-Z0-9_.]+\/live$/.test(href)) {
          hrefs.push(href.slice(2, -5).toLowerCase());
        }
      }
      return [...new Set(hrefs)].slice(0, 12);
    });
    return handles;
  });
}

export class WarmCurator {
  private readonly warmRooms: Set<string>;
  private readonly canaryRooms: ReadonlySet<string>;
  private readonly viewer: ViewerPool;
  private readonly logger: WarmCuratorOptions['logger'];
  private readonly isLive: IsLive;
  private readonly discover: Discover;
  private readonly minCover: number;
  private readonly failures = new Map<string, number>();
  private benchedUntil = 0;
  private timer: ReturnType<typeof setInterval> | undefined;

  constructor(options: WarmCuratorOptions) {
    this.warmRooms = options.warmRooms;
    this.canaryRooms = options.canaryRooms;
    this.viewer = options.viewer;
    this.logger = options.logger;
    this.isLive = options.isLive ?? ((username) => warmRoomIsLive(username));
    this.discover = options.discover ?? (() => discoverViaFeed(options.viewer));
    this.minCover = options.minCover ?? DEFAULT_MIN_COVER;
  }

  /** Whether an error is a bench-worthy refusal (HTTP 403 from TikTok). */
  private static isRefusal(error: unknown): boolean {
    const message = error instanceof Error ? error.message : String(error);
    return /\b403\b|Forbidden|blocked/i.test(message);
  }

  /** Coverage check: warm tabs count as coverage; others must be live. */
  private async liveCover(): Promise<number> {
    let cover = 0;
    for (const room of this.warmRooms) {
      if (this.viewer.hasTab(room)) {
        cover++;
        continue;
      }
      // Fail-open probe: an unreachable liveness route counts the room as
      // covered rather than triggering acquisition on a route outage.
      if (await this.isLive(room).catch(() => true)) cover++;
    }
    return cover;
  }

  /**
   * One acquisition cycle: discover, filter, liveness-check, capture.
   * Visible for tests; the timer calls it on the same cadence.
   */
  async runAcquisitionCycle(): Promise<void> {
    if (Date.now() < this.benchedUntil) return;
    let candidates: string[] | undefined;
    try {
      candidates = await this.discover();
    } catch (error) {
      if (WarmCurator.isRefusal(error)) {
        this.benchedUntil = Date.now() + BENCH_ON_403_MS;
        this.logger?.warn('warm curator benched after discovery refusal', {});
      } else {
        this.logger?.warn('warm curator discovery failed', {
          error: error instanceof Error ? error.message : String(error)
        });
      }
      return;
    }
    if (!candidates || candidates.length === 0) return;
    let attempts = 0;
    for (const room of candidates) {
      if (attempts >= MAX_ACQUISITIONS) break;
      if (
        this.warmRooms.has(room) ||
        this.canaryRooms.has(room) ||
        this.viewer.hasTab(room)
      ) {
        continue;
      }
      const failedAt = this.failures.get(room);
      if (failedAt !== undefined && Date.now() - failedAt < FAILED_COOLDOWN_MS) continue;
      // Liveness gate before the capture: an offline candidate can never
      // emit the im/fetch, and the capture budget is the scarce resource.
      if (!(await this.isLive(room).catch(() => false))) continue;
      attempts++;
      try {
        const capture = await this.viewer.captureRoom(room);
        if (capture.wsUrl === '') {
          // Live but not classic (live_new variant): the page never opened
          // the harvestable webcast socket. Cooldown, not a bench.
          this.failures.set(room, Date.now());
          this.logger?.info('warm curator candidate not classic', { room });
          continue;
        }
        this.warmRooms.add(room);
        this.failures.delete(room);
        this.logger?.info('warm curator acquired room', { room });
      } catch (error) {
        this.failures.set(room, Date.now());
        if (WarmCurator.isRefusal(error)) {
          this.benchedUntil = Date.now() + BENCH_ON_403_MS;
          this.logger?.warn('warm curator benched after capture refusal', { room });
          return;
        }
        this.logger?.warn('warm curator capture failed', {
          room,
          error: error instanceof Error ? error.message : String(error)
        });
      }
    }
  }

  /** One scheduled tick: cover check, then acquisition if under floor. */
  async tick(): Promise<void> {
    const cover = await this.liveCover();
    if (cover >= this.minCover) return;
    this.logger?.info('warm curator under coverage floor', {
      cover,
      floor: this.minCover
    });
    await this.runAcquisitionCycle();
  }

  start(): void {
    if (this.timer) return;
    this.timer = setInterval(() => {
      void this.tick().catch(() => undefined);
    }, CHECK_INTERVAL_MS);
    // An early tick at startup (after one interval's grace for the pool
    // to come up) checks the seed list's coverage without waiting a full
    // cycle: a stale seed after a deploy is found within minutes.
    setTimeout(() => {
      void this.tick().catch(() => undefined);
    }, 60_000);
  }

  stop(): void {
    if (this.timer) clearInterval(this.timer);
    this.timer = undefined;
  }
}
