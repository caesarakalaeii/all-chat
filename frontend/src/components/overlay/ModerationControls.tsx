'use client'

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
import { Ban, Clock, ShieldOff, Trash2 } from 'lucide-react'

import { Popover } from '@/components/ui/popover'
import { MODERATABLE_PLATFORMS, TIMEOUT_PRESETS } from '@/lib/types/moderation'
import type { SourceCapability } from '@/lib/types/moderation'
import type { ViewItem } from '@/lib/utils/overlayViewModel'

export interface ModerationControlsProps {
  item: ViewItem
  /** Capability for the item's source, or undefined if the source isn't listed. */
  capability?: SourceCapability
  onDelete: (item: ViewItem) => void
  onTimeout: (item: ViewItem, durationSeconds: number) => void
  onBan: (item: ViewItem) => void
  onUnban: (item: ViewItem) => void
}

/**
 * Per-message / per-user moderation controls for a monitor chat row. A delete
 * button reveals on row hover (the parent row owns the `group` class); a small
 * popover offers timeout presets, ban and unban.
 *
 * The per-user menu is a ui/popover, so it is portaled to `document.body` and
 * positioned by Base UI. It has to be: as an absolutely-positioned child of the
 * row it was clipped by ChatPanel's `overflow-y-auto` scroll container, and
 * since it only ever opened downward, the newest row's menu had no room at all
 * — the ban action for the viewer who just spoke was unreachable. z-index does
 * not help with clipping; leaving the scroll container is the fix.
 *
 * Affordances follow the source's capability: a platform with no moderation API
 * (e.g. TikTok) or a source missing the required scope renders every control
 * disabled with an explanatory tooltip. Individual actions are further gated by
 * the source's `actions` list. `system` rows render no controls at all (the
 * parent skips this component for them).
 */
