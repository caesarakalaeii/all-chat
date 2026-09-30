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

import { beforeEach, describe, expect, it } from 'vitest';

import type { SignConfiguration } from './config.js';
import {
  asRouteHandler,
  installSignConfiguration,
  type ConnectorGlobals,
  type InstallerLogger
} from './installer.js';
import type { SignAttempt, SignObserver } from './shadow.js';
import type { SignRequest, SignResult, WebcastSigner } from './signer.js';

/**
 * Stand-in for the connector's module-level mutable configuration.
 *
 * The real `RouteConfig`, `RoomIdRouteConfig`, `IsLiveRouteConfig` and `SignConfig` are plain
 * exported objects that the library mutates and reads at connect time, so an object with the
 * same fields is a faithful double — no module mocking required. That is the same property the
 * production code relies on to avoid forking the library.
 */
function makeGlobals(): ConnectorGlobals {
  return {
    routeConfig: {
      fetchSignedWebSocketFromProvider: async () => ({ marker: 'library-default' })
    },
    roomIdRouteConfig: { skipFetchRoomIdFromEulerRoute: false },
    isLiveRouteConfig: { skipFetchRoomIdFromEulerRoute: false },
    signConfig: {
      basePath: 'https://api.eulerstream.com',
      apiKey: undefined,
      cachedInstance: { warm: true }
    }
  };
}

class RecordingObserver implements SignObserver {
  readonly attempts: SignAttempt[] = [];
  recordSignAttempt(attempt: SignAttempt): void {
    this.attempts.push(attempt);
  }
}

class CapturingLogger implements InstallerLogger {
  readonly infos: { message: string; meta?: Record<string, unknown> }[] = [];
  readonly warns: { message: string; meta?: Record<string, unknown> }[] = [];
  info(message: string, meta?: Record<string, unknown>): void {
    this.infos.push({ message, meta });
  }
  warn(message: string, meta?: Record<string, unknown>): void {
    this.warns.push({ message, meta });
  }
}

class StubSigner implements WebcastSigner {
  calls: SignRequest[] = [];
  constructor(
    readonly name: string,
    private readonly result: SignResult = {
      fetchResult: {},
      fetchResultCookieHeader: 'c=1'
    }
  ) {}
  async sign(request: SignRequest): Promise<SignResult> {
    this.calls.push(request);
    return this.result;
  }
}

const baseConfig: SignConfiguration = {
  signerMode: 'euler',
  signerBaseUrl: '',
  selfSignFallback: true,
  disableEulerFallbacks: true,
  enableExtendedGiftInfo: false,
  eulerApiKey: ''
};

function install(
  globals: ConnectorGlobals,
  overrides: Partial<SignConfiguration>,
  signers: { euler: WebcastSigner; self?: WebcastSigner; pureNode?: WebcastSigner },
  observer: SignObserver = new RecordingObserver(),
  logger: InstallerLogger = new CapturingLogger()
) {
  return installSignConfiguration(
    globals,
    { ...baseConfig, ...overrides },
    signers,
    observer,
    logger
  );
}

