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

import { describe, it, expect } from 'vitest'
import type { ChatMessage, NameGradient, PlatformStatus } from '@/lib/types/message'
import {
  advanceWatermark,
  classifyEnvelope,
  computeBackoffDelay,
  HEARTBEAT_INTERVAL_MS,
  isConnectionStale,
  isSourceRecovery,
  LIVENESS_TIMEOUT_MS,
  makeSeenIdCache,
  parseNameGradientGuard,
  platformStatusReducer,
  type PlatformStatusState,
} from '@/lib/utils/overlayStreamCore'

function chat(id: string, ts = '2026-05-30T10:00:00.000Z'): ChatMessage {
  return {
    id,
    overlay_id: 'o1',
    platform: 'twitch',
    channel_id: 'c1',
    channel_name: 'chan',
    user: { id: 'u1', username: 'u', display_name: 'U', badges: [] },
    message: { text: 'hi', emotes: [] },
    timestamp: ts,
    metadata: {},
  }
}

function status(over: Partial<PlatformStatus>): PlatformStatus {
  return { platform: 'twitch', channel_id: 'c1', status: 'connected', ...over }
}

describe('computeBackoffDelay', () => {
  it('uses base delay at attempt 0 plus jitter', () => {
    expect(computeBackoffDelay(0, () => 0)).toBe(1000)
    expect(computeBackoffDelay(0, () => 0.999)).toBeCloseTo(1999, 0)
  })

  it('grows by 1.5^attempts until the 30s cap', () => {
    expect(computeBackoffDelay(1, () => 0)).toBe(1500)
    expect(computeBackoffDelay(2, () => 0)).toBe(2250)
    // 1000 * 1.5^10 ≈ 57665 -> capped at 30000
    expect(computeBackoffDelay(10, () => 0)).toBe(30000)
    expect(computeBackoffDelay(50, () => 0)).toBe(30000)
  })

  it('is monotonic up to the cap with fixed jitter', () => {
    let prev = -1
    for (let n = 0; n <= 9; n++) {
      const d = computeBackoffDelay(n, () => 0)
      expect(d).toBeGreaterThan(prev)
      prev = d
    }
  })

  it('keeps jitter within [0,1000)', () => {
    expect(computeBackoffDelay(0, () => 0)).toBe(1000)
    const hi = computeBackoffDelay(0, () => 0.5)
    expect(hi).toBeGreaterThanOrEqual(1000)
    expect(hi).toBeLessThan(2000)
  })
})

describe('advanceWatermark', () => {
  it('advances on newer messages', () => {
    let last = 0
    last = advanceWatermark(last, '2026-05-15T10:00:00.000Z')
    expect(last).toBe(Date.UTC(2026, 4, 15, 10, 0, 0))
    last = advanceWatermark(last, '2026-05-15T10:00:01.000Z')
    expect(last).toBe(Date.UTC(2026, 4, 15, 10, 0, 1))
  })

  it('does not regress on older or out-of-order messages', () => {
    const last = Date.UTC(2026, 4, 15, 10, 0, 5)
    expect(advanceWatermark(last, '2026-05-15T10:00:00.000Z')).toBe(last)
  })

  it('ignores malformed timestamps', () => {
    expect(advanceWatermark(1000, 'not-a-date')).toBe(1000)
  })
})

describe('makeSeenIdCache', () => {
  it('returns false on first sight and true on duplicate', () => {
    const cache = makeSeenIdCache(1024)
    expect(cache.markSeen('msg-1')).toBe(false)
    expect(cache.markSeen('msg-1')).toBe(true)
  })

  it('treats empty id as never-seen so empty ids never dedup', () => {
    const cache = makeSeenIdCache(1024)
    expect(cache.markSeen('')).toBe(false)
    expect(cache.markSeen('')).toBe(false)
  })

  it('evicts oldest id once capacity is exceeded', () => {
    const cache = makeSeenIdCache(3)
    cache.markSeen('a')
    cache.markSeen('b')
    cache.markSeen('c')
    cache.markSeen('d') // evicts 'a' (oldest)
    expect(cache.size()).toBe(3)
    // Check survivors first: markSeen() re-records on a miss, so probing the
    // evicted id mutates the cache — assert survivors before the evicted id.
    expect(cache.markSeen('b')).toBe(true)
    expect(cache.markSeen('c')).toBe(true)
    expect(cache.markSeen('d')).toBe(true)
    expect(cache.markSeen('a')).toBe(false) // 'a' was evicted, reappears as new
  })

  it('keeps only the most recent ids under heavy streaming', () => {
    const cache = makeSeenIdCache(4)
    for (let i = 0; i < 100; i++) cache.markSeen(`id-${i}`)
    expect(cache.size()).toBe(4)
    expect(cache.markSeen('id-99')).toBe(true)
    expect(cache.markSeen('id-0')).toBe(false)
  })
})

