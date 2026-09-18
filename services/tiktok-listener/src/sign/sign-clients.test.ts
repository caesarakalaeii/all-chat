/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published
 * by the Free Software Foundation, either version 3 of the License, or
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
import { fallbackPromotionAvailable, pickSignClients } from './sign-clients.js';

/**
 * The mode-gated signer-construction rules (PR 2 plan ruling 6) and the
 * fallback predicate that encodes them. The euler+URL and shadow+URL rows
 * are regression guards for PRE-EXISTING behaviour: the k8s default runs
 * euler WITH TIKTOK_SIGNER_URL set and relies on the SelfSigner for lane
 * pinning and relay promotion — a predicate or construction slip that
 * keys on mode === 'self' would silently break those deployments.
 */

const URL_DEPS = { signerBaseUrl: 'http://signer:8092', authToken: 't' };

describe('pickSignClients', () => {
  it('pure-node constructs a PureNodeSigner and NO SelfSigner', () => {
    const clients = pickSignClients('pure-node', URL_DEPS);
    expect(clients.selfSigner).toBeUndefined();
    expect(clients.pureNodeSigner).toBeDefined();
    expect(clients.pureNodeSigner!.name).toBe('pure-node');
  });

  it('euler with a signer URL keeps the SelfSigner (k8s default behaviour)', () => {
    const clients = pickSignClients('euler', URL_DEPS);
    expect(clients.selfSigner).toBeDefined();
    expect(clients.pureNodeSigner).toBeUndefined();
  });

  it('shadow and self with a signer URL construct the SelfSigner', () => {
    for (const mode of ['shadow', 'self'] as const) {
      const clients = pickSignClients(mode, URL_DEPS);
      expect(clients.selfSigner, mode).toBeDefined();
      expect(clients.pureNodeSigner, mode).toBeUndefined();
    }
  });

  it('no signer URL constructs nothing, in every mode', () => {
    for (const mode of ['euler', 'shadow', 'self', 'pure-node'] as const) {
      const clients = pickSignClients(mode, { signerBaseUrl: '', authToken: 't' });
      expect(clients.selfSigner, mode).toBeUndefined();
      expect(clients.pureNodeSigner, mode).toBeUndefined();
    }
  });
});

describe('fallbackPromotionAvailable', () => {
  it('pure-node mode is available: PR 3 warms the target tab at promotion time', () => {
    // The relay (GET /v1/stream/:username) attaches to a pool tab; pure-node
    // leases a warm-CLASSIC-room session that never creates one for the
    // target room, so promotion warms it first (sign/warm-target-tab.ts).
    const clients = pickSignClients('pure-node', URL_DEPS);
    expect(fallbackPromotionAvailable('http://signer:8092', clients, 'pure-node')).toBe(true);
  });

  it('self mode with a signer URL is available (pre-existing behaviour)', () => {
    const clients = pickSignClients('self', URL_DEPS);
    expect(fallbackPromotionAvailable('http://signer:8092', clients, 'self')).toBe(true);
  });

  it('euler with a signer URL is available (the k8s default keeps relay promotion)', () => {
    const clients = pickSignClients('euler', URL_DEPS);
    expect(fallbackPromotionAvailable('http://signer:8092', clients, 'euler')).toBe(true);
  });

  it('shadow with a signer URL is available', () => {
    const clients = pickSignClients('shadow', URL_DEPS);
    expect(fallbackPromotionAvailable('http://signer:8092', clients, 'shadow')).toBe(true);
  });

  it('no signer URL is unavailable in every mode', () => {
    for (const mode of ['euler', 'shadow', 'self', 'pure-node'] as const) {
      const clients = pickSignClients(mode, { signerBaseUrl: '' });
      expect(fallbackPromotionAvailable('', clients, mode), mode).toBe(false);
    }
  });
});
