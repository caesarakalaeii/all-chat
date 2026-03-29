/**
 * Landing Page
 *
 * The home page of All-Chat.
 * Magnetic glow hero with 4 platform stat cards and platform OAuth login buttons.
 *
 * This is a Client Component because it uses browser APIs and state.
 */

'use client'

import Link from 'next/link'
import { useEffect, useRef, useCallback, useState } from 'react'
import { useAuthStore } from '@/lib/stores/auth-store'
import { InfinityLogo } from '@/components/InfinityLogo'
import { PlatformBadge } from '@/components/ui/badge'
import { PLATFORM_COLORS } from '@/lib/platform-colors'
import { toastManager } from '@/lib/toast'
import { LayoutGrid, Zap, Palette, Puzzle, Github } from 'lucide-react'
import { cn } from '@/lib/utils'

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
    icon: LayoutGrid,
    title: 'Unified Chat',
    description: 'Combine Twitch, YouTube, Kick, and TikTok messages in one beautiful overlay.',
  },
  {
    icon: Zap,
    title: 'Real-time Sync',
    description: 'Low-latency delivery under 500ms for a seamless multi-platform experience.',
  },
  {
    icon: Palette,
    title: 'Marketplace Themes',
    description: 'Full control over appearance with 7TV, BTTV, FFZ emotes and custom themes.',
  },
]

// ---------------------------------------------------------------------------
// LandingPage
// ---------------------------------------------------------------------------
export default function LandingPage() {
  const { user, token } = useAuthStore()
  const [msgCounts, setMsgCounts] = useState<Record<Platform, number> | null>(null)

  const { cardRefs, glowRefs } = useMagneticGlow(TOTAL_CARDS)

  useEffect(() => {
    fetch('/api/v1/stats')
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (data) setMsgCounts(data)
      })
      .catch(() => {}) // fail silently — stats are decorative
  }, [])

  const isLoggedIn = !!(user && token)

  // Tailwind JIT needs static class strings — map auth_provider to button styles
  const DASHBOARD_BUTTON_STYLES: Record<string, { bg: string; ring: string; text: string }> = {
    twitch:  { bg: 'bg-twitch',  ring: 'focus-visible:ring-twitch',  text: 'text-white' },
    youtube: { bg: 'bg-youtube', ring: 'focus-visible:ring-youtube', text: 'text-white' },
    kick:    { bg: 'bg-kick',    ring: 'focus-visible:ring-kick',    text: 'text-bg' },
  }
  const dashStyle = DASHBOARD_BUTTON_STYLES[user?.auth_provider ?? ''] ?? DASHBOARD_BUTTON_STYLES.twitch

  const handleTwitchLogin = async () => {
    try {
      const response = await fetch('/api/v1/auth/twitch/login')
      const data = await response.json()
      if (data.auth_url) {
        window.location.href = data.auth_url
      } else {
        toastManager.add({ title: 'Login failed', description: 'No auth URL returned. Try again.' })
      }
    } catch {
      toastManager.add({ title: 'Login error', description: 'Failed to initiate Twitch login.' })
    }
  }

  const handleYouTubeLogin = async () => {
    try {
      const response = await fetch('/api/v1/auth/youtube/login')
      const data = await response.json()
      if (data.auth_url) {
        window.location.href = data.auth_url
      } else {
        toastManager.add({ title: 'Login failed', description: 'No auth URL returned. Try again.' })
      }
    } catch {
      toastManager.add({ title: 'Login error', description: 'Failed to initiate YouTube login.' })
    }
  }

  const handleKickLogin = async () => {
    try {
      const response = await fetch('/api/v1/auth/kick/login')
      const data = await response.json()
      if (data.auth_url) {
        window.location.href = data.auth_url
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
          Aggregate chat from Twitch, YouTube, Kick, and TikTok into a single stream overlay.
        </p>

        {/* Platform stat cards — magnetic glow hero */}
        <div className="mx-auto mb-12 grid w-full max-w-3xl grid-cols-2 gap-4 md:grid-cols-4">
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
                <div className="text-xs text-text-sub">messages / 7d</div>
              </MagGlowCard>
            )
          })}
        </div>

        {/* CTA — dashboard link for logged-in users, login buttons otherwise */}
        {isLoggedIn ? (
          <Link
            href="/dashboard"
            className={cn(
              'inline-flex items-center gap-2.5 rounded-lg px-8 py-3 font-semibold transition-opacity hover:opacity-90 focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none',
              dashStyle.bg, dashStyle.ring, dashStyle.text
            )}
          >
            <LayoutGrid className="h-5 w-5" aria-hidden="true" />
            Go to Dashboard
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
              Add all-chat overlays directly to any streaming site without OBS. Works in Chrome and
              Firefox.
            </p>
            <a
              href="https://github.com/caesarakalaeii/all-chat-extension/releases"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-block rounded-full border border-border bg-surface-2 px-3 py-1 text-xs font-medium text-text-sub underline-offset-4 hover:text-text hover:underline"
            >
              Install beta extension
            </a>
            <p className="mt-2 text-xs text-text-sub">Currently in beta — expect ongoing updates.</p>
          </div>
        </div>
      </section>

      {/* ------------------------------------------------------------------ */}
      {/* Footer                                                              */}
      {/* ------------------------------------------------------------------ */}
      <footer className="space-y-2 pb-12 text-center text-sm text-text-sub">
        <p>Open Source &bull; Built with Go + React &bull; Multi-Platform Chat Aggregation</p>
        <p className="flex flex-wrap items-center justify-center gap-3 text-xs">
          <a
            href="https://github.com/caesarakalaeii/all-chat"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-1 underline-offset-4 hover:text-text hover:underline"
          >
            <Github className="h-3.5 w-3.5" aria-hidden="true" />
            GitHub
          </a>
          <span aria-hidden="true">&bull;</span>
          <a
            href="https://discord.gg/xCGBSuz39P"
            target="_blank"
            rel="noopener noreferrer"
            className="underline-offset-4 hover:text-text hover:underline"
          >
            Discord
          </a>
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
        </p>
      </footer>
    </main>
  )
}
