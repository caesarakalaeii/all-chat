/**
 * TikTok Backoff Manager
 *
 * Manages exponential backoff for offline checks and connection errors.
 * Implements separate backoff strategies for:
 * - Offline checks: User not streaming (1m → 10m)
 * - Connection errors: Network/API failures (2s → 5m)
 */

import { Logger } from '../types/logger.js';

/**
 * Backoff state for a single username
 */
export interface BackoffState {
  username: string;
  consecutiveOfflineChecks: number;
  consecutiveErrors: number;
  lastCheckTime: number;
  nextCheckTime: number;
  currentBackoffMs: number;
  lastSeenLive?: number; // Timestamp of last successful connection
}

/**
 * Configuration for backoff behavior
 */
export interface BackoffConfig {
  baseOfflineBackoffMs: number; // 1 minute default
  maxOfflineBackoffMs: number;  // 10 minutes default
  errorBackoffMs: number;        // 2 seconds default
  maxErrorBackoffMs: number;     // 5 minutes default
}

/**
 * BackoffManager handles exponential backoff for TikTok live detection
 */
export class BackoffManager {
  private logger: Logger;
  private backoffStates: Map<string, BackoffState> = new Map();

  // Configuration with defaults
  private readonly BASE_OFFLINE_BACKOFF_MS: number;
  private readonly MAX_OFFLINE_BACKOFF_MS: number;
  private readonly ERROR_BACKOFF_MS: number;
  private readonly MAX_ERROR_BACKOFF_MS: number;

  /**
   * @param logger Winston logger instance
   * @param config Optional backoff configuration
   */
  constructor(logger: Logger, config?: Partial<BackoffConfig>) {
    this.logger = logger;

    // Apply configuration with defaults
    this.BASE_OFFLINE_BACKOFF_MS = config?.baseOfflineBackoffMs ?? 60000; // 1 minute
    this.MAX_OFFLINE_BACKOFF_MS = config?.maxOfflineBackoffMs ?? 600000;  // 10 minutes
    this.ERROR_BACKOFF_MS = config?.errorBackoffMs ?? 2000;              // 2 seconds
    this.MAX_ERROR_BACKOFF_MS = config?.maxErrorBackoffMs ?? 300000;     // 5 minutes
  }

  /**
   * Record an offline check (user is not streaming)
   * Implements exponential backoff: 1m → 2m → 4m → 8m → 10m (max)
   *
   * @param username TikTok username
   */
  recordOfflineCheck(username: string): void {
    const state = this.getOrCreateState(username);
    state.consecutiveOfflineChecks++;
    state.lastCheckTime = Date.now();

    // Calculate exponential backoff for offline status
    // Progression: 1min, 2min, 4min, 8min, 10min (capped)
    const backoffMinutes = Math.pow(2, state.consecutiveOfflineChecks - 1);
    state.currentBackoffMs = Math.min(
      backoffMinutes * this.BASE_OFFLINE_BACKOFF_MS,
      this.MAX_OFFLINE_BACKOFF_MS
    );

    state.nextCheckTime = Date.now() + state.currentBackoffMs;

    this.logger.info('User offline - increasing backoff', {
      username,
      consecutive_offline_checks: state.consecutiveOfflineChecks,
      next_check_in_ms: state.currentBackoffMs,
      next_check_in_minutes: Math.round(state.currentBackoffMs / 60000),
      next_check_at: new Date(state.nextCheckTime).toISOString()
    });
  }

  /**
   * Record a connection error (network error, API error, etc.)
   * Uses faster backoff than offline checks: 2s → 4s → 8s → 16s → ... → 5m (max)
   *
   * @param username TikTok username
   * @param error Error that occurred
   */
  recordConnectionError(username: string, error: Error): void {
    const state = this.getOrCreateState(username);
    state.consecutiveErrors++;
    state.lastCheckTime = Date.now();

    // Exponential backoff for errors: 2s, 4s, 8s, 16s, 32s, ..., 5min (max)
    const backoffSeconds = Math.pow(2, state.consecutiveErrors);
    state.currentBackoffMs = Math.min(
      backoffSeconds * this.ERROR_BACKOFF_MS,
      this.MAX_ERROR_BACKOFF_MS
    );

    state.nextCheckTime = Date.now() + state.currentBackoffMs;

    this.logger.warn('Connection error - increasing backoff', {
      username,
      error: error.message,
      consecutive_errors: state.consecutiveErrors,
      next_check_in_ms: state.currentBackoffMs,
      next_check_in_seconds: Math.round(state.currentBackoffMs / 1000)
    });
  }

