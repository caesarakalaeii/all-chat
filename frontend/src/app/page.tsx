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
import { LayoutGrid, Zap, Palette, Puzzle, Github } from 'lucide-react'
import { cn } from '@/lib/utils'

// ---------------------------------------------------------------------------
// InfinityLogo — animated 4-colour infinity snake inside a chat bubble
// Variant B (solid bubble, 2.5px stroke). Animation via rAF dashoffset trick.
// ---------------------------------------------------------------------------
function InfinityLogo({ size = 36 }: { size?: number }) {
  const svgRef = useRef<SVGSVGElement>(null)
  const rafRef = useRef<number>(0)

  useEffect(() => {
    const svg = svgRef.current
    if (!svg) return

    const segs = svg.querySelectorAll<SVGPathElement>('.inf-seg')
    const segBs = svg.querySelectorAll<SVGPathElement>('.inf-seg-b')
    if (!segs.length) return

    const total = segs[0].getTotalLength()
    const SEG_FRAC = 0.55
    const LOOP_MS = 6000
    const seg = total * SEG_FRAC
    const piece = seg / 4

    function tick(now: number) {
      const head = ((now / LOOP_MS) * total) % total
      segs.forEach((path, ci) => {
        const colourOffset = ci * piece
        const t = ((head - colourOffset) % total + total) % total
        path.style.strokeDasharray = `${piece} ${total * 2}`
        path.style.strokeDashoffset = `${-t}`
        const b = segBs[ci]
        if (b) {
          b.style.strokeDasharray = `${piece} ${total * 2}`
          b.style.strokeDashoffset = `${-(t - total)}`
        }
      })
      rafRef.current = requestAnimationFrame(tick)
    }

    rafRef.current = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(rafRef.current)
  }, [])

  const inf = 'M6 10c5 0 7-8 12-8a4 4 0 0 1 0 8c-5 0-7-8-12-8a4 4 0 1 0 0 8'

  return (
    <div
      className="relative flex items-center justify-center shrink-0"
      style={{ width: size, height: size }}
      aria-hidden="true"
    >
      {/* Chat bubble background */}
      <svg width={size} height={size} viewBox="0 0 24 24" fill="none" className="absolute inset-0">
        <path
          fillRule="evenodd"
          clipRule="evenodd"
          d="M4.84836 2.771C7.18302 2.42773 9.57113 2.25 12.0003 2.25C14.4292 2.25 16.8171 2.4277 19.1516 2.77091C21.1299 3.06177 22.5 4.79445 22.5 6.74056V12.7595C22.5 14.7056 21.1299 16.4382 19.1516 16.7291C17.2123 17.0142 15.2361 17.1851 13.2302 17.2348C13.1266 17.2374 13.0318 17.2788 12.9638 17.3468L8.78033 21.5303C8.56583 21.7448 8.24324 21.809 7.96299 21.6929C7.68273 21.5768 7.5 21.3033 7.5 21V17.045C6.60901 16.9634 5.72491 16.8579 4.84836 16.729C2.87004 16.4381 1.5 14.7054 1.5 12.7593V6.74064C1.5 4.79455 2.87004 3.06188 4.84836 2.771Z"
          fill="rgba(255,255,255,0.07)"
          stroke="rgba(255,255,255,0.10)"
          strokeWidth="0.5"
        />
      </svg>
      {/* Infinity snake — positioned with -10% upward nudge to centre optically */}
      <svg
        ref={svgRef}
        style={{
          position: 'absolute',
          width: size * 0.67,
          height: size * 0.39,
          transform: 'translateY(-10%)',
          filter: 'drop-shadow(0 0 3px rgba(0,0,0,0.9))',
        }}
        viewBox="0 0 24 14"
        fill="none"
      >
        {/* Ghost trail */}
        <path d={inf} stroke="rgba(255,255,255,0.08)" strokeWidth="2.5" strokeLinecap="round" />
        {/* 4 colour segments (primary + secondary copy for seamless wrap) */}
        <path className="inf-seg" d={inf} stroke="#9146FF" strokeWidth="2.5" strokeLinecap="round" />
        <path className="inf-seg-b" d={inf} stroke="#9146FF" strokeWidth="2.5" strokeLinecap="round" />
        <path className="inf-seg" d={inf} stroke="#FF0000" strokeWidth="2.5" strokeLinecap="round" />
        <path className="inf-seg-b" d={inf} stroke="#FF0000" strokeWidth="2.5" strokeLinecap="round" />
        <path className="inf-seg" d={inf} stroke="#53FC18" strokeWidth="2.5" strokeLinecap="round" />
        <path className="inf-seg-b" d={inf} stroke="#53FC18" strokeWidth="2.5" strokeLinecap="round" />
        <path className="inf-seg" d={inf} stroke="#69C9D0" strokeWidth="2.5" strokeLinecap="round" />
        <path className="inf-seg-b" d={inf} stroke="#69C9D0" strokeWidth="2.5" strokeLinecap="round" />
      </svg>
    </div>
  )
}

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
  twitch:  'var(--color-stat-glow-twitch)',
  youtube: 'var(--color-stat-glow-youtube)',
  kick:    'var(--color-stat-glow-kick)',
  tiktok:  'var(--color-stat-glow-tiktok)',
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
        className,
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
// Platform stat data — no internal tech details exposed
// ---------------------------------------------------------------------------
const PLATFORM_STATS = [
  { platform: 'twitch'  as const, label: 'Twitch',  stat: 'Real-time',   detail: 'Live chat' },
  { platform: 'youtube' as const, label: 'YouTube', stat: 'Live Chat',    detail: 'Stream chat' },
  { platform: 'kick'    as const, label: 'Kick',    stat: 'WebSocket',    detail: 'Live chat' },
  { platform: 'tiktok'  as const, label: 'TikTok',  stat: 'Live',         detail: 'LIVE chat' },
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
    <main className="min-h-screen">
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

        {/* Logo + wordmark */}
        <div className="relative flex items-center gap-3 mb-6">
          <InfinityLogo size={36} />
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
              glowColor={PLATFORM_GLOW_COLORS[item.platform]}
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
          {/* Twitch */}
          <button
            onClick={handleTwitchLogin}
            className="flex items-center gap-2.5 px-6 py-3 rounded-lg bg-twitch text-white font-semibold hover:opacity-90 transition-opacity focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-twitch focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
            aria-label="Sign in with Twitch"
          >
            <svg className="w-5 h-5 shrink-0" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
              <path fill="#FFFFFF" d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714z" />
            </svg>
            Sign in with Twitch
          </button>

          {/* YouTube — exact brand red #FF0000, white text + icon per YouTube guidelines */}
          <button
            onClick={handleYouTubeLogin}
            className="flex items-center gap-2.5 px-6 py-3 rounded-lg text-white font-semibold hover:opacity-90 transition-opacity focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
            style={{ backgroundColor: '#FF0000', '--tw-ring-color': '#FF0000' } as React.CSSProperties}
            aria-label="Sign in with YouTube"
          >
            {/* Official YouTube icon — white play button on brand red */}
            <svg className="w-5 h-5 shrink-0" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
              <path fill="#FFFFFF" d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z" />
            </svg>
            Sign in with YouTube
          </button>

          {/* Kick — brand green, dark text */}
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
      <section className="px-4 pb-16 max-w-5xl mx-auto">
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
      {/* Extension promo                                                     */}
      {/* ------------------------------------------------------------------ */}
      <section className="px-4 pb-16 max-w-3xl mx-auto">
        <div className="flex flex-col sm:flex-row items-center gap-6 rounded-xl border border-border bg-surface p-8 text-center sm:text-left">
          <div className="flex-shrink-0 rounded-xl bg-surface-2 p-4">
            <Puzzle className="w-10 h-10 text-twitch" aria-hidden="true" />
          </div>
          <div className="flex-1">
            <h2 className="text-xl font-bold text-text mb-1">Browser Extension</h2>
            <p className="text-sm text-text-sub mb-3">
              Add all-chat overlays directly to any streaming site without OBS. Works in Chrome and Firefox.
            </p>
            <span className="inline-block rounded-full bg-surface-2 px-3 py-1 text-xs font-medium text-text-sub border border-border">
              Coming soon
            </span>
          </div>
        </div>
      </section>

      {/* ------------------------------------------------------------------ */}
      {/* Footer                                                              */}
      {/* ------------------------------------------------------------------ */}
      <footer className="pb-12 text-center text-text-sub text-sm space-y-2">
        <p>Open Source &bull; Built with Go + React &bull; Multi-Platform Chat Aggregation</p>
        <p className="flex flex-wrap items-center justify-center gap-3 text-xs">
          <a
            href="https://github.com/caesarakalaeii/all-chat"
            target="_blank"
            rel="noopener noreferrer"
            className="hover:text-text underline-offset-4 hover:underline flex items-center gap-1"
          >
            <Github className="w-3.5 h-3.5" aria-hidden="true" />
            GitHub
          </a>
          <span aria-hidden="true">&bull;</span>
          <a
            href="https://discord.gg/xCGBSuz39P"
            target="_blank"
            rel="noopener noreferrer"
            className="hover:text-text underline-offset-4 hover:underline"
          >
            Discord
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
