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
 * Platform Status Indicators Component
 *
 * Displays small icons for each configured source (Twitch, YouTube, Kick, TikTok, Discord).
 * Renders one indicator per source (channel), not per platform.
 * Active sources (connected) appear in color.
 * Inactive sources appear in grayscale.
 * Reconnecting sources show a countdown timer.
 *
 * Can be hidden via CSS by targeting `.platform-status-indicators`
 */

'use client'

import { useEffect, useState } from 'react'
import clsx from 'clsx'
import type { PlatformStatus } from '@/lib/types/message'

export interface SourceInfo {
  platform: string
  channelId: string
  channelName: string
}

interface PlatformStatusIndicatorsProps {
  configuredSources: Map<string, SourceInfo>
  activeChannels: Set<string>
  channelStatuses: Map<string, PlatformStatus>
  /**
   * 'fixed' (default) pins the cluster to the top-right corner for the OBS
   * overlay. 'inline' drops the fixed positioning so it can sit inside a header
   * (used by the observability view).
   */
  variant?: 'fixed' | 'inline'
}

// Platform SVG Icons - Using official brand colors per platform guidelines
const TwitchIcon = () => (
  <svg viewBox="0 0 24 24" className="h-5 w-5">
    {/* Twitch official purple: #9146FF - Per Twitch brand guidelines */}
    <path
      fill="#9146FF"
      d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714Z"
    />
  </svg>
)

const YouTubeIcon = () => (
  <svg viewBox="0 0 24 24" className="h-5 w-5">
    {/* YouTube official red: #FF0000 - Never modify this color per branding guidelines */}
    <path
      fill="#FF0000"
      d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"
    />
  </svg>
)

const KickIcon = () => (
  <svg viewBox="0 0 24 24" className="h-5 w-5" style={{ imageRendering: 'pixelated' }}>
    {/* Kick - Simple green K */}
    <text
      x="12"
      y="18"
      fontSize="20"
      fontWeight="bold"
      fill="#00E701"
      textAnchor="middle"
      fontFamily="monospace"
    >
      K
    </text>
  </svg>
)

const TikTokIcon = () => (
  <svg viewBox="0 0 24 24" className="h-5 w-5">
    {/* TikTok teal (#69C9D0) used here for visibility on dark overlay backgrounds */}
    <path
      fill="#69C9D0"
      d="M19.59 6.69a4.83 4.83 0 0 1-3.77-4.25V2h-3.45v13.67a2.89 2.89 0 0 1-5.2 1.74 2.89 2.89 0 0 1 2.31-4.64 2.93 2.93 0 0 1 .88.13V9.4a6.84 6.84 0 0 0-1-.05A6.33 6.33 0 0 0 5 20.1a6.34 6.34 0 0 0 10.86-4.43v-7a8.16 8.16 0 0 0 4.77 1.52v-3.4a4.85 4.85 0 0 1-1-.1z"
    />
  </svg>
)

const DiscordIcon = () => (
  <svg viewBox="0 0 24 24" className="h-5 w-5">
    <path
      fill="#5865F2"
      d="M20.317 4.37a19.791 19.791 0 0 0-4.885-1.515.074.074 0 0 0-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 0 0-5.487 0 12.64 12.64 0 0 0-.617-1.25.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 0 0 .031.057 19.9 19.9 0 0 0 5.993 3.03.078.078 0 0 0 .084-.028 14.09 14.09 0 0 0 1.226-1.994.076.076 0 0 0-.041-.106 13.107 13.107 0 0 1-1.872-.892.077.077 0 0 1-.008-.128 10.2 10.2 0 0 0 .372-.292.074.074 0 0 1 .077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127 12.299 12.299 0 0 1-1.873.892.077.077 0 0 0-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028 19.839 19.839 0 0 0 6.002-3.03.077.077 0 0 0 .032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 0 0-.031-.03z"
    />
  </svg>
)

const platformIcons: Record<string, { icon: React.FC; label: string }> = {
  twitch: { icon: TwitchIcon, label: 'Twitch' },
  youtube: { icon: YouTubeIcon, label: 'YouTube' },
  kick: { icon: KickIcon, label: 'Kick' },
  tiktok: { icon: TikTokIcon, label: 'TikTok' },
  discord: { icon: DiscordIcon, label: 'Discord' },
}

