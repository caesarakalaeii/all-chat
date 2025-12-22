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
/**
 * MessageDeduplicator tracks recently seen messages to prevent duplicates
 */
export class MessageDeduplicator {
    logger;
    seenMessages = new Map();
    cleanupTimer;
    duplicateCount = 0;
    processedCount = 0;
    TTL_MS;
    CLEANUP_INTERVAL_MS;
    MAX_CACHE_SIZE;
    /**
     * @param logger Winston logger instance
     * @param config Optional deduplicator configuration
     */
    constructor(logger, config) {
        this.logger = logger;
        // Apply configuration with defaults
        this.TTL_MS = config?.ttlMs ?? 300000; // 5 minutes
        this.CLEANUP_INTERVAL_MS = config?.cleanupIntervalMs ?? 60000; // 1 minute
        this.MAX_CACHE_SIZE = config?.maxCacheSize ?? 10000;
        this.logger.info('MessageDeduplicator initialized', {
            ttl_ms: this.TTL_MS,
            ttl_minutes: Math.round(this.TTL_MS / 60000),
            cleanup_interval_ms: this.CLEANUP_INTERVAL_MS,
            max_cache_size: this.MAX_CACHE_SIZE
        });
    }
    /**
     * Start periodic cleanup of expired entries
     */
    start() {
        if (this.cleanupTimer) {
            this.logger.warn('Deduplicator cleanup already running');
            return;
        }
        this.cleanupTimer = setInterval(() => {
            this.cleanup();
        }, this.CLEANUP_INTERVAL_MS);
        this.logger.info('Deduplicator cleanup started', {
            interval_ms: this.CLEANUP_INTERVAL_MS
        });
    }
    /**
     * Stop periodic cleanup
     */
    stop() {
        if (this.cleanupTimer) {
            clearInterval(this.cleanupTimer);
            this.cleanupTimer = undefined;
            this.logger.info('Deduplicator cleanup stopped');
        }
    }
    /**
     * Check if a message is a duplicate based on TikTok's native message ID
     *
     * @param msgId TikTok's native message ID (from data.common.msgId)
     * @param username TikTok username who sent the message
     * @param text Message text (for logging only)
     * @returns true if this is a duplicate, false if it's new
     */
    isDuplicate(msgId, username, text) {
        this.processedCount++;
        // If no msgId provided, cannot deduplicate (allow through)
        if (!msgId) {
            this.logger.warn('Message without msgId, cannot deduplicate', {
                username,
                text_preview: text.substring(0, 50)
            });
            return false;
        }
        const existing = this.seenMessages.get(msgId);
        if (existing) {
            // Check if it's still within TTL
            const age = Date.now() - existing.timestamp;
            if (age < this.TTL_MS) {
                this.duplicateCount++;
                this.logger.info('Duplicate message detected (prevented replay)', {
                    msg_id: msgId,
                    username,
                    text_preview: text.substring(0, 50),
                    age_ms: age,
                    age_seconds: Math.round(age / 1000),
                    duplicate_rate: `${((this.duplicateCount / this.processedCount) * 100).toFixed(2)}%`
                });
                return true;
            }
            // Expired, remove it
            this.seenMessages.delete(msgId);
        }
        // Not a duplicate, record it
        this.recordMessage(msgId, username, text);
        return false;
    }
    /**
     * Record a message as seen
     *
     * @private
     */
    recordMessage(msgId, username, text) {
        // Check cache size limit
        if (this.seenMessages.size >= this.MAX_CACHE_SIZE) {
            this.logger.warn('Cache size limit reached, forcing cleanup', {
                current_size: this.seenMessages.size,
                max_size: this.MAX_CACHE_SIZE
            });
            this.cleanup();
        }
        this.seenMessages.set(msgId, {
            msgId,
            timestamp: Date.now(),
            username,
            text: text.substring(0, 100) // Store preview for debugging
        });
        this.logger.debug('Message recorded for deduplication', {
            msg_id: msgId,
            username,
            text_preview: text.substring(0, 50),
            cache_size: this.seenMessages.size
        });
    }
    /**
     * Clean up expired entries
     *
     * @private
     */
    cleanup() {
        const now = Date.now();
        let removed = 0;
        for (const [msgId, fingerprint] of this.seenMessages) {
            const age = now - fingerprint.timestamp;
            if (age >= this.TTL_MS) {
                this.seenMessages.delete(msgId);
                removed++;
            }
        }
        if (removed > 0) {
            this.logger.debug('Cleaned up expired deduplication entries', {
                removed,
                remaining: this.seenMessages.size
            });
        }
    }
    /**
     * Get deduplication statistics
     */
    getStats() {
        const now = Date.now();
        const entries = Array.from(this.seenMessages.values());
        const ages = entries.map(e => now - e.timestamp);
        const avgAge = ages.length > 0
            ? ages.reduce((sum, age) => sum + age, 0) / ages.length
            : 0;
        const duplicateRate = this.processedCount > 0
            ? (this.duplicateCount / this.processedCount) * 100
            : 0;
        return {
            totalEntries: this.seenMessages.size,
            maxCacheSize: this.MAX_CACHE_SIZE,
            utilizationPercent: Math.round((this.seenMessages.size / this.MAX_CACHE_SIZE) * 100),
            ttlMs: this.TTL_MS,
            averageAgeMs: Math.round(avgAge),
            oldestEntryAgeMs: ages.length > 0 ? Math.max(...ages) : 0,
            processedCount: this.processedCount,
            duplicateCount: this.duplicateCount,
            duplicateRatePercent: Math.round(duplicateRate * 100) / 100
        };
    }
    /**
     * Clear all cached messages
     * Useful for testing or when global refresh is needed
     */
    clear() {
        const size = this.seenMessages.size;
        this.seenMessages.clear();
        this.logger.info('Cleared all deduplication cache', { entries_cleared: size });
    }
}
//# sourceMappingURL=message-deduplicator.js.map