describe('installSignConfiguration', () => {
  let globals: ConnectorGlobals;
  let euler: StubSigner;
  let logger: CapturingLogger;

  beforeEach(() => {
    globals = makeGlobals();
    euler = new StubSigner('euler');
    logger = new CapturingLogger();
  });

  describe('composite Euler fallbacks (step 4, the independent win)', () => {
    it('switches off the Euler leg of both composites by default', () => {
      // Room ID and is-live both try HTML then TikTok's API before reaching for Euler, so this
      // removes free-tier calls without removing a capability.
      install(globals, {}, { euler });

      expect(globals.roomIdRouteConfig.skipFetchRoomIdFromEulerRoute).toBe(true);
      expect(globals.isLiveRouteConfig.skipFetchRoomIdFromEulerRoute).toBe(true);
    });

    it('restores the fallback when explicitly re-enabled', () => {
      globals.roomIdRouteConfig.skipFetchRoomIdFromEulerRoute = true;
      globals.isLiveRouteConfig.skipFetchRoomIdFromEulerRoute = true;

      install(globals, { disableEulerFallbacks: false }, { euler });

      // The flag has to work in both directions, otherwise there is no way to roll the change
      // back without a redeploy of the previous image.
      expect(globals.roomIdRouteConfig.skipFetchRoomIdFromEulerRoute).toBe(false);
      expect(globals.isLiveRouteConfig.skipFetchRoomIdFromEulerRoute).toBe(false);
    });

    it('is independent of the signer mode', () => {
      install(globals, { signerMode: 'euler', disableEulerFallbacks: true }, { euler });

      expect(globals.roomIdRouteConfig.skipFetchRoomIdFromEulerRoute).toBe(true);
    });
  });

  describe('generic HTTP URL signing seam (gift list)', () => {
    it('is left untouched in euler mode', () => {
      const original = globals.routeConfig.fetchWebcastSignatureFromProvider;
      install(globals, { signerMode: 'euler' }, { euler });
      expect(globals.routeConfig.fetchWebcastSignatureFromProvider).toBe(original);
    });

    it('routes to the self signer when one is installed', async () => {
      const self = new (class extends StubSigner {
        signUrlCalls: Array<{ url: string; method?: string }> = [];
        async signUrl(url: string, method = 'GET') {
          this.signUrlCalls.push({ url, method });
          return { response: { signedUrl: url + '&signed=1', userAgent: 'UA' } };
        }
      })('self');
      install(globals, { signerMode: 'self' }, { euler, self });

      const handler = globals.routeConfig.fetchWebcastSignatureFromProvider;
      expect(handler).toBeDefined();
      if (!handler) throw new Error('URL seam handler not installed');
      const result = (await handler({ url: 'https://webcast.tiktok.com/x', method: 'POST' } as never)) as {
        response: { signedUrl: string };
      };
      // The connector reads response.signedUrl off this shape.
      expect(result.response.signedUrl).toBe('https://webcast.tiktok.com/x&signed=1');
      expect(self.signUrlCalls[0]).toEqual({
        url: 'https://webcast.tiktok.com/x',
        method: 'POST'
      });
    });
  });

  describe('euler mode', () => {
    it('leaves the library route untouched', async () => {
      const original = globals.routeConfig.fetchSignedWebSocketFromProvider;

      const report = install(globals, { signerMode: 'euler' }, { euler });

      // The baseline mode should not put our code on the connect path at all: its entire value
      // is being the unchanged, known-good comparison for the other two.
      expect(globals.routeConfig.fetchSignedWebSocketFromProvider).toBe(original);
      expect(report.signerName).toBeUndefined();
    });

    it('reports Euler as still needed for signatures', () => {
      expect(install(globals, { signerMode: 'euler' }, { euler }).eulerReachableForSignature)
        .toBe(true);
    });
  });

  describe('shadow mode', () => {
    it('installs a shadow signer that keeps Euler load bearing', async () => {
      const self = new StubSigner('self');
      const report = install(globals, { signerMode: 'shadow' }, { euler, self });

      expect(report.signerName).toBe('shadow(euler->self)');
      expect(globals.routeConfig.fetchSignedWebSocketFromProvider).not.toBe(
        makeGlobals().routeConfig.fetchSignedWebSocketFromProvider
      );
    });

    it('still reports Euler as needed, because Euler serves the connection', () => {
      const self = new StubSigner('self');

      expect(
        install(globals, { signerMode: 'shadow' }, { euler, self }).eulerReachableForSignature
      ).toBe(true);
    });
  });

  describe('self mode', () => {
    it('prefers our signer but keeps Euler behind it while the fallback is on', () => {
      const self = new StubSigner('self');

      const report = install(
        globals,
        { signerMode: 'self', selfSignFallback: true },
        { euler, self }
      );

      expect(report.signerName).toBe('fallback(self->euler)');
      expect(report.eulerReachableForSignature).toBe(true);
    });

    it('drops Euler entirely once the fallback is turned off', () => {
      const self = new StubSigner('self');

      const report = install(
        globals,
        { signerMode: 'self', selfSignFallback: false },
        { euler, self }
      );

      // This is the actual finish line for #698: only here is Euler off the signature path.
      expect(report.signerName).toBe('self');
      expect(report.eulerReachableForSignature).toBe(false);
    });
  });

  describe('pure-node mode', () => {
    it('installs the pure-node signer with no Euler fallback', () => {
      const pureNode = new StubSigner('pure-node');
      const report = install(
        globals,
        { signerMode: 'pure-node' },
        { euler, pureNode }
      );

      // The whole point of the mode: the leased session replaces the
      // Euler signature, and MeasuredSigner (not FallbackSigner) wraps
      // it, so a lease failure cannot silently dial Euler.
      expect(report.signerName).toBe('pure-node');
      expect(report.eulerReachableForSignature).toBe(false);
    });

    it('degrades to Euler with a warning when no pure-node signer is supplied', () => {
      const report = install(globals, { signerMode: 'pure-node' }, { euler }, undefined, logger);

      expect(report.signerName).toBeUndefined();
      expect(logger.warns[0].message).toMatch(/no pure-node signer/i);
      // Effective state, not requested: it stayed on Euler.
      expect(report.eulerReachableForSignature).toBe(true);
    });
  });

  describe('a mode that asks for a signer we do not have', () => {
    // The flag is deliberately allowed to be set before the signing spike lands, so that
    // deployment config and code can move independently. What must not happen is a listener that
    // refuses to start, or one that claims to have retired Euler when it has not.
    it.each(['shadow', 'self'] as const)('degrades %s to Euler rather than failing', mode => {
      const report = install(globals, { signerMode: mode }, { euler }, undefined, logger);

      expect(report.signerName).toBeUndefined();
      expect(globals.routeConfig.fetchSignedWebSocketFromProvider).toBeDefined();
    });

    it('warns loudly, since the requested configuration is not what is running', () => {
      install(globals, { signerMode: 'self' }, { euler }, undefined, logger);

      expect(logger.warns).toHaveLength(1);
      expect(logger.warns[0].message).toMatch(/no self signer/i);
    });

    it('reports Euler as reachable, not as retired', () => {
      // Reporting the requested configuration rather than the effective one would let a
      // dashboard show Euler retired while every signature still goes through them.
      const report = install(
        globals,
        { signerMode: 'self', selfSignFallback: false },
        { euler },
        undefined,
        logger
      );

      expect(report.eulerReachableForSignature).toBe(true);
    });
  });

  describe('Euler SDK configuration', () => {
    it('repoints basePath at our own sign service when one is configured', () => {
      install(globals, { signerBaseUrl: 'https://sign.all-chat.internal' }, { euler });

      expect(globals.signConfig.basePath).toBe('https://sign.all-chat.internal');
    });

    it('leaves basePath alone when we sign in-process', () => {
      install(globals, { signerBaseUrl: '' }, { euler });

      expect(globals.signConfig.basePath).toBe('https://api.eulerstream.com');
    });

    it('clears the cached client so a changed basePath actually takes effect', () => {
      // createEulerClient() returns SignConfig.cachedInstance if it is set, so without this a
      // repointed basePath is silently ignored and the flag looks broken.
      install(globals, { signerBaseUrl: 'https://sign.all-chat.internal' }, { euler });

      expect(globals.signConfig.cachedInstance).toBeUndefined();
    });

    it('applies an API key when one is supplied', () => {
      install(globals, { eulerApiKey: 'secret-key' }, { euler });

      expect(globals.signConfig.apiKey).toBe('secret-key');
    });

    it('does not overwrite an existing key with an empty one', () => {
      globals.signConfig.apiKey = 'from-env';

      install(globals, { eulerApiKey: '' }, { euler });

      expect(globals.signConfig.apiKey).toBe('from-env');
    });
  });

  it('is idempotent, so re-applying configuration is safe', () => {
    const self = new StubSigner('self');

    const first = install(globals, { signerMode: 'self' }, { euler, self });
    const second = install(globals, { signerMode: 'self' }, { euler, self });

    expect(second.signerName).toBe(first.signerName);
    expect(globals.roomIdRouteConfig.skipFetchRoomIdFromEulerRoute).toBe(true);
  });

  it('logs the effective configuration once', () => {
    install(globals, { signerMode: 'euler' }, { euler }, undefined, logger);

    expect(logger.infos).toHaveLength(1);
    expect(logger.infos[0].meta).toMatchObject({
      signer_mode: 'euler',
      euler_fallbacks_disabled: true,
      euler_reachable_for_signature: true
    });
  });
});

