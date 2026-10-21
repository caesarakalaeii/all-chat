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

import { UserAvatar } from '@/components/UserAvatar'
import { type SyntheticEvent, use, useEffect, useLayoutEffect, useState, useRef } from 'react'
import clsx from 'clsx'
import { type TFunction, formatDate, formatNumber, useTranslations } from '@/lib/i18n'
import type { CreditRollResponse, CreditRollConfig, LeaderboardEntry } from '@/lib/types/overlay'
import { creditRollValue } from '@/lib/utils/creditRollValue'
import {
  CREDIT_ROLL_PREVIEW_MESSAGE,
  sampleCreditRoll,
  creditFontSize,
  withCreditTexts,
} from '@/lib/utils/creditRollCustomization'
import { fetchClipVideoUrl } from '@/lib/twitchClipVideo'

// Decoration: the warning triangle sits above copy naming the failure, and the
// film clapper and camera each precede a heading that says the same in words.
const WARNING_GLYPH = '⚠️'
const CAMERA_GLYPH = '🎥'

// Darkest the scrim over a playing clip gets, at background opacity 1.
const CLIP_SCRIM_MAX = 0.5

// The OBS canvas the roll is laid out for. The editor preview zooms the page to
// fit this width into its pane, so sizes there read the way they will on stream.
const PREVIEW_CANVAS_WIDTH = 1920

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

/**
 * The session line's values. {date} follows the UI language; day, month and
 * year let streamer-written copy order the date its own way ("17/10/2026").
 */