export function ModerationControls({
  item,
  capability,
  onDelete,
  onTimeout,
  onBan,
  onUnban,
}: ModerationControlsProps) {
  const platformSupported = MODERATABLE_PLATFORMS.has(item.platform)
  // A source is actionable only when its platform has a mod API, the viewer owns
  // the overlay (capability present) and the backend reports it moderatable.
  const disabled = !platformSupported || !capability || !capability.moderatable
  const actions = capability?.actions ?? []
  const can = (a: string) => !disabled && actions.includes(a as never)
  const hasUserActions = can('timeout') || can('ban') || can('unban')

  // When a source can't be moderated at all, keep a disabled affordance (carrying the
  // reason tooltip) for hover feedback. When it can, render only the controls whose
  // actions are actually granted — so a source without delete (YouTube, or a Kick credential
  // holding only the ban scope) shows no delete button, and a platform whose bot/credential
  // lacks per-user permissions shows no
  // per-user menu, rather than a dead control. (Discord reports whatever its bot's guild
  // permissions allow — delete and/or timeout/ban/unban.)
  const showDelete = disabled || can('delete')
  const showUserActions = disabled || hasUserActions

  // Each string names what stands in the way, and where the reader can act, who to ask. Sending
  // someone at a fix that is not theirs to make is the failure mode the reason vocabulary exists
  // to prevent (ADR-0048).
  const disabledReason = !platformSupported
    ? `${platformLabel(item.platform)} has no moderation API`
    : !capability
      ? 'Moderation is unavailable for this source'
      : capability.reason === 'missing_scope'
        ? 'Grant moderation permissions to enable mod actions'
        : capability.reason === 'unsupported_platform'
          ? `${platformLabel(item.platform)} has no moderation API`
          : capability.reason === 'needs_discord_link'
            ? 'Link your Discord account to moderate here'
            : capability.reason === 'owner_channel_unverified'
              ? "This streamer's Discord account isn't connected, so nothing can be moderated here"
              : capability.reason === 'bot_missing_permission'
                ? "The All-Chat bot wasn't given this Discord permission — ask the streamer to re-invite it"
                : 'Moderation is unavailable for this source'

  return (
    <div className="ml-1 inline-flex shrink-0 items-center gap-1 align-text-bottom">
      {/* Delete — revealed on row hover. Hidden for moderatable sources that don't
          support single-message delete (Kick/YouTube). */}
      {showDelete && (
        <button
          type="button"
          onClick={() => onDelete(item)}
          disabled={!can('delete')}
          title={disabled ? disabledReason : 'Delete message'}
          aria-label="Delete message"
          className={clsx(
            'rounded p-0.5 opacity-0 transition group-hover:opacity-100 focus-visible:opacity-100 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none',
            can('delete')
              ? 'text-text-dim hover:bg-red-500/10 hover:text-red-400'
              : 'cursor-not-allowed text-text-dim/50'
          )}
        >
          <Trash2 className="h-3.5 w-3.5" />
        </button>
      )}

      {/* Per-user actions trigger. Hidden for moderatable sources with no per-user
          action granted (e.g. a Discord bot invited without ban/timeout permissions). */}
      {showUserActions && (
        <Popover.Root>
          <Popover.Trigger
            render={
              <button
                type="button"
                disabled={disabled}
                title={disabled ? disabledReason : 'Moderate user'}
                aria-label="Moderate user"
                className={clsx(
                  'rounded p-0.5 opacity-0 transition group-hover:opacity-100 focus-visible:opacity-100 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none',
                  disabled
                    ? 'cursor-not-allowed text-text-dim/50'
                    : 'text-text-dim hover:bg-surface-2 hover:text-text'
                )}
              >
                <Clock className="h-3.5 w-3.5" />
              </button>
            }
          />
          <Popover.Content className="w-44 border-border bg-surface p-2">
            <Popover.Title className="sr-only">Moderate user</Popover.Title>
            {can('timeout') && (
              <div className="mb-1">
                <p className="mb-1 px-1 text-[10px] font-semibold tracking-wide text-text-dim uppercase">
                  Timeout
                </p>
                <div className="flex gap-1">
                  {TIMEOUT_PRESETS.map((preset) => (
                    <Popover.Close
                      key={preset.seconds}
                      render={
                        <button
                          type="button"
                          onClick={() => onTimeout(item, preset.seconds)}
                          className="flex flex-1 items-center justify-center gap-1 rounded border border-border px-1.5 py-1 text-xs font-medium text-text-sub transition-colors hover:border-border-md hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                        >
                          <Clock className="h-3 w-3" />
                          {preset.label}
                        </button>
                      }
                    />
                  ))}
                </div>
              </div>
            )}

            {can('ban') && (
              <Popover.Close
                render={
                  <button
                    type="button"
                    onClick={() => onBan(item)}
                    className="flex w-full items-center gap-2 rounded border border-red-500/20 bg-red-500/10 px-2 py-1.5 text-xs font-medium text-red-400 transition-colors hover:bg-red-500/20 focus-visible:ring-2 focus-visible:ring-red-400 focus-visible:outline-none"
                  >
                    <Ban className="h-3.5 w-3.5" />
                    Ban user
                  </button>
                }
              />
            )}

            {can('unban') && (
              <Popover.Close
                render={
                  <button
                    type="button"
                    onClick={() => onUnban(item)}
                    className="mt-1 flex w-full items-center gap-2 rounded border border-border px-2 py-1.5 text-xs font-medium text-text-sub transition-colors hover:border-border-md hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
                  >
                    <ShieldOff className="h-3.5 w-3.5" />
                    Unban user
                  </button>
                }
              />
            )}
          </Popover.Content>
        </Popover.Root>
      )}
    </div>
  )
}

/** Human-readable platform name for tooltips. */
function platformLabel(platform: string): string {
  switch (platform) {
    case 'tiktok':
      return 'TikTok'
    case 'youtube':
      return 'YouTube'
    case 'twitch':
      return 'Twitch'
    case 'kick':
      return 'Kick'
    case 'discord':
      return 'Discord'
    default:
      return platform
  }
}
