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
  chatRowKeys,
  countNewItems,
  deletionKind,
  isAudienceEvent,
  isDeletionTarget,
  isPinnedToLiveEdge,
  matchesUserFilter,
  mergeAutoModResolution,
  mergeByAgg,
  partitionItems,
  shouldAutoScroll,
  toModActionEntry,
  toModEntry,
  type ModEntryData,
  userFilterFor,
  type ViewItem,
} from '@/lib/utils/overlayViewModel'

function item(
  id: string,
  opts: { userId?: string; eventType?: EventType; aggId?: string } = {}
): ViewItem {
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

describe('isAudienceEvent', () => {
  it('is true for audience activity (subs, gifts/roses, follows, likes)', () => {
    expect(isAudienceEvent(item('m1', { eventType: 'subscription' }))).toBe(true)
    expect(isAudienceEvent(item('m2', { eventType: 'gift' }))).toBe(true)
    expect(isAudienceEvent(item('m3', { eventType: 'follow' }))).toBe(true)
    expect(isAudienceEvent(item('m4', { eventType: 'channel_points' }))).toBe(true)
    expect(isAudienceEvent(item('m5', { eventType: 'like_aggregate' }))).toBe(true)
  })

  it('is false for plain chat, system notices, and deletions', () => {
    expect(isAudienceEvent(item('m1'))).toBe(false)
    expect(isAudienceEvent(item('m2', { eventType: 'token_expiration_warning' }))).toBe(false)
    expect(isAudienceEvent(item('m3', { eventType: 'source_permission_error' }))).toBe(false)
    expect(isAudienceEvent(item('m4', { eventType: 'listener_deprecation_notice' }))).toBe(false)
    expect(isAudienceEvent(item('m5', { eventType: 'message_deletion' }))).toBe(false)
  })

  it('matches partitionItems: it is true iff the item lands in the events pane', () => {
    const samples: ViewItem[] = [
      item('m1'),
      item('m2', { eventType: 'subscription' }),
      item('m3', { eventType: 'gift' }),
      item('m4', { eventType: 'token_expiration_warning' }),
      item('m5', { eventType: 'message_deletion' }),
    ]
    for (const s of samples) {
      const inEventsPane = partitionItems([s]).events.length === 1
      expect(isAudienceEvent(s)).toBe(inEventsPane)
    }
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
    const meta: DeletionMetadata = {
      deletion_type: 'batch',
      target_user_id: 'u9',
      target_username: 'spammer',
      ban_duration: 600,
    }
    expect(deletionKind(meta)).toBe('timeout')
    const e = toModEntry(meta, 'live', 1)
    expect(e).toMatchObject({
      kind: 'timeout',
      username: 'spammer',
      targetUserId: 'u9',
      banDuration: 600,
    })
  })

  it('maps batch with zero/absent duration to ban', () => {
    expect(deletionKind({ deletion_type: 'batch', target_user_id: 'u9', ban_duration: 0 })).toBe(
      'ban'
    )
    expect(deletionKind({ deletion_type: 'batch', target_user_id: 'u9' })).toBe('ban')
  })

  it('maps clear to clear', () => {
    expect(deletionKind({ deletion_type: 'clear' })).toBe('clear')
    expect(toModEntry({ deletion_type: 'clear' }, 'replay', 5).kind).toBe('clear')
  })
})

describe('toModActionEntry', () => {
  it('maps a timeout to the existing timeout kind, keeping the moderator and duration', () => {
    const entry = toModActionEntry(
      {
        action: 'timeout',
        moderator_login: 'modperson',
        target_login: 'spammer',
        ban_duration: 600,
        reason: 'spam',
      },
      'live',
      42
    )
    expect(entry).toMatchObject({
      kind: 'timeout',
      action: 'timeout',
      moderator: 'modperson',
      username: 'spammer',
      banDuration: 600,
      reason: 'spam',
      source: 'live',
      at: 42,
    })
  })

  it('maps an AutoMod hold to the automod kind with no moderator (AutoMod has no human actor)', () => {
    const entry = toModActionEntry(
      {
        action: 'automod_hold',
        target_login: 'chatter',
        held_message_id: 'm1',
        held_text: 'something rude',
        automod_category: 'profanity',
        automod_level: 3,
      },
      'live',
      7
    )
    expect(entry).toMatchObject({
      kind: 'automod',
      heldMessageId: 'm1',
      heldText: 'something rude',
      automodCategory: 'profanity',
      automodLevel: 3,
      username: 'chatter',
    })
    expect(entry?.moderator).toBeUndefined()
  })

  // Twitch adds channel.moderate actions over time. An action this build has never
  // heard of must still reach the moderation log — dropping it would silently hide
  // real moderation from the person watching the channel.
  it('renders an unknown Twitch action as a generic entry instead of dropping it', () => {
    const entry = toModActionEntry(
      { action: 'some_future_action', moderator_login: 'modperson', target_login: 'someone' },
      'live',
      9
    )
    expect(entry).not.toBeNull()
    expect(entry).toMatchObject({
      kind: 'action',
      action: 'some_future_action',
      moderator: 'modperson',
      username: 'someone',
    })
  })

  it('returns null when the action is missing or not a string', () => {
    expect(toModActionEntry({}, 'live', 1)).toBeNull()
    expect(toModActionEntry({ action: 7 }, 'live', 1)).toBeNull()
  })
})

describe('mergeAutoModResolution', () => {
  const hold: ModEntryData = {
    kind: 'automod',
    action: 'automod_hold',
    heldMessageId: 'm1',
    heldText: 'something rude',
    source: 'live',
    at: 100,
  }

  it('folds a resolution into the hold it resolves, keeping its position', () => {
    const resolution: ModEntryData = {
      kind: 'automod',
      action: 'automod_resolved',
      heldMessageId: 'm1',
      resolution: 'approved',
      resolvedBy: 'modperson',
      source: 'live',
      at: 200,
    }
    const merged = mergeAutoModResolution([hold], resolution)
    expect(merged).toHaveLength(1)
    expect(merged[0]).toMatchObject({
      heldMessageId: 'm1',
      resolution: 'approved',
      resolvedBy: 'modperson',
      at: 100,
    })
  })

  it('appends a resolution whose held message is not in the log', () => {
    const orphan: ModEntryData = {
      kind: 'automod',
      action: 'automod_resolved',
      heldMessageId: 'other',
      resolution: 'denied',
      source: 'live',
      at: 200,
    }
    expect(mergeAutoModResolution([hold], orphan)).toHaveLength(2)
  })
})

describe('isDeletionTarget', () => {
  it('single matches by message uuid', () => {
    expect(isDeletionTarget(item('m1'), { deletion_type: 'single', target_uuid: 'm1' })).toBe(true)
    expect(isDeletionTarget(item('m1'), { deletion_type: 'single', target_uuid: 'other' })).toBe(
      false
    )
    expect(isDeletionTarget(item('m1'), { deletion_type: 'single' })).toBe(false)
  })

  it('batch matches by user id', () => {
    expect(
      isDeletionTarget(item('m1', { userId: 'u5' }), {
        deletion_type: 'batch',
        target_user_id: 'u5',
      })
    ).toBe(true)
    expect(
      isDeletionTarget(item('m1', { userId: 'u5' }), {
        deletion_type: 'batch',
        target_user_id: 'u6',
      })
    ).toBe(false)
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
    const items = [
      item('m1', { userId: 'ua' }),
      item('m2', { userId: 'ub' }),
      item('m3', { userId: 'ua' }),
    ]
    const next = applyModerationMark(items, {
      deletion_type: 'batch',
      target_user_id: 'ua',
      ban_duration: 0,
    })
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
    expect(shouldAutoScroll({ scrollHeight: 1000, scrollTop: 800, clientHeight: 100 }, 100)).toBe(
      true
    )
    expect(shouldAutoScroll({ scrollHeight: 1000, scrollTop: 800, clientHeight: 100 }, 50)).toBe(
      false
    )
  })
})

describe('isPinnedToLiveEdge', () => {
  it('newest-last: pinned at the bottom, not pinned when scrolled up', () => {
    expect(
      isPinnedToLiveEdge({ scrollHeight: 1000, scrollTop: 980, clientHeight: 20 }, false)
    ).toBe(true)
    expect(
      isPinnedToLiveEdge({ scrollHeight: 1000, scrollTop: 500, clientHeight: 200 }, false)
    ).toBe(false)
  })

  it('newest-first: pinned at the top, not pinned when scrolled down', () => {
    expect(isPinnedToLiveEdge({ scrollHeight: 1000, scrollTop: 0, clientHeight: 200 }, true)).toBe(
      true
    )
    expect(isPinnedToLiveEdge({ scrollHeight: 1000, scrollTop: 39, clientHeight: 200 }, true)).toBe(
      true
    )
    expect(
      isPinnedToLiveEdge({ scrollHeight: 1000, scrollTop: 500, clientHeight: 200 }, true)
    ).toBe(false)
    // The bottom edge is the FAR edge in this mode, so it is not pinned there.
    expect(
      isPinnedToLiveEdge({ scrollHeight: 1000, scrollTop: 800, clientHeight: 200 }, true)
    ).toBe(false)
  })

  it('respects a custom threshold in both modes', () => {
    expect(
      isPinnedToLiveEdge({ scrollHeight: 1000, scrollTop: 800, clientHeight: 100 }, false, 100)
    ).toBe(true)
    expect(
      isPinnedToLiveEdge({ scrollHeight: 1000, scrollTop: 800, clientHeight: 100 }, false, 50)
    ).toBe(false)
    expect(
      isPinnedToLiveEdge({ scrollHeight: 1000, scrollTop: 80, clientHeight: 100 }, true, 100)
    ).toBe(true)
    expect(
      isPinnedToLiveEdge({ scrollHeight: 1000, scrollTop: 80, clientHeight: 100 }, true, 50)
    ).toBe(false)
  })
})

describe('chatRowKeys', () => {
  it('distinguishes repeats of the same id by occurrence', () => {
    expect(chatRowKeys([item('a'), item('b'), item('a')])).toEqual(['a#0', 'b#0', 'a#1'])
  })

  it('keeps a row\u2019s key when another item is appended or prepended', () => {
    const middle = [item('a'), item('b')]
    const before = chatRowKeys(middle)
    expect(chatRowKeys([...middle, item('c')]).slice(0, 2)).toEqual(before)
    expect(chatRowKeys([item('z'), ...middle]).slice(1)).toEqual(before)
  })

  it('returns one key per item', () => {
    expect(chatRowKeys([])).toEqual([])
    expect(chatRowKeys([item('a'), item('b'), item('c')])).toHaveLength(3)
  })
})

describe('userFilterFor / matchesUserFilter', () => {
  it('keys the filter on platform + user id and labels it with the display name', () => {
    const filter = userFilterFor(item('m1', { userId: 'u7' }))
    expect(filter).toEqual({ key: 'twitch:u7', label: 'U' })
  })

  it('falls back to the lowercased username when the user has no id', () => {
    const it1 = item('m1')
    it1.user = { id: '', username: 'Alice', display_name: '', badges: [] }
    expect(userFilterFor(it1)).toEqual({ key: 'twitch:alice', label: 'Alice' })
  })

  it('returns null for items without a user identity', () => {
    // The types promise a user, but system rows can arrive without one.
    const it1 = { ...item('m1'), user: undefined } as unknown as ChatMessage
    expect(userFilterFor(it1)).toBeNull()
    const it2 = item('m2')
    it2.user = { id: '', username: '', display_name: '', badges: [] }
    expect(userFilterFor(it2)).toBeNull()
  })

  it('matches only the same identity on the same platform', () => {
    const filter = userFilterFor(item('m1', { userId: 'u7' }))!
    expect(matchesUserFilter(item('m2', { userId: 'u7' }), filter)).toBe(true)
    expect(matchesUserFilter(item('m3', { userId: 'u8' }), filter)).toBe(false)
    const otherPlatform = { ...item('m4', { userId: 'u7' }), platform: 'kick' as const }
    expect(matchesUserFilter(otherPlatform, filter)).toBe(false)
  })
})

describe('countNewItems', () => {
  it('counts live items missing from the snapshot, surviving front-trimming', () => {
    const snapshot = [item('m1'), item('m2'), item('m3')]
    // Buffer capped at 3: m1 trimmed away, m4 + m5 arrived since the pause.
    const live = [item('m3'), item('m4'), item('m5')]
    expect(countNewItems(snapshot, live)).toBe(2)
  })

  it('returns 0 when nothing arrived', () => {
    const snapshot = [item('m1')]
    expect(countNewItems(snapshot, [item('m1')])).toBe(0)
  })
})

// Type-only guard: ChatMessage is assignable to ViewItem.
const _typeCheck: ViewItem = {} as ChatMessage
void _typeCheck
