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
 * Premium fallback transport (2026-09-16 transport plan, phase 3).
 *
 * When a room's primary WS cannot connect (flap retries exhausted) and the
 * room's streamer is premium, the listener switches that room's delivery
 * from its own Node WebSocket to the signer's viewer-tab relay: the same
 * SSE stream the canary only watches, but here the frames are DELIVERED —
 * decoded with the connector's own schemas and replayed into the room's
 * TikTokLiveConnection, so every chat/gift/social/envelope handler, the
 * dedup layer and the heartbeat monitor work exactly as on the primary
 * path. One decoder on both sides means a delivery gap is a wire gap.
 *
 * Lifecycle (owned by the service, not this class):
 *  - promotion happens once, at flap-exhaustion, in connectToStream
 *  - demotion happens when the room ends (relay ws_closed / max duration),
 *    which returns the room to the poller for a fresh primary attempt
 *
 * The connection object handed in is never .connect()-ed here — it stays
 * DISCONNECTED, which also keeps the heartbeat monitor from timing it out.
 * Its role is to be the emitter whose listeners were already wired in
 * connectToStream before the primary attempt failed.
 */

import type { TikTokLiveConnection } from 'tiktok-live-connector';
import { deserializeWebSocketMessage } from 'tiktok-live-connector';
import { RelayStream, type RelayLogger } from './relay-stream.js';

/** Frames delivered downstream by the fallback, by outcome. */
export type FallbackDeliveryOutcome =
  | 'delivered'
  | 'no_messages' // ack/keepalive PushFrame: counted, nothing to replay
  | 'decode_failure' // threw in deserializeWebSocketMessage (malformed frame)
  | 'replay_error'; // connector replay threw: room state likely stale

export type FallbackConsumerOptions = {
  username: string;
  /** Signer base URL, e.g. http://tiktok-signer:8092. */
  signerUrl: string;
  /**
   * The room's connection object, listeners already wired by
   * connectToStream. Kept DISCONNECTED; only used as the event emitter
   * whose processProtoMessageFetchResult re-emits chat/gift/etc.
   */
  connection: TikTokLiveConnection;
  /** Bearer token for the signer, if it runs with auth. */
  signerAuthToken?: string;
  /**
   * Hard ceiling on one fallback stint, in milliseconds. The fallback is
   * a bridge, not a home: past this the room goes back to the poller and
   * retries the primary WS, so a tier can never get silently stuck.
   */
  maxDurationMs: number;
  logger?: RelayLogger;
};

/**
 * Delivers the signer relay's frames as the room's message stream.
 *
 * Emits:
 *  - 'delivered' (outcome: FallbackDeliveryOutcome): one relay frame was
 *    processed; the service records it for metrics.
 *  - 'ended' (reason: 'ws_closed' | 'tap_error' | 'rejected' |
 *    'max_duration'): the fallback stint is over; the service must demote
 *    the room back to the primary tier.
 *  - 'error' (Error): the SSE stream failed; reconnection with capped
 *    backoff follows, delivery is unaffected until 'ended'.
 */
export class FallbackConsumer extends RelayStream {
  private readonly connection: TikTokLiveConnection;
  private readonly maxDurationMs: number;
  private maxDurationTimer?: NodeJS.Timeout;
  private ended = false;

  constructor(options: FallbackConsumerOptions) {
    super({
      username: options.username,
      signerUrl: options.signerUrl,
      signerAuthToken: options.signerAuthToken,
      logger: options.logger
    });
    this.connection = options.connection;
    this.maxDurationMs = options.maxDurationMs;

    this.on('frame', (base64: string) => {
      void this.deliverFrame(base64);
    });
    this.on('state', (state: string) => {
      if (state === 'ws_closed' || state === 'tap_error') {
        // The room's stream (or the tab serving it) is gone. Reconnects
        // will only spin against a dead tab — end the stint and let the
        // service re-evaluate the primary tier.
        this.end(state);
      }
    });
    this.on('rejected', () => {
      this.end('rejected');
    });
  }

  override start(): void {
    if (this.ended) return;
    super.start();
    // Arm the max-duration timer once per stint, not per reconnect.
    if (!this.maxDurationTimer) {
      this.maxDurationTimer = setTimeout(() => this.end('max_duration'), this.maxDurationMs);
    }
  }

  override stop(): void {
    if (this.maxDurationTimer) clearTimeout(this.maxDurationTimer);
    this.maxDurationTimer = undefined;
    super.stop();
  }

  private end(reason: 'ws_closed' | 'tap_error' | 'rejected' | 'max_duration'): void {
    if (this.ended) return;
    this.ended = true;
    this.stop();
    this.logger?.info('fallback stint ended', { username: this.username, reason });
    this.emit('ended', reason);
  }

  private async deliverFrame(base64: string): Promise<void> {
    if (this.ended) return;
    try {
      const decoded = await deserializeWebSocketMessage(Buffer.from(base64, 'base64'));
      const fetchResult = decoded.protoMessageFetchResult;
      if (!fetchResult || (fetchResult.messages?.length ?? 0) === 0) {
        this.emit('delivered', 'no_messages');
        return;
      }
      // Replay through the connector's own pipeline: this re-emits
      // decodedData per message and then chat/gift/social/envelope for the
      // listeners connectToStream wired before the primary attempt failed.
      // A "connected" TikTokLiveConnection would do this itself on WS
      // data; here we hold it DISCONNECTED and drive its processor by
      // hand, which is the whole difference from the canary.
      await (
        this.connection as unknown as {
          processProtoMessageFetchResult(fr: unknown): Promise<void>;
        }
      ).processProtoMessageFetchResult(fetchResult);
      this.emit('delivered', 'delivered');
    } catch {
      // Either the frame failed decode (premature-EOF ack shapes land in
      // no_messages above; this is malformed) or the connector replay
      // threw. Both are counted; delivery continues with the next frame.
      this.emit('delivered', 'decode_failure');
    }
  }
}
