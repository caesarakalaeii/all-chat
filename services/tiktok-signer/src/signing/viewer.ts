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

import puppeteerExtra from 'puppeteer-extra';
import type { PuppeteerExtra } from 'puppeteer-extra/dist/index.js';
import StealthPlugin from 'puppeteer-extra-plugin-stealth';
import type { Browser, Page } from 'puppeteer';
import { randomUUID } from 'crypto';

// Stealth evasions: TikTok's secSDK fingerprints the browser environment, and
// since 2026-09-09 TikTok refuses webcast data endpoints to sessions that look
// automated (zerodytrash/TikTok-Live-Connector#329). Same plugin stack the
// signature path uses.
const puppeteer = puppeteerExtra as unknown as PuppeteerExtra;
puppeteer.use(StealthPlugin());

export interface ViewerPoolOptions {
  /**
   * Residential proxy list (host:port each). One browser profile per proxy: a
   * profile's session identity is bound to its IP, and mixing IPs inside one
   * profile reads as account-hopping to TikTok's bot detection. Rooms are
   * pinned to the browser they first captured on. With no proxies, a single
   * direct-connection browser serves everything.
   */
  proxyHosts?: string[];
  /** Proxy credentials; shared across the list (webshare shape). */
  proxyUser?: string;
  proxyPass?: string;
  /** Directory base for browser profiles; must be writable (emptyDir in k8s). */
  userDataDir?: string;
  /** Where Chromium is; resolved by puppeteer when unset. */
  executablePath?: string;
  /**
   * X server display to render on. Page-viewer mode needs a real rendering
   * stack: measured 2026-09-15, im/fetch answers 403 to every headless attempt
   * and 200-with-full-payload to the same Chromium on a real display (Xvfb +
   * llvmpipe passes; --disable-gpu/SwiftShader does not). If unset, Chromium
   * runs headless and viewer capture is expected to fail against TikTok's bot
   * detection.
   */
  display?: string;
  /** How long an idle room tab is kept before eviction (default 5 min). */
  tabIdleMs?: number;
  /** Cooldown for a proxy after a failed capture (default 10 min). */
  proxyCooldownMs?: number;
}

export interface RoomCapture {
  /** The room TikTok actually served (from the fetch URL). */
  roomId: string;
  /** /im/fetch/ protobuf body, base64. Caller decodes with connector schemas. */
  protoBase64: string;
  /** Set-Cookie content of the fetch + the page session's cookies. */
  cookieHeader: string;
  /** The UA the page ran with; the listener pins its presets to it. */
  userAgent: string;
  /** Which proxy served the capture ("" = direct). For logs/metrics. */
  proxyHost: string;
  elapsedMs: number;
}

interface TabEntry {
  page: Page;
  lastUsed: number;
}

/** One browser instance = one egress identity. Keyed by proxy host ("" = direct). */
interface ProxyLane {
  /** The lane's stable identity: the proxy host:port, or "" for direct. */
  host: string;
  browser: Browser | null;
  launching: Promise<Browser> | null;
  /** Usernames currently being captured on this lane. */
  active: Set<string>;
  /** Until when this lane is benched after a failure. */
  benchedUntil: number;
}

/**
 * A pool of "real viewer" browser tabs. Each capture opens (or reuses) a tab
 * on the room's live page and records the SDK-signed /webcast/im/fetch/
 * response TikTok serves the player — the same exchange a logged-out human
 * browser gets. This is the post-2026-09-09 shape of ADR-0052: TikTok now
 * gates webcast data on a browser-grade session fingerprint, which no
 * signature (X-Bogus/X-Gnarly) can substitute for, so the signer *is* the
 * viewer.
 *
 * With a proxy list configured, each proxy gets its own browser (own profile
 * directory, own session identity) and rooms are pinned to the lane they
 * first captured on — the profile keeps its TikTok cookies, which is part of
 * the session-grade identity that passes the gate. A failed capture benches
 * the lane for a cooldown and the next attempt rides another one. Lanes are
 * keyed by proxy host, so `refreshProxies` (fed by the webshare API) can
 * swap the list underneath without disturbing surviving lanes.
 *
 * Tabs stay open after a capture: the page keeps receiving live push (which
 * keeps the session warm on TikTok's side) and the next capture for the same
 * room reuses its warmed session identity instead of bootstrapping a new one.
 */
export class ViewerPool {
  /** Lanes keyed by proxy host ("" = direct). */
  private readonly lanes = new Map<string, ProxyLane>();
  /** Round-robin cursor over the lane keys in insertion order. */
  private laneOrder: string[] = [];
  private readonly tabs = new Map<string, TabEntry>();
  /** username -> lane host, so a room keeps its browser identity. */
  private readonly roomLane = new Map<string, string>();
  private roundRobin = 0;
  private readonly options: Required<Pick<ViewerPoolOptions, 'tabIdleMs' | 'proxyCooldownMs'>> &
    ViewerPoolOptions;

