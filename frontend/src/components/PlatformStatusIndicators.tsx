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
import { DiscordIcon as DiscordMark } from '@/components/icons/DiscordIcon'
import type { PlatformStatus } from '@/lib/types/message'
import { useTranslations, type MessageKey, type TFunction } from '@/lib/i18n'

export interface SourceInfo {
  platform: string
  channelId: string
  channelName: string
}

interface PlatformStatusIndicatorsProps {
  /**
   * All three collections are keyed alike, by `sourceKey()` from
   * `@/core/overlayStreamCore` — a channel id alone collides between
   * platforms. This component only ever joins them on that key, so it never
   * has to build one.
   */
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
      {KICK_GLYPH}
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

const DiscordIcon = () => <DiscordMark className="h-5 w-5 text-discord" />

// The letter the Kick mark draws. A brand glyph, not copy.
const KICK_GLYPH = 'K'

// The icon plus the catalog key naming the platform. `as const satisfies` rather
// than an annotation: an annotation widens the key to string and a typo would
// stop failing tsc.
const platformIcons = {
  twitch: { icon: TwitchIcon, nameKey: 'common.platforms.twitch' },
  youtube: { icon: YouTubeIcon, nameKey: 'common.platforms.youtube' },
  kick: { icon: KickIcon, nameKey: 'common.platforms.kick' },
  tiktok: { icon: TikTokIcon, nameKey: 'common.platforms.tiktok' },
  discord: { icon: DiscordIcon, nameKey: 'common.platforms.discord' },
} as const satisfies Record<string, { icon: React.FC; nameKey: MessageKey }>

type KnownPlatform = keyof typeof platformIcons

/**
 * The icon and name key for a stored platform value, or undefined when this
 * build does not know it. A stored source can name a platform an older or newer
 * release added, and indexing the const table with a bare string would need a
 * cast to do it.
 */
function platformDefinition(platform: string): (typeof platformIcons)[KnownPlatform] | undefined {
  return platform in platformIcons ? platformIcons[platform as KnownPlatform] : undefined
}

/**
 * The Tailwind classes for one channel's indicator. Split out of the render body
 * alongside indicatorTooltip so the two stay in step: they branch on exactly the
 * same status values, and a status handled by one but not the other renders a
 * colour that contradicts its own tooltip.
 */
function indicatorClass(
  isActive: boolean,
  status: PlatformStatus | undefined,
  countdown: number | undefined
): string {
  if (!status) return isActive ? 'bg-white/10' : 'opacity-40 bg-surface/50'
  if (status.status === 'reconnecting' && countdown !== undefined) {
    return 'bg-yellow-500/20 opacity-100'
  }
  if (status.status === 'quota_exceeded') return 'bg-red-500/20 opacity-100'
  if (status.status === 'error') return 'bg-red-500/20 opacity-100 border border-red-500/50'
  // Discovery gave up after the 1h cap and is parked awaiting a manual
  // re-trigger (chat monitor → Rediscover). Distinct from a red error: nothing
  // is broken, an action is needed.
  if (status.status === 'paused') {
    return 'bg-indigo-500/20 opacity-100 border border-indigo-500/40'
  }
  if (status.status === 'offline') {
    return isAuthError(status)
      ? 'bg-red-500/20 opacity-100 border border-red-500/50'
      : 'opacity-20 bg-surface/50'
  }
  if (status.status === 'connected') return 'bg-green-500/20 opacity-100'
  return isActive ? 'bg-white/10' : 'opacity-40 bg-surface/50'
}

/** An offline channel whose error names OAuth or a token needs re-authorising. */
function isAuthError(status: PlatformStatus): boolean {
  const message = status.error_message?.toLowerCase()
  return !!message && (message.includes('oauth') || message.includes('token'))
}

/**
 * One channel's tooltip. Takes the translator first because a plain module
 * function cannot call a hook. Each branch resolves a whole sentence rather than
 * concatenating a name, a separator and a status phrase.
 */
function indicatorTooltip(
  t: TFunction,
  platform: string,
  channel: string,
  isActive: boolean,
  status: PlatformStatus | undefined,
  countdown: number | undefined
): string {
  if (!status) {
    return isActive
      ? t('viewerOverlay.statusIndicator.active', { platform, channel })
      : t('viewerOverlay.statusIndicator.inactive', { platform, channel })
  }
  // A backend error_message is not copy, so it is passed through as a param.
  const error = status.error_message
  if (status.status === 'reconnecting' && countdown !== undefined) {
    return error
      ? t('viewerOverlay.statusIndicator.reconnectingWithError', {
          platform,
          channel,
          error,
          seconds: countdown,
        })
      : t('viewerOverlay.statusIndicator.reconnecting', {
          platform,
          channel,
          seconds: countdown,
        })
  }
  if (status.status === 'quota_exceeded') {
    return t('viewerOverlay.statusIndicator.quotaExceeded', { platform, channel })
  }
  if (status.status === 'error') {
    return error
      ? t('viewerOverlay.statusIndicator.withErrorMessage', { platform, channel, error })
      : t('viewerOverlay.statusIndicator.error', { platform, channel })
  }
  if (status.status === 'paused') {
    return error
      ? t('viewerOverlay.statusIndicator.withErrorMessage', { platform, channel, error })
      : t('viewerOverlay.statusIndicator.discoveryPaused', { platform, channel })
  }
  if (status.status === 'offline') {
    if (isAuthError(status)) {
      return t('viewerOverlay.statusIndicator.authRequired', { platform, channel })
    }
    return error
      ? t('viewerOverlay.statusIndicator.withErrorMessage', { platform, channel, error })
      : t('viewerOverlay.statusIndicator.offline', { platform, channel })
  }
  if (status.status === 'connected') {
    return t('viewerOverlay.statusIndicator.connected', { platform, channel })
  }
  return isActive
    ? t('viewerOverlay.statusIndicator.active', { platform, channel })
    : t('viewerOverlay.statusIndicator.inactive', { platform, channel })
}

export default function PlatformStatusIndicators({
  configuredSources,
  activeChannels,
  channelStatuses,
  variant = 'fixed',
}: PlatformStatusIndicatorsProps) {
  const t = useTranslations()
  const [countdowns, setCountdowns] = useState<Map<string, number>>(new Map())

  // Update countdown timers every second
  useEffect(() => {
    const interval = setInterval(() => {
      const newCountdowns = new Map<string, number>()

      channelStatuses.forEach((status, key) => {
        if (status.status === 'reconnecting' && status.next_retry_at) {
          const nextRetry = new Date(status.next_retry_at).getTime()
          const now = Date.now()
          const secondsRemaining = Math.max(0, Math.ceil((nextRetry - now) / 1000))
          newCountdowns.set(key, secondsRemaining)
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
      {entries.map(([key, source]) => {
        const platformDef = platformDefinition(source.platform)
        if (!platformDef) return null

        const isActive = activeChannels.has(key)
        const status = channelStatuses.get(key)
        const countdown = countdowns.get(key)
        const Icon = platformDef.icon
        const statusClass = indicatorClass(isActive, status, countdown)
        const tooltipText = indicatorTooltip(
          t,
          t(platformDef.nameKey),
          source.channelName,
          isActive,
          status,
          countdown
        )

        return (
          <div
            key={key}
            className={clsx(
              'platform-indicator',
              'relative flex h-8 w-8 items-center justify-center rounded-md transition-all duration-300',
              statusClass
            )}
            data-platform={source.platform}
            data-channel-id={source.channelId}
            title={tooltipText}
          >
            <Icon />
            {status?.status === 'reconnecting' && countdown !== undefined && (
              <div className="absolute -right-1 -bottom-1 rounded bg-yellow-500 px-1 font-mono text-xs text-white">
                {t('viewerOverlay.statusIndicator.countdownSeconds', { seconds: countdown })}
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}