  /**
   * Record successful connection (stream detected and connected)
   * Resets all backoff counters
   *
   * @param username TikTok username
   */
  recordSuccessfulConnection(username: string): void {
    const state = this.getOrCreateState(username);

    this.logger.info('Successful connection - resetting backoff', {
      username,
      previous_offline_checks: state.consecutiveOfflineChecks,
      previous_errors: state.consecutiveErrors
    });

    // Reset backoff completely
    state.consecutiveOfflineChecks = 0;
    state.consecutiveErrors = 0;
    state.currentBackoffMs = 0;
    state.lastSeenLive = Date.now();
    state.lastCheckTime = Date.now();
    state.nextCheckTime = 0; // Check immediately next cycle
  }

  /**
   * Record disconnection (user went offline during stream)
   * Resets to base interval for quick re-detection
   *
   * @param username TikTok username
   */
  recordDisconnection(username: string): void {
    const state = this.getOrCreateState(username);

    const streamDurationMs = state.lastSeenLive
      ? Date.now() - state.lastSeenLive
      : null;

    this.logger.info('Stream disconnected - resetting to base backoff', {
      username,
      stream_duration_ms: streamDurationMs,
      stream_duration_minutes: streamDurationMs
        ? Math.round(streamDurationMs / 60000)
        : null
    });

    // Reset to minimal backoff for quick re-check
    state.consecutiveOfflineChecks = 0;
    state.consecutiveErrors = 0;
    state.currentBackoffMs = this.BASE_OFFLINE_BACKOFF_MS; // Start at 1 minute
    state.lastCheckTime = Date.now();
    state.nextCheckTime = Date.now() + state.currentBackoffMs;
  }

  /**
   * Check if we should attempt to check this username now
   *
   * @param username TikTok username
   * @returns true if enough time has passed
   */
  shouldCheckNow(username: string): boolean {
    const state = this.backoffStates.get(username);
    if (!state) {
      return true; // Never checked, check now
    }

    const now = Date.now();
    return now >= state.nextCheckTime;
  }

  /**
   * Get time until next check (in milliseconds)
   *
   * @param username TikTok username
   * @returns milliseconds until next check, or 0 if should check now
   */
  getTimeUntilNextCheck(username: string): number {
    const state = this.backoffStates.get(username);
    if (!state) {
      return 0; // Never checked
    }

    return Math.max(0, state.nextCheckTime - Date.now());
  }

  /**
   * Get current backoff state for a username
   *
   * @param username TikTok username
   * @returns BackoffState or undefined if not found
   */
  getState(username: string): BackoffState | undefined {
    return this.backoffStates.get(username);
  }

  /**
   * Remove backoff state when channel is no longer active
   *
   * @param username TikTok username
   */
  removeState(username: string): void {
    const deleted = this.backoffStates.delete(username);
    if (deleted) {
      this.logger.debug('Removed backoff state', { username });
    }
  }

  /**
   * Get all usernames currently tracked
   *
   * @returns Array of usernames
   */
  getAllUsernames(): string[] {
    return Array.from(this.backoffStates.keys());
  }

  /**
   * Get statistics about current backoff states
   */
  getStats() {
    const states = Array.from(this.backoffStates.values());

    return {
      totalTracked: states.length,
      withOfflineBackoff: states.filter(s => s.consecutiveOfflineChecks > 0).length,
      withErrorBackoff: states.filter(s => s.consecutiveErrors > 0).length,
      seenLiveBefore: states.filter(s => s.lastSeenLive !== undefined).length,
      avgOfflineChecks: states.length > 0
        ? states.reduce((sum, s) => sum + s.consecutiveOfflineChecks, 0) / states.length
        : 0,
      avgErrorCount: states.length > 0
        ? states.reduce((sum, s) => sum + s.consecutiveErrors, 0) / states.length
        : 0
    };
  }

  /**
   * Create or retrieve backoff state for a username
   *
   * @private
   */
  private getOrCreateState(username: string): BackoffState {
    let state = this.backoffStates.get(username);
    if (!state) {
      state = {
        username,
        consecutiveOfflineChecks: 0,
        consecutiveErrors: 0,
        lastCheckTime: 0,
        nextCheckTime: 0,
        currentBackoffMs: 0
      };
      this.backoffStates.set(username, state);
      this.logger.debug('Created new backoff state', { username });
    }
    return state;
  }
}