  constructor(options: ViewerPoolOptions = {}) {
    this.options = {
      tabIdleMs: 5 * 60_000,
      proxyCooldownMs: 10 * 60_000,
      ...options
    };
    const hosts = options.proxyHosts ?? [];
    if (hosts.length === 0) {
      this.addLane('');
    } else {
      hosts.forEach((host) => this.addLane(host));
    }
  }

  private addLane(host: string): ProxyLane {
    const lane: ProxyLane = { host, browser: null, launching: null, active: new Set(), benchedUntil: 0 };
    this.lanes.set(host, lane);
    this.laneOrder = [...this.lanes.keys()];
    return lane;
  }

  /**
   * Refresh the proxy list (fed by the webshare API). Lanes for proxies that
   * disappeared are closed — their pinned rooms recapture on surviving lanes —
   * and new proxies get fresh lanes. Surviving lanes keep their browsers,
   * profiles and pinned rooms untouched.
   */
  async refreshProxies(hosts: string[]): Promise<void> {
    const wanted = new Set(hosts);
    for (const [host, lane] of [...this.lanes]) {
      if (host !== '' && !wanted.has(host)) {
        this.lanes.delete(host);
        this.laneOrder = [...this.lanes.keys()];
        try {
          await lane.browser?.close();
        } catch {
          // Closing an already-dead browser is not an error.
        }
        lane.active.clear();
        for (const [username, laneHost] of [...this.roomLane]) {
          if (laneHost === host) {
            this.roomLane.delete(username);
            const tab = this.tabs.get(username);
            if (tab) {
              this.tabs.delete(username);
              void tab.page.close().catch(() => undefined);
            }
          }
        }
      }
    }
    for (const host of hosts) {
      if (!this.lanes.has(host)) this.addLane(host);
    }
  }

  private pickLane(username: string): ProxyLane {
    // A room already captured keeps its lane: the profile's cookies are part
    // of the identity TikTok judged the first time.
    const pinnedHost = this.roomLane.get(username);
    if (pinnedHost !== undefined) {
      const lane = this.lanes.get(pinnedHost);
      if (lane && Date.now() >= lane.benchedUntil) return lane;
      // The pinned lane is benched or gone; fall through to picking another.
    }
    const now = Date.now();
    const all = this.laneOrder
      .map((h) => this.lanes.get(h))
      .filter((l): l is ProxyLane => l !== undefined);
    const available = all.filter((l) => now >= l.benchedUntil);
    const pool = available.length > 0 ? available : all;
    const lane = pool[this.roundRobin % pool.length];
    this.roundRobin++;
    return lane;
  }

  private benchLane(lane: ProxyLane): void {
    // Only bench proxy lanes: the direct lane has nothing to rotate away from,
    // and benching it would just serialize captures for no benefit.
    if (lane.host !== '') {
      lane.benchedUntil = Date.now() + this.options.proxyCooldownMs;
    }
  }

  private async ensureBrowser(lane: ProxyLane): Promise<Browser> {
    if (lane.browser) return lane.browser;
    if (lane.launching) return lane.launching;

    const args = [
      '--no-sandbox',
      '--disable-setuid-sandbox',
      '--disable-dev-shm-usage',
      '--disable-blink-features=AutomationControlled',
      '--window-size=1920,1080',
      // Real rendering, not SwiftShader: the display/timing stack is part of
      // what TikTok's bot detection measures (see class doc). --disable-gpu
      // forces software rendering and was measured to fail the same way as
      // headless on 2026-09-15.
      '--use-gl=angle',
      '--enable-gpu-rasterization'
    ];
    if (this.options.display) {
      args.push(`--display=${this.options.display}`);
    }
    if (lane.host) {
      args.push(`--proxy-server=http://${lane.host}`);
    }

    lane.launching = puppeteer
      .launch({
        // Non-headless, always: measured 2026-09-15, headless Chromium —
        // stealth plugin included — gets im/fetch 403 on every attempt while
        // the same binary on a real display gets 200 with a full protobuf
        // payload in ~5s. Headless here is not a trade-off, it is broken.
        headless: false,
        executablePath: this.options.executablePath,
        args,
        // Profile per lane: the session identity (cookies, device reputation)
        // is bound to the egress IP and must never mix.
        userDataDir: `${this.options.userDataDir ?? '/tmp/tiktok-signer-profile-viewer'}-${lane.host || 'direct'}`,
        ignoreDefaultArgs: ['--enable-automation']
      })
      .then((browser: Browser) => {
        lane.browser = browser;
        lane.launching = null;
        return browser;
      });
    return lane.launching;
  }

  private evictIdleTabs(): void {
    const now = Date.now();
    for (const [username, entry] of this.tabs) {
      if (now - entry.lastUsed > this.options.tabIdleMs) {
        this.tabs.delete(username);
        this.roomLane.delete(username);
        void entry.page.close().catch(() => undefined);
      }
    }
  }

