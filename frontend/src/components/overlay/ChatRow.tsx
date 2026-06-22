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

import Image from 'next/image'
import clsx from 'clsx'

import { AllChatBadge } from '@/components/AllChatBadge'
import { PremiumBadge } from '@/components/PremiumBadge'
import { UserAvatar } from '@/components/UserAvatar'
import { PlatformGlyph, PlatformGlyphs } from '@/components/overlay/PlatformGlyph'
import {
  ModerationControls,
  type ModerationControlsProps,
} from '@/components/overlay/ModerationControls'
import { DEFAULT_VIEW_PREFS, type MonitorViewPrefs } from '@/app/overlay/[id]/view/viewPrefs'
import { renderMessageContent } from '@/lib/renderMessage'
import type { SourceCapability } from '@/lib/types/moderation'
import { buildGradientCSS } from '@/lib/utils/gradient'
import type { ModKind, ViewItem } from '@/lib/utils/overlayViewModel'

const MOD_LABEL: Record<ModKind, string> = {
  delete: 'deleted',
  timeout: 'timed out',
  ban: 'banned',
  clear: 'cleared',
}

/** Moderation wiring passed down from the monitor page (omit to hide controls). */
export interface ChatRowModeration extends Pick<
  ModerationControlsProps,
  'onDelete' | 'onTimeout' | 'onBan' | 'onUnban'
> {
  /** Capability for this row's source, or undefined if the source isn't listed. */
  capability?: SourceCapability
}

/**
 * Compact, theme-agnostic chat row for the observability view. Shows every
 * signal the overlay carries (platform, badges, pronouns, username color /
 * gradient, emotes) using neutral design tokens — no overlay theme CSS, no
 * animations. Moderated messages stay visible but struck-through and tagged.
 */
export function ChatRow({
  item,
  prefs = DEFAULT_VIEW_PREFS,
  moderation,
}: {
  item: ViewItem
  prefs?: MonitorViewPrefs
  moderation?: ChatRowModeration
}) {
  const mod = item._moderated
  const displayName = item.user?.display_name || item.user?.username
  const time = new Date(item.timestamp).toLocaleTimeString()
  const isShared = item.metadata?.is_shared_chat === true
  // System rows are operational notices, never moderatable.
  const showControls = prefs.showModeration && !!moderation && item.platform !== 'system'

  return (
    <div
      data-platform={item.platform}
      data-username={item.user?.username}
      className={clsx(
        'group flex gap-2 border-b border-border/60 px-3 py-1.5 text-sm leading-relaxed',
        mod && 'opacity-60'
      )}
    >
      {prefs.showTimestamps && (
        <span className="shrink-0 pt-0.5 font-mono text-xs text-text-dim tabular-nums select-none">
          {time}
        </span>
      )}
      {prefs.showPlatformGlyph && (
        <span className="shrink-0 pt-0.5">
          {item.platforms && item.platforms.length > 1 ? (
            <PlatformGlyphs platforms={item.platforms} />
          ) : (
            <PlatformGlyph platform={item.platform} />
          )}
        </span>
      )}
      {prefs.showAvatars && (
        <span className="shrink-0 pt-0.5">
          <UserAvatar
            size={20}
            avatarUrl={item.user?.avatar_url}
            frameUrl={item.user?.avatar_frame_url}
            flairUrl={item.user?.avatar_flair_url}
            displayName={displayName}
          />
        </span>
      )}
      <div className="min-w-0 flex-1 break-words">
        {prefs.showBadges &&
          item.user?.badges?.map((badge, idx) =>
            badge.name === 'allchat' ? (
              <span key={idx} className="mr-1 inline-block align-text-bottom">
                <AllChatBadge size={16} title={badge.name} />
              </span>
            ) : badge.name === 'allchat-premium' ? (
              <span key={idx} className="mr-1 inline-block align-text-bottom">
                <PremiumBadge size={16} title={badge.name} />
              </span>
            ) : badge.icon_url ? (
              <Image
                key={idx}
                src={badge.icon_url}
                alt={badge.name}
                width={16}
                height={16}
                title={badge.name}
                className="mr-1 inline-block h-4 w-auto object-contain align-text-bottom"
              />
            ) : null
          )}
        {item.user?.name_gradient ? (
          <span
            className="font-semibold"
            style={{
              backgroundImage: buildGradientCSS(item.user.name_gradient),
              WebkitBackgroundClip: 'text',
              backgroundClip: 'text',
              color: 'transparent',
            }}
          >
            {displayName}
          </span>
        ) : (
          <span className="font-semibold" style={{ color: item.user?.color || undefined }}>
            {displayName}
          </span>
        )}
        {prefs.showPronouns && item.user?.pronouns && (
          <span className="ml-1 rounded bg-surface-2 px-1 text-[10px] font-medium text-text-sub">
            {item.user.pronouns}
          </span>
        )}
        {isShared && (
          <span className="ml-1 rounded bg-twitch/20 px-1 text-[10px] font-medium text-twitch uppercase">
            shared
          </span>
        )}
        <span className="text-text-dim">: </span>
        <span className={clsx('text-text', mod && 'line-through')}>
          {renderMessageContent(item)}
        </span>
        {mod && (
          <span className="ml-2 rounded bg-youtube/15 px-1.5 py-0.5 text-[10px] font-semibold text-youtube uppercase">
            {MOD_LABEL[mod.kind]}
            {mod.kind === 'timeout' && mod.banDuration ? ` ${mod.banDuration}s` : ''}
          </span>
        )}
        {showControls && moderation && (
          <ModerationControls
            item={item}
            capability={moderation.capability}
            onDelete={moderation.onDelete}
            onTimeout={moderation.onTimeout}
            onBan={moderation.onBan}
            onUnban={moderation.onUnban}
          />
        )}
      </div>
    </div>
  )
}
