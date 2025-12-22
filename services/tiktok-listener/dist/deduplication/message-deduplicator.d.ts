/**
 * Message Deduplicator
 *
 * Prevents duplicate messages from being published to Redis, especially
 * important when connections restart and TikTok may replay recent messages.
 *
 * Uses TikTok's native message ID (msgId) from the message's common property
 * for accurate deduplication, along with time-based expiration to prevent
 * memory leaks.
 */
import winston from 'winston';
/**
 * Configuration for the deduplicator
 */
export interface DeduplicatorConfig {
    /** How long to remember messages (default: 5 minutes) */
    ttlMs: number;
    /** How often to clean up expired entries (default: 1 minute) */
    cleanupIntervalMs: number;
    /** Maximum cache size before forced cleanup (default: 10000) */
    maxCacheSize: number;
}
/**
 * MessageDeduplicator tracks recently seen messages to prevent duplicates
 */
export declare class MessageDeduplicator {
    private logger;
    private seenMessages;
    private cleanupTimer?;
    private duplicateCount;
    private processedCount;
    private readonly TTL_MS;
    private readonly CLEANUP_INTERVAL_MS;
    private readonly MAX_CACHE_SIZE;
    /**
     * @param logger Winston logger instance
     * @param config Optional deduplicator configuration
     */
    constructor(logger: winston.Logger, config?: Partial<DeduplicatorConfig>);
    /**
     * Start periodic cleanup of expired entries
     */
    start(): void;
    /**
     * Stop periodic cleanup
     */
    stop(): void;
    /**
     * Check if a message is a duplicate based on TikTok's native message ID
     *
     * @param msgId TikTok's native message ID (from data.common.msgId)
     * @param username TikTok username who sent the message
     * @param text Message text (for logging only)
     * @returns true if this is a duplicate, false if it's new
     */
    isDuplicate(msgId: string | undefined, username: string, text: string): boolean;
    /**
     * Record a message as seen
     *
     * @private
     */
    private recordMessage;
    /**
     * Clean up expired entries
     *
     * @private
     */
    private cleanup;
    /**
     * Get deduplication statistics
     */
    getStats(): {
        totalEntries: number;
        maxCacheSize: number;
        utilizationPercent: number;
        ttlMs: number;
        averageAgeMs: number;
        oldestEntryAgeMs: number;
        processedCount: number;
        duplicateCount: number;
        duplicateRatePercent: number;
    };
    /**
     * Clear all cached messages
     * Useful for testing or when global refresh is needed
     */
    clear(): void;
}
//# sourceMappingURL=message-deduplicator.d.ts.map