  /**
   * Capture the initial fetch exchange for a live room. Resolves when the
   * page's player receives a non-trivial /im/fetch/ response; rejects after
   * `timeoutMs` (default 45s — page load plus player init measured at ~5s,
   * the timeout covers cold starts and slow rooms).
   */
  async captureRoom(
    username: string,
    { timeoutMs = 45_000 }: { timeoutMs?: number } = {}
  ): Promise<RoomCapture> {
    this.evictIdleTabs();
    const lane = this.pickLane(username);
    const browser = await this.ensureBrowser(lane);
    const startedAt = Date.now();
    lane.active.add(username);

    // Reuse a warm tab when we have one for this room's streamer.
    const existing = this.tabs.get(username);
    const page: Page = existing
      ? existing.page
      : await browser.newPage().then(async (p) => {
          await p.setUserAgent(VIEWER_UA);
          // The tab must not stream the video: with a residential proxy wired
          // (the answer to TikTok's datacenter-IP gating), media would be 99%
          // of the proxy's bill. The player initializes and fetches chat data
          // the same without the stream itself.
          await p.setRequestInterception(true);
          p.on('request', (request) => {
            if (['media', 'font'].includes(request.resourceType())) {
              void request.abort().catch(() => undefined);
              return;
            }
            void request.continue().catch(() => undefined);
          });
          if (this.options.proxyUser && this.options.proxyPass) {
            await p.authenticate({
              username: this.options.proxyUser,
              password: this.options.proxyPass
            });
          }
          return p;
        });

    const firstCapture = !existing;
    try {
      return await new Promise<RoomCapture>((resolve, reject) => {
        let settled = false;
        const jobId = randomUUID(); // correlation for logs; capture is per-tab
        const fail = setTimeout(async () => {
          if (settled) return;
          settled = true;
          // Drop the tab: a page that never produced a fetch is a dead session.
          this.tabs.delete(username);
          this.roomLane.delete(username);
          this.benchLane(lane);
          await page.close().catch(() => undefined);
          reject(new Error(`im/fetch 200 not captured within ${timeoutMs}ms (proxy ${lane.host || 'direct'}, ${jobId})`));
        }, timeoutMs);

        const onResponse = async (response: {
          url(): string;
          status(): number;
          buffer(): Promise<Buffer>;
        }) => {
          if (settled) return;
          const url = response.url();
          if (!url.includes('/webcast/im/fetch/')) return;
          if (response.status() !== 200) return;
          let buf: Buffer;
          try {
            buf = await response.buffer();
          } catch {
            return;
          }
          if (buf.length < 1000) return; // keepalive/empty shapes are not a capture
          settled = true;
          clearTimeout(fail);
          page.off('response', onResponse);

          try {
            const cookies = (await page.cookies('https://www.tiktok.com'))
              .map((c) => `${c.name}=${c.value}`)
              .join('; ');
            const roomId = new URL(url).searchParams.get('room_id') ?? username;
            this.tabs.set(username, { page, lastUsed: Date.now() });
            this.roomLane.set(username, lane.host);
            resolve({
              roomId,
              protoBase64: buf.toString('base64'),
              cookieHeader: cookies,
              userAgent: VIEWER_UA,
              proxyHost: lane.host,
              elapsedMs: Date.now() - startedAt
            });
          } catch (error) {
            reject(error as Error);
          }
        };

        // The player fetches on its own after page init; on a reused tab (page
        // already live) it will not re-fetch, so trigger a fresh page load which
        // replays the player bootstrap for the same room.
        page.on('response', onResponse);
        const nav = firstCapture
          ? page.goto(`https://www.tiktok.com/@${username}/live`, {
              waitUntil: 'domcontentloaded',
              timeout: 30_000
            })
          : page.reload({ waitUntil: 'domcontentloaded', timeout: 30_000 });
        nav.catch((error: Error) => {
          if (settled) return;
          settled = true;
          clearTimeout(fail);
          page.off('response', onResponse);
          this.tabs.delete(username);
          this.roomLane.delete(username);
          this.benchLane(lane);
          void page.close().catch(() => undefined);
          reject(new Error(`navigation failed on proxy ${lane.host || 'direct'}: ${error.message}`));
        });
      });
    } finally {
      lane.active.delete(username);
    }
  }

  async close(): Promise<void> {
    this.tabs.clear();
    this.roomLane.clear();
    for (const lane of this.lanes.values()) {
      if (lane.browser) {
        try {
          await lane.browser.close();
        } catch {
          // Closing an already-dead browser is not an error.
        }
        lane.browser = null;
      }
    }
  }
}

/** The UA the viewer pages run with. Must match what the listener pins. */
export const VIEWER_UA =
  'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36';
