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

import { describe, expect, it } from 'vitest';
import { CaptureBreaker } from './capture-breaker.js';

// A deterministic clock: the breaker is pure timing logic, and real sleeps
// would make these tests flaky and slow.
function clock(start: number) {
  let now = start;
  return {
    now: () => now,
    advance: (ms: number) => {
      now += ms;
    }
  };
}

describe('CaptureBreaker', () => {
  it('refuses captures only after the failure threshold', () => {
    const t = clock(0);
    const breaker = new CaptureBreaker({ threshold: 3, cooldownMs: 60_000 });
    expect(breaker.recordFailure('room', t.now())).toBe(false);
    expect(breaker.isRefused('room', t.now())).toBe(false);
    breaker.recordFailure('room', t.now());
    expect(breaker.recordFailure('room', t.now())).toBe(true);
    expect(breaker.isRefused('room', t.now())).toBe(true);
  });

  it('answers a fast refusal without touching the pool while open', () => {
    const t = clock(0);
    const breaker = new CaptureBreaker({ threshold: 1, cooldownMs: 60_000 });
    breaker.recordFailure('room', t.now());
    // The request path checks isRefused before captureRoom; while open it
    // must stay refused for the whole window...
    t.advance(59_999);
    expect(breaker.isRefused('room', t.now())).toBe(true);
    // ...and clear the instant the window ends.
    t.advance(1);
    expect(breaker.isRefused('room', t.now())).toBe(false);
    expect(breaker.refusedForMs('room', t.now())).toBe(0);
  });

  it('resets a room entirely on success', () => {
    const t = clock(0);
    const breaker = new CaptureBreaker({ threshold: 2, cooldownMs: 60_000 });
    breaker.recordFailure('room', t.now());
    breaker.recordSuccess('room');
    // The streak restarts from zero: two more failures are needed to trip.
    expect(breaker.recordFailure('room', t.now())).toBe(false);
    expect(breaker.isRefused('room', t.now())).toBe(false);
    expect(breaker.recordFailure('room', t.now())).toBe(true);
  });

  it('escalates the cooldown on consecutive trips and caps it', () => {
    const t = clock(0);
    const breaker = new CaptureBreaker({
      threshold: 1,
      cooldownMs: 60_000,
      factor: 2,
      maxCooldownMs: 300_000
    });
    // Trip 1: 60s window.
    breaker.recordFailure('room', t.now());
    expect(breaker.refusedForMs('room', t.now())).toBe(60_000);
    t.advance(60_000);

    // Trip 2 without an intervening success: 120s.
    breaker.recordFailure('room', t.now());
    expect(breaker.refusedForMs('room', t.now())).toBe(120_000);
    t.advance(120_000);

    // Trip 3: would be 240s, uncapped; trip 4 would be 480s but caps at 300s.
    breaker.recordFailure('room', t.now());
    expect(breaker.refusedForMs('room', t.now())).toBe(240_000);
    t.advance(240_000);
    breaker.recordFailure('room', t.now());
    expect(breaker.refusedForMs('room', t.now())).toBe(300_000);
  });

  it('keeps rooms independent: one gated room never benches a healthy one', () => {
    // The 2026-09-16 lab verdict is that failures are room-correlated. The
    // breaker is keyed per room on purpose; if this test fails the breaker
    // has grown shared state and healthy rooms starve behind gated ones.
    const t = clock(0);
    const breaker = new CaptureBreaker({ threshold: 2, cooldownMs: 60_000 });
    breaker.recordFailure('gated', t.now());
    breaker.recordFailure('gated', t.now());
    expect(breaker.isRefused('gated', t.now())).toBe(true);
    expect(breaker.isRefused('healthy', t.now())).toBe(false);
    expect(breaker.recordFailure('healthy', t.now())).toBe(false);
    expect(breaker.refusedCount(t.now())).toBe(1);
  });

  it('does not trip on failures spread wider than the threshold implies', () => {
    // Interleaved success keeps the streak from reaching the threshold —
    // the 2026-09-16 webshare pool verdict showed captures succeed
    // first-attempt through healthy lanes between gated-room failures.
    const t = clock(0);
    const breaker = new CaptureBreaker({ threshold: 3, cooldownMs: 60_000 });
    breaker.recordFailure('room', t.now());
    breaker.recordFailure('room', t.now());
    breaker.recordSuccess('room');
    breaker.recordFailure('room', t.now());
    breaker.recordFailure('room', t.now());
    expect(breaker.isRefused('room', t.now())).toBe(false);
  });

  it('forgets a room entirely', () => {
    const t = clock(0);
    const breaker = new CaptureBreaker({ threshold: 1, cooldownMs: 60_000 });
    breaker.recordFailure('room', t.now());
    breaker.forget('room');
    expect(breaker.isRefused('room', t.now())).toBe(false);
  });
});