describe('asRouteHandler', () => {
  function webClient(cookie = 'sessionid=abc; tt-target-idc=alisg') {
    return {
      clientHeaders: { 'User-Agent': 'Mozilla/5.0 (test)' },
      cookieJar: { getCookieString: async () => cookie }
    };
  }

  it('passes the room, cursor and User-Agent through to the signer', async () => {
    const signer = new StubSigner('self');

    await asRouteHandler(signer)({
      roomId: '7300000000000000000',
      cursor: 'cursor-42',
      webClient: webClient()
    });

    expect(signer.calls[0]).toMatchObject({
      roomId: '7300000000000000000',
      cursor: 'cursor-42',
      // The signature is bound to the User-Agent, and TikTok checks that the request carrying it
      // agrees. Since the connector randomises a device preset per connection, this has to come
      // from the client rather than from the signer.
      userAgent: 'Mozilla/5.0 (test)'
    });
  });

  it('withholds the session cookie unless an authenticated socket was asked for', async () => {
    // Third-party credential exposure is one of the three costs #698 names. Even with our own
    // signer, forwarding a session cookie that nothing requested widens the blast radius of a
    // sign-service compromise for no benefit.
    const signer = new StubSigner('self');

    await asRouteHandler(signer)({ roomId: '73', webClient: webClient() });

    expect(signer.calls[0].cookieHeader).toBeUndefined();
  });

  it('forwards the session cookie when the socket is authenticated', async () => {
    const signer = new StubSigner('self');

    await asRouteHandler(signer)({
      roomId: '73',
      authenticateWs: true,
      webClient: webClient()
    });

    expect(signer.calls[0].cookieHeader).toBe('sessionid=abc; tt-target-idc=alisg');
  });

  it('reads the cookie from the jar, not from a bundle, so it reflects the live session', async () => {
    // The jar accumulates cookies TikTok set earlier in the same connect; a signature bound to a
    // stale cookie set is rejected.
    const signer = new StubSigner('self');
    let current = 'sessionid=old';
    const client = {
      clientHeaders: { 'User-Agent': 'UA' },
      cookieJar: { getCookieString: async () => current }
    };

    current = 'sessionid=new';
    await asRouteHandler(signer)({ roomId: '73', authenticateWs: true, webClient: client });

    expect(signer.calls[0].cookieHeader).toBe('sessionid=new');
  });

  it('normalises an empty cookie jar to undefined', async () => {
    const signer = new StubSigner('self');

    await asRouteHandler(signer)({ roomId: '73', authenticateWs: true, webClient: webClient('') });

    expect(signer.calls[0].cookieHeader).toBeUndefined();
  });

  it('returns the signer result in the shape the connector destructures', async () => {
    const signer = new StubSigner('self', {
      fetchResult: { cursor: 'c', internalExt: 'e' },
      fetchResultCookieHeader: 'tt=1',
      fetchResultRoomId: '7399'
    });

    const result = await asRouteHandler(signer)({ roomId: '73', webClient: webClient() });

    expect(result).toEqual({
      fetchResult: { cursor: 'c', internalExt: 'e' },
      fetchResultCookieHeader: 'tt=1',
      fetchResultRoomId: '7399'
    });
  });

  it('seeds every cookie pair into the jar — the connector absorbs only the first pair', async () => {
    // The session jar is many cookies (ttwid, msToken, ...), but the
    // connector's processSetCookieHeader takes split(";")[0] — without
    // this split the WS Cookie header would carry one pair plus whatever
    // the page view set. A signer without a jar.setCookie (older shape)
    // is skipped silently.
    const signer = new StubSigner('self', { fetchResult: {}, fetchResultCookieHeader: 'ttwid=1; msToken=2; sessionid=3' });
    const set: string[] = [];
    const client = {
      clientHeaders: { 'User-Agent': 'UA' },
      cookieJar: {
        getCookieString: async () => '',
        setCookie: async (raw: string) => {
          set.push(raw);
        }
      }
    };

    await asRouteHandler(signer)({ roomId: '73', webClient: client });

    expect(set).toEqual(['ttwid=1', 'msToken=2', 'sessionid=3']);
  });

  it('tolerates a jar.setCookie that throws on one malformed pair', async () => {
    // The jar's decodeURIComponent is unguarded: a cookie value with a
    // stray '%' throws. One bad pair must not fail the whole connect.
    const signer = new StubSigner('self', { fetchResult: {}, fetchResultCookieHeader: 'ttwid=1; bad=%zz; msToken=2' });
    const set: string[] = [];
    const client = {
      clientHeaders: { 'User-Agent': 'UA' },
      cookieJar: {
        getCookieString: async () => '',
        setCookie: async (raw: string) => {
          if (raw.includes('%')) throw new Error('decodeURIComponent failed');
          set.push(raw);
        }
      }
    };

    const result = await asRouteHandler(signer)({ roomId: '73', webClient: client });

    expect(set).toEqual(['ttwid=1', 'msToken=2']);
    expect(result.fetchResultCookieHeader).toBe('ttwid=1; bad=%zz; msToken=2');
  });
});
