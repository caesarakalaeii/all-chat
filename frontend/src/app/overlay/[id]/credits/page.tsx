/**
 * Credit Roll Display Page (Unauthenticated)
 *
 * Public endpoint for displaying end-of-stream credits in OBS.
 * Shows leaderboards for subs, bits, raids, donations, follows, etc.
 *
 * Features:
 * - Scrolling credits with theme support
 * - Leaderboard categories
 * - User avatars and platform badges
 * - Configurable styling
 * - Auto-loop or single-play based on config
 *
 * This is a Client Component for animation and dynamic rendering.
 */

'use client';

import Image from 'next/image';
import { use, useEffect, useState, useRef } from 'react';
import Script from 'next/script';
import type { CreditRollResponse, CreditRollConfig, LeaderboardEntry } from '@/lib/types/overlay';

declare global {
  interface Window {
    Twitch: any;
  }
}

export default function CreditRollPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const [creditData, setCreditData] = useState<CreditRollResponse | null>(null);
  const [config, setConfig] = useState<CreditRollConfig | null>(null);
  const [customCss, setCustomCss] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [currentClipIndex, setCurrentClipIndex] = useState(0);
  const [twitchReady, setTwitchReady] = useState(false);
  const playerRef = useRef<any>(null);
  const playerContainerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const loadCreditRoll = async () => {
      try {
        // Load config
        const configResponse = await fetch(`/api/v1/overlays/public/${id}/creditroll`);
        if (configResponse.ok) {
          const configData = await configResponse.json();
          setConfig(configData);
          setCustomCss(configData.custom_css || '');
        }

        // Load credit roll data
        const dataResponse = await fetch(`/api/v1/overlays/public/${id}/credit-roll`);
        if (!dataResponse.ok) {
          const errorData = await dataResponse.json();
          throw new Error(errorData.error || 'Failed to load credit roll');
        }

        const data = await dataResponse.json();
        setCreditData(data);
      } catch (err) {
        console.error('Failed to load credit roll:', err);
        setError(err instanceof Error ? err.message : 'Failed to load credit roll');
      } finally {
        setLoading(false);
      }
    };

    loadCreditRoll();
  }, [id]);

  // Initialize Twitch Player when clip changes
  useEffect(() => {
    if (!twitchReady || !creditData?.clips || creditData.clips.length === 0) return;
    if (!playerContainerRef.current) return;

    const currentClip = creditData.clips[currentClipIndex];
    if (!currentClip) return;

    // Destroy previous player
    if (playerRef.current) {
      playerRef.current = null;
    }

    // Clear container
    playerContainerRef.current.innerHTML = '';

    // Create new player
    const options = {
      width: '100%',
      height: '100%',
      clip: currentClip.id,
      parent: [window.location.hostname],
      autoplay: true,
      muted: config?.clips_muted ?? true,
    };

    try {
      playerRef.current = new window.Twitch.Embed(playerContainerRef.current, options);

      // Listen for player ready event
      playerRef.current.addEventListener(window.Twitch.Embed.VIDEO_READY, () => {
        const player = playerRef.current.getPlayer();

        // Auto-advance when clip ends
        player.addEventListener(window.Twitch.Player.ENDED, () => {
          setCurrentClipIndex((prev) => (prev + 1) % creditData.clips.length);
        });

        // Fallback timer in case ENDED event doesn't fire
        const duration = (currentClip.duration || 30) * 1000 + 3000;
        setTimeout(() => {
          if (playerRef.current) {
            setCurrentClipIndex((prev) => (prev + 1) % creditData.clips.length);
          }
        }, duration);
      });
    } catch (err) {
      console.error('Failed to initialize Twitch player:', err);
    }
  }, [twitchReady, creditData, currentClipIndex, config?.clips_muted]);

  const renderLeaderboard = (title: string, entries: LeaderboardEntry[] | undefined, emoji: string) => {
    if (!entries || entries.length === 0) return null;

    return (
      <div className="mb-12">
        <h2 className="text-4xl font-bold text-white mb-6 flex items-center gap-3">
          <span className="text-5xl">{emoji}</span>
          {title}
        </h2>
        <div className="space-y-4">
          {entries.map((entry, index) => (
            <div
              key={index}
              className={`flex items-center gap-4 p-4 rounded-lg ${
                index === 0 ? 'bg-yellow-500/20 border-2 border-yellow-500' :
                index === 1 ? 'bg-gray-400/20 border-2 border-gray-400' :
                index === 2 ? 'bg-orange-600/20 border-2 border-orange-600' :
                'bg-gray-800/50 border border-gray-700'
              }`}
            >
              <div className={`text-3xl font-bold w-12 text-center ${
                index === 0 ? 'text-yellow-400' :
                index === 1 ? 'text-gray-300' :
                index === 2 ? 'text-orange-500' :
                'text-gray-500'
              }`}>
                #{entry.rank}
              </div>
              {entry.avatar_url && (
                <div className="relative w-12 h-12 rounded-full overflow-hidden">
                  <Image
                    src={entry.avatar_url}
                    alt={entry.display_name}
                    fill
                    className="object-cover"
                  />
                </div>
              )}
              <div className="flex-1">
                <div className="text-xl font-semibold text-white">{entry.display_name}</div>
                <div className="text-sm text-gray-400 capitalize">{entry.platform}</div>
              </div>
              <div className="text-2xl font-bold text-white">
                {entry.total_value !== undefined && entry.total_value > 0
                  ? `$${entry.total_value.toFixed(2)}`
                  : entry.count || ''}
              </div>
            </div>
          ))}
        </div>
      </div>
    );
  };

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-linear-to-b from-gray-900 to-black">
        <div className="text-center">
          <div className="animate-spin rounded-full h-16 w-16 border-b-4 border-white mx-auto mb-4"></div>
          <p className="text-white text-xl">Loading Credits...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-linear-to-b from-gray-900 to-black">
        <div className="text-center">
          <div className="text-6xl mb-4">⚠️</div>
          <p className="text-white text-xl mb-2">Unable to Load Credit Roll</p>
          <p className="text-gray-400 text-sm">{error}</p>
          <p className="text-gray-500 text-xs mt-4">Make sure you have an active streaming session</p>
        </div>
      </div>
    );
  }

  if (!creditData) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-linear-to-b from-gray-900 to-black">
        <div className="text-center">
          <p className="text-white text-xl">No credit roll data available</p>
        </div>
      </div>
    );
  }

  const theme = config?.theme || 'cinematic';
  const bgOpacity = config?.background_opacity || 0.8;

  const currentClip = creditData?.clips?.[currentClipIndex];

  return (
    <>
      {/* Load Twitch Embed SDK */}
      <Script
        src="https://embed.twitch.tv/embed/v1.js"
        onLoad={() => setTwitchReady(true)}
        strategy="afterInteractive"
      />

      {/* Custom CSS Injection */}
      {customCss && customCss.trim() && (
        <style dangerouslySetInnerHTML={{ __html: customCss }} />
      )}

      {/* Background Clip Video */}
      {config?.clips_enabled && currentClip && (
        <div className="fixed inset-0 z-0">
          <div
            ref={playerContainerRef}
            id="twitch-clip-player"
            className="absolute inset-0 w-full h-full"
          />
          {/* Overlay gradient for better text readability */}
          <div
            className="absolute inset-0 z-10"
            style={{
              background: `linear-gradient(to bottom, rgba(0, 0, 0, ${bgOpacity * 0.7}), rgba(0, 0, 0, ${bgOpacity * 0.5}))`,
              pointerEvents: 'none'
            }}
          />
        </div>
      )}

      <div
        className="min-h-screen overflow-hidden relative z-10"
        style={{
          background: (!config?.clips_enabled || !currentClip) ? (
            theme === 'cinematic'
              ? `linear-gradient(to bottom, rgba(17, 24, 39, ${bgOpacity}), rgba(0, 0, 0, ${bgOpacity}))`
              : theme === 'modern'
              ? `linear-gradient(135deg, rgba(99, 102, 241, ${bgOpacity * 0.3}), rgba(139, 92, 246, ${bgOpacity * 0.3}))`
              : `rgba(17, 24, 39, ${bgOpacity})`
          ) : 'transparent'
        }}
      >
        <div className="container mx-auto px-8 py-12">
          {/* Header */}
          <div className="text-center mb-16">
            <h1 className="text-6xl font-bold text-white mb-4 animate-fade-in">
              🎬 Stream Credits
            </h1>
            <p className="text-2xl text-gray-300">
              Thank you to everyone who supported the stream!
            </p>
            <div className="mt-4 text-gray-400">
              <p>Session: {new Date(creditData.session_started_at).toLocaleDateString()}</p>
              <p>Duration: {Math.floor(creditData.session_duration_seconds / 60)} minutes</p>
            </div>
          </div>

          {/* Leaderboards */}
          <div className="max-w-4xl mx-auto">
            {renderLeaderboard('Top Subscribers', creditData.leaderboards.subs, '⭐')}
            {renderLeaderboard('Top Gifters', creditData.leaderboards.gifts, '🎁')}
            {renderLeaderboard('Top Cheerers', creditData.leaderboards.bits, '💎')}
            {renderLeaderboard('Top Channel Points', creditData.leaderboards.points, '🎯')}
            {renderLeaderboard('Top Raiders', creditData.leaderboards.raids, '⚔️')}
            {renderLeaderboard('Top Super Chats', creditData.leaderboards.super_chats, '💰')}
            {renderLeaderboard('New Followers', creditData.leaderboards.follows, '❤️')}
          </div>

          {/* Now Playing Indicator */}
          {config?.clips_enabled && currentClip && (
            <div className="fixed bottom-8 right-8 z-50">
              <div className="bg-black/80 backdrop-blur-sm rounded-lg p-4 border border-gray-700 shadow-2xl max-w-sm">
                <div className="flex items-center gap-3 mb-2">
                  <span className="text-2xl">🎥</span>
                  <span className="text-white font-semibold">Now Playing</span>
                </div>
                <div className="text-lg text-white font-medium mb-1">{currentClip.title}</div>
                <div className="flex items-center justify-between text-sm text-gray-400">
                  <span>{currentClip.view_count.toLocaleString()} views</span>
                  <span>Clip {currentClipIndex + 1}/{creditData?.clips?.length || 0}</span>
                </div>
              </div>
            </div>
          )}

          {/* Footer */}
          <div className="text-center mt-24 mb-12">
            <div className="text-4xl font-bold text-white mb-4">
              Thank you for watching! ❤️
            </div>
            <p className="text-xl text-gray-300">
              See you next stream!
            </p>
          </div>
        </div>
      </div>

      <style jsx global>{`
        @keyframes fade-in {
          from {
            opacity: 0;
            transform: translateY(-20px);
          }
          to {
            opacity: 1;
            transform: translateY(0);
          }
        }

        .animate-fade-in {
          animation: fade-in 1s ease-out;
        }
        `}</style>
      </>
    );
}
