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

import { MessageAttachments } from '@/components/overlay/MessageAttachments'
import { renderMessageContent } from '@/lib/renderMessage'
import type { ChatMessage } from '@/lib/types/message'
import { resolveUsernameColor } from '@/lib/utils/usernameColor'

/**
 * Inner content for an event / system-notice row: icon + title + user + optional
 * value + user-text + system-notice body + numeric metadata.
 *
 * Shared by BOTH overlay surfaces — the live OBS overlay (`/overlay/[id]`) and the
 * dashboard editor preview pane (`/overlays/[id]/preview/embed`, mounted as an
 * iframe by `SplitView`) — so the two can never drift. They previously did: the
 * system-notice cases (`listener_deprecation_notice`, `source_permission_error`,
 * `token_expiration_warning`) were added to the live overlay but not the preview,
 * so every system notice rendered as the generic "✨ EVENT!" fallback in the
 * (narrow) preview pane.
 *
 * The `.event-type-{type}` / `.event-tier-{tier}` wrapper classes and the
 * platform/username row header live in each call site's row wrapper, not here.
 */
export function EventContent({ message }: { message: ChatMessage }) {
  const event = message.event!

  // Event icon based on type
  const getEventIcon = () => {
    switch (event.type) {
      case 'subscription':
      case 'resubscription':
      case 'gift_subscription':
      case 'kick_subscription':
      case 'new_sponsor':
        return '⭐'
      case 'bits':
        return '💎'
      case 'raid':
        return '🚀'
      case 'watch_streak':
        return '🔥'
      case 'announcement':
        return '📣'
      case 'bits_badge_tier':
        return '🏅'
      case 'unraid':
        return '🛑'
      case 'modiversary':
        return '🛡️'
      case 'charity_donation':
        return '💝'
      case 'gift_paid_upgrade':
      case 'prime_paid_upgrade':
      case 'pay_it_forward':
        return '⭐'
      case 'channel_points':
        return '🎁'
      case 'super_chat':
        return '💰'
      case 'super_sticker':
        return '🎨'
      case 'gift':
        return '🎁'
      case 'follow':
        return '❤️'
      case 'like_aggregate':
        return '👍'
      case 'share':
        return '🔗'
      case 'member_milestone':
        return '🎂'
      case 'membership_gift':
        return '🎁'
      case 'token_expiration_warning':
        return '⚠️'
      case 'source_permission_error':
        return '🔒'
      case 'listener_deprecation_notice':
        return '🔄'
      default:
        return '✨'
    }
  }

  // Event title based on type
  const getEventTitle = () => {
    switch (event.type) {
      case 'subscription':
        return 'New Subscriber!'
      case 'resubscription':
        return 'Resubscribed!'
      case 'gift_subscription':
        return 'Gift Subscription!'
      case 'mystery_gift':
        return 'Mystery Gift Bomb!'
      case 'bits':
        return 'Bits Cheered!'
      case 'raid':
        return 'Raid Incoming!'
      case 'watch_streak':
        return 'Watch Streak!'
      case 'announcement':
        return 'Announcement'
      case 'bits_badge_tier':
        // Deliberately not "Bits Cheered!" — a lifetime badge threshold was crossed, no bits
        // were cheered in this moment.
        return 'Bits Badge Unlocked!'
      case 'unraid':
        return 'Raid Cancelled'
      case 'modiversary':
        return 'Mod Anniversary!'
      case 'charity_donation':
        return 'Charity Donation!'
      case 'gift_paid_upgrade':
        return 'Continued Their Gift Sub!'
      case 'prime_paid_upgrade':
        return 'Upgraded From Prime!'
      case 'pay_it_forward':
        return 'Paid It Forward!'
      case 'channel_points':
        return 'Channel Points Redeemed!'
      case 'super_chat':
        return 'Super Chat!'
      case 'super_sticker':
        return 'Super Sticker!'
      case 'new_sponsor':
        return 'New Member!'
      case 'member_milestone':
        return 'Member Milestone!'
      case 'membership_gift':
        return 'Membership Gift!'
      case 'gift':
        return 'Gift Received!'
      case 'follow':
        return 'New Follower!'
      case 'like_aggregate':
        return 'Likes!'
      case 'share':
        return 'Stream Shared!'
      case 'token_expiration_warning': {
        const platform = (event.metadata?.platform as string) || 'Platform'
        return `${platform.charAt(0).toUpperCase() + platform.slice(1)} Authentication Error`
      }
      case 'source_permission_error':
        return 'Bot Missing Channel Permission'
      case 'listener_deprecation_notice':
        return 'Twitch Connection Update'
      default:
        return 'Event!'
    }
  }

  return (
    <div className="event-content">
      <div className="mb-1 flex items-center gap-3">
        <span className="event-icon text-4xl leading-none">{getEventIcon()}</span>
        <div className="flex-1">
          <div className="event-title text-lg font-bold text-white">{getEventTitle()}</div>
          <div
            className="event-user text-sm font-semibold"
            style={{ color: resolveUsernameColor(message.user) }}
          >
            {message.user?.display_name || message.user?.username}
          </div>
        </div>
        {event.value && (
          <div className="event-value text-2xl font-bold text-yellow-300">
            {event.value.display_text}
          </div>
        )}
      </div>
      {message.message.text && (
        <div className="event-message-text ml-14 text-sm text-slate-200">
          {/* Rendered through the shared chat renderer, not as a bare string: several events
              carry the chatter's own message (a watch streak IS their chat message, plus resub
              messages, announcements, Super Chats), and the pipeline enriches those with emotes.
              Printing message.text directly showed emote codes as literal text — "Kappa" instead
              of the image — which undercut the notice work in ADR-0046. */}
          {renderMessageContent(message)}
        </div>
      )}
      {/* Twitch chat GIFs and Discord uploads attached to an event's message (ADR-0037). The
          overlay's chat rows render these separately; without this an event-borne GIF was dropped
          entirely. Renders nothing when there are no attachments. */}
      <div className="event-message-attachments ml-14">
        <MessageAttachments message={message} />
      </div>
      {event.type === 'token_expiration_warning' && (
        <div className="event-warning-message mt-2 ml-14 space-y-1 text-sm text-orange-200">
          <div className="font-semibold">
            {event.metadata?.failure_reason === 'expired'
              ? 'OAuth token has expired'
              : 'Failed to refresh OAuth token'}
            {event.metadata?.username ? ` for ${String(event.metadata.username)}` : ''}
          </div>
          <div className="text-xs text-orange-300">
            {'→ Please reconnect your account in Settings → Connections'}
          </div>
        </div>
      )}
      {event.type === 'source_permission_error' && (
        <div className="event-warning-message mt-2 ml-14 space-y-1 text-sm text-red-200">
          <div className="font-semibold">
            {`Channel ${String(event.metadata?.channel_id || '')} is not accessible`}
          </div>
          <div className="text-xs text-red-300">
            {'→ Grant the bot "View Channel" permission in your Discord server settings'}
          </div>
        </div>
      )}
      {event.type === 'listener_deprecation_notice' && (
        <div className="event-warning-message mt-2 ml-14 space-y-1 text-sm text-amber-200">
          <div className="font-semibold">
            {String(
              event.metadata?.description || 'The legacy Twitch chat connection is being retired.'
            )}
          </div>
          <div className="text-xs text-amber-300">
            {'→ Re-add your Twitch source to switch to the new EventSub connection'}
          </div>
        </div>
      )}
      {event.metadata &&
        Object.keys(event.metadata).length > 0 &&
        (() => {
          const m = event.metadata as Record<string, unknown>
          const num = (v: unknown): number =>
            typeof v === 'number' ? v : typeof v === 'string' ? Number(v) : NaN
          const parts: string[] = []
          const viewers = num(m.viewer_count)
          if (viewers > 0) parts.push(`${viewers.toLocaleString()} viewers`)
          if (num(m.months) > 0) parts.push(`${num(m.months)} months`)
          if (num(m.streak) > 0) parts.push(`${num(m.streak)} month streak`)
          if (num(m.gift_count) > 0) parts.push(`${num(m.gift_count)} gifts`)
          if (num(m.bits) > 0) parts.push(`${num(m.bits)} bits`)
          // Watch streaks pay out channel points; the streak count itself is already in the value
          // pill, so only the reward is added here.
          if (num(m.channel_points_awarded) > 0)
            parts.push(`+${num(m.channel_points_awarded).toLocaleString()} points`)
          if (num(m.like_count) > 0) parts.push(`${num(m.like_count)} likes`)
          if (num(m.diamonds) > 0) parts.push(`${num(m.diamonds)} diamonds`)
          return parts.length > 0 ? (
            <div className="event-metadata mt-1 ml-14 text-xs text-slate-400">
              {parts.join(' • ')}
            </div>
          ) : null
        })()}
    </div>
  )
}
