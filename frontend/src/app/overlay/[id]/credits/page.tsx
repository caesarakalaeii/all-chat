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
 * Credit Roll Display Page (Unauthenticated)
 *
 * Public endpoint for displaying end-of-stream credits in OBS.
 * Shows leaderboards for subs, bits, raids, donations, follows, etc.
 *
 * Features:
 * - Scrolling credits with theme support
 * - Leaderboard categories
 * - User avatars and platform badges
 * - Configurable styling
 * - Auto-loop or single-play based on config
 *
 * This is a Client Component for animation and dynamic rendering.
 */

'use client'

import Image from 'next/image'
import { use, useEffect, useState, useRef } from 'react'
import clsx from 'clsx'
import Script from 'next/script'
import { useTranslations, type TFunction } from '@/lib/i18n'
import type { CreditRollResponse, CreditRollConfig, LeaderboardEntry } from '@/lib/types/overlay'

// Decoration: the warning triangle sits above copy naming the failure, and the
// film clapper and camera each precede a heading that says the same in words.
const WARNING_GLYPH = '⚠️'
const CAMERA_GLYPH = '🎥'

/**
 * Render the session length the way the roll header does: hours and minutes if
 * both are non-zero, one of them alone otherwise, and a fallback for a session
 * that has only just begun.
 *
 * English pluralisation is a per-language rule, so each form is its own catalog
 * key rather than an 's' appended at the render site.
 */
function sessionDuration(t: TFunction, totalSeconds: number): string {
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const hourText =
    hours === 1
      ? t('viewerOverlay.credits.durationHourOne', { hours })
      : t('viewerOverlay.credits.durationHourMany', { hours })
  const minuteText =
    minutes === 1
      ? t('viewerOverlay.credits.durationMinuteOne', { minutes })
      : t('viewerOverlay.credits.durationMinuteMany', { minutes })
  if (hours > 0 && minutes > 0)
    return t('viewerOverlay.credits.durationHoursAndMinutes', {
      hours: hourText,
      minutes: minuteText,
    })
  if (hours > 0) return hourText
  if (minutes > 0) return minuteText
  return t('viewerOverlay.credits.durationJustStarted')
}

declare global {
  interface Window {
    Twitch: any
  }
}

