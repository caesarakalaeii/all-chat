/**
 * Landing Page
 *
 * The home page of All-Chat.
 * Displays product information and "Login with Twitch" button.
 *
 * Features:
 * - Hero section with gradient background
 * - Twitch OAuth login button
 * - Feature highlights
 * - Platform icons
 *
 * This is a Client Component because it uses browser APIs and state.
 */

'use client';

import Link from 'next/link';
import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/lib/stores/auth-store';
import { BetaWarning } from '@/components/BetaWarning';

export default function LandingPage() {
  const router = useRouter();
  const { user, token } = useAuthStore();
  const [showBetaWarning, setShowBetaWarning] = useState<'youtube' | 'tiktok' | null>(null);

  // Redirect to dashboard if already logged in
  useEffect(() => {
    if (user && token) {
      router.push('/dashboard');
    }
  }, [user, token, router]);

  const handleLogin = async (platform: 'twitch' | 'youtube' | 'tiktok' | 'kick') => {
    // Show beta warning for YouTube and TikTok
    if (platform === 'youtube' || platform === 'tiktok') {
      setShowBetaWarning(platform);
      return;
    }

    try {
      // Use relative URL - Nginx will proxy to API Gateway
      const endpoint = `/api/v1/auth/${platform}/login`;
      const response = await fetch(endpoint);
      const data = await response.json();

      if (data.auth_url) {
        window.location.href = data.auth_url;
      } else {
        console.error('No auth_url in response:', data);
      }
    } catch (error) {
      console.error('Failed to initiate login:', error);
    }
  };

  const proceedWithLogin = async (platform: 'youtube' | 'tiktok') => {
    setShowBetaWarning(null);
    try {
      const endpoint = `/api/v1/auth/${platform}/login`;
      const response = await fetch(endpoint);
      const data = await response.json();

      if (data.auth_url) {
        window.location.href = data.auth_url;
      } else {
        console.error('No auth_url in response:', data);
      }
    } catch (error) {
      console.error('Failed to initiate login:', error);
    }
  };

  return (
    <div className="min-h-screen bg-gradient-to-br from-purple-900 via-blue-900 to-indigo-900">
      <div className="container mx-auto px-4 py-20">
        <div className="text-center">
          {/* Hero Section */}
          <h1 className="text-6xl font-bold text-white mb-6 drop-shadow-lg">All-Chat</h1>
          <p className="text-xl text-gray-300 mb-12 max-w-2xl mx-auto">
            Aggregate chat from Twitch, YouTube, Kick, TikTok and more in one beautiful overlay for your stream
          </p>

          {/* Login Buttons */}
          <div className="max-w-md mx-auto space-y-4">
            <button
              onClick={() => handleLogin('twitch')}
              className="w-full bg-twitch hover:bg-purple-700 text-white font-semibold py-4 px-6 rounded-lg text-lg transition-colors flex items-center justify-center gap-2"
            >
              {/* Twitch Logo SVG */}
              <svg
                className="w-6 h-6"
                fill="currentColor"
                viewBox="0 0 24 24"
                xmlns="http://www.w3.org/2000/svg"
              >
                <path d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714z" />
              </svg>
              Login with Twitch
            </button>

            <button
              onClick={() => handleLogin('youtube')}
              className="w-full bg-red-600 hover:bg-red-700 text-white font-semibold py-4 px-6 rounded-lg text-lg transition-colors flex items-center justify-center gap-2 relative"
            >
              {/* YouTube Logo SVG */}
              <svg
                className="w-6 h-6"
                fill="currentColor"
                viewBox="0 0 24 24"
                xmlns="http://www.w3.org/2000/svg"
              >
                <path d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z" />
              </svg>
              Login with YouTube
              <span className="absolute -top-2 -right-2 bg-yellow-500 text-black text-xs font-bold px-2 py-1 rounded-full">
                BETA
              </span>
            </button>

            <button
              onClick={() => handleLogin('tiktok')}
              className="w-full bg-black hover:bg-gray-900 text-white font-semibold py-4 px-6 rounded-lg text-lg transition-colors flex items-center justify-center gap-2 relative"
            >
              {/* TikTok Logo SVG */}
              <svg
                className="w-6 h-6"
                fill="currentColor"
                viewBox="0 0 24 24"
                xmlns="http://www.w3.org/2000/svg"
              >
                <path d="M19.59 6.69a4.83 4.83 0 0 1-3.77-4.25V2h-3.45v13.67a2.89 2.89 0 0 1-5.2 1.74 2.89 2.89 0 0 1 2.31-4.64 2.93 2.93 0 0 1 .88.13V9.4a6.84 6.84 0 0 0-1-.05A6.33 6.33 0 0 0 5 20.1a6.34 6.34 0 0 0 10.86-4.43v-7a8.16 8.16 0 0 0 4.77 1.52v-3.4a4.85 4.85 0 0 1-1-.1z" />
              </svg>
              Login with TikTok
              <span className="absolute -top-2 -right-2 bg-yellow-500 text-black text-xs font-bold px-2 py-1 rounded-full">
                BETA
              </span>
            </button>

            <button
              onClick={() => handleLogin('kick')}
              className="w-full bg-green-500 hover:bg-green-600 text-white font-semibold py-4 px-6 rounded-lg text-lg transition-colors flex items-center justify-center gap-2"
            >
              {/* Kick Logo SVG */}
              <svg
                className="w-6 h-6"
                fill="currentColor"
                viewBox="0 0 24 24"
                xmlns="http://www.w3.org/2000/svg"
              >
                <path d="M21 0H3C1.343 0 0 1.343 0 3v18c0 1.657 1.343 3 3 3h18c1.657 0 3-1.343 3-3V3c0-1.657-1.343-3-3-3zm-9.5 17.5l-4-4 4-4v3h6v2h-6v3z" />
              </svg>
              Login with Kick
            </button>

            {/* Platform Indicators */}
            <div className="flex items-center justify-center gap-4 text-gray-400 text-sm flex-wrap">
              <div className="flex items-center gap-2">
                <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M11.571 4.714h1.715v5.143H11.57zm4.715 0H18v5.143h-1.714zM6 0L1.714 4.286v15.428h5.143V24l4.286-4.286h3.428L22.286 12V0zm14.571 11.143l-3.428 3.428h-3.429l-3 3v-3H6.857V1.714h13.714z" />
                </svg>
                Twitch
              </div>
              <div className="flex items-center gap-2">
                <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z" />
                </svg>
                YouTube
              </div>
              <div className="flex items-center gap-2">
                <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M19.59 6.69a4.83 4.83 0 0 1-3.77-4.25V2h-3.45v13.67a2.89 2.89 0 0 1-5.2 1.74 2.89 2.89 0 0 1 2.31-4.64 2.93 2.93 0 0 1 .88.13V9.4a6.84 6.84 0 0 0-1-.05A6.33 6.33 0 0 0 5 20.1a6.34 6.34 0 0 0 10.86-4.43v-7a8.16 8.16 0 0 0 4.77 1.52v-3.4a4.85 4.85 0 0 1-1-.1z" />
                </svg>
                TikTok
                <span className="text-xs bg-yellow-500/20 text-yellow-400 px-1.5 py-0.5 rounded">Beta</span>
              </div>
              <div className="flex items-center gap-2">
                <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M21 0H3C1.343 0 0 1.343 0 3v18c0 1.657 1.343 3 3 3h18c1.657 0 3-1.343 3-3V3c0-1.657-1.343-3-3-3zm-9.5 17.5l-4-4 4-4v3h6v2h-6v3z" />
                </svg>
                Kick
              </div>
            </div>
          </div>

          {/* Feature Grid */}
          <div className="mt-20 grid grid-cols-1 md:grid-cols-3 gap-8 max-w-4xl mx-auto">
            <div className="text-center p-6 bg-white/5 rounded-lg backdrop-blur-sm">
              <div className="text-4xl mb-4">🌐</div>
              <h3 className="text-xl font-semibold text-white mb-2">Multi-Platform</h3>
              <p className="text-gray-400">
                Combine chat from Twitch, YouTube, Kick, TikTok in one unified overlay
              </p>
            </div>

            <div className="text-center p-6 bg-white/5 rounded-lg backdrop-blur-sm">
              <div className="text-4xl mb-4">⚡</div>
              <h3 className="text-xl font-semibold text-white mb-2">Real-Time</h3>
              <p className="text-gray-400">Low latency chat delivery under 500ms for Twitch</p>
            </div>

            <div className="text-center p-6 bg-white/5 rounded-lg backdrop-blur-sm">
              <div className="text-4xl mb-4">🎨</div>
              <h3 className="text-xl font-semibold text-white mb-2">Customizable</h3>
              <p className="text-gray-400">
                Full control over appearance, emotes (7TV, BTTV, FFZ), and filtering
              </p>
            </div>
          </div>

          {/* Footer */}
          <div className="mt-20 text-gray-500 text-sm space-y-2">
            <p>Open Source • Built with Go + React • Multi-Platform Chat Aggregation</p>
            <p className="flex flex-wrap items-center justify-center gap-3 text-xs text-gray-400">
              <a
                href="https://discord.gg/xCGBSuz39P"
                target="_blank"
                rel="noopener noreferrer"
                className="hover:text-gray-200 underline-offset-4 hover:underline flex items-center gap-1"
              >
                <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
                  <path d="M20.317 4.37a19.791 19.791 0 0 0-4.885-1.515.074.074 0 0 0-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 0 0-5.487 0 12.64 12.64 0 0 0-.617-1.25.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 0 0 .031.057 19.9 19.9 0 0 0 5.993 3.03.078.078 0 0 0 .084-.028c.462-.63.874-1.295 1.226-1.994a.076.076 0 0 0-.041-.106 13.107 13.107 0 0 1-1.872-.892.077.077 0 0 1-.008-.128 10.2 10.2 0 0 0 .372-.292.074.074 0 0 1 .077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127 12.299 12.299 0 0 1-1.873.892.077.077 0 0 0-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028 19.839 19.839 0 0 0 6.002-3.03.077.077 0 0 0 .032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 0 0-.031-.03zM8.02 15.33c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.955-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.946 2.418-2.157 2.418z"/>
                </svg>
                Get Support on Discord
              </a>
              <span aria-hidden="true">•</span>
              <Link href="/legal/privacy" className="hover:text-gray-200 underline-offset-4 hover:underline">
                Privacy Policy
              </Link>
              <span aria-hidden="true">•</span>
              <Link href="/legal/terms" className="hover:text-gray-200 underline-offset-4 hover:underline">
                Terms of Service
              </Link>
            </p>
          </div>
        </div>
      </div>

      {/* Beta Warning Modal */}
      {showBetaWarning && (
        <BetaWarning
          platform={showBetaWarning}
          onCancel={() => setShowBetaWarning(null)}
          onContinue={() => {
            const platform = showBetaWarning;
            setShowBetaWarning(null);
            proceedWithLogin(platform);
          }}
        />
      )}
    </div>
  );
}
