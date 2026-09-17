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
import { rm } from 'fs/promises';
/**
 * How many consecutive capture failures a lane tolerates before its browser
 * profile is wiped and rebuilt. See ProxyLane.consecutiveFailures.
 */
const PROFILE_ROTATE_FAILURES = 3;
/**
 * Fast-fail budget: if a freshly navigated page has not produced the live
 * player's root element by this deadline, the attempt is a zombie (page hung,
 * proxy stalling) and is aborted instead of burning the full per-attempt
 * timeout. In-cluster captures that succeed show the player root within a few
 * seconds of domcontentloaded; waiting longer only pays for lanes that were
 * never going to answer.
 */
const LIVE_READY_DEADLINE_MS = 15_000;
// Player roots only. A bare div[class*="live"] substring would match the
// site-wide LIVE nav and let a zombie page that reached domcontentloaded
// pass the heartbeat, defeating the fast-fail the deadline exists for.
const LIVE_PAGE_SELECTOR = '#live-player, #LoginCanvas, div[class*="LIVE"], div[class*="Live"]';
/**
 * How many concurrent room captures may run on one lane's browser. Every
 * capture renders a TikTok live page; on llvmpipe several simultaneous renders
 * starve each other's renderer loop and the slowest becomes an im/fetch
 * timeout. Over-budget captures wait their turn instead of piling on.
 */
const LANE_MAX_CONCURRENT_CAPTURES = 2;
/**
 * How many single-use prewarmed tabs a lane keeps parked on the TikTok home
 * page. A capture served from one skips the initial domain bootstrap (TLS,
 * cookies seeding, first-paint JS): on cold starts measured 2026-09-16 the
 * handshake overhead dominates before the player even starts. The tab is
 * handed out, navigated to the room and becomes that room's warm tab.
 */
const PREWARM_PER_LANE = 2;
const PREWARM_URL = 'https://www.tiktok.com/';
/**
 * Rejection marker for a race attempt that lost to another racer (or was
 * cancelled by the race owner after a winner settled): not a lane failure.
 * captureRoom's rejection handler checks the marker and skips
 * recordFailure, so a healthy lane does not accrue consecutiveFailures —
 * and eventually a profile rotation — for delivering a valid capture a
 * hair slower than its sibling.
 */
