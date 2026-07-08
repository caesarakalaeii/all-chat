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
 * Magnetic glow hero with 4 platform stat cards and platform OAuth login buttons.
 * Rendered by the server wrapper in `page.tsx`, which owns the page metadata and the
 * JSON-LD structured data. Stays a Client Component because it uses browser APIs/state.
 */

'use client'

import Link from 'next/link'
import { useEffect, useRef, useCallback, useState } from 'react'
import { useAuthStore } from '@/lib/stores/auth-store'
import { InfinityLogo } from '@/components/InfinityLogo'
import { FaqSection } from '@/components/FaqSection'
import { ThemeSwitcher } from '@/components/ThemeSwitcher'
import { PlatformBadge } from '@/components/ui/badge'
import { PLATFORM_COLORS } from '@/lib/platform-colors'
import { toastManager } from '@/lib/toast'
import { LayoutGrid, Zap, Palette, Puzzle, LogIn, Plus, MonitorPlay, Code2 } from 'lucide-react'
import { cn } from '@/lib/utils'
import { trackEvent } from '@/lib/analytics'
import { safeExternalRedirect } from '@/lib/auth/redirect-allowlist'

// ---------------------------------------------------------------------------
// InfinityLogo — animated 4-colour infinity snake inside a chat bubble
// Variant B (solid bubble, 2.5px stroke). Animation via rAF dashoffset trick.

// ---------------------------------------------------------------------------
// useMagneticGlow — pointer-driven glow positions via direct DOM mutation
// (no useState to avoid re-render storms)
// ---------------------------------------------------------------------------
function useMagneticGlow(count: number) {
  const cardRefs = useRef<(HTMLDivElement | null)[]>([])
  const glowRefs = useRef<(HTMLDivElement | null)[]>([])

  const handlePointerMove = useCallback((e: PointerEvent) => {
    const MAX_DIST = 520
    cardRefs.current.forEach((card, i) => {
      if (!card || !glowRefs.current[i]) return
      const glow = glowRefs.current[i]!
      const rect = card.getBoundingClientRect()
      const dx = Math.max(rect.left - e.clientX, 0, e.clientX - rect.right)
      const dy = Math.max(rect.top - e.clientY, 0, e.clientY - rect.bottom)
      const dist = Math.hypot(dx, dy)
      const intensity = Math.pow(Math.max(0, 1 - dist / MAX_DIST), 2)
      if (intensity < 0.003) {
        glow.style.opacity = '0'
        return
      }
      glow.style.left = `${e.clientX - rect.left}px`
      glow.style.top = `${e.clientY - rect.top}px`
      glow.style.opacity = String(Math.min(intensity * 1.35, 1))
    })
  }, [])

  useEffect(() => {
    cardRefs.current = cardRefs.current.slice(0, count)
    glowRefs.current = glowRefs.current.slice(0, count)
    window.addEventListener('pointermove', handlePointerMove)
    return () => window.removeEventListener('pointermove', handlePointerMove)
  }, [count, handlePointerMove])

  return { cardRefs, glowRefs }
}

// ---------------------------------------------------------------------------
// MagGlowCard — card with a pointer-tracking glow blob, per-platform coloured
// ---------------------------------------------------------------------------
const PLATFORM_GLOW_COLORS: Record<string, string> = {
  twitch: 'var(--color-stat-glow-twitch)',
  youtube: 'var(--color-stat-glow-youtube)',
  kick: 'var(--color-stat-glow-kick)',
  tiktok: 'var(--color-stat-glow-tiktok)',
}

