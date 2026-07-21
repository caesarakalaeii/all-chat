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
 * Landing Page — client portion
 *
 * Rendered by the server wrapper in `page.tsx`, which owns the page metadata and
 * the JSON-LD structured data. Stays a Client Component because it reads auth
 * state and browser APIs.
 *
 * Structure (declutter pass, PR #542):
 *   - Sticky HomeHeader keeps the primary action (Dashboard / Sign in) reachable
 *     at scroll 0 for both audiences.
 *   - Hero branches on auth: logged-out gets the pitch with the login CTA placed
 *     ABOVE the theme showcase; logged-in returning users get a compact welcome
 *     with the Dashboard CTA first, then the showcase as a "what's new" tease.
 *   - The full marketing funnel (features + steps, promos, FAQ) renders for
 *     logged-out visitors only; logged-in users get a collapsed "Explore" row so
 *     discovery stays one click away without re-showing the whole pitch.
 */

'use client'

import Link from 'next/link'
import { useEffect, useState } from 'react'
import { useAuthStore } from '@/lib/stores/auth-store'
import { FaqSection } from '@/components/FaqSection'
import { FeaturedAmbassadors } from '@/components/FeaturedAmbassadors'
import { HomeHeader } from '@/components/HomeHeader'
import { ThemeSwitcher } from '@/components/ThemeSwitcher'
import { PlatformBadge } from '@/components/ui/badge'
import { SpotlightCard } from '@/components/SpotlightCard'
import { PLATFORM_COLORS } from '@/lib/platform-colors'
import { toastManager } from '@/lib/toast'
import { DISCORD_INVITE_URL } from '@/lib/constants'
import { LayoutGrid, Zap, Palette, Puzzle, Code2, ChevronDown } from 'lucide-react'
import { cn } from '@/lib/utils'
import { trackEvent } from '@/lib/analytics'
import { safeExternalRedirect } from '@/lib/auth/redirect-allowlist'
import { dashStyleFor } from '@/lib/dashboard-button-styles'

// ---------------------------------------------------------------------------
// Platform stat data
// ---------------------------------------------------------------------------
const PLATFORMS = ['twitch', 'youtube', 'kick', 'tiktok'] as const
type Platform = (typeof PLATFORMS)[number]

function formatCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`
  return n.toLocaleString()
}

// ---------------------------------------------------------------------------
// Feature card data (logged-out "Why All-Chat" band)
// ---------------------------------------------------------------------------
const FEATURE_CARDS = [
  {
    icon: Palette,
    title: '16 themes, full control',
    description:
      'From Win98 retro to cyberpunk neon — pick a built-in theme, tweak it point-and-click, or write your own CSS.',
  },
  {
    icon: Zap,
    title: 'Every emote, everywhere',
    description:
      '7TV, BTTV, FFZ plus native Twitch and YouTube emotes — they all render correctly in your overlay.',
  },
  {
    icon: LayoutGrid,
    title: 'Smart resource usage',
    description:
      'Only polls a platform while your overlay is live in OBS. Switch scenes and All-Chat stands down.',
  },
]

const STEPS = ['Sign in', 'Add your channels', 'Paste the URL in OBS'] as const

// Adaptive column counts for the stat strip — static strings so Tailwind's JIT
// sees them (never a template literal). Keyed by how many platforms have data.
const STAT_GRID_COLS: Record<number, string> = {
  1: 'grid-cols-1',
  2: 'grid-cols-2',
  3: 'grid-cols-2 sm:grid-cols-3',
  4: 'grid-cols-2 sm:grid-cols-4',
}

// ---------------------------------------------------------------------------
// LandingPage
// ---------------------------------------------------------------------------
export default function HomeClient() {
  const { user, init } = useAuthStore()
  const [msgCounts, setMsgCounts] = useState<Record<Platform, number> | null>(null)

  useEffect(() => {
    init()
  }, [init])

  useEffect(() => {
    fetch('/api/v1/stats')
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (data) setMsgCounts(data)
      })
      .catch(() => {}) // fail silently — stats are decorative
  }, [])

  const isLoggedIn = !!user
  const dashStyle = dashStyleFor(user?.auth_provider)

  // Zero-guard: only surface platforms that actually delivered messages this week,
  // so a quiet week or empty stat buckets never renders an embarrassing "0".
  const activePlatforms = msgCounts ? PLATFORMS.filter((p) => (msgCounts[p] ?? 0) > 0) : []

  const handleTwitchLogin = async () => {
    trackEvent('signin_started', { platform: 'twitch' })
    try {
      const response = await fetch('/api/v1/auth/twitch/login')
      const data = await response.json()
      if (data.auth_url) {
        safeExternalRedirect(data.auth_url)
      } else {
        toastManager.add({ title: 'Login failed', description: 'No auth URL returned. Try again.' })
      }
    } catch {
      toastManager.add({ title: 'Login error', description: 'Failed to initiate Twitch login.' })
    }
  }

  const handleYouTubeLogin = async () => {
    trackEvent('signin_started', { platform: 'youtube' })
    try {
      const response = await fetch('/api/v1/auth/youtube/login')
      const data = await response.json()
      if (data.auth_url) {
        safeExternalRedirect(data.auth_url)
      } else {
        toastManager.add({ title: 'Login failed', description: 'No auth URL returned. Try again.' })
      }
    } catch {
      toastManager.add({ title: 'Login error', description: 'Failed to initiate YouTube login.' })
    }
  }

  const handleKickLogin = async () => {
    trackEvent('signin_started', { platform: 'kick' })
    try {
      const response = await fetch('/api/v1/auth/kick/login')
      const data = await response.json()
      if (data.auth_url) {
        safeExternalRedirect(data.auth_url)
      } else {
        toastManager.add({ title: 'Login failed', description: 'No auth URL returned. Try again.' })
      }
    } catch {
      toastManager.add({ title: 'Login error', description: 'Failed to initiate Kick login.' })
    }
  }

  return (
    <main id="main-content" tabIndex={-1} className="min-h-screen scroll-smooth">
      <HomeHeader />

      {/* ------------------------------------------------------------------ */}
      {/* Hero — branches on auth state                                       */}
      {/* ------------------------------------------------------------------ */}
      <section className="relative flex flex-col items-center px-4 pt-16 pb-16 text-center">
        {/* Noise overlay — SVG feTurbulence at opacity 0.03 */}
        <div aria-hidden="true" className="pointer-events-none absolute inset-0 opacity-[0.03]">
          <svg width="100%" height="100%" xmlns="http://www.w3.org/2000/svg">
            <filter id="noise">
              <feTurbulence
                type="fractalNoise"
                baseFrequency="0.65"
                numOctaves="3"
                stitchTiles="stitch"
              />
              <feColorMatrix type="saturate" values="0" />
            </filter>
            <rect width="100%" height="100%" filter="url(#noise)" />
          </svg>
        </div>

        {isLoggedIn ? (
          <>
            <p className="relative mb-4 text-xs font-bold tracking-widest text-text-sub uppercase">
              Welcome back
            </p>
            <h1 className="relative mb-6 max-w-2xl text-4xl font-extrabold tracking-tight text-text sm:text-5xl">
              Welcome back, {user.display_name}.
            </h1>
            <Link
              href="/dashboard"
              onClick={() => trackEvent('cta_click', { cta: 'dashboard', location: 'hero' })}
              className={cn(
                'relative inline-flex items-center gap-2.5 rounded-lg px-8 py-3 font-semibold transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none',
                dashStyle.bg,
                dashStyle.ring,
                dashStyle.text
              )}
            >
              <LayoutGrid className="h-5 w-5" aria-hidden="true" />
              {user.is_admin ? 'Welcome aboard, captain!' : 'Go to Dashboard'}
            </Link>
          </>
        ) : (
          <>
            <p className="relative mb-4 text-xs font-bold tracking-widest text-text-sub uppercase">
              Free · open source · every platform
            </p>
            <h1 className="relative mb-4 max-w-2xl text-4xl font-extrabold tracking-tight text-text sm:text-5xl">
              One overlay. Every platform.
            </h1>
            <p className="relative mb-8 max-w-xl text-lg text-text-sub">
              Every message from Twitch, YouTube, Kick, TikTok and Discord in one OBS overlay.
            </p>

            {/* Primary CTA — moved ABOVE the theme showcase so it stays in view */}
            <div
              id="get-started"
              className="relative flex scroll-mt-24 flex-col justify-center gap-3 sm:flex-row"
            >
              {/* Twitch */}
              <button
                onClick={handleTwitchLogin}
                className="flex items-center gap-2.5 rounded-lg bg-twitch px-6 py-3 font-semibold text-bg transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none"
                aria-label="Sign in with Twitch"
              >
                <svg
                  className="h-5 w-5 shrink-0"
                  viewBox="0 0 24 24"
                  xmlns="http://www.w3.org/2000/svg"
                  aria-hidden="true"
                >
                  <path
                    fill="currentColor"
                    d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714z"
                  />
                </svg>
                Sign in with Twitch
              </button>

              {/* YouTube — exact brand red #FF0000; dark label for WCAG AA (white on
                  #FF0000 is ~4.0:1), official white-on-red icon kept (logo exemption) */}
              <button
                onClick={handleYouTubeLogin}
                className="flex items-center gap-2.5 rounded-lg px-6 py-3 font-semibold text-bg transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none"
                style={
                  {
                    backgroundColor: '#FF0000',
                    '--tw-ring-color': '#FF0000',
                  } as React.CSSProperties
                }
                aria-label="Sign in with YouTube"
              >
                {/* Official YouTube icon — white play button on brand red */}
                <svg
                  className="h-5 w-5 shrink-0"
                  viewBox="0 0 24 24"
                  xmlns="http://www.w3.org/2000/svg"
                  aria-hidden="true"
                >
                  <path
                    fill="#FFFFFF"
                    d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"
                  />
                </svg>
                Sign in with YouTube
              </button>

              {/* Kick — brand green, dark text + official block-K logo */}
              <button
                onClick={handleKickLogin}
                className="flex items-center gap-2.5 rounded-lg px-6 py-3 font-semibold text-bg transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-kick focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none"
                style={{ backgroundColor: 'var(--color-kick)' }}
                aria-label="Sign in with Kick"
              >
                <svg
                  className="h-5 w-5 shrink-0"
                  viewBox="0 0 512 512"
                  xmlns="http://www.w3.org/2000/svg"
                  aria-hidden="true"
                >
                  <path
                    fill="currentColor"
                    d="M37 .036h164.448v113.621h54.71v-56.82h54.731V.036h164.448v170.777h-54.73v56.82h-54.711v56.8h54.71v56.82h54.73V512.03H310.89v-56.82h-54.73v-56.8h-54.711v113.62H37V.036z"
                  />
                </svg>
                Sign in with Kick
              </button>
            </div>

            <p className="relative mt-4 text-xs text-text-sub">
              Free &amp; open source · No bots · Just a URL in OBS
            </p>
          </>
        )}

        {/* Theme switcher — the showcase, now below the primary action in both states */}
        <ThemeSwitcher className="relative mt-12" />

        {/* Platform stat strip — hairline social proof, logged-out only. Hidden
            entirely when no platform has data (see activePlatforms zero-guard). */}
        {!isLoggedIn && msgCounts && activePlatforms.length > 0 && (
          <div className="relative mx-auto mt-10 w-full max-w-2xl">
            <div
              className={cn(
                'grid gap-px overflow-hidden rounded-xl bg-border',
                STAT_GRID_COLS[activePlatforms.length] ?? 'grid-cols-2 sm:grid-cols-4'
              )}
            >
              {activePlatforms.map((platform) => (
                <div
                  key={platform}
                  className="flex flex-col items-center gap-1.5 bg-surface px-4 py-4"
                >
                  <PlatformBadge platform={platform} size="sm" />
                  <span
                    className={cn('text-xl font-bold', PLATFORM_COLORS[platform].text)}
                    style={{ textShadow: `var(--shadow-glow-${platform})` }}
                  >
                    {formatCount(msgCounts[platform])}
                  </span>
                </div>
              ))}
            </div>
            <p className="mt-3 text-center text-xs text-text-dim">messages delivered this week</p>
          </div>
        )}
      </section>

      {isLoggedIn ? (
        /* ---------------------------------------------------------------- */
        /* Returning user — discovery stays one click away, no re-pitch      */
        /* ---------------------------------------------------------------- */
        <details className="group mx-auto w-full max-w-2xl px-4 pb-20">
          <summary className="flex cursor-pointer list-none items-center justify-center gap-2 rounded-lg border border-border bg-surface px-4 py-3 text-sm font-medium text-text-sub transition-colors hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none [&::-webkit-details-marker]:hidden">
            Explore All-Chat
            <ChevronDown
              className="h-4 w-4 transition-transform group-open:rotate-180"
              aria-hidden="true"
            />
          </summary>
          <div className="mt-4 flex flex-wrap justify-center gap-x-6 gap-y-2 text-sm text-text-sub">
            <a
              href="https://addons.mozilla.org/en-US/firefox/addon/all-chat-extension/"
              target="_blank"
              rel="noopener noreferrer"
              onClick={() => trackEvent('outbound_click', { dest: 'ext_firefox' })}
              className="underline-offset-4 hover:text-text hover:underline"
            >
              Browser extension
            </a>
            <Link
              href="/docs/api"
              onClick={() => trackEvent('cta_click', { cta: 'api-docs', location: 'explore' })}
              className="underline-offset-4 hover:text-text hover:underline"
            >
              Developer API
            </Link>
            <Link href="/docs" className="underline-offset-4 hover:text-text hover:underline">
              Docs &amp; FAQ
            </Link>
          </div>
        </details>
      ) : (
        <>
          {/* -------------------------------------------------------------- */}
          {/* Why All-Chat — features + the 3-step flow in one band           */}
          {/* -------------------------------------------------------------- */}
          <section className="mx-auto max-w-5xl border-t border-border px-4 py-16">
            <p className="mb-3 text-center text-xs font-bold tracking-widest text-text-sub uppercase">
              Why All-Chat
            </p>
            <h2 className="mb-10 text-center text-2xl font-bold text-text">
              Built for multistreamers
            </h2>
            <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
              {FEATURE_CARDS.map((feature) => {
                const Icon = feature.icon
                return (
                  <SpotlightCard
                    key={feature.title}
                    className="rounded-xl border border-border bg-surface p-5 transition-colors hover:border-border-md"
                  >
                    <Icon className="mb-4 h-7 w-7 text-text-sub" aria-hidden="true" />
                    <h3 className="mb-2 text-lg font-semibold text-text">{feature.title}</h3>
                    <p className="text-sm text-text-sub">{feature.description}</p>
                  </SpotlightCard>
                )
              })}
            </div>

            {/* Slim 3-step strip — replaces the old full "Live in 3 steps" section */}
            <div className="mt-12 flex flex-wrap items-center justify-center gap-x-3 gap-y-3 text-sm">
              <span className="font-semibold text-text">Live in 3 steps:</span>
              {STEPS.map((label, i) => (
                <span key={label} className="flex items-center gap-3">
                  <span className="flex items-center gap-2 text-text-sub">
                    <span className="flex h-6 w-6 items-center justify-center rounded-full border border-border text-xs font-bold text-text">
                      {i + 1}
                    </span>
                    {label}
                  </span>
                  {i < STEPS.length - 1 && (
                    <span className="text-text-dim" aria-hidden="true">
                      →
                    </span>
                  )}
                </span>
              ))}
            </div>
          </section>

          {/* -------------------------------------------------------------- */}
          {/* Featured Ambassadors — social proof (ADR-0041). Self-hides when */}
          {/* there are no opted-in ambassadors.                              */}
          {/* -------------------------------------------------------------- */}
          <FeaturedAmbassadors />

          {/* -------------------------------------------------------------- */}
          {/* Beyond the overlay — extension + API in one compact 2-up band   */}
          {/* -------------------------------------------------------------- */}
          <section className="mx-auto max-w-5xl border-t border-border px-4 py-16">
            <p className="mb-3 text-center text-xs font-bold tracking-widest text-text-sub uppercase">
              Beyond the overlay
            </p>
            <h2 className="mb-10 text-center text-2xl font-bold text-text">
              Do more with All-Chat
            </h2>
            <div className="grid gap-4 md:grid-cols-2">
              {/* Browser extension */}
              <div className="rounded-xl border border-border bg-surface p-6">
                <h3 className="mb-1.5 flex items-center gap-2 text-lg font-bold text-text">
                  <Puzzle className="h-5 w-5 text-twitch" aria-hidden="true" />
                  Browser extension
                </h3>
                <p className="mb-4 text-sm text-text-sub">
                  Give your viewers unified cross-platform chat right in their browser — it replaces
                  native Twitch, YouTube, and Kick chat.
                </p>
                <div className="flex flex-wrap items-center gap-2">
                  <a
                    href="https://addons.mozilla.org/en-US/firefox/addon/all-chat-extension/"
                    target="_blank"
                    rel="noopener noreferrer"
                    onClick={() => trackEvent('outbound_click', { dest: 'ext_firefox' })}
                    className="inline-flex items-center gap-1.5 rounded-full border border-border bg-surface-2 px-3 py-1 text-xs font-medium text-text-sub underline-offset-4 hover:text-text hover:underline"
                  >
                    <svg
                      className="h-3.5 w-3.5 shrink-0"
                      viewBox="0 0 24 24"
                      xmlns="http://www.w3.org/2000/svg"
                      aria-hidden="true"
                    >
                      <path
                        fill="currentColor"
                        d="M8.824 7.287c.008 0 .004 0 0 0zm-2.8-1.4c.006 0 .003 0 0 0zm16.754 2.161c-.505-1.215-1.53-2.528-2.333-2.943.654 1.283 1.033 2.57 1.177 3.53l.002.02c-1.314-3.278-3.544-4.6-5.366-7.477-.091-.147-.184-.292-.273-.446a3.545 3.545 0 01-.13-.24 2.118 2.118 0 01-.172-.46.03.03 0 00-.027-.03.038.038 0 00-.021 0l-.006.001a.037.037 0 00-.01.005L15.624 0c-2.585 1.515-3.657 4.168-3.932 5.856a6.197 6.197 0 00-2.305.587.297.297 0 00-.147.37c.057.162.24.24.396.17a5.622 5.622 0 012.008-.523l.067-.005a5.847 5.847 0 011.957.222l.095.03a5.816 5.816 0 01.616.228c.08.036.16.073.238.112l.107.055a5.835 5.835 0 01.368.211 5.953 5.953 0 012.034 2.104c-.62-.437-1.733-.868-2.803-.681 4.183 2.09 3.06 9.292-2.737 9.02a5.164 5.164 0 01-1.513-.292 4.42 4.42 0 01-.538-.232c-1.42-.735-2.593-2.121-2.74-3.806 0 0 .537-2 3.845-2 .357 0 1.38-.998 1.398-1.287-.005-.095-2.029-.9-2.817-1.677-.422-.416-.622-.616-.8-.767a3.47 3.47 0 00-.301-.227 5.388 5.388 0 01-.032-2.842c-1.195.544-2.124 1.403-2.8 2.163h-.006c-.46-.584-.428-2.51-.402-2.913-.006-.025-.343.176-.389.206-.406.29-.787.616-1.136.974-.397.403-.76.839-1.085 1.303a9.816 9.816 0 00-1.562 3.52c-.003.013-.11.487-.19 1.073-.013.09-.026.181-.037.272a7.8 7.8 0 00-.069.667l-.002.034-.023.387-.001.06C.386 18.795 5.593 24 12.016 24c5.752 0 10.527-4.176 11.463-9.661.02-.149.035-.298.052-.448.232-1.994-.025-4.09-.753-5.844z"
                      />
                    </svg>
                    Firefox
                  </a>
                  <a
                    href="https://chromewebstore.google.com/detail/all-chat-extension/ioneembbnocfljgbhgfknbbnpfeadacm"
                    target="_blank"
                    rel="noopener noreferrer"
                    onClick={() => trackEvent('outbound_click', { dest: 'ext_chrome' })}
                    className="inline-flex items-center gap-1.5 rounded-full border border-border bg-surface-2 px-3 py-1 text-xs font-medium text-text-sub underline-offset-4 hover:text-text hover:underline"
                  >
                    <svg
                      className="h-3.5 w-3.5 shrink-0"
                      viewBox="0 0 24 24"
                      xmlns="http://www.w3.org/2000/svg"
                      aria-hidden="true"
                    >
                      <path
                        fill="currentColor"
                        d="M12 0C8.21 0 4.831 1.757 2.632 4.501l3.953 6.848A5.454 5.454 0 0 1 12 6.545h10.691A12 12 0 0 0 12 0zM1.931 5.47A11.943 11.943 0 0 0 0 12c0 6.012 4.42 10.991 10.189 11.864l3.953-6.847a5.45 5.45 0 0 1-6.865-2.29zm13.342 2.166a5.446 5.446 0 0 1 1.45 7.09l.002.001h-.002l-5.344 9.257c.206.01.413.016.621.016 6.627 0 12-5.373 12-12 0-1.54-.29-3.011-.818-4.364zM12 16.364a4.364 4.364 0 1 1 0-8.728 4.364 4.364 0 0 1 0 8.728Z"
                      />
                    </svg>
                    Chrome
                  </a>
                  <a
                    href="https://github.com/caesarakalaeii/all-chat-extension/releases"
                    target="_blank"
                    rel="noopener noreferrer"
                    onClick={() => trackEvent('outbound_click', { dest: 'ext_releases' })}
                    className="inline-flex items-center gap-1.5 rounded-full border border-border bg-surface-2 px-3 py-1 text-xs font-medium text-text-sub underline-offset-4 hover:text-text hover:underline"
                  >
                    <svg
                      className="h-3.5 w-3.5 shrink-0"
                      viewBox="0 0 24 24"
                      xmlns="http://www.w3.org/2000/svg"
                      aria-hidden="true"
                    >
                      <path
                        fill="currentColor"
                        d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"
                      />
                    </svg>
                    GitHub Releases
                  </a>
                </div>
              </div>

              {/* Developer API */}
              <div className="rounded-xl border border-border bg-surface p-6">
                <h3 className="mb-1.5 flex items-center gap-2 text-lg font-bold text-text">
                  <Code2 className="h-5 w-5 text-twitch" aria-hidden="true" />
                  Build on the API
                </h3>
                <p className="mb-4 text-sm text-text-sub">
                  One unified chat WebSocket — every platform, one message format. There&apos;s a
                  public test stream you can hook up in seconds, no account needed.
                </p>
                <Link
                  href="/docs/api"
                  onClick={() => trackEvent('cta_click', { cta: 'api-docs', location: 'promo' })}
                  className="inline-flex items-center gap-2 rounded-lg bg-linear-to-r from-twitch to-tiktok px-5 py-2.5 text-sm font-semibold text-bg transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none"
                >
                  <Code2 className="h-4 w-4" aria-hidden="true" />
                  Read the API docs
                </Link>
              </div>
            </div>
          </section>

          {/* FAQ */}
          <FaqSection />
        </>
      )}

      {/* ------------------------------------------------------------------ */}
      {/* Footer                                                              */}
      {/* ------------------------------------------------------------------ */}
      <footer className="space-y-2 border-t border-border pt-12 pb-12 text-center text-sm text-text-sub">
        <p>Free. Open source. Built for streamers who refuse to pick just one platform.</p>
        <p className="flex flex-wrap items-center justify-center gap-3 text-xs">
          <a
            href="https://github.com/caesarakalaeii/all-chat"
            target="_blank"
            rel="noopener noreferrer"
            onClick={() => trackEvent('outbound_click', { dest: 'github' })}
            className="flex items-center gap-1 underline-offset-4 hover:text-text hover:underline"
          >
            <svg
              className="h-3.5 w-3.5"
              viewBox="0 0 24 24"
              xmlns="http://www.w3.org/2000/svg"
              aria-hidden="true"
            >
              <path
                fill="currentColor"
                d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"
              />
            </svg>
            GitHub
          </a>
          <span aria-hidden="true">&bull;</span>
          <a
            href={DISCORD_INVITE_URL}
            target="_blank"
            rel="noopener noreferrer"
            onClick={() => trackEvent('outbound_click', { dest: 'discord' })}
            className="underline-offset-4 hover:text-text hover:underline"
          >
            Discord
          </a>
          <span aria-hidden="true">&bull;</span>
          <Link href="/docs" className="underline-offset-4 hover:text-text hover:underline">
            Docs
          </Link>
          <span aria-hidden="true">&bull;</span>
          <Link href="/docs/api" className="underline-offset-4 hover:text-text hover:underline">
            API
          </Link>
          <span aria-hidden="true">&bull;</span>
          <Link
            href="/legal/privacy"
            className="underline-offset-4 hover:text-text hover:underline"
          >
            Privacy Policy
          </Link>
          <span aria-hidden="true">&bull;</span>
          <Link href="/legal/terms" className="underline-offset-4 hover:text-text hover:underline">
            Terms of Service
          </Link>
          <span aria-hidden="true">&bull;</span>
          <Link
            href="/legal/impressum"
            className="underline-offset-4 hover:text-text hover:underline"
          >
            Impressum
          </Link>
        </p>
      </footer>
    </main>
  )
}