export default function PlatformStatusIndicators({
  configuredSources,
  activeChannels,
  channelStatuses,
  variant = 'fixed',
}: PlatformStatusIndicatorsProps) {
  const [countdowns, setCountdowns] = useState<Map<string, number>>(new Map())

  // Update countdown timers every second
  useEffect(() => {
    const interval = setInterval(() => {
      const newCountdowns = new Map<string, number>()

      channelStatuses.forEach((status, channelId) => {
        if (status.status === 'reconnecting' && status.next_retry_at) {
          const nextRetry = new Date(status.next_retry_at).getTime()
          const now = Date.now()
          const secondsRemaining = Math.max(0, Math.ceil((nextRetry - now) / 1000))
          newCountdowns.set(channelId, secondsRemaining)
        }
      })

      setCountdowns(newCountdowns)
    }, 1000)

    return () => clearInterval(interval)
  }, [channelStatuses])

  // Don't render if config hasn't loaded yet
  if (configuredSources.size === 0) return null

  const entries = Array.from(configuredSources.entries())

  return (
    <div
      className={clsx(
        'platform-status-indicators flex gap-2 rounded-lg bg-bg/80 px-3 py-2 shadow-lg backdrop-blur-sm',
        variant === 'fixed' && 'fixed top-4 right-4 z-50'
      )}
    >
      {entries.map(([channelId, source]) => {
        const platformDef = platformIcons[source.platform]
        if (!platformDef) return null

        const isActive = activeChannels.has(channelId)
        const status = channelStatuses.get(channelId)
        const countdown = countdowns.get(channelId)
        const Icon = platformDef.icon
        const platformLabel = platformDef.label

        // Determine status class
        let statusClass = isActive ? 'bg-white/10' : 'opacity-40 bg-surface/50'
        let tooltipText = `${platformLabel} - ${source.channelName} ${isActive ? '(Active)' : '(Inactive)'}`

        if (status) {
          if (status.status === 'reconnecting' && countdown !== undefined) {
            statusClass = 'bg-yellow-500/20 opacity-100'
            if (status.error_message) {
              tooltipText = `${platformLabel} - ${source.channelName} - ${status.error_message} (retry in ${countdown}s)`
            } else {
              tooltipText = `${platformLabel} - ${source.channelName} - Reconnecting in ${countdown}s`
            }
          } else if (status.status === 'quota_exceeded') {
            statusClass = 'bg-red-500/20 opacity-100'
            tooltipText = `${platformLabel} - ${source.channelName} - Quota exceeded`
          } else if (status.status === 'error') {
            statusClass = 'bg-red-500/20 opacity-100 border border-red-500/50'
            tooltipText = status.error_message
              ? `${platformLabel} - ${source.channelName} - ${status.error_message}`
              : `${platformLabel} - ${source.channelName} - Error`
          } else if (status.status === 'offline') {
            // Check if offline is due to auth error
            const isAuthError =
              status.error_message?.toLowerCase().includes('oauth') ||
              status.error_message?.toLowerCase().includes('token')

            if (isAuthError) {
              statusClass = 'bg-red-500/20 opacity-100 border border-red-500/50'
              tooltipText = `${platformLabel} - ${source.channelName} - Auth Required`
            } else {
              statusClass = 'opacity-20 bg-surface/50'
              tooltipText = status.error_message
                ? `${platformLabel} - ${source.channelName} - ${status.error_message}`
                : `${platformLabel} - ${source.channelName} - Offline`
            }
          } else if (status.status === 'connected') {
            statusClass = 'bg-green-500/20 opacity-100'
            tooltipText = `${platformLabel} - ${source.channelName} (Connected)`
          }
        }

        return (
          <div
            key={channelId}
            className={clsx(
              'platform-indicator',
              'relative flex h-8 w-8 items-center justify-center rounded-md transition-all duration-300',
              statusClass
            )}
            data-platform={source.platform}
            data-channel-id={channelId}
            title={tooltipText}
          >
            <Icon />
            {status?.status === 'reconnecting' && countdown !== undefined && (
              <div className="absolute -right-1 -bottom-1 rounded bg-yellow-500 px-1 font-mono text-xs text-white">
                {countdown}s
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}
