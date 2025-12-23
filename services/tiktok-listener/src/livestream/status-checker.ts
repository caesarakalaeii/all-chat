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
}

/**
 * TikTokStatusChecker handles lightweight live status checks for TikTok users
 */
export class TikTokStatusChecker {
  private logger: Logger;
  private statusCache: Map<string, CachedStatus> = new Map();
  private readonly cacheTTLMs: number;

  /**
   * @param logger Winston logger instance
   * @param cacheTTLMs Cache TTL in milliseconds (default: 10 seconds)
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
    const cached = this.statusCache.get(username);
    if (cached && Date.now() - cached.timestamp < this.cacheTTLMs) {
      this.logger.debug('Using cached live status', {
        username,
        isLive: cached.isLive,
        age_ms: Date.now() - cached.timestamp
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

      this.logger.debug('Live status check complete', {
        username,
        isLive,
        cached: false
      });

      // Update cache
      this.statusCache.set(username, {
        isLive,
        timestamp: Date.now()
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
      this.logger.error('Failed to check live status', {
        username,
        error: error instanceof Error ? error.message : String(error),
        stack: error instanceof Error ? error.stack : undefined
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
   */
  getCacheStats() {
    const now = Date.now();
    let validEntries = 0;
    let expiredEntries = 0;

    for (const [_, cached] of this.statusCache) {
      if (now - cached.timestamp < this.cacheTTLMs) {
        validEntries++;
      } else {
        expiredEntries++;
      }
    }

    return {
      totalEntries: this.statusCache.size,
      validEntries,
      expiredEntries,
      cacheTTLMs: this.cacheTTLMs
    };
  }

  /**
   * Clean up expired cache entries
   * Call this periodically to prevent memory leaks
   */
  cleanupExpiredCache(): number {
    const now = Date.now();
    let cleaned = 0;

    for (const [username, cached] of this.statusCache) {
      if (now - cached.timestamp >= this.cacheTTLMs) {
        this.statusCache.delete(username);
        cleaned++;
      }
    }

    if (cleaned > 0) {
      this.logger.debug('Cleaned up expired cache entries', { count: cleaned });
    }

    return cleaned;
  }
}
