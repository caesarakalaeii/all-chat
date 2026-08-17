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

import type { SignConfiguration } from './config.js';
import { eulerStillReachableForSignature } from './config.js';
import { FallbackSigner, ShadowSigner, type SignObserver } from './shadow.js';
import {
  classifySignatureFailure,
  SignatureFailure,
  type SignRequest,
  type SignResult,
  type WebcastSigner
} from './signer.js';

/**
 * The parts of `tiktok-live-connector`'s global configuration we mutate.
 *
 * Declared structurally rather than imported as concrete types so the installer can be driven by
 * a plain object in tests. The connector's real `RouteConfig` / `RoomIdRouteConfig` /
 * `IsLiveRouteConfig` are module-level mutable singletons, so a test double is simply another
 * object with the same fields — no module mocking needed.
 */
export interface ConnectorGlobals {
  /**
   * The connector's route registry. Its own docs invite exactly this:
   * "Call sites should read handlers from here rather than importing the route functions
   * directly, so downstream consumers can swap implementations."
   */
  routeConfig: {
    fetchSignedWebSocketFromProvider: (args: never) => Promise<unknown>;
  };
  /** Composite-route switches controlling whether Euler is consulted as a last resort. */
  roomIdRouteConfig: { skipFetchRoomIdFromEulerRoute: boolean };
  isLiveRouteConfig: { skipFetchRoomIdFromEulerRoute: boolean };
  /** Euler client configuration. `basePath` is what the connector's whitelist check compares. */
  signConfig: { basePath?: string; apiKey?: string; cachedInstance?: unknown };
}

/** Minimal logger surface, so this module does not depend on the winston instance's shape. */
export interface InstallerLogger {
  info(message: string, meta?: Record<string, unknown>): void;
  warn(message: string, meta?: Record<string, unknown>): void;
}

/** What `installSignConfiguration` actually did, for logging and for assertions in tests. */
export interface InstallReport {
  /** `name` of the signer now installed, or `undefined` when Euler's own route was left alone. */
  signerName?: string;
  /** Whether the Euler leg of the room-id / is-live composites was switched off. */
  eulerFallbacksDisabled: boolean;
  /** Whether Euler can still be reached for a signature under this configuration. */
  eulerReachableForSignature: boolean;
  /** Human-readable notes, one per decision taken. */
  notes: string[];
}

/**
 * Wrap a `WebcastSigner` so it can stand in for `fetchSignedWebSocketFromProvider`.
 *
 * The connector passes the route a bag containing `webClient`, `apiClient`, `roomId`, `cursor`
 * and the bundled auth options, and expects `{ fetchResult, fetchResultCookieHeader,
 * fetchResultRoomId }` back. This narrows that bag to the four things a signer actually needs.
 *
 * The cookie header is read from the client's jar rather than from the auth bundle because that
 * is where the connector itself reads it: the jar accumulates cookies TikTok set on earlier
 * requests in the same connect, and a signature bound to a stale cookie set is rejected.
 */
export function asRouteHandler(signer: WebcastSigner) {
  return async (args: {
    roomId: string;
    cursor?: string;
    authenticateWs?: boolean;
    webClient: {
      clientHeaders: Record<string, string>;
      cookieJar: { getCookieString(): Promise<string> };
    };
  }): Promise<SignResult> => {
    const cookieHeader = (await args.webClient.cookieJar.getCookieString()) || undefined;

    return signer.sign({
      roomId: args.roomId,
      cursor: args.cursor,
      userAgent: args.webClient.clientHeaders['User-Agent'],
      // Only bind the signature to the session when the caller actually asked for an
      // authenticated socket. Forwarding a session cookie that nothing requested is how the
      // credential-exposure problem in #698 happens in the first place.
      cookieHeader: args.authenticateWs ? cookieHeader : undefined
    });
  };
}

/**
 * Apply a `SignConfiguration` to the connector's global state.
 *
 * The central finding of #698's investigation is that none of this needs a fork. The connector
 * exposes three module-level mutable objects that between them cover every Euler call site:
 *
 *  - `RouteConfig.fetchSignedWebSocketFromProvider` — the signature itself, and the only one
 *    with no direct-to-TikTok alternative.
 *  - `RoomIdRouteConfig.skipFetchRoomIdFromEulerRoute` and the same field on
 *    `IsLiveRouteConfig` — the Euler leg of the two composites.
 *  - `SignConfig.basePath` — where the Euler SDK points, for when we run a sign *service* rather
 *    than signing in-process.
 *
 * Called once at startup, before any `TikTokLiveConnection` is constructed. It is idempotent and
 * safe to call again, which matters because the connector caches its Euler client on
 * `SignConfig.cachedInstance` — a `basePath` change after the cache is warm would otherwise be
 * silently ignored, so this clears it.
 *
 * @param globals    The connector's mutable configuration objects.
 * @param config     Desired configuration, normally from `loadSignConfiguration()`.
 * @param signers    Signer implementations to compose. `euler` is required; `self` is required
 *                   whenever `config.signerMode` is not `euler`.
 * @param observer   Sink for signature attempt outcomes.
 * @param logger     Where decisions are reported.
 * @returns A report of what was changed.
 */
