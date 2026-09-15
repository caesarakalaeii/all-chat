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
 * The signing session: one headless Chromium that loads a real TikTok page with
 * TikTok's own web SDK injected locally, and then answers sign requests by
 * asking that SDK to compute X-Bogus for a URL we hand it. X-Gnarly we compute
 * with the vendored encoder (vendor/xgnarly.mjs), using the page's msToken and
 * user agent so all three signature parameters describe the same browser.
 *
 * Approach and both vendored files come from carcabot/tiktok-signature (MIT);
 * see vendor/PROVENANCE.md. The reason to keep this architecture rather than a
 * pure-algorithm port: TikTok churns its signature algorithms, and running
 * without a code change here.
 */

import { randomBytes } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import puppeteerExtra from 'puppeteer-extra';
import type { PuppeteerExtra } from 'puppeteer-extra/dist/index.js';
import StealthPlugin from 'puppeteer-extra-plugin-stealth';
import type { Browser, Page } from 'puppeteer';
// Typed import of the vendored encoder. Plain relative path so tsc copies the
// dependency-free module as-is and NodeNext resolves it next to the build.
import { encode as encodeXGnarly } from '../../vendor/xgnarly.mjs';

const puppeteer = puppeteerExtra as unknown as PuppeteerExtra;
puppeteer.use(StealthPlugin());

/** Stable identity the signer presents to TikTok. Exposed via GET /identity. */
export interface SignerIdentity {
  userAgent: string;
  browserPlatform: string;
  os: string;
  screenWidth: number;
  screenHeight: number;
}

const IDENTITY: SignerIdentity = {
  // Safari on macOS. Everything TikTok sees (User-Agent, browser_* query
  // params, the fingerprint X-Gnarly encodes) must agree with this, so it is
  // one object and callers read it from /identity instead of guessing.
  userAgent:
    'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.6 Safari/605.1.15',
  browserPlatform: 'MacIntel',
  os: 'mac',
  screenWidth: 1920,
  screenHeight: 1080
};

export interface SignedUrl {
  signedUrl: string;
  userAgent: string;
  cookies: string;
}

export interface SigningSessionOptions {
  /** Proxy as host:port. Proxying is the documented answer to datacenter-IP blocks; see README. */
  proxyHost?: string;
  proxyUser?: string;
  proxyPass?: string;
  /** Restart the browser after this many signatures (default 500, as upstream). */
  maxGenerationsBeforeRefresh?: number;
  /** Restart the browser after this much session age in ms (default 30 min, as upstream). */
  maxSessionAgeMs?: number;
  /** Directory for the browser profile; must be writable (emptyDir in k8s). */
  userDataDir?: string;
  /** Where Chromium is; resolved by puppeteer when unset. */
  executablePath?: string;
  /** Page used for the warm-up navigation to TikTok. */
  warmUpTarget?: string;
  /** Tests only: skip warm-up *and* the browser entirely — no network, no Chromium. */
  skipWarmUp?: boolean;
  /**
   * Attach the vendored X-Gnarly encoder's output. Off by default: measured
   * 2026-09-15, /im/fetch/ 403s with our X-Gnarly and 200s without it.
   * See finishSign before enabling.
   */
  attachXGnarly?: boolean;
}

interface SdkEvaluation {
  urlBase: string;
  queryString: string;
  xBogus: string;
  msTokenUsed: string;
  userAgent: string;
  cookies: string;
  counters: {
    totalXHRRequests?: number;
    totalFetchRequests?: number;
    interceptedXHRRequests?: number;
    interceptedFetchRequests?: number;
  };
}

interface QueuedJob {
  run: () => Promise<unknown>;
  resolve: (value: unknown) => void;
  reject: (error: unknown) => void;
}

/** A minimal structural view of window.__sdkN for evaluate; TikTok obfuscates the real shape. */
type SdkTable = Record<number, { v?: unknown }> & { u?: unknown; B?: { o?: unknown }; o?: unknown };

/**
 * One live browser session. Signs requests strictly sequentially through the
 * queue: the SDK's internal counter state is per-page, so concurrent evaluate
 * calls would race it.
 */
export class SigningSession {
  private browser: Browser | null = null;
  private page: Page | null = null;
  private generationCount = 0;
  private initializedAt = 0;
  private initializing: Promise<void> | null = null;
  private readonly queue: QueuedJob[] = [];
  private draining = false;

  private readonly options: Required<
    Pick<SigningSessionOptions, 'maxGenerationsBeforeRefresh' | 'maxSessionAgeMs'>
  > &
    SigningSessionOptions;

