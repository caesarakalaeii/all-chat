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
 * Pure, React-free building blocks for the overlay realtime stream.
 *
 * These were extracted from the inline logic in `app/overlay/[id]/page.tsx` so
 * that the OBS overlay page and the new observability view (`.../view`) can
 * share one battle-tested implementation through `useOverlayStream`, and so the
 * subtle reconnect / replay / dedup / platform-status rules can be unit-tested
 * without a WebSocket. Keep this file free of React and browser globals.
 */

import type {
  ChatMessage,
  DeletionMetadata,
  NameGradient,
  PlatformStatus,
} from '@/lib/types/message'

// ---------------------------------------------------------------------------
// Reconnect backoff
// ---------------------------------------------------------------------------

/**
 * Exponential backoff with jitter, identical to the overlay page's onclose
 * handler: min(1000 * 1.5^attempts, 30000) + [0,1000) ms of jitter. `rng` is
 * injectable so tests can pin the jitter.
 */
export function computeBackoffDelay(attempts: number, rng: () => number = Math.random): number {
  const baseDelay = 1000
  const maxDelay = 30000
  const jitter = rng() * 1000
  return Math.min(baseDelay * Math.pow(1.5, attempts), maxDelay) + jitter
}

// ---------------------------------------------------------------------------
// Liveness / heartbeat
// ---------------------------------------------------------------------------
//
// Browsers never surface the WebSocket's protocol-level ping/pong to JS, so a
// half-open connection (Wi-Fi blip, NAT/proxy idle timeout, sleep/wake) leaves
// `readyState === OPEN` with `onclose` never firing — messages silently stop
// and the UI keeps claiming "connected". To detect that, the client sends its
// own app-level `ping` and watches for ANY inbound traffic; prolonged silence
// means the path is dead and we must force a reconnect.

/** How often the client sends an app-level `ping` liveness probe. */
export const HEARTBEAT_INTERVAL_MS = 20000

/**
 * How long the client tolerates total inbound silence before declaring the
 * socket dead. Must exceed HEARTBEAT_INTERVAL_MS (so a single missed probe
 * doesn't trip it) and stay under the gateway's 60s PongWait read deadline so
 * the client, not the server, drives recovery (and the `?since=` chat replay).
 */
export const LIVENESS_TIMEOUT_MS = 40000

/** How often the watchdog re-evaluates staleness (bounds detection latency). */
export const WATCHDOG_INTERVAL_MS = 5000

/**
 * True when the connection has gone silent for longer than `timeoutMs`. A
 * non-positive `lastActivityMs` means "no inbound traffic recorded yet" (the
 * socket is still opening), which is never stale.
 */
export function isConnectionStale(
  lastActivityMs: number,
  nowMs: number,
  timeoutMs: number = LIVENESS_TIMEOUT_MS,
): boolean {
  if (lastActivityMs <= 0) return false
  return nowMs - lastActivityMs > timeoutMs
}

// ---------------------------------------------------------------------------
// Replay watermark (?since=)
// ---------------------------------------------------------------------------

/**
 * Advance the last-seen watermark from a message timestamp. Newer wins; older,
 * out-of-order, or malformed timestamps leave it unchanged.
 */
export function advanceWatermark(current: number, timestampIso: string): number {
  const tsMs = Date.parse(timestampIso)
  if (Number.isFinite(tsMs) && tsMs > current) return tsMs
  return current
}

// ---------------------------------------------------------------------------
// Bounded seen-id dedup cache
// ---------------------------------------------------------------------------

export interface SeenIdCache {
  /** Returns true if this id was already seen; records it otherwise. Empty ids never dedup. */
  markSeen(id: string): boolean
  size(): number
}

/**
 * FIFO-evicting set of recently-seen message ids. Guards against the rare race
 * where a message is both broadcast live and replayed on reconnect.
 */
export function makeSeenIdCache(capacity: number): SeenIdCache {
  const set = new Set<string>()
  const order: string[] = []
  return {
    markSeen(id: string): boolean {
      if (!id) return false // can't dedup without an id
      if (set.has(id)) return true
      set.add(id)
      order.push(id)
      if (order.length > capacity) {
        const evicted = order.shift()
        if (evicted) set.delete(evicted)
      }
      return false
    },
    size(): number {
      return set.size
    },
  }
}

// ---------------------------------------------------------------------------
// name_gradient parse guard
// ---------------------------------------------------------------------------

