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
 * useOverlayStream — the shared realtime pipeline for a public overlay.
 *
 * Extracted from `app/overlay/[id]/page.tsx` so the OBS overlay and the
 * observability view (`.../view`) share one implementation of the subtle
 * connection rules: WebSocket lifecycle, exponential-backoff reconnect, the
 * `?since=` replay watermark (persisted to localStorage), id dedup, message
 * enrichment, the public-config fetch (+30s refresh) and platform_status state.
 *
 * The hook deliberately does NOT own the message array, filtering, fade, sound
 * or TTS — those policies differ per consumer. Instead it pushes enriched
 * messages out via callbacks (`onChat`/`onMessageUpdate`/`onDeletion`) and the
 * consumer applies them to its own state.
 */

import { useEffect, useRef, useState } from 'react'

import { sortMessageBadges } from '@/lib/badgeOrder'
import { resolveTwitchBadgeIcons } from '@/lib/twitchBadges'
import type { ChatMessage, DeletionMetadata, PlatformStatus } from '@/lib/types/message'
import type { PublicOverlayConfig } from '@/lib/types/overlay'
import {
  advanceWatermark,
  classifyEnvelope,
  computeBackoffDelay,
  HEARTBEAT_INTERVAL_MS,
  isConnectionStale,
  LIVENESS_TIMEOUT_MS,
  makeSeenIdCache,
  parseNameGradientGuard,
  platformStatusReducer,
  WATCHDOG_INTERVAL_MS,
  type PlatformStatusState,
} from '@/lib/utils/overlayStreamCore'
import type { SourceInfo } from '@/components/PlatformStatusIndicators'

export type ConnectionStatus = 'connecting' | 'open' | 'reconnecting'

export interface UseOverlayStreamOptions {
  /** Enriched, deduped regular chat message (no event, or a non-deletion event). */
  onChat?: (message: ChatMessage) => void
  /** Enriched TikTok like-aggregate update (no dedup — may repeat by aggregation_id). */
  onMessageUpdate?: (message: ChatMessage) => void
  /** A deletion to apply. `source` is 'replay' for reconnect-buffered deletions, 'live' otherwise. */
  onDeletion?: (deletion: DeletionMetadata, source: 'replay' | 'live') => void
  /** Fired once per successful (re)connection. */
  onConnected?: () => void
}

export interface UseOverlayStreamResult {
  /** Raw public config JSON (display/filter/visual/sources/emote set). null until first load. */
  config: PublicOverlayConfig | null
  /** channel_id -> SourceInfo, from the overlay's configured sources. */
  sources: Map<string, SourceInfo>
  /** channel_ids currently reporting 'connected'. */
  activeChannels: Set<string>
  /** channel_id -> latest PlatformStatus. */
  channelStatuses: Map<string, PlatformStatus>
  connectionStatus: ConnectionStatus
  reconnectAttempts: number
}

const SEEN_ID_CAPACITY = 1024

