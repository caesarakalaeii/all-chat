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
 * SSE client for the signer's /v1/stream/:username relay (2026-09-16
 * transport plan, phases 2-3).
 *
 * Both consumers of the relay ride this class: the phase-2 canary (compares
 * the viewer-tab frames against the primary WS) and the phase-3 premium
 * fallback (delivers them instead of the primary WS). The connection
 * machinery — fetch, SSE parsing, capped backoff — is identical; only what
 * a frame means to the caller differs, so it lives here once.
 *
 * Contract with the signer (see services/tiktok-signer/src/signing/relay.ts):
 *   event: frame  data: {"type":"frame","payload":"<base64 PushFrame>"}
 *   event: state  data: {"type":"state","event":"capture|open|ws_closed|recapture|tap_error",...}
 *   404 → room not relayable for this consumer; stop (canary set / fallback
 *   gate excludes it, or the signer was redeployed without it)
 *   409 → no warm tab for the room; a capture must run first
 *   : ping comment every 15s keeps proxies from idling the stream out
 */

import { EventEmitter } from 'events';

/** SSE event stream entry as the signer emits it. */
export interface SseEvent {
  event: 'frame' | 'state';
  data: string;
}

export type RelayLogger = {
  info: (msg: string, meta?: Record<string, unknown>) => void;
  warn: (msg: string, meta?: Record<string, unknown>) => void;
};

export type RelayStreamOptions = {
  username: string;
  /** Signer base URL, e.g. http://tiktok-signer:8092. */
  signerUrl: string;
  /** Bearer token for the signer, if it runs with auth. */
  signerAuthToken?: string;
  logger?: RelayLogger;
};

/** Backoff ceiling for relay reconnects. Matches the canary's 2026-09-16 shape. */
const MAX_RECONNECT_DELAY_MS = 60_000;

/**
 * Connects to one room's relay SSE stream and emits parsed events.
 *
 * Emits:
 *  - 'frame' (base64 PushFrame payload): one binary WS frame from the
 *    room's viewer tab.
 *  - 'state' (event name, e.g. 'ws_closed'): relay lifecycle transitions.
 *  - 'error' (Error): the stream failed; reconnection with capped backoff
 *    follows, nothing is emitted to the caller's primary path.
 *  - 'rejected': the signer answered 404 for this room — stop() was called
 *    internally and the consumer must not come back on its own.
 */
export class RelayStream extends EventEmitter {
  readonly username: string;
  protected readonly signerUrl: string;
  protected readonly authToken?: string;
  protected readonly logger?: RelayLogger;

  private controller?: AbortController;
  private reconnectTimer?: NodeJS.Timeout;
  private reconnectAttempts = 0;
  protected stopped = false;

  constructor(options: RelayStreamOptions) {
    super();
    this.username = options.username;
    this.signerUrl = options.signerUrl.replace(/\/$/, '');
    this.authToken = options.signerAuthToken;
    this.logger = options.logger;
  }

  /** Start consuming the SSE feed. Idempotent. */
  start(): void {
    if (this.controller) return;
    this.stopped = false;
    void this.connect();
  }

  /** Stop consuming and clear timers. */
  stop(): void {
    this.stopped = true;
    this.controller?.abort();
    this.controller = undefined;
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer);
    this.reconnectTimer = undefined;
  }

  private async connect(): Promise<void> {
    this.controller = new AbortController();
    const headers: Record<string, string> = { accept: 'text/event-stream' };
    if (this.authToken) headers.authorization = `Bearer ${this.authToken}`;

    const url = `${this.signerUrl}/v1/stream/${encodeURIComponent(this.username)}`;
    try {
      const response = await fetch(url, {
        headers,
        signal: this.controller.signal
      });
      if (response.status === 404) {
        this.logger?.warn('room rejected by signer relay; stopping', {
          username: this.username
        });
        this.stop();
        this.emit('rejected');
        return;
      }
      if (!response.ok || !response.body) {
        // 409 (no warm tab) and 5xx share the retry path: both can clear
        // on their own — the tab warms up on the next capture, the signer
        // recovers — so long as the delay stays bounded.
        throw new Error(`relay stream HTTP ${response.status}`);
      }
      this.reconnectAttempts = 0;
      this.logger?.info('relay stream connected', { username: this.username });
      this.emit('state', 'connected');

      await this.readSse(response.body);
      // Server closed the stream without an error: fall through to
      // reconnect below.
      throw new Error('relay stream ended');
    } catch (error) {
      if (this.stopped) return;
      this.emit('error', error as Error);
      this.reconnect();
    }
  }

  /** Parse the SSE byte stream: events separated by \n\n, lines by \n. */
  private async readSse(body: ReadableStream<Uint8Array>): Promise<void> {
    const reader = body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      let sep: number;
      while ((sep = buffer.indexOf('\n\n')) !== -1) {
        const chunk = buffer.slice(0, sep);
        buffer = buffer.slice(sep + 2);
        this.handleSseChunk(chunk);
      }
    }
  }

  private handleSseChunk(chunk: string): void {
    const event: SseEvent = { event: 'frame', data: '' };
    for (const line of chunk.split('\n')) {
      if (line.startsWith('event: ')) event.event = line.slice(7).trim() as SseEvent['event'];
      if (line.startsWith('data: ')) event.data = line.slice(6);
    }
    if (event.event === 'state') {
      let eventName = '';
      try {
        eventName = (JSON.parse(event.data) as { event?: string }).event ?? '';
      } catch {
        return;
      }
      this.logger?.info('relay state', { username: this.username, state: eventName });
      this.emit('state', eventName);
      return;
    }
    if (event.event !== 'frame') return; // heartbeat comments never reach here

    let payload: string;
    try {
      payload = (JSON.parse(event.data) as { payload: string }).payload;
    } catch {
      return;
    }
    this.emit('frame', payload);
  }

  /** Capped backoff reconnect: 1s, 2s, 4s... max 60s. */
  private reconnect(): void {
    if (this.stopped) return;
    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), MAX_RECONNECT_DELAY_MS);
    this.reconnectAttempts++;
    this.logger?.info('relay stream reconnecting', {
      username: this.username,
      attempt: this.reconnectAttempts,
      delay_ms: delay
    });
    this.reconnectTimer = setTimeout(() => {
      if (!this.stopped) void this.connect();
    }, delay);
  }
}
