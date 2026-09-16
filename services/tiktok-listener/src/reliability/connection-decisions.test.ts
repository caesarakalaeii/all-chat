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
import {
  connectionCeilingReached,
  isWsFlapError,
  shouldBackOffReconnect,
  SILENT_FAILURE_STREAK_THRESHOLD,
  WS_FLAP_MAX_FAST_RETRIES,
  WS_FLAP_RETRY_DELAY_MS
} from './connection-decisions.js';

describe('connectionCeilingReached', () => {
  // The ceiling check is what stops a pod from opening more Euler-proxied
  // WebSockets than the free tier serves (ADR-0052). Off-by-one here
  // reintroduces the 2026-09-14 incident: too low strands demanded streams,
  // too high deafens every channel on the pod.

  it('blocks a further connection when the pod is at the ceiling', () => {
    expect(connectionCeilingReached(20, 20)).toBe(true);
  });

  it('blocks a further connection when the pod is over the ceiling', () => {
    expect(connectionCeilingReached(21, 20)).toBe(true);
  });

  it('allows a connection when the pod is below the ceiling', () => {
    expect(connectionCeilingReached(19, 20)).toBe(false);
  });

  it('treats zero live connections as under any positive ceiling', () => {
    expect(connectionCeilingReached(0, 20)).toBe(false);
  });

  it('blocks the very first connection when the ceiling is zero', () => {
    expect(connectionCeilingReached(0, 0)).toBe(true);
  });
});

describe('shouldBackOffReconnect', () => {
  // The threshold decides whether a heartbeat-forced disconnect takes growing
  // error backoff or the fast reconnect path. Flipping the comparison to >=
  // would put every normally-ended stream on error backoff; dropping the
  // threshold to 0 would never damp the deaf-room reconnect churn the
  // 2026-09-14 incident was made of.

  it('reconnects quickly on the first silent failure (streak 1)', () => {
    expect(shouldBackOffReconnect(1)).toBe(false);
  });

  it('reconnects quickly when the stream was healthy (streak 0)', () => {
    expect(shouldBackOffReconnect(0)).toBe(false);
  });

  it('backs off once reconnecting did not fix the silence (streak 2)', () => {
    expect(shouldBackOffReconnect(2)).toBe(true);
  });

  it('backs off on long streaks', () => {
    expect(shouldBackOffReconnect(7)).toBe(true);
  });

  it('is exclusive of the threshold, so streak and threshold stay decoupled', () => {
    // If someone tunes the threshold constant, the boundary must follow it —
    // pinning a literal 1 in the comparisons above would hide a change here.
    expect(SILENT_FAILURE_STREAK_THRESHOLD).toBe(1);
    expect(shouldBackOffReconnect(SILENT_FAILURE_STREAK_THRESHOLD)).toBe(false);
    expect(shouldBackOffReconnect(SILENT_FAILURE_STREAK_THRESHOLD + 1)).toBe(true);
  });
});

describe('isWsFlapError', () => {
  // The classifier decides which connect failures get the immediate-retry
  // loop in connectToStream. Too narrow and a real flap (ws wording changes
  // slightly) goes into escalating backoff, parking a healthy room for
  // minutes; too wide and genuine connect failures (room offline, signer
  // down) get hammered with fast retries.

  it('classifies the ws library flap message', () => {
    expect(isWsFlapError(new Error('Unexpected server response: 200'))).toBe(true);
  });

  it('classifies the flap when the message carries trailing context', () => {
    // The ws library appends the response body to the message; the signature
    // must match on the prefix, not the exact string.
    expect(isWsFlapError(new Error('Unexpected server response: 200 {"code":10000}'))).toBe(true);
  });

  it('does not classify a non-200 upgrade rejection', () => {
    expect(isWsFlapError(new Error('Unexpected server response: 403'))).toBe(false);
  });

  it('does not classify unrelated connect errors', () => {
    expect(isWsFlapError(new Error('ETIMEDOUT'))).toBe(false);
    expect(isWsFlapError(new Error('fetchRoomId failed'))).toBe(false);
  });

  it('does not classify non-Error values', () => {
    expect(isWsFlapError('Unexpected server response: 200')).toBe(false);
    expect(isWsFlapError(undefined)).toBe(false);
  });

  it('bounds the fast-retry budget so a wall eventually backs off', () => {
    // Lab flap cleared on attempt 3; the budget must cover that with slack
    // but stay small enough that a persistent refusal reaches the poller's
    // normal backoff within seconds, not minutes.
    expect(WS_FLAP_MAX_FAST_RETRIES).toBeGreaterThanOrEqual(3);
    expect(WS_FLAP_MAX_FAST_RETRIES).toBeLessThanOrEqual(5);
    expect(WS_FLAP_RETRY_DELAY_MS).toBeGreaterThanOrEqual(1000);
    expect(WS_FLAP_RETRY_DELAY_MS).toBeLessThanOrEqual(2000);
  });
});
