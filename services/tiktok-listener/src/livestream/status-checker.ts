/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
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
 * TikTok Live Status Checker
 *
 * Provides lightweight pre-connection checks to determine if a TikTok user is live
 * without establishing a full WebSocket connection. Uses the tiktok-live-connector's
 * fetchIsLive() method for efficient status checking.
 */

import { TikTokLiveConnection } from 'tiktok-live-connector';
import { Logger } from '../types/logger.js';

/**
 * Result of a live status check
 */
export interface LiveStatusResult {
  isLive: boolean;
  roomId?: string;
  error?: Error;
}

/**
 * Cached status entry
 */
interface CachedStatus {
  isLive: boolean;
  timestamp: number;
  ttlMs: number; // Dynamic TTL based on result type
}

/**
 * TikTokStatusChecker handles lightweight live status checks for TikTok users
 * UPDATED: Implements dynamic cache TTL based on result type
 */
export class TikTokStatusChecker {
  private logger: Logger;
  private statusCache: Map<string, CachedStatus> = new Map();
  private readonly cacheTTLMs: number; // Kept for backwards compatibility

  // Dynamic TTL based on result type
  private readonly liveTTLMs: number = 5000;     // 5 seconds - stream could end
  private readonly offlineTTLMs: number = 15000; // 15 seconds - less critical
  private readonly errorTTLMs: number = 2000;    // 2 seconds - retry quickly

  /**
   * @param logger Winston logger instance
   * @param cacheTTLMs Cache TTL in milliseconds (default: 10 seconds) - used as fallback
   */
  constructor(logger: Logger, cacheTTLMs: number = 10000) {
    this.logger = logger;
    this.cacheTTLMs = cacheTTLMs;
  }

  /**
   * Check if a TikTok user is currently live
   * Uses lightweight fetchIsLive() API from tiktok-live-connector
   *
   * @param username TikTok username (without @ symbol)
   * @returns LiveStatusResult with isLive flag and optional roomId
   */
  async checkLiveStatus(username: string): Promise<LiveStatusResult> {
    // Check cache first to prevent rapid duplicate checks
    // UPDATED: Use dynamic TTL from cache entry
    const cached = this.statusCache.get(username);
    if (cached && Date.now() - cached.timestamp < cached.ttlMs) {
      this.logger.debug('Using cached live status (dynamic TTL)', {
        username,
        isLive: cached.isLive,
        age_ms: Date.now() - cached.timestamp,
        ttl_ms: cached.ttlMs
      });
      return { isLive: cached.isLive };
    }

    try {
      // Create temporary connection instance (does not connect WebSocket)
      const connection = new TikTokLiveConnection(username, {
        processInitialData: false,
        enableExtendedGiftInfo: false
      });

      // Lightweight live check (no WebSocket connection)
      this.logger.debug('Performing live status check', { username });
      const isLive = await connection.fetchIsLive();

      // Determine dynamic TTL based on result
      // Live: 5s (stream could end), Offline: 15s (less critical)
      const ttlMs = isLive ? this.liveTTLMs : this.offlineTTLMs;

      this.logger.debug('Live status check complete (dynamic TTL)', {
        username,
        isLive,
        cached: false,
        ttl_ms: ttlMs
      });

      // Update cache with dynamic TTL
      this.statusCache.set(username, {
        isLive,
        timestamp: Date.now(),
        ttlMs
      });

      // If live, fetch roomId for connection
      let roomId: string | undefined;
      if (isLive) {
        try {
          // Fetch room ID needed for connection
          // Note: fetchRoomId is a method on the connection instance
          // We'll get this during actual connection instead
          this.logger.debug('User is live, room ID will be fetched during connection', {
            username
          });
        } catch (err) {
          this.logger.warn('Failed to prepare room info', {
            username,
            error: err instanceof Error ? err.message : String(err)
          });
        }
      }

      return { isLive, roomId };
    } catch (error) {
      // tiktok-live-connector's FetchIsLiveError calls super() with no message,
      // so error.message is always "". Extract the sub-errors from error.errors[] for diagnostics.
      const subErrors = (error as any)?.errors;
      const errorDetail = Array.isArray(subErrors) && subErrors.length > 0
        ? subErrors.map((e: unknown) => (e instanceof Error ? e.message : String(e))).filter(Boolean).join(' | ')
        : (error instanceof Error ? error.message : String(error));

      this.logger.error('Failed to check live status', {
        username,
        error: errorDetail || 'unknown (FetchIsLiveError with no sub-errors)',
        error_type: error instanceof Error ? error.constructor.name : typeof error,
        stack: error instanceof Error ? error.stack : undefined
      });

      // Cache error result with short TTL (2 seconds) to retry quickly
      this.statusCache.set(username, {
        isLive: false,
        timestamp: Date.now(),
        ttlMs: this.errorTTLMs
      });

      return {
        isLive: false,
        error: error as Error
      };
    }
  }

  /**
   * Clear cache entry for a username
   * Call this when you know the status has changed
   *
   * @param username TikTok username to clear
   */
  clearCache(username: string): void {
    const deleted = this.statusCache.delete(username);
    if (deleted) {
      this.logger.debug('Cleared status cache', { username });
    }
  }

  /**
   * Clear entire cache
   * Useful for testing or when global refresh is needed
   */
  clearAllCache(): void {
    const size = this.statusCache.size;
    this.statusCache.clear();
    this.logger.debug('Cleared all status cache', { entries_cleared: size });
  }

  /**
   * Get cache statistics
   * UPDATED: Uses dynamic TTL from cache entries
   */
  getCacheStats() {
    const now = Date.now();
    let validEntries = 0;
    let expiredEntries = 0;
    let liveEntries = 0;
    let offlineEntries = 0;
    let errorEntries = 0;

    for (const [_, cached] of this.statusCache) {
      if (now - cached.timestamp < cached.ttlMs) {
        validEntries++;
      } else {
        expiredEntries++;
      }

      // Count by TTL type
      if (cached.ttlMs === this.liveTTLMs) {
        liveEntries++;
      } else if (cached.ttlMs === this.offlineTTLMs) {
        offlineEntries++;
      } else if (cached.ttlMs === this.errorTTLMs) {
        errorEntries++;
      }
    }

    return {
      totalEntries: this.statusCache.size,
      validEntries,
      expiredEntries,
      liveEntries,
      offlineEntries,
      errorEntries,
      dynamicTTL: {
        live_ms: this.liveTTLMs,
        offline_ms: this.offlineTTLMs,
        error_ms: this.errorTTLMs
      }
    };
  }

  /**
   * Clean up expired cache entries
   * Call this periodically to prevent memory leaks
   * UPDATED: Uses dynamic TTL from cache entries
   */
  cleanupExpiredCache(): number {
    const now = Date.now();
    let cleaned = 0;

    for (const [username, cached] of this.statusCache) {
      // Use dynamic TTL from cache entry
      if (now - cached.timestamp >= cached.ttlMs) {
        this.statusCache.delete(username);
        cleaned++;
      }
    }

    if (cleaned > 0) {
      this.logger.debug('Cleaned up expired cache entries (dynamic TTL)', { count: cleaned });
    }

    return cleaned;
  }
}
