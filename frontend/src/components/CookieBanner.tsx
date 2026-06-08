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
 * - Essential: localStorage items for authentication (jwt_token, refresh_token)
 * - Analytics: Self-hosted Umami — cookieless, sets nothing on the device, stores no personal data
 * - Functional: None (no additional functional cookies)
 * - Third-party: None (no third-party cookies)
 *
 * The banner informs users about essential data storage and provides links to
 * privacy policy and terms of service as required by GDPR.
 */

import { useState, useEffect } from 'react'
import { useHydrated } from '@/hooks/useHydrated'

export default function CookieBanner() {
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
      <div className="animate-slide-up pointer-events-auto w-full max-w-4xl rounded-xl border border-border bg-surface shadow-2xl">
        {/* Main Banner */}
        <div className="p-6">
          <div className="flex items-start gap-4">
            {/* Cookie Icon */}
            <div className="flex-shrink-0 text-4xl" role="img" aria-label="Cookie">
              🍪
            </div>

            {/* Content */}
            <div className="flex-1">
              <h2 className="mb-2 text-xl font-semibold text-text">Privacy &amp; Data Storage</h2>
              <p className="mb-3 text-sm text-text-sub">
                All-Chat uses <strong className="text-text">browser local storage</strong> to save
                your authentication tokens and keep you logged in. This is essential for the service
                to function properly.
              </p>
              <p className="mb-3 text-sm text-text-sub">
                <strong className="text-text">
                  We use no tracking cookies, and our usage analytics are cookieless and store no
                  personal data.
                </strong>{' '}
                We do not share your data with third parties for advertising purposes.
              </p>

              {/* What we store */}
              <details className="mb-4">
                <summary className="cursor-pointer text-sm font-medium text-text transition-colors hover:text-twitch">
                  What data do we store locally?
                </summary>
                <div className="mt-2 space-y-2 pl-4 text-sm text-text-sub">
                  <div className="flex items-start gap-2">
                    <span className="mt-0.5 text-kick">&#10003;</span>
                    <div>
                      <strong className="text-text">Authentication tokens</strong> - Required to
                      keep you logged in and authenticate with streaming platforms (Twitch, YouTube,
                      TikTok, Kick)
                    </div>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="mt-0.5 text-kick">&#10003;</span>
                    <div>
                      <strong className="text-text">User preferences</strong> - Your overlay
                      configurations and settings
                    </div>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="mt-0.5 text-youtube">&#10007;</span>
                    <div>
                      <strong className="text-text">No tracking cookies</strong> - We don&apos;t
                      track your browsing behavior
                    </div>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="mt-0.5 text-youtube">&#10007;</span>
                    <div>
                      <strong className="text-text">No advertising cookies</strong> - We don&apos;t
                      serve ads or share data with advertisers
                    </div>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="mt-0.5 text-kick">&#10003;</span>
                    <div>
                      <strong className="text-text">Cookieless analytics</strong> - We measure
                      aggregate usage with self-hosted Umami. It sets no cookies, stores no personal
                      identifier, and does not track public overlays
                    </div>
                  </div>
                  <div className="mt-3 border-t border-border pt-2">
                    <p className="text-xs text-text-dim">
                      <strong className="text-text">Fonts:</strong> All fonts (including ones
                      originally distributed by Google Fonts) are self-hosted on our
                      infrastructure &ndash; your IP address is never sent to Google.
                    </p>
                    <p className="mt-2 text-xs text-text-dim">
                      <strong className="text-text">Third-party resources:</strong> Dashboard
                      pages may load fallback avatars from <strong>UI Avatars</strong> and themes
                      from the <strong>GitHub API</strong>. These requests transmit your IP
                      address to the respective providers. See our{' '}
                      <a
                        href="/legal/privacy"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-twitch underline underline-offset-2"
                      >
                        Privacy Policy
                      </a>{' '}
                      for details.
                    </p>
                  </div>
                </div>
              </details>

              <p className="mb-4 text-sm text-text-sub">
                By using All-Chat, you agree to this essential data storage. For more details, please
                read our{' '}
                <a
                  href="/legal/privacy"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="font-medium text-twitch underline decoration-twitch/30 underline-offset-4"
                >
                  Privacy Policy
                </a>{' '}
                and{' '}
                <a
                  href="/legal/terms"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="font-medium text-twitch underline decoration-twitch/30 underline-offset-4"
                >
                  Terms of Service
                </a>
                .
              </p>

              {/* Action Buttons */}
              <div className="flex flex-wrap gap-3">
                <button
                  onClick={acknowledgeBanner}
                  className="rounded-lg bg-twitch px-6 py-2.5 font-medium text-white transition-colors hover:bg-twitch/80 focus-visible:ring-3 focus-visible:ring-twitch/50 focus-visible:outline-none"
                >
                  I Understand
                </button>
                <a
                  href="/legal/privacy"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center justify-center rounded-lg border border-border bg-surface-2 px-6 py-2.5 font-medium text-text transition-colors hover:bg-surface-2/80 focus-visible:ring-3 focus-visible:ring-twitch/50 focus-visible:outline-none"
                >
                  Learn More
                </a>
              </div>
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="rounded-b-xl border-t border-border bg-bg px-6 py-3">
          <p className="text-center text-xs text-text-dim">
            Your data is stored locally in your browser and transmitted securely via HTTPS
          </p>
        </div>
      </div>
    </div>
  )
}
