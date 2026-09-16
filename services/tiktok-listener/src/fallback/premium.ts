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
 * Premium entitlement for the transport fallback (2026-09-16 transport
 * plan, phase 3).
 *
 * The fallback tier is premium-only: a viewer tab per room is ~150MB in
 * the signer pod, so it is reserved for entitled streamers. Entitlement
 * is `users.is_premium` — the materialized flag the payment service keeps
 * current (ADR-0018/0027) and the moderation write-path already trusts for
 * its own gate — reached through overlay ownership: a TikTok room is
/** The checker only reads; the narrow type keeps tests from stubbing a full Pool. */
export interface PremiumQueryPool {
  query(text: string, values?: unknown[]): Promise<{ rows: Array<{ premium: boolean }> }>;
}

export type PremiumCheckerOptions = {
  /** Shared PostgreSQL pool (the service's this.db). */
  db: PremiumQueryPool;
  /** How long a cached answer stays valid. Default 5 minutes. */
  ttlMs?: number;
  logger?: { warn: (msg: string, meta?: Record<string, unknown>) => void };
};

interface CacheEntry {
  premium: boolean;
  expiresAt: number;
}
 

const DEFAULT_TTL_MS = 5 * 60 * 1000;

export class PremiumChecker {
  private readonly db: PremiumQueryPool;
  private readonly ttlMs: number;
  private readonly logger?: PremiumCheckerOptions['logger'];
  private readonly cache = new Map<string, CacheEntry>();

  constructor(options: PremiumCheckerOptions) {
    this.db = options.db;
    this.ttlMs = options.ttlMs ?? DEFAULT_TTL_MS;
    this.logger = options.logger;
  }

  /**
   * Whether any overlay demanding this TikTok room belongs to a premium
   * user. Fail-closed: on a database error the answer is false, so the
   * fallback tier never opens on the strength of a failed check.
   */
  async isPremiumRoom(username: string): Promise<boolean> {
    const cached = this.cache.get(username);
    if (cached && cached.expiresAt > Date.now()) return cached.premium;

    let premium = false;
    try {
      // channel_id holds the platform handle for TikTok sources (see the
      // demand payloads: channel_id is the username this service receives).
      const result = await this.db.query(
        `SELECT EXISTS (
           SELECT 1
           FROM overlay_chat_sources s
           JOIN overlays o ON o.id = s.overlay_id
           JOIN users u ON u.id = o.user_id
           WHERE s.platform = 'tiktok'
             AND s.is_active
             AND LOWER(s.channel_id) = LOWER($1)
             AND u.is_premium
         ) AS premium`,
        [username]
      );
      premium = Boolean(result.rows[0]?.premium);
    } catch (error) {
      this.logger?.warn('premium fallback entitlement check failed; fail-closed', {
        username,
        error: (error as Error).message
      });
      return false;
    }

    this.cache.set(username, { premium, expiresAt: Date.now() + this.ttlMs });
    if (this.cache.size > 5000) {
      // Bounded: room count is small, but the map is per-process and
      // never explicitly invalidated on room removal.
      const oldest = this.cache.keys().next().value;
      if (oldest !== undefined) this.cache.delete(oldest);
    }
    return premium;
  }

  /** Drop the cached answer (e.g. after a promotion attempt went wrong). */
  invalidate(username: string): void {
    this.cache.delete(username);
  }
}
