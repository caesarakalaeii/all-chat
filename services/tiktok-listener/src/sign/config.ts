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
 * Configuration for retiring Euler Stream (issue #698).
 *
 * Euler Stream sits on the critical path of every TikTok connection: `tiktok-live-connector`
 * asks them for a signed WebSocket URL before it can connect at all. Their free tier caps how
 * many rooms we can hold concurrently (twelve simultaneous connection attempts exhausted it on
 * 2026-08-14) and paywalls the gift list, so it is a ceiling on how many TikTok streamers we can
 * serve and on how well we can enrich what they send.
 *
 * There are two independent levers here, and they are deliberately separate flags because they
 * carry very different risk:
 *
 *  - `disableEulerFallbacks` (cheap, safe). Room ID, room info and is-live all have
 *    direct-to-TikTok routes that already ship in the library, reached through composites that
 *    try each source in turn. Turning off the Euler leg of those composites removes free-tier
 *    calls immediately and cannot break anything Euler was uniquely providing, because Euler was
 *    only ever the *last* fallback for them.
 *
 *  - `signerMode` (the real work). The WebSocket signature has no non-Euler route in the
 *    library. Signing it ourselves means owning a reverse-engineered, actively churned
 *    algorithm: when TikTok breaks it, TikTok ingest is down until we fix it, where today it is
 *    Euler's problem and we get the fix for free. So this defaults to `euler` and has to be
 *    turned on deliberately.
 *
 * Neither lever requires forking the connector. See `installer.ts` for why.
 */

/**
 * Who signs the webcast WebSocket URL.
 *
 * - `euler`  — Euler Stream only. The pre-#698 behaviour, and the default.
 * - `shadow` — Euler signs the connection we actually use, but our own signer runs against the
 *              same room in parallel and the outcome is recorded. Nothing user-visible depends
 *              on our signature succeeding. This is how we earn the right to `self`: step 2 of
 *              the issue's sequence asks for a measured success rate before we trust it.
 * - `self`   — We sign. Euler is used only if our signer throws and `selfSignFallback` is on.
 */
export type SignerMode = 'euler' | 'shadow' | 'self';

export const SIGNER_MODES: readonly SignerMode[] = ['euler', 'shadow', 'self'];

export interface SignConfiguration {
  /** Who signs the webcast WebSocket URL. */
  signerMode: SignerMode;

  /**
   * Base URL of our own sign service, when `signerMode` is not `euler`.
   *
   * Empty means "sign in-process": the signer runs inside this service rather than calling out.
   * A URL points at a separately deployed sign service speaking the same small API. Step 1 of
   * the issue's sequence is in-process; step 3 extracts it, and only this value changes.
   */
  signerBaseUrl: string;

  /**
   * Fall back to Euler when our own signer fails, while `signerMode` is `self`.
   *
   * On during rollout — the issue explicitly asks to keep Euler configurable as a fallback
   * rather than cutting over hard. Turning it off is what actually retires the dependency, and
   * should only follow a healthy measured signature success rate.
   */
  selfSignFallback: boolean;

  /**
   * Skip Euler's leg of the room-id / room-info / is-live composites.
   *
   * Independent of `signerMode`, and worth doing on its own merits: it reduces free-tier calls
   * immediately even if the signing work is never finished. Defaults to on for exactly that
   * reason — the direct routes are tried first anyway, so this only removes a fallback we would
   * rather not depend on.
   */
  disableEulerFallbacks: boolean;

  /**
   * Ask TikTok for the room's gift list during connect, so gift events carry `extendedGiftInfo`.
   *
   * This was off because Euler paywalls `fetchAvailableGifts()` behind a Business plan. The
   * library's `fetchRoomGiftsRoute` goes direct to TikTok's `gift/list/` instead — but that
   * request has to be signed, so this only becomes free of Euler once `signerMode` is `self`.
   */
  enableExtendedGiftInfo: boolean;

  /** Euler Stream API key, when Euler is used at all. Empty means the free tier. */
  eulerApiKey: string;
}

function readBool(raw: string | undefined, fallback: boolean): boolean {
  if (raw === undefined || raw === '') return fallback;
  const normalized = raw.trim().toLowerCase();
  if (['1', 'true', 'yes', 'on'].includes(normalized)) return true;
  if (['0', 'false', 'no', 'off'].includes(normalized)) return false;
  return fallback;
}

function readSignerMode(raw: string | undefined): SignerMode {
  const normalized = (raw ?? '').trim().toLowerCase();
  return (SIGNER_MODES as readonly string[]).includes(normalized)
    ? (normalized as SignerMode)
    : 'euler';
}

/**
 * Read the sign configuration from the environment.
 *
 * Unrecognised values fall back to the safe default rather than throwing: a typo in
 * `TIKTOK_SIGNER_MODE` should leave us on Euler, not refuse to start the listener.
 *
 * @param env Environment to read from. Defaults to `process.env`.
 */
export function loadSignConfiguration(env: NodeJS.ProcessEnv = process.env): SignConfiguration {
  const signerMode = readSignerMode(env.TIKTOK_SIGNER_MODE);

  return {
    signerMode,
    signerBaseUrl: (env.TIKTOK_SIGNER_URL ?? '').trim().replace(/\/+$/, ''),
    selfSignFallback: readBool(env.TIKTOK_SELF_SIGN_FALLBACK, true),
    disableEulerFallbacks: readBool(env.TIKTOK_DISABLE_EULER_FALLBACKS, true),
    // Only meaningful once we sign ourselves; leaving it on under `euler` would just reinstate
    // the Business-plan error on every connect.
    enableExtendedGiftInfo: readBool(env.TIKTOK_EXTENDED_GIFT_INFO, signerMode === 'self'),
    eulerApiKey: (env.SIGN_API_KEY ?? '').trim()
  };
}

/**
 * True when Euler can still be reached for the WebSocket signature under this configuration.
 *
 * Used for the "have we actually retired the dependency yet" check, which is not the same
 * question as "are we signing ourselves" — `self` with `selfSignFallback` still needs Euler.
 */
export function eulerStillReachableForSignature(config: SignConfiguration): boolean {
  switch (config.signerMode) {
    case 'euler':
      return true;
    case 'shadow':
      return true;
    case 'self':
      return config.selfSignFallback;
  }
}
