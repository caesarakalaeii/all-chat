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

import { describe, expect, it } from 'vitest';

import {
  eulerStillReachableForSignature,
  loadSignConfiguration,
  type SignConfiguration
} from './config.js';

describe('loadSignConfiguration', () => {
  it('defaults to Euler for signatures, since self-signing is the risky half of #698', () => {
    const config = loadSignConfiguration({});

    expect(config.signerMode).toBe('euler');
    expect(config.selfSignFallback).toBe(true);
  });

  it('defaults to skipping the Euler fallback on the composites', () => {
    // Step 4 of the issue's sequence is called out as "worth doing on its own merits even if we
    // never finish the signing work". The composites try TikTok directly first, so Euler was
    // only ever the last resort here and dropping it cannot lose us a capability.
    expect(loadSignConfiguration({}).disableEulerFallbacks).toBe(true);
  });

  it('leaves extended gift info off while Euler still signs', () => {
    // fetchAvailableGifts() through Euler returns "This endpoint requires a Business plan."
    // Turning enrichment on before we can sign our own gift/list/ request would reinstate that
    // error on every single connect.
    expect(loadSignConfiguration({}).enableExtendedGiftInfo).toBe(false);
    expect(loadSignConfiguration({ TIKTOK_SIGNER_MODE: 'shadow' }).enableExtendedGiftInfo).toBe(
      false
    );
  });

  it('turns extended gift info on by default once we sign for ourselves', () => {
    // Gift enrichment is one of the two acceptance criteria the issue names for dropping Euler,
    // and self-signing is what unblocks it, so it should not need a second flag.
    expect(loadSignConfiguration({ TIKTOK_SIGNER_MODE: 'self' }).enableExtendedGiftInfo).toBe(true);
  });

  it('still allows extended gift info to be forced off under self mode', () => {
    expect(
      loadSignConfiguration({ TIKTOK_SIGNER_MODE: 'self', TIKTOK_EXTENDED_GIFT_INFO: 'false' })
        .enableExtendedGiftInfo
    ).toBe(false);
  });

  it('accepts each documented signer mode', () => {
    expect(loadSignConfiguration({ TIKTOK_SIGNER_MODE: 'euler' }).signerMode).toBe('euler');
    expect(loadSignConfiguration({ TIKTOK_SIGNER_MODE: 'shadow' }).signerMode).toBe('shadow');
    expect(loadSignConfiguration({ TIKTOK_SIGNER_MODE: 'self' }).signerMode).toBe('self');
  });

  it('normalises case and surrounding whitespace in the signer mode', () => {
    expect(loadSignConfiguration({ TIKTOK_SIGNER_MODE: '  SELF ' }).signerMode).toBe('self');
  });

  it('falls back to Euler rather than throwing on an unrecognised signer mode', () => {
    // A typo in the deployment manifest should leave TikTok ingest working on the known-good
    // path, not refuse to start the listener.
    expect(loadSignConfiguration({ TIKTOK_SIGNER_MODE: 'slef' }).signerMode).toBe('euler');
    expect(loadSignConfiguration({ TIKTOK_SIGNER_MODE: '' }).signerMode).toBe('euler');
  });

  it('reads booleans in the spellings deployment manifests actually use', () => {
    for (const truthy of ['1', 'true', 'TRUE', 'yes', 'on']) {
      expect(loadSignConfiguration({ TIKTOK_DISABLE_EULER_FALLBACKS: truthy })
        .disableEulerFallbacks).toBe(true);
    }
    for (const falsy of ['0', 'false', 'FALSE', 'no', 'off']) {
      expect(loadSignConfiguration({ TIKTOK_DISABLE_EULER_FALLBACKS: falsy })
        .disableEulerFallbacks).toBe(false);
    }
  });

  it('keeps the default when a boolean is set to something uninterpretable', () => {
    expect(
      loadSignConfiguration({ TIKTOK_DISABLE_EULER_FALLBACKS: 'maybe' }).disableEulerFallbacks
    ).toBe(true);
  });

  it('strips trailing slashes from the signer URL so basePath joins cleanly', () => {
    expect(loadSignConfiguration({ TIKTOK_SIGNER_URL: 'https://sign.internal/' }).signerBaseUrl)
      .toBe('https://sign.internal');
    expect(loadSignConfiguration({ TIKTOK_SIGNER_URL: 'https://sign.internal///' }).signerBaseUrl)
      .toBe('https://sign.internal');
  });

  it('treats an absent signer URL as "sign in-process"', () => {
    expect(loadSignConfiguration({}).signerBaseUrl).toBe('');
  });
});

describe('eulerStillReachableForSignature', () => {
  const base: SignConfiguration = {
    signerMode: 'euler',
    signerBaseUrl: '',
    selfSignFallback: true,
    disableEulerFallbacks: true,
    enableExtendedGiftInfo: false,
    eulerApiKey: ''
  };

  it('reports Euler reachable under plain euler mode', () => {
    expect(eulerStillReachableForSignature(base)).toBe(true);
  });

  it('reports Euler reachable under shadow mode, since Euler still serves the connection', () => {
    expect(eulerStillReachableForSignature({ ...base, signerMode: 'shadow' })).toBe(true);
  });

  it('reports Euler reachable under self mode while the fallback is on', () => {
    // This is the distinction the helper exists for: "we sign" is not the same as "we no longer
    // need Euler". During rollout self mode still calls Euler every time our signer fails.
    expect(
      eulerStillReachableForSignature({ ...base, signerMode: 'self', selfSignFallback: true })
    ).toBe(true);
  });

  it('reports Euler unreachable only once self mode drops the fallback', () => {
    expect(
      eulerStillReachableForSignature({ ...base, signerMode: 'self', selfSignFallback: false })
    ).toBe(false);
  });
});
