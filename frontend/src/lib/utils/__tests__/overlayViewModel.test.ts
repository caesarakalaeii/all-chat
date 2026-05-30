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
import type { ChatMessage, DeletionMetadata, EventType } from '@/lib/types/message'
import {
  applyModerationMark,
  deletionKind,
  isDeletionTarget,
  mergeByAgg,
  partitionItems,
  shouldAutoScroll,
  toModEntry,
  type ViewItem,
} from '@/lib/utils/overlayViewModel'

function item(id: string, opts: { userId?: string; eventType?: EventType; aggId?: string } = {}): ViewItem {
  const base: ViewItem = {
    id,
    overlay_id: 'o1',
    platform: 'twitch',
    channel_id: 'c1',
    channel_name: 'chan',
    user: { id: opts.userId ?? 'u1', username: 'u', display_name: 'U', badges: [] },
    message: { text: 'hi', emotes: [] },
    timestamp: '2026-05-30T10:00:00.000Z',
    metadata: {},
  }
  if (opts.eventType) {
    base.event = {
      type: opts.eventType,
      tier: 'low',
      duration: 0,
      is_update: false,
      metadata: {},
      aggregation_id: opts.aggId,
    }
  }
  return base
}

describe('partitionItems', () => {
  it('routes regular messages to chat, activity to events, notices to system', () => {
    const items = [
      item('m1'),
      item('m2', { eventType: 'subscription' }),
      item('m3', { eventType: 'like_aggregate' }),
      item('m4', { eventType: 'token_expiration_warning' }),
      item('m5', { eventType: 'source_permission_error' }),
    ]
    const { chat, events, system } = partitionItems(items)
    expect(chat.map((i) => i.id)).toEqual(['m1'])
    expect(events.map((i) => i.id)).toEqual(['m2', 'm3'])
    expect(system.map((i) => i.id)).toEqual(['m4', 'm5'])
  })

  it('never surfaces message_deletion items in any pane', () => {
    const { chat, events, system } = partitionItems([item('d1', { eventType: 'message_deletion' })])
    expect(chat).toHaveLength(0)
    expect(events).toHaveLength(0)
    expect(system).toHaveLength(0)
  })
})

describe('deletionKind / toModEntry', () => {
  it('maps single to delete', () => {
    const meta: DeletionMetadata = { deletion_type: 'single', target_uuid: 'm0' }
    expect(deletionKind(meta)).toBe('delete')
    const e = toModEntry(meta, 'live', 123)
    expect(e).toMatchObject({ kind: 'delete', targetUuid: 'm0', source: 'live', at: 123 })
  })

  it('maps batch with positive duration to timeout, carrying duration + username', () => {
    const meta: DeletionMetadata = { deletion_type: 'batch', target_user_id: 'u9', target_username: 'spammer', ban_duration: 600 }
    expect(deletionKind(meta)).toBe('timeout')
    const e = toModEntry(meta, 'live', 1)
    expect(e).toMatchObject({ kind: 'timeout', username: 'spammer', targetUserId: 'u9', banDuration: 600 })
  })

  it('maps batch with zero/absent duration to ban', () => {
    expect(deletionKind({ deletion_type: 'batch', target_user_id: 'u9', ban_duration: 0 })).toBe('ban')
    expect(deletionKind({ deletion_type: 'batch', target_user_id: 'u9' })).toBe('ban')
  })

  it('maps clear to clear', () => {
    expect(deletionKind({ deletion_type: 'clear' })).toBe('clear')
    expect(toModEntry({ deletion_type: 'clear' }, 'replay', 5).kind).toBe('clear')
  })
})

describe('isDeletionTarget', () => {
  it('single matches by message uuid', () => {
    expect(isDeletionTarget(item('m1'), { deletion_type: 'single', target_uuid: 'm1' })).toBe(true)
    expect(isDeletionTarget(item('m1'), { deletion_type: 'single', target_uuid: 'other' })).toBe(false)
    expect(isDeletionTarget(item('m1'), { deletion_type: 'single' })).toBe(false)
  })

  it('batch matches by user id', () => {
    expect(isDeletionTarget(item('m1', { userId: 'u5' }), { deletion_type: 'batch', target_user_id: 'u5' })).toBe(true)
    expect(isDeletionTarget(item('m1', { userId: 'u5' }), { deletion_type: 'batch', target_user_id: 'u6' })).toBe(false)
  })

  it('clear matches every item', () => {
    expect(isDeletionTarget(item('m1'), { deletion_type: 'clear' })).toBe(true)
  })
})

describe('applyModerationMark', () => {
  it('marks single target struck-through and leaves others untouched', () => {
    const items = [item('m1'), item('m2')]
    const next = applyModerationMark(items, { deletion_type: 'single', target_uuid: 'm2' })
    expect(next[0]._moderated).toBeUndefined()
    expect(next[1]._moderated).toEqual({ kind: 'delete', banDuration: undefined })
    expect(next[0]).toBe(items[0]) // unmatched item keeps its reference
  })

  it('marks all messages from a banned user', () => {
    const items = [item('m1', { userId: 'ua' }), item('m2', { userId: 'ub' }), item('m3', { userId: 'ua' })]
    const next = applyModerationMark(items, { deletion_type: 'batch', target_user_id: 'ua', ban_duration: 0 })
    expect(next[0]._moderated?.kind).toBe('ban')
    expect(next[1]._moderated).toBeUndefined()
    expect(next[2]._moderated?.kind).toBe('ban')
  })

  it('returns the same array reference when nothing matches', () => {
    const items = [item('m1')]
    expect(applyModerationMark(items, { deletion_type: 'single', target_uuid: 'nope' })).toBe(items)
  })
})

describe('mergeByAgg', () => {
  it('appends and caps when there is no aggregation id', () => {
    const prev = [item('m1'), item('m2')]
    const next = mergeByAgg(prev, item('m3'), 2)
    expect(next.map((i) => i.id)).toEqual(['m2', 'm3'])
  })

  it('appends when the aggregation id is not yet present', () => {
    const prev = [item('m1', { eventType: 'like_aggregate', aggId: 'A' })]
    const next = mergeByAgg(prev, item('m2', { eventType: 'like_aggregate', aggId: 'B' }), 10)
    expect(next).toHaveLength(2)
  })

  it('replaces in place when the aggregation id matches (length unchanged)', () => {
    const prev = [item('x', { eventType: 'like_aggregate', aggId: 'A' }), item('m1')]
    const updated = item('x2', { eventType: 'like_aggregate', aggId: 'A' })
    const next = mergeByAgg(prev, updated, 2)
    expect(next).toHaveLength(2)
    expect(next[0].id).toBe('x2')
    expect(next[1].id).toBe('m1')
  })
})

describe('shouldAutoScroll', () => {
  it('returns true when pinned within the threshold', () => {
    expect(shouldAutoScroll({ scrollHeight: 1000, scrollTop: 980, clientHeight: 20 })).toBe(true)
  })

  it('returns false when scrolled up beyond the threshold', () => {
    expect(shouldAutoScroll({ scrollHeight: 1000, scrollTop: 500, clientHeight: 200 })).toBe(false)
  })

  it('respects a custom threshold', () => {
    expect(shouldAutoScroll({ scrollHeight: 1000, scrollTop: 800, clientHeight: 100 }, 100)).toBe(true)
    expect(shouldAutoScroll({ scrollHeight: 1000, scrollTop: 800, clientHeight: 100 }, 50)).toBe(false)
  })
})

// Type-only guard: ChatMessage is assignable to ViewItem.
const _typeCheck: ViewItem = {} as ChatMessage
void _typeCheck