  constructor(options: SigningSessionOptions = {}) {
    this.options = {
      maxGenerationsBeforeRefresh: options.maxGenerationsBeforeRefresh ?? 500,
      maxSessionAgeMs: options.maxSessionAgeMs ?? 30 * 60 * 1000,
      ...options
    };
  }

  get identity(): SignerIdentity {
    return IDENTITY;
  }

  isReady(): boolean {
    return this.browser !== null && this.page !== null;
  }

  /**
   * Sign a TikTok URL: strip stale signature params, compute fresh
   * X-Bogus (via the in-page SDK), msToken and X-Gnarly.
   */
  async signUrl(targetUrl: string): Promise<SignedUrl> {
    return this.enqueue(async () => {
      await this.ensureReady();

      const evaluation = await this.evaluateSdk(targetUrl);
      if ('error' in evaluation) {
        // The SDK can detach from the page (navigation, crash). Rebuild the
        // session once and retry, mirroring upstream's recovery path.
        await this.rebuild('SDK detached: ' + evaluation.error);
        const retry = await this.evaluateSdk(targetUrl);
        if ('error' in retry) {
          throw new Error(`X-Bogus computation failed: ${retry.error}`);
        }
        return this.finishSign(targetUrl, retry);
      }
      return this.finishSign(targetUrl, evaluation);
    });
  }

  async close(): Promise<void> {
    this.queue.length = 0;
    if (this.browser) {
      try {
        await this.browser.close();
      } catch {
        // Closing an already-dead browser is not an error.
      }
      this.browser = null;
      this.page = null;
    }
    this.generationCount = 0;
    this.initializedAt = 0;
  }

  private async finishSign(targetUrl: string, evaluation: SdkEvaluation): Promise<SignedUrl> {
    const url = new URL(evaluation.urlBase);
    url.searchParams.set('X-Bogus', evaluation.xBogus);
    if (url.searchParams.get('msToken') !== evaluation.msTokenUsed) {
      url.searchParams.set('msToken', evaluation.msTokenUsed);
    }

    // X-Gnarly is deliberately NOT attached by default. Measured 2026-09-15:
    // /webcast/im/fetch/ accepts msToken + X-Bogus (200) and rejects the same
    // request carrying our vendored X-Gnarly (403) — TikTok validates the
    // parameter server-side and the vendored encoder has drifted from whatever
    // the live web client computes. When TikTok starts *requiring* X-Gnarly,
    // re-validate the encoder against the live client before flipping this on
    // (scripts/ has the bisect harness), or grab the parameter from the page
    // intercept path like upstream's navigateTo mode does.
    if (this.options.attachXGnarly) {
      const xGnarly = encodeXGnarly(evaluation.queryString, '', evaluation.userAgent, evaluation.counters, {
        ubcode: 4,
        sdkVersion: '1.0.0.368'
      });
      url.searchParams.set('X-Gnarly', xGnarly);
    }

    this.generationCount++;
    return {
      signedUrl: url.toString(),
      userAgent: evaluation.userAgent,
      cookies: evaluation.cookies
    };
  }