function sessionDateParams(start: Date) {
  const pad = (n: number) => String(n).padStart(2, '0')
  return {
    date: formatDate(start),
    day: pad(start.getDate()),
    month: pad(start.getMonth() + 1),
    year: start.getFullYear(),
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
  // Counts clip plays rather than indexing clips, so a single clip replays: the
  // count still changes, which remounts the player.
  const [clipPlay, setClipPlay] = useState(0)
  // The current clip's MP4 (ADR-0076), tagged with the play it was looked up
  // for so a previous clip's URL never shows. A null URL means the lookup or
  // the playback failed, and the clip plays through Twitch's embed instead.
  const [clipVideo, setClipVideo] = useState<{ play: number; url: string | null }>()
  const rollRef = useRef<HTMLDivElement>(null)

  // The editor's live preview (?preview=true): sample data instead of the real
  // roll, whose fetch counts as a display, and the editor's unsaved config over
  // postMessage instead of the saved one.
  const [preview] = useState(
    () =>
      typeof window !== 'undefined' &&
      new URLSearchParams(window.location.search).get('preview') === 'true'
  )
  const [previewZoom, setPreviewZoom] = useState(1)

  useEffect(() => {
    if (!preview) return
    const fit = () => setPreviewZoom(window.innerWidth / PREVIEW_CANVAS_WIDTH)
    fit()
    window.addEventListener('resize', fit)

    // The editor answers EMBED_READY with its current config and re-sends on
    // every change, so the preview never reads the saved one.
    const onMessage = (event: MessageEvent) => {
      // Same-origin only: the editor iframes this page from its own origin.
      if (event.origin !== window.location.origin) return
      if (event.data?.type !== CREDIT_ROLL_PREVIEW_MESSAGE) return
      const next = event.data.config as CreditRollConfig
      setConfig(next)
      setCustomCss(next.custom_css || '')
      setCreditData(sampleCreditRoll(next))
      setLoading(false)
    }
    window.addEventListener('message', onMessage)
    window.parent.postMessage({ type: 'EMBED_READY' }, window.location.origin)

    return () => {
      window.removeEventListener('resize', fit)
      window.removeEventListener('message', onMessage)
    }
  }, [preview])

  useEffect(() => {
    if (preview) return
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
  }, [id, t, preview])

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
  const ct = withCreditTexts(t, config?.texts)
  const fontSize = (element: Parameters<typeof creditFontSize>[1], ratio?: number) =>
    creditFontSize(config?.font_scales, element, ratio)
  const avatarSize = Math.round((48 * (config?.font_scales?.entry ?? 100)) / 100)

  // One pass carries the roll from just below the screen until its last line
  // reaches the bottom edge, at scroll_speed pixels per second. The roll then
  // holds until display_duration_seconds is up and starts over, so a short roll
  // still fills the configured time and a long one is never cut off. A layout
  // effect, so the roll never paints unmoved at the top first. The preview holds
  // still, so the text being edited stays readable.
  useLayoutEffect(() => {
    const roll = rollRef.current
    if (!roll || preview) return
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
  }, [creditData, scrollSpeed, displaySeconds, preview])

  const renderLeaderboard = (
    title: string,
    entries: LeaderboardEntry[] | undefined,
    emoji: string
  ) => {
    if (!entries || entries.length === 0) return null

    return (
      <div className="mb-12">
        <h2
          className="mb-6 flex items-center gap-3 text-4xl font-bold text-white"
          style={{ fontSize: fontSize('category') }}
        >
          <span className="text-5xl" style={{ fontSize: fontSize('category', 4 / 3) }}>
            {emoji}
          </span>
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
                // A scaled rank outgrows w-12; min-width overrides width.
                style={{
                  fontSize: fontSize('entry', 1.5),
                  minWidth: fontSize('entry') && '1.6em',
                }}
              >
                #{entry.rank}
              </div>
              <UserAvatar
                avatarUrl={entry.avatar_url}
                size={avatarSize}
                displayName={entry.display_name}
              />
              <div className="flex-1">
                <div
                  className="text-xl font-semibold text-white"
                  style={{ fontSize: fontSize('entry') }}
                >
                  {entry.display_name}
                </div>
                <div
                  className="text-sm text-slate-400 capitalize"
                  style={{ fontSize: fontSize('entry', 0.7) }}
                >
                  {entry.platform}
                </div>
              </div>
              <div
                className="text-2xl font-bold text-white"
                style={{ fontSize: fontSize('entry', 1.2) }}
              >
                {creditRollValue(ct, entry)}
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
          <p className="text-xl text-white">{ct('viewerOverlay.credits.empty')}</p>
        </div>
      </div>
    )
  }

  const theme = config?.theme || 'cinematic'
  const bgOpacity = config?.background_opacity ?? 0.8
  const sessionStart = new Date(creditData.session_started_at)

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
          {/* Behind a clip the opacity only dims it, never hides it: at the
              maximum the clip still shows through at half brightness. The roll
              keeps its contrast from the text shadow on the content below. */}
          <div
            className="absolute inset-0 z-10"
            style={{
              background: `rgba(0, 0, 0, ${bgOpacity * CLIP_SCRIM_MAX})`,
              pointerEvents: 'none',
            }}
          />
        </div>
      )}

      <div
        className={clsx(
          'relative z-10 min-h-screen',
          preview ? 'overflow-y-auto' : 'h-screen overflow-hidden'
        )}
        style={{
          background: !currentClip
            ? theme === 'cinematic'
              ? `linear-gradient(to bottom, rgba(17, 24, 39, ${bgOpacity}), rgba(0, 0, 0, ${bgOpacity}))`
              : theme === 'modern'
                ? `linear-gradient(135deg, rgba(99, 102, 241, ${bgOpacity}), rgba(139, 92, 246, ${bgOpacity}))`
                : `rgba(17, 24, 39, ${bgOpacity})`
            : 'transparent',
          textShadow: currentClip ? '0 2px 6px rgba(0, 0, 0, 0.9)' : undefined,
          ...(preview && { zoom: previewZoom, height: `calc(100vh / ${previewZoom})` }),
        }}
      >
        <div ref={rollRef} className="container mx-auto px-8 py-12">
          {/* Header */}
          <div className="mb-16 text-center">
            <h1
              className="animate-fade-in mb-4 text-6xl font-bold text-white"
              style={{ fontSize: fontSize('heading') }}
            >
              {ct('viewerOverlay.credits.heading')}
            </h1>
            <p className="text-2xl text-slate-300" style={{ fontSize: fontSize('subheading') }}>
              {ct('viewerOverlay.credits.subheading')}
            </p>
            <div className="mt-4 text-slate-400" style={{ fontSize: fontSize('session') }}>
              <p>{ct('viewerOverlay.credits.session', sessionDateParams(sessionStart))}</p>
              <p>
                {ct('viewerOverlay.credits.duration', {
                  duration: sessionDuration(ct, creditData.session_duration_seconds),
                  hours: Math.floor(creditData.session_duration_seconds / 3600),
                  minutes: Math.floor((creditData.session_duration_seconds % 3600) / 60),
                })}
              </p>
            </div>
          </div>

          {/* Leaderboards */}
          <div className="mx-auto max-w-4xl">
            {renderLeaderboard(
              ct('viewerOverlay.credits.topSubscribers'),
              creditData.leaderboards.subs,
              '⭐'
            )}
            {renderLeaderboard(
              ct('viewerOverlay.credits.topGifters'),
              creditData.leaderboards.gifts,
              '🎁'
            )}
            {renderLeaderboard(
              ct('viewerOverlay.credits.topCheerers'),
              creditData.leaderboards.bits,
              '💎'
            )}
            {renderLeaderboard(
              ct('viewerOverlay.credits.topChannelPoints'),
              creditData.leaderboards.points,
              '🎯'
            )}
            {renderLeaderboard(
              ct('viewerOverlay.credits.topRaiders'),
              creditData.leaderboards.raids,
              '⚔️'
            )}
            {renderLeaderboard(
              ct('viewerOverlay.credits.topSuperChats'),
              creditData.leaderboards.super_chats,
              '💰'
            )}
            {renderLeaderboard(
              ct('viewerOverlay.credits.newFollowers'),
              creditData.leaderboards.follows,
              '❤️'
            )}
            {renderLeaderboard(
              ct('viewerOverlay.credits.topMembers'),
              creditData.leaderboards.memberships,
              '🌟'
            )}
            {renderLeaderboard(
              ct('viewerOverlay.credits.topCoinChests'),
              creditData.leaderboards.treasure_chests,
              '🪙'
            )}
            {renderLeaderboard(
              ct('viewerOverlay.credits.topSharers'),
              creditData.leaderboards.shares,
              '🔗'
            )}
            {renderLeaderboard(
              ct('viewerOverlay.credits.topLikers'),
              creditData.leaderboards.likes,
              '👍'
            )}
          </div>

          {/* Footer */}
          <div className="mt-24 mb-12 text-center">
            <div
              className="mb-4 text-4xl font-bold text-white"
              style={{ fontSize: fontSize('footer') }}
            >
              {ct('viewerOverlay.credits.thanks')}
            </div>
            <p
              className="text-xl text-slate-300"
              style={{ fontSize: fontSize('footer', 1.25 / 2.25) }}
            >
              {ct('viewerOverlay.credits.seeYou')}
            </p>
          </div>
        </div>
      </div>

      {/* Now Playing Indicator, outside the roll so its transform cannot move it */}
      {currentClip && (
        <div className="fixed right-8 bottom-8 z-50">
          <div
            className="max-w-sm rounded-lg border border-slate-700 bg-black/80 p-4 shadow-2xl backdrop-blur-sm"
            style={{ maxWidth: fontSize('nowPlaying', 24) }}
          >
            <div className="mb-2 flex items-center gap-3">
              <span className="text-2xl" style={{ fontSize: fontSize('nowPlaying', 1.5) }}>
                {CAMERA_GLYPH}
              </span>
              <span
                className="font-semibold text-white"
                style={{ fontSize: fontSize('nowPlaying') }}
              >
                {ct('viewerOverlay.credits.nowPlaying')}
              </span>
            </div>
            <div
              className="mb-1 text-lg font-medium text-white"
              style={{ fontSize: fontSize('nowPlaying', 1.125) }}
            >
              {currentClip.title}
            </div>
            <div
              className="flex items-center justify-between text-sm text-slate-400"
              style={{ fontSize: fontSize('nowPlaying', 0.875) }}
            >
              <span>
                {ct('viewerOverlay.credits.clipViews', {
                  views: formatNumber(currentClip.view_count),
                })}
              </span>
              <span>
                {ct('viewerOverlay.credits.clipCounter', {
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
