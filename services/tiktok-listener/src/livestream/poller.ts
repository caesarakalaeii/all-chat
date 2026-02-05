/**
 * TikTok Live Stream Poller
 *
 * Background task that periodically checks if inactive TikTok channels went live.
 * Respects backoff intervals and coordinates with status checker and backoff manager.
 */

import { Logger } from '../types/logger.js';
import { TikTokStatusChecker } from './status-checker.js';
import { BackoffManager } from './backoff-manager.js';

/**
 * Target to poll for live status
 */
export interface PollingTarget {
  username: string;
  overlayId: string;
}

/**
 * Callback when a target goes live
 */
export type OnLiveCallback = (username: string, overlayId: string) => Promise<void>;

/**
 * Configuration for the poller
 */
export interface PollerConfig {
  pollIntervalMs: number; // How often to check the pool (default: 30 seconds)
}

/**
 * LiveStreamPoller periodically checks if offline users went live
 */
export class LiveStreamPoller {
  private logger: Logger;
  private statusChecker: TikTokStatusChecker;
  private backoffManager: BackoffManager;

  private pollingTargets: Map<string, PollingTarget> = new Map();
  private pollingTimer?: NodeJS.Timeout;
  private readonly POLL_INTERVAL_MS: number;
  private isRunning = false;

  // Callback for when a target goes live
  private onLiveCallback?: OnLiveCallback;

  /**
   * @param statusChecker TikTokStatusChecker instance
   * @param backoffManager BackoffManager instance
   * @param logger Winston logger instance
   * @param config Optional poller configuration
   */
  constructor(
    statusChecker: TikTokStatusChecker,
    backoffManager: BackoffManager,
    logger: Logger,
    config?: Partial<PollerConfig>
  ) {
    this.statusChecker = statusChecker;
    this.backoffManager = backoffManager;
    this.logger = logger;
    this.POLL_INTERVAL_MS = config?.pollIntervalMs ?? 30000; // 30 seconds default
  }

  /**
   * Set callback function to be called when a target goes live
   *
   * @param callback Function to call with username and overlayId
   */
  setOnLiveCallback(callback: OnLiveCallback): void {
    this.onLiveCallback = callback;
  }

  /**
   * Start periodic polling
   */
  start(): void {
    if (this.isRunning) {
      this.logger.warn('Poller already running');
      return;
    }

    this.isRunning = true;
    this.logger.info('Starting livestream poller', {
      interval_ms: this.POLL_INTERVAL_MS,
      interval_seconds: this.POLL_INTERVAL_MS / 1000
    });

    // Run immediately, then on interval
    this.runPollingCycle().catch(err => {
      this.logger.error('Error in initial polling cycle', { error: err.message });
    });

    this.pollingTimer = setInterval(() => {
      this.runPollingCycle().catch(err => {
        this.logger.error('Error in polling cycle', { error: err.message });
      });
    }, this.POLL_INTERVAL_MS);
  }

  /**
   * Stop periodic polling
   */
  stop(): void {
    if (this.pollingTimer) {
      clearInterval(this.pollingTimer);
      this.pollingTimer = undefined;
    }
    this.isRunning = false;
    this.logger.info('Stopped livestream poller');
  }

  /**
   * Add a username to polling targets
   *
   * @param username TikTok username
   * @param overlayId Overlay ID that needs this stream
   */
  addTarget(username: string, overlayId: string): void {
    this.pollingTargets.set(username, { username, overlayId });
    this.logger.debug('Added polling target', { username, overlay_id: overlayId });
  }

  /**
   * Remove a username from polling targets
   *
   * @param username TikTok username
   */
  removeTarget(username: string): void {
    const deleted = this.pollingTargets.delete(username);
    if (deleted) {
      this.backoffManager.removeState(username);
      this.statusChecker.clearCache(username);
      this.logger.debug('Removed polling target', { username });
    }
  }

  /**
   * Get current polling targets count
   *
   * @returns Number of targets being polled
   */
  getTargetCount(): number {
    return this.pollingTargets.size;
  }

  /**
   * Get all current targets
   *
   * @returns Array of polling targets
   */
  getTargets(): PollingTarget[] {
    return Array.from(this.pollingTargets.values());
  }

  /**
   * Check if a username is being polled
   *
   * @param username TikTok username
   * @returns true if currently being polled
   */
  isTargetActive(username: string): boolean {
    return this.pollingTargets.has(username);
  }

  /**
   * Get poller status
   */
  getStatus() {
    return {
      isRunning: this.isRunning,
      targetCount: this.pollingTargets.size,
      pollIntervalMs: this.POLL_INTERVAL_MS,
      backoffStats: this.backoffManager.getStats(),
      cacheStats: this.statusChecker.getCacheStats()
    };
  }

