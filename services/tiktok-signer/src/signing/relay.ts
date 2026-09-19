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
 * Viewer-tab relay for canary rooms (2026-09-16 transport plan, phase 2).
 *
 * For a canary room the signer keeps the warmed viewer tab alive and taps
 * its live-page WebSocket via a second CDP session
 * (Network.webSocketFrameReceived). Frames are relayed to the listener as
 * opaque base64 PushFrames; decoding stays with the listener's connector
 * schemas — the signer never interprets them.
 *
 * Constraints learned in the lab spike, enforced here:
 *  - The tap is attached before the page reload for a capture, or the new
 *    socket's early frames are missed. captureRoom() drives page reloads,
 *    so the tap listens to ALL of the target's websockets, not one socket
 *    object, and survives reloads by design: a reload closes the old WS
 *    and opens a new one, which the tap sees as ws_closed + open.
 *  - tabIdleMs (5 min) eviction would kill a subscribed canary tab, so the
 *    pool's tab entry is refreshed on every frame (touch) and the hub drops
 *    its subscription only when it has zero SSE subscribers AND the room
 *    leaves the canary set.
 *  - ~50% of frames are ack/keepalive ("msg_type r") that fail the inner
 *    decode with premature EOF; the listener counts and drops them, they
 *    are relayed as-is. Counting happens on the consumer side.
 */

import type { CDPSession, Page } from 'puppeteer';

/** Frame relayed to canary subscribers. Opaque base64 PushFrame payload. */
export interface RelayFrame {
  type: 'frame';
  /** base64 of the raw PushFrame binaryMessage the page received. */
  payload: string;
}

export interface RelayStateEvent {
  type: 'state';
  event: 'capture' | 'open' | 'ws_closed' | 'recapture' | 'tap_error' | 'close';
  detail?: string;
}

export type RelayMessage = RelayFrame | RelayStateEvent;

export type RelayLogger = {
  info: (msg: string, meta?: Record<string, unknown>) => void;
  warn?: (msg: string, meta?: Record<string, unknown>) => void;
  error: (msg: string, meta?: Record<string, unknown>) => void;
};

export interface RelayHubOptions {
  logger?: RelayLogger;
}

interface RoomTap {
  page: Page;
  cdp: CDPSession;
  /** Socket requestIds we currently see frames for. */
  openSockets: Set<string>;
  /** Emitted when the room's WS (re)opens after a close/reload gap. */
  lastEvent: 'capture' | 'open' | 'ws_closed' | 'recapture' | 'tap_error';
}

/**
 * Owns one CDP tap per canary room. The tabs themselves belong to the
 * ViewerPool; the hub only keeps them pinned (via touch callbacks) and
 * forwards frames to subscribers.
 */
export class RelayHub {
  private readonly taps = new Map<string, RoomTap>();
  private readonly subscribers = new Map<string, Set<(msg: RelayMessage) => void>>();
  private readonly logger?: RelayLogger;
  /** Refreshes the pool tab's lastUsed so idle eviction cannot kill it. */
  private readonly touchTab: (username: string) => void;

  constructor(
    touchTab: (username: string) => void,
    options: RelayHubOptions = {}
  ) {
    this.touchTab = touchTab;
    this.logger = options.logger;
  }

  /**
   * Subscribe to a room's frames. Ensures the tab is warm (returns a
   * `capture` state event once the pool confirms the page exists) and the
   * CDP tap is attached. The returned unsubscribe drops the subscriber;
   * the tap is torn down when the last subscriber for a room leaves.
   */
  async subscribe(
    username: string,
    page: Page,
    onMessage: (msg: RelayMessage) => void
  ): Promise<() => void> {
    let subs = this.subscribers.get(username);
    if (!subs) {
      subs = new Set();
      this.subscribers.set(username, subs);
    }
    subs.add(onMessage);

    if (!this.taps.has(username)) {
      await this.attachTap(username, page, onMessage);
    } else {
      // Existing tap: the new subscriber gets the current state inline.
      const tap = this.taps.get(username);
      onMessage({ type: 'state', event: 'capture', detail: 'attached to existing tap' });
      if (tap && tap.lastEvent === 'open') {
        onMessage({ type: 'state', event: 'open' });
      }
    }

    return () => {
      const current = this.subscribers.get(username);
      if (!current) return;
      current.delete(onMessage);
      if (current.size === 0) {
        this.subscribers.delete(username);
        void this.detachTap(username);
      }
    };
  }