export function installSignConfiguration(
  globals: ConnectorGlobals,
  config: SignConfiguration,
  signers: { euler: WebcastSigner; self?: WebcastSigner },
  observer: SignObserver,
  logger: InstallerLogger
): InstallReport {
  const notes: string[] = [];

  // Step 4 of #698's sequence, and the reason it is listed as worth doing on its own merits:
  // these two flags remove Euler calls immediately and cannot regress anything, because Euler
  // was only ever consulted after the direct HTML and API routes had both already failed.
  if (config.disableEulerFallbacks) {
    globals.roomIdRouteConfig.skipFetchRoomIdFromEulerRoute = true;
    globals.isLiveRouteConfig.skipFetchRoomIdFromEulerRoute = true;
    notes.push('room-id and is-live composites will not fall back to Euler');
  } else {
    globals.roomIdRouteConfig.skipFetchRoomIdFromEulerRoute = false;
    globals.isLiveRouteConfig.skipFetchRoomIdFromEulerRoute = false;
    notes.push('room-id and is-live composites retain their Euler fallback');
  }

  if (config.eulerApiKey) {
    globals.signConfig.apiKey = config.eulerApiKey;
  }

  // Point the Euler SDK at our own sign service, if we run one. The connector compares this
  // host against WHITELIST_AUTHENTICATED_SESSION_ID_HOST for authenticated sockets, so it has to
  // be the real externally visible URL of the service, not an internal alias.
  if (config.signerBaseUrl) {
    globals.signConfig.basePath = config.signerBaseUrl;
    notes.push(`sign SDK base path repointed to ${config.signerBaseUrl}`);
  }

  // Drop the memoised client so a changed basePath or apiKey is actually picked up. Without
  // this, a config change applied after the first connection is a no-op and looks like the flag
  // does not work.
  globals.signConfig.cachedInstance = undefined;

  let installed: WebcastSigner | undefined;

  switch (config.signerMode) {
    case 'euler':
      // Leave the connector's own route in place rather than wrapping Euler in our own adapter.
      // Wrapping would buy nothing and would put our code on the critical path of the mode whose
      // entire purpose is to be the unchanged, known-good baseline.
      notes.push('WebSocket signatures come from Euler Stream (unchanged)');
      break;

    case 'shadow':
      if (!signers.self) {
        notes.push('shadow mode requested without a self signer; staying on Euler');
        logger.warn('Shadow sign mode requested but no self signer supplied; staying on Euler');
        break;
      }
      installed = new ShadowSigner(signers.euler, signers.self, observer);
      notes.push('Euler signs; our signer runs in parallel and is measured but not used');
      break;

    case 'self':
      if (!signers.self) {
        notes.push('self mode requested without a self signer; staying on Euler');
        logger.warn('Self sign mode requested but no self signer supplied; staying on Euler');
        break;
      }
      installed = config.selfSignFallback
        ? new FallbackSigner(signers.self, signers.euler, observer)
        : new MeasuredSigner(signers.self, observer);
      notes.push(
        config.selfSignFallback
          ? 'we sign, falling back to Euler on failure'
          : 'we sign, with no Euler fallback'
      );
      break;
  }

  if (installed) {
    globals.routeConfig.fetchSignedWebSocketFromProvider = asRouteHandler(
      installed
    ) as unknown as (args: never) => Promise<unknown>;
  }

  const report: InstallReport = {
    signerName: installed?.name,
    eulerFallbacksDisabled: config.disableEulerFallbacks,
    eulerReachableForSignature:
      // A mode that asked for a self signer but did not get one has silently stayed on Euler,
      // so report the effective state rather than the requested one.
      config.signerMode !== 'euler' && !signers.self
        ? true
        : eulerStillReachableForSignature(config),
    notes
  };

  logger.info('TikTok sign configuration installed', {
    signer_mode: config.signerMode,
    signer: report.signerName ?? 'euler (library default)',
    euler_fallbacks_disabled: report.eulerFallbacksDisabled,
    euler_reachable_for_signature: report.eulerReachableForSignature,
    notes: report.notes
  });

  return report;
}

/**
 * Records outcomes for a single signer without adding any fallback behaviour.
 *
 * Used for `self` mode with the fallback turned off — the end state #698 is aiming at. The
 * measurement has to survive the removal of the fallback, otherwise the moment we finally retire
 * Euler is also the moment we go blind to our own signature failure rate.
 */
class MeasuredSigner implements WebcastSigner {
  readonly name: string;

  private readonly inner: WebcastSigner;
  private readonly observer: SignObserver;

  constructor(inner: WebcastSigner, observer: SignObserver) {
    this.inner = inner;
    this.observer = observer;
    this.name = inner.name;
  }

  async sign(request: SignRequest): Promise<SignResult> {
    const startedAt = Date.now();
    try {
      const result = await this.inner.sign(request);
      this.observer.recordSignAttempt({
        signer: this.name,
        outcome: 'success',
        durationMs: Date.now() - startedAt,
        loadBearing: true
      });
      return result;
    } catch (error) {
      this.observer.recordSignAttempt({
        signer: this.name,
        outcome: 'failure',
        reason: classifySignatureFailure(error),
        durationMs: Date.now() - startedAt,
        loadBearing: true
      });
      throw error instanceof SignatureFailure
        ? error
        : new SignatureFailure(this.name, 'signature failed with no fallback configured', error);
    }
  }
}
