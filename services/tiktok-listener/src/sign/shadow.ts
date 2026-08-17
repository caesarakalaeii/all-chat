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

import {
  classifySignatureFailure,
  SignatureFailure,
  type SignRequest,
  type SignResult,
  type WebcastSigner
} from './signer.js';

/**
 * Where a signature attempt ended up. Bounded, because these become metric label values.
 */
export type SignOutcome = 'success' | 'failure';

/** One recorded signature attempt. */
export interface SignAttempt {
  /** `name` of the signer that made the attempt. */
  signer: string;
  outcome: SignOutcome;
  /** Result of `classifySignatureFailure`, or `undefined` on success. */
  reason?: string;
  /** Wall-clock duration of the attempt. */
  durationMs: number;
  /** Whether this attempt's result was the one actually used to connect. */
  loadBearing: boolean;
}

/**
 * Sink for signature attempts. Implemented by the Prometheus recorder in production and by a
 * plain array in tests, which is the whole reason it is an interface.
 */
export interface SignObserver {
  recordSignAttempt(attempt: SignAttempt): void;
}

/**
 * Runs a candidate signer alongside the signer we actually trust, and reports on the difference.
 *
 * This exists because of step 2 of #698's sequence: *"measure our own signature success rate
 * against Euler's for the same rooms, in parallel, before trusting it."* The trade-off the issue
 * names — that after cutover a TikTok change takes our ingest down rather than Euler's — is only
 * worth accepting if we know our success rate first, and the only honest way to know is to run
 * both against the same rooms at the same time.
 *
 * Semantics:
 *
 *  - The **primary** signer's result is what gets returned. If it throws, this throws. Enabling
 *    shadow mode can never change connection behaviour, which is what makes it safe to leave on
 *    in production while we gather data.
 *  - The **candidate** runs concurrently and its outcome is recorded and then discarded. A
 *    candidate failure is never propagated, never logged as an error, and never retried.
 *
 * The candidate is started before the primary is awaited so the two overlap in time. That
 * matters: TikTok's behaviour varies by room and by minute, and a candidate run seconds later
 * against a different room state is not a comparison.
 *
 * Cost note: shadow mode doubles the initial `/im/fetch/` traffic per connect, and while the
 * primary is Euler it consumes one free-tier sign per connect exactly as before — the candidate
 * does not touch Euler. It does not double the WebSocket connections; only the handshake.
 */
export class ShadowSigner implements WebcastSigner {
  readonly name: string;

  private readonly primary: WebcastSigner;
  private readonly candidate: WebcastSigner;
  private readonly observer: SignObserver;

  /**
   * @param primary   Signer whose result is returned and whose failure propagates.
   * @param candidate Signer being evaluated. Its outcome is recorded and discarded.
   * @param observer  Where attempts are reported.
   */
  constructor(primary: WebcastSigner, candidate: WebcastSigner, observer: SignObserver) {
    this.primary = primary;
    this.candidate = candidate;
    this.observer = observer;
    this.name = `shadow(${primary.name}->${candidate.name})`;
  }

  async sign(request: SignRequest): Promise<SignResult> {
    // Start both before awaiting either, so the comparison is genuinely concurrent.
    const candidateRun = this.observe(this.candidate, request, false);
    const primaryRun = this.observe(this.primary, request, true);

    // Settle the candidate regardless of how the primary went, so its outcome is always recorded
    // — including when the primary fails, which is precisely the case where we most want to know
    // whether our own signer would have coped.
    const [primaryOutcome] = await Promise.all([
      primaryRun.catch((error: unknown) => ({ error })),
      candidateRun
    ]);

    if (primaryOutcome && typeof primaryOutcome === 'object' && 'error' in primaryOutcome) {
      throw primaryOutcome.error;
    }

    return primaryOutcome as SignResult;
  }

  /**
   * Run one signer, record the attempt, and re-raise only if it was load bearing.
   *
   * @param signer      Signer to invoke.
   * @param request     Request to sign.
   * @param loadBearing Whether this result is the one the caller will use.
   */
  private async observe(
    signer: WebcastSigner,
    request: SignRequest,
    loadBearing: boolean
  ): Promise<SignResult | undefined> {
    const startedAt = Date.now();

    try {
      const result = await signer.sign(request);
      this.observer.recordSignAttempt({
        signer: signer.name,
        outcome: 'success',
        durationMs: Date.now() - startedAt,
        loadBearing
      });
      return result;
    } catch (error) {
      this.observer.recordSignAttempt({
        signer: signer.name,
        outcome: 'failure',
        reason: classifySignatureFailure(error),
        durationMs: Date.now() - startedAt,
        loadBearing
      });
      if (loadBearing) throw error;
      return undefined;
    }
  }
}

/**
 * Tries `preferred` and falls back to `fallback` when it fails.
 *
 * This is the rollout shape #698 asks for: *"keep Euler configurable as a fallback during
 * rollout rather than cutting over hard."* With our own signer preferred and Euler behind it, a
 * TikTok change that breaks our signature degrades to the old cost profile instead of taking
 * TikTok ingest down — which is exactly the trade the issue wants to be able to make
 * deliberately, and to be able to walk back.
 *
 * Both attempts are recorded, so the observed failure rate of the preferred signer stays visible
 * even while the fallback is quietly rescuing every connection. Without that, a fully broken
 * self-signer looks identical to a healthy one from the outside, and we would only discover it
 * when we finally turned the fallback off.
 */
export class FallbackSigner implements WebcastSigner {
  readonly name: string;

  private readonly preferred: WebcastSigner;
  private readonly fallback: WebcastSigner;
  private readonly observer: SignObserver;

  constructor(preferred: WebcastSigner, fallback: WebcastSigner, observer: SignObserver) {
    this.preferred = preferred;
    this.fallback = fallback;
    this.observer = observer;
    this.name = `fallback(${preferred.name}->${fallback.name})`;
  }

  async sign(request: SignRequest): Promise<SignResult> {
    const preferredStartedAt = Date.now();

    try {
      const result = await this.preferred.sign(request);
      this.observer.recordSignAttempt({
        signer: this.preferred.name,
        outcome: 'success',
        durationMs: Date.now() - preferredStartedAt,
        loadBearing: true
      });
      return result;
    } catch (preferredError) {
      this.observer.recordSignAttempt({
        signer: this.preferred.name,
        outcome: 'failure',
        reason: classifySignatureFailure(preferredError),
        durationMs: Date.now() - preferredStartedAt,
        loadBearing: true
      });

      const fallbackStartedAt = Date.now();
      try {
        const result = await this.fallback.sign(request);
        this.observer.recordSignAttempt({
          signer: this.fallback.name,
          outcome: 'success',
          durationMs: Date.now() - fallbackStartedAt,
          loadBearing: true
        });
        return result;
      } catch (fallbackError) {
        this.observer.recordSignAttempt({
          signer: this.fallback.name,
          outcome: 'failure',
          reason: classifySignatureFailure(fallbackError),
          durationMs: Date.now() - fallbackStartedAt,
          loadBearing: true
        });

        // Surface the fallback's failure as the cause but name both, so an on-call reading the
        // log does not conclude Euler is broken when the real story is that our signer failed
        // first and Euler then hit its rate limit.
        throw new SignatureFailure(
          this.name,
          `both signers failed: ${this.preferred.name} (${classifySignatureFailure(preferredError)}), ` +
            `${this.fallback.name} (${classifySignatureFailure(fallbackError)})`,
          fallbackError
        );
      }
    }
  }
}
