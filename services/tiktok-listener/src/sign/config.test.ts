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
    // The issue calls its step 4 "worth doing on its own merits even if we
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
    expect(loadSignConfiguration({ TIKTOK_SIGNER_MODE: 'pure-node' }).signerMode).toBe('pure-node');
  });

  it('pure-node retires Euler for signatures and keeps gift info off', () => {
    const config = loadSignConfiguration({ TIKTOK_SIGNER_MODE: 'pure-node' });
    expect(eulerStillReachableForSignature(config)).toBe(false);
    // The gift seam's URL signing keys on the self signer; pure-node
    // declares gifts-off rather than re-instating the Business-plan error.
    expect(config.enableExtendedGiftInfo).toBe(false);
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

  it('defaults the per-pod WS-connect rate and burst', () => {
    expect(loadSignConfiguration({}).maxWsConnectsPerHour).toBe(18);
    expect(loadSignConfiguration({}).wsConnectBurst).toBe(12);
  });

  it('reads the per-pod WS-connect rate and burst from the manifest', () => {
    // Every pod dials on the one session the signer keeps warm, so the
    // session carries replicas x these values and the manifest must be
    // able to size them without a code change.
    const config = loadSignConfiguration({
      TIKTOK_MAX_WS_CONNECTS_PER_HOUR: '6',
      TIKTOK_WS_CONNECT_BURST: '4'
    });
    expect(config.maxWsConnectsPerHour).toBe(6);
    expect(config.wsConnectBurst).toBe(4);
  });

  it('reads the lease consumer key so the signer assigns this pod its own session', () => {
    // The pod name, from the downward API. Without it every pod leases the
    // same warm session and the fleet shares one session's connect budget
    // (TikTokPureNodeConnectBudgetExhausted, 2026-11-18).
    expect(loadSignConfiguration({ TIKTOK_LEASE_CONSUMER: 'tiktok-listener-77d-whf8z' }).leaseConsumer)
      .toBe('tiktok-listener-77d-whf8z');
  });

  it('defaults the lease consumer key to empty so an unset pod still leases', () => {
    expect(loadSignConfiguration({}).leaseConsumer).toBe('');
  });

  it('keeps the default WS-connect budget when the value is not a positive integer', () => {
    // Same reasoning as the signer mode: a typo in the manifest should leave
    // the listener on the known-good budget, not start it with a budget of
    // zero (which refuses every room) or NaN (which refuses every room too).
    for (const bad of ['', 'six', '0', '-1', '2.5']) {
      expect(
        loadSignConfiguration({ TIKTOK_MAX_WS_CONNECTS_PER_HOUR: bad }).maxWsConnectsPerHour,
        bad
      ).toBe(18);
      expect(loadSignConfiguration({ TIKTOK_WS_CONNECT_BURST: bad }).wsConnectBurst, bad).toBe(12);
    }
  });
});

describe('eulerStillReachableForSignature', () => {
  const base: SignConfiguration = {
    signerMode: 'euler',
    signerBaseUrl: '',
    selfSignFallback: true,
    disableEulerFallbacks: true,
    enableExtendedGiftInfo: false,
    eulerApiKey: '',
    maxWsConnectsPerHour: 18,
    wsConnectBurst: 12,
    leaseConsumer: ''
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