export function useOverlayStream(
  id: string,
  options: UseOverlayStreamOptions,
): UseOverlayStreamResult {
  const [config, setConfig] = useState<PublicOverlayConfig | null>(null)
  const [sources, setSources] = useState<Map<string, SourceInfo>>(new Map())
  const [platformState, setPlatformState] = useState<PlatformStatusState>({
    activeChannels: new Set(),
    channelStatuses: new Map(),
  })
  const [reconnectAttempts, setReconnectAttempts] = useState(0)
  const [forceReconnect, setForceReconnect] = useState(0)
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('connecting')

  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const lastSeenTimestampRef = useRef<number>(0)
  const seenCacheRef = useRef(makeSeenIdCache(SEEN_ID_CAPACITY))
  // Liveness state: the heartbeat sender, the silence watchdog, the timestamp
  // of the last inbound frame, and a handle to the current connection's
  // reconnect trigger (so window-level online/visibility handlers can poke it).
  const heartbeatRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const watchdogRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const lastActivityRef = useRef<number>(0)
  const requestReconnectRef = useRef<((immediate?: boolean) => void) | null>(null)
  // Consecutive failed reconnects since the last successful open. The source of
  // truth for both the backoff curve and the `reconnectAttempts` render state —
  // a ref (not the captured state) so a successful open resets it for the next
  // drop instead of escalating from wherever the last burst left off.
  const attemptsRef = useRef(0)

  // Keep callbacks fresh without re-subscribing the socket (mirrors the
  // filterSettingsRef/maxMessagesRef trick the overlay page used inline).
  const optsRef = useRef(options)
  useEffect(() => {
    optsRef.current = options
  })

  // Load public overlay config on mount, then refresh every 30s. Only the raw
  // config + the sources map live here; interpreting config into display state
  // is the consumer's job (the overlay and the view want very different things).
  useEffect(() => {
    let cancelled = false
    const loadConfig = async () => {
      try {
        const response = await fetch(`/api/v1/overlays/public/${id}/config`)
        if (!response.ok) throw new Error('failed to load config')
        const data = (await response.json()) as PublicOverlayConfig
        if (cancelled) return
        setConfig(data)
        if (Array.isArray(data.sources)) {
          const next = new Map<string, SourceInfo>()
          data.sources.forEach((source) => {
            next.set(source.channel_id, {
              platform: source.platform,
              channelId: source.channel_id,
              channelName: source.channel_name || source.channel_id,
            })
          })
          setSources(next)
        }
      } catch (error) {
        console.warn('[useOverlayStream] Failed to load config', error)
      }
    }
    loadConfig()
    const interval = setInterval(loadConfig, 30000)
    return () => {
      cancelled = true
      clearInterval(interval)
    }
  }, [id])

  // WebSocket lifecycle. Deps are intentionally just [id, forceReconnect]: the
  // socket re-binds only when the overlay changes or a scheduled reconnect
  // fires. `sources` is captured at bind time on purpose — re-reading it live
  // would change the documented behavior (e.g. the "accept all while sources is
  // empty" config-load race). Reconnect/liveness bookkeeping lives in refs so it
  // survives across binds without forcing a re-subscribe.
  useEffect(() => {
    const storageKey = `ws_last_seen_${id}`
    const storedTimestamp = localStorage.getItem(storageKey)
    if (storedTimestamp) {
      lastSeenTimestampRef.current = parseInt(storedTimestamp, 10)
    }

    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    let wsUrl = `${protocol}//${window.location.host}/ws/overlay/${id}`
    if (lastSeenTimestampRef.current > 0) {
      wsUrl += `?since=${lastSeenTimestampRef.current}`
    }

    setConnectionStatus(attemptsRef.current > 0 ? 'reconnecting' : 'connecting')

    const persist = (ms: number) => {
      try {
        localStorage.setItem(storageKey, String(ms))
      } catch {
        /* storage may be unavailable; replay still works in-session */
      }
    }

    const ws = new WebSocket(wsUrl)
    wsRef.current = ws
    lastActivityRef.current = 0 // no inbound frame yet on this socket

    const clearLivenessTimers = () => {
      if (heartbeatRef.current) {
        clearInterval(heartbeatRef.current)
        heartbeatRef.current = null
      }
      if (watchdogRef.current) {
        clearInterval(watchdogRef.current)
        watchdogRef.current = null
      }
    }

    const fireReconnect = () => {
      reconnectTimeoutRef.current = null
      setReconnectAttempts(attemptsRef.current) // mirror the ref into render state
      setForceReconnect(Date.now())
    }

    // Single funnel for every "this socket is gone, reconnect" trigger: the
    // server close, the silence watchdog, and the online/visibility handlers.
    // `scheduled` is per-socket so each connection schedules exactly one
    // reconnect, no matter how many triggers fire.
    let scheduled = false
    const requestReconnect = (immediate = false) => {
      if (scheduled) {
        // Already tearing this socket down. A fresh "network is back" / "tab is
        // visible" signal collapses any remaining backoff so recovery is instant.
        if (immediate && reconnectTimeoutRef.current) {
          clearTimeout(reconnectTimeoutRef.current)
          reconnectTimeoutRef.current = setTimeout(fireReconnect, 0)
        }
        return
      }
      scheduled = true
      clearLivenessTimers()
      // Detach handlers before closing so the browser's asynchronous onclose
      // (fired after close()) can't re-enter and schedule a second reconnect.
      ws.onopen = null
      ws.onmessage = null
      ws.onerror = null
      ws.onclose = null
      try {
        ws.close()
      } catch {
        /* already closing */
      }
      setConnectionStatus('reconnecting')
      // immediate (online/visibility) retries now and restarts the backoff curve;
      // a normal drop backs off from the current attempt count, then increments.
      const delay = immediate ? 0 : computeBackoffDelay(attemptsRef.current)
      attemptsRef.current = immediate ? 1 : attemptsRef.current + 1
      if (reconnectTimeoutRef.current) clearTimeout(reconnectTimeoutRef.current)
      reconnectTimeoutRef.current = setTimeout(fireReconnect, delay)
    }
    requestReconnectRef.current = requestReconnect

    ws.onopen = () => {
      lastActivityRef.current = Date.now()
      attemptsRef.current = 0 // success resets the backoff curve for the next drop
      setReconnectAttempts(0)
      setConnectionStatus('open')
      optsRef.current.onConnected?.()

      // Heartbeat: send an app-level ping the gateway echoes as a pong. That
      // pong (or any chat/status frame) refreshes lastActivity below.
      heartbeatRef.current = setInterval(() => {
        if (ws.readyState === WebSocket.OPEN) {
          try {
            ws.send(JSON.stringify({ type: 'ping', timestamp: new Date().toISOString() }))
          } catch {
            /* a failed send means the socket is gone; the watchdog will catch it */
          }
        }
      }, HEARTBEAT_INTERVAL_MS)

      // Watchdog: prolonged inbound silence means the path died without an
      // onclose (half-open socket). Force the reconnect ourselves.
      watchdogRef.current = setInterval(() => {
        if (isConnectionStale(lastActivityRef.current, Date.now())) {
          requestReconnect(false)
        }
      }, WATCHDOG_INTERVAL_MS)
    }

    ws.onmessage = async (event) => {
      lastActivityRef.current = Date.now() // ANY inbound frame proves liveness
      try {
        const envelope = JSON.parse(event.data)
        const classified = classifyEnvelope(envelope)

        switch (classified.kind) {
          case 'replay':
            classified.deletions.forEach((deletion) =>
              optsRef.current.onDeletion?.(deletion, 'replay'),
            )
            return

          case 'deletion':
            // Deletions advance the watermark by wall-clock (they carry no
            // message timestamp), matching the original overlay behavior.
            lastSeenTimestampRef.current = Date.now()
            persist(lastSeenTimestampRef.current)
            optsRef.current.onDeletion?.(classified.deletion, 'live')
            return

          case 'chat': {
            let message = classified.message
            if (seenCacheRef.current.markSeen(message.id)) return // drop duplicate
            const advanced = advanceWatermark(lastSeenTimestampRef.current, message.timestamp)
            if (advanced !== lastSeenTimestampRef.current) {
              lastSeenTimestampRef.current = advanced
              persist(advanced)
            }
            message = await resolveTwitchBadgeIcons(message)
            message = sortMessageBadges(message)
            parseNameGradientGuard(message.user)
            optsRef.current.onChat?.(message)
            return
          }

          case 'update': {
            // No dedup for aggregate updates (they intentionally repeat by id).
            let message = classified.message
            const advanced = advanceWatermark(lastSeenTimestampRef.current, message.timestamp)
            if (advanced !== lastSeenTimestampRef.current) {
              lastSeenTimestampRef.current = advanced
              persist(advanced)
            }
            message = await resolveTwitchBadgeIcons(message)
            message = sortMessageBadges(message)
            parseNameGradientGuard(message.user)
            optsRef.current.onMessageUpdate?.(message)
            return
          }

          case 'status':
            setPlatformState((prev) => platformStatusReducer(prev, classified.status, sources))
            return

          case 'ignore':
            return
        }
      } catch (error) {
        console.error('[useOverlayStream] Failed to parse message:', error)
      }
    }

    ws.onerror = (error) => {
      console.error('[useOverlayStream] WebSocket error:', error)
    }

    ws.onclose = () => {
      requestReconnect(false)
    }

    return () => {
      clearLivenessTimers()
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current)
        reconnectTimeoutRef.current = null
      }
      if (requestReconnectRef.current === requestReconnect) {
        requestReconnectRef.current = null
      }
      // Detach so a post-unmount async onclose can't schedule a stray reconnect.
      ws.onopen = null
      ws.onmessage = null
      ws.onerror = null
      ws.onclose = null
      if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
        ws.close()
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- `sources` captured at bind time by design; reconnect bookkeeping lives in refs
  }, [id, forceReconnect])

  // Recover instantly when the environment signals the network is usable again:
  // the OS reports connectivity (`online`) or the streamer refocuses a
  // background tab (`visibilitychange`). A backgrounded tab is throttled, so its
  // socket can die long before the watchdog timer (also throttled) would notice;
  // these events let us reconnect the moment the tab is interactive again.
  // Only references stable refs, so it binds once for the hook's lifetime.
  useEffect(() => {
    const reconnectIfDead = () => {
      const ws = wsRef.current
      if (!ws || ws.readyState === WebSocket.CONNECTING) return // nothing to do / already trying
      const healthy =
        ws.readyState === WebSocket.OPEN &&
        !isConnectionStale(lastActivityRef.current, Date.now())
      if (healthy) return
      requestReconnectRef.current?.(true)
    }
    const onVisible = () => {
      if (document.visibilityState === 'visible') reconnectIfDead()
    }
    window.addEventListener('online', reconnectIfDead)
    document.addEventListener('visibilitychange', onVisible)
    return () => {
      window.removeEventListener('online', reconnectIfDead)
      document.removeEventListener('visibilitychange', onVisible)
    }
  }, [])

  return {
    config,
    sources,
    activeChannels: platformState.activeChannels,
    channelStatuses: platformState.channelStatuses,
    connectionStatus,
    reconnectAttempts,
  }
}
