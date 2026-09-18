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

/**
 * Pure signer-construction decisions, extracted from index.ts so they are
 * testable without constructing the 2400-line service (same precedent as
 * src/reliability/connection-decisions.ts).
 *
 * The rules these functions encode (PR 2 plan ruling 6, PR 3 extension):
 *
 *  - `euler` and `shadow` construct a SelfSigner whenever a signer URL is
 *    configured — unchanged from pre-PR2 behaviour, because the k8s default
 *    runs euler WITH TIKTOK_SIGNER_URL set and relies on the SelfSigner for
 *    lane pinning and relay promotion.
 *  - `pure-node` constructs a PureNodeSigner instead and NO SelfSigner:
 *    nothing in pure-node mode should drive a per-room page capture on the
 *    connect path.
 *  - Fallback (relay) promotion needs a target-room warm tab: self mode's
 *    pre-sign already ran one, and pure-node mode warms it at promotion
 *    time (PR 3, sign/warm-target-tab.ts) — so promotion is available in
 *    every mode with a signer URL.
 */

import type { SignerMode } from './config.js';
import { SelfSigner } from './self.js';
import { PureNodeSigner } from './pure-node.js';

/** Construction inputs the callers in index.ts already hold. */
export interface SignClientDeps {
  /** TIKTOK_SIGNER_URL; empty means no sign service is configured. */
  signerBaseUrl: string;
  /** TIKTOK_SIGNER_AUTH_TOKEN, when the signer service runs with auth. */
  authToken?: string;
  /** SelfSigner request timeout (TIKTOK_SIGNER_TIMEOUT_MS, capture-tuned). */
  selfTimeoutMs?: number;
  /** PureNodeSigner lease timeout (30 s default — a lease is fast). */
  sessionTimeoutMs?: number;
}

/** Which signers a mode should construct. At most one is ever set per mode. */
export interface SignClients {
  selfSigner?: SelfSigner;
  pureNodeSigner?: PureNodeSigner;
}

/**
 * Pick the signer clients a mode needs. Callers pass the result into
 * installSignConfiguration's `signers` and keep the instances for the
 * lane-pin pre-sign (`pureNodeSigner ?? selfSigner`).
 */
export function pickSignClients(mode: SignerMode, deps: SignClientDeps): SignClients {
  if (!deps.signerBaseUrl) return {};
  switch (mode) {
    case 'euler':
    case 'shadow':
    case 'self':
      return {
        selfSigner: new SelfSigner({
          baseUrl: deps.signerBaseUrl,
          authToken: deps.authToken,
          timeoutMs: deps.selfTimeoutMs
        })
      };
    case 'pure-node':
      return {
        pureNodeSigner: new PureNodeSigner({
          baseUrl: deps.signerBaseUrl,
          authToken: deps.authToken,
          sessionTimeoutMs: deps.sessionTimeoutMs
        })
      };
  }
}

/**
 * Whether premium fallback (relay) promotion can run. The relay attaches to
 * a target-room warm tab: self/euler/shadow modes get one from the
 * connect-time pre-sign, and pure-node mode warms it at promotion time
 * (PR 3, sign/warm-target-tab.ts) — so any mode with a signer URL can
 * promote; without one there is no relay to promote to.
 */
export function fallbackPromotionAvailable(
  signerUrl: string,
  clients: SignClients,
  mode: SignerMode
): boolean {
  return (
    Boolean(signerUrl) &&
    (Boolean(clients.selfSigner) ||
      // PR 3: pure-node promotion warms the target tab at promotion time
      // (sign/warm-target-tab.ts), so the relay is attachable now.
      mode === 'pure-node')
  );
}
