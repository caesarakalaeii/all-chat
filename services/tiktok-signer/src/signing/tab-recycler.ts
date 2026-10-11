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
 * Warm-tab recycler. Pinned warm tabs never idle-evict, and a live room's
 * page keeps growing while it stays open: on 2026-10-11 three 25-hour-old
 * renderers held ~1.2 GB each with media already blocked, and the pod was
 * heading for its second OOM kill in two days. Each OOM restart drops every
 * warm tab, which is how the signer went cold for eight hours on 2026-10-10.
 *
 * Every CHECK_INTERVAL the recycler takes the oldest pinned tab past the age
 * limit and re-captures its room on a fresh page (ViewerPool.recycleTab),
 * closing the old page only once the new one serves. Budget discipline
 * matches the curator's: one capture per tick, live rooms only (an offline
 * room's tab is the only session it has, so it is kept), a failed room
 * waits a full age window before its next try, and a 403 benches the
 * recycler for an hour.
 */

import type { ViewerPool } from './viewer.js';
import { warmRoomIsLive } from '../api.js';
import type { IsLive } from './warm-curator.js';

export interface TabRecyclerOptions {
  /** The warm set (the pool's pinned rooms), read on every tick. */
  pinnedRooms: ReadonlySet<string>;
  viewer: ViewerPool;
  /** Rooms with relay subscribers are skipped: closing the page ends their stream. */
  relay?: { hasSubscribers(username: string): boolean };
  /** Tabs older than this are recycled. */
  maxAgeMs: number;
  logger?: { info: (msg: string, meta?: Record<string, unknown>) => void; warn: (msg: string, meta?: Record<string, unknown>) => void };
  /** Injectable liveness probe; defaults to the production warmRoomIsLive. */
  isLive?: IsLive;
}

const CHECK_INTERVAL_MS = 30 * 60_000;
const BENCH_ON_403_MS = 60 * 60_000;

export class TabRecycler {
  private readonly options: TabRecyclerOptions;
  private readonly isLive: IsLive;
  private readonly retryAt = new Map<string, number>();
  private benchedUntil = 0;
  private timer: NodeJS.Timeout | undefined;

  constructor(options: TabRecyclerOptions) {
    this.options = options;
    this.isLive = options.isLive ?? ((username) => warmRoomIsLive(username));
  }

  /** One scheduled tick: recycle at most one tab, the oldest eligible. */
  async tick(): Promise<void> {
    const now = Date.now();
    if (now < this.benchedUntil) return;
    const { viewer, relay, maxAgeMs, logger } = this.options;

    const stale: Array<{ room: string; openedAt: number }> = [];
    for (const room of this.options.pinnedRooms) {
      const openedAt = viewer.tabOpenedAt(room);
      if (openedAt === undefined || now - openedAt < maxAgeMs) continue;
      if ((this.retryAt.get(room) ?? 0) > now) continue;
      if (relay?.hasSubscribers(room)) continue;
      stale.push({ room, openedAt });
    }
    stale.sort((a, b) => a.openedAt - b.openedAt);

    for (const { room, openedAt } of stale) {
      // Fail closed: an unknown liveness costs nothing to skip, while a
      // capture of an offline room burns a full capture timeout.
      if (!(await this.isLive(room).catch(() => false))) continue;
      try {
        await viewer.recycleTab(room);
        this.retryAt.delete(room);
        logger?.info('warm tab recycled', { room, age_h: Math.round((now - openedAt) / 3_600_000) });
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        this.retryAt.set(room, now + maxAgeMs);
        if (/\b403\b|Forbidden|blocked/i.test(message)) {
          this.benchedUntil = now + BENCH_ON_403_MS;
        }
        logger?.warn('warm tab recycle failed; old tab kept', { room, error: message });
      }
      return;
    }
  }

  start(): void {
    if (this.timer) return;
    this.timer = setInterval(() => {
      void this.tick().catch(() => undefined);
    }, CHECK_INTERVAL_MS);
  }

  stop(): void {
    clearInterval(this.timer);
    this.timer = undefined;
  }
}
