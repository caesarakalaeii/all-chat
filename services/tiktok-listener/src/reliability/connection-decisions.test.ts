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
import { connectionCeilingReached, shouldBackOffReconnect, SILENT_FAILURE_STREAK_THRESHOLD } from './connection-decisions.js';

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