  /**
   * Single polling cycle - checks all targets respecting backoff
   *
   * @private
   */
  private async runPollingCycle(): Promise<void> {
    const targets = Array.from(this.pollingTargets.values());

    if (targets.length === 0) {
      this.logger.debug('No polling targets, skipping cycle');
      return;
    }

    this.logger.debug('Running polling cycle', {
      target_count: targets.length
    });

    // Check all targets in parallel
    const checkPromises = targets.map(target => this.checkTarget(target));
    const results = await Promise.allSettled(checkPromises);

    // Log any failures
    let successCount = 0;
    let liveCount = 0;
    let skippedCount = 0;

    results.forEach((result, index) => {
      if (result.status === 'fulfilled') {
        successCount++;
        if (result.value) {
          liveCount++;
        }
      } else {
        this.logger.error('Target check failed', {
          username: targets[index].username,
          error: result.reason
        });
      }
    });

    skippedCount = targets.length - successCount;

    this.logger.debug('Polling cycle complete', {
      total_targets: targets.length,
      checked: successCount,
      skipped_backoff: skippedCount,
      found_live: liveCount
    });

    // Run stuck state recovery (checks for channels in max backoff >5min)
    this.recoverStuckChannels();

    // Clean up expired cache entries
    this.statusChecker.cleanupExpiredCache();
  }

  /**
   * Check a single target, respecting backoff
   *
   * @param target Polling target to check
   * @returns true if live (and callback was called), false otherwise
   * @private
   */
  private async checkTarget(target: PollingTarget): Promise<boolean> {
    const { username, overlayId } = target;

    // Check if backoff allows checking now
    if (!this.backoffManager.shouldCheckNow(username)) {
      const timeUntilNext = this.backoffManager.getTimeUntilNextCheck(username);
      this.logger.debug('Skipping check due to backoff', {
        username,
        time_until_next_check_ms: timeUntilNext,
        time_until_next_check_minutes: Math.round(timeUntilNext / 60000)
      });
      return false;
    }

    // Perform live status check
    this.logger.debug('Checking target status', { username });
    const result = await this.statusChecker.checkLiveStatus(username);

    if (result.error) {
      this.backoffManager.recordConnectionError(username, result.error);
      return false;
    }

    if (result.isLive) {
      this.logger.info('User is now live!', {
        username,
        overlay_id: overlayId
      });

      // Call the live callback if set
      if (this.onLiveCallback) {
        try {
          await this.onLiveCallback(username, overlayId);
          // Note: Successful connection will be recorded by the connection handler
          // We don't reset backoff here to avoid race conditions
        } catch (error) {
          this.logger.error('Error in live callback', {
            username,
            error: error instanceof Error ? error.message : String(error)
          });
          // Record as error so we retry sooner
          this.backoffManager.recordConnectionError(
            username,
            error instanceof Error ? error : new Error(String(error))
          );
        }
      }

      return true;
    } else {
      // Not live - record offline check
      this.backoffManager.recordOfflineCheck(username);
      return false;
    }
  }
}

  /**
   * Recover stuck channels (channels in max backoff for too long)
   * Run periodically to prevent channels from being permanently stuck
   * 
   * Detects:
   * - Max backoff (3min) for >5 minutes
   * - Last check >5 minutes ago
   * 
   * Action: Force reset to base backoff
   */
  recoverStuckChannels(): void {
    const now = Date.now();
    const stuckChannels: string[] = [];

    for (const username of this.backoffManager.getAllUsernames()) {
      const state = this.backoffManager.getState(username);
      
      if (!state) continue;

      // Check if stuck: max backoff (180000ms = 3min) for >5 minutes
      const isAtMaxBackoff = state.currentBackoffMs >= 180000;
      const timeSinceLastCheck = now - state.lastCheckTime;
      const stuckDuration = 5 * 60 * 1000; // 5 minutes

      if (isAtMaxBackoff && timeSinceLastCheck > stuckDuration) {
        stuckChannels.push(username);

        this.logger.warn('Detected stuck channel in max backoff, forcing recovery', {
          username,
          current_backoff_ms: state.currentBackoffMs,
          time_since_last_check_ms: timeSinceLastCheck,
          time_since_last_check_minutes: Math.round(timeSinceLastCheck / 60000),
          consecutive_offline: state.consecutiveOfflineChecks,
          consecutive_errors: state.consecutiveErrors,
          action: 'auto_recovery'
        });

        // Force reset to base backoff (20 seconds)
        this.backoffManager.removeState(username);

        // Mark that we should check this immediately
        const target = this.pollingTargets.get(username);
        if (target) {
          // Target will be checked in next cycle with fresh backoff
          this.logger.info('Stuck channel will be rechecked immediately', { username });
        }
      }
    }

    if (stuckChannels.length > 0) {
      this.logger.info('Auto-recovery cycle complete', {
        stuck_channels_recovered: stuckChannels.length,
        usernames: stuckChannels
      });
    }
  }