  private async attachTap(
    username: string,
    page: Page,
    notify: (msg: RelayMessage) => void
  ): Promise<void> {
    const cdp = await page.createCDPSession();
    const tap: RoomTap = {
      page,
      cdp,
      openSockets: new Set(),
      lastEvent: 'capture'
    };
    this.taps.set(username, tap);

    cdp.on('Network.webSocketCreated', (params: { requestId: string; url: string }) => {
      // Only the push server's socket carries chat; ignore everything else
      // (the page keeps several analytics sockets open).
      if (!params.url.includes('webcast')) return;
      tap.openSockets.add(params.requestId);
    });
    cdp.on('Network.webSocketFrameReceived', (params: {
      requestId: string;
      response: { payloadData: string; opcode: number };
    }) => {
      if (!tap.openSockets.has(params.requestId)) return;
      // Binary frames carry the protobuf PushFrame; text frames are
      // heartbeat/ack payloads ("pong" etc.) that the spike measured as
      // non-protobuf — skip opcode 1 text frames.
      if (params.response.opcode !== 2) return;
      this.touchTab(username);
      const previous = tap.lastEvent;
      if (previous === 'ws_closed' || previous === 'recapture') {
        tap.lastEvent = 'open';
        this.broadcast(username, { type: 'state', event: 'recapture' });
      }
      this.broadcast(username, { type: 'frame', payload: params.response.payloadData });
    });
    cdp.on('Network.webSocketClosed', (params: { requestId: string }) => {
      if (!tap.openSockets.delete(params.requestId)) return;
      if (tap.openSockets.size === 0) {
        tap.lastEvent = 'ws_closed';
        this.broadcast(username, { type: 'state', event: 'ws_closed' });
      }
    });
    cdp.on('Network.webSocketFrameSent', () => {
      this.touchTab(username);
    });

    try {
      await cdp.send('Network.enable');
      // The tab's live-page WebSocket was created before the tap existed,
      // so its requestId is unknown and every frame would be filtered.
      // Reload the page: the player reboots and opens a fresh WS under the
      // live tap (same mechanism a capture's reload rides on). The spike
      // constraint "attach before goto/reload" is satisfied by construction.
      await page.reload({ waitUntil: 'domcontentloaded', timeout: 30_000 });
      this.broadcast(username, { type: 'state', event: 'capture', detail: 'tap attached' });
      notify({ type: 'state', event: 'open', detail: 'waiting for frames' });
      this.logger?.info('relay tap attached', { username });
    } catch (error) {
      this.broadcast(username, {
        type: 'state',
        event: 'tap_error',
        detail: (error as Error).message
      });
      this.logger?.error('relay tap attach failed', {
        username,
        error: (error as Error).message
      });
    }
  }

  private async detachTap(username: string): Promise<void> {
    const tap = this.taps.get(username);
    if (!tap) return;
    this.taps.delete(username);
    try {
      await tap.cdp.detach();
    } catch {
      // The page/tab may already be gone (rotation, pool close).
    }
    this.logger?.info('relay tap detached (no subscribers)', { username });
  }

  private broadcast(username: string, msg: RelayMessage): void {
    const subs = this.subscribers.get(username);
    if (!subs) return;
    for (const fn of subs) {
      try {
        fn(msg);
      } catch (error) {
        this.logger?.warn?.('relay subscriber threw', {
          username,
          error: (error as Error).message
        });
      }
    }
  }

  /** Whether the room has live relay subscribers (session recovery skips it). */
  hasSubscribers(username: string): boolean {
    return (this.subscribers.get(username)?.size ?? 0) > 0;
  }

  /** Room counts currently relayed (for metrics). */
  get size(): number {
    return this.taps.size;
  }

  /** Detach everything (shutdown). */
  async close(): Promise<void> {
    for (const username of [...this.taps.keys()]) {
      await this.detachTap(username);
    }
    this.subscribers.clear();
  }
}
