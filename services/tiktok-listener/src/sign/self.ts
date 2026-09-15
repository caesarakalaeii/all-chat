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
 * `WebcastSigner` backed by our own tiktok-signer service — the missing piece
 * of ADR-0052 step 1. Where `EulerSigner` delegates to the library's Euler
 * route, this calls POST /v1/sign on `TIKTOK_SIGNER_URL`, which signs
 * TikTok's `/webcast/im/fetch/` with TikTok's own SDK in a headless browser
 * (services/tiktok-signer) and executes it. The response is the same exchange
 * Euler used to return: protobuf body, Set-Cookie header, optional room ID.
 *
 * Protobuf decoding happens here with the connector's own
 * `deserializeMessage`, so the service stays dependency-free and the decoded
 * `ProtoMessageFetchResult` is produced by the same schemas the connector will
 * hand to `processInitialData`/`connect`.
 */

import { request as undiciRequest } from 'undici';
import { deserializeMessage } from 'tiktok-live-connector';
import { SignatureFailure, type SignRequest, type SignResult, type WebcastSigner } from './signer.js';

interface SignServiceResponse {
  fetchResult?: string;
  fetchResultCookieHeader?: string;
  fetchResultRoomId?: string;
  error?: string;
  message?: string;
}

export interface SelfSignerOptions {
  /** Base URL of the tiktok-signer service, no trailing slash. */
  baseUrl: string;
  /** Bearer token when the signer service runs with SIGNER_AUTH_TOKEN. */
  authToken?: string;
  /** Per-request timeout. Signing involves a browser round trip; keep it generous. */
  timeoutMs?: number;
  /** Injectable fetch for tests. */
  fetchImpl?: typeof undiciRequest;
}

export class SelfSigner implements WebcastSigner {
  readonly name = 'self';

  private readonly baseUrl: string;
  private readonly headers: Record<string, string>;
  private readonly timeoutMs: number;
  private readonly fetchImpl: typeof undiciRequest;

  constructor(options: SelfSignerOptions) {
    this.baseUrl = options.baseUrl.replace(/\/+$/, '');
    this.headers = {
      'Content-Type': 'application/json',
      ...(options.authToken ? { Authorization: `Bearer ${options.authToken}` } : {})
    };
    this.timeoutMs = options.timeoutMs ?? 15_000;
    this.fetchImpl = options.fetchImpl ?? undiciRequest;
  }

  async sign(request: SignRequest): Promise<SignResult> {
    const payload = JSON.stringify({
      roomId: request.roomId,
      cursor: request.cursor,
      cookieHeader: request.cookieHeader
    });

    let bodyJson: SignServiceResponse;
    let status: number;
    try {
      const response = await this.fetchImpl(`${this.baseUrl}/v1/sign`, {
        method: 'POST',
        headers: this.headers,
        body: payload,
        bodyTimeout: this.timeoutMs,
        headersTimeout: this.timeoutMs
      });
      status = response.statusCode;
      const text = await response.body.text();
      bodyJson = text ? (JSON.parse(text) as SignServiceResponse) : {};
    } catch (error) {
      throw new SignatureFailure(
        this.name,
        `sign service unreachable at ${this.baseUrl}: ${(error as Error).message}`,
        error
      );
    }

    if (status !== 200 || typeof bodyJson.fetchResult !== 'string') {
      // The service maps TikTok 429s to 429; keep that reason legible for the
      // metrics classifier rather than folding it into a generic failure.
      const detail = bodyJson.error ?? bodyJson.message ?? `status ${status}`;
      const message =
        status === 429
          ? `sign service rate limited: ${detail}`
          : `sign service rejected the request: ${detail}`;
      const error = new Error(message);
      error.name = status === 429 ? 'SignatureRateLimitError' : 'SignAPIError';
      throw new SignatureFailure(this.name, message, error);
    }

    let fetchResult: unknown;
    try {
      fetchResult = deserializeMessage('ProtoMessageFetchResult', Buffer.from(bodyJson.fetchResult, 'base64'));
    } catch (error) {
      throw new SignatureFailure(
        this.name,
        'signed fetch result did not decode as ProtoMessageFetchResult',
        error
      );
    }

    return {
      fetchResult,
      fetchResultCookieHeader: bodyJson.fetchResultCookieHeader ?? '',
      fetchResultRoomId: bodyJson.fetchResultRoomId || undefined
    };
  }

  /**
   * The second Euler seam (ADR-0052): sign a generic webcast HTTP URL via
   * POST /v1/sign-url. Returns the `response.signedUrl` shape the connector's
   * WebcastHttpClient.request expects. Used for signed gift-list requests.
   */
  async signUrl(url: string, method = 'GET'): Promise<{ response: { signedUrl: string; userAgent?: string } }> {
    let bodyJson: { response?: { signedUrl?: string; userAgent?: string }; error?: string; message?: string };
    let status: number;
    try {
      const response = await this.fetchImpl(`${this.baseUrl}/v1/sign-url`, {
        method: 'POST',
        headers: this.headers,
        body: JSON.stringify({ url, method }),
        bodyTimeout: this.timeoutMs,
        headersTimeout: this.timeoutMs
      });
      status = response.statusCode;
      const text = await response.body.text();
      bodyJson = text ? JSON.parse(text) : {};
    } catch (error) {
      throw new SignatureFailure(
        this.name,
        `sign service unreachable at ${this.baseUrl}: ${(error as Error).message}`,
        error
      );
    }

    if (status !== 200 || typeof bodyJson.response?.signedUrl !== 'string') {
      const detail = bodyJson.error ?? bodyJson.message ?? `status ${status}`;
      throw new SignatureFailure(this.name, `sign service could not sign URL: ${detail}`);
    }

    return {
      response: {
        signedUrl: bodyJson.response.signedUrl,
        userAgent: bodyJson.response.userAgent
      }
    };
  }

}