  private async evaluateSdk(targetUrl: string): Promise<SdkEvaluation | { error: string }> {
    const page = this.page;
    if (!page) return { error: 'no page' };

    return page.evaluate((url: string) => {
      // The vendored SDK exposes byted_acrawler.frontierSign: the documented
      // signing entry that returns X-Bogus for a query string. __sdkN (which
      // upstream's newest code reaches into) is an obfuscated internal this
      // SDK build does not expose, so frontierSign is the stable contract.
      const acrawler = (window as unknown as {
        byted_acrawler?: { frontierSign?: (args: unknown) => Record<string, unknown> };
      }).byted_acrawler;
      if (!acrawler || typeof acrawler.frontierSign !== 'function') {
        return { error: 'SDK not initialized' };
      }
      const frontierSign = acrawler.frontierSign;

      const u = new URL(url);
      u.searchParams.delete('X-Bogus');
      u.searchParams.delete('X-Gnarly');
      const msTokenMatches = [...document.cookie.matchAll(/msToken=([^;]+)/g)];
      const msToken = msTokenMatches.length
        ? msTokenMatches[msTokenMatches.length - 1][1]
        : '';
      u.searchParams.set('msToken', msToken);
      const queryString = u.search.slice(1);

      // The SDK tracks how many requests the page has made and mixes those
      // counters into the signature; feeding plausible counters rather than
      // zeros matches what a real page looks like.
      let counters = {
        totalXHRRequests: 60,
        totalFetchRequests: 40,
        interceptedXHRRequests: 10,
        interceptedFetchRequests: 5
      };
      try {
        const cap3 = (window as unknown as { __cap3?: unknown[] }).__cap3 || [];
        const lastNat = [...cap3]
          .reverse()
          .find(
            (c) =>
              (c as { fn?: string }).fn === 'gnarly_x' &&
              (c as { args?: unknown[] }).args?.[3] &&
              ((c as { args?: Array<{ v?: unknown }> }).args?.[3] as { v?: unknown }).v
          );
        const v = lastNat && ((lastNat as { args?: Array<{ v?: unknown }> }).args?.[3] as { v?: unknown }).v;
        if (v) {
          const fromCap = v as Record<string, { v?: number }>;
          const totalXHR = fromCap.totalXHRRequests?.v || 0;
          const totalFetch = fromCap.totalFetchRequests?.v || 0;
          if (totalXHR + totalFetch > 0) {
            counters = {
              totalXHRRequests: totalXHR,
              totalFetchRequests: totalFetch,
              interceptedXHRRequests: fromCap.interceptedXHRRequests?.v || 0,
              interceptedFetchRequests: fromCap.interceptedFetchRequests?.v || 0
            };
          }
        }
      } catch {
        // counters stay at their plausible defaults
      }
      try {
        // frontierSign signs a full URL and returns the X-Bogus parameter it
        // computed; the empty string matches the no-body GET we issue.
        const signed = frontierSign({ url: u.toString(), method: 'GET' });
        const xb = signed?.['X-Bogus'];
        if (typeof xb !== 'string' || xb.length === 0) {
          return { error: 'frontierSign returned no X-Bogus' };
        }
        return {
          urlBase: u.toString(),
          queryString,
          xBogus: xb,
          msTokenUsed: msToken,
          userAgent: navigator.userAgent,
          cookies: document.cookie,
          counters
        };
      } catch (e) {
        return { error: (e as Error).message ?? String(e) };
      }
    }, targetUrl);
  }

  /** Debug helper: fetch a URL from inside the warmed page. Not for prod paths. */
  async debugFetchInPage(url: string): Promise<{ status: number; bodyLength: number }> {
    const page = this.page;
    if (!page) throw new Error('no page');
    return page.evaluate(async (target: string) => {
      const response = await fetch(target, { credentials: 'include' });
      const body = await response.arrayBuffer();
      return { status: response.status, bodyLength: body.byteLength };
    }, url);
  }

  private async ensureReady(): Promise<void> {
    if (this.shouldRefresh()) {
      await this.rebuild('session refresh due');
      return;
    }
    if (this.isReady()) return;
    await this.initialize();
  }

  private shouldRefresh(): boolean {
    if (!this.initializedAt) return false;
    return (
      this.generationCount >= this.options.maxGenerationsBeforeRefresh ||
      Date.now() - this.initializedAt >= this.options.maxSessionAgeMs
    );
  }

  private async rebuild(reason: string): Promise<void> {
    await this.close();
    try {
      await this.initialize();
    } catch (error) {
      // Fresh session failed to come up; surface the original reason plus this.
      throw new Error(`session rebuild after "${reason}" failed: ${(error as Error).message}`);
    }
  }

