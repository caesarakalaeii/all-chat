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
 * WebSocket Client
 *
 * Manages WebSocket connection to the API Gateway for real-time chat messages.
 * WebSocket connections are proxied through Nginx at /ws/* paths.
 *
 * Features:
 * - Automatic reconnection with exponential backoff
 * - Ping/Pong keep-alive handling
 * - Type-safe message handling
 * - Same-origin WebSocket (proxied via Nginx)
 *
 * Usage:
 *   const client = new WebSocketClient();
 *   client.connect(overlayId, token);
 *   client.onMessage((message) => console.log(message));
 *   client.disconnect();
 */

import type { ChatMessage, WebSocketMessage } from '../types/message'
import {
  computeBackoffDelay,
  HEARTBEAT_INTERVAL_MS,
  isConnectionStale,
  WATCHDOG_INTERVAL_MS,
} from '../utils/overlayStreamCore'

// Automatically detect WebSocket URL based on page protocol
// In production: wss://domain.com proxied to backend via Nginx
// In development: ws://localhost:8080
function getWebSocketUrl(): string {
  if (typeof window !== 'undefined') {
    // Browser: use same origin with ws/wss protocol
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${protocol}//${window.location.host}`
  }
  // SSR: use env var or localhost
  return process.env.NEXT_PUBLIC_WS_URL || 'ws://localhost:8080'
}

const WS_URL = getWebSocketUrl()

const SEEN_ID_CAPACITY = 1024

export class WebSocketClient {
  private ws: WebSocket | null = null
  private reconnectTimeout: ReturnType<typeof setTimeout> | null = null
  private messageCallbacks: ((message: ChatMessage) => void)[] = []
  private reconnectAttempts = 0
  private overlayId: string = ''
  private token: string = ''
  private lastSeenTimestamp: number = 0
  // Liveness: browsers never surface protocol ping/pong to JS, so a half-open
  // socket sits in readyState OPEN with onclose never firing. We send our own
  // app-level ping, treat any inbound frame as proof of life, and a watchdog
  // forces a reconnect after prolonged silence. `stopped` latches on
  // disconnect() so nothing re-opens the socket afterwards.
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null
  private watchdogTimer: ReturnType<typeof setInterval> | null = null
  private lastActivity = 0
  private stopped = false
  // Bounded recent-id cache for client-side dedup. Same FIFO eviction as the
  // overlay page. Catches race-window duplicates the server can't fully
  // suppress (broadcast-to-closing-conn followed by replay-buffer write).
  private seenIds: Set<string> = new Set()
  private seenOrder: string[] = []

  private markIdSeen(id: string): boolean {
    if (!id) return false
    if (this.seenIds.has(id)) return true
    this.seenIds.add(id)
    this.seenOrder.push(id)
    if (this.seenOrder.length > SEEN_ID_CAPACITY) {
      const evicted = this.seenOrder.shift()
      if (evicted) this.seenIds.delete(evicted)
    }
    return false
  }

  /**
   * Connect to WebSocket for a specific overlay
   */
  connect(overlayId: string, token?: string | null) {
    this.overlayId = overlayId
    this.token = token ?? ''
    this.stopped = false
    this.clearLivenessTimers()
    this.lastActivity = 0 // no inbound frame yet on this socket

    // Load last seen timestamp from localStorage (survives page reload)
    const storageKey = `ws_last_seen_${overlayId}`
    const storedTimestamp = localStorage.getItem(storageKey)
    if (storedTimestamp) {
      this.lastSeenTimestamp = parseInt(storedTimestamp, 10)
    }

    // Pass ?since= so the server's replay buffer flushes only the messages
    // we haven't already rendered (server uses an exclusive lower bound).
    //
    // SECURITY (audit H5): when a token is provided, it is sent via the
    // WebSocket subprotocol (`bearer.<token>`) instead of the URL query string
    // so it does not leak into access/proxy logs. The gateway extracts the
    // token from the Sec-WebSocket-Protocol header and echoes it back for the
    // handshake. With no token (H3 cookie auth), the owner overlay WS handshake
    // is same-origin, so the browser sends the httpOnly access cookie and the
    // gateway authenticates via CookieToBearer.
    let url = `${WS_URL}/ws/overlay/${overlayId}`
    if (this.lastSeenTimestamp > 0) {
      url += `?since=${this.lastSeenTimestamp}`
    }
    console.log('[WebSocket] Connecting to:', url)

    this.ws = new WebSocket(url, this.token ? [`bearer.${this.token}`] : undefined)

    this.ws.onopen = () => {
      console.log('[WebSocket] Connected')
      this.reconnectAttempts = 0
      this.lastActivity = Date.now()
      this.startLiveness()
      // No in-band replay_request: server handles replay via ?since= during
      // the connect handshake.
    }

    this.ws.onmessage = (event) => {
      this.lastActivity = Date.now() // any inbound frame proves the path is alive
      try {
        const wsMessage: WebSocketMessage = JSON.parse(event.data)

        if (wsMessage.type === 'chat_message' && wsMessage.data) {
          const chat = wsMessage.data as ChatMessage
          if (this.markIdSeen(chat.id)) {
            // Already rendered — drop duplicate.
            return
          }
          // Advance the lastSeen watermark so future reconnects skip this msg.
          const tsMs = Date.parse(chat.timestamp)
          if (Number.isFinite(tsMs) && tsMs > this.lastSeenTimestamp) {
            this.lastSeenTimestamp = tsMs
            try { localStorage.setItem(storageKey, String(tsMs)) } catch {}
          }
          this.messageCallbacks.forEach((cb) => cb(chat))
        } else if (wsMessage.type === 'ping') {
          // Respond to server ping
          this.ws?.send(
            JSON.stringify({
              type: 'pong',
              timestamp: new Date().toISOString(),
            })
          )
        } else if (wsMessage.type === 'error') {
          console.error('[WebSocket] Server error:', wsMessage.error)
        }
        // Note: platform_status messages are not handled here - they're handled in the overlay page component
      } catch (error) {
        console.error('[WebSocket] Failed to parse message:', error)
      }
    }

    this.ws.onerror = (error) => {
      console.error('[WebSocket] Error:', error)
    }

    this.ws.onclose = (event) => {
      console.log('[WebSocket] Closed:', event.code, event.reason)
      this.scheduleReconnect()
    }
  }

