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
 * Owns one CDP tap per canary room. The tabs themselves belong to the
 * ViewerPool; the hub only keeps them pinned (via touch callbacks) and
 * forwards frames to subscribers.
 */
export class RelayHub {
    taps = new Map();
    subscribers = new Map();
    logger;
    /** Refreshes the pool tab's lastUsed so idle eviction cannot kill it. */
    touchTab;
    constructor(touchTab, options = {}) {
        this.touchTab = touchTab;
        this.logger = options.logger;
    }
    /**
     * Subscribe to a room's frames. Ensures the tab is warm (returns a
     * `capture` state event once the pool confirms the page exists) and the
     * CDP tap is attached. The returned unsubscribe drops the subscriber;
     * the tap is torn down when the last subscriber for a room leaves.
     */
    async subscribe(username, page, onMessage) {
        let subs = this.subscribers.get(username);
        if (!subs) {
            subs = new Set();
            this.subscribers.set(username, subs);
        }
        subs.add(onMessage);
        if (!this.taps.has(username)) {
            await this.attachTap(username, page, onMessage);
        }
        else {
            // Existing tap: the new subscriber gets the current state inline.
            const tap = this.taps.get(username);
            onMessage({ type: 'state', event: 'capture', detail: 'attached to existing tap' });
            if (tap && tap.lastEvent === 'open') {
                onMessage({ type: 'state', event: 'open' });
            }
        }
        return () => {
            const current = this.subscribers.get(username);
            if (!current)
                return;
            current.delete(onMessage);
            if (current.size === 0) {
                this.subscribers.delete(username);
                void this.detachTap(username);
            }
        };
    }
    async attachTap(username, page, notify) {
        const cdp = await page.createCDPSession();
        const tap = {
            page,
            cdp,
            openSockets: new Set(),
            lastEvent: 'capture'
        };
        this.taps.set(username, tap);
        cdp.on('Network.webSocketCreated', (params) => {
            // Only the push server's socket carries chat; ignore everything else
            // (the page keeps several analytics sockets open).
            if (!params.url.includes('webcast'))
                return;
            tap.openSockets.add(params.requestId);
        });
        cdp.on('Network.webSocketFrameReceived', (params) => {
            if (!tap.openSockets.has(params.requestId))
                return;
            // Binary frames carry the protobuf PushFrame; text frames are
            // heartbeat/ack payloads ("pong" etc.) that the spike measured as
            // non-protobuf — skip opcode 1 text frames.
            if (params.response.opcode !== 2)
                return;
            this.touchTab(username);
            const previous = tap.lastEvent;
            if (previous === 'ws_closed' || previous === 'recapture') {
                tap.lastEvent = 'open';
                this.broadcast(username, { type: 'state', event: 'recapture' });
            }
            this.broadcast(username, { type: 'frame', payload: params.response.payloadData });
        });
        cdp.on('Network.webSocketClosed', (params) => {
            if (!tap.openSockets.delete(params.requestId))
                return;
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
        }
        catch (error) {
            this.broadcast(username, {
                type: 'state',
                event: 'tap_error',
                detail: error.message
            });
            this.logger?.error('relay tap attach failed', {
                username,
                error: error.message
            });
        }
    }
    async detachTap(username) {
        const tap = this.taps.get(username);
        if (!tap)
            return;
        this.taps.delete(username);
        try {
            await tap.cdp.detach();
        }
        catch {
            // The page/tab may already be gone (rotation, pool close).
        }
        this.logger?.info('relay tap detached (no subscribers)', { username });
    }
    broadcast(username, msg) {
        const subs = this.subscribers.get(username);
        if (!subs)
            return;
        for (const fn of subs) {
            try {
                fn(msg);
            }
            catch (error) {
                this.logger?.warn?.('relay subscriber threw', {
                    username,
                    error: error.message
                });
            }
        }
    }
    /** Room counts currently relayed (for metrics). */
    get size() {
        return this.taps.size;
    }
    /** Detach everything (shutdown). */
    async close() {
        for (const username of [...this.taps.keys()]) {
            await this.detachTap(username);
        }
        this.subscribers.clear();
    }
}
//# sourceMappingURL=relay.js.map