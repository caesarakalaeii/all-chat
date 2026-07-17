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

import { useState } from 'react'
import { Radio, X } from 'lucide-react'
import { safeExternalRedirect } from '@/lib/auth/redirect-allowlist'
import { startAddSourceReflow } from '@/lib/api/add-source'
import { toastManager } from '@/lib/toast'
import type { ChatSource } from '@/lib/types/overlay'

const DISMISS_KEY = 'eventsub-migration-banner-dismissed'

export interface MigratableChannel {
  channelId: string
  /** An overlay containing this source — used to start the per-channel re-consent reflow. */
  overlayId: string
}

/**
 * getMigratableChannels returns the distinct Twitch channels the requesting user owns that
 * still read chat via the legacy IRC path (is_own_channel && !chat_via_eventsub). Each is
 * paired with one overlay id usable to start the add-source re-consent reflow. A single
 * re-consent grants the chat scopes for the user's whole Twitch account, so migrating any
 * one entry migrates all of that account's own-channel sources.
 */
export function getMigratableChannels(
  sourcesByOverlay: Record<string, ChatSource[]>
): MigratableChannel[] {
  const byChannel = new Map<string, MigratableChannel>()
  for (const [overlayId, sources] of Object.entries(sourcesByOverlay)) {
    for (const source of sources) {
      if (source.platform === 'twitch' && source.is_own_channel && !source.chat_via_eventsub) {
        const key = source.channel_id.toLowerCase()
        if (!byChannel.has(key)) {
          byChannel.set(key, { channelId: source.channel_id, overlayId })
        }
      }
    }
  }
  return Array.from(byChannel.values())
}

/**
 * EventSubMigrationBanner nudges streamers whose own Twitch channel still reads chat over the
 * legacy IRC connection to re-consent and move to EventSub. IRC is capped (~100 channels) and
 * silently drops the overflow once that many streams are live, so the migration prevents
 * message loss. Dismissible (non-blocking) per the chosen informative tone.
 */
export function EventSubMigrationBanner({
  sourcesByOverlay,
}: {
  sourcesByOverlay: Record<string, ChatSource[]>
}) {
  const [dismissed, setDismissed] = useState(
    () => typeof window !== 'undefined' && window.localStorage.getItem(DISMISS_KEY) === '1'
  )

  const channels = getMigratableChannels(sourcesByOverlay)
  if (dismissed || channels.length === 0) return null

  async function handleUpgrade() {
    // Routed through startAddSourceReflow (apiClient) so an expired access
    // cookie is refreshed and retried under H3 cookie auth instead of failing.
    const result = await startAddSourceReflow(
      `/api/v1/auth/twitch/add-source/${channels[0].overlayId}`
    )
    if (result.kind === 'redirect') {
      safeExternalRedirect(result.authUrl)
      return
    }
    if (result.kind === 'added') {
      // Already authorized with the chat scopes — the channel is moving to
      // EventSub; hide the nudge.
      toastManager.add({ title: 'Twitch chat connected', type: 'success' })
      setDismissed(true)
      return
    }
    toastManager.add({
      title: 'Could not start the upgrade',
      description: result.message,
      type: 'error',
    })
  }

  function handleDismiss() {
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(DISMISS_KEY, '1')
    }
    setDismissed(true)
  }

  const count = channels.length

  return (
    <div
      role="status"
      className="flex items-start gap-3 rounded-lg border border-twitch/30 bg-twitch/10 px-4 py-3 text-sm text-text"
    >
      <Radio className="mt-0.5 size-4 shrink-0 text-twitch" aria-hidden="true" />
      <div className="min-w-0 flex-1">
        <p className="font-medium">
          {count === 1
            ? 'Your Twitch chat uses the legacy connection'
            : `${count} of your Twitch channels use the legacy connection`}
        </p>
        <p className="mt-0.5 text-text-sub">
          The old IRC chat connection is being retired and can drop messages when many streams are
          live. Reconnect to move to the new connection and keep your chat reliable.
        </p>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <button
          onClick={handleUpgrade}
          className="rounded-md bg-twitch px-3 py-1.5 text-xs font-semibold text-bg transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:outline-none"
        >
          Reconnect now
        </button>
        <button
          onClick={handleDismiss}
          aria-label="Dismiss"
          className="rounded p-0.5 text-text-sub opacity-60 transition-opacity hover:opacity-100 focus-visible:ring-2 focus-visible:ring-current focus-visible:outline-none"
        >
          <X className="size-3.5" />
        </button>
      </div>
    </div>
  )
}