export default function CreditRollPage({ params }: { params: Promise<{ id: string }> }) {
  const t = useTranslations()
  const { id } = use(params)
  const [creditData, setCreditData] = useState<CreditRollResponse | null>(null)
  const [config, setConfig] = useState<CreditRollConfig | null>(null)
  const [customCss, setCustomCss] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [currentClipIndex, setCurrentClipIndex] = useState(0)
  const [twitchReady, setTwitchReady] = useState(false)
  const playerRef = useRef<any>(null)
  const playerContainerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const loadCreditRoll = async () => {
      try {
        // Load config
        const configResponse = await fetch(`/api/v1/overlays/public/${id}/creditroll`)
        if (configResponse.ok) {
          const configData = await configResponse.json()
          setConfig(configData)
          setCustomCss(configData.custom_css || '')
        }

        // Load credit roll data
        const dataResponse = await fetch(`/api/v1/overlays/public/${id}/credit-roll`)
        if (!dataResponse.ok) {
          const errorData = await dataResponse.json()
          throw new Error(errorData.error || t('viewerOverlay.credits.loadFailed'))
        }

        const data = await dataResponse.json()
        setCreditData(data)
      } catch (err) {
        console.error('Failed to load credit roll:', err)
        setError(err instanceof Error ? err.message : t('viewerOverlay.credits.loadFailed'))
      } finally {
        setLoading(false)
      }
    }

    loadCreditRoll()
  }, [id, t])

  // Initialize Twitch Player when clip changes
  useEffect(() => {
    if (!twitchReady || !creditData?.clips || creditData.clips.length === 0) return
    if (!playerContainerRef.current) return

    const currentClip = creditData.clips[currentClipIndex]
    if (!currentClip) return

    // Destroy previous player
    if (playerRef.current) {
      playerRef.current = null
    }

    // Clear container
    playerContainerRef.current.innerHTML = ''

    // Create new player
    const options = {
      width: '100%',
      height: '100%',
      clip: currentClip.id,
      parent: [window.location.hostname],
      autoplay: true,
      muted: config?.clips_muted ?? true,
    }

    try {
      playerRef.current = new window.Twitch.Embed(playerContainerRef.current, options)

      // Listen for player ready event
      playerRef.current.addEventListener(window.Twitch.Embed.VIDEO_READY, () => {
        const player = playerRef.current.getPlayer()

        // Auto-advance when clip ends
        player.addEventListener(window.Twitch.Player.ENDED, () => {
          setCurrentClipIndex((prev) => (prev + 1) % creditData.clips.length)
        })

        // Fallback timer in case ENDED event doesn't fire
        const duration = (currentClip.duration || 30) * 1000 + 3000
        setTimeout(() => {
          if (playerRef.current) {
            setCurrentClipIndex((prev) => (prev + 1) % creditData.clips.length)
          }
        }, duration)
      })
    } catch (err) {
      console.error('Failed to initialize Twitch player:', err)
    }
  }, [twitchReady, creditData, currentClipIndex, config?.clips_muted])

  const renderLeaderboard = (
    title: string,
    entries: LeaderboardEntry[] | undefined,
    emoji: string
  ) => {
    if (!entries || entries.length === 0) return null

    return (
      <div className="mb-12">
        <h2 className="mb-6 flex items-center gap-3 text-4xl font-bold text-white">
          <span className="text-5xl">{emoji}</span>
          {title}
        </h2>
        <div className="space-y-4">
          {entries.map((entry, index) => (
            <div
              key={index}
              className={clsx(
                'flex items-center gap-4 rounded-lg p-4',
                index === 0 && 'border-2 border-yellow-500 bg-yellow-500/20',
                index === 1 && 'border-2 border-slate-400 bg-slate-400/20',
                index === 2 && 'border-2 border-orange-600 bg-orange-600/20',
                index > 2 && 'border border-slate-700 bg-slate-800/50'
              )}
            >
              <div
                className={clsx(
                  'w-12 text-center text-3xl font-bold',
                  index === 0 && 'text-yellow-400',
                  index === 1 && 'text-slate-300',
                  index === 2 && 'text-orange-500',
                  index > 2 && 'text-slate-500'
                )}
              >
                #{entry.rank}
              </div>
              {entry.avatar_url && (
                <div className="relative h-12 w-12 overflow-hidden rounded-full">
                  <Image
                    src={entry.avatar_url}
                    alt={entry.display_name}
                    fill
                    className="object-cover"
                  />
                </div>
              )}
              <div className="flex-1">
                <div className="text-xl font-semibold text-white">{entry.display_name}</div>
                <div className="text-sm text-slate-400 capitalize">{entry.platform}</div>
              </div>
              <div className="text-2xl font-bold text-white">
                {entry.total_value !== undefined && entry.total_value > 0
                  ? `$${entry.total_value.toFixed(2)}`
                  : entry.count || ''}
              </div>
            </div>
          ))}
        </div>
      </div>
    )
  }

  if (loading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-linear-to-b from-slate-900 to-black">
        <div className="text-center">
          <div className="mx-auto mb-4 h-16 w-16 animate-spin rounded-full border-b-4 border-white"></div>
          <p className="text-xl text-white">{t('viewerOverlay.credits.loading')}</p>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-linear-to-b from-slate-900 to-black">
        <div className="text-center">
          <div className="mb-4 text-6xl">{WARNING_GLYPH}</div>
          <p className="mb-2 text-xl text-white">{t('viewerOverlay.credits.errorHeading')}</p>
          <p className="text-sm text-slate-400">{error}</p>
          <p className="mt-4 text-xs text-slate-500">{t('viewerOverlay.credits.errorHint')}</p>
        </div>
      </div>
    )
  }

  if (!creditData) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-linear-to-b from-slate-900 to-black">
        <div className="text-center">
          <p className="text-xl text-white">{t('viewerOverlay.credits.empty')}</p>
        </div>
      </div>
    )
  }

  const theme = config?.theme || 'cinematic'
  const bgOpacity = config?.background_opacity || 0.8

  const currentClip = creditData?.clips?.[currentClipIndex]

  return (
    <>
      {/* Load Twitch Embed SDK */}
      <Script
        src="https://embed.twitch.tv/embed/v1.js"
        onLoad={() => setTwitchReady(true)}
        strategy="afterInteractive"
      />

      {/* Custom CSS Injection */}
      {customCss && customCss.trim() && <style dangerouslySetInnerHTML={{ __html: customCss }} />}

      {/* Background Clip Video */}
      {config?.clips_enabled && currentClip && (
        <div className="fixed inset-0 z-0">
          <div
            ref={playerContainerRef}
            id="twitch-clip-player"
            className="absolute inset-0 h-full w-full"
          />
          {/* Overlay gradient for better text readability */}
          <div
            className="absolute inset-0 z-10"
            style={{
              background: `linear-gradient(to bottom, rgba(0, 0, 0, ${bgOpacity * 0.7}), rgba(0, 0, 0, ${bgOpacity * 0.5}))`,
              pointerEvents: 'none',
            }}
          />
        </div>
      )}

      <div
        className="relative z-10 min-h-screen overflow-hidden"
        style={{
          background:
            !config?.clips_enabled || !currentClip
              ? theme === 'cinematic'
                ? `linear-gradient(to bottom, rgba(17, 24, 39, ${bgOpacity}), rgba(0, 0, 0, ${bgOpacity}))`
                : theme === 'modern'
                  ? `linear-gradient(135deg, rgba(99, 102, 241, ${bgOpacity * 0.3}), rgba(139, 92, 246, ${bgOpacity * 0.3}))`
                  : `rgba(17, 24, 39, ${bgOpacity})`
              : 'transparent',
        }}
      >
        <div className="container mx-auto px-8 py-12">
          {/* Header */}
          <div className="mb-16 text-center">
            <h1 className="animate-fade-in mb-4 text-6xl font-bold text-white">
              {t('viewerOverlay.credits.heading')}
            </h1>
            <p className="text-2xl text-slate-300">{t('viewerOverlay.credits.subheading')}</p>
            <div className="mt-4 text-slate-400">
              <p>
                {t('viewerOverlay.credits.session', {
                  date: new Date(creditData.session_started_at).toLocaleDateString(),
                })}
              </p>
              <p>
                {t('viewerOverlay.credits.duration', {
                  duration: sessionDuration(t, creditData.session_duration_seconds),
                })}
              </p>
            </div>
          </div>

          {/* Leaderboards */}
          <div className="mx-auto max-w-4xl">
            {renderLeaderboard(
              t('viewerOverlay.credits.topSubscribers'),
              creditData.leaderboards.subs,
              '⭐'
            )}
            {renderLeaderboard(
              t('viewerOverlay.credits.topGifters'),
              creditData.leaderboards.gifts,
              '🎁'
            )}
            {renderLeaderboard(
              t('viewerOverlay.credits.topCheerers'),
              creditData.leaderboards.bits,
              '💎'
            )}
            {renderLeaderboard(
              t('viewerOverlay.credits.topChannelPoints'),
              creditData.leaderboards.points,
              '🎯'
            )}
            {renderLeaderboard(
              t('viewerOverlay.credits.topRaiders'),
              creditData.leaderboards.raids,
              '⚔️'
            )}
            {renderLeaderboard(
              t('viewerOverlay.credits.topSuperChats'),
              creditData.leaderboards.super_chats,
              '💰'
            )}
            {renderLeaderboard(
              t('viewerOverlay.credits.newFollowers'),
              creditData.leaderboards.follows,
              '❤️'
            )}
          </div>

          {/* Now Playing Indicator */}
          {config?.clips_enabled && currentClip && (
            <div className="fixed right-8 bottom-8 z-50">
              <div className="max-w-sm rounded-lg border border-slate-700 bg-black/80 p-4 shadow-2xl backdrop-blur-sm">
                <div className="mb-2 flex items-center gap-3">
                  <span className="text-2xl">{CAMERA_GLYPH}</span>
                  <span className="font-semibold text-white">
                    {t('viewerOverlay.credits.nowPlaying')}
                  </span>
                </div>
                <div className="mb-1 text-lg font-medium text-white">{currentClip.title}</div>
                <div className="flex items-center justify-between text-sm text-slate-400">
                  <span>
                    {t('viewerOverlay.credits.clipViews', {
                      views: currentClip.view_count.toLocaleString(),
                    })}
                  </span>
                  <span>
                    {t('viewerOverlay.credits.clipCounter', {
                      index: currentClipIndex + 1,
                      total: creditData?.clips?.length || 0,
                    })}
                  </span>
                </div>
              </div>
            </div>
          )}

          {/* Footer */}
          <div className="mt-24 mb-12 text-center">
            <div className="mb-4 text-4xl font-bold text-white">
              {t('viewerOverlay.credits.thanks')}
            </div>
            <p className="text-xl text-slate-300">{t('viewerOverlay.credits.seeYou')}</p>
          </div>
        </div>
      </div>

      {/*
       * The roll header's entrance animation. Injected the way the sibling
       * overlay routes inject their global CSS (see overlay/layout.tsx and
       * overlay/[id]/view/layout.tsx) rather than through styled-jsx: both emit
       * the same global rules, and this shape keeps the stylesheet out of a JSX
       * text node, which the i18n gate reads as copy. Hoisting the styled-jsx
       * template to a constant is not an option — styled-jsx only transforms an
       * inline literal, so a hoisted one ships unscoped and unminified.
       */}
      <style
        dangerouslySetInnerHTML={{
          __html: `
        @keyframes fade-in {
          from {
            opacity: 0;
            transform: translateY(-20px);
          }
          to {
            opacity: 1;
            transform: translateY(0);
          }
        }

        .animate-fade-in {
          animation: fade-in 1s ease-out;
        }
      `,
        }}
      />
    </>
  )
}
