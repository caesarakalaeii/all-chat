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
 * Which warm session each lease consumer gets (GET /v1/session?consumer=...).
 *
 * TikTok scores its ~15/hour WS-connect flag budget PER SESSION, and every
 * warm room is already a distinct session: its own viewer tab, its own proxy
 * lane, its own profile directory and cookie jar. Serving the first servable
 * room to everyone therefore funnelled the whole listener fleet through one
 * session's budget while the other warm sessions stayed idle — the cause of
 * TikTokPureNodeConnectBudgetExhausted (2026-11-18).
 *
 * Spreading consumers across rooms multiplies the fleet's real ceiling by the
 * number of warm sessions without raising any single session's connect rate,
 * which is the number TikTok actually judges.
 *
 * Assignment is sticky, and that is load-bearing rather than cosmetic: the
 * consumer meters its own connects against the session it was given, so a
 * consumer that hopped sessions would start a fresh budget on each hop while
 * its previous connects still counted against TikTok's window for the old
 * session — exactly the overrun the per-session budget exists to prevent.
 */
export class SessionAssignments {
  /** Consumer key (a listener pod) → the warm room it leases. */
  private readonly assigned = new Map<string, string>();

  /**
   * The rooms to try for this consumer, most preferred first.
   *
   * The caller resolves the list in order and serves the first room that
   * yields a servable lease, so this is a preference, never a restriction:
   * a consumer whose room went offline still falls through to the rest and
   * is re-assigned by `record` to whatever actually served.
   *
   * Preference order: the consumer's current room, then rooms no consumer
   * holds (the spread that wins back the unused budget), then the rooms
   * other consumers hold, least-shared first, so consumers beyond the
   * number of warm sessions pile up evenly rather than all on room one.
   */
  preferenceFor(consumer: string, rooms: readonly string[]): string[] {
    const holders = new Map<string, number>();
    for (const room of this.assigned.values()) {
      holders.set(room, (holders.get(room) ?? 0) + 1);
    }
    const current = this.assigned.get(consumer);
    return [...rooms].sort((a, b) => {
      if (a === current) return -1;
      if (b === current) return 1;
      return (holders.get(a) ?? 0) - (holders.get(b) ?? 0);
    });
  }

  /** Pin the consumer to the room that actually served it. */
  record(consumer: string, room: string): void {
    this.assigned.set(consumer, room);
  }
}
