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
// ADR-0007 stabilization gate: releasing leases before the fleet view settles
// during a scale event makes released streams bounce between pods. Mirrors
// rebalanceStabilizationPeriod in shared/sourcemanager/coordinator.go.
const REBALANCE_STABILIZATION_MS = 30_000;

interface LeaseEntry {
  streamID: string;
  timer: NodeJS.Timeout;
  lostCallback: () => void;
  consecutiveFailures: number;
  stopped: boolean;
}

interface RebalanceState {
  lastPeerCount: number;
  peerCountStableAt: number;
}

/**
 * Which of `leases` to give up when this pod must shed `excess` of them.
 *
 * Release order is by room freshness, oldest messages first: a hot room (a
 * message within HOT_ROOM_THRESHOLD_SECONDS) must survive its receiving pod
 * a full reconnect cycle (re-handshake, flap exposure) for a move that buys
 * nothing between healthy pods, while an idle room was likely already
 * reconnecting. A room with no freshness entry (never connected, fresh
 * demand) counts as maximally releasable so cold-start rooms still flow to
 * the least-loaded pod. Ties break alphabetically, matching the Go
 * coordinator: both coordinators facing the same fleet state shed the same
 * streams.
 */
export const HOT_ROOM_THRESHOLD_SECONDS = 60;

export function selectLeasesToRelease(
  leases: string[],
  excess: number,
  secondsSinceLastMessage: (streamID: string) => number | undefined
): string[] {
  // Most-releasable first: oldest message, a missing monitor entry (never
  // connected) counting as infinitely old, alphabetical on ties so both
  // coordinators shed the same streams for the same fleet state.
  const ranked = [...leases].sort((a, b) => {
    const freshnessA = secondsSinceLastMessage(a) ?? Number.POSITIVE_INFINITY;
    const freshnessB = secondsSinceLastMessage(b) ?? Number.POSITIVE_INFINITY;
    if (freshnessA !== freshnessB) return freshnessB - freshnessA;
    return a.localeCompare(b);
  });
  return ranked.slice(0, excess).sort();
}

export class LeadershipCoordinator {
  private platform: string;
  private callerID: string;
  private client: SourceManagerClient;
  private logger: Logger;
  private leases: Map<string, LeaseEntry> = new Map();
  private rebalanceState?: RebalanceState;

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
   * ADR-0007 rebalancing, ported from shared/sourcemanager/coordinator.go.
   *
   * Registers this pod as a peer, then sheds leases in excess of
   * min(ceil(totalStreams / peerCount), maxPerPod). Release selection ranks
   * by room freshness when `secondsSinceLastMessage` is provided — idle
   * rooms first, hot rooms (message within HOT_ROOM_THRESHOLD_SECONDS)
   * last — because a moved hot room must re-handshake on the receiving pod
   * while an idle room was likely already reconnecting; without the
   * accessor the selection falls back to alphabetical, the Go coordinator's
   * behaviour. Returns the released stream IDs so the caller can disconnect
   * them and exclude them from re-acquisition for a cycle.
   *
   * maxPerPod carries the caller's hard connection ceiling
   * (TIKTOK_MAX_STREAMS_PER_POD): the Euler proxy caps concurrent proxied
   * WebSockets (ADR-0052), so a pod must never hold more leases than it can
   * connect — a leased-but-unconnectable stream is deaf everywhere, because no
   * other pod may claim it. The Go coordinator has no such parameter because
   * no Go listener connects through the Euler proxy.
   *
   * The stabilization gate prevents the release→re-acquire oscillation during
   * scale events: the first call after a peer-count change only records the new
   * count; releases happen once the count has been stable for
   * REBALANCE_STABILIZATION_MS.
   */
  async rebalance(
    totalStreams: number,
    maxPerPod?: number,
    secondsSinceLastMessage?: (streamID: string) => number | undefined
  ): Promise<string[]> {
    let peerCount: number;
    try {
      peerCount = await this.client.registerPeer(this.platform, this.callerID);
    } catch (err) {
      this.logger.warn('Failed to register peer for rebalancing', {
        platform: this.platform,
        error: String(err),
      });
      return [];
    }

    if (peerCount <= 0) peerCount = 1;

    const now = Date.now();
    if (!this.rebalanceState || this.rebalanceState.lastPeerCount !== peerCount) {
      this.rebalanceState = {
        lastPeerCount: peerCount,
        peerCountStableAt: now + REBALANCE_STABILIZATION_MS,
      };

      this.logger.info('Peer count changed, waiting for stabilization before rebalancing', {
        platform: this.platform,
        peer_count: peerCount,
        stabilization_period_ms: REBALANCE_STABILIZATION_MS,
      });
      return [];
    }
    if (now < this.rebalanceState.peerCountStableAt) return [];

    const fairShare = Math.ceil(totalStreams / peerCount);
    const targetLeases = maxPerPod !== undefined ? Math.min(fairShare, maxPerPod) : fairShare;
    const currentCount = this.leases.size;
    const excess = currentCount - targetLeases;
    if (excess <= 0) return [];

    const candidates = [...this.leases.keys()];
    const toRelease = secondsSinceLastMessage
      ? selectLeasesToRelease(candidates, excess, secondsSinceLastMessage)
      : candidates.sort().slice(targetLeases);

    this.logger.debug('rebalance candidates', {
      candidates: candidates.map((streamID) => ({
        stream_id: streamID,
        last_message_seconds_ago: secondsSinceLastMessage?.(streamID) ?? null,
      })),
    });


    // One release path for demand removal, leadership loss and rebalancing.
    // Fire-and-forget per stream (void, matching the Go coordinator's
    // asynchronous release): a failed release lets the lease expire
    // server-side, so rebalancing must not stall on one slow request.
    for (const streamID of toRelease) {
      void this.release(streamID);
    }

    this.logger.info('Rebalanced leadership leases', {
      platform: this.platform,
      peer_count: peerCount,
      total_streams: totalStreams,
      max_per_pod: targetLeases,
      had: currentCount,
      released: toRelease.length,
      kept: currentCount - toRelease.length,
    });

    return toRelease;
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
