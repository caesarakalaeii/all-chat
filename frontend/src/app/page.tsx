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

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/lib/stores/auth-store';

export default function LandingPage() {
  const router = useRouter();
  const { user, token } = useAuthStore();

  // Redirect to dashboard if already logged in
  useEffect(() => {
    if (user && token) {
      router.push('/dashboard');
    }
  }, [user, token, router]);

  const handleLogin = async (platform: 'twitch' | 'youtube' | 'tiktok') => {
    try {
      // Use relative URL - Nginx will proxy to API Gateway
      const endpoint = platform === 'twitch' ? '/api/v1/auth/login' : `/api/v1/auth/${platform}/login`;
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
            Aggregate chat from Twitch, YouTube, TikTok and more in one beautiful overlay for your stream
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
              className="w-full bg-red-600 hover:bg-red-700 text-white font-semibold py-4 px-6 rounded-lg text-lg transition-colors flex items-center justify-center gap-2"
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
              <div className="text-xs text-gray-500">+ More coming</div>
            </div>
          </div>

          {/* Feature Grid */}
          <div className="mt-20 grid grid-cols-1 md:grid-cols-3 gap-8 max-w-4xl mx-auto">
            <div className="text-center p-6 bg-white/5 rounded-lg backdrop-blur-sm">
              <div className="text-4xl mb-4">🌐</div>
              <h3 className="text-xl font-semibold text-white mb-2">Multi-Platform</h3>
              <p className="text-gray-400">
                Combine chat from Twitch, YouTube, and TikTok in one unified overlay
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
          <div className="mt-20 text-gray-500 text-sm">
            <p>Open Source • Built with Go + React • Multi-Platform Chat Aggregation</p>
          </div>
        </div>
      </div>
    </div>
  );
}