describe('parseNameGradientGuard', () => {
  it('converts a JSON string to a NameGradient object', () => {
    const gradient: NameGradient = { type: 'linear', colors: ['#9146ff', '#00b5ad'], angle: 90 }
    const user: { name_gradient?: NameGradient | string } = { name_gradient: JSON.stringify(gradient) }
    parseNameGradientGuard(user)
    expect(typeof user.name_gradient).toBe('object')
    expect((user.name_gradient as NameGradient).colors).toEqual(['#9146ff', '#00b5ad'])
    expect((user.name_gradient as NameGradient).angle).toBe(90)
  })

  it('leaves a NameGradient object unchanged (same reference)', () => {
    const gradient: NameGradient = { type: 'linear', colors: ['#ff0000', '#0000ff'], angle: 45 }
    const user: { name_gradient?: NameGradient | string } = { name_gradient: gradient }
    parseNameGradientGuard(user)
    expect(user.name_gradient).toBe(gradient)
  })

  it('leaves undefined unchanged', () => {
    const user: { name_gradient?: NameGradient | string } = {}
    parseNameGradientGuard(user)
    expect(user.name_gradient).toBeUndefined()
  })
})

describe('platformStatusReducer', () => {
  const empty: PlatformStatusState = { activeChannels: new Set(), channelStatuses: new Map() }
  const sources = new Map<string, unknown>([['c1', {}]])

  it('accepts any status while configured sources is empty (config not loaded yet)', () => {
    const next = platformStatusReducer(empty, status({ channel_id: 'unknown', status: 'connected' }), new Map())
    expect(next.activeChannels.has('unknown')).toBe(true)
  })

  it('rejects status for a channel not configured once sources are known', () => {
    const next = platformStatusReducer(empty, status({ channel_id: 'nope', status: 'connected' }), sources)
    expect(next).toBe(empty) // same reference — ignored
  })

  it('adds connected channels to activeChannels', () => {
    const next = platformStatusReducer(empty, status({ status: 'connected' }), sources)
    expect(next.activeChannels.has('c1')).toBe(true)
    expect(next.channelStatuses.get('c1')?.status).toBe('connected')
  })

  it('removes a channel from activeChannels when it goes offline', () => {
    const connected = platformStatusReducer(empty, status({ status: 'connected' }), sources)
    const offline = platformStatusReducer(connected, status({ status: 'offline' }), sources)
    expect(offline.activeChannels.has('c1')).toBe(false)
    expect(offline.channelStatuses.get('c1')?.status).toBe('offline')
  })

  it('does not overwrite a connected status with reconnecting', () => {
    const connected = platformStatusReducer(empty, status({ status: 'connected' }), sources)
    const next = platformStatusReducer(connected, status({ status: 'reconnecting' }), sources)
    expect(next.channelStatuses.get('c1')?.status).toBe('connected')
    // activeChannels unchanged, channelStatuses unchanged -> same reference
    expect(next).toBe(connected)
  })

  it('reconnecting from a fresh channel records the status without activating it', () => {
    const next = platformStatusReducer(empty, status({ status: 'reconnecting' }), sources)
    expect(next.channelStatuses.get('c1')?.status).toBe('reconnecting')
    expect(next.activeChannels.has('c1')).toBe(false)
  })

  it('falls back to platform as the channel key when channel_id is empty', () => {
    const next = platformStatusReducer(empty, status({ channel_id: '', platform: 'kick', status: 'connected' }), new Map())
    expect(next.activeChannels.has('kick')).toBe(true)
  })
})

