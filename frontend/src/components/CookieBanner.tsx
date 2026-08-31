'use client'

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
 * GDPR-Compliant Cookie Banner
 *
 * This component displays a cookie consent banner that complies with GDPR requirements.
 *
 * Current Cookie Usage in All-Chat:
 * - Essential: localStorage items for authentication (jwt_token). The refresh
 *   token is stored in-memory only (audit H3) and is never persisted to
 *   localStorage.
 * - Analytics: Self-hosted Umami — cookieless, sets nothing on the device, stores no personal data
 * - Functional: None (no additional functional cookies)
 * - Third-party: None (no third-party cookies)
 *
 * The banner informs users about essential data storage and provides links to
 * privacy policy and terms of service as required by GDPR.
 */

import { useState, useEffect } from 'react'
import { useHydrated } from '@/hooks/useHydrated'
import { useTranslations } from '@/lib/i18n'
import { interpolateElements } from '@/lib/i18n/emphasise'

// Not copy: the glyphs the list draws beside an affirmed or denied row. They
// are U+2713 CHECK MARK and U+2717 BALLOT X, rendered as decoration next to
// text that already says which is which.
const AFFIRMED_GLYPH = '✓'
const DENIED_GLYPH = '✗'

// Not copy either: the banner's illustration. It carries role="img" and an
// aria-label, so its accessible name is the translated string, not this glyph.
const COOKIE_GLYPH = '🍪'

