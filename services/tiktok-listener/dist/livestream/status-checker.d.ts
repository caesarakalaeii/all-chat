/**
 * TikTok Live Status Checker
 *
 * Provides lightweight pre-connection checks to determine if a TikTok user is live
 * without establishing a full WebSocket connection. Uses the tiktok-live-connector's
 * fetchIsLive() method for efficient status checking.
 */
import winston from 'winston';
/**
 * Result of a live status check
 */
export interface LiveStatusResult {
    isLive: boolean;
    roomId?: string;
    error?: Error;
}
/**
 * TikTokStatusChecker handles lightweight live status checks for TikTok users
 */
export declare class TikTokStatusChecker {
    private logger;
    private statusCache;
    private readonly cacheTTLMs;
    /**
     * @param logger Winston logger instance
     * @param cacheTTLMs Cache TTL in milliseconds (default: 10 seconds)
     */
    constructor(logger: winston.Logger, cacheTTLMs?: number);
    /**
     * Check if a TikTok user is currently live
     * Uses lightweight fetchIsLive() API from tiktok-live-connector
     *
     * @param username TikTok username (without @ symbol)
     * @returns LiveStatusResult with isLive flag and optional roomId
     */
    checkLiveStatus(username: string): Promise<LiveStatusResult>;
    /**
     * Clear cache entry for a username
     * Call this when you know the status has changed
     *
     * @param username TikTok username to clear
     */
    clearCache(username: string): void;
    /**
     * Clear entire cache
     * Useful for testing or when global refresh is needed
     */
    clearAllCache(): void;
    /**
     * Get cache statistics
     */
    getCacheStats(): {
        totalEntries: number;
        validEntries: number;
        expiredEntries: number;
        cacheTTLMs: number;
    };
    /**
     * Clean up expired cache entries
     * Call this periodically to prevent memory leaks
     */
    cleanupExpiredCache(): number;
}
//# sourceMappingURL=status-checker.d.ts.map