describe('classifyEnvelope', () => {
  it('ignores null / typeless envelopes', () => {
    expect(classifyEnvelope(null).kind).toBe('ignore')
    expect(classifyEnvelope({}).kind).toBe('ignore')
  })

  it('classifies replay_response as replay with the deletion array', () => {
    const res = classifyEnvelope({ type: 'replay_response', data: [{ deletion_type: 'clear' }] })
    expect(res.kind).toBe('replay')
    if (res.kind === 'replay') expect(res.deletions).toHaveLength(1)
  })

  it('detects message_deletion before regular chat', () => {
    const env = {
      type: 'chat_message',
      data: { ...chat('m1'), event: { type: 'message_deletion', tier: 'low', duration: 0, is_update: false, metadata: { deletion_type: 'single', target_uuid: 'm0' } } },
    } as never
    const res = classifyEnvelope(env)
    expect(res.kind).toBe('deletion')
    if (res.kind === 'deletion') expect(res.deletion.target_uuid).toBe('m0')
  })

  it('classifies a regular chat message', () => {
    const res = classifyEnvelope({ type: 'chat_message', data: chat('m2') })
    expect(res.kind).toBe('chat')
    if (res.kind === 'chat') expect(res.message.id).toBe('m2')
  })

  it('classifies message_update', () => {
    const res = classifyEnvelope({ type: 'message_update', data: chat('m3') })
    expect(res.kind).toBe('update')
  })

  it('classifies platform_status', () => {
    const res = classifyEnvelope({ type: 'platform_status', data: status({ status: 'connected' }) })
    expect(res.kind).toBe('status')
  })

  it('ignores unknown types and empty data', () => {
    expect(classifyEnvelope({ type: 'pong' }).kind).toBe('ignore')
    expect(classifyEnvelope({ type: 'chat_message', data: null }).kind).toBe('ignore')
  })
})

describe('isConnectionStale / heartbeat constants', () => {
  it('exposes sane heartbeat timings (probe faster than the timeout, both < server 60s PongWait)', () => {
    expect(HEARTBEAT_INTERVAL_MS).toBeGreaterThan(0)
    // Must tolerate at least one missed probe before declaring death.
    expect(LIVENESS_TIMEOUT_MS).toBeGreaterThan(HEARTBEAT_INTERVAL_MS)
    // Detect before the gateway's own 60s read deadline so the client drives recovery.
    expect(LIVENESS_TIMEOUT_MS).toBeLessThan(60000)
  })

  it('is not stale before any activity has been recorded (lastActivity <= 0)', () => {
    expect(isConnectionStale(0, 999999)).toBe(false)
  })

  it('is not stale while inbound traffic is recent', () => {
    const now = 1_000_000
    expect(isConnectionStale(now - (LIVENESS_TIMEOUT_MS - 1), now)).toBe(false)
  })

  it('is stale once inbound silence exceeds the timeout', () => {
    const now = 1_000_000
    expect(isConnectionStale(now - (LIVENESS_TIMEOUT_MS + 1), now)).toBe(true)
  })

  it('honors an explicit timeout override', () => {
    expect(isConnectionStale(100, 100 + 5001, 5000)).toBe(true)
    expect(isConnectionStale(100, 100 + 4999, 5000)).toBe(false)
  })
})

describe('isSourceRecovery', () => {
  it('is NOT a recovery on the first-ever status (no prior recorded)', () => {
    // The initial connect must not trigger a spurious replay reconnect.
    expect(isSourceRecovery(undefined, 'connected')).toBe(false)
  })

  it('is a recovery when a source returns to connected from any down state', () => {
    for (const down of ['offline', 'error', 'paused', 'reconnecting']) {
      expect(isSourceRecovery(down, 'connected')).toBe(true)
    }
  })

  it('is NOT a recovery while staying connected (steady-state heartbeats)', () => {
    expect(isSourceRecovery('connected', 'connected')).toBe(false)
  })

  it('is NOT a recovery when a source goes down', () => {
    expect(isSourceRecovery('connected', 'offline')).toBe(false)
    expect(isSourceRecovery('connected', 'reconnecting')).toBe(false)
  })
})
