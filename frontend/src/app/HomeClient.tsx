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
 * along with this program. If not, <https://www.gnu.org/licenses/>.
 */

/**
 * Landing Page — client portion (mockup G "lanes" redesign)
 *
 * Rendered by the server wrapper in `page.tsx`, which owns the page metadata
 * and the JSON-LD structured data, and passes the server-read landing stats as
 * `initialStats` so the first HTML carries real numbers. Stays a Client
 * Component because it reads auth state and browser APIs.
 *
 * Structure:
 *   - LanesHero: five proportional platform lanes under fixed mono chrome.
 *   - Logged-out visitors get the full funnel (convergence, wedge, numbers,
 *     steps, theme showcase, FAQ, ambassadors) ending in the sign-in band; logged-in users
 *     get the hero's dashboard CTA plus a collapsed "Explore" row, so
 *     discovery stays one click away without re-showing the whole pitch.
 *   - The footer is SiteFooter, mounted by the root layout for every page.
 *   - The two display faces (Archivo Black / Space Mono) are scoped here via
 *     next/font CSS variables on the .lanes-home wrapper, not the root
 *     layout, which also wraps /overlay and /docs.
 */

'use client'

import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { archivoBlack, spaceMono } from '@/lib/fonts'
import { useEffect, useRef, useState } from 'react'
import { ChevronDown } from 'lucide-react'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/lib/stores/auth-store'
import { FaqSection } from '@/components/FaqSection'
import { FeaturedAmbassadors } from '@/components/FeaturedAmbassadors'
import { ConvergenceSection } from '@/components/home/ConvergenceSection'
import { FinalSection } from '@/components/home/FinalSection'
import { LanesHero } from '@/components/home/LanesHero'
import { NumbersSection } from '@/components/home/NumbersSection'
import { StepsSection } from '@/components/home/StepsSection'
import { ThemeShowcaseSection } from '@/components/home/ThemeShowcaseSection'
import { WedgeSection } from '@/components/home/WedgeSection'
import { toastManager } from '@/lib/toast'
import { trackEvent } from '@/lib/analytics'
import type { LandingStats } from '@/lib/api/stats'
import { stashSigninPlatform } from '@/lib/analytics-auth'
import { safeExternalRedirect } from '@/lib/auth/redirect-allowlist'
import { type TFunction, formatNumber, useTranslations } from '@/lib/i18n'
import { useReducedMotion } from '@/hooks/useReducedMotion'
import { useReveal } from '@/hooks/useReveal'

/**
 * The failure copy is identical for every platform except the platform name in
 * `errorBody`, so all three login handlers raise it from here. Takes `t` as its
 * first argument because a module-scope function cannot call a hook.
 */
function reportLoginFailure(t: TFunction, reason: 'noAuthUrl' | 'requestFailed', platform: string) {
  if (reason === 'noAuthUrl') {
    toastManager.add({
      title: t('marketing.login.failedTitle'),
      description: t('marketing.login.failedBody'),
    })
    return
  }
  toastManager.add({
    title: t('marketing.login.errorTitle'),
    description: t('marketing.login.errorBody', { platform }),
  })
}