class RaceLostError extends Error {
    constructor(lane) {
        super(`lane race cancelled (${lane || 'direct'})`);
        this.name = 'RaceLostError';
    }
}
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
    lanes = new Map();
    /** Round-robin cursor over the lane keys in insertion order. */
    laneOrder = [];
    tabs = new Map();
    /** username -> lane host, so a room keeps its browser identity. */
    roomLane = new Map();
    roundRobin = 0;
    options;
    constructor(options = {}) {
        this.options = {
            tabIdleMs: 5 * 60_000,
            proxyCooldownMs: 10 * 60_000,
            maxLaneAttempts: 3,
            ...options
        };
        const hosts = options.proxyHosts ?? [];
        if (hosts.length === 0) {
            this.addLane('');
        }
        else {
            hosts.forEach((host) => this.addLane(host));
        }
    }
    addLane(host) {
        const lane = {
            host,
            browser: null,
            launching: null,
            benchedUntil: 0,
            consecutiveFailures: 0,
            prewarmed: [],
            prewarmRefilling: false,
            waiters: [],
            slots: 0,
            detached: false
        };
        this.lanes.set(host, lane);
        this.laneOrder = [...this.lanes.keys()];
        return lane;
    }
    /**
     * Refresh the proxy list (fed by the webshare API). Lanes for proxies that
     * disappeared are closed — their pinned rooms recapture on surviving lanes —
     * and new proxies get fresh lanes. Surviving lanes keep their browsers,
     * profiles and pinned rooms untouched.
     *
     * The credentials arrive with the list (webshare shape: shared user/pass):
     * a pool constructed from the static-list fallback may have none at all,
     * and pages must not authenticate with stale empty values.
     *
     * A pool that started direct (no static list) drops its direct lane once
     * real proxies arrive: a direct lane next to residential ones would eat
     * every Nth capture with TikTok's datacenter-IP refusal.
     */
    async refreshProxies(hosts, credentials, perLaneCredentials) {
        if (credentials) {
            this.options.proxyUser = credentials.username;
            this.options.proxyPass = credentials.password;
        }
        if (perLaneCredentials) {
            this.options.proxyCredentials = {
                ...this.options.proxyCredentials,
                ...perLaneCredentials
            };
        }
        const wanted = new Set(hosts);
        for (const [host, lane] of [...this.lanes]) {
            const gone = host !== '' && !wanted.has(host);
            const dropDirect = host === '' && hosts.length > 0;
            if (!gone && !dropDirect)
                continue;
            this.lanes.delete(host);
            this.laneOrder = [...this.lanes.keys()];
            try {
                await lane.browser?.close();
            }
            catch {
                // Closing an already-dead browser is not an error.
            }
            this.teardownLane(lane);
        }
        for (const host of hosts) {
            if (!this.lanes.has(host))
                this.addLane(host);
        }
    }
    pickLane(username) {
        // A room already captured keeps its lane: the profile's cookies are part
        // of the identity TikTok judged the first time.
        const pinnedHost = this.roomLane.get(username);
        if (pinnedHost !== undefined) {
            const lane = this.lanes.get(pinnedHost);
            if (lane && Date.now() >= lane.benchedUntil)
                return lane;
            // The pinned lane is benched or gone; fall through to picking another.
        }
        const now = Date.now();
        const all = this.laneOrder
            .map((h) => this.lanes.get(h))
            .filter((l) => l !== undefined);
        const available = all.filter((l) => now >= l.benchedUntil);
        const pool = available.length > 0 ? available : all;
        const lane = pool[this.roundRobin % pool.length];
        this.roundRobin++;
        return lane;
    }
    benchLane(lane) {
        // Only bench proxy lanes: the direct lane has nothing to rotate away from,
        // and benching it would just serialize captures for no benefit.
        if (lane.host !== '') {
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
            '--window-size=1920,1080'
            // No GPU flags: measured 2026-09-16 in-cluster (signing lab, A/B over
            // the same lane+room), the ANGLE-on-Vulkan/lavapipe stack wedges the
            // page entirely — zero webcast calls in 240s — while plain default GL
            // (llvmpipe) boots the player and captures im/fetch 200 with a full
            // 2613-byte payload on every attempt. The renderer spoof stopped
            // helping after the image's Chromium update; do not re-add GPU
            // renderer flags without a fresh in-cluster A/B.
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
            .then((browser) => {
            lane.browser = browser;
            lane.launching = null;
            // Park the first tabs now, not after the lane's first successful
            // capture: a lane's first cold capture is exactly the one that
            // benefits most from skipping the domain bootstrap.
            this.refillPrewarm(lane);
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
     * The warmed tab for a previously captured room, refreshed so idle
     * eviction cannot reclaim it while a relay subscriber is attached.
     * Undefined when the room has no warm tab (never captured, evicted, or
     * rotated away).
     */
    pinTab(username) {
        const entry = this.tabs.get(username);
        if (!entry)
            return undefined;
        entry.lastUsed = Date.now();
        return entry.page;
    }
    /**
     * Capture the initial fetch exchange for a live room. Resolves when the
     * page's player receives a non-trivial /im/fetch/ response; rejects after
     * `timeoutMs` overall. The first attempt runs alone on the pinned or
     * round-robin lane; if it fails, the remaining candidate lanes race in
     * parallel and the first capture wins, so a cold room on a bad lane costs
     * one attempt, not maxLaneAttempts sequential ones. Race losers are
     * cancelled clean: their page closes, but the lane is not benched and the
     * winner's warm tab stays registered.
     */
    async captureRoom(username, { timeoutMs = 120_000 } = {}) {
        this.evictIdleTabs();
        const attempts = Math.max(1, this.options.maxLaneAttempts);
        const overallStart = Date.now();
        const failures = [];
        const recordFailure = (lane, message, attempt) => {
            failures.push({ lane: lane.host || 'direct', error: message });
            lane.consecutiveFailures++;
            this.options.logger?.warn('viewer capture failed on lane', {
                username,
                lane: lane.host || 'direct',
                elapsed_ms: Date.now() - overallStart,
                attempt,
                consecutive_failures: lane.consecutiveFailures,
                error: message
            });
        };
        const recordSuccess = (lane, capture, attempt) => {
            lane.consecutiveFailures = 0;
            this.options.logger?.info('viewer capture ok', {
                username,
                lane: lane.host || 'direct',
                elapsed_ms: capture.elapsedMs,
                attempt
            });
            return capture;
        };
        const firstLane = this.pickLane(username);
        const firstResult = await this.attemptOnLane(username, firstLane, Math.max(60_000, Math.floor(timeoutMs / attempts)), overallStart).promise.then((capture) => ({ ok: true, capture }), (error) => ({ ok: false, error }));
        if (firstResult.ok)
            return recordSuccess(firstLane, firstResult.capture, 1);
        recordFailure(firstLane, firstResult.error.message, 1);
        if (firstLane.consecutiveFailures >= PROFILE_ROTATE_FAILURES) {
            await this.rotateLaneProfile(firstLane);
        }
        // Race the remaining candidate lanes: first capture wins, losers are
        // cancelled by the cancel flag each attempt checks between phases. A
        // slow lane then costs one fast-fail deadline, not the whole caller
        // budget.
        const remaining = timeoutMs - (Date.now() - overallStart);
        if (remaining <= 0 || attempts <= 1) {
            throw this.captureFailure(failures);
        }
        const now = Date.now();
        const candidates = [...this.lanes.values()].filter((l) => l !== firstLane && now >= l.benchedUntil);
        if (candidates.length === 0) {
            // Every other lane is benched: nothing to race, the first failure stands.
            throw this.captureFailure(failures);
        }
        const racing = candidates.slice(0, attempts - 1);
        const raceStart = Date.now();
        const cancels = [];
        // Shared across all racers: the first to reach a capture claims the
        // registration; the others close their pages without registering.
        const claim = { winner: false };
        const racers = racing.map((lane) => {
            const { promise, cancel } = this.attemptOnLane(username, lane, remaining, raceStart, {
                registerCancel: (fn) => cancels.push(fn),
                claim
            });
            return promise.then((capture) => ({ lane, capture }), (error) => {
                if (error instanceof RaceLostError) {
                    // Lost the race to a sibling racer (or cancelled by the owner
                    // after a winner settled): not a lane failure — a lane that
                    // delivered a valid capture a hair slower must not accrue
                    // consecutiveFailures toward profile rotation.
                    throw new Error(`race loser (${lane.host}): cancelled`);
                }
                recordFailure(lane, error.message, 2);
                throw error;
            });
        });
        try {
            const winner = await Promise.any(racers);
            const drain = cancels.splice(0);
            for (const cancel of drain)
                cancel();
            return recordSuccess(winner.lane, winner.capture, 2);
        }
        catch {
            const drain = cancels.splice(0);
            for (const cancel of drain)
                cancel();
            // Per-lane failures were recorded in each racer's rejection handler.
            throw this.captureFailure(failures);
        }
    }
    captureFailure(failures) {
        const summary = failures.map((f) => `${f.lane}: ${f.error}`).join(' | ');
        return new Error(`viewer capture failed across ${failures.length} lane(s): ${summary}`);
    }
    /**
     * Close the lane's browser, wipe its profile directory, and clear the
     * failure counter so the next capture builds a fresh session identity.
     * Pinned rooms on the lane are unpinned and their tabs dropped; they
     * recapture on the next call.
     */
    async rotateLaneProfile(lane) {
        const profileDir = `${this.options.userDataDir ?? '/tmp/tiktok-signer-profile-viewer'}-${lane.host || 'direct'}`;
        this.options.logger?.warn('rotating lane profile after repeated capture failures', {
            lane: lane.host || 'direct',
            consecutive_failures: lane.consecutiveFailures,
            profile_dir: profileDir
        });
        lane.consecutiveFailures = 0;
        try {
            await lane.browser?.close();
        }
        catch {
            // Closing an already-dead browser is not an error.
        }
        this.teardownLane(lane);
        await rm(profileDir, { recursive: true, force: true }).catch(() => undefined);
    }
    /**
     * Tear down a lane's runtime state after its browser is closed: pinned
     * rooms dropped, parked tabs closed, queued slot waiters released. Shared
     * by proxy removal (refreshProxies) and profile rotation.
     */
    teardownLane(lane) {
        lane.browser = null;
        lane.launching = null;
        lane.detached = true;
        for (const [username, laneHost] of [...this.roomLane]) {
            if (laneHost === lane.host) {
                this.roomLane.delete(username);
                const tab = this.tabs.get(username);
                if (tab) {
                    this.tabs.delete(username);
                    void tab.page.close().catch(() => undefined);
                }
            }
        }
        for (const page of lane.prewarmed)
            void page.close().catch(() => undefined);
        lane.prewarmed = [];
        // Queued waiters must not book a slot on the detached lane: its proxy
        // is gone from this.lanes, and a booked waiter would launch a browser
        // for it that nothing ever closes. The detached flag makes their
        // release() resolve false (lane gone, fail fast).
        for (const release of lane.waiters.splice(0))
            release();
    }
    /**
     * Book one of the lane's capture slots. Concurrent room captures share one
     * browser and its renderer loop; on llvmpipe the contention turns the
     * slowest page into an im/fetch timeout. Over-budget captures queue and
     * take a slot when a current holder's attempt finishes, or give up when
     * the attempt budget runs out.
     */
    async acquireLaneSlot(lane, timeoutMs) {
        if (lane.slots < LANE_MAX_CONCURRENT_CAPTURES) {
            lane.slots++;
            return true;
        }
        const deadline = Date.now() + timeoutMs;
        const booked = await new Promise((resolve) => {
            const release = () => {
                if (lane.detached) {
                    lane.waiters = lane.waiters.filter((w) => w !== release);
                    resolve(false);
                }
                else if (lane.slots < LANE_MAX_CONCURRENT_CAPTURES) {
                    lane.slots++;
                    lane.waiters = lane.waiters.filter((w) => w !== release);
                    resolve(true);
                }
                else if (Date.now() >= deadline) {
                    lane.waiters = lane.waiters.filter((w) => w !== release);
                    resolve(false);
                }
            };
            lane.waiters.push(release);
        });
        return booked;
    }
    releaseLaneSlot(lane) {
        lane.slots = Math.max(0, lane.slots - 1);
        lane.waiters.shift()?.();
    }
    /** A page parked on the TikTok home page, ready to be handed to a capture. */
    async takePrewarmedTab(lane) {
        const parked = lane.prewarmed.pop();
        if (parked) {
            // Verify it survived; a crashed tab would hang the capture. A dead
            // one permanently consumed a prewarm slot until the next successful
            // capture refilled it, so kick a refill here too.
            let dead = false;
            try {
                dead = parked.isClosed();
            }
            catch {
                dead = true;
            }
            if (dead) {
                this.refillPrewarm(lane);
                return undefined;
            }
            return parked;
        }
        return undefined;
    }
    /** Refill the lane's prewarm slots after a capture, best-effort. */
    refillPrewarm(lane) {
        if (lane.prewarmRefilling)
            return;
        if (lane.prewarmed.length >= PREWARM_PER_LANE)
            return;
        lane.prewarmRefilling = true;
        void this.ensureBrowser(lane)
            .then(async (browser) => {
            while (lane.prewarmed.length < PREWARM_PER_LANE) {
                const page = await this.newLanePage(lane, browser);
                await page.goto(PREWARM_URL, { waitUntil: 'domcontentloaded', timeout: 30_000 });
                lane.prewarmed.push(page);
            }
        })
            .catch(() => undefined)
            .finally(() => {
            lane.prewarmRefilling = false;
        });
    }
    /** New page configured for this lane: UA, request blocking, proxy auth. */
    async newLanePage(lane, browser) {
        const page = await browser.newPage();
        await page.setUserAgent(VIEWER_UA);
        // The tab must not stream the video: with a residential proxy wired
        // (the answer to TikTok's datacenter-IP gating), media would be 99% of
        // the proxy's bill. Images, stylesheets and trackers are likewise dead
        // weight the player never needs — measured on the lab lane, blocking
        // them measurably shortens time-to-im/fetch and none of the player's
        // fetches were to those resource types. Known telemetry hosts are
        // blocked even though they sit on tiktok.com: the player never calls
        // them, and exempting "anything tiktok.com" would let analytics.tiktok
        // com straight back in. Non-media/font/image requests on tiktok.com,
        // tiktokcdn and webcast hosts are allowed through.
        await page.setRequestInterception(true);
        page.on('request', (request) => {
            const type = request.resourceType();
            if (type === 'media' || type === 'font' || type === 'image') {
                void request.abort().catch(() => undefined);
                return;
            }
            const url = request.url();
            if (isTrackerUrl(url)) {
                void request.abort().catch(() => undefined);
                return;
            }
            const firstParty = url.startsWith('https://www.tiktok.com/') ||
                url.startsWith('https://webcast') ||
                url.includes('.tiktok.com/') ||
                url.includes('.tiktokcdn.com/');
            if (!firstParty && type === 'stylesheet') {
                void request.abort().catch(() => undefined);
                return;
            }
            void request.continue().catch(() => undefined);
        });
        // Webshare issues one credential pair per proxy; a lane's own entry
        // wins, the shared pair is the fallback (static-list pools).
        const laneCreds = lane.host ? this.options.proxyCredentials?.[lane.host] : undefined;
        const authUser = laneCreds?.username ?? this.options.proxyUser;
        const authPass = laneCreds?.password ?? this.options.proxyPass;
        if (authUser && authPass) {
            await page.authenticate({ username: authUser, password: authPass });
        }
        return page;
    }
    /**
     * One capture attempt on a lane. Cancellable: a parallel lane race calls
     * the returned cancel when another racer wins — the cancelled attempt
     * closes its own page but does not bench the lane or unregister the
     * room's tab, both of which belong to the winner now.
     *
     * The lane slot is held for the whole attempt, not the setup: the cap
     * exists to bound simultaneous live-page renders on one llvmpipe browser,
     * and the render spans from navigation to im/fetch — released where the
     * attempt settles (success, giveUp, cancel or reject), never in a
     * finally over the setup block.
     *
     * Registration (tabs/roomLane) is race-winner-only: in a parallel race
     * two racers can both reach onResponse before either resolves, and the
     * loser must not overwrite the winner's entries — its page gets closed
     * by cancel, and a closed page registered as the room's warm tab turns
     * the next capture for that room into a guaranteed dead-tab attempt.
     * Two mechanisms keep it single-winner: the shared `claim` token
     * (first racer to reach a capture wins it; the rest loseRace and close
     * their own page, rejecting with RaceLostError so the lane accrues no
     * failure) and the owner-side `willRegister` flip, which cancels a
     * claimed winner mid-registration if the race owner settles against it.
     */
    attemptOnLane(username, lane, timeoutMs, startedAt, options = {}) {
        const { promise, resolve, reject } = Promise.withResolvers();
        const claim = options.claim;
        const willRegister = options.willRegister ?? { value: true };
        let cancelled = false;
        let fail;
        let heartbeat;
        let page = null;
        let onResponse = null;
        let slotHeld = false;
        const releaseSlotOnce = () => {
            if (!slotHeld)
                return;
            slotHeld = false;
            this.releaseLaneSlot(lane);
        };
        const cancel = () => {
            if (cancelled)
                return;
            cancelled = true;
            willRegister.value = false;
            clearTimeout(fail);
            clearTimeout(heartbeat);
            if (page && onResponse)
                page.off('response', onResponse);
            if (page)
                void page.close().catch(() => undefined);
            releaseSlotOnce();
            reject(new RaceLostError(lane.host));
        };
        options.registerCancel?.(cancel);
        void (async () => {
            const booked = await this.acquireLaneSlot(lane, timeoutMs);
            if (!booked) {
                reject(new Error(`lane busy (proxy ${lane.host || 'direct'}): no capture slot within ${timeoutMs}ms`));
                return;
            }
            slotHeld = true;
            if (cancelled) {
                // cancel() ran while the slot was being booked and could not release
                // it (slotHeld was still false there). Release it here.
                releaseSlotOnce();
                return;
            }
            try {
                // Reuse a warm tab when we have one for this room's streamer AND it
                // lives on this lane's browser — tabs are lane-bound pages and a
                // cross-lane reuse would mix session cookies across profiles, which
                // the profile-per-lane design exists to prevent. A racer reaching a
                // room whose warm tab sits on another lane takes a prewarmed/new
                // page on its own lane instead. Prewarmed parked tab serves a cold
                // room (skips the domain bootstrap); the browser only launches when
                // a brand-new page is needed.
                const existing = this.roomLane.get(username) === lane.host ? this.tabs.get(username) : undefined;
                const prewarmed = existing ? undefined : await this.takePrewarmedTab(lane);
                page = existing?.page ?? prewarmed ?? null;
                if (!page) {
                    page = await this.newLanePage(lane, await this.ensureBrowser(lane));
                    if (cancelled) {
                        // cancel() fired while the page was being created: it saw
                        // page === null and could not close this one. Release it here.
                        void page.close().catch(() => undefined);
                        return;
                    }
                }
                const capturePage = page;
                const giveUp = async (message) => {
                    if (cancelled)
                        return;
                    cancelled = true;
                    clearTimeout(fail);
                    clearTimeout(heartbeat);
                    capturePage.off('response', onResponse);
                    // Drop the tab only when this page is the room's registered one:
                    // in a parallel lane race the loser's page is its own, while the
                    // winner may have already registered a different tab for the room.
                    const owned = this.tabs.get(username);
                    if (!owned || owned.page === capturePage) {
                        this.tabs.delete(username);
                        this.roomLane.delete(username);
                    }
                    this.benchLane(lane);
                    await capturePage.close().catch(() => undefined);
                    releaseSlotOnce();
                    reject(new Error(`${message} (proxy ${lane.host || 'direct'})`));
                };
                fail = setTimeout(() => {
                    void giveUp(`im/fetch 200 not captured within ${timeoutMs}ms`);
                }, timeoutMs);
                onResponse = async (response) => {
                    if (cancelled)
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
                    const loseRace = async () => {
                        // A racer that reached a capture but is not the race's winner:
                        // close our own page, keep the winner's registration intact.
                        // Rejected with the marker so captureRoom does not count it
                        // as a lane failure.
                        cancelled = true;
                        clearTimeout(fail);
                        clearTimeout(heartbeat);
                        capturePage.off('response', onResponse);
                        await capturePage.close().catch(() => undefined);
                        releaseSlotOnce();
                        reject(new RaceLostError(lane.host));
                    };
                    if (!willRegister.value) {
                        // The race owner cancelled us (loser of the race).
                        await loseRace();
                        return;
                    }
                    if (claim) {
                        if (claim.winner) {
                            // Another racer won the claim between our check and here.
                            await loseRace();
                            return;
                        }
                        claim.winner = true;
                    }
                    cancelled = true;
                    clearTimeout(fail);
                    clearTimeout(heartbeat);
                    capturePage.off('response', onResponse);
                    try {
                        const cookies = (await capturePage.cookies('https://www.tiktok.com'))
                            .map((c) => `${c.name}=${c.value}`)
                            .join('; ');
                        if (!willRegister.value) {
                            // Lost the race while the cookie read was in flight (the
                            // owner's cancel() flipped the flag and closed the page).
                            await loseRace();
                            return;
                        }
                        const roomId = new URL(url).searchParams.get('room_id') ?? username;
                        this.tabs.set(username, { page: capturePage, lastUsed: Date.now() });
                        this.roomLane.set(username, lane.host);
                        this.refillPrewarm(lane);
                        releaseSlotOnce();
                        resolve({
                            roomId,
                            protoBase64: buf.toString('base64'),
                            cookieHeader: cookies,
                            userAgent: VIEWER_UA,
                            proxyHost: lane.host,
                            elapsedMs: Date.now() - startedAt
                        });
                    }
                    catch (error) {
                        releaseSlotOnce();
                        reject(error);
                    }
                };
                capturePage.on('response', onResponse);
                // The player fetches on its own after page init. On a reused tab the
                // page is already live and will not re-fetch, so instead of a full
                // reload (which replays the whole page bootstrap) click through the
                // SPA: the site is a React app, and an in-page navigation to the room
                // route re-runs just the player bootstrap against the warm session.
                const nav = existing
                    ? Promise.race([
                        capturePage.evaluate((target) => {
                            const anchor = document.createElement('a');
                            anchor.href = target;
                            anchor.style.display = 'none';
                            document.body.append(anchor);
                            anchor.click();
                            anchor.remove();
                            return true;
                        }, `https://www.tiktok.com/@${username}/live`),
                        // evaluate carries no timeout of its own; a wedged warm
                        // tab would otherwise hang the nav chain until the
                        // overall fail timer, skipping the reload fallback and
                        // the zombie fast-fail entirely.
                        new Promise((_, timeout) => setTimeout(() => timeout(new Error('spa navigate timed out')), 30_000))
                    ])
                        .then(() => capturePage.waitForNavigation({
                        waitUntil: 'domcontentloaded',
                        timeout: 30_000
                    }))
                        .catch(() => capturePage.reload({ waitUntil: 'domcontentloaded', timeout: 30_000 }))
                    : capturePage.goto(`https://www.tiktok.com/@${username}/live`, {
                        waitUntil: 'domcontentloaded',
                        timeout: 30_000
                    });
                nav
                    .then(() => {
                    // Fast-fail heartbeat, armed at domcontentloaded: successful
                    // captures show the live player within a few seconds of the
                    // document being ready. A page without the player root by
                    // LIVE_READY_DEADLINE_MS is a zombie — hung page, stalling
                    // proxy — and pays the deadline instead of the full timeout,
                    // freeing the lane slot for the next attempt. Armed on nav
                    // completion, not attempt start: a residential proxy can burn
                    // half the deadline on goto alone without the page being a
                    // zombie.
                    heartbeat = setTimeout(() => {
                        void capturePage
                            .$$(LIVE_PAGE_SELECTOR)
                            .then((els) => {
                            if (els.length === 0 && !cancelled) {
                                void giveUp(`live player not up within ${LIVE_READY_DEADLINE_MS}ms of navigation (zombie page fast-fail)`);
                            }
                        })
                            .catch(() => undefined);
                    }, LIVE_READY_DEADLINE_MS);
                })
                    .catch((error) => {
                    if (cancelled)
                        return;
                    void giveUp(`navigation failed: ${error.message}`);
                });
                // The attempt runs on from here on timers and the response listener;
                // the slot must stay booked until the attempt settles, which is
                // exactly why it is not released in the outer finally.
            }
            catch (error) {
                releaseSlotOnce();
                reject(error);
            }
        })();
        return { promise, cancel };
    }
    async close() {
        this.tabs.clear();
        this.roomLane.clear();
        for (const lane of this.lanes.values()) {
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
/** Known third-party telemetry the player never depends on. */
function isTrackerUrl(url) {
    return (url.includes('analytics.tiktok.com') ||
        url.includes('log.byteoversea.net') ||
        url.includes('mon.byteoversea.net') ||
        url.includes('/collect') ||
        url.includes('ads-sdk'));
}
/** The UA the viewer pages run with. Must match what the listener pins. */
export const VIEWER_UA = 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 Safari/537.36';
/**
 * Identity matching VIEWER_UA, exposed via /v1/identity when viewer mode is
 * active. The connector pins its device presets to whatever /v1/identity
 * reports, so the WebSocket handshake's User-Agent and browser_* params
 * describe the same browser that captured the session — a mismatch (the
 * signature-path Safari identity against a Chrome viewer capture) gets the
 * WS handshake answered with a plain HTTP 200 and no upgrade.
 */
export const VIEWER_IDENTITY = {
    userAgent: VIEWER_UA,
    browserPlatform: 'Linux x86_64',
    os: 'linux',
    screenWidth: 1920,
    screenHeight: 1080
};
//# sourceMappingURL=viewer.js.map