function MagGlowCard({
  children,
  cardRef,
  glowRef,
  glowColor,
  className,
}: {
  children: React.ReactNode
  cardRef: (el: HTMLDivElement | null) => void
  glowRef: (el: HTMLDivElement | null) => void
  glowColor?: string
  className?: string
}) {
  return (
    <div
      ref={cardRef}
      className={cn(
        'relative overflow-hidden rounded-xl border border-border bg-surface p-6',
        className
      )}
    >
      <div
        ref={glowRef}
        className="pointer-events-none absolute size-[300px] -translate-x-1/2 -translate-y-1/2 rounded-full opacity-0 transition-opacity duration-300"
        style={{
          background: `radial-gradient(circle, ${glowColor ?? 'rgba(163,123,255,0.15)'} 0%, transparent 70%)`,
        }}
      />
      {children}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Platform stat data
// ---------------------------------------------------------------------------
const PLATFORMS = ['twitch', 'youtube', 'kick', 'tiktok'] as const
type Platform = (typeof PLATFORMS)[number]

const PLATFORM_LABELS: Record<Platform, string> = {
  twitch: 'Twitch',
  youtube: 'YouTube',
  kick: 'Kick',
  tiktok: 'TikTok',
}

function formatCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`
  return n.toLocaleString()
}

const TOTAL_CARDS = PLATFORMS.length + 3 // 4 stat cards + 3 feature cards

// ---------------------------------------------------------------------------
// Feature card data
// ---------------------------------------------------------------------------
const FEATURE_CARDS = [
  {
    icon: Palette,
    title: '16 Themes, Full CSS Control',
    description:
      'From Win98 retro to cyberpunk neon. Pick a built-in theme or write your own CSS — your overlay, your brand.',
  },
  {
    icon: Zap,
    title: 'Every Emote, Everywhere',
    description:
      '7TV, BTTV, FFZ, plus native Twitch and YouTube emotes — they all render correctly in your overlay.',
  },
  {
    icon: LayoutGrid,
    title: 'Smart Resource Usage',
    description:
      'Only polls platforms when your overlay is visible in OBS. Switch scenes and All-Chat stands down automatically.',
  },
]

// ---------------------------------------------------------------------------
// LandingPage
// ---------------------------------------------------------------------------
export default function HomeClient() {
  const { user, init } = useAuthStore()
  const [msgCounts, setMsgCounts] = useState<Record<Platform, number> | null>(null)

  const { cardRefs, glowRefs } = useMagneticGlow(TOTAL_CARDS)

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

  // Tailwind JIT needs static class strings — map auth_provider to button styles
  const DASHBOARD_BUTTON_STYLES: Record<string, { bg: string; ring: string; text: string }> = {
    twitch:  { bg: 'bg-twitch',  ring: 'focus-visible:ring-twitch',  text: 'text-white' },
    youtube: { bg: 'bg-youtube', ring: 'focus-visible:ring-youtube', text: 'text-white' },
    kick:    { bg: 'bg-kick',    ring: 'focus-visible:ring-kick',    text: 'text-bg' },
  }
  const dashStyle = DASHBOARD_BUTTON_STYLES[user?.auth_provider ?? ''] ?? DASHBOARD_BUTTON_STYLES.twitch

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
    <main className="min-h-screen">
      {/* ------------------------------------------------------------------ */}
      {/* Hero section                                                        */}
      {/* ------------------------------------------------------------------ */}
      <section className="relative flex flex-col items-center justify-center px-4 pt-24 pb-16 text-center">
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

        {/* Logo + wordmark */}
        <div className="relative mb-6 flex items-center gap-3">
          <InfinityLogo size={36} />
          <span className="text-4xl font-extrabold tracking-tight text-text">all-chat</span>
        </div>

        <h1 className="mb-4 max-w-2xl text-5xl font-extrabold tracking-tight text-text">
          One overlay. Every platform.
        </h1>
        <p className="mb-10 max-w-xl text-lg text-text-sub">
          See every message from Twitch, YouTube, Kick, TikTok, and Discord in one place
          — no bots, no setup, just a URL in OBS.
        </p>

        {/* Theme switcher — hero centerpiece. The built-in themes are a headline
            selling point, so a live, customizable preview is the hero's main visual. */}
        <ThemeSwitcher className="mb-12" />

        {/* CTA — dashboard link for logged-in users, login buttons otherwise */}
        {isLoggedIn ? (
          <Link
            href="/dashboard"
            onClick={() => trackEvent('cta_click', { cta: 'dashboard', location: 'hero' })}
            className={cn(
              'inline-flex items-center gap-2.5 rounded-lg px-8 py-3 font-semibold transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none',
              dashStyle.bg, dashStyle.ring, dashStyle.text
            )}
          >
            <LayoutGrid className="h-5 w-5" aria-hidden="true" />
            {user?.is_admin ? 'Welcome aboard, captain!' : 'Go to Dashboard'}
          </Link>
        ) : (
          <div className="flex flex-col justify-center gap-3 sm:flex-row">
            {/* Twitch */}
            <button
              onClick={handleTwitchLogin}
              className="flex items-center gap-2.5 rounded-lg bg-twitch px-6 py-3 font-semibold text-white transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none"
              aria-label="Sign in with Twitch"
            >
              <svg
                className="h-5 w-5 shrink-0"
                viewBox="0 0 24 24"
                xmlns="http://www.w3.org/2000/svg"
                aria-hidden="true"
              >
                <path
                  fill="#FFFFFF"
                  d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714z"
                />
              </svg>
              Sign in with Twitch
            </button>

            {/* YouTube — exact brand red #FF0000, white text + icon per YouTube guidelines */}
            <button
              onClick={handleYouTubeLogin}
              className="flex items-center gap-2.5 rounded-lg px-6 py-3 font-semibold text-white transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none"
              style={
                { backgroundColor: '#FF0000', '--tw-ring-color': '#FF0000' } as React.CSSProperties
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
        )}

        {/* Platform stat cards — social proof, demoted below the theme preview + CTA */}
        <div className="mx-auto mt-14 grid w-full max-w-3xl grid-cols-2 gap-4 md:grid-cols-4">
          {PLATFORMS.map((platform, i) => {
            const count = msgCounts?.[platform]
            return (
              <MagGlowCard
                key={platform}
                cardRef={(el) => {
                  cardRefs.current[i] = el
                }}
                glowRef={(el) => {
                  glowRefs.current[i] = el
                }}
                glowColor={PLATFORM_GLOW_COLORS[platform]}
              >
                <PlatformBadge platform={platform} className="mb-3" />
                <div className={cn('mb-1 text-2xl font-bold', PLATFORM_COLORS[platform].text)}>
                  {count != null ? formatCount(count) : '—'}
                </div>
                <div className="text-xs text-text-sub">messages delivered this week</div>
              </MagGlowCard>
            )
          })}
        </div>
      </section>

      {/* ------------------------------------------------------------------ */}
      {/* How it works — 3-step walkthrough                                  */}
      {/* ------------------------------------------------------------------ */}
      <section className="mx-auto max-w-3xl px-4 pb-20">
        <h2 className="mb-10 text-center text-2xl font-bold text-text">Live in 3 steps</h2>
        <div className="grid grid-cols-1 gap-8 md:grid-cols-3">
          {[
            {
              icon: LogIn,
              step: '1',
              title: 'Sign in & create an overlay',
              description: 'Log in with Twitch, YouTube, or Kick. Create an overlay and add your chat sources.',
            },
            {
              icon: Plus,
              step: '2',
              title: 'Add your channels',
              description: 'Connect Twitch channels, YouTube streams, Kick channels — mix and match across platforms.',
            },
            {
              icon: MonitorPlay,
              step: '3',
              title: 'Paste the URL in OBS',
              description: 'Add a Browser Source, paste your overlay URL. Every platform\'s chat in one feed.',
            },
          ].map((item) => {
            const Icon = item.icon
            return (
              <div key={item.step} className="flex flex-col items-center text-center">
                <div className="mb-3 flex h-12 w-12 items-center justify-center rounded-full border border-border bg-surface">
                  <Icon className="h-6 w-6 text-text-sub" aria-hidden="true" />
                </div>
                <div className="mb-1 text-xs font-bold uppercase tracking-widest text-text-sub">
                  Step {item.step}
                </div>
                <h3 className="mb-1 text-base font-semibold text-text">{item.title}</h3>
                <p className="text-sm text-text-sub">{item.description}</p>
              </div>
            )
          })}
        </div>
      </section>

      {/* ------------------------------------------------------------------ */}
      {/* Feature grid — same magnetic glow cards                            */}
      {/* ------------------------------------------------------------------ */}
      <section className="mx-auto max-w-5xl px-4 pb-16">
        <h2 className="mb-8 text-center text-2xl font-bold text-text">Why all-chat?</h2>
        <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
          {FEATURE_CARDS.map((feature, i) => {
            const Icon = feature.icon
            const cardIdx = PLATFORMS.length + i
            return (
              <MagGlowCard
                key={feature.title}
                cardRef={(el) => {
                  cardRefs.current[cardIdx] = el
                }}
                glowRef={(el) => {
                  glowRefs.current[cardIdx] = el
                }}
              >
                <div className="mb-4 text-text-sub">
                  <Icon className="h-7 w-7" aria-hidden="true" />
                </div>
                <h3 className="mb-2 text-lg font-semibold text-text">{feature.title}</h3>
                <p className="text-sm text-text-sub">{feature.description}</p>
              </MagGlowCard>
            )
          })}
        </div>
      </section>

      {/* ------------------------------------------------------------------ */}
      {/* Extension promo                                                     */}
      {/* ------------------------------------------------------------------ */}
      <section className="mx-auto max-w-3xl px-4 pb-16">
        <div className="flex flex-col items-center gap-6 rounded-xl border border-border bg-surface p-8 text-center sm:flex-row sm:text-left">
          <div className="flex-shrink-0 rounded-xl bg-surface-2 p-4">
            <Puzzle className="h-10 w-10 text-twitch" aria-hidden="true" />
          </div>
          <div className="flex-1">
            <h2 className="mb-1 text-xl font-bold text-text">Browser Extension</h2>
            <p className="mb-3 text-sm text-text-sub">
              Your viewers get unified chat too. The extension replaces native Twitch, YouTube, and
              Kick chat with All-Chat — so your community can chat across platforms from their
              browser.
            </p>
            <div className="flex flex-wrap items-center gap-2">
              <a
                href="https://addons.mozilla.org/en-US/firefox/addon/all-chat-extension/"
                target="_blank"
                rel="noopener noreferrer"
                onClick={() => trackEvent('outbound_click', { dest: 'ext_firefox' })}
                className="inline-flex items-center gap-1.5 rounded-full border border-border bg-surface-2 px-3 py-1 text-xs font-medium text-text-sub underline-offset-4 hover:text-text hover:underline"
              >
                <svg className="h-3.5 w-3.5 shrink-0" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                  <path fill="currentColor" d="M8.824 7.287c.008 0 .004 0 0 0zm-2.8-1.4c.006 0 .003 0 0 0zm16.754 2.161c-.505-1.215-1.53-2.528-2.333-2.943.654 1.283 1.033 2.57 1.177 3.53l.002.02c-1.314-3.278-3.544-4.6-5.366-7.477-.091-.147-.184-.292-.273-.446a3.545 3.545 0 01-.13-.24 2.118 2.118 0 01-.172-.46.03.03 0 00-.027-.03.038.038 0 00-.021 0l-.006.001a.037.037 0 00-.01.005L15.624 0c-2.585 1.515-3.657 4.168-3.932 5.856a6.197 6.197 0 00-2.305.587.297.297 0 00-.147.37c.057.162.24.24.396.17a5.622 5.622 0 012.008-.523l.067-.005a5.847 5.847 0 011.957.222l.095.03a5.816 5.816 0 01.616.228c.08.036.16.073.238.112l.107.055a5.835 5.835 0 01.368.211 5.953 5.953 0 012.034 2.104c-.62-.437-1.733-.868-2.803-.681 4.183 2.09 3.06 9.292-2.737 9.02a5.164 5.164 0 01-1.513-.292 4.42 4.42 0 01-.538-.232c-1.42-.735-2.593-2.121-2.74-3.806 0 0 .537-2 3.845-2 .357 0 1.38-.998 1.398-1.287-.005-.095-2.029-.9-2.817-1.677-.422-.416-.622-.616-.8-.767a3.47 3.47 0 00-.301-.227 5.388 5.388 0 01-.032-2.842c-1.195.544-2.124 1.403-2.8 2.163h-.006c-.46-.584-.428-2.51-.402-2.913-.006-.025-.343.176-.389.206-.406.29-.787.616-1.136.974-.397.403-.76.839-1.085 1.303a9.816 9.816 0 00-1.562 3.52c-.003.013-.11.487-.19 1.073-.013.09-.026.181-.037.272a7.8 7.8 0 00-.069.667l-.002.034-.023.387-.001.06C.386 18.795 5.593 24 12.016 24c5.752 0 10.527-4.176 11.463-9.661.02-.149.035-.298.052-.448.232-1.994-.025-4.09-.753-5.844z"/>
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
                <svg className="h-3.5 w-3.5 shrink-0" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                  <path fill="currentColor" d="M12 0C8.21 0 4.831 1.757 2.632 4.501l3.953 6.848A5.454 5.454 0 0 1 12 6.545h10.691A12 12 0 0 0 12 0zM1.931 5.47A11.943 11.943 0 0 0 0 12c0 6.012 4.42 10.991 10.189 11.864l3.953-6.847a5.45 5.45 0 0 1-6.865-2.29zm13.342 2.166a5.446 5.446 0 0 1 1.45 7.09l.002.001h-.002l-5.344 9.257c.206.01.413.016.621.016 6.627 0 12-5.373 12-12 0-1.54-.29-3.011-.818-4.364zM12 16.364a4.364 4.364 0 1 1 0-8.728 4.364 4.364 0 0 1 0 8.728Z"/>
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
                <svg className="h-3.5 w-3.5 shrink-0" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                  <path fill="currentColor" d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"/>
                </svg>
                GitHub Releases
              </a>
            </div>
            <p className="mt-2 text-xs text-text-sub">GitHub Releases get updates first — store versions follow shortly after.</p>
          </div>
        </div>
      </section>

      {/* ------------------------------------------------------------------ */}
      {/* Developer docs promo                                                */}
      {/* ------------------------------------------------------------------ */}
      <section className="mx-auto max-w-3xl px-4 pb-16">
        <div className="flex flex-col items-center gap-6 rounded-xl border border-border bg-surface p-8 text-center sm:flex-row sm:text-left">
          <div className="flex-shrink-0 rounded-xl bg-surface-2 p-4">
            <Code2 className="h-10 w-10 text-twitch" aria-hidden="true" />
          </div>
          <div className="flex-1">
            <h2 className="mb-1 text-xl font-bold text-text">Build on All-Chat</h2>
            <p className="mb-4 text-sm text-text-sub">
              Connect bots, overlays, vote counters, and analytics to one unified chat WebSocket —
              every platform, one message format. There&apos;s a public test stream you can hook up
              in seconds, no account needed.
            </p>
            <div className="flex flex-wrap items-center justify-center gap-2 sm:justify-start">
              <Link
                href="/docs/api"
                onClick={() => trackEvent('cta_click', { cta: 'api-docs', location: 'promo' })}
                className="inline-flex items-center gap-2 rounded-lg bg-linear-to-r from-twitch to-tiktok px-5 py-2.5 text-sm font-semibold text-white transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-twitch focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none"
              >
                <Code2 className="h-4 w-4" aria-hidden="true" />
                Read the API docs
              </Link>
            </div>
          </div>
        </div>
      </section>

      {/* ------------------------------------------------------------------ */}
      {/* FAQ                                                                 */}
      {/* ------------------------------------------------------------------ */}
      <FaqSection />

      {/* ------------------------------------------------------------------ */}
      {/* Footer                                                              */}
      {/* ------------------------------------------------------------------ */}
      <footer className="space-y-2 pb-12 text-center text-sm text-text-sub">
        <p>Free. Open source. Built for streamers who refuse to pick just one platform.</p>
        <p className="flex flex-wrap items-center justify-center gap-3 text-xs">
          <a
            href="https://github.com/caesarakalaeii/all-chat"
            target="_blank"
            rel="noopener noreferrer"
            onClick={() => trackEvent('outbound_click', { dest: 'github' })}
            className="flex items-center gap-1 underline-offset-4 hover:text-text hover:underline"
          >
            <svg className="h-3.5 w-3.5" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
              <path fill="currentColor" d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"/>
            </svg>
            GitHub
          </a>
          <span aria-hidden="true">&bull;</span>
          <a
            href="https://discord.gg/xCGBSuz39P"
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
          <Link href="/legal/impressum" className="underline-offset-4 hover:text-text hover:underline">
            Impressum
          </Link>
        </p>
      </footer>
    </main>
  )
}
