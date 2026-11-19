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
import { SessionAssignments } from './session-assignments.js';

/**
 * The spread itself is covered through the HTTP surface in api.test.ts.
 * What is only reachable here is the staleness of the holder counts that
 * drive it, because that needs a clock.
 */
describe('SessionAssignments', () => {
  const ROOMS = ['rooma', 'roomb'];

  it('stops counting a consumer that has not leased within the keepalive', () => {
    // Consumer keys are pod names and the signer is one long-lived
    // replica, so every listener rollout leaves its dead pods behind.
    // Live pods keep their rooms (stickiness wins), but a dead pod still
    // counted as a holder makes "rooms no consumer holds" progressively
    // untrue, and a NEW pod is then spread against ghosts — degrading
    // back toward the one-session funnelling this class exists to stop.
    let now = 0;
    const assignments = new SessionAssignments(() => now);

    assignments.record('dead-pod', 'rooma');
    now += 61 * 60 * 1000;

    // rooma is held only by a pod that stopped leasing an hour ago, so a
    // fresh consumer must still see it as free and take it.
    expect(assignments.preferenceFor('new-pod', ROOMS)[0]).toBe('rooma');
  });

  it('keeps counting a consumer that is still leasing', () => {
    // A live pod leases up to 8 times an hour, so it refreshes well
    // inside the keepalive and must never be mistaken for a dead one.
    let now = 0;
    const assignments = new SessionAssignments(() => now);

    assignments.record('live-pod', 'rooma');
    now += 50 * 60 * 1000;
    assignments.record('live-pod', 'rooma');
    now += 50 * 60 * 1000;

    expect(assignments.preferenceFor('new-pod', ROOMS)[0]).toBe('roomb');
  });
});