  private async initialize(): Promise<void> {
    // Serialize initialization: concurrent first requests would each launch a browser.
    if (this.initializing) {
      await this.initializing;
      return;
    }
    this.initializing = (async () => {
      // Tests run without Chromium: skipWarmUp means no browser at all, so
      // sign jobs fail fast through the normal error path instead of
      // stalling on a launch (or succeeding on a machine that happens to
      // have one cached).
      if (this.options.skipWarmUp) {
        throw new Error('signing session not initialized (skipWarmUp)');
      }
      const sdkContent = await this.loadSdk();

      const browserArgs = [
        '--no-sandbox',
        '--disable-setuid-sandbox',
        '--disable-dev-shm-usage',
        '--disable-blink-features=AutomationControlled',
        '--disable-gpu',
        `--window-size=${IDENTITY.screenWidth},${IDENTITY.screenHeight}`
      ];
      if (this.options.proxyHost) {
        browserArgs.push(`--proxy-server=http://${this.options.proxyHost}`);
      }
      const browser = await puppeteer.launch({
        executablePath: this.options.executablePath,
        args: browserArgs,
        userDataDir: this.options.userDataDir,
        ignoreDefaultArgs: ['--enable-automation']
      });

      let page: Page;
      try {
        page = await browser.newPage();

        page.on('request', (request) => {
          // Serve our local copy of TikTok's SDK instead of theirs: keeps the
          // exact version we computed X-Gnarly against, and works even when
          // TikTok's CDN path changes. Everything else on the page loads
          // normally, keeping the session realistic.
          const url = request.url();
          if (url.includes('/webmssdk/')) {
            void request
              .respond({
                status: 200,
                contentType: 'application/javascript; charset=utf-8',
                body: sdkContent
              })
              .catch(() => request.abort().catch(() => undefined));
            return;
          }
          if (url.includes('slardar') || url.includes('acrawler.js')) {
            void request.abort().catch(() => undefined);
            return;
          }
          if (['image', 'media', 'font'].includes(request.resourceType())) {
            void request.abort().catch(() => undefined);
            return;
          }
          // Every intercepted request must be resolved or the page hangs forever.
          void request.continue().catch(() => undefined);
        });

        await page.setUserAgent(IDENTITY.userAgent);
        await page.setViewport({
          width: IDENTITY.screenWidth,
          height: IDENTITY.screenHeight
        });
        await page.evaluateOnNewDocument(() => {
          Object.defineProperty(navigator, 'platform', {
            get: () => 'MacIntel',
            configurable: true
          });
        });
        await page.setRequestInterception(true);

        if (this.options.proxyUser && this.options.proxyPass) {
          await page.authenticate({
            username: this.options.proxyUser,
            password: this.options.proxyPass
          });
        }

        await page.evaluateOnNewDocument((sdkCode: string) => {
          try {
            // The SDK is obfuscated and self-invoking; eval is how upstream
            // injects it, and it is our own vendored copy, not remote code.
            (0, eval)(sdkCode);
          } catch {
            // A failed injection surfaces as "SDK not initialized" on the
            // first sign, which triggers the rebuild-and-retry path.
          }
        }, sdkContent);

        // Warm up on a real profile page: the page bundle initializes __sdkN
        // (the X-Bogus table we call) on top of the injected SDK, and the visit
        // primes msToken + cookies the signatures need. Upstream's sequence:
        // load, scroll, reload — the reload is what makes the bundle stick.
        const warmUpTarget =
          this.options.warmUpTarget ?? 'https://www.tiktok.com/@tiktok';
        await page.goto(warmUpTarget, {
          waitUntil: 'domcontentloaded',
          timeout: 60_000
        });
        const { promise: scrollSettled, resolve: scrollResolve } = Promise.withResolvers<void>();
        setTimeout(scrollResolve, 2000);
        await scrollSettled;
        await page.reload({ waitUntil: 'domcontentloaded', timeout: 60_000 }).catch(() => undefined);
        const { promise: settled, resolve: settledResolve } = Promise.withResolvers<void>();
        setTimeout(settledResolve, 3000);
        await settled;

        const status = await page.evaluate(() => {
          // The vendored SDK exposes byted_acrawler on load; the page bundle
          // later builds __sdkN on top of it. Upstream gates on frontierSign.
          const acrawler = (window as unknown as { byted_acrawler?: { frontierSign?: unknown } })
            .byted_acrawler;
          return { hasSdk: !!acrawler && typeof acrawler.frontierSign === 'function' };
        });
        if (!status.hasSdk) {
          throw new Error('local webmssdk did not initialize on the warm-up page');
        }

        this.browser = browser;
        this.page = page;
        this.initializedAt = Date.now();
        this.generationCount = 0;
      } catch (error) {
        try {
          await browser.close();
        } catch {
          // Ignore secondary close failure.
        }
        throw error;
      }
    })();

    try {
      await this.initializing;
    } finally {
      this.initializing = null;
    }
  }

  private async loadSdk(): Promise<string> {
    const sdkPath = fileURLToPath(new URL('../../vendor/webmssdk_5.1.3.js', import.meta.url));
    return readFile(sdkPath, 'utf-8');
  }

  private enqueue<T>(run: () => Promise<T>): Promise<T> {
    const { promise, resolve, reject } = Promise.withResolvers<T>();
    // T is erased at the queue boundary; the wrapper restores it on resolve.
    this.queue.push({
      run: run as () => Promise<unknown>,
      resolve: resolve as (value: unknown) => void,
      reject
    });
    void this.drain();
    return promise;
  }

  private async drain(): Promise<void> {
    if (this.draining) return;
    this.draining = true;
    try {
      while (this.queue.length > 0) {
        const job = this.queue.shift() as QueuedJob;
        try {
          job.resolve(await job.run());
        } catch (error) {
          job.reject(error);
        }
      }
    } finally {
      this.draining = false;
    }
  }
}

/** Fresh, undeterministic X-Gnarly entropy: used by tests to pin it. */
export function gnarlyEntropy(): { randomLow16: Buffer; random32: Buffer; randomKey: Buffer } {
  return {
    randomLow16: randomBytes(2),
    random32: randomBytes(4),
    randomKey: randomBytes(48)
  };
}
