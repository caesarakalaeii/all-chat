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

import clsx from 'clsx'

import { PlatformGlyph } from '@/components/overlay/PlatformGlyph'
import type { EventType } from '@/lib/types/message'
import type { ViewItem } from '@/lib/utils/overlayViewModel'

const EVENT_TITLE: Partial<Record<EventType, string>> = {
  subscription: 'Subscription',
  resubscription: 'Resub',
  gift_subscription: 'Gift Sub',
  mystery_gift: 'Mystery Gifts',
  bits: 'Bits',
  raid: 'Raid',
  channel_points: 'Channel Points',
  ritual: 'Ritual',
  super_chat: 'Super Chat',
  super_sticker: 'Super Sticker',
  new_sponsor: 'New Member',
  member_milestone: 'Member Milestone',
  membership_gift: 'Membership Gift',
  gift_received: 'Gift Received',
  kick_subscription: 'Kick Sub',
  kick_gift_subscription: 'Kick Gift Sub',
  kick_donation: 'Kick Donation',
  gift: 'Gift',
  follow: 'Follow',
  like_aggregate: 'Likes',
  share: 'Share',
  message_deleted: 'Message Deleted',
  user_banned: 'User Banned',
  token_expiration_warning: 'Auth Warning',
  source_permission_error: 'Permission Error',
}

const SYSTEM_TYPES = new Set<EventType>(['token_expiration_warning', 'source_permission_error'])

/** A single audience-activity (or system-notice) row for the activity feed. */
export function CompactEvent({ item }: { item: ViewItem }) {
  const event = item.event!
  const title = EVENT_TITLE[event.type] ?? 'Event'
  const name = item.user?.display_name || item.user?.username
  const time = new Date(item.timestamp).toLocaleTimeString()
  const isSystem = SYSTEM_TYPES.has(event.type)

  return (
    <div
      data-event-type={event.type}
      className={clsx(
        'flex gap-2 border-b border-border/60 px-3 py-1.5 text-sm',
        isSystem && 'bg-youtube/5'
      )}
    >
      <span className="shrink-0 pt-0.5 font-mono text-xs text-text-dim tabular-nums select-none">
        {time}
      </span>
      <span className="shrink-0 pt-0.5">
        <PlatformGlyph platform={item.platform} />
      </span>
      <div className="min-w-0 flex-1 break-words">
        <span
          className={clsx(
            'rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase',
            isSystem ? 'bg-youtube/15 text-youtube' : 'bg-twitch/15 text-twitch'
          )}
        >
          {title}
        </span>
        {name && <span className="ml-2 font-semibold text-text">{name}</span>}
        {event.value?.display_text && (
          <span className="ml-2 font-semibold text-text-sub">{event.value.display_text}</span>
        )}
        {item.message?.text && <span className="ml-2 text-text-sub">{item.message.text}</span>}
      </div>
    </div>
  )
}
