'use client';

/**
 * GDPR-Compliant Cookie Banner
 *
 * This component displays a cookie consent banner that complies with GDPR requirements.
 *
 * Current Cookie Usage in All-Chat:
 * - Essential: localStorage items for authentication (jwt_token, refresh_token)
 * - Analytics: None (no tracking scripts currently installed)
 * - Functional: None (no additional functional cookies)
 * - Third-party: None (no third-party cookies)
 *
 * The banner informs users about essential data storage and provides links to
 * privacy policy and terms of service as required by GDPR.
 */

import { useState, useEffect } from 'react';
import { useHydrated } from '@/hooks/useHydrated';

export default function CookieBanner() {
  const isHydrated = useHydrated();
  const [showBanner, setShowBanner] = useState(false);

  useEffect(() => {
    if (!isHydrated) return; // Wait for hydration

    // Do not render the banner on public overlays where it obstructs the chat view
    if (window.location.pathname.startsWith('/overlay')) {
      return;
    }

    // Check if user has already acknowledged the banner
    const acknowledged = localStorage.getItem('cookieBannerAcknowledged');
    if (!acknowledged) {
      // Show banner after a short delay for better UX
      const timer = setTimeout(() => setShowBanner(true), 1000);
      return () => clearTimeout(timer);
    }
  }, [isHydrated]);

  const acknowledgeBanner = () => {
    localStorage.setItem('cookieBannerAcknowledged', 'true');
    setShowBanner(false);
  };

  if (!isHydrated || !showBanner) return null;

  return (
    <div className="pointer-events-none fixed inset-0 z-50 flex items-end justify-center p-4">
      <div className="pointer-events-auto w-full max-w-4xl animate-slide-up rounded-lg border-2 border-slate-200 bg-white shadow-2xl dark:border-slate-700 dark:bg-slate-800">
        {/* Main Banner */}
        <div className="p-6">
          <div className="flex items-start gap-4">
            {/* Cookie Icon */}
            <div className="flex-shrink-0 text-4xl" role="img" aria-label="Cookie">
              🍪
            </div>

            {/* Content */}
            <div className="flex-1">
              <h2 className="mb-2 text-xl font-semibold text-slate-900 dark:text-white">
                Privacy & Data Storage
              </h2>
              <p className="mb-3 text-sm text-slate-600 dark:text-slate-300">
                All-Chat uses <strong>browser local storage</strong> to save your authentication
                tokens and keep you logged in. This is essential for the service to function
                properly.
              </p>
              <p className="mb-3 text-sm text-slate-600 dark:text-slate-300">
                <strong>We do not use cookies for tracking or analytics.</strong> We do not share
                your data with third parties for advertising purposes.
              </p>

              {/* What we store */}
              <details className="mb-4">
                <summary className="cursor-pointer text-sm font-medium text-slate-900 hover:text-blue-600 dark:text-white dark:hover:text-blue-400">
                  What data do we store locally?
                </summary>
                <div className="mt-2 space-y-2 pl-4 text-sm text-slate-600 dark:text-slate-400">
                  <div className="flex items-start gap-2">
                    <span className="mt-0.5 text-green-600 dark:text-green-400">✓</span>
                    <div>
                      <strong>Authentication tokens</strong> - Required to keep you logged in and
                      authenticate with streaming platforms (Twitch, YouTube, TikTok, Kick)
                    </div>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="mt-0.5 text-green-600 dark:text-green-400">✓</span>
                    <div>
                      <strong>User preferences</strong> - Your overlay configurations and settings
                    </div>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="mt-0.5 text-red-600 dark:text-red-400">✗</span>
                    <div>
                      <strong>No tracking cookies</strong> - We don&apos;t track your browsing
                      behavior
                    </div>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="mt-0.5 text-red-600 dark:text-red-400">✗</span>
                    <div>
                      <strong>No advertising cookies</strong> - We don&apos;t serve ads or share
                      data with advertisers
                    </div>
                  </div>
                </div>
              </details>

              <p className="mb-4 text-sm text-slate-600 dark:text-slate-300">
                By using All-Chat, you agree to this essential data storage. For more details,
                please read our{' '}
                <a
                  href="/legal/privacy"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="font-medium text-blue-600 hover:underline dark:text-blue-400"
                >
                  Privacy Policy
                </a>{' '}
                and{' '}
                <a
                  href="/legal/terms"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="font-medium text-blue-600 hover:underline dark:text-blue-400"
                >
                  Terms of Service
                </a>
                .
              </p>

              {/* Action Button */}
              <div className="flex flex-wrap gap-3">
                <button
                  onClick={acknowledgeBanner}
                  className="rounded-lg bg-blue-600 px-6 py-2.5 font-medium text-white transition-colors hover:bg-blue-700 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2"
                >
                  I Understand
                </button>
                <a
                  href="/legal/privacy"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center justify-center rounded-lg bg-slate-200 px-6 py-2.5 font-medium text-slate-900 transition-colors hover:bg-slate-300 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-slate-500 focus-visible:ring-offset-2 dark:bg-slate-700 dark:text-white dark:hover:bg-slate-600"
                >
                  Learn More
                </a>
              </div>
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="rounded-b-lg border-t border-slate-200 bg-slate-50 px-6 py-3 dark:border-slate-700 dark:bg-slate-900">
          <p className="text-center text-xs text-slate-500 dark:text-slate-400">
            🔒 Your data is stored locally in your browser and transmitted securely via HTTPS
          </p>
        </div>
      </div>
    </div>
  );
}