export default function CookieBanner() {
  const t = useTranslations()
  const isHydrated = useHydrated()
  const [showBanner, setShowBanner] = useState(false)

  useEffect(() => {
    if (!isHydrated) return // Wait for hydration

    // Do not render the banner on public overlays where it obstructs the chat view
    if (window.location.pathname.startsWith('/overlay')) {
      return
    }

    // Check if user has already acknowledged the banner
    const acknowledged = localStorage.getItem('cookieBannerAcknowledged')
    if (!acknowledged) {
      // Show banner after a short delay for better UX
      const timer = setTimeout(() => setShowBanner(true), 1000)
      return () => clearTimeout(timer)
    }
  }, [isHydrated])

  const acknowledgeBanner = () => {
    localStorage.setItem('cookieBannerAcknowledged', 'true')
    setShowBanner(false)
  }

  if (!isHydrated || !showBanner) return null

  return (
    <div className="pointer-events-none fixed inset-0 z-50 flex items-end justify-center p-4">
      <div
        role="region"
        aria-label={t('legal.cookieBanner.regionLabel')}
        className="animate-slide-up pointer-events-auto w-full max-w-4xl rounded-xl border border-border bg-surface shadow-2xl"
      >
        {/* Main Banner */}
        <div className="p-6">
          <div className="flex items-start gap-4">
            {/* Cookie Icon */}
            <div
              className="flex-shrink-0 text-4xl"
              role="img"
              aria-label={t('legal.cookieBanner.iconLabel')}
            >
              {COOKIE_GLYPH}
            </div>

            {/* Content */}
            <div className="flex-1">
              <h2 className="mb-2 text-xl font-semibold text-text">
                {t('legal.cookieBanner.title')}
              </h2>
              <p className="mb-3 text-sm text-text-sub">
                {interpolateElements(t('legal.cookieBanner.storageBody'), {
                  storage: (
                    <strong className="text-text">{t('legal.cookieBanner.storageEmphasis')}</strong>
                  ),
                })}
              </p>
              <p className="mb-3 text-sm text-text-sub">
                {interpolateElements(t('legal.cookieBanner.noTrackingBody'), {
                  emphasis: (
                    <strong className="text-text">
                      {t('legal.cookieBanner.noTrackingEmphasis')}
                    </strong>
                  ),
                })}
              </p>

              {/* What we store */}
              <details className="mb-4">
                <summary className="cursor-pointer text-sm font-medium text-text transition-colors hover:text-twitch">
                  {t('legal.cookieBanner.detailsSummary')}
                </summary>
                <div className="mt-2 space-y-2 pl-4 text-sm text-text-sub">
                  <div className="flex items-start gap-2">
                    <span className="mt-0.5 text-kick">{AFFIRMED_GLYPH}</span>
                    <div>
                      {interpolateElements(t('legal.cookieBanner.tokensRow'), {
                        label: (
                          <strong className="text-text">
                            {t('legal.cookieBanner.tokensLabel')}
                          </strong>
                        ),
                      })}
                    </div>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="mt-0.5 text-kick">{AFFIRMED_GLYPH}</span>
                    <div>
                      {interpolateElements(t('legal.cookieBanner.preferencesRow'), {
                        label: (
                          <strong className="text-text">
                            {t('legal.cookieBanner.preferencesLabel')}
                          </strong>
                        ),
                      })}
                    </div>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="mt-0.5 text-youtube">{DENIED_GLYPH}</span>
                    <div>
                      {interpolateElements(t('legal.cookieBanner.noTrackingRow'), {
                        label: (
                          <strong className="text-text">
                            {t('legal.cookieBanner.noTrackingLabel')}
                          </strong>
                        ),
                      })}
                    </div>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="mt-0.5 text-youtube">{DENIED_GLYPH}</span>
                    <div>
                      {interpolateElements(t('legal.cookieBanner.noAdsRow'), {
                        label: (
                          <strong className="text-text">
                            {t('legal.cookieBanner.noAdsLabel')}
                          </strong>
                        ),
                      })}
                    </div>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="mt-0.5 text-kick">{AFFIRMED_GLYPH}</span>
                    <div>
                      {interpolateElements(t('legal.cookieBanner.analyticsRow'), {
                        label: (
                          <strong className="text-text">
                            {t('legal.cookieBanner.analyticsLabel')}
                          </strong>
                        ),
                      })}
                    </div>
                  </div>
                  <div className="mt-3 border-t border-border pt-2">
                    <p className="text-xs text-text-dim">
                      {interpolateElements(t('legal.cookieBanner.fontsNote'), {
                        label: (
                          <strong className="text-text">
                            {t('legal.cookieBanner.fontsLabel')}
                          </strong>
                        ),
                      })}
                    </p>
                    <p className="mt-2 text-xs text-text-dim">
                      {interpolateElements(t('legal.cookieBanner.thirdPartyNote'), {
                        label: (
                          <strong className="text-text">
                            {t('legal.cookieBanner.thirdPartyLabel')}
                          </strong>
                        ),
                        avatars: <strong>{t('legal.cookieBanner.thirdPartyAvatars')}</strong>,
                        github: <strong>{t('legal.cookieBanner.thirdPartyGithub')}</strong>,
                        privacy: (
                          <a
                            href="/legal/privacy"
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-twitch underline underline-offset-2"
                          >
                            {t('legal.cookieBanner.privacyPolicy')}
                          </a>
                        ),
                      })}
                    </p>
                  </div>
                </div>
              </details>

              <p className="mb-4 text-sm text-text-sub">
                {interpolateElements(t('legal.cookieBanner.agreement'), {
                  privacy: (
                    <a
                      href="/legal/privacy"
                      target="_blank"
                      rel="noopener noreferrer"
                      className="font-medium text-twitch underline decoration-twitch/30 underline-offset-4"
                    >
                      {t('legal.cookieBanner.privacyPolicy')}
                    </a>
                  ),
                  terms: (
                    <a
                      href="/legal/terms"
                      target="_blank"
                      rel="noopener noreferrer"
                      className="font-medium text-twitch underline decoration-twitch/30 underline-offset-4"
                    >
                      {t('legal.cookieBanner.termsOfService')}
                    </a>
                  ),
                })}
              </p>

              {/* Action Buttons */}
              <div className="flex flex-wrap gap-3">
                <button
                  onClick={acknowledgeBanner}
                  className="rounded-lg bg-twitch px-6 py-2.5 font-medium text-bg transition-colors hover:bg-twitch/80 focus-visible:ring-3 focus-visible:ring-twitch/50 focus-visible:outline-none"
                >
                  {t('legal.cookieBanner.acknowledge')}
                </button>
                <a
                  href="/legal/privacy"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center justify-center rounded-lg border border-border bg-surface-2 px-6 py-2.5 font-medium text-text transition-colors hover:bg-surface-2/80 focus-visible:ring-3 focus-visible:ring-twitch/50 focus-visible:outline-none"
                >
                  {t('legal.cookieBanner.learnMore')}
                </a>
              </div>
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="rounded-b-xl border-t border-border bg-bg px-6 py-3">
          <p className="text-center text-xs text-text-dim">{t('legal.cookieBanner.footer')}</p>
        </div>
      </div>
    </div>
  )
}
