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

'use client'

/**
 * useNotificationSocket — an authenticated, self-healing realtime socket for
 * owner-facing overlay notifications on the dashboard (e.g. `share_revoked`).
 *
 * Same liveness contract as `useOverlayStream` and the legacy `WebSocketClient`:
 * browsers never surface the WebSocket protocol-level ping/pong to JS, so a
 * half-open path (Wi-Fi blip, NAT/proxy idle timeout, sleep/wake, background-tab
 * throttling) leaves `readyState === OPEN` with `onclose` never firing — frames
 * silently stop arriving and the page is none the wiser. We send an app-level
 * `ping` the gateway echoes as a `pong`, treat ANY inbound frame as proof of
 * life, and a watchdog forces a reconnect after prolonged silence.
 *
 * The inline socket this replaced had NO `onclose` handler and NO reconnect at
 * all, so a single drop — clean or half-open — killed realtime notifications
 * for the whole session until a manual reload. This hook reconnects with capped
 * exponential backoff and never permanently gives up.
 */

import { useEffect, useRef } from 'react'

import {
  computeBackoffDelay,
  HEARTBEAT_INTERVAL_MS,
  isConnectionStale,
  WATCHDOG_INTERVAL_MS,
} from '@/lib/utils/overlayStreamCore'

export interface NotificationEnvelope {
  type?: string
  data?: unknown
}

/**
 * Subscribe to owner notifications for `id` on the authenticated overlay socket.
 * `onEnvelope` is invoked with each parsed inbound envelope (parse failures are
 * swallowed). The socket is torn down and rebuilt only when `id` or `token`
 * change; the callback is kept fresh via a ref so updating it never re-subscribes.
 */
export function useNotificationSocket(
  id: string | null | undefined,
  token: string | null | undefined,
  onEnvelope: (envelope: NotificationEnvelope) => void,
): void {
  const onEnvelopeRef = useRef(onEnvelope)
  useEffect(() => {
    onEnvelopeRef.current = onEnvelope
  })

  useEffect(() => {
    if (!id || !token) return

    let stopped = false
    let ws: WebSocket | null = null
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null
    let heartbeatTimer: ReturnType<typeof setInterval> | null = null
    let watchdogTimer: ReturnType<typeof setInterval> | null = null
    // Consecutive failed reconnects since the last successful open — drives the
    // backoff curve; reset to 0 on every successful handshake.
    let attempts = 0
    // Wall-clock of the last inbound frame on the current socket; 0 means "no
    // frame yet" (socket still opening), which isConnectionStale treats as live.
    let lastActivity = 0

    const clearLivenessTimers = () => {
      if (heartbeatTimer) {
        clearInterval(heartbeatTimer)
        heartbeatTimer = null
      }
      if (watchdogTimer) {
        clearInterval(watchdogTimer)
        watchdogTimer = null
      }
    }

    // Detach handlers before closing so the browser's asynchronous onclose
    // (fired after close()) can't re-enter and schedule a second reconnect.
    const teardown = (sock: WebSocket | null) => {
      if (!sock) return
      sock.onopen = null
      sock.onmessage = null
      sock.onerror = null
      sock.onclose = null
      try {
        sock.close()
      } catch {
        /* already closing */
      }
    }

    // `immediate` (online/visibility recovery) collapses the backoff and resets
    // the curve so a returning tab reconnects at once.
    const scheduleReconnect = (immediate = false) => {
      if (stopped || reconnectTimer) return
      clearLivenessTimers()
      const delay = immediate ? 0 : computeBackoffDelay(attempts)
      if (immediate) attempts = 0
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null
        attempts++
        connect()
      }, delay)
    }

    const connect = () => {
      if (stopped) return
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const wsUrl = `${protocol}//${window.location.host}/ws/overlay/${id}`
      const sock = new WebSocket(wsUrl, [`bearer.${token}`])
      ws = sock
      lastActivity = 0 // no inbound frame yet on this socket

      sock.onopen = () => {
        attempts = 0 // success resets the backoff curve for the next drop
        lastActivity = Date.now()
        clearLivenessTimers()

        // Heartbeat: an app-level ping the gateway echoes as a pong. That pong
        // (or any other inbound frame) refreshes lastActivity below.
        heartbeatTimer = setInterval(() => {
          if (sock.readyState === WebSocket.OPEN) {
            try {
              sock.send(JSON.stringify({ type: 'ping', timestamp: new Date().toISOString() }))
            } catch {
              /* a failed send means the socket is gone; the watchdog will catch it */
            }
          }
        }, HEARTBEAT_INTERVAL_MS)

        // Watchdog: prolonged inbound silence means a half-open socket (no
        // onclose). Force the reconnect ourselves.
        watchdogTimer = setInterval(() => {
          if (isConnectionStale(lastActivity, Date.now())) {
            teardown(sock)
            scheduleReconnect()
          }
        }, WATCHDOG_INTERVAL_MS)
      }

      sock.onmessage = (event) => {
        lastActivity = Date.now() // ANY inbound frame proves liveness
        try {
          onEnvelopeRef.current(JSON.parse(event.data) as NotificationEnvelope)
        } catch {
          /* ignore parse errors — malformed frame still counts as liveness */
        }
      }

      sock.onerror = () => {
        /* surfaced via onclose / the watchdog; nothing actionable here */
      }

      sock.onclose = () => {
        scheduleReconnect()
      }
    }

    connect()

    // Recover instantly when the OS reports connectivity or a backgrounded tab
    // is refocused — a throttled background tab's watchdog timer is also
    // throttled, so without this a dead socket lingers until the tab is active.
    const reconnectIfDead = () => {
      if (!ws || ws.readyState === WebSocket.CONNECTING) return
      const healthy =
        ws.readyState === WebSocket.OPEN && !isConnectionStale(lastActivity, Date.now())
      if (healthy) return
      teardown(ws)
      scheduleReconnect(true)
    }
    const onVisible = () => {
      if (document.visibilityState === 'visible') reconnectIfDead()
    }
    window.addEventListener('online', reconnectIfDead)
    document.addEventListener('visibilitychange', onVisible)

    return () => {
      stopped = true
      clearLivenessTimers()
      if (reconnectTimer) {
        clearTimeout(reconnectTimer)
        reconnectTimer = null
      }
      window.removeEventListener('online', reconnectIfDead)
      document.removeEventListener('visibilitychange', onVisible)
      teardown(ws)
      ws = null
    }
  }, [id, token])
}
