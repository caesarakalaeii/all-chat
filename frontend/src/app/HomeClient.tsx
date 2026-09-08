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
 * and the JSON-LD structured data. Stays a Client Component because it reads
 * auth state and browser APIs.
 *
 * Structure:
 *   - LanesHero: five proportional platform lanes under fixed mono chrome.
 *   - Logged-out visitors get the full funnel (convergence, wedge, numbers,
 *     steps, FAQ, ambassadors) ending in the sign-in band; logged-in users
 *     get the hero's dashboard CTA plus a collapsed "Explore" row, so
 *     discovery stays one click away without re-showing the whole pitch.
 *   - The two display faces (Archivo Black / Space Mono) are scoped here via
 *     next/font CSS variables on the .lanes-home wrapper, not the root
 *     layout, which also wraps /overlay and /docs.
 */

'use client'

import Link from 'next/link'
import { useRouter } from 'next/navigation'
import { archivoBlack, spaceMono } from '@/lib/fonts'
import { useEffect, useState } from 'react'
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
import { WedgeSection } from '@/components/home/WedgeSection'
import { toastManager } from '@/lib/toast'
import { DISCORD_INVITE_URL } from '@/lib/constants'
import { trackEvent } from '@/lib/analytics'
import { stashSigninPlatform } from '@/lib/analytics-auth'
import { safeExternalRedirect } from '@/lib/auth/redirect-allowlist'
import { type TFunction, formatNumber, useTranslations } from '@/lib/i18n'
import { useReducedMotion } from '@/hooks/useReducedMotion'

/** The public /stats payload the counters below read. */
interface LandingStats {
  platforms: Record<string, number>
  all_time: number
  users: number
  overlays_live: number
}

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

export default function HomeClient() {
  const t = useTranslations()
  const router = useRouter()
  const reducedMotion = useReducedMotion()
  const { user, init } = useAuthStore()
  const [stats, setStats] = useState<LandingStats | null>(null)
  const [totalCount, setTotalCount] = useState(0)

  useEffect(() => {
    init()
  }, [init])

  useEffect(() => {
    fetch('/api/v1/stats')
      .then((r) => (r.ok ? r.json() : null))
      .then((data: LandingStats | null) => {
        if (!data) return
        setStats(data)
        if (data.all_time > 0) setTotalCount(data.all_time)
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
    <div className={cn('lanes-home', archivoBlack.variable, spaceMono.variable)}>
      <main id="main-content" tabIndex={-1} className="min-h-screen scroll-smooth">
        <LanesHero
          totalDisplay={totalDisplay}
          userName={user?.display_name}
          overlaysLive={overlaysLive}
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

        {/* ------------------------------------------------------------------ */}
        {/* Footer                                                              */}
        {/* ------------------------------------------------------------------ */}
        <footer
          className={cn(
            'lanes-footer space-y-2 border-t border-border pt-12 pb-12 text-center text-sm text-text-sub'
          )}
        >
          <p>{t('marketing.footer.tagline')}</p>
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
              {t('marketing.footer.github')}
            </a>
            <span aria-hidden="true">&bull;</span>
            <a
              href={DISCORD_INVITE_URL}
              target="_blank"
              rel="noopener noreferrer"
              onClick={() => trackEvent('outbound_click', { dest: 'discord' })}
              className="underline-offset-4 hover:text-text hover:underline"
            >
              {t('marketing.footer.discord')}
            </a>
            <span aria-hidden="true">&bull;</span>
            <Link href="/docs" className="underline-offset-4 hover:text-text hover:underline">
              {t('marketing.footer.docs')}
            </Link>
            <span aria-hidden="true">&bull;</span>
            <Link href="/docs/api" className="underline-offset-4 hover:text-text hover:underline">
              {t('marketing.footer.api')}
            </Link>
            <span aria-hidden="true">&bull;</span>
            <Link
              href="/legal/privacy"
              className="underline-offset-4 hover:text-text hover:underline"
            >
              {t('marketing.footer.privacy')}
            </Link>
            <span aria-hidden="true">&bull;</span>
            <Link
              href="/legal/terms"
              className="underline-offset-4 hover:text-text hover:underline"
            >
              {t('marketing.footer.terms')}
            </Link>
            <span aria-hidden="true">&bull;</span>
            <Link
              href="/legal/impressum"
              className="underline-offset-4 hover:text-text hover:underline"
            >
              {t('marketing.footer.impressum')}
            </Link>
          </p>
        </footer>
      </main>
    </div>
  )
}