export default function HomeClient({ initialStats }: { initialStats: LandingStats | null }) {
  const t = useTranslations()
  const router = useRouter()
  const reducedMotion = useReducedMotion()
  const { user, init } = useAuthStore()
  const [stats, setStats] = useState<LandingStats | null>(initialStats)
  const [totalCount, setTotalCount] = useState(initialStats?.all_time ?? 0)
  const homeRef = useRef<HTMLDivElement>(null)

  // Scroll-activated reveals: one observer for every [data-reveal] section
  // on the page (see useReveal.ts). Scoped from this wrapper so the hook
  // does not need to run in each section component.
  useReveal(homeRef)

  useEffect(() => {
    init()
  }, [init])

  useEffect(() => {
    fetch('/api/v1/stats')
      .then((r) => (r.ok ? r.json() : null))
      .then((data: LandingStats | null) => {
        if (!data) return
        setStats(data)
        if (data.all_time > 0) setTotalCount((count) => Math.max(count, data.all_time))
      })
      .catch(() => {}) // fail silently — stats are decorative
  }, [])

  // All-time counter, ticking. The hero total and the numbers row share this
  // state, so both stay in lockstep; the tick is JS motion, so it stops under
  // prefers-reduced-motion (the CSS drift marquee is gated globally already).
  useEffect(() => {
    if (reducedMotion || totalCount === 0) return
    const timer = window.setInterval(
      () => setTotalCount((count) => count + 1 + Math.floor(Math.random() * 5)),
      1200
    )
    return () => window.clearInterval(timer)
  }, [reducedMotion, totalCount === 0])

  const isLoggedIn = !!user
  const totalDisplay = formatNumber(totalCount)
  const overlaysLive = stats?.overlays_live ?? 0
  // Real per-platform weekly shares, handed to the hero so lane heights
  // reflect live traffic (decorative morphing lives in LanesHero).
  const platformShares = stats?.platforms ?? null

  const handleTwitchLogin = async () => {
    trackEvent('signin_started', { platform: 'twitch' })
    // Remember the clicked platform so /auth/callback can attribute the eventual
    // signin_completed reliably (auth_provider is often empty on the exchange).
    stashSigninPlatform('twitch')
    try {
      const response = await fetch('/api/v1/auth/twitch/login')
      const data = await response.json()
      if (data.auth_url) {
        safeExternalRedirect(data.auth_url)
      } else {
        reportLoginFailure(t, 'noAuthUrl', t('common.platforms.twitch'))
      }
    } catch {
      reportLoginFailure(t, 'requestFailed', t('common.platforms.twitch'))
    }
  }

  const handleYouTubeLogin = async () => {
    trackEvent('signin_started', { platform: 'youtube' })
    stashSigninPlatform('youtube')
    try {
      const response = await fetch('/api/v1/auth/youtube/login')
      const data = await response.json()
      if (data.auth_url) {
        safeExternalRedirect(data.auth_url)
      } else {
        reportLoginFailure(t, 'noAuthUrl', t('common.platforms.youtube'))
      }
    } catch {
      reportLoginFailure(t, 'requestFailed', t('common.platforms.youtube'))
    }
  }

  const handleKickLogin = async () => {
    trackEvent('signin_started', { platform: 'kick' })
    stashSigninPlatform('kick')
    try {
      const response = await fetch('/api/v1/auth/kick/login')
      const data = await response.json()
      if (data.auth_url) {
        safeExternalRedirect(data.auth_url)
      } else {
        reportLoginFailure(t, 'noAuthUrl', t('common.platforms.kick'))
      }
    } catch {
      reportLoginFailure(t, 'requestFailed', t('common.platforms.kick'))
    }
  }

  /** The hero and final CTAs: dashboard for the returning user, the sign-in
   * band for everyone else. */
  const handleCta = () => {
    if (isLoggedIn) {
      trackEvent('cta_click', { cta: 'dashboard', location: 'lanes-hero' })
      router.push('/dashboard')
      return
    }
    trackEvent('cta_click', { cta: 'get-started', location: 'lanes-hero' })
    document.getElementById('get-started')?.scrollIntoView()
  }

  return (
    <div ref={homeRef} className={cn('lanes-home', archivoBlack.variable, spaceMono.variable)}>
      <main id="main-content" tabIndex={-1} className="min-h-screen scroll-smooth">
        <LanesHero
          totalCount={totalCount}
          userName={user?.display_name}
          overlaysLive={overlaysLive}
          platformShares={platformShares}
          onCta={handleCta}
        />

        {isLoggedIn ? (
          /* ---------------------------------------------------------------- */
          /* Returning user — discovery stays one click away, no re-pitch      */
          /* ---------------------------------------------------------------- */
          <details className="group mx-auto w-full max-w-2xl px-4 pb-20">
            <summary className="flex cursor-pointer list-none items-center justify-center gap-2 rounded-lg border border-border bg-surface px-4 py-3 text-sm font-medium text-text-sub transition-colors hover:text-text focus-visible:ring-2 focus-visible:ring-twitch focus-visible:ring-offset-2 focus-visible:ring-offset-bg focus-visible:outline-none [&::-webkit-details-marker]:hidden">
              {t('marketing.explore.summary')}
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
                {t('marketing.explore.extension')}
              </a>
              <Link
                href="/docs/api"
                onClick={() => trackEvent('cta_click', { cta: 'api-docs', location: 'explore' })}
                className="underline-offset-4 hover:text-text hover:underline"
              >
                {t('marketing.explore.api')}
              </Link>
              <Link href="/docs" className="underline-offset-4 hover:text-text hover:underline">
                {t('marketing.explore.docsAndFaq')}
              </Link>
            </div>
          </details>
        ) : (
          <>
            <ConvergenceSection />
            <WedgeSection />
            <NumbersSection
              platforms={stats?.platforms ?? null}
              totalDisplay={totalDisplay}
              users={stats?.users ?? 0}
              overlaysLive={overlaysLive}
            />
            <StepsSection />
            <ThemeShowcaseSection />
            <FaqSection />
            <FeaturedAmbassadors />
          </>
        )}

        <FinalSection
          userName={user?.display_name}
          onTwitchLogin={handleTwitchLogin}
          onYouTubeLogin={handleYouTubeLogin}
          onKickLogin={handleKickLogin}
          onCta={handleCta}
        />
      </main>
    </div>
  )
}
