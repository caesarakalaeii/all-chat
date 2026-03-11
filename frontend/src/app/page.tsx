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
import { useRouter } from 'next/navigation'
import { useAuthStore } from '@/lib/stores/auth-store'
import { BetaWarning } from '@/components/BetaWarning'
import { PlatformBadge } from '@/components/ui/badge'
import { PLATFORM_COLORS } from '@/lib/platform-colors'
import { toastManager } from '@/lib/toast'
import { LayoutGrid, Zap, Palette } from 'lucide-react'
import { cn } from '@/lib/utils'

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
    // Ensure arrays are sized
    cardRefs.current = cardRefs.current.slice(0, count)
    glowRefs.current = glowRefs.current.slice(0, count)

    window.addEventListener('pointermove', handlePointerMove)
    return () => {
      window.removeEventListener('pointermove', handlePointerMove)
    }
  }, [count, handlePointerMove])

  return { cardRefs, glowRefs }
}

// ---------------------------------------------------------------------------
// MagGlowCard — card with a 300px pointer-tracking glow blob
// ---------------------------------------------------------------------------
function MagGlowCard({
  children,
  cardRef,
  glowRef,
  className,
}: {
  children: React.ReactNode
  cardRef: (el: HTMLDivElement | null) => void
  glowRef: (el: HTMLDivElement | null) => void
  className?: string
}) {
  return (
    <div
      ref={cardRef}
      className={cn(
        'relative overflow-hidden rounded-xl border border-border bg-surface p-6',
        className,
      )}
    >
      <div
        ref={glowRef}
        className="pointer-events-none absolute size-[300px] -translate-x-1/2 -translate-y-1/2 rounded-full opacity-0 transition-opacity duration-300"
        style={{
          background: 'radial-gradient(circle, rgba(163,123,255,0.15) 0%, transparent 70%)',
        }}
      />
      {children}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Platform stat data (static)
// ---------------------------------------------------------------------------
const PLATFORM_STATS = [
  { platform: 'twitch' as const, label: 'Twitch', stat: 'IRC-based', detail: 'Real-time chat' },
  { platform: 'youtube' as const, label: 'YouTube', stat: 'InnerTube', detail: 'Quota-free polling' },
  { platform: 'kick' as const, label: 'Kick', stat: 'Pusher WS', detail: 'Live WebSocket' },
  { platform: 'tiktok' as const, label: 'TikTok', stat: 'Live API', detail: 'Unofficial library' },
]

const TOTAL_CARDS = PLATFORM_STATS.length + 3 // 4 stat cards + 3 feature cards

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
    description: 'Low-latency delivery under 500ms. IRC for Twitch, WebSocket for Kick.',
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
  const router = useRouter()
  const { user, token } = useAuthStore()
  const [showBetaWarning, setShowBetaWarning] = useState<'youtube' | null>(null)

  const { cardRefs, glowRefs } = useMagneticGlow(TOTAL_CARDS)

  // Redirect to dashboard if already logged in
  useEffect(() => {
    if (user && token) {
      router.push('/dashboard')
    }
  }, [user, token, router])

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

  const handleYouTubeLogin = () => {
    setShowBetaWarning('youtube')
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

  const proceedWithYouTubeLogin = async () => {
    setShowBetaWarning(null)
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

  return (
    <main className="min-h-screen" style={{ background: '#07070a' }}>
      {/* ------------------------------------------------------------------ */}
      {/* Hero section                                                        */}
      {/* ------------------------------------------------------------------ */}
      <section className="relative flex flex-col items-center justify-center px-4 pt-24 pb-16 text-center">
        {/* Noise overlay — SVG feTurbulence at opacity 0.03 */}
        <div aria-hidden="true" className="pointer-events-none absolute inset-0 opacity-[0.03]">
          <svg width="100%" height="100%" xmlns="http://www.w3.org/2000/svg">
            <filter id="noise">
              <feTurbulence type="fractalNoise" baseFrequency="0.65" numOctaves="3" stitchTiles="stitch" />
              <feColorMatrix type="saturate" values="0" />
            </filter>
            <rect width="100%" height="100%" filter="url(#noise)" />
          </svg>
        </div>

        {/* Logo + headline */}
        <div className="relative flex items-center gap-3 mb-6">
          <div className="logo-ring" aria-hidden="true" />
          <span className="text-4xl font-extrabold tracking-tight text-text">all-chat</span>
        </div>

        <h1 className="text-5xl font-extrabold tracking-tight text-text mb-4 max-w-2xl">
          One overlay. Every platform.
        </h1>
        <p className="text-text-sub text-lg mb-10 max-w-xl">
          Aggregate chat from Twitch, YouTube, Kick, and TikTok into a single stream overlay.
        </p>

        {/* Platform stat cards — magnetic glow hero */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4 max-w-3xl w-full mx-auto mb-12">
          {PLATFORM_STATS.map((item, i) => (
            <MagGlowCard
              key={item.platform}
              cardRef={el => { cardRefs.current[i] = el }}
              glowRef={el => { glowRefs.current[i] = el }}
            >
              <PlatformBadge platform={item.platform} className="mb-3" />
              <div className={cn('text-2xl font-bold mb-1', PLATFORM_COLORS[item.platform].text)}>
                {item.stat}
              </div>
              <div className="text-sm text-text-sub">{item.detail}</div>
            </MagGlowCard>
          ))}
        </div>

        {/* Login buttons */}
        <div className="flex flex-col sm:flex-row gap-3 justify-center">
          {/* Twitch — purple bg, white text */}
          <button
            onClick={handleTwitchLogin}
            className="flex items-center gap-2.5 px-6 py-3 rounded-lg bg-twitch text-white font-semibold hover:opacity-90 transition-opacity focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
            aria-label="Sign in with Twitch"
          >
            {/* Twitch icon */}
            <svg className="w-5 h-5 shrink-0" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
              <path fill="#FFFFFF" d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714z" />
            </svg>
            Sign in with Twitch
          </button>

          {/* YouTube — white play icon on red per YouTube branding guidelines */}
          <button
            onClick={handleYouTubeLogin}
            className="flex items-center gap-2.5 px-6 py-3 rounded-lg bg-youtube text-white font-semibold hover:opacity-90 transition-opacity focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-youtube focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
            aria-label="Sign in with YouTube"
          >
            {/* YouTube play icon (white) */}
            <svg className="w-5 h-5 shrink-0" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
              <path fill="#FFFFFF" d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z" />
            </svg>
            Sign in with YouTube
          </button>

          {/* Kick — brand green bg, dark text (Kick branding) */}
          <button
            onClick={handleKickLogin}
            className="flex items-center gap-2.5 px-6 py-3 rounded-lg text-bg font-semibold hover:opacity-90 transition-opacity focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-kick focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
            style={{ backgroundColor: 'var(--color-kick)' }}
            aria-label="Sign in with Kick"
          >
            Sign in with Kick
          </button>
        </div>
      </section>

      {/* ------------------------------------------------------------------ */}
      {/* Feature grid — same magnetic glow cards                            */}
      {/* ------------------------------------------------------------------ */}
      <section className="px-4 pb-24 max-w-5xl mx-auto">
        <h2 className="text-2xl font-bold text-text text-center mb-8">Why all-chat?</h2>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {FEATURE_CARDS.map((feature, i) => {
            const Icon = feature.icon
            const cardIdx = PLATFORM_STATS.length + i
            return (
              <MagGlowCard
                key={feature.title}
                cardRef={el => { cardRefs.current[cardIdx] = el }}
                glowRef={el => { glowRefs.current[cardIdx] = el }}
              >
                <div className="mb-4 text-text-sub">
                  <Icon className="w-7 h-7" aria-hidden="true" />
                </div>
                <h3 className="text-lg font-semibold text-text mb-2">{feature.title}</h3>
                <p className="text-sm text-text-sub">{feature.description}</p>
              </MagGlowCard>
            )
          })}
        </div>
      </section>

      {/* ------------------------------------------------------------------ */}
      {/* Footer                                                              */}
      {/* ------------------------------------------------------------------ */}
      <footer className="pb-12 text-center text-text-sub text-sm space-y-2">
        <p>Open Source &bull; Built with Go + React &bull; Multi-Platform Chat Aggregation</p>
        <p className="flex flex-wrap items-center justify-center gap-3 text-xs">
          <a
            href="https://discord.gg/xCGBSuz39P"
            target="_blank"
            rel="noopener noreferrer"
            className="hover:text-text underline-offset-4 hover:underline flex items-center gap-1"
          >
            Get Support on Discord
          </a>
          <span aria-hidden="true">&bull;</span>
          <Link href="/legal/privacy" className="hover:text-text underline-offset-4 hover:underline">
            Privacy Policy
          </Link>
          <span aria-hidden="true">&bull;</span>
          <Link href="/legal/terms" className="hover:text-text underline-offset-4 hover:underline">
            Terms of Service
          </Link>
        </p>
      </footer>

      {/* Beta Warning Dialog */}
      {showBetaWarning && (
        <BetaWarning
          platform={showBetaWarning}
          onCancel={() => setShowBetaWarning(null)}
          onContinue={() => {
            setShowBetaWarning(null)
            proceedWithYouTubeLogin()
          }}
        />
      )}
    </main>
  )
}
