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
 * Leadership Coordinator
 *
 * Manages per-stream leadership leases with source-manager.
 * Mirrors Go shared/sourcemanager/coordinator.go LeadershipCoordinator.
 *
 * Lifecycle per stream:
 *   ensureLeadership(streamID) → claim → heartbeat loop (5s tick) → renew
 *   On failure: 2 consecutive failures grace → lostCallback() → cleanup
 *   On shutdown: stop() → release all leases
 */

import { randomUUID } from 'crypto';
import { SourceManagerClient } from './client.js';
import { Logger } from '../types/logger.js';

const RENEWAL_INTERVAL_MS = 5000;    // 5 seconds, matches Go
const RETRY_DELAYS_MS = [100, 200, 400]; // Exponential backoff for renewal retries
const MAX_CONSECUTIVE_FAILURES = 2;  // Grace period before declaring leadership lost

interface LeaseEntry {
  streamID: string;
  timer: NodeJS.Timeout;
  lostCallback: () => void;
  consecutiveFailures: number;
  stopped: boolean;
}

export class LeadershipCoordinator {
  private platform: string;
  private callerID: string;
  private client: SourceManagerClient;
  private logger: Logger;
  private leases: Map<string, LeaseEntry> = new Map();

  constructor(platform: string, client: SourceManagerClient, logger: Logger) {
    this.platform = platform;
    this.callerID = randomUUID();
    this.client = client;
    this.logger = logger;

    this.logger.info('Leadership coordinator initialized', {
      platform,
      caller_id: this.callerID,
      renewal_interval_ms: RENEWAL_INTERVAL_MS,
    });
  }

  /**
   * EnsureLeadership claims leadership for a stream and starts a renewal loop.
   * If already holding leadership for this stream, returns true immediately.
   * Returns false if another pod holds leadership (not an error).
   */
  async ensureLeadership(streamID: string, lostCallback: () => void): Promise<boolean> {
    // Already have a lease
    if (this.leases.has(streamID)) {
      return true;
    }

    try {
      const acquired = await this.client.claimLeadership(this.platform, streamID, this.callerID);
      if (!acquired) {
        this.logger.debug('Leadership claim skipped (held by another pod)', {
          stream_id: streamID,
        });
        return false;
      }
    } catch (err) {
      this.logger.error('Leadership claim failed', {
        stream_id: streamID,
        error: String(err),
      });
      return false;
    }

    // Start heartbeat loop
    const entry: LeaseEntry = {
      streamID,
      lostCallback,
      consecutiveFailures: 0,
      stopped: false,
      timer: setInterval(() => this.heartbeat(streamID), RENEWAL_INTERVAL_MS),
    };

    this.leases.set(streamID, entry);

    this.logger.info('Leadership acquired', {
      stream_id: streamID,
      caller_id: this.callerID,
    });

    return true;
  }

  /**
   * Release leadership for a specific stream. Stops the heartbeat and calls
   * source-manager to release. Used when demand is removed for a stream.
   */
  async release(streamID: string): Promise<void> {
    const entry = this.leases.get(streamID);
    if (!entry) return;

    entry.stopped = true;
    clearInterval(entry.timer);
    this.leases.delete(streamID);

    try {
      await this.client.releaseLeadership(this.platform, streamID, this.callerID);
      this.logger.info('Leadership released', { stream_id: streamID });
    } catch (err) {
      this.logger.warn('Failed to release leadership (lease will expire)', {
        stream_id: streamID,
        error: String(err),
      });
    }
  }

  /**
   * Check if we hold leadership for a stream.
   */
  hasLeadership(streamID: string): boolean {
    return this.leases.has(streamID);
  }

  /**
   * Stop all leases and release leadership. Called during graceful shutdown.
   */
  async stop(): Promise<void> {
    const streamIDs = Array.from(this.leases.keys());
    this.logger.info('Releasing all leadership leases', { count: streamIDs.length });

    // Stop all heartbeat timers first
    for (const [, entry] of this.leases) {
      entry.stopped = true;
      clearInterval(entry.timer);
    }

    // Release all leases in parallel (fire-and-forget, matches Go async pattern)
    const releases = streamIDs.map(async (streamID) => {
      try {
        await this.client.releaseLeadership(this.platform, streamID, this.callerID);
      } catch {
        // Non-fatal — lease will expire naturally
      }
    });

    // Wait briefly but don't block shutdown
    await Promise.allSettled(releases);
    this.leases.clear();
  }

  /**
   * Get the caller ID for this coordinator instance.
   */
  getCallerID(): string {
    return this.callerID;
  }

  /**
   * Get count of active leases.
   */
  getLeaseCount(): number {
    return this.leases.size;
  }

  /**
   * Heartbeat loop — renew leadership with retry and grace period.
   */
  private async heartbeat(streamID: string): Promise<void> {
    const entry = this.leases.get(streamID);
    if (!entry || entry.stopped) return;

    // Try renewal with retry
    let renewed = false;
    for (const delay of RETRY_DELAYS_MS) {
      try {
        renewed = await this.client.renewLeadership(this.platform, streamID, this.callerID);
        if (renewed) {
          entry.consecutiveFailures = 0;
          return;
        }
      } catch (err) {
        const errMsg = String(err);
        if (errMsg.includes('leadership_lost')) {
          // Definitive loss — no retry
          this.logger.warn('Leadership lost (410 GONE)', { stream_id: streamID });
          this.handleLeadershipLost(streamID, entry);
          return;
        }
        // Network/transient error — retry after delay
        await new Promise((resolve) => setTimeout(resolve, delay));
      }
    }

    // All retries failed
    entry.consecutiveFailures++;
    this.logger.warn('Leadership renewal failed', {
      stream_id: streamID,
      consecutive_failures: entry.consecutiveFailures,
      max_before_lost: MAX_CONSECUTIVE_FAILURES,
    });

    if (entry.consecutiveFailures >= MAX_CONSECUTIVE_FAILURES) {
      this.logger.error('Leadership considered lost after grace period', {
        stream_id: streamID,
        consecutive_failures: entry.consecutiveFailures,
      });
      this.handleLeadershipLost(streamID, entry);
    }
  }

  /**
   * Handle leadership loss — stop heartbeat and invoke callback.
   */
  private handleLeadershipLost(streamID: string, entry: LeaseEntry): void {
    entry.stopped = true;
    clearInterval(entry.timer);
    this.leases.delete(streamID);

    // Invoke callback asynchronously (matches Go pattern)
    try {
      entry.lostCallback();
    } catch (err) {
      this.logger.error('Leadership lost callback threw error', {
        stream_id: streamID,
        error: String(err),
      });
    }
  }
}
