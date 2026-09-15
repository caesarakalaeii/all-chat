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
import StealthPlugin from 'puppeteer-extra-plugin-stealth';
import { randomUUID } from 'crypto';
// Stealth evasions: TikTok's secSDK fingerprints the browser environment, and
// since 2026-09-09 TikTok refuses webcast data endpoints to sessions that look
// automated (zerodytrash/TikTok-Live-Connector#329). Same plugin stack the
// signature path uses.
const puppeteer = puppeteerExtra;
puppeteer.use(StealthPlugin());
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
 * the lane for a cooldown and the next attempt rides another one.
 *
 * Tabs stay open after a capture: the page keeps receiving live push (which
 * keeps the session warm on TikTok's side) and the next capture for the same
 * room reuses its warmed session identity instead of bootstrapping a new one.
 */
export class ViewerPool {
    lanes = [];
    tabs = new Map();
    /** username -> lane index, so a room keeps its browser identity. */
    roomLane = new Map();
    roundRobin = 0;
    options;
    constructor(options = {}) {
        this.options = {
            tabIdleMs: 5 * 60_000,
            proxyCooldownMs: 10 * 60_000,
            ...options
        };
        const hosts = options.proxyHosts ?? [];
        if (hosts.length === 0) {
            this.lanes.push(this.newLane(0));
        }
        else {
            hosts.forEach((_, i) => this.lanes.push(this.newLane(i)));
        }
    }
    newLane(index) {
        return { index, browser: null, launching: null, active: new Set(), benchedUntil: 0 };
    }
    laneProxyHost(index) {
        const hosts = this.options.proxyHosts ?? [];
        return hosts[index];
    }
    pickLane(username) {
        // A room already captured keeps its lane: the profile's cookies are part
        // of the identity TikTok judged the first time.
        const pinned = this.roomLane.get(username);
        if (pinned !== undefined) {
            const lane = this.lanes[pinned];
            if (Date.now() >= lane.benchedUntil)
                return lane;
            // The pinned lane is benched; fall through to picking another.
        }
        const now = Date.now();
        const available = this.lanes.filter((l) => now >= l.benchedUntil);
        const pool = available.length > 0 ? available : this.lanes;
        const lane = pool[this.roundRobin % pool.length];
        this.roundRobin++;
        return lane;
    }
    benchLane(lane) {
        // Only bench proxy lanes: the direct lane has nothing to rotate away from,
        // and benching it would just serialize captures for no benefit.
        if ((this.options.proxyHosts ?? []).length > 0) {
            lane.benchedUntil = Date.now() + this.options.proxyCooldownMs;
        }
    }
    async ensureBrowser(lane) {
        if (lane.browser)
            return lane.browser;
        if (lane.launching)
            return lane.launching;
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
        const proxyHost = this.laneProxyHost(lane.index);
        if (proxyHost) {
            args.push(`--proxy-server=http://${proxyHost}`);
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
            userDataDir: `${this.options.userDataDir ?? '/tmp/tiktok-signer-profile-viewer'}-p${lane.index}`,
            ignoreDefaultArgs: ['--enable-automation']
        })
            .then((browser) => {
            lane.browser = browser;
            lane.launching = null;
            return browser;
        });
        return lane.launching;
    }
    evictIdleTabs() {
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
    async captureRoom(username, { timeoutMs = 45_000 } = {}) {
        this.evictIdleTabs();
        const lane = this.pickLane(username);
        const browser = await this.ensureBrowser(lane);
        const startedAt = Date.now();
        lane.active.add(username);
        // Reuse a warm tab when we have one for this room's streamer.
        const existing = this.tabs.get(username);
        const page = existing
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
            return await new Promise((resolve, reject) => {
                let settled = false;
                const jobId = randomUUID(); // correlation for logs; capture is per-tab
                const fail = setTimeout(async () => {
                    if (settled)
                        return;
                    settled = true;
                    // Drop the tab: a page that never produced a fetch is a dead session.
                    this.tabs.delete(username);
                    this.roomLane.delete(username);
                    this.benchLane(lane);
                    await page.close().catch(() => undefined);
                    reject(new Error(`im/fetch 200 not captured within ${timeoutMs}ms (proxy ${lane.index}, ${jobId})`));
                }, timeoutMs);
                const onResponse = async (response) => {
                    if (settled)
                        return;
                    const url = response.url();
                    if (!url.includes('/webcast/im/fetch/'))
                        return;
                    if (response.status() !== 200)
                        return;
                    let buf;
                    try {
                        buf = await response.buffer();
                    }
                    catch {
                        return;
                    }
                    if (buf.length < 1000)
                        return; // keepalive/empty shapes are not a capture
                    settled = true;
                    clearTimeout(fail);
                    page.off('response', onResponse);
                    try {
                        const cookies = (await page.cookies('https://www.tiktok.com'))
                            .map((c) => `${c.name}=${c.value}`)
                            .join('; ');
                        const roomId = new URL(url).searchParams.get('room_id') ?? username;
                        this.tabs.set(username, { page, lastUsed: Date.now() });
                        this.roomLane.set(username, lane.index);
                        resolve({
                            roomId,
                            protoBase64: buf.toString('base64'),
                            cookieHeader: cookies,
                            userAgent: VIEWER_UA,
                            proxyIndex: lane.index,
                            elapsedMs: Date.now() - startedAt
                        });
                    }
                    catch (error) {
                        reject(error);
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
                nav.catch((error) => {
                    if (settled)
                        return;
                    settled = true;
                    clearTimeout(fail);
                    page.off('response', onResponse);
                    this.tabs.delete(username);
                    this.roomLane.delete(username);
                    this.benchLane(lane);
                    void page.close().catch(() => undefined);
                    reject(new Error(`navigation failed on proxy ${lane.index}: ${error.message}`));
                });
            });
        }
        finally {
            lane.active.delete(username);
        }
    }
    async close() {
        this.tabs.clear();
        this.roomLane.clear();
        for (const lane of this.lanes) {
            if (lane.browser) {
                try {
                    await lane.browser.close();
                }
                catch {
                    // Closing an already-dead browser is not an error.
                }
                lane.browser = null;
            }
        }
    }
}
/** The UA the viewer pages run with. Must match what the listener pins. */
export const VIEWER_UA = 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36';
//# sourceMappingURL=viewer.js.map