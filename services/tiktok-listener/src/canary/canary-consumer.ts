/**
 * This file is part of All-Chat.
 * Copyright (C) 2026 caesarakalaeii
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
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
 * Canary consumer (2026-09-16 transport plan, phase 2).
 *
 * For rooms listed in TIKTOK_CANARY_ROOMS, the listener consumes the
 * signer's SSE relay of the viewer tab's own WS frames alongside its own
 * Node WebSocket and compares per-method decoded-frame counts. The
 * canary's job is to watch the primary transport's blind spot: while the
 * listener's own connection keeps receiving, the viewer-tab frames must
 * keep flowing too, and divergence shows up here before it becomes an
 * outage report.
 *
 * Frames from the relay are decoded with the same connector schemas the
 * primary path uses, so a decode-failure delta means the two transports
 * are seeing different wire traffic, not different decoders.
 *
 * ~50% of relay frames are ack/keepalive PushFrames whose inner message
 * decode throws premature EOF (measured in the lab spike); those count
 * as `ack` and are expected on both transports.
 */

import { deserializeWebSocketMessage } from 'tiktok-live-connector';
import { RelayStream, type RelayLogger } from './relay-stream.js';

export interface CanaryDivergence {
  username: string;
  /** Which check fired. */
  kind: 'method_set' | 'decode_failure_rate' | 'frame_rate' | 'stalled';
  detail: string;
  /** Method counts of the primary WS at divergence time. */
  primary: Record<string, number>;
  /** Method counts of the canary SSE at divergence time. */
  canary: Record<string, number>;
}

export type CanaryConsumerOptions = {
  username: string;
  /** Signer base URL, e.g. http://tiktok-signer:8092. */
  signerUrl: string;
  /** Bearer token for the signer, if it runs with auth. */
  signerAuthToken?: string;
  /** How many seconds of frames each comparison window covers. */
  windowSeconds?: number;
  /** Max canary frames/sec below primary before frame_rate divergence. */
  frameRateRatio?: number;
  /**
   * Warm the target-room tab on the signer (pure-node mode, where the
   * lease never creates one). Called AT MOST ONCE per stint: each call is
   * a signer-side capture against a single-digit-per-hour budget, so the
   * consumer's own retry discipline must never loop into it.
   */
  warm?: () => Promise<boolean>;
  logger?: RelayLogger;
};

/** Comparison window state for one room. */
interface WindowState {
  /** method -> decoded count on the listener's own WS. */
  primary: Map<string, number>;
  /** method -> decoded count on the canary SSE. */
  canary: Map<string, number>;
  /** Undecodable frames on each side (premature EOF acks included). */
  primaryDecodeFailures: number;
  canaryDecodeFailures: number;
  primaryFrames: number;
  canaryFrames: number;
}

/**
 * Consumes the signer relay for one canary room and compares it against
 * the primary WS. The service wires `notePrimaryFrame(method)` /
 * `notePrimaryDecodeFailure()` into its own connection for the same room.
 *
 * Emits:
 *  - 'divergence' (CanaryDivergence): a check fired; structured log + alert
 *    rule key off this.
 *  - 'error' (Error): the SSE stream failed; the consumer reconnects with
 *    capped backoff, the primary is never affected.
 */
export class CanaryConsumer extends RelayStream {
  private readonly windowMs: number;
  private readonly frameRateRatio: number;
  private warmAttempted = false;
  private window: WindowState = CanaryConsumer.emptyWindow();
  private windowTimer?: NodeJS.Timeout;
  private static emptyWindow(): WindowState {
    return {
      primary: new Map(),
      canary: new Map(),
      primaryDecodeFailures: 0,
      canaryDecodeFailures: 0,
      primaryFrames: 0,
      canaryFrames: 0
    };
  }

  constructor(options: CanaryConsumerOptions) {
    super({
      username: options.username,
      signerUrl: options.signerUrl,
      signerAuthToken: options.signerAuthToken,
      // One warm per stint: the RelayStream calls this hook on every 409
      // connect, the flag below makes exactly the first one run the
      // capture — the signer's per-room breaker bounds even a bug here.
      onNoWarmTab: options.warm
        ? () => {
            if (this.warmAttempted) return;
            this.warmAttempted = true;
            const warm = options.warm;
            if (warm) void warm().catch(() => undefined);
          }
        : undefined,
      logger: options.logger
    });
    this.windowMs = (options.windowSeconds ?? 60) * 1000;
    this.frameRateRatio = options.frameRateRatio ?? 0.5;

    this.on('frame', (base64: string) => {
      void this.noteCanaryFrame(base64);
    });
  }

