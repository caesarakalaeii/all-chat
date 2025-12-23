/**
 * Timestamp Filter
 *
 * Filters out messages that are too old (potentially from reconnection replays).
 * This prevents stale messages from being published when TikTok replays
 * historical messages after a connection restart.
 */

import { Logger } from '../types/logger.js';

/**
 * Configuration for the timestamp filter
 */
export interface TimestampFilterConfig {
  /** Maximum age of messages to accept in milliseconds (default: 60 seconds) */
  maxAgeMs: number;
}

/**
 * TimestampFilter rejects messages older than a configured threshold
 */
export class TimestampFilter {
  private logger: Logger;
  private readonly MAX_AGE_MS: number;
  private droppedCount = 0;
  private acceptedCount = 0;

  /**
   * @param logger Winston logger instance
   * @param config Optional filter configuration
   */
  constructor(logger: Logger, config?: Partial<TimestampFilterConfig>) {
    this.logger = logger;
    
    // Apply configuration with defaults
    this.MAX_AGE_MS = config?.maxAgeMs ?? 60000; // 60 seconds
    
    this.logger.info('TimestampFilter initialized', {
      max_age_ms: this.MAX_AGE_MS,
      max_age_seconds: Math.round(this.MAX_AGE_MS / 1000)
    });
  }

  /**
   * Check if a message should be accepted based on its timestamp
   * 
   * @param timestamp Message timestamp (ISO string or Unix timestamp in ms)
   * @param context Optional context for logging (e.g., username)
   * @returns true if message should be accepted, false if too old
   */
  shouldAccept(timestamp: string | number, context?: { username?: string; text?: string }): boolean {
    let messageTime: number;
    
    // Convert timestamp to milliseconds
    if (typeof timestamp === 'string') {
      messageTime = new Date(timestamp).getTime();
    } else {
      messageTime = timestamp;
    }
    
    // Check if timestamp is valid
    if (isNaN(messageTime)) {
      this.logger.warn('Invalid timestamp, rejecting message', {
        timestamp,
        ...context
      });
      this.droppedCount++;
      return false;
    }
    
    const now = Date.now();
    const age = now - messageTime;
    
    // Accept if within threshold
    if (age <= this.MAX_AGE_MS) {
      this.acceptedCount++;
      this.logger.debug('Message accepted (recent)', {
        age_ms: age,
        age_seconds: Math.round(age / 1000),
        ...context
      });
      return true;
    }
    
    // Reject if too old
    this.droppedCount++;
    this.logger.info('Message rejected (too old)', {
      age_ms: age,
      age_seconds: Math.round(age / 1000),
      max_age_ms: this.MAX_AGE_MS,
      max_age_seconds: Math.round(this.MAX_AGE_MS / 1000),
      ...context
    });
    
    return false;
  }

  /**
   * Get filter statistics
   */
  getStats() {
    const total = this.acceptedCount + this.droppedCount;
    const dropRate = total > 0 ? (this.droppedCount / total) * 100 : 0;
    
    return {
      acceptedCount: this.acceptedCount,
      droppedCount: this.droppedCount,
      totalProcessed: total,
      dropRatePercent: Math.round(dropRate * 100) / 100,
      maxAgeMs: this.MAX_AGE_MS,
      maxAgeSeconds: Math.round(this.MAX_AGE_MS / 1000)
    };
  }

  /**
   * Reset statistics
   */
  resetStats(): void {
    this.acceptedCount = 0;
    this.droppedCount = 0;
    this.logger.info('Timestamp filter stats reset');
  }
}
