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
 * Contract tests between the installer and the *real* `tiktok-live-connector` globals.
 *
 * `installer.test.ts` drives the installer with structural doubles, which is the right way to
 * test its decision logic but proves nothing about whether those decisions land anywhere. The
 * whole approach in #698 rests on an assumption about a third party's internals: that
 * `RouteConfig`, `RoomIdRouteConfig`, `IsLiveRouteConfig` and `SignConfig` are module-level
 * mutable singletons with these exact field names.
 *
 * If a connector upgrade renames a field, freezes an object, or turns one of these into a getter,
 * every double-based test still passes and TikTok signing silently keeps going through Euler --
 * i.e. the failure is invisible in CI and only shows up as a bill or a rate limit. These tests
 * exist so that drift breaks the build instead.
 *
 * They make no network calls: installation is pure mutation of in-process configuration.
 */

import { describe, it, expect, beforeEach } from 'vitest';
import {
  RouteConfig,
  RoomIdRouteConfig,
  IsLiveRouteConfig,
  SignConfig
} from 'tiktok-live-connector';

import { loadSignConfiguration } from './config.js';
import { EulerSigner } from './euler.js';
import { installSignConfiguration, type ConnectorGlobals } from './installer.js';
import type { SignRequest, SignResult, WebcastSigner } from './signer.js';

/** The real globals, assembled the same way `src/index.ts` assembles them. */
function realGlobals(): ConnectorGlobals {
  return {
    routeConfig: RouteConfig as unknown as ConnectorGlobals['routeConfig'],
    roomIdRouteConfig: RoomIdRouteConfig,
    isLiveRouteConfig: IsLiveRouteConfig,
    signConfig: SignConfig as unknown as ConnectorGlobals['signConfig']
  };
}

const silentLogger = { info: () => {}, warn: () => {} };
const silentObserver = { recordSignAttempt: () => {} };

class StubSelfSigner implements WebcastSigner {
  readonly name = 'stub-self';
  async sign(_request: SignRequest): Promise<SignResult> {
    return { fetchResult: {}, fetchResultCookieHeader: '' };
  }
}

describe('installer against the real tiktok-live-connector globals', () => {
  let pristineProvider: typeof RouteConfig.fetchSignedWebSocketFromProvider;

  beforeEach(() => {
    // Restore the library's own defaults between tests; these are process-wide singletons, so
    // leaking state here would make the suite order-dependent.
    pristineProvider = RouteConfig.fetchSignedWebSocketFromProvider;
    RoomIdRouteConfig.skipFetchRoomIdFromEulerRoute = false;
    IsLiveRouteConfig.skipFetchRoomIdFromEulerRoute = false;
  });

  it('exposes the mutable configuration objects the whole approach depends on', () => {
    // Guards the "no fork is required" finding in ADR-0052.
    expect(typeof RouteConfig.fetchSignedWebSocketFromProvider).toBe('function');
    expect(RoomIdRouteConfig).toHaveProperty('skipFetchRoomIdFromEulerRoute');
    expect(IsLiveRouteConfig).toHaveProperty('skipFetchRoomIdFromEulerRoute');
    expect(Object.isFrozen(RouteConfig)).toBe(false);
    expect(Object.isFrozen(RoomIdRouteConfig)).toBe(false);
    expect(Object.isFrozen(IsLiveRouteConfig)).toBe(false);
    expect(Object.isFrozen(SignConfig)).toBe(false);
  });

  it('ships Euler enabled by default, so disabling it is genuinely our change', () => {
    // If the library ever flips these defaults, the step-4 win becomes a no-op and the
    // corresponding assertions below would pass for the wrong reason.
    expect(RoomIdRouteConfig.skipFetchRoomIdFromEulerRoute).toBe(false);
    expect(IsLiveRouteConfig.skipFetchRoomIdFromEulerRoute).toBe(false);
  });

  it('takes the Euler leg out of both composites under the default configuration', () => {
    const config = loadSignConfiguration({} as NodeJS.ProcessEnv);
    expect(config.disableEulerFallbacks).toBe(true);

    const report = installSignConfiguration(
      realGlobals(),
      config,
      { euler: new EulerSigner(pristineProvider as never) },
      silentObserver,
      silentLogger
    );

    // The cheap, independent win of #698 step 4, asserted on the real objects.
    expect(RoomIdRouteConfig.skipFetchRoomIdFromEulerRoute).toBe(true);
    expect(IsLiveRouteConfig.skipFetchRoomIdFromEulerRoute).toBe(true);
    expect(report.eulerFallbacksDisabled).toBe(true);
  });

  it('leaves the real signing route untouched in euler mode', () => {
    // Euler mode is the known-good baseline; our code must not be on its critical path.
    installSignConfiguration(
      realGlobals(),
      { ...loadSignConfiguration({} as NodeJS.ProcessEnv), signerMode: 'euler' },
      { euler: new EulerSigner(pristineProvider as never) },
      silentObserver,
      silentLogger
    );

    expect(RouteConfig.fetchSignedWebSocketFromProvider).toBe(pristineProvider);
  });

  it('replaces the real signing route once we sign for ourselves', () => {
    installSignConfiguration(
      realGlobals(),
      {
        ...loadSignConfiguration({} as NodeJS.ProcessEnv),
        signerMode: 'self',
        selfSignFallback: false
      },
      { euler: new EulerSigner(pristineProvider as never), self: new StubSelfSigner() },
      silentObserver,
      silentLogger
    );

    // Proves the assignment actually takes on the real registry rather than a copy of it.
    expect(RouteConfig.fetchSignedWebSocketFromProvider).not.toBe(pristineProvider);

    // Put it back: this is a process-wide singleton shared with every other suite.
    RouteConfig.fetchSignedWebSocketFromProvider = pristineProvider;
  });

  it('repoints the real SignConfig.basePath and clears the memoised client', () => {
    const originalBasePath = SignConfig.basePath;
    // The connector memoises its Euler client here; a stale cache would silently ignore the
    // repoint, which is the failure mode that makes the flag look broken.
    (SignConfig as { cachedInstance?: unknown }).cachedInstance = { stale: true };

    installSignConfiguration(
      realGlobals(),
      {
        ...loadSignConfiguration({} as NodeJS.ProcessEnv),
        signerBaseUrl: 'https://sign.example.internal'
      },
      { euler: new EulerSigner(pristineProvider as never) },
      silentObserver,
      silentLogger
    );

    expect(SignConfig.basePath).toBe('https://sign.example.internal');
    expect((SignConfig as { cachedInstance?: unknown }).cachedInstance).toBeUndefined();

    SignConfig.basePath = originalBasePath;
  });
});
