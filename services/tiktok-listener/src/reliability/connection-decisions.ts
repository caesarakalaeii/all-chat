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
 * Pure decisions behind index.ts's connection throttling, extracted so they can
 * be tested directly (index.ts is a 1800-line entry point that cannot be
 * constructed in a unit test). Each function answers one question the incident
 * mitigations depend on.
 */
import { BUDGET_REFUSAL_SIGNATURE, SignatureFailure } from '../sign/signer.js';

/**
 * Whether a pod at `liveConnectionCount` active+connecting WebSocket
 * connections may open another one.
 *
 * Inclusive on purpose: at the ceiling there is no budget for a further
 * connection. The caller must release its leadership lease and re-park the
 * stream — a leased-but-unconnectable stream is deaf everywhere, because no
 * other pod may claim it (the exact failure of the 2026-09-14 incident).
 */
export function connectionCeilingReached(liveConnectionCount: number, ceiling: number): boolean {
  return liveConnectionCount >= ceiling;
}

/**
 * Whether a heartbeat-forced disconnect should take growing error backoff
 * instead of the fast reconnect path.
 *
 * A silent-failure streak above SILENT_FAILURE_STREAK_THRESHOLD means
 * reconnecting did not fix the silence, so the reconnect loop must back off
 * instead of hammering Euler every ~2 minutes per channel (the churn that kept
 * the 2026-09-14 incident wedged for a day). A streak of 1 — the first forced
 * disconnect, possibly a one-off — still reconnects quickly; a genuinely-ended
 * stream never grows a streak because delivering messages heals it.
 */
export const SILENT_FAILURE_STREAK_THRESHOLD = 1;

export function shouldBackOffReconnect(silentFailureStreak: number): boolean {
  return silentFailureStreak > SILENT_FAILURE_STREAK_THRESHOLD;
}

/**
 * The signature of the 2026-09-16 WS connect flap: TikTok answers the upgrade
 * request with a plain HTTP 200 instead of switching protocols, which the ws
 * library surfaces as "Unexpected server response: 200". Lab-measured on
 * diamondslay: a flap, not a wall — attempt 3 connected and held a sustained
 * stream — so it must be retried immediately instead of entering the
 * escalating error backoff (which is what kept rooms parked through it).
 */
export const WS_FLAP_SIGNATURE = 'Unexpected server response: 200';

/** Fast retries a flap gets before the error is allowed into normal backoff. */
export const WS_FLAP_MAX_FAST_RETRIES = 3;

/** Delay between flap retries. Lab flap cleared on attempt 3; 1s is enough. */
export const WS_FLAP_RETRY_DELAY_MS = 1000;

/**
 * Wait before flap retry `attempt` (the attempt number that just flapped,
 * 1-based). Prod flaps 2026-09-17 cleared on attempt 4-8: they are
 * session-scoring artifacts that ease with spacing, so past the first two
 * quick retries the wait triples instead of burning handshakes every second.
 */
export function nextFlapRetryDelayMs(attempt: number, baseMs: number): number {
  return attempt <= 2 ? baseMs : baseMs * 3;
}

/**
 * Negative cache for the WS pre-sign lane check. Under direct egress the
 * signer answers with no proxy lane, so the pre-sign cannot pin anything
 * and its ~50s capture round-trip is pure latency on every connect attempt.
 * A successful no-lane answer parks the pre-sign for the TTL; the first
 * answer that does carry a lane clears it, so pinning resumes the moment
 * lanes return without a restart. Sign *failures* never touch the cache:
 * that is an availability problem, not a lane state, and hiding it would
 * delay diagnosing a signer outage.
 */
export const NO_LANE_CACHE_TTL_MS = 10 * 60_000;

export class NoLaneCache {
  private until = 0;

  skipActive(now: number = Date.now()): boolean {
    return now < this.until;
  }

  markNoLane(now: number = Date.now()): void {
    this.until = now + NO_LANE_CACHE_TTL_MS;
  }

  markLane(): void {
    this.until = 0;
  }
}

export function isWsFlapError(error: unknown): boolean {
  return error instanceof Error && error.message.includes(WS_FLAP_SIGNATURE);
}

/**
 * Our own hourly sign budget refusing a connect (PureNodeSigner WS-connect
 * or lease budget). Distinct from an external rate limit: the refusal is
 * self-protection and carries the time until the rolling window slides,
 * so the connect loop parks the room until then instead of entering the
 * escalating error backoff — which re-signs, gets refused again, and spins
 * the failure counter that fired the 2026-09-23 budget-exhausted alert flap.
 */
export function isBudgetRefusalError(error: unknown): boolean {
  const cause = unwrapConnectorError(error);
  return cause instanceof Error
    && cause.message.includes(BUDGET_REFUSAL_SIGNATURE);
}

/** Time until the signer's window slides, 0 when the error does not say. */
export function budgetRefusalRetryAfterMs(error: unknown): number {
  const cause = unwrapConnectorError(error);
  return isBudgetRefusalError(error) && cause instanceof SignatureFailure
    ? cause.retryAfterMs ?? 0
    : 0;
}

/**
 * tiktok-live-connector's handleError emits its 'error' events as
 * `{ info, exception }`, not as the raw Error — the emitter.on('error')
 * handler in index.ts receives that envelope, while the connect() catch
 * receives the raw throw. Unwrap so both paths classify identically.
 */
function unwrapConnectorError(error: unknown): unknown {
  const candidate = (error as { exception?: unknown } | null | undefined)?.exception;
  return candidate !== undefined ? candidate : error;
}
