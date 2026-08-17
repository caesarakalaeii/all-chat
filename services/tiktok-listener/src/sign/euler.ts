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

import { SignatureFailure, type SignRequest, type SignResult, type WebcastSigner } from './signer.js';

/**
 * The connector's own Euler route, reduced to the `WebcastSigner` shape.
 *
 * Structural, not the imported type, so tests can drive this with a stub. The real route needs
 * `webClient` and `apiClient` handles that only exist inside a live `TikTokLiveConnection`.
 */
export type EulerSignedWebSocketRoute = (args: {
  webClient: unknown;
  apiClient: unknown;
  roomId: string;
  cursor?: string;
  useMobile?: false;
}) => Promise<SignResult>;

/**
 * Wraps `fetchSignedWebSocketFromEulerRoute` so Euler can be composed with our own signer.
 *
 * Needed because `ShadowSigner` and `FallbackSigner` both take two `WebcastSigner`s, and one of
 * them is always Euler. In plain `euler` mode nothing constructs this — the installer leaves the
 * library's route untouched — so this only ever runs on a path where we have deliberately taken
 * responsibility for signing.
 *
 * The awkward part is that the Euler route wants live `webClient` / `apiClient` handles, which
 * belong to the connection, not to a long-lived signer. They arrive through
 * `bindConnectionContext`, called by the route adapter for each connect.
 */
export class EulerSigner implements WebcastSigner {
  readonly name = 'euler';

  private readonly route: EulerSignedWebSocketRoute;
  private context?: { webClient: unknown; apiClient: unknown };

  constructor(route: EulerSignedWebSocketRoute) {
    this.route = route;
  }

  /**
   * Supply the per-connection handles the Euler SDK route needs.
   *
   * @param webClient The connection's `WebcastHttpClient`.
   * @param apiClient The connection's Euler API client.
   */
  bindConnectionContext(webClient: unknown, apiClient: unknown): void {
    this.context = { webClient, apiClient };
  }

  async sign(request: SignRequest): Promise<SignResult> {
    if (!this.context) {
      throw new SignatureFailure(
        this.name,
        'no connection context bound; bindConnectionContext must be called before sign'
      );
    }

    try {
      return await this.route({
        webClient: this.context.webClient,
        apiClient: this.context.apiClient,
        roomId: request.roomId,
        cursor: request.cursor,
        useMobile: false
      });
    } catch (error) {
      // Re-raise as-is when it is already one of the connector's typed sign errors, because
      // `classifySignatureFailure` keys off their `name` and wrapping would hide it. Anything
      // else gets named so the log says which signer failed.
      const name = (error as { name?: unknown })?.name;
      if (
        typeof name === 'string' &&
        ['SignAPIError', 'SignatureRateLimitError', 'PremiumFeatureError', 'SignatureMissingTokensError'].includes(
          name
        )
      ) {
        throw error;
      }
      throw new SignatureFailure(this.name, 'Euler sign request failed', error);
    }
  }
}