/**
 * Server may send `name_gradient` as a JSON string; parse it in place to a
 * NameGradient object. Objects and undefined are left untouched (no re-parse).
 */
export function parseNameGradientGuard(user: { name_gradient?: NameGradient | string }): void {
  if (user?.name_gradient && typeof user.name_gradient === 'string') {
    user.name_gradient = JSON.parse(user.name_gradient as unknown as string) as NameGradient
  }
}

// ---------------------------------------------------------------------------
// platform_status reducer
// ---------------------------------------------------------------------------

export interface PlatformStatusState {
  activeChannels: Set<string>
  channelStatuses: Map<string, PlatformStatus>
}

/**
 * Apply a platform_status update, mirroring the overlay page exactly:
 *  - Gate by configured sources: accept when the map is empty (config not yet
 *    loaded) or the channel is configured; otherwise ignore.
 *  - `connected` adds to activeChannels; `offline` removes from it.
 *  - Never overwrite a `connected` status with a `reconnecting` one.
 * Returns the SAME state reference when nothing changes, so React consumers can
 * skip re-renders.
 */
export function platformStatusReducer(
  state: PlatformStatusState,
  statusData: PlatformStatus,
  configuredSources: ReadonlyMap<string, unknown>,
): PlatformStatusState {
  const channelId = statusData.channel_id || statusData.platform
  const isConfigured = configuredSources.size === 0 || configuredSources.has(channelId)
  if (!isConfigured) return state

  let activeChannels = state.activeChannels
  if (statusData.status === 'connected') {
    if (!activeChannels.has(channelId)) {
      activeChannels = new Set(activeChannels)
      activeChannels.add(channelId)
    }
  } else if (
    statusData.status === 'offline' ||
    statusData.status === 'error' ||
    statusData.status === 'paused'
  ) {
    if (activeChannels.has(channelId)) {
      activeChannels = new Set(activeChannels)
      activeChannels.delete(channelId)
    }
  }

  let channelStatuses = state.channelStatuses
  const existing = state.channelStatuses.get(channelId)
  // Don't overwrite connected with reconnecting from a different channel.
  if (!(existing?.status === 'connected' && statusData.status === 'reconnecting')) {
    channelStatuses = new Map(channelStatuses)
    channelStatuses.set(channelId, statusData)
  }

  if (activeChannels === state.activeChannels && channelStatuses === state.channelStatuses) {
    return state
  }
  return { activeChannels, channelStatuses }
}

// ---------------------------------------------------------------------------
// Envelope classification
// ---------------------------------------------------------------------------

export interface WsEnvelope {
  type?: string
  data?: ChatMessage | PlatformStatus | DeletionMetadata[] | null
  timestamp?: string
  error?: string
}

export type EnvelopeClassification =
  | { kind: 'replay'; deletions: DeletionMetadata[] }
  | { kind: 'deletion'; deletion: DeletionMetadata }
  | { kind: 'chat'; message: ChatMessage }
  | { kind: 'update'; message: ChatMessage }
  | { kind: 'status'; status: PlatformStatus }
  | { kind: 'ignore' }

/**
 * Classify a parsed WebSocket envelope into the action a consumer should take.
 * Mirrors the dispatch order of the overlay page's onmessage handler: deletion
 * envelopes are detected before regular chat.
 */
export function classifyEnvelope(envelope: WsEnvelope | null | undefined): EnvelopeClassification {
  if (!envelope || typeof envelope.type !== 'string') return { kind: 'ignore' }

  switch (envelope.type) {
    case 'replay_response':
      return {
        kind: 'replay',
        deletions: Array.isArray(envelope.data) ? (envelope.data as DeletionMetadata[]) : [],
      }
    case 'chat_message': {
      const data = envelope.data as ChatMessage | undefined
      if (!data) return { kind: 'ignore' }
      if (data.event?.type === 'message_deletion') {
        return { kind: 'deletion', deletion: data.event.metadata as unknown as DeletionMetadata }
      }
      return { kind: 'chat', message: data }
    }
    case 'message_update': {
      const data = envelope.data as ChatMessage | undefined
      if (!data) return { kind: 'ignore' }
      return { kind: 'update', message: data }
    }
    case 'platform_status': {
      const data = envelope.data as PlatformStatus | undefined
      if (!data) return { kind: 'ignore' }
      return { kind: 'status', status: data }
    }
    default:
      return { kind: 'ignore' }
  }
}