  override start(): void {
    if (this.windowTimer) return; // RelayStream.start() is idempotent, but the window timer needs its own guard
    // One comparison window per connection; drift across reconnects is
    // bounded because both counters reset together.
    this.window = CanaryConsumer.emptyWindow();
    this.warmAttempted = false; // a new stint earns one fresh warm
    this.windowTimer = setInterval(() => this.compareWindow(), this.windowMs);
    super.start();
  }

  override stop(): void {
    super.stop();
    if (this.windowTimer) clearInterval(this.windowTimer);
    this.windowTimer = undefined;
  }

  /** Wire from the primary connection's decodedData event. */
  notePrimaryFrame(method: string): void {
    this.window.primaryFrames++;
    this.window.primary.set(method, (this.window.primary.get(method) ?? 0) + 1);
  }

  /** Wire from the primary connection's decode failures (if surfaced). */
  notePrimaryDecodeFailure(): void {
    this.window.primaryFrames++;
    this.window.primaryDecodeFailures++;
  }

  private async noteCanaryFrame(base64: string): Promise<void> {
    this.window.canaryFrames++;
    // Decode with the connector's own schemas so both transports share one
    // decoder; a method count mismatch is then a wire mismatch, not a
    // decoder mismatch. deserializeWebSocketMessage is async (the payload
    // is gzip; the ack/keepalive frames throw inside it).
    try {
      const decoded = await deserializeWebSocketMessage(Buffer.from(base64, 'base64'));
      const messages = decoded.protoMessageFetchResult?.messages ?? [];
      if (messages.length === 0) {
        // Ack/keepalive frames decode to no messages — expected ~50%.
        return;
      }
      for (const message of messages) {
        const method = message.method ?? 'unknown';
        this.window.canary.set(method, (this.window.canary.get(method) ?? 0) + 1);
      }
    } catch {
      this.window.canaryDecodeFailures++;
    }
  }

  /** Compare the closing window and start a fresh one. */
  private compareWindow(): void {
    const w = this.window;
    this.window = CanaryConsumer.emptyWindow();

    if (w.canaryFrames === 0) {
      // Relay silent for a whole window: either the room is quiet or the
      // tap is dead. Quiet rooms also stall the primary, so only flag when
      // the primary kept receiving.
      if (w.primaryFrames > 10) {
        this.emitDivergence('stalled', 'relay silent while primary received frames', w);
      }
      return;
    }

    // Method set change: methods the canary decoded that the primary never
    // saw. The reverse (primary-only) is normal: primary counts its own
    // connection's gift/social plumbing the tab may not mirror exactly.
    const missingOnPrimary: string[] = [];
    for (const method of w.canary.keys()) {
      if (!w.primary.has(method)) missingOnPrimary.push(method);
    }
    if (missingOnPrimary.length > 0 && w.canaryFrames > 20) {
      this.emitDivergence(
        'method_set',
        `canary decoded methods the primary never saw: ${missingOnPrimary.join(', ')}`,
        w
      );
    }

    // Frame-rate ratio: canary flowing at well under half the primary's rate
    // over a whole window means the tab's WS is degraded, not just quieter.
    if (w.primaryFrames > 50 && w.canaryFrames < w.primaryFrames * this.frameRateRatio) {
      this.emitDivergence(
        'frame_rate',
        `canary frame rate ${w.canaryFrames} below ${Math.round(this.frameRateRatio * 100)}% of primary ${w.primaryFrames}`,
        w
      );
    }

    // Decode-failure rate delta: the two transports should fail inner
    // decode at a similar (small) rate; a canary failure spike means the
    // wire traffic diverged in kind, not just volume.
    const canaryRate = w.canaryDecodeFailures / Math.max(1, w.canaryFrames);
    const primaryRate = w.primaryDecodeFailures / Math.max(1, w.primaryFrames);
    if (w.canaryFrames > 20 && canaryRate - primaryRate > 0.2) {
      this.emitDivergence(
        'decode_failure_rate',
        `canary decode failure rate ${(canaryRate * 100).toFixed(0)}% vs primary ${(primaryRate * 100).toFixed(0)}%`,
        w
      );
    }
  }

  private emitDivergence(
    kind: CanaryDivergence['kind'],
    detail: string,
    w: WindowState
  ): void {
    const divergence: CanaryDivergence = {
      username: this.username,
      kind,
      detail,
      primary: Object.fromEntries(w.primary),
      canary: Object.fromEntries(w.canary)
    };
    this.logger?.warn('canary divergence detected', {
      username: this.username,
      kind,
      detail
    });
    this.emit('divergence', divergence);
  }
}
