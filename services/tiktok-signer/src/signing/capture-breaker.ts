/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
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
 * Per-room circuit breaker for viewer captures.
 *
 * The 2026-09-16 lab verdict: capture failures are room-correlated, not
 * lane-correlated — the gated rooms (asahiicc, tv_whiteshark) failed on every
 * lane while the same lanes captured healthy rooms first-attempt. So a room
 * whose captures keep failing is not evidence the pool is sick, and the
 * request path must not keep paying for it: one capture burns up to
 * maxLaneAttempts x per-attempt budget (~60s+ each), and the listener retries
 * connect every backoff cycle, so a gated period costs 3x60s per request per
 * room, forever.
 *
 * The breaker is keyed by room (username), not by lane: a lane-level breaker
 * would bench healthy lanes on the strength of gated rooms' failures.
 *
 * After `threshold` consecutive failures a room's captures are refused for
 * `cooldownMs`; the request answers fast 502 instead of burning pool time.
 * A success resets the room's streak. Cooldowns escalate by `factor` per
 * consecutive trip so a persistently gated room backs off harder instead of
 * re-tripping at the floor forever.
 */

export interface CaptureBreakerOptions {
  /** Consecutive failures before the room is refused. Default 3. */
  threshold?: number;
  /** Base refusal window per trip, milliseconds. Default 5 minutes. */
  cooldownMs?: number;
  /** Cooldown multiplier per consecutive trip. Default 2. */
  factor?: number;
  /** Cap on the escalated cooldown, milliseconds. Default 60 minutes. */
  maxCooldownMs?: number;
}

interface RoomState {
  consecutiveFailures: number;
  /** Until when captures for this room are refused. 0 = open. */
  refusedUntil: number;
  /** Completed cooldown trips without an intervening success. */
  trips: number;
}

export class CaptureBreaker {
  private readonly threshold: number;
  private readonly cooldownMs: number;
  private readonly factor: number;
  private readonly maxCooldownMs: number;
  private readonly rooms = new Map<string, RoomState>();

  constructor(options: CaptureBreakerOptions = {}) {
    this.threshold = options.threshold ?? 3;
    this.cooldownMs = options.cooldownMs ?? 5 * 60_000;
    this.factor = options.factor ?? 2;
    this.maxCooldownMs = options.maxCooldownMs ?? 60 * 60_000;
  }

  /**
   * Whether this room's captures are currently refused. When true the caller
   * must fast-fail the request instead of touching the viewer pool.
   */
  isRefused(username: string, now: number = Date.now()): boolean {
    const state = this.rooms.get(username);
    return state !== undefined && now < state.refusedUntil;
  }

  /** Milliseconds remaining in the current refusal window, 0 when open. */
  refusedForMs(username: string, now: number = Date.now()): number {
    const state = this.rooms.get(username);
    return state === undefined ? 0 : Math.max(0, state.refusedUntil - now);
  }

  /** Record one successful capture: clears the room's streak and trips. */
  recordSuccess(username: string): void {
    this.rooms.delete(username);
  }

  /**
   * Record one failed capture. Returns true when this failure tripped the
   * breaker (the caller should log/metric the trip distinctly).
   */
  recordFailure(username: string, now: number = Date.now()): boolean {
    let state = this.rooms.get(username);
    if (!state) {
      state = { consecutiveFailures: 0, refusedUntil: 0, trips: 0 };
      this.rooms.set(username, state);
    }
    state.consecutiveFailures++;
    if (state.consecutiveFailures < this.threshold) return false;

    state.trips++;
    // Escalate from the base cooldown by trip count, capped: a room that has
    // been gated for an hour should not come back at the 5-minute floor.
    const cooldown = Math.min(
      this.cooldownMs * Math.pow(this.factor, state.trips - 1),
      this.maxCooldownMs
    );
    state.refusedUntil = now + cooldown;
    return true;
  }

  /** Rooms currently inside a refusal window (for metrics/logging). */
  refusedCount(now: number = Date.now()): number {
    let count = 0;
    for (const state of this.rooms.values()) {
      if (now < state.refusedUntil) count++;
    }
    return count;
  }

  /** Drop all state for a room (shutdown / test hygiene). */
  forget(username: string): void {
    this.rooms.delete(username);
  }
}