  /** Start the heartbeat sender and the silence watchdog (called on open). */
  private startLiveness() {
    this.clearLivenessTimers()
    this.heartbeatTimer = setInterval(() => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        try {
          this.ws.send(JSON.stringify({ type: 'ping', timestamp: new Date().toISOString() }))
        } catch {
          /* a failed send means the socket is gone; the watchdog will catch it */
        }
      }
    }, HEARTBEAT_INTERVAL_MS)
    this.watchdogTimer = setInterval(() => {
      if (isConnectionStale(this.lastActivity, Date.now())) {
        console.warn('[WebSocket] No traffic — connection looks dead, reconnecting')
        this.forceReconnect()
      }
    }, WATCHDOG_INTERVAL_MS)
  }

  private clearLivenessTimers() {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer)
      this.heartbeatTimer = null
    }
    if (this.watchdogTimer) {
      clearInterval(this.watchdogTimer)
      this.watchdogTimer = null
    }
  }

  /** Tear down a half-open socket and reconnect (the watchdog's response). */
  private forceReconnect() {
    if (this.ws) {
      // Detach first so the browser's async onclose can't double-schedule.
      this.ws.onopen = null
      this.ws.onmessage = null
      this.ws.onerror = null
      this.ws.onclose = null
      try {
        this.ws.close()
      } catch {
        /* already closing */
      }
    }
    this.scheduleReconnect()
  }

  /**
   * Schedule a reconnect with capped exponential backoff. Unlike the old
   * client this NEVER permanently gives up — an overlay left running for hours
   * must survive any number of network blips. `reconnectTimeout` guards against
   * double-scheduling when onclose and the watchdog both fire for one socket.
   */
  private scheduleReconnect() {
    if (this.stopped || this.reconnectTimeout) return
    this.clearLivenessTimers()
    const delay = computeBackoffDelay(this.reconnectAttempts)
    console.log(`[WebSocket] Reconnecting in ${Math.round(delay)}ms (attempt ${this.reconnectAttempts + 1})`)
    this.reconnectTimeout = setTimeout(() => {
      this.reconnectTimeout = null
      this.reconnectAttempts++
      this.connect(this.overlayId, this.token)
    }, delay)
  }

  /**
   * Register a callback for new messages
   * Returns an unsubscribe function
   */
  onMessage(callback: (message: ChatMessage) => void): () => void {
    this.messageCallbacks.push(callback)
    return () => {
      this.messageCallbacks = this.messageCallbacks.filter((cb) => cb !== callback)
    }
  }

  /**
   * Disconnect WebSocket and clear reconnection attempts
   */
  disconnect() {
    this.stopped = true
    this.clearLivenessTimers()
    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout)
      this.reconnectTimeout = null
    }
    if (this.ws) {
      // Detach handlers so the close we trigger here can't schedule a reconnect.
      this.ws.onopen = null
      this.ws.onmessage = null
      this.ws.onerror = null
      this.ws.onclose = null
      this.ws.close()
    }
    this.ws = null
    this.messageCallbacks = []
    this.reconnectAttempts = 0
    console.log('[WebSocket] Disconnected')
  }

  /**
   * Check if the WebSocket is currently connected AND live. A half-open socket
   * sits in readyState OPEN with no traffic; treating that as "connected" is
   * exactly the stale-indicator bug, so prolonged silence reads as disconnected.
   */
  isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN && !isConnectionStale(this.lastActivity, Date.now())
  }
}
