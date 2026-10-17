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
 * - Credits that scroll up at the configured speed and start over once the
 *   configured display duration is up
 * - Leaderboard categories
 * - User avatars and platform badges
 * - Configurable styling
 * - Twitch clips playing full-screen behind the roll, one after another, with
 *   sound in OBS unless muted (ADR-0076)
 *
 * This is a Client Component for animation and dynamic rendering.
 */

'use client'

import Image from 'next/image'
import { type SyntheticEvent, use, useEffect, useLayoutEffect, useState, useRef } from 'react'
import clsx from 'clsx'
import { type TFunction, formatDate, formatNumber, useTranslations } from '@/lib/i18n'
import type { CreditRollResponse, CreditRollConfig, LeaderboardEntry } from '@/lib/types/overlay'
import { creditRollValue } from '@/lib/utils/creditRollValue'
import { fetchClipVideoUrl } from '@/lib/twitchClipVideo'

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

export default function CreditRollPage({ params }: { params: Promise<{ id: string }> }) {
  const t = useTranslations()
  const { id } = use(params)
  const [creditData, setCreditData] = useState<CreditRollResponse | null>(null)
  const [config, setConfig] = useState<CreditRollConfig | null>(null)
  const [customCss, setCustomCss] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  // Counts clip plays rather than indexing clips, so a single clip replays: the
  // count still changes, which remounts the player.
  const [clipPlay, setClipPlay] = useState(0)
  // The current clip's MP4 (ADR-0076), tagged with the play it was looked up
  // for so a previous clip's URL never shows. A null URL means the lookup or
  // the playback failed, and the clip plays through Twitch's embed instead.
  const [clipVideo, setClipVideo] = useState<{ play: number; url: string | null }>()
  const rollRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const loadCreditRoll = async () => {
      try {
        const configResponse = await fetch(`/api/v1/overlays/public/${id}/creditroll`)
        if (configResponse.ok) {
          const configData = await configResponse.json()
          setConfig(configData)
          setCustomCss(configData.custom_css || '')
        }

        const dataResponse = await fetch(`/api/v1/overlays/public/${id}/credit-roll`)
        if (!dataResponse.ok) {
          const errorData = await dataResponse.json()
          throw new Error(errorData.error || t('viewerOverlay.credits.loadFailed'))
        }

        const data = await dataResponse.json()
        setCreditData(data)
        setClipPlay(0)
        setError(null)
      } catch (err) {
        console.error('Failed to load credit roll:', err)
        setError(err instanceof Error ? err.message : t('viewerOverlay.credits.loadFailed'))
      } finally {
        setLoading(false)
      }
    }

    // OBS loads a browser source when OBS starts, not when its scene goes live,
    // so a roll fetched once would show the stream's start state and ignore any
    // settings saved since. Re-fetch each time the source becomes active.
    const onActiveChanged = (e: Event) => {
      if ((e as CustomEvent<{ active?: boolean }>).detail?.active) loadCreditRoll()
    }

    loadCreditRoll()
    window.addEventListener('obsSourceActiveChanged', onActiveChanged)
    return () => window.removeEventListener('obsSourceActiveChanged', onActiveChanged)
  }, [id, t])

  const clips = creditData?.clips ?? []
  const currentClip = config?.clips_enabled ? clips[clipPlay % clips.length] : undefined
  const clipVideoUrl = clipVideo?.play === clipPlay ? clipVideo.url : undefined
  const nextClip = () => setClipPlay((n) => n + 1)

  // The clip renders muted, since Twitch offers no captions for it (a broadcast
  // surface, see docs/ACCESSIBILITY.md), and takes the streamer's audio choice
  // once it can play. OBS lets it play with sound; an ordinary browser tab
  // refuses, so there it plays muted rather than standing still.
  const playClip = (e: SyntheticEvent<HTMLVideoElement>) => {
    const video = e.currentTarget
    video.muted = config?.clips_muted ?? true
    video.play().catch(() => {
      video.muted = true
      video.play().catch(() => {})
    })
  }

  useEffect(() => {
    if (!currentClip) return
    let cancelled = false
    fetchClipVideoUrl(currentClip.id).then((url) => {
      if (!cancelled) setClipVideo({ play: clipPlay, url })
    })
    return () => {
      cancelled = true
    }
  }, [currentClip, clipPlay])

  // Twitch's clip embed reports no end event, so the next clip starts once this
  // one's length, plus slack for the player to load, has passed.
  useEffect(() => {
    if (!currentClip || clipVideoUrl !== null) return
    const timer = setTimeout(
      () => setClipPlay((n) => n + 1),
      (currentClip.duration || 30) * 1000 + 3000
    )
    return () => clearTimeout(timer)
  }, [currentClip, clipPlay, clipVideoUrl])

  const scrollSpeed = config?.scroll_speed || 50
  const displaySeconds = config?.display_duration_seconds || 60

  // One pass carries the roll from just below the screen until its last line
  // reaches the bottom edge, at scroll_speed pixels per second. The roll then
  // holds until display_duration_seconds is up and starts over, so a short roll
  // still fills the configured time and a long one is never cut off. A layout
  // effect, so the roll never paints unmoved at the top first.
  useLayoutEffect(() => {
    const roll = rollRef.current
    if (!roll) return
    const passMs = (roll.offsetHeight / scrollSpeed) * 1000
    const cycleMs = Math.max(passMs, displaySeconds * 1000)
    const end = 'translateY(calc(100vh - 100%))'
    const animation = roll.animate(
      [
        { transform: 'translateY(100vh)' },
        { transform: end, offset: passMs / cycleMs },
        { transform: end },
      ],
      { duration: cycleMs, iterations: Infinity }
    )
    return () => animation.cancel()
  }, [creditData, scrollSpeed, displaySeconds])

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
              <div className="text-2xl font-bold text-white">{creditRollValue(t, entry)}</div>
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
  const bgOpacity = config?.background_opacity ?? 0.8

  return (
    <>
      {/* Custom CSS Injection */}
      {customCss && customCss.trim() && <style dangerouslySetInnerHTML={{ __html: customCss }} />}

      {/* Background Clip Video */}
      {currentClip && (
        <div className="fixed inset-0 z-0">
          {clipVideoUrl ? (
            <video
              key={clipPlay}
              id="twitch-clip-player"
              src={clipVideoUrl}
              muted
              playsInline
              onLoadedData={playClip}
              onEnded={nextClip}
              onError={() => setClipVideo({ play: clipPlay, url: null })}
              className="absolute inset-0 h-full w-full object-cover"
            />
          ) : (
            clipVideoUrl === null && (
              <iframe
                key={clipPlay}
                id="twitch-clip-player"
                src={`https://clips.twitch.tv/embed?clip=${encodeURIComponent(currentClip.id)}&parent=${window.location.hostname}&autoplay=true&muted=${config?.clips_muted ?? true}`}
                title={currentClip.title}
                allow="autoplay; fullscreen"
                className="absolute inset-0 h-full w-full border-0"
              />
            )
          )}
          {/* Overlay gradient for better text readability */}
          <div
            className="absolute inset-0 z-10"
            style={{
              background: `linear-gradient(to bottom, rgba(0, 0, 0, ${bgOpacity}), rgba(0, 0, 0, ${bgOpacity * 0.85}))`,
              pointerEvents: 'none',
            }}
          />
        </div>
      )}

      <div
        className="relative z-10 h-screen min-h-screen overflow-hidden"
        style={{
          background: !currentClip
            ? theme === 'cinematic'
              ? `linear-gradient(to bottom, rgba(17, 24, 39, ${bgOpacity}), rgba(0, 0, 0, ${bgOpacity}))`
              : theme === 'modern'
                ? `linear-gradient(135deg, rgba(99, 102, 241, ${bgOpacity}), rgba(139, 92, 246, ${bgOpacity}))`
                : `rgba(17, 24, 39, ${bgOpacity})`
            : 'transparent',
        }}
      >
        <div ref={rollRef} className="container mx-auto px-8 py-12">
          {/* Header */}
          <div className="mb-16 text-center">
            <h1 className="animate-fade-in mb-4 text-6xl font-bold text-white">
              {t('viewerOverlay.credits.heading')}
            </h1>
            <p className="text-2xl text-slate-300">{t('viewerOverlay.credits.subheading')}</p>
            <div className="mt-4 text-slate-400">
              <p>
                {t('viewerOverlay.credits.session', {
                  date: formatDate(new Date(creditData.session_started_at)),
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
            {renderLeaderboard(
              t('viewerOverlay.credits.topMembers'),
              creditData.leaderboards.memberships,
              '🌟'
            )}
            {renderLeaderboard(
              t('viewerOverlay.credits.topCoinChests'),
              creditData.leaderboards.treasure_chests,
              '🪙'
            )}
            {renderLeaderboard(
              t('viewerOverlay.credits.topSharers'),
              creditData.leaderboards.shares,
              '🔗'
            )}
            {renderLeaderboard(
              t('viewerOverlay.credits.topLikers'),
              creditData.leaderboards.likes,
              '👍'
            )}
          </div>

          {/* Footer */}
          <div className="mt-24 mb-12 text-center">
            <div className="mb-4 text-4xl font-bold text-white">
              {t('viewerOverlay.credits.thanks')}
            </div>
            <p className="text-xl text-slate-300">{t('viewerOverlay.credits.seeYou')}</p>
          </div>
        </div>
      </div>

      {/* Now Playing Indicator, outside the roll so its transform cannot move it */}
      {currentClip && (
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
                  views: formatNumber(currentClip.view_count),
                })}
              </span>
              <span>
                {t('viewerOverlay.credits.clipCounter', {
                  index: (clipPlay % clips.length) + 1,
                  total: clips.length,
                })}
              </span>
            </div>
          </div>
        </div>
      )}

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
