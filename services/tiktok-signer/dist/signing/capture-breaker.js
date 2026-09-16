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
export class CaptureBreaker {
    threshold;
    cooldownMs;
    factor;
    maxCooldownMs;
    rooms = new Map();
    constructor(options = {}) {
        this.threshold = options.threshold ?? 3;
        this.cooldownMs = options.cooldownMs ?? 5 * 60_000;
        this.factor = options.factor ?? 2;
        this.maxCooldownMs = options.maxCooldownMs ?? 60 * 60_000;
    }
    /**
     * Whether this room's captures are currently refused. When true the caller
     * must fast-fail the request instead of touching the viewer pool.
     */
    isRefused(username, now = Date.now()) {
        const state = this.rooms.get(username);
        return state !== undefined && now < state.refusedUntil;
    }
    /** Milliseconds remaining in the current refusal window, 0 when open. */
    refusedForMs(username, now = Date.now()) {
        const state = this.rooms.get(username);
        return state === undefined ? 0 : Math.max(0, state.refusedUntil - now);
    }
    /** Record one successful capture: clears the room's streak and trips. */
    recordSuccess(username) {
        this.rooms.delete(username);
    }
    /**
     * Record one failed capture. Returns true when this failure tripped the
     * breaker (the caller should log/metric the trip distinctly).
     */
    recordFailure(username, now = Date.now()) {
        let state = this.rooms.get(username);
        if (!state) {
            state = { consecutiveFailures: 0, refusedUntil: 0, trips: 0 };
            this.rooms.set(username, state);
        }
        state.consecutiveFailures++;
        if (state.consecutiveFailures < this.threshold)
            return false;
        state.trips++;
        // Escalate from the base cooldown by trip count, capped: a room that has
        // been gated for an hour should not come back at the 5-minute floor.
        const cooldown = Math.min(this.cooldownMs * Math.pow(this.factor, state.trips - 1), this.maxCooldownMs);
        state.refusedUntil = now + cooldown;
        return true;
    }
    /** Rooms currently inside a refusal window (for metrics/logging). */
    refusedCount(now = Date.now()) {
        let count = 0;
        for (const state of this.rooms.values()) {
            if (now < state.refusedUntil)
                count++;
        }
        return count;
    }
    /** Drop all state for a room (shutdown / test hygiene). */
    forget(username) {
        this.rooms.delete(username);
    }
}
//# sourceMappingURL=capture-breaker.js.map