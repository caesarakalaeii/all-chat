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
 * Tabs stay open after a capture: the page keeps receiving live push (which
 * keeps the session warm on TikTok's side) and the next capture for the same
 * room reuses its warmed session identity instead of bootstrapping a new one.
 */
export class ViewerPool {
    browser = null;
    tabs = new Map();
    launching = null;
    options;
    constructor(options = {}) {
        this.options = { tabIdleMs: 5 * 60_000, ...options };
    }
    async ensureBrowser() {
        if (this.browser)
            return this.browser;
        if (this.launching)
            return this.launching;
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
        if (this.options.proxyHost) {
            args.push(`--proxy-server=http://${this.options.proxyHost}`);
        }
        this.launching = puppeteer
            .launch({
            // Non-headless, always: measured 2026-09-15, headless Chromium —
            // stealth plugin included — gets im/fetch 403 on every attempt while
            // the same binary on a real display gets 200 with a full protobuf
            // payload in ~5s. Headless here is not a trade-off, it is broken.
            headless: false,
            executablePath: this.options.executablePath,
            args,
            userDataDir: this.options.userDataDir,
            ignoreDefaultArgs: ['--enable-automation']
        })
            .then((browser) => {
            this.browser = browser;
            this.launching = null;
            return browser;
        });
        return this.launching;
    }
    evictIdleTabs() {
        const now = Date.now();
        for (const [roomId, entry] of this.tabs) {
            if (now - entry.lastUsed > this.options.tabIdleMs) {
                this.tabs.delete(roomId);
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
        const browser = await this.ensureBrowser();
        const startedAt = Date.now();
        // Reuse a warm tab when we have one for this room's streamer.
        const existing = this.tabs.get(username);
        const page = existing
            ? existing.page
            : await browser.newPage().then(async (p) => {
                await p.setUserAgent(VIEWER_UA);
                if (this.options.proxyUser && this.options.proxyPass) {
                    await p.authenticate({
                        username: this.options.proxyUser,
                        password: this.options.proxyPass
                    });
                }
                return p;
            });
        const firstCapture = !existing;
        return await new Promise((resolve, reject) => {
            let settled = false;
            const jobId = randomUUID(); // correlation for logs; capture is per-tab
            const fail = setTimeout(async () => {
                if (settled)
                    return;
                settled = true;
                // Drop the tab: a page that never produced a fetch is a dead session.
                this.tabs.delete(username);
                await page.close().catch(() => undefined);
                reject(new Error(`im/fetch 200 not captured within ${timeoutMs}ms (${jobId})`));
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
                    resolve({
                        roomId,
                        protoBase64: buf.toString('base64'),
                        cookieHeader: cookies,
                        userAgent: VIEWER_UA,
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
                void page.close().catch(() => undefined);
                reject(error);
            });
        });
    }
    async close() {
        this.tabs.clear();
        if (this.browser) {
            try {
                await this.browser.close();
            }
            catch {
                // Closing an already-dead browser is not an error.
            }
            this.browser = null;
        }
    }
}
/** The UA the viewer pages run with. Must match what the listener pins. */
export const VIEWER_UA = 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36';
//# sourceMappingURL=viewer.js.map