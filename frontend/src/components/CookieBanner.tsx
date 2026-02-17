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
    <div className="fixed inset-0 z-50 flex items-end justify-center p-4 pointer-events-none">
      <div className="w-full max-w-4xl bg-white dark:bg-gray-800 rounded-lg shadow-2xl pointer-events-auto border-2 border-gray-200 dark:border-gray-700 animate-slide-up">
        {/* Main Banner */}
        <div className="p-6">
          <div className="flex items-start gap-4">
            {/* Cookie Icon */}
            <div className="flex-shrink-0 text-4xl" role="img" aria-label="Cookie">
              🍪
            </div>

            {/* Content */}
            <div className="flex-1">
              <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-2">
                Privacy & Data Storage
              </h2>
              <p className="text-sm text-gray-600 dark:text-gray-300 mb-3">
                All-Chat uses <strong>browser local storage</strong> to save your authentication tokens
                and keep you logged in. This is essential for the service to function properly.
              </p>
              <p className="text-sm text-gray-600 dark:text-gray-300 mb-3">
                <strong>We do not use cookies for tracking or analytics.</strong> We do not share your
                data with third parties for advertising purposes.
              </p>

              {/* What we store */}
              <details className="mb-4">
                <summary className="text-sm font-medium text-gray-900 dark:text-white cursor-pointer hover:text-blue-600 dark:hover:text-blue-400">
                  What data do we store locally?
                </summary>
                <div className="mt-2 pl-4 space-y-2 text-sm text-gray-600 dark:text-gray-400">
                  <div className="flex items-start gap-2">
                    <span className="text-green-600 dark:text-green-400 mt-0.5">✓</span>
                    <div>
                      <strong>Authentication tokens</strong> - Required to keep you logged in and
                      authenticate with streaming platforms (Twitch, YouTube, TikTok, Kick)
                    </div>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="text-green-600 dark:text-green-400 mt-0.5">✓</span>
                    <div>
                      <strong>User preferences</strong> - Your overlay configurations and settings
                    </div>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="text-red-600 dark:text-red-400 mt-0.5">✗</span>
                    <div>
                      <strong>No tracking cookies</strong> - We don&apos;t track your browsing behavior
                    </div>
                  </div>
                  <div className="flex items-start gap-2">
                    <span className="text-red-600 dark:text-red-400 mt-0.5">✗</span>
                    <div>
                      <strong>No advertising cookies</strong> - We don&apos;t serve ads or share data with advertisers
                    </div>
                  </div>
                </div>
              </details>

              <p className="text-sm text-gray-600 dark:text-gray-300 mb-4">
                By using All-Chat, you agree to this essential data storage. For more details, please
                read our{' '}
                <a
                  href="/legal/privacy"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-blue-600 dark:text-blue-400 hover:underline font-medium"
                >
                  Privacy Policy
                </a>{' '}
                and{' '}
                <a
                  href="/legal/terms"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-blue-600 dark:text-blue-400 hover:underline font-medium"
                >
                  Terms of Service
                </a>.
              </p>

              {/* Action Button */}
              <div className="flex flex-wrap gap-3">
                <button
                  onClick={acknowledgeBanner}
                  className="px-6 py-2.5 bg-blue-600 hover:bg-blue-700 text-white font-medium rounded-lg transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
                >
                  I Understand
                </button>
                <a
                  href="/legal/privacy"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="px-6 py-2.5 bg-gray-200 hover:bg-gray-300 dark:bg-gray-700 dark:hover:bg-gray-600 text-gray-900 dark:text-white font-medium rounded-lg transition-colors focus:outline-none focus:ring-2 focus:ring-gray-500 focus:ring-offset-2 inline-flex items-center justify-center"
                >
                  Learn More
                </a>
              </div>
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="px-6 py-3 bg-gray-50 dark:bg-gray-900 rounded-b-lg border-t border-gray-200 dark:border-gray-700">
          <p className="text-xs text-gray-500 dark:text-gray-400 text-center">
            🔒 Your data is stored locally in your browser and transmitted securely via HTTPS
          </p>
        </div>
      </div>
    </div>
  